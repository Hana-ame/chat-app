# 后端 Lock 分析报告

审查范围：`server/internal/`（handlers, ws, auth, testutil）  
审查日期：2026-07-08

---

## 1. Lock 全面清单

| 编号 | 文件 | 变量名 | 类型 | 保护对象 | 获取/释放模式 |
|------|------|--------|------|----------|---------------|
| L1 | `ws/client.go:19` | `c.mu` | `sync.RWMutex` | Client.subs map, closed 标志 | `RLock` / `Lock` 混用 |
| L2 | `ws/client.go:22` | `c.closeOnce` | `sync.Once` | 确保 conn.Close 只执行一次 | `Do()` |
| L3 | `ws/hub.go:43` | `h.mu` | `sync.RWMutex` | Hub.clients, hub.sseClients maps | `RLock` / `Lock` 混用 |
| L4 | `handlers/handler.go:31` | `s.refreshMu` | `sync.Mutex` | refresh token 旋转（读取+删除） | `Lock` / `Unlock` |
| L5 | `handlers/auth.go:104-128` | `s.Auth.lock()` | `func(){}`（空操作） | 原意为保护 JWT 签名（未实现） | 空函数，无实际效果 |

### 测试用途

| 编号 | 文件 | 变量名 | 类型 | 用途 |
|------|------|--------|------|------|
| T1 | `testutil/auth_flow_test.go:218` | `mu` | `sync.Mutex` | 统计并发 refresh 测试结果 |

---

## 2. 逐锁分析

### L1 — `ws/client.go: mu sync.RWMutex`

| 字段 | 内容 |
|------|------|
| **保护** | `subs map[string]struct{}` 和 `closed bool` |
| **方法** | `subscribed()` — RLock, `subscribe()` — Lock, `unsubscribe()` — Lock, `queue()` — RLock, `close()` — Lock |

**使用模式：**

```
subscribed()    RLock → RUnlock
subscribe()     Lock  → Unlock
unsubscribe()   Lock  → Unlock
queue()         RLock → (check closed) → RUnlock → (send via select)
close()         closeOnce.Do → Lock → Unlock → WriteControl → conn.Close
```

**潜在问题：** `queue()` 中先 RLock 读 `closed`，释放后再非阻塞发送到 chan。若 chan 满直接 `close()`，但此时其他 goroutine 可能已通过 `closed` 检查。属于**良性竞态**，可接受。

### L2 — `ws/client.go: closeOnce sync.Once`

| 字段 | 内容 |
|------|------|
| **保护** | `conn.WriteControl` + `conn.Close` 只执行一次 |
| **调用路径** | `readPump()` 退出时、`writePump()` 写入失败时、`queue()` 满时 |

**安全：** `sync.Once` 确保多个 goroutine 同时触发 close 不会重复关闭连接。

### L3 — `ws/hub.go: mu sync.RWMutex`

| 字段 | 内容 |
|------|------|
| **保护** | `clients map[string]map[*Client]struct{}` 和 `sseClients map[string][]chan []byte` |
| **方法数** | 12 个方法中使用（register, unregister, ClientCount, Online, OnlineUserIDs, snapshotForUser, sendToChat, BroadcastUserUpdate, sendToAllSSE, broadcastPresence, SSERegister, SSEUnregister, sseSend） |

**关键路径分析：**

```
register(h.mu.Lock):
   1. 添加 client 到 clients map
   2. 如果是首次上线：
      → h.db.UpdateUserStatus(...)  ← DB 操作在锁内！
      → go h.broadcastPresence(...)

broadcastPresence(h.mu.RLock):
   1. 遍历所有 clients（RLock）
   2. 对每个 client 调用 c.queue()
   3. 之后 sseSend 再次读 sseClients
```

**风险：**
- `register()` 在锁内执行 `db.UpdateUserStatus`（可能慢），会导致并发注册/注销被阻塞
- `sendToChat()` 中先 RLock 读 snapshot，释放后对每个 client 调用 `c.queue()`（持有 client.mu.RLock）。若 snapshot 中途 client 已关闭，`queue` 会跳过，安全但不影响锁

### L4 — `handlers/auth.go: refreshMu sync.Mutex`

| 字段 | 内容 |
|------|------|
| **位置** | `Server.refreshMu` |
| **保护** | Refresh 流程的 **find → check expiration → delete → issue** |
| **使用** | `Refresh` handler 中加锁 |

```
Refresh():
   refreshMu.Lock()
   hash := HashRefreshToken(c.Value)
   rt, err := FindRefreshToken(hash)      ← DB 查询
   if expired:
       DeleteRefreshToken(rt.ID)          ← DB 删除
       refreshMu.Unlock()
   err := DeleteRefreshToken(rt.ID)       ← DB 删除
   refreshMu.Unlock()
   issueSession(...)                      ← 签发新 token，不持锁
```

**正确性：** 锁覆盖了整个 find–delete–issue 窗口，保证同一 refresh token 只能被成功 refresh 一次。其他并发请求会 find 失败返回 401。✅

### L5 — `auth/auth.go: lock() / unlock()` 空函数

```go
func (s *Service) lock()   {}
func (s *Service) unlock() {}
```

**问题：** 这是占位代码，现无实际保护。`IssueAccessToken()` 调用它们但不做任何事。在多 goroutine 无共享 mutable 状态下无害，但容易误导未来维护者。

---

## 3. 锁的使用模式总结

| 模式 | 出现次数 | 示例 |
|------|----------|------|
| **RWMutex — 读多写少** | 2 | `ws/client.go`, `ws/hub.go` |
| **Mutex — 临界区互斥** | 1 | `handlers/auth.go` refreshMu |
| **sync.Once — 只执行一次** | 1 | `ws/client.go` closeOnce |
| **空占位锁** | 1 | `auth/auth.go` lock/unlock |
| **测试锁** | 1 | `auth_flow_test.go` |

---

## 4. 锁 Duration 统计

| 锁 | 持有锁时长 | 持锁时是否执行 I/O | 优化建议 |
|----|-----------|---------------------|----------|
| `ws/hub.mu` (register) | ~1-10ms | **是**（db.UpdateUserStatus） | 释放锁后再调用 DB |
| `ws/hub.mu` (unregister) | ~1-10ms | **是**（db.UpdateUserStatus） | 释放锁后再调用 DB |
| `ws/hub.mu` (其他读操作) | < 1µs | 否 | 无需优化 |
| `ws/client.mu` | < 1µs | 否 | 无需优化 |
| `handlers.refreshMu` | ~1-5ms | **是**（DB 查/删各一次） | 已是最小窗口 ✅ |
| `auth.authSvc.lock` | 0ms | 否 | 空实现，可删除 |

---

## 5. 锁等待图

```
┌─────────────────────────────┐
│  handlers/refreshMu (写)     │ ← 串行化 refresh 流程，保证 at-most-once
│   └─ DB FindRefreshToken     │
│   └─ DB DeleteRefreshToken   │
│   └─ unlock → issueSession   │
└─────────────────────────────┘

┌─────────────────────────────┐
│  ws/hub.mu (RW)              │ ← 多读单写
│   ├─ register (写 + DB)      │   ← 可在锁外执行 DB
│   ├─ unregister (写 + DB)    │   ← 同上
│   ├─ broadcastPresence (读)   │
│   ├─ sendToChat (读)         │
│   └─ sseSend (读)            │
└─────────────────────────────┘

┌─────────────────────────────┐
│  ws/client.mu (RW)           │ ← 多读单写
│   ├─ subscribe (写)          │
│   ├─ unsubscribe (写)        │
│   ├─ subscribed (读)         │
│   └─ queue (读 + 非阻塞发送)  │
└─────────────────────────────┘
```

---

## 6. 结论与建议

| 优先级 | 问题 | 建议 |
|--------|------|------|
| **中** | `hub.register/unregister` 在锁内执行 DB | 先释放锁再 `UpdateUserStatus` |
| **低** | `auth/lock.go` 空锁占位 | 移除空函数或补充实现 |
| **低** | `hub.BroadcastUserUpdate` 连续两次 RLock/RUnlock | 合并为一次区间 |
| 无需处理 | `client.mu` + `closeOnce` | 安全 |
| 无需处理 | `refreshMu` | 安全 |
