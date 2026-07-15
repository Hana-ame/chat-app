# Chat App: 7 Days From Zero to Production — PPT Outline (15 Slides)

> 30 轮迭代 · 318 commits · 80+ bugs fixed

---

## Slide 1 — 封面

**Chat App 开发纪实：7 天从零到生产级**

Go + React 全栈聊天应用 · 2026-07-10 → 07-16

---

## Slide 2 — 技术栈与架构概览

[插图建议: 系统架构框图 — React → API → Service → DB ↔ SQLite, WS ↔ Hub]

| 层 | 选型 |
|---|---|
| 后端 | Go + chi + SQLite |
| 实时 | WebSocket (主) / SSE / Polling (降级) |
| 前端 | React + Vite + Zustand |
| 质量 | go vet + JSDoc + testify |

**开发策略**: 先 Mock 后端 → 快速验证前端 → 再对接真实后端

---

## Slide 3 — 时间线总览

[插图建议: 甘特图 — 4 个阶段横跨 7 天]

```
Day 1  ██████  Phase 1: Mock 驱动前端迭代（1-9 轮）
Day 2  ██████  Phase 2: 撞上真实后端的墙（10-16 轮）
Day 3-4 ██████ Phase 3: 架构重构铺路（11-20 轮）
Day 5-7 ██████ Phase 4: 测试覆盖 + 上锁收尾（21-30 轮）
```

---

## Slide 4 — Act 1: 先飞起来 — Mock 策略

**问题**: 前端开发必须等后端就绪？不。

**方案**: MITM 模式 Mock
- 拦截 API 调用，模拟完整数据层
- 前 9 轮全部在前端完成
- 3 小时修复 11 个 Bug

**经典案例 — AI 消息消失**:
```
用户发消息 → Mock写入数据层 + onMessageCreate → 即时可见
AI 回复 → store流式逐字 (streaming=true) → 数据层持久化
         → 轮询从数据层读 → 不丢失
```

[插图建议: 消息流箭头图: 用户→Mock→Store(即时)/DataLayer(持久)←轮询]

---

## Slide 5 — Act 1: Mock 交付了什么

**7 天前 9 轮产出**:
- 消息流式发送 + AI Bot 逐字回复
- 附件上传、Reaction、搜索、排序
- 7 个 UI 组件、自动滚动、输入框伸缩
- 红点、Pinned、头像上传

**代价**: Mock 多用户支持拖到第 7 轮才修；排序 comparator `undefined` 陷阱被 `!!` 解决

---

## Slide 6 — Act 2: 撞墙 — 真实后端对接

[插图建议: 两张图对比 — Mock 数据 vs 后端实际返回的字段差异]

**公开测试 → 全线崩溃**:

| 问题 | 根因 | 影响 |
|------|------|------|
| `member_count=0` | 字段名不一致 | 群组大小错误 |
| Leave 变成 Delete | API 路径混淆 | 解散代替退出 |
| 头像、搜索、404 | 16 项 API 差异 | 功能全挂 |

**修复**: 逐项对齐 16 个 API 端点，统一字段命名和状态码

---

## Slide 7 — Act 2: 架构摇摆中学习

**Members 存储方案 — 3 次尝试**:
- Store map `membersByChatId` → 轮询覆盖 → 弃用
- 组件 `useEffect` + 本地 state → 最终方案
- 决策依据: 轮询已含完整 members，不需要额外 store

**Reaction `me` 字段 — 从建到拆**:
1. Handler `enrichReactions` N+1 解析 → WS 广播无效
2. 专用端点 `GET /:id/reactions` → 更干净的 API 边界

[插图建议: 两列对比 — "做了又改"的决策树，标注最终选择]

---

## Slide 8 — Act 2 小结: 教训

**"先快后稳"的代价**:
- Hover 菜单改 4 次才定案
- Pinned API 全量改名（路径+前端+6 个文档）
- Members 存储凌晨加、午前删

**结论**: Mock 可以快跑，但 API 契约文档应在第一天写好

---

## Slide 9 — Act 3: 铺路 — Service 层重构

[插图建议: 架构对比图 — 左: Handler→DB (扁平)，右: Handler→Service→DB+Hub (分层)]

**问题**: Handlers 直调 `s.DB.*`，权限检查、广播、验证重复

**方案**: 提取 `server/internal/service/` 包

```
之前: Handler → DB
之后: Handler → Service → DB
                      ↕
                   Hub 广播
```

**6 个新文件**: authz / chat / message / member / reaction / errors

---

## Slide 10 — Act 3: Service 层设计要点

**4 个关键设计**:

| 设计 | 效果 |
|------|------|
| Sentinel 错误 | `mapServiceError` 消除 HTTP 硬编码 |
| AuthZ 解耦 | `MustBeMember` 被 4 个 Service 共用 |
| 广播集中 | Service 层内调 Hub，handler 不复用 |
| WithTx 占位 | 预备未来跨表事务 |

**结果**: Handler 减半（`chat.go` -179/+86, `messages.go` -199/+88）

---

## Slide 11 — Act 3: 实时连接重构

[插图建议: 状态机图 — IDLE↔CONNECTING↔CONNECTED↔DISCONNECTING]

**问题**: Store 中 `connectWS/SSE/Polling/disconnect` 混乱

**方案**: RealtimeCoordinator 状态机 + 4 个 Transport

```
状态守卫锁 → 防止双通道
_closeGuard → 手动断开阻止重连
自动重连   → transport onClose → 3s 后重连
```

**Mock 解耦**: `MOCKABLE[]` → `Proxy(realApi, { get })`，新增 API 只需一行

**删除**: `dev/mock-ws.js`（死代码）

---

## Slide 12 — Act 3: 从零到 92.6% 测试覆盖

[插图建议: 折线图 — 轮次 vs 覆盖率，从 0% 到 92.6%]

**第 1-23 轮**: 零测试，全靠手工验证

**第 24 轮开始**: 系统化补充

| 轮次 | 包 | 覆盖率 |
|------|----|--------|
| 24 | config, service, ws | 起步 |
| 30 | config 100%, service 92.6%, db 81.6% | ✅ |

**DB 拆分**: `chats.go` → 5 个文件（chats/messages/members/reactions/tokens）

**Migration 陷阱**: `V001` ASCII 排序在 `init.sql` 前 → 改名 `000__init.sql`

---

## Slide 13 — 数据总览

**交付了什么**:

| 功能 | 状态 |
|------|------|
| 实时消息 + AI Bot | ✅ |
| 群组管理 + 角色权限 | ✅ |
| Reaction + 附件 | ✅ |
| 搜索 + 公告 | ✅ |
| 注册/登录 + JWT | ✅ |
| 速率限制 + CSP | ✅ |

| 指标 | 数值 |
|------|------|
| 开发周期 | 7 天 |
| 迭代轮次 | 30 |
| Git 提交 | 318 |
| 修复 Bug | ~80+ |
| 前端 Bundle | 316 KB |
| 后端测试 | 14 包 |

---

## Slide 14 — 经验与反思

**做得好的 ✅**
1. Mock 先行 → 前端迭代不受后端阻塞
2. Service 层提取 → handler 职责单一
3. 测试覆盖 0→92.6% → 重构的安全垫
4. MITM Mock 模式 → 解决轮询覆盖根本问题

**可以更好的 🔧**
1. API 契约应在第 1 天对齐，而不是第 2 天
2. Store 层方案决策前应验证"轮询怎么合并"
3. "做了又改"的 30% 工作可通过设计文档避免
4. Mock 多用户支持是 Day 1 就该做的事

---

## Slide 15 — 总结

**7 天完整生命周期**

```
原型 ── 撞墙 ── 重构 ── 上锁
  ↓       ↓       ↓       ↓
快速  暴露问题  夯实架构  守住质量
验证         ↘       ↙
          弯路中学习
```

**一句话**: 先跑通，再重构，最后上锁——但别等到第 7 天才开始写测试。
