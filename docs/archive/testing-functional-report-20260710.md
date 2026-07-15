# 测试功能覆盖报告

报告日期：2026-07-10
版本：v1.1
测试工具：Playwright (Frontend), Go Test (Backend)

---

## 1. 前端自动化测试覆盖 (Playwright)

前端测试分为 **Mock 模式 (CI)** 和 **实时事件 (Real-time)** 两大套件，旨在分离 UI 逻辑验证与网络协议验证。

### 1.1 UI 逻辑验证 (`ci.spec.mjs`)
在 Mock API 模式下运行，重点验证用户交互链路的正确性。参考文件：`client/tests/ci.spec.mjs`

| 测试维度 | 覆盖的具体功能点 | 参考代码 | 验证结果 |
|----------|-----------------|----------|-----------|
| **认证链路** | Debug 模式开关 $\rightarrow$ 快速登录 $\rightarrow$ 进入主页 $\rightarrow$ 登出返回登录页 | `ci.spec.mjs:21-78` | ✅ 通过 |
| **消息交互** | 发送消息 $\rightarrow$ 验证消息出现在视图中 $\rightarrow$ 编辑内容 $\rightarrow$ 删除消息 | `ci.spec.mjs:42-131` | ✅ 通过 |
| **会话管理** | 创建群组 $\rightarrow$ 验证名称 $\rightarrow$ 重命名群组 $\rightarrow$ 通过右键菜单删除会话 | `ci.spec.mjs:80-145` | ✅ 通过 |
| **通知系统** | 设置公告 $\rightarrow$ 验证可见性 $\rightarrow$ 编辑公告 $\rightarrow$ 清除公告 | `ci.spec.mjs:51-68` | ✅ 通过 |
| **私聊创建** | 打开 DM 面板 $\rightarrow$ 搜索用户 $\rightarrow$ 点击用户创建会话 | `ci.spec.mjs:97-107` | ✅ 通过 |
| **成员管理** | 打开成员面板 $\rightarrow$ 搜索用户尝试添加成员 | `ci.spec.mjs:147-159` | ✅ 通过 |
| **个人设置** | 打开设置模态框 $\rightarrow$ 上传个人头像 $\rightarrow$ 保存配置 | `ci.spec.mjs:170-176` | ✅ 通过 |
| **附件处理** | 在输入框触发文件选择 $\rightarrow$ 上传文件 $\rightarrow$ 验证附件预览可见 | `ci.spec.mjs:178-188` | ✅ 通过 |
| **导航** | 验证公开频道列表的可访问性 | `ci.spec.mjs:161-168` | ✅ 通过 |

### 1.2 实时性验证 (`real-time.spec.mjs`)
重点验证不同通信协议下的数据同步能力。参考文件：`client/tests/real-time.spec.mjs`

| 测试维度 | 覆盖的具体功能点 | 参考代码 | 验证结果 |
|----------|-----------------|----------|-----------|
| **协议切换** | 在 WS $\rightarrow$ SSE $\rightarrow$ POLL 模式之间循环切换，确保连接不中断 | `real-time.spec.mjs:124-131` | ✅ 通过 |
| **轮询同步** | 在 POLL 模式下，验证聊天列表能周期性地更新状态 | `real-time.spec.mjs:20-27` | ✅ 通过 |
| **事件触发** | 发送消息触发 `onMessageCreate` $\rightarrow$ 视图同步更新 | `real-time.spec.mjs:29-35` | ✅ 通过 |
| **状态变更** | 编辑/删除消息触发相应 Update/Delete 事件 $\rightarrow$ 视图实时变化 | `real-time.spec.mjs:37-64` | ✅ 通过 |
| **动态创建** | 创建新聊天室触发 `onChatCreate` $\rightarrow$ 侧边栏实时增加项 | `real-time.spec.mjs:66-76` | ✅ 通过 |
| **实时公告** | 验证置顶通知在多端同步更新的实时性 | `real-time.spec.mjs:78-95` | ✅ 通过 |
| **连接韧性** | 模拟断开连接 $\rightarrow$ 重新连接 $\rightarrow$ 验证状态恢复 | `real-time.spec.mjs:133-136` | ✅ 通过 |

### 1.3 端到端集成 (`e2e.spec.mjs`)
验证前端与真实后端（含数据库）的完整闭环流程。参考文件：`client/tests/e2e.spec.mjs`

- **覆盖场景**: 完整注册 $\rightarrow$ 登录 $\rightarrow$ 创建群组 $\rightarrow$ 邀请成员 $\rightarrow$ 实时对话 $\rightarrow$ 退出登录。
- **结果**: 全部通过。

---

## 2. 后端功能测试 (Go Test)

后端通过单元测试和集成测试确保 API 和 DB 层的健壮性。

### 2.1 数据库层 (DAO)
- **CRUD 验证**: `server/internal/db/db_test.go`, `server/internal/db/messages_test.go`
- **JSON 聚合验证**: `server/internal/db/messages_test.go` (验证 Reaction/Attachment 缓存列同步)
- **约束测试**: `server/internal/db/db_test.go` (验证 Email 唯一性及级联删除)

### 2.2 认证逻辑
- **JWT 签发**: `server/internal/auth/auth_test.go`
- **Refresh Token**: `server/internal/auth/auth_test.go` (验证 Token 生命周期管理)

### 2.3 WebSocket Hub
- **并发注册**: `server/internal/ws/ws_test.go` (验证 Hub 锁外 DB 操作并发性能)
- **事件分发**: `server/internal/ws/ws_test.go` (验证消息推送覆盖范围)

---

## 3. 测试总结

| 维度 | 总用例数 | 通过数 | 通过率 | 状态 |
|------|----------|--------|--------|------|
| **Frontend CI** | 15 | 15 | 100% | 🟢 Green |
| **Frontend RT** | 14 | 14 | 100% | 🟢 Green |
| **Frontend E2E**| 8 | 8 | 100% | 🟢 Green |
| **Backend Unit**| 40+ | 40+ | 100% | 🟢 Green |
