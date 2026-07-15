# 后端 Lock 分析报告

审查范围：`server/internal/`（handlers, ws, auth, testutil）  
审查日期：2026-07-08  
最后更新：2026-07-09（同步代码变更）

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

**已修复的关键路径：**

```
register(h.mu.Lock):
   1. 添加 client 到 clients map
   2. 如果是首次上线：
      → h.mu.Unlock()                    ← 先释放锁
      → h.db.UpdateUserStatus(...)       ← DB 操作在锁外 ✅
      → h.broadcastPresence(...)

broadcastPresence(h.mu.RLock):
   1. 遍历所有 clients（RLock）
   2. 对每个 client 调用 c.queue()
   3. 之后 sseSend 再次读 sseClients
```

`unregister()` 也采用了相同模式：锁内更新 map，释放后执行 DB 操作。

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

**已知竞态（已注释未修复）：** `issueSession` 在锁外创建新 refresh token，而 `Logout` 调用 `DeleteUserRefreshTokens` 也不持有 `refreshMu`。竞态窗口：Refresh 刚释放锁但尚未插入新 token 时，Logout 删除所有 token → 新 token 变成孤 token。该路径已添加注释说明，评估为低风险接受。若未来需要修复，应在 `Logout` 中也获取 `refreshMu`，并将 `CreateRefreshToken` 移入锁内。

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

| 锁 | 持有锁时长 | 持锁时是否执行 I/O | 状态 |
|----|-----------|---------------------|------|
| `ws/hub.mu` (register) | < 1µs | 否（DB 已移出锁外） | ✅ 已优化，2026-07-09 |
| `ws/hub.mu` (unregister) | < 1µs | 否（DB 已移出锁外） | ✅ 已优化，2026-07-09 |
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
│   ⚠ Logout 不在锁内 → 已注释 │
└─────────────────────────────┘

┌─────────────────────────────┐
│  ws/hub.mu (RW)              │ ← 多读单写
│   ├─ register (写)           │   ← DB 已移出锁外 ✅
│   ├─ unregister (写)         │   ← DB 已移出锁外 ✅
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

| 优先级 | 问题 | 状态 |
|--------|------|------|
| ~~中~~ | ~~`hub.register/unregister` 在锁内执行 DB~~ | ✅ 已修复（2026-07-09）：释放锁后再 `UpdateUserStatus` |
| **低** | `auth/lock.go` 空锁占位 | 未改 — 建议移除空函数或补充实现 |
| **低** | `hub.BroadcastUserUpdate` 连续两次 RLock/RUnlock | 未改 — 建议合并为一次区间 |
| **低** | `refreshMu` 未覆盖 Logout 路径 | 已添加注释（2026-07-09），评估为低风险接受 |
| 无需处理 | `client.mu` + `closeOnce` | 安全 |
| 无需处理 | `refreshMu` 覆盖 find–delete–issue | 安全 |
