# WebSocket 实现细节规范 (WS Implementation Spec)

## 1. Gateway: 握手与连接建立
### `ServeHTTP(w, r)`
**目的:** 处理 WS 升级请求，验证身份，初始化 Client 并将其注册到 Hub。

```go
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if v := os.Getenv("WS_ENABLED"); v != "" && v != "true" {
        http.Error(w, "WebSocket is disabled in this version", http.StatusForbidden)
        return
    }
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
```
**依赖链:** `os.Getenv(WS_ENABLED) → r.URL.Query(access_token) → Auth.ParseAccessToken → DB.GetUserByID → upgrader.Upgrade → DB.ListUserChats → c.subscribe → c.send(OpReady) → Hub.register → go writePump/readPump`

**条件分支:**
- `WS_ENABLED != "true"` $\rightarrow$ `403 Forbidden`
- `access_token == ""` $\rightarrow$ `401 Unauthorized`
- `Auth.ParseAccessToken` 失败 $\rightarrow$ `401 Unauthorized`
- `DB.GetUserByID` 失败 $\rightarrow$ `401 Unauthorized`
- `upgrader.Upgrade` 失败 $\rightarrow$ 连接终止

---

## 2. Client: 连接读写泵
### `readPump()`
**目的:** 监听客户端发送的消息，解析 `Envelope` 并执行相应指令。

```go
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister(c)
        c.close()
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
            var p struct{ ChatID string `json:"chat_id"` }
            if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
                continue
            }
            if c.hub.db != nil {
                if ok, _ := c.hub.db.IsChatMember(context.Background(), p.ChatID, c.userID); ok {
                    c.subscribe(p.ChatID)
                }
            }
        case OpUnsubscribe:
            var p struct{ ChatID string `json:"chat_id"` }
            if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
                continue
            }
            c.unsubscribe(p.ChatID)
        case OpTyping:
            var p struct{ ChatID string `json:"chat_id"` }
            if err := json.Unmarshal(env.Payload, &p); err != nil || p.ChatID == "" {
                continue
            }
            if c.hub.db != nil {
                if ok, _ := c.hub.db.IsChatMember(context.Background(), p.ChatID, c.userID); ok {
                    c.hub.BroadcastTyping(p.ChatID, c.userID)
                }
            }
        }
    }
}
```
**依赖链:** `SetReadDeadline → PongHandler → ReadMessage → json.Unmarshal(Envelope) → switch env.Op → (DB.IsChatMember $\rightarrow$ Action) → defer unregister`

**条件分支:**
- `ReadMessage` 错误 $\rightarrow$ 退出循环 $\rightarrow$ `unregister`
- `Unmarshal(Envelope)` 失败 $\rightarrow$ 发送 `OpError` $\rightarrow$ 继续
- `OpSubscribe/Typing` $\rightarrow$ `DB.IsChatMember == false` $\rightarrow$ 忽略操作

### `writePump()`
**目的:** 将 `send` channel 中的消息写入 WS 连接，并定期发送 Ping 维持心跳。

```go
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
```
**依赖链:** `Ticker(50s) $\rightarrow$ select { c.send / ticker.C } $\rightarrow$ SetWriteDeadline $\rightarrow$ WriteJSON/WriteMessage`

---

## 3. Hub: 连接管理与广播
### `register(c *Client)` & `unregister(c *Client)`
**目的:** 管理用户连接集合，并同步更新数据库在线状态。

```go
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
    if wasOffline && h.db != nil {
        _ = h.db.UpdateUserStatus(context.Background(), c.userID, "online")
        _ = h.db.UpdateUserLastSeen(context.Background(), c.userID)
        h.broadcastPresence(c.userID, "online")
    }
}
```
**逻辑流:** `Lock $\rightarrow$ 更新 clients map $\rightarrow$ 判定是否为首个连接 (wasOffline) $\rightarrow$ Unlock $\rightarrow$ DB.UpdateUserStatus("online") $\rightarrow$ broadcastPresence`

### `sendToChat(chatID, env, exceptUser)`
**目的:** 将消息广播给特定聊天的所有在线成员。

```go
func (h *Hub) sendToChat(chatID string, env Envelope, exceptUser string) {
    if h.db == nil { return }
    members, err := h.db.GetChatMembers(context.Background(), chatID)
    if err != nil { return }
    b, _ := json.Marshal(env)
    for _, m := range members {
        if m.ID == exceptUser { continue }
        for _, c := range h.snapshotForUser(m.ID) {
            c.queue(env)
        }
        h.sseSend(m.ID, b)
    }
}
```
**依赖链:** `DB.GetChatMembers $\rightarrow$ 遍历成员 $\rightarrow$ skip exceptUser $\rightarrow$ snapshotForUser $\rightarrow$ c.queue $\rightarrow$ h.sseSend`

---

## 4. 广播事件实现 (Event Implementations)
所有广播函数均通过 `sendToUser` 或 `sendToChat` 实现。

| 函数 | OpCode | 实现逻辑 |
|---|---|---|
| `BroadcastMessageCreate` | `OpMessageCreate` | `sendToChat(m.ChatID, envelope(OpMessageCreate, m), "")` |
| `BroadcastReaction` | `OpReactionAdd/Remove` | 根据 `added` 布尔值选择 Op $\rightarrow$ `sendToChat` $\rightarrow$ Payload: `{chat_id, message_id, emoji, user_id}` |
| `BroadcastUserUpdate` | `OpUserUpdate` | `envelope(OpUserUpdate, u)` $\rightarrow$ 遍历所有在线 Client $\rightarrow$ `c.queue` $\rightarrow$ `sendToAllSSE` |
| `BroadcastTyping` | `OpTyping` | `envelope(OpTyping, {chat_id, user_id, timestamp})` $\rightarrow$ `sendToChat` $\rightarrow$ 排除发送者本人 |
