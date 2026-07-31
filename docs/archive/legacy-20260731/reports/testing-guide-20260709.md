# 测试指南

## 1. 测试前准备

```bash
# 启动后端服务（真实 API 模式）
cd server
go run ./cmd/server/ &

# 启动前端（开发模式，自动打开浏览器）
cd client
npm run dev
```

---

## 2. 真实 API 测试流程

### 2.1 注册 -> 登录 -> 进入主界面

1. 打开 `http://localhost:5173` → 自动跳转到 `/login`
2. 点 **"Need an account? Register"** 链接 → 进入 `/register`
3. 填写：
   - Email: `test1@test.com`
   - Username: `TestUser`
   - Password: `password123`
4. 点 **"Continue"**
5. ✅ 预期：自动跳转到主界面，左侧显示聊天列表，右侧显示空状态 "No messages yet. Start the conversation!"

### 2.2 搜索用户 + 创建群聊

1. 点左侧聊天列表底部的 **"+"** 或 **"Create Group"** 按钮
2. 填写群组名：`测试群`
3. ✅ 预期：群组创建成功，自动进入该群聊，顶部显示群名

### 2.3 发送消息

1. 在底部输入框输入 `Hello World!`，点 **"Send"**
2. ✅ 预期：消息出现在聊天区域，左侧聊天列表显示最后消息预览

### 2.4 消息操作

| 操作 | 步骤 | 预期 |
| :--- | :--- | :--- |
| **编辑** | 鼠标移到自己的消息上 → 点 **"Edit"** → 修改内容 → 点 **"Save"** | 消息内容更新，显示 `(edited)` |
| **删除** | 点 **"Delete"** → 确认 | 消息变为 `(message deleted)` |
| **React** | 点 **"😀"** → 选择一个 emoji | emoji 出现在消息下方 |

### 2.5 公告栏（仅 Owner）

1. 创建群聊后，如果是 Owner（创建者）
2. ✅ 预期：公告栏区域出现 **"+ Set Notice"** 按钮
3. 点 **"+ Set Notice"** → 输入 `这是公告` → 点 **"Save"**
4. ✅ 预期：公告栏显示 "📌 Notice: 这是公告"
5. 点 **"Edit"** → 修改 → **"Save"** → 公告内容更新
6. 点 **"Clear"** → 公告栏消失

### 2.6 登出

1. 点左下角用户名旁边的齿轮/登出按钮
2. ✅ 预期：跳转到 `/login`，localStorage 中的认证数据被清除

---

## 3. Mock API 测试流程

### 3.1 进入 Mock 模式

1. 打开 `http://localhost:5173`
2. 在登录页，勾选 **"Debug mode"**
3. ✅ 预期：出现 **"⚡ Quick Enter (mock)"** 按钮
4. 点该按钮

### 3.2 Mock 模式验证

| 检查项 | 步骤 | 预期 |
| :--- | :--- | :--- |
| 进入 | 点 Quick Enter 后 | 自动跳转到主界面，左侧有 10 个 mock 聊天 |
| 消息列表 | 点击任意聊天 | 右侧显示 50 条 mock 消息，包含 @提及、Markdown 格式文本 |
| 发送消息 | 输入内容 → 点 Send | 自己发的消息出现 + AI Bot 自动回复（流式打字效果）|
| 公告栏 | 勾选 Debug 进入后 | Owner 身份，可见 **"+ Set Notice"** |
| 公告设置 | 设置 → 编辑 → 清除 | 公告内容即时更新 |
| 返回真实 | 登出 → 重新登录 | API 切回真实模式 |

### 3.3 刷新页面

1. 在 Mock 模式下刷新浏览器
2. ✅ 预期：仍然停留在 Mock 模式（`accessToken === 'mock-token'` 持久化）
3. 数据仍然存在（mock 数据在内存中，刷新后需要重新点击聊天加载）

---

## 4. E2E 测试

```bash
# 确保后端运行中
cd client
npx playwright test
```

✅ 预期：8 个测试全部通过（包括公告栏测试）

---

## 5. CI/CD 自动化测试

### 5.1 CI 流水线结构

```
Push / PR → main
  ├── Job 1: mock-test（纯前端 Mock API，~1min）
  │   无需后端，8 个 CI 测试
  │   └── 通过后触发 Job 2
  └── Job 2: full-e2e（全链路，需后端，~3min）
       8 个完整 E2E 测试
```

### 5.2 本地运行 CI 测试

```bash
# 1. Mock API 测试（不依赖后端，最快，推荐本地开发用）
cd client
npm test               # 等价于 playwright test tests/ci.spec.js

# 2. 全链路 E2E 测试（需要后端运行中）
npm run test:full      # playwright test tests/e2e.spec.js

# 3. 跑全部测试
npm run test:all       # playwright test
```

### 5.3 测试文件说明

| 文件 | 依赖 | 适用场景 | 测试数 |
| :--- | :--- | :--- | :--- |
| `tests/ci.spec.js` | 仅前端 `vite dev` | CI Job 1 / 本地开发快速验证 | 9 个 |
| `tests/e2e.spec.js` | 前端 + 后端 | CI Job 2 / 发版前全量验证 | 8 个 |

### 5.4 预期结果

```
✓ ci.spec.js: 9 passed (all green)
✓ e2e.spec.js: 8 passed (all green)
```

---

## 6. 常见问题排查

| 问题 | 可能原因 | 解决 |
| :--- | :--- | :--- |
| 登录后立即跳回登录页 | 服务端未启动或端口错误 | 确认 `go run ./cmd/server/` 成功运行 |
| Mock 模式点任何按钮都没反应 | Mock 未正确启用 | 检查控制台是否有报错，重新登出再 Quick Enter |
| 公告栏按钮不显示 | 成员数不足 3 人（真实模式）| 先通过 API 或注册多个账号添加成员 |
| 发送消息没反应 | 速率限制（30/min）| 等一会再试 |