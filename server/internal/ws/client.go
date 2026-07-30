package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/gorilla/websocket"
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID string
	send   chan Envelope

	mu        sync.RWMutex
	subs      map[string]struct{}
	closed    bool
	closeOnce sync.Once
}

func (c *Client) subscribed(chatID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.subs[chatID]
	return ok
}

func (c *Client) subscribe(chatID string) {
	c.mu.Lock()
	c.subs[chatID] = struct{}{}
	c.mu.Unlock()
}

func (c *Client) unsubscribe(chatID string) {
	c.mu.Lock()
	delete(c.subs, chatID)
	c.mu.Unlock()
}

func (c *Client) queue(env Envelope) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()
	select {
	case c.send <- env:
	default:
		c.close()
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
		_ = c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"),
			time.Now().Add(writeWait))
		_ = c.conn.Close()
	})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.close()
		logutil.Debug("ws read pump ended: user=%s", logutil.SafeID(c.userID))
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			c.queue(envelope(OpError, map[string]string{"message": "invalid json"}))
			continue
		}
		switch env.Op {
		case OpPing:
			c.queue(Envelope{Op: OpPong})
		case OpSubscribe:
			var p struct {
				ChatID string `json:"chat_id"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
				continue
			}
			if c.hub.memberStore != nil {
				ok, err := c.hub.memberStore.IsChatMember(context.Background(), p.ChatID, c.userID)
				if err != nil {
					logutil.Error("ws: check member for subscribe %s: %v", logutil.SafeID(p.ChatID), err)
					continue
				}
				if ok {
					c.subscribe(p.ChatID)
				}
			}
		case OpUnsubscribe:
			var p struct {
				ChatID string `json:"chat_id"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
				continue
			}
			c.unsubscribe(p.ChatID)
		case OpListMembers:
			var p struct {
				ChatID string `json:"chat_id"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
				continue
			}
			if c.hub.memberStore != nil {
				members, err := c.hub.memberStore.GetChatMembers(context.Background(), p.ChatID)
				if err == nil {
					b, err := json.Marshal(map[string]any{"chat_id": p.ChatID, "members": members})
					if err != nil {
						logutil.Error("ws: marshal member list: %v", err)
						continue
					}
					c.queue(Envelope{Op: OpMembersList, ReqID: env.ReqID, Payload: b})
				}
			}
		case OpTyping:
			var p struct {
				ChatID string `json:"chat_id"`
			}
			if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
				continue
			}
			if c.hub.memberStore != nil {
				ok, err := c.hub.memberStore.IsChatMember(context.Background(), p.ChatID, c.userID)
				if err != nil {
					logutil.Error("ws: check member for typing %s: %v", logutil.SafeID(p.ChatID), err)
					continue
				}
				if ok {
					c.hub.BroadcastTyping(p.ChatID, c.userID)
				}
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case env, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(env); err != nil {
				logutil.Error("ws write error for %s: %v", logutil.SafeID(c.userID), err)
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
