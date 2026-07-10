# SSE 实现细节规范 (SSE Implementation Spec)

## 1. SSE Handler: 连接与流管理
### `SSE(w, r)`
**目的:** 建立单向事件流连接，发送初始化 `ready` 事件，并进入实时推送循环。

```go
func (s *Server) SSE(w http.ResponseWriter, r *http.Request) {
    tok := bearerToken(r)
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
**依赖链:** `bearerToken(r) $\rightarrow$ Auth.ParseAccessToken $\rightarrow$ w.(http.Flusher) $\rightarrow$ Set headers $\rightarrow$ DB.GetUserByID + DB.ListUserChats $\rightarrow$ Send "ready" event $\rightarrow$ Hub.SSERegister $\rightarrow$ Loop (select notify / channel data) $\rightarrow$ defer Hub.SSEUnregister`

**条件分支:**
- `tok == ""` $\rightarrow$ `401 Unauthorized`
- `Auth.ParseAccessToken` 失败 $\rightarrow$ `401 Unauthorized`
- `w.(http.Flusher)` 失败 $\rightarrow$ `500 SSE not supported`
- `r.Context().Done()` $\rightarrow$ 客户端断开 $\rightarrow$ 退出循环 $\rightarrow$ 执行 `defer SSEUnregister`
- `ch` 关闭 $\rightarrow$ 退出循环

---

## 2. Hub: SSE 通道管理
### `SSERegister(userID, ch)` & `SSEUnregister(userID)`
**目的:** 管理用户关联的 SSE 写入通道（支持同一用户多设备连接）。

```go
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
```
**逻辑流:** `Lock $\rightarrow$ 追加 ch 到 sseClients[userID] slice $\rightarrow$ Unlock` (注：注销时直接删除整个 userID 条目)。

### `sseSend(userID, data)`
**目的:** 向特定用户的所有活跃 SSE 连接非阻塞地推送数据。

```go
func (h *Hub) sseSend(userID string, data []byte) {
    h.mu.RLock()
    chs := h.sseClients[userID]
    h.mu.RUnlock()
    for _, ch := range chs {
        select {
        case ch <- data:
        default:
            // Channel full, drop message to avoid blocking hub
        }
    }
}
```
**依赖链:** `RLock $\rightarrow$ 获取 sseClients[userID] $\rightarrow$ RUnlock $\rightarrow$ 遍历 chs $\rightarrow$ non-blocking write (select default)`

---

## 3. 事件格式与数据流 (Data Flow)

### 3.1 初始就绪事件
- **格式:** `id: 0\nevent: ready\ndata: { "user": ..., "chats": ..., "online_user_ids": ... }\n\n`
- **触发点:** `SSE` handler 在写入 `http.StatusOK` 之后立即发送。

### 3.2 实时广播事件
- **格式:** `data: {"op": "...", "data": { ... }}\n\n`
- **数据来源:** 所有的 `Hub.sendToUser` 和 `Hub.sendToChat` 内部都会调用 `sseSend`。
- **同步逻辑:** 
  1. 业务触发 `BroadcastXXX` (如 `BroadcastMessageCreate`)。
  2. 调用 `sendToChat` $\rightarrow$ 遍历成员 $\rightarrow$ 对每个成员执行 `sseSend(m.ID, b)`。
  3. `sseSend` 将 `Envelope` 的 JSON 字节流写入该用户的 SSE channel。
  4. `SSE` handler 循环监听到 channel 数据 $\rightarrow$ 格式化为 `data: ...\n\n` $\rightarrow$ `Flush` 到 HTTP 响应流。

---

## 4. 约束与性能
- **缓冲区**: 每个 SSE 连接拥有 64 字节的 channel 缓冲区。
- **非阻塞**: `sseSend` 使用 `select default`，若客户端消费过慢导致缓冲区满，则直接丢弃该条消息，确保 Hub 主线程不被慢连接阻塞。
- **资源释放**: 依赖 `r.Context().Done()` 及时触发 `SSEUnregister`，防止内存泄漏。
