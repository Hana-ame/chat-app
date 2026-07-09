# 前端对齐与质量增强报告 (Frontend Alignment Report)

审查日期：2026-07-09  
覆盖范围：`client/src/` + `client/tests/` + `.github/workflows/`

---

## 1. 核心变更：Pin 功能语义对齐

经过审计，发现前端原有的 "Pin" 逻辑（对话置顶）与后端实际实现的 "Pinned Message"（聊天室公告）完全脱节。本次更新将前端逻辑彻底对齐至后端 Spec。

### 1.1 功能变更对照表

| 维度 | 旧逻辑 (Broken) | 新逻辑 (Aligned) | 状态 |
| :--- | :--- | :--- | :--- |
| **功能定义** | 将对话置顶到列表顶部 | 在聊天室顶部设置文本公告 | ✅ 修正 |
| **API 路径** | `POST /api/chats/{id}/unpin` | `DELETE /api/chats/{id}/pin` | ✅ 修正 |
| **API 参数** | 无 Body | 发送 `{"content": "..."}` | ✅ 修正 |
| **权限控制** | 客户端假定所有人可 Pin | 仅 Owner 可见管理界面 $\rightarrow$ 后端校验 | ✅ 修正 |
| **数据结构** | `pinnedMessages: { chatId: [Msg] }` | `pinnedMessage: { chatId: string }` | ✅ 修正 |

### 1.2 UI/UX 重构
- **对话列表 (`ChatListItem`)**：移除了不可用的“对话置顶”按钮，避免用户触发 400/404 错误。
- **消息项 (`MessageItem`)**：移除了单条消息的 Pin 按钮，改为由 Owner 在聊天室顶部统一管理公告。
- **聊天视图 (`ChatView`)**：
    - 引入 **Notice Board (公告栏)** 模式。
    - 为 Owner 用户增加“设置公告” $\rightarrow$ “编辑公告” $\rightarrow$ “清除公告” 的完整管理链路。
    - 支持公告内容的 Markdown 渲染（通过 `renderContent`）。

---

## 2. 鲁棒性增强：速率限制处理

针对后端引入的 `httprate` 限制，前端增加了对应的错误拦截机制。

- **拦截层**：在 `api/client.js` 的 `request` 函数中捕获 `res.status === 429`。
- **错误抛出**：抛出带有 `error: 'too_many_requests'` 标记的异常。
- **用户反馈**：在 `ChatView` 等关键调用点捕获该异常，通过 `alert` (或 Toast) 告知用户“请求过于频繁，请稍后再试”。

---

## 3. 测试与自动化 (QA)

### 3.1 E2E 测试增强
在 `client/tests/e2e.spec.js` 中新增了针对置顶公告的完整链路测试：
- **Owner 权限验证**：验证 `+ Set Notice` 按钮的可见性。
- **创建流程**：输入文本 $\rightarrow$ 保存 $\rightarrow$ 验证公告栏出现。
- **更新流程**：点击 Edit $\rightarrow$ 修改文本 $\rightarrow$ 验证内容更新。
- **删除流程**：点击 Clear $\rightarrow$ 验证公告栏消失。

### 3.2 CI/CD 流水线
新建 `.github/workflows/frontend-ci.yml`，实现自动化回归测试：
1. **环境构建**：Ubuntu-latest $\rightarrow$ Node.js 20 $\rightarrow$ npm install。
2. **依赖安装**：安装 Playwright 浏览器及其系统依赖。
3. **全链路运行**：
    - 构建前端项目。
    - 在后台启动 Go 后端服务。
    - 执行 `npm test` 运行所有 E2E 测试用例。
4. **触发条件**：每次 `push` 或 `pull_request` 到 `main` 分支。

---

## 4. 结论

前端已完成与后端 API Spec 的完全对齐。移除了所有基于错误假设的“置顶”功能，实现了符合后端逻辑的“公告栏”功能。通过引入 429 错误处理和 GitHub Actions 自动化测试，显著提升了系统的稳定性与可维护性。
