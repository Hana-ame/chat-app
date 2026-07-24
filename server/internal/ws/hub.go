package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type Op string

const (
	OpReady          Op = "ready"
	OpMessageCreate  Op = "message_create"
	OpMessageUpdate  Op = "message_update"
	OpMessageDelete  Op = "message_delete"
	OpReactionAdd    Op = "reaction_add"
	OpReactionRemove Op = "reaction_remove"
	OpChatCreate     Op = "chat_create"
	OpChatUpdate     Op = "chat_update"
	OpChatDelete     Op = "chat_delete"
	OpChatRemove     Op = "chat_remove"
	OpUserUpdate     Op = "user_update"
	OpPresenceUpdate Op = "presence_update"
	OpTyping         Op = "typing"
	OpPing           Op = "ping"
	OpPong           Op = "pong"
	OpSubscribe      Op = "subscribe"
	OpUnsubscribe    Op = "unsubscribe"
	OpListMembers    Op = "list_members"
	OpMembersList    Op = "members_list"
	OpError          Op = "error"
)

type Envelope struct {
	Op      Op              `json:"op"`
	ReqID   int             `json:"req_id,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type memberCacheEntry struct {
	members []models.User
	expires time.Time
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]struct{}
	sseClients map[string][]chan []byte
	db         *db.DB
	memberMu   sync.Mutex
	memberCache map[string]*memberCacheEntry
}

func NewHub(database *db.DB) *Hub {
	return &Hub{
		clients:     make(map[string]map[*Client]struct{}),
		sseClients:  make(map[string][]chan []byte),
		db:          database,
		memberCache: make(map[string]*memberCacheEntry),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	set, ok := h.clients[c.userID]
	if !ok {
		set = map[*Client]struct{}{}
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
	wasOffline := len(set) == 1
	h.mu.Unlock()
	logutil.Info("ws client registered: user=%s (total=%d)", c.userID[:8], h.ClientCount())
	if wasOffline && h.db != nil {
		if err := h.db.UpdateUserStatus(context.Background(), c.userID, "online"); err != nil {
			logutil.Error("presence: failed to set online for %s: %v", c.userID[:8], err)
		}
		if err := h.db.UpdateUserLastSeen(context.Background(), c.userID); err != nil {
			logutil.Error("presence: failed to update last_seen for %s: %v", c.userID[:8], err)
		}
		logutil.Debug("presence: %s -> online", c.userID[:8])
		h.broadcastPresence(c.userID, "online")
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
	logutil.Info("ws client unregistered: user=%s (total=%d)", c.userID[:8], h.ClientCount())
	if wasLast && h.db != nil {
		if err := h.db.UpdateUserStatus(context.Background(), c.userID, "offline"); err != nil {
			logutil.Error("presence: failed to set offline for %s: %v", c.userID[:8], err)
		}
		if err := h.db.UpdateUserLastSeen(context.Background(), c.userID); err != nil {
			logutil.Error("presence: failed to update last_seen for %s: %v", c.userID[:8], err)
		}
		logutil.Debug("presence: %s -> offline", c.userID[:8])
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
	clients := h.snapshotForUser(userID)
	for _, c := range clients {
		c.queue(env)
	}
	if len(clients) > 0 {
		return
	}
	b, err := json.Marshal(env)
	if err != nil {
		logutil.Error("ws: marshal envelope %s: %v", env.Op, err)
		return
	}
	h.sseSend(userID, b)
}

func (h *Hub) sendToChat(chatID string, env Envelope, exceptUser string) {
	if h.db == nil {
		return
	}
	members, ok := h.getCachedMembers(chatID)
	if !ok {
		var err error
		members, err = h.db.GetChatMembers(context.Background(), chatID)
		if err != nil {
			logutil.Error("ws: failed to load members for chat %s: %v", chatID[:8], err)
			return
		}
		h.setCachedMembers(chatID, members)
	}
	b, err := json.Marshal(env)
	if err != nil {
		logutil.Error("ws: marshal %s for chat %s: %v", env.Op, chatID[:8], err)
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0)
	sseTargets := make([]string, 0, len(members))
	for _, m := range members {
		if m.ID == exceptUser {
			continue
		}
		if set, ok := h.clients[m.ID]; ok && len(set) > 0 {
			for c := range set {
				clients = append(clients, c)
			}
		} else {
			sseTargets = append(sseTargets, m.ID)
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		c.queue(env)
	}
	for _, uid := range sseTargets {
		h.sseSend(uid, b)
	}
}

const memberCacheTTL = 1 * time.Second

func (h *Hub) getCachedMembers(chatID string) ([]models.User, bool) {
	h.memberMu.Lock()
	defer h.memberMu.Unlock()
	e, ok := h.memberCache[chatID]
	if !ok || time.Now().After(e.expires) {
		return nil, false
	}
	return e.members, true
}

func (h *Hub) setCachedMembers(chatID string, members []models.User) {
	h.memberMu.Lock()
	defer h.memberMu.Unlock()
	h.memberCache[chatID] = &memberCacheEntry{members: members, expires: time.Now().Add(memberCacheTTL)}
}

func envelope(op Op, payload interface{}) Envelope {
	b, err := json.Marshal(payload)
	if err != nil {
		logutil.Error("ws: marshal envelope %s: %v", op, err)
		return Envelope{Op: op}
	}
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
	h.sendToChat(c.ID, envelope(OpChatCreate, c), "")
}

func (h *Hub) BroadcastChatUpdated(c *models.Chat) {
	h.sendToChat(c.ID, envelope(OpChatUpdate, c), "")
}

func (h *Hub) BroadcastChatDeleted(c *models.Chat, chatID string) {
	h.sendToChat(chatID, envelope(OpChatDelete, map[string]string{"chat_id": chatID}), "")
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
	all := make([]*Client, 0)
	wsUserIDs := make(map[string]struct{})
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
			wsUserIDs[c.userID] = struct{}{}
		}
	}
	sseUserIDs := make([]string, 0, len(h.sseClients))
	for uid := range h.sseClients {
		if _, ok := wsUserIDs[uid]; !ok {
			sseUserIDs = append(sseUserIDs, uid)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.queue(env)
	}
	b, err := json.Marshal(env)
	if err != nil {
		logutil.Error("ws: marshal user_update: %v", err)
		return
	}
	for _, uid := range sseUserIDs {
		h.sseSend(uid, b)
	}
}

func (h *Hub) sendToAllSSE(b []byte) {
	h.mu.RLock()
	userIDs := make([]string, 0, len(h.sseClients))
	for uid := range h.sseClients {
		userIDs = append(userIDs, uid)
	}
	h.mu.RUnlock()
	for _, uid := range userIDs {
		h.sseSend(uid, b)
	}
}

func (h *Hub) broadcastPresence(userID, status string) {
	env := envelope(OpPresenceUpdate, map[string]string{
		"user_id": userID, "status": status,
	})
	h.mu.RLock()
	all := make([]*Client, 0)
	wsUserIDs := make(map[string]struct{})
	for _, set := range h.clients {
		for c := range set {
			all = append(all, c)
			wsUserIDs[c.userID] = struct{}{}
		}
	}
	sseUserIDs := make([]string, 0, len(h.sseClients))
	for uid := range h.sseClients {
		if _, ok := wsUserIDs[uid]; !ok {
			sseUserIDs = append(sseUserIDs, uid)
		}
	}
	h.mu.RUnlock()
	for _, c := range all {
		c.queue(env)
	}
	b, err := json.Marshal(env)
	if err != nil {
		logutil.Error("ws: marshal presence_update: %v", err)
		return
	}
	for _, uid := range sseUserIDs {
		h.sseSend(uid, b)
	}
}

func (h *Hub) BroadcastTyping(chatID, userID string) {
	h.sendToChat(chatID, envelope(OpTyping, map[string]any{
		"chat_id":   chatID,
		"user_id":   userID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}), userID)
}

func (h *Hub) SSERegister(userID string, ch chan []byte) {
	h.mu.Lock()
	h.sseClients[userID] = append(h.sseClients[userID], ch)
	h.mu.Unlock()
}

func (h *Hub) SSEUnregister(userID string) {
	h.mu.Lock()
	delete(h.sseClients, userID)
	h.mu.Unlock()
}

func (h *Hub) Shutdown() {
	h.mu.Lock()
	for _, set := range h.clients {
		for c := range set {
			c.close()
		}
	}
	h.clients = make(map[string]map[*Client]struct{})
	for uid, chs := range h.sseClients {
		for _, ch := range chs {
			close(ch)
		}
		delete(h.sseClients, uid)
	}
	h.mu.Unlock()
	logutil.Info("ws hub shut down")
}

func (h *Hub) sseSend(userID string, data []byte) {
	h.mu.RLock()
	chs := h.sseClients[userID]
	h.mu.RUnlock()
	for _, ch := range chs {
		select {
		case ch <- data:
		default:
		}
	}
}
