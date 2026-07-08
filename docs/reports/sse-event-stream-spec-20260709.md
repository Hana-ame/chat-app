# SSE 事件流规范

> 原始来源：
> - `server/internal/handlers/sse.go`
> - `server/internal/ws/hub.go`

---

## 一、概述

SSE（Server-Sent Events）提供单向实时事件推送，用于不支持 WebSocket 的客户端（如移动端、部分浏览器）。通过 `GET /api/events` 建立持久连接。

**依赖组件：**

```go
type Hub struct {
    sseClients map[string][]chan []byte  // userID → SSE channels
    mu         sync.RWMutex
}
```

---

## 二、原始代码

### Handler 入口

**文件:** `server/internal/handlers/sse.go`

```go
func (s *Server) SSE(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	if tok == "" {
		tok = r.URL.Query().Get("access_token")
	}
	if tok == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}
	claims, err := s.Auth.ParseAccessToken(tok)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	userID := claims.UserID
	user, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		return
	}

	chats, _ := s.DB.ListUserChats(r.Context(), userID)
	ready, _ := json.Marshal(map[string]any{
		"user": user, "chats": chats,
		"online_user_ids": s.Hub.OnlineUserIDs(),
	})
	fmt.Fprintf(w, "id: 0\nevent: ready\ndata: %s\n\n", ready)
	flusher.Flush()

	ch := make(chan []byte, 64)
	s.Hub.SSERegister(userID, ch)
	defer s.Hub.SSEUnregister(userID)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

### Hub 注册/注销/发送

**文件:** `server/internal/ws/hub.go`

```go
// sseClients: map[userID][]chan []byte
var sseClients map[string][]chan []byte

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
```

---

## 三、依赖链与数据流

```
客户端请求 GET /api/events?access_token=<JWT>
  │
  ├─ bearerToken(r)
  │   └─ 失败 → 401
  │
  ├─ Auth.ParseAccessToken(tok)
  │   └─ 失败 → 401
  │
  ├─ w.(http.Flusher)
  │   └─ 失败 → 500
  │
  ├─ 设置 SSE headers
  │
  ├─ DB.GetUserByID + DB.ListUserChats + Hub.OnlineUserIDs
  │   └─ 发送 ready 事件（id: 0, event: ready）
  │
  ├─ Hub.SSERegister(userID, ch)
  │
  ├─ for-select 循环：
  │   ├─ r.Context().Done() → 客户端断开 → Hub.SSEUnregister → return
  │   └─ ch ← data → fmt.Fprintf(w, "data: %s\n\n") → Flush
  │
  └─ defer Hub.SSEUnregister(userID)
```

**广播触发（`sseSend` 被调用的位置）：**

| 触发事件 | 调用方 | 数据内容 |
|----------|--------|----------|
| 新消息 | `Hub.BroadcastMessageCreate` | `{"op":"message_create","data":{...}}` |
| 消息编辑 | `Hub.BroadcastMessageUpdate` | `{"op":"message_update","data":{...}}` |
| 消息删除 | `Hub.BroadcastMessageDelete` | `{"op":"message_delete","data":{...}}` |
| 反应添加 | `Hub.BroadcastReaction`(added=true) | `{"op":"reaction_add","data":{...}}` |
| 反应移除 | `Hub.BroadcastReaction`(added=false) | `{"op":"reaction_remove","data":{...}}` |
| 聊天创建 | `Hub.BroadcastChatCreated` | `{"op":"chat_create","data":{...}}` |
| 聊天更新 | `Hub.BroadcastChatUpdated` | `{"op":"chat_update","data":{...}}` |
| 聊天删除 | `Hub.BroadcastChatDeleted` | `{"op":"chat_delete","data":{...}}` |
| 用户更新 | `Hub.BroadcastUserUpdate` | `{"op":"user_update","data":{...}}` |
| 在线状态 | `Hub.BroadcastPresence` | `{"op":"presence_update","data":{...}}` |
| 输入状态 | `Hub.BroadcastTyping` | `{"op":"typing","data":{...}}` |

---

## 四、条件分支

| 条件 | 行为 | 响应 |
|------|------|------|
| `bearerToken(r)` 为空且 URL 无 `access_token` | 拒绝 | `401 {"error":"unauthorized","message":"missing token"}` |
| `Auth.ParseAccessToken` 失败 | 拒绝 | `401 {"error":"unauthorized","message":"invalid token"}` |
| `w.(http.Flusher)` 不支持 | 拒绝 | `500 "SSE not supported"` |
| `DB.GetUserByID` 失败 | 静默关闭 | 无响应（连接关闭） |
| `r.Context().Done()`（客户端断开） | 注销 + 退出 | — |
| `ch` 关闭（`ok == false`） | 退出 | — |

---

## 五、事件格式

### 初始事件（`ready`）

```http
id: 0
event: ready
data: {"user":{...},"chats":[...],"online_user_ids":["uuid1","uuid2",...]}

```

### 广播事件

```http
data: {"op":"message_create","data":{"message_id":"...","chat_id":"...","content":"..."}}

```

### 心跳/保活

SSE 本身内置 TCP keepalive。如需应用层心跳，可通过 `BroadcastTyping` 或定时发送空注释行：

```http
: heartbeat

```