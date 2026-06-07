package ws

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 50 * time.Second
	maxMessageSize = 1 << 16
	sendQueueSize  = 64
)

type Gateway struct {
	hub     *Hub
	db      *db.DB
	authSvc *auth.Service
}

func NewGateway(hub *Hub, database *db.DB, authSvc *auth.Service) *Gateway {
	return &Gateway{hub: hub, db: database, authSvc: authSvc}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok := r.URL.Query().Get("access_token")
	if tok == "" {
		http.Error(w, "missing access_token", http.StatusUnauthorized)
		return
	}
	claims, err := g.authSvc.ParseAccessToken(tok)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	user, err := g.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "user gone", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(maxMessageSize)

	c := &Client{
		hub:    g.hub,
		conn:   conn,
		userID: user.ID,
		send:   make(chan Envelope, sendQueueSize),
		subs:   make(map[string]struct{}),
	}
	chats, _ := g.db.ListUserChats(r.Context(), user.ID)
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