# WS 客户端与网关规范

> 原始来源：
> - `server/internal/ws/client.go`
> - `server/internal/ws/gateway.go`

---

## 一、原始代码

### Gateway（升级入口）

**文件:** `ws/gateway.go`

```go
package ws

import (
	"encoding/json"
	"net/http"
	"os"
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

type Gateway struct {
	hub     *Hub
	db      *db.DB
	authSvc *auth.Service
}

func NewGateway(hub *Hub, database *db.DB, authSvc *auth.Service) *Gateway {
	return &Gateway{hub: hub, db: database, authSvc: authSvc}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if v := os.Getenv("WS_ENABLED"); v != "" && v != "true" {
		http.Error(w, "WebSocket is disabled", http.StatusForbidden)
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
	conn.SetReadLimit(1 << 16)

	c := &Client{
		hub: g.hub, conn: conn, userID: user.ID,
		send: make(chan Envelope, 64),
		subs: make(map[string]struct{}),
	}
	chats, _ := g.db.ListUserChats(r.Context(), user.ID)
	for _, ch := range chats {
		c.subscribe(ch.ID)
	}

	readyPayload, _ := json.Marshal(map[string]any{
		"user": user, "chats": chats,
		"online_user_ids": g.hub.OnlineUserIDs(),
	})
	c.send <- Envelope{Op: OpReady, Payload: readyPayload}
	g.hub.register(c)

	go c.writePump()
	go c.readPump()
}
```

### Client（连接管理）

**文件:** `ws/client.go`

```go
package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

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

func (c *Client) subscribed(chatID string) bool
func (c *Client) subscribe(chatID string)
func (c *Client) unsubscribe(chatID string)

func (c *Client) queue(env Envelope) {
	c.mu.RLock()
	if c.closed { c.mu.RUnlock(); return }
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
		c.conn.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(1000, "bye"),
			time.Now().Add(10*time.Second))
		c.conn.Close()
	})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(60*time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60*time.Second))
		return nil
	})
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil { return }
		var env Envelope
		json.Unmarshal(raw, &env)
		switch env.Op {
		case OpPing:
			c.queue(Envelope{Op: OpPong})
		case OpSubscribe:
			// 校验 chat member 后 subscribe
		case OpUnsubscribe:
			c.unsubscribe(p.ChatID)
		case OpTyping:
			// 校验 chat member 后 BroadcastTyping
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(50*time.Second)
	defer ticker.Stop()
	for {
		select {
		case env, ok := <-c.send:
			if !ok { return }
			c.conn.WriteJSON(env)
		case <-ticker.C:
			c.conn.WriteMessage(websocket.PingMessage, nil)
		}
	}
}
```

---

## 二、类型总表

### Gateway

| 字段 | 类型 | 说明 |
|------|------|------|
| hub | `*Hub` | Hub 引用，用于注册/注销客户端 |
| db | `*db.DB` | 数据库引用，用于认证和订阅校验 |
| authSvc | `*auth.Service` | JWT 认证服务 |

### Client

| 字段 | 类型 | 说明 |
|------|------|------|
| hub | `*Hub` | Hub 引用 |
| conn | `*websocket.Conn` | WebSocket 连接 |
| userID | `string` | 已认证用户 UUID |
| send | `chan Envelope` | 写队列（buffer=64） |
| subs | `map[string]struct{}` | 订阅的 chatID 集合 |
| closed | `bool` | 连接已关闭标记 |
| closeOnce | `sync.Once` | 确保仅关闭一次 |

---

## 三、函数总表

### Gateway

| 函数 | 签名 | 说明 |
|------|------|------|
| `NewGateway` | `(hub, db, authSvc) *Gateway` | 创建网关 |
| `ServeHTTP` | `(w, r)` | WS 升级入口（HTTP handler） |

### Client

| 函数 | 签名 | 说明 |
|------|------|------|
| `subscribed` | `(chatID) bool` | 检查是否已订阅 |
| `subscribe` | `(chatID)` | 添加订阅 |
| `unsubscribe` | `(chatID)` | 移除订阅 |
| `queue` | `(env Envelope)` | 入队写消息（队列满时关闭连接） |
| `close` | `()` | 优雅关闭（write control + close） |
| `readPump` | `()` | 读循环（处理 ping/subscribe/typing） |
| `writePump` | `()` | 写循环（消息出队 + ping 保活） |

---

## 四、连接生命周期

```
GET /ws?access_token=<JWT>
  │
  ├─ WS_ENABLED == false → 403
  ├─ 无 token → 401
  ├─ JWT 无效 → 401
  ├─ 用户不存在 → 401
  │
  ├─ upgrader.Upgrade → WebSocket
  │
  ├─ 创建 Client{hub, conn, userID, send, subs}
  ├─ ListUserChats → 自动订阅所有聊天
  ├─ 发送 ready 事件
  ├─ hub.register(c)
  │
  ├─ goroutine: writePump
  │   ├─ 接收 c.send 消息 → WriteJSON
  │   └─ 每 50s → PingMessage
  │
  └─ goroutine: readPump
      ├─ PongHandler → 重置读超时 60s
      ├─ 接收客户端消息
      │   ├─ ping → 回复 pong
      │   ├─ subscribe → 校验 member → subscribe
      │   ├─ unsubscribe → unsubscribe
      │   └─ typing → 校验 member → BroadcastTyping
      └─ 断开 → hub.unregister + close
```

---

## 五、条件分支

| 条件 | 行为 | 响应 |
|------|------|------|
| `WS_ENABLED` 为空或不为 `"true"` | 拒绝 | `403 WebSocket is disabled` |
| 无 `access_token` | 拒绝 | `401 missing access_token` |
| `Auth.ParseAccessToken` 失败 | 拒绝 | `401 invalid token` |
| `DB.GetUserByID` 失败 | 拒绝 | `401 user gone` |
| `upgrader.Upgrade` 失败 | 拒绝 | — |
| `readPump` 读错误 | 注销 + 关闭 | — |
| `send` 队列满 | 关闭连接（backpressure） | — |
| 客户端断开 | `r.Context().Done()` → 退出 | — |

---

## 六、常量总表

| 常量 | 值 | 说明 |
|------|-----|------|
| `writeWait` | `10s` | 写入超时 |
| `pongWait` | `60s` | 等待 pong 超时 |
| `pingPeriod` | `50s` | ping 发送间隔 |
| `maxMessageSize` | `65536` | 最大消息字节数 |
| `sendQueueSize` | `64` | 写队列长度 |

---

## 七、约束汇总

| 约束 | 说明 |
|------|------|
| 传输 | `gorilla/websocket` |
| 认证 | 仅支持 `access_token` URL 参数（不支持 Header/Cookie） |
| 订阅 | 连接时自动订阅用户所有聊天 |
| 读超时 | 60s 无消息后断开 |
| Ping | 服务端每 50s 发 ping，客户端需回复 pong |
| 背压 | `send` channel buffer=64，满则关闭连接 |
| 并发 | `sync.Once` 确保 close 仅一次，`sync.RWMutex` 保护 subs |
| 禁用开关 | 环境变量 `WS_ENABLED=false` 可完全禁用 WS |", "filePath": "/mnt/d/WorkPlace/chat-app/docs/reports/ws-client-gateway-spec-20260709.md"