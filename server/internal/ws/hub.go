package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type Op string

const (
	OpReady           Op = "ready"
	OpMessageCreate   Op = "message_create"
	OpMessageUpdate   Op = "message_update"
	OpMessageDelete   Op = "message_delete"
	OpReactionAdd     Op = "reaction_add"
	OpReactionRemove  Op = "reaction_remove"
	OpChatCreate      Op = "chat_create"
	OpChatUpdate      Op = "chat_update"
	OpChatDelete      Op = "chat_delete"
	OpChatRemove      Op = "chat_remove"
	OpUserUpdate      Op = "user_update"
	OpPresenceUpdate  Op = "presence_update"
	OpTyping          Op = "typing"
	OpPing            Op = "ping"
	OpPong            Op = "pong"
	OpSubscribe       Op = "subscribe"
	OpUnsubscribe     Op = "unsubscribe"
	OpError           Op = "error"
)

type Envelope struct {
	Op      Op              `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
	db      *db.DB
}

func NewHub(database *db.DB) *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
		db:      database,
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		set = map[*Client]struct{}{}
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
	wasOffline := len(set) == 1
	if wasOffline && h.db != nil {
		_ = h.db.UpdateUserStatus(context.Background(), c.userID, "online")
		go h.broadcastPresence(c.userID, "online")
	}
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	wasLast := false
	if set, ok := h.clients[c.userID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.userID)
			wasLast = true
		}
	}
	h.mu.Unlock()
	if wasLast && h.db != nil {
		_ = h.db.UpdateUserStatus(context.Background(), c.userID, "offline")
		h.broadcastPresence(c.userID, "offline")
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.clients {
		n += len(set)
	}
	return n
}

func (h *Hub) Online(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

func (h *Hub) OnlineUserIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.clients))
	for u := range h.clients {
		out = append(out, u)
	}
	return out
}

func (h *Hub) snapshotForUser(userID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	set, ok := h.clients[userID]
	if !ok {
		return nil
	}
	out := make([]*Client, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	return out
}

func (h *Hub) sendToUser(userID string, env Envelope) {
	for _, c := range h.snapshotForUser(userID) {
		c.queue(env)
	}
}

func (h *Hub) sendToChat(chatID string, env Envelope, exceptUser string) {
	if h.db == nil {
		return
	}
	members, err := h.db.GetChatMembers(context.Background(), chatID)
	if err != nil {
		log.Printf("ws: failed to load members for chat %s: %v", chatID, err)
		return
	}
	for _, m := range members {
		if m.ID == exceptUser {
			continue
		}
		for _, c := range h.snapshotForUser(m.ID) {
			if c.subscribed(chatID) {
				c.queue(env)
			} else {
				c.queue(env)
			}
		}
	}
}

func envelope(op Op, payload interface{}) Envelope {
	b, _ := json.Marshal(payload)
	return Envelope{Op: op, Payload: b}
}

func (h *Hub) BroadcastMessageCreate(m *models.Message) {
	h.sendToChat(m.ChatID, envelope(OpMessageCreate, m), "")
}

func (h *Hub) BroadcastMessageUpdate(m *models.Message) {
	h.sendToChat(m.ChatID, envelope(OpMessageUpdate, m), "")
}

func (h *Hub) BroadcastMessageDelete(chatID, messageID string) {
	h.sendToChat(chatID, envelope(OpMessageDelete, map[string]string{
		"chat_id": chatID, "message_id": messageID,
	}), "")
}

func (h *Hub) BroadcastReaction(chatID, messageID, emoji, userID string, added bool) {
	op := OpReactionRemove
	if added {
		op = OpReactionAdd
	}
	h.sendToChat(chatID, envelope(op, map[string]string{
		"chat_id":    chatID,
		"message_id": messageID,
		"emoji":      emoji,
		"user_id":    userID,
	}), "")
}

func (h *Hub) BroadcastChatCreated(c *models.Chat) {
	for _, m := range c.Members {
		h.sendToUser(m.ID, envelope(OpChatCreate, c))
	}
}

func (h *Hub) BroadcastChatUpdated(c *models.Chat) {
	for _, m := range c.Members {
		h.sendToUser(m.ID, envelope(OpChatUpdate, c))
	}
}

func (h *Hub) BroadcastChatDeleted(c *models.Chat, chatID string) {
	payload := map[string]string{"chat_id": chatID}
	for _, m := range c.Members {
		h.sendToUser(m.ID, envelope(OpChatDelete, payload))
	}
}

func (h *Hub) NotifyUserNewChat(userID string, c *models.Chat) {
	h.sendToUser(userID, envelope(OpChatCreate, c))
}

func (h *Hub) NotifyUserLeftChat(userID, chatID string) {
	h.sendToUser(userID, envelope(OpChatRemove, map[string]string{"chat_id": chatID}))
}

func (h *Hub) BroadcastUserUpdate(u *models.User) {
	env := envelope(OpUserUpdate, u)
	h.mu.RLock()
	chatIDs := map[string]struct{}{}
	for c := range h.clients[u.ID] {
		for cid := range c.subs {
			chatIDs[cid] = struct{}{}
		}
	}
	h.mu.RUnlock()
	h.mu.RLock()
	all := make([]*Client, 0)
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.queue(env)
	}
}

func (h *Hub) broadcastPresence(userID, status string) {
	env := envelope(OpPresenceUpdate, map[string]string{
		"user_id": userID, "status": status,
	})
	h.mu.RLock()
	all := make([]*Client, 0)
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.queue(env)
	}
}

func (h *Hub) BroadcastTyping(chatID, userID string) {
	h.sendToChat(chatID, envelope(OpTyping, map[string]any{
		"chat_id":   chatID,
		"user_id":   userID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}), userID)
}
