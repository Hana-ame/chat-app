package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/gorilla/websocket"
)

const (
	writeWait     = 10 * time.Second
	pongWait      = 60 * time.Second
	pingPeriod    = 50 * time.Second
	sendQueueSize = 64
)

type Gateway struct {
	hub            *Hub
	db             *db.DB
	authSvc        *auth.Service
	maxMessageSize int64
	upgrader       websocket.Upgrader
}

// NewGateway 装配 WS 网关。CheckOrigin 按 CORS 白名单注入:浏览器 WS 握手
// 携带 Origin,跨站页面发起的连接在此被拒绝(CSWSH 纵深防御;认证本身由
// access_token 负责)。gorilla 对未携带 Origin 的原始请求不调用
// CheckOrigin;多数非浏览器库会自动补 "http://<ws-host>" 形式的 Origin,
// 该值同样按白名单校验。
func NewGateway(hub *Hub, database *db.DB, authSvc *auth.Service, maxMessageSize int64, cfg *config.Config) *Gateway {
	if maxMessageSize <= 0 {
		maxMessageSize = 1 << 16
	}
	hub.SetPresenceHandler(func(ctx context.Context, userID string, online bool) {
		var status string
		if online {
			status = "online"
		} else {
			status = "offline"
		}
		if err := database.UpdateUserStatus(ctx, userID, status); err != nil {
			logutil.Error("presence: failed to set %s for %s: %v", status, logutil.SafeID(userID), err)
		}
		if err := database.UpdateUserLastSeen(ctx, userID); err != nil {
			logutil.Error("presence: failed to update last_seen for %s: %v", logutil.SafeID(userID), err)
		}
	})
	return &Gateway{
		hub:            hub,
		db:             database,
		authSvc:        authSvc,
		maxMessageSize: maxMessageSize,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return cfg.OriginAllowed(r.Header.Get("Origin"))
			},
		},
	}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if v := os.Getenv("WS_ENABLED"); v != "" && v != "true" {
		logutil.Warn("WebSocket disabled by WS_ENABLED env")
		http.Error(w, "WebSocket is disabled in this version", http.StatusForbidden)
		return
	}
	tok := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		tok = strings.TrimSpace(h[7:])
	}
	if tok == "" {
		if c, err := r.Cookie("access_token"); err == nil {
			tok = c.Value
		}
	}
	// Note: no ?access_token= query support — tokens in URLs leak via access
	// logs and Referer headers. Browsers send cookies automatically; other
	// clients must use the Authorization header or the cookie.
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

	conn, err := g.upgrader.Upgrade(w, r, nil)
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
