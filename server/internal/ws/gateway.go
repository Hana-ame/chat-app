package ws

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	writeWait     = 10 * time.Second
	pongWait      = 60 * time.Second
	pingPeriod    = 50 * time.Second
	sendQueueSize = 64
)

type Gateway struct {
	hub           *Hub
	db            *db.DB
	authSvc       *auth.Service
	maxMessageSize int64
}

func NewGateway(hub *Hub, database *db.DB, authSvc *auth.Service, maxMessageSize int64) *Gateway {
	if maxMessageSize <= 0 {
		maxMessageSize = 1 << 16
	}
	return &Gateway{hub: hub, db: database, authSvc: authSvc, maxMessageSize: maxMessageSize}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Enable WebSocket by default; optional env var WS_ENABLED can be used to disable.
    if v := os.Getenv("WS_ENABLED"); v != "" && v != "true" {
        logutil.Warn("WebSocket disabled by WS_ENABLED env")
        http.Error(w, "WebSocket is disabled in this version", http.StatusForbidden)
        return
    }
	tok := r.URL.Query().Get("access_token")
	if tok == "" {
		logutil.Warn("ws connect: missing access_token")
		http.Error(w, "missing access_token", http.StatusUnauthorized)
		return
	}
	claims, err := g.authSvc.ParseAccessToken(tok)
	if err != nil {
		logutil.Warn("ws connect: invalid token")
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	user, err := g.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		logutil.Warn("ws connect: user gone (%s)", logutil.SafeID(claims.UserID))
		http.Error(w, "user gone", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logutil.Error("ws upgrade failed for %s: %v", logutil.SafeID(user.ID), err)
		return
	}
	conn.SetReadLimit(g.maxMessageSize)
	logutil.Info("ws connected: user=%s", logutil.SafeID(user.ID))

	c := &Client{
		hub:    g.hub,
		conn:   conn,
		userID: user.ID,
		send:   make(chan Envelope, sendQueueSize),
		subs:   make(map[string]struct{}),
	}
	chats, err := g.db.ListUserChats(r.Context(), user.ID)
	if err != nil {
		logutil.Error("ws ready: list chats for %s: %v", logutil.SafeID(user.ID), err)
		chats = []models.Chat{}
	}
	for _, ch := range chats {
		c.subscribe(ch.ID)
	}

	readyPayload, _ := json.Marshal(map[string]any{
		"user":            user,
		"chats":           chats,
		"online_user_ids": g.hub.OnlineUserIDs(),
	})
	c.send <- Envelope{Op: OpReady, Payload: readyPayload}
	g.hub.register(c)

	go c.writePump()
	go c.readPump()
}