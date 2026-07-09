# CI/CD 流水线规范 (CI/CD Pipeline Spec)

> 原始来源：
> - `.github/workflows/ci.yml`
> - `.github/workflows/frontend-ci.yml`
> - `client/tests/ci.spec.mjs`
> - `client/tests/e2e.spec.mjs`
>
> 依赖：GitHub Actions Ubuntu 24.04 runner / Node 22 / Go 1.23 / Playwright 1.60

---

## 一、流水线架构总览

整个项目有两条独立的 GitHub Actions 流水线：

```
                     Push / PR → main/master
                              │
              ┌───────────────┴───────────────┐
              │                               │
         CI (ci.yml)                   Frontend CI (frontend-ci.yml)
      (全量 CI + 发布)               (前端专项测试)
              │                               │
    ┌─────────┼──────────┐          ┌─────────┴─────────┐
    │         │          │          │                   │
  go-test  frontend-  go-build   mock-test          full-e2e
            build       │     (15 tests ✅)    (needs: mock-test)
    │         │         │          │              (8 tests ✅)
    └─────────┴─────────┘          └───────────────────┘
              │                         
           release (main only)
              │
    GitHub Release (chatd 跨平台 + client-dist)
```

两条流水线的区别：

| 流水线 | 触发分支 | 包含 | 用途 |
|--------|---------|------|------|
| `CI` | main, master | go-test + frontend-build + go-build + release | 全量检查 + 发布 |
| `Frontend CI` | main | mock-test + full-e2e | 前端专项回归测试 |

---

## 二、CI 流水线 (ci.yml) 完整解析

### 2.1 文件位置

`.github/workflows/ci.yml`（119 行）

### 2.2 触发条件

```yaml
on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]
```

- push 到 main/master → 全量执行
- PR 到 main/master → 只跑 go-test + frontend-build（go-build 和 release 跳过，因为 `if: github.ref == 'refs/heads/main'`）

### 2.3 运行环境

```yaml
runs-on: ubuntu-latest
```

GitHub 托管的 Ubuntu 24.04 虚拟机。

**预装软件版本：**

| 工具 | 版本 |
|------|------|
| OS | Ubuntu 24.04.4 LTS |
| Kernel | Linux (Azure westcentralus) |
| Node.js | 22.23.1（通过 actions/setup-node@v4 设置） |
| npm | 10.9.8 |
| Go | 1.23（通过 actions/setup-go@v5 设置） |
| Git | 2.54.0 |

### 2.4 Job 1: `go-test`

**用途：** 运行 Go 后端全部测试，上传覆盖率报告。

**YAML 源码：**

```yaml
go-test:
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: server
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with: { go-version: '1.23' }
    - run: go vet ./...
    - run: go test ./... -cover -coverprofile=coverage.out -count=1 -timeout 120s
    - uses: actions/upload-artifact@v4
      if: always()
      with:
        name: go-coverage
        path: server/coverage.out
```

**每个步骤详解：**

| 步骤 | 命令 | 实际执行 | 预期输出 |
|------|------|---------|---------|
| checkout | `actions/checkout@v4` | git clone + checkout b1791f68 | "Syncing repository" |
| setup-go | `actions/setup-go@v5` | 安装/缓存 Go 1.23 | "go version go1.23.X" |
| vet | `go vet ./...` | 静态分析所有包 | 无错误输出 |
| test | `go test ./... -cover ...` | 运行 4 个包全部测试 | "ok /internal/auth 0.376s" 等 |
| upload | `actions/upload-artifact` | 上传 coverage.out | Artifact go-coverage |

**覆盖的包和测试数：**

| 包路径 | 测试文件 | 测试数 | 行数 | 覆盖内容 |
|--------|---------|--------|------|---------|
| `internal/auth` | `auth_test.go` | ~5 | 165 行 | JWT 签发/解析、bcrypt 哈希/验证、ValidateUsername、密码截断 |
| `internal/db` | `db_test.go`, `messages_test.go` | ~66 | 1040 行 | users/chats/messages/reactions/tokens DB 操作，所有 ErrNotFound/ErrConflict/空列表/no-op 边界 |
| `internal/testutil` | `handler_test.go`, `auth_flow_test.go`, `integration_test.go` | ~68 | 2032 行 | HTTP handler 集成测试、认证流程、完整 CRUD 场景 |
| `internal/ws` | `ws_test.go` | ~6 | 255 行 | WebSocket 连接、消息广播、typing、presence |

**总测试数：~134 个，总行数：~3500 行**

**预期运行时间：~11s**（DB 单元测试 ~1.7s + 集成测试 ~9s）

### 2.5 Job 2: `frontend-build`

**用途：** 安装前端依赖 + 构建 + 尝试运行 E2E 测试（失败不阻断）。

```yaml
frontend-build:
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: client
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with: { node-version: '22' }
    - run: npm ci
    - run: npx playwright install chromium --with-deps
    - run: npm run build
    - run: npx playwright test --project=chromium || echo "E2E skipped"
    - uses: actions/upload-artifact@v4
      with:
        name: client-dist
        path: client/dist/
```

**每个步骤详解：**

| 步骤 | 命令 | 做什么 | 失败时 |
|------|------|--------|-------|
| checkout | `actions/checkout@v4` | 拉取代码 | job 失败 |
| setup-node | `actions/setup-node@v4` | 设置 Node 22 | job 失败 |
| npm ci | `npm ci` | 精确按 lockfile 安装依赖（172 packages） | job 失败 |
| install browsers | `npx playwright install chromium --with-deps` | 安装 Chromium 1223 + 系统依赖（libasound, libcairo, libnss 等） | job 失败 |
| build | `npm run build` | vite build → dist/（67 modules, 297KB JS, 9KB CSS） | job 失败 |
| test | `npx playwright test --project=chromium \|\| echo "E2E skipped"` | 尝试跑测试，失败不打标记 | 不阻断 |
| upload artifact | `...` | 上传 dist/ | 不阻断 |

**构建产物：**

| 文件 | 大小 | 说明 |
|------|------|------|
| `dist/index.html` | 0.57 KB (gzip 0.39 KB) | SPA 入口 |
| `dist/assets/index-*.css` | 9.36 KB (gzip 2.50 KB) | 样式 |
| `dist/assets/index-*.js` | 297 KB (gzip 93.8 KB) | React 应用 + 依赖 |
| `dist/assets/index-*.js.map` | 1536 KB | Sourcemap |

### 2.6 Job 3: `go-build`

**用途：** 跨平台编译 chatd 二进制。只在 main 分支运行。

**执行条件：** `if: github.ref == 'refs/heads/main'` + `needs: [go-test]`

**编译矩阵：**

```yaml
strategy:
  matrix:
    include:
      - goos: linux
        goarch: amd64
        suffix: ''
      - goos: linux
        goarch: arm64
        suffix: ''
      - goos: windows
        goarch: amd64
        suffix: '.exe'
```

| 平台 | 输出文件名 | 大小（预估） |
|------|-----------|------------|
| Linux amd64 | `chatd-linux-amd64` | ~9 MB |
| Linux arm64 | `chatd-linux-arm64` | ~8.5 MB |
| Windows amd64 | `chatd-windows-amd64.exe` | ~9 MB |

**编译参数：** `-ldflags="-s -w"`（去掉符号表和调试信息，减小体积）

### 2.7 Job 4: `release`

**用途：** 下载构建产物 → 打包 → 创建 GitHub Release。只在 main 分支运行。

**执行条件：** `if: github.ref == 'refs/heads/main'` + `needs: [go-build, frontend-build]`

**权限：** `permissions: contents: write`（需要创建 Release 的权限）

**步骤详解：**

| 步骤 | 做什么 |
|------|--------|
| download chatd-* | 下载 3 个跨平台二进制（pattern: `chatd-*`, merge-multiple: true） |
| download client-dist | 下载前端构建产物 |
| assemble release | 创建 `release/` 目录，放入 chatd 二进制 + client-dist |
| upload artifact | 上传完整 release 包（chatd-release） |
| create release | `gh release create build-{sha}` → GitHub Release 页面 |

**Release 命名格式：** `build-{前 7 位 SHA}`，例如 `build-7e82cce`。

**Release 内容：**
```
release/
├── chatd-linux-amd64
├── chatd-linux-arm64
├── chatd-windows-amd64.exe
└── client-dist/
    ├── index.html
    └── assets/
        ├── index-*.js
        ├── index-*.css
        └── index-*.js.map
```

---

## 三、Frontend CI 流水线 (frontend-ci.yml) 完整解析

### 3.1 文件位置

`.github/workflows/frontend-ci.yml`（60 行）

### 3.2 触发条件

```yaml
on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]
```

只对 main 分支生效，master 不触发。

### 3.3 Job 1: `mock-test`

**用途：** 纯前端 Mock API 测试，不依赖后端。

**完整 YAML：**

```yaml
mock-test:
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: client
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: '22'
    - run: npm ci
    - run: npx playwright install chromium --with-deps
    - run: npm run build
    - name: Start Vite and run tests
      run: |
        npx vite --port 5173 &
        sleep 3
        npx playwright test tests/ci.spec.mjs --reporter=list 2>&1
```

**每个步骤详解：**

**Step 1 - checkout:** `actions/checkout@v4`

拉取仓库到 `/home/runner/work/chat-app/chat-app`。

**Step 2 - setup-node:** `actions/setup-node@v4` with node-version: '22'

从缓存或下载安装 Node.js 22.23.1 + npm 10.9.8。

**Step 3 - npm ci:**

精确按 `package-lock.json` 安装依赖。与 `npm install` 的区别：
| 命令 | 行为 |
|------|------|
| `npm ci` | 严格按 lockfile，删除 node_modules 重建，更快更可靠 |
| `npm install` | 可能更新 lockfile，安装最新兼容版本 |

安装 172 packages，94 个有资金赞助。

**Step 4 - playwright install chromium --with-deps:**

- 安装 Chromium for Testing 148.0.7778.96（playwright chromium v1223）
- `--with-deps` 自动安装系统库：`libasound2t64`, `libatk-bridge2.0-0t64`, `libcairo2`, `libcups2t64`, `libdbus-1-3`, `libdrm2`, `libgbm1`, `libglib2.0-0t64`, `libnspr4`, `libnss3`, `libpango-1.0-0`, `libx11-6` 等

**Step 5 - npm run build:**

```
vite v6.4.3 building for production...
transforming... ✓ 67 modules transformed.
rendering chunks...
✓ built in ~1.5s
  dist/index.html                 0.57 KB
  dist/assets/index-*.css         9.36 KB
  dist/assets/index-*.js        297 KB (sourcemap 1536 KB)
```

**Step 6 - Start Vite and run tests:**

关键步骤。在后台启动 Vite 开发服务器，然后运行 Playwright 测试。

```bash
npx vite --port 5173 &    # 后台启动开发服务器
sleep 3                    # 等待 Vite 启动
npx playwright test tests/ci.spec.mjs --reporter=list 2>&1
```

**Vite 启动日志：**
```
VITE v6.4.3  ready in ~1300ms
  ➜  Local:   http://localhost:5173/
```

**Playwright 配置：** `playwright.config.js`
```javascript
import { defineConfig } from '@playwright/test';
export default defineConfig({
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',  // 指向 Vite 开发服务器
    headless: true,                      // 无头模式
  },
});
```

**测试结果：15/15 通过 ✅**

#### Mock API 机制

测试不依赖真实后端，通过 Mock API 模拟全部 28 个后端方法。

**Mock 开关系统（`client/src/api/client.js`）：**

```javascript
const MOCKABLE = [
  ['register', mockRegister],
  ['login', mockLogin],
  ['logout', mockLogout],
  ['me', mockMe],
  // ... 共 28 对
];

api.enableMock = () => {
  for (const [key, mock] of MOCKABLE) {
    _originals[key] = api[key];  // 保存原函数
    api[key] = mock;             // 替换为 mock
  }
};

api.disableMock = () => {
  for (const [key] of MOCKABLE) {
    api[key] = _originals[key];  // 恢复原函数
  }
};
```

**Mock 数据初始化（`client/src/dev/dummy.js`）：**

```javascript
generateDummyData({ chatCount: 10, msgPerChat: 150 })
// → 10 个聊天（群组 + DM + 公开频道）
// → 1500 条消息（每聊天 150 条，含 @提及、Markdown、code block）
```

**Mock 自动恢复（`client/src/store/auth.js`）：**

```javascript
const saved = JSON.parse(localStorage.getItem('auth') || '{}');
if (saved.accessToken === 'mock-token') {
  api.enableMock();  // 页面刷新后自动恢复 Mock 模式
}
```

**Mock 登录流程：**
```
mockLogin()
  ├── api.enableMock()      // 替换 28 个 API 方法
  ├── setMode('poll')       // 切换到轮询模式
  ├── save({ accessToken: 'mock-token', user: mockUser })
  └── React Router → /      // 跳转到聊天页
```

#### 15 个测试详细说明

| # | 测试名 | 源码行 | 测试场景 | 前置条件 | 核心断言 | 耗时 | 覆盖的 Mock 方法 |
|---|--------|-------|---------|---------|---------|------|----------------|
| 1 | `debug mode toggle shows mock button` | 21 | Debug 模式 UI | `goto('/login')` + wait `.form-box` | `text=Quick Enter (mock)` 可见 | ~900ms | 无 |
| 2 | `mock login shows sidebar` | 28 | Mock 登录 | `mockLogin()` 助手 | `.sidebar` 可见 | ~350ms | `register`, `login`, `me`, `listChats` |
| 3 | `mock login shows chat items` | 33 | 聊天列表 | `mockLogin()` | `.chat-item` ≥ 1 个 | ~350ms | `listChats` |
| 4 | `mock send message` | 42 | 消息发送 | `openFirstChat()` | `.msg-content` 含 "Hello from CI!" | ~560ms | `listMessages`, `sendMessage` |
| 5 | `mock notice board: set, edit, clear` | 51 | 公告栏 CRUD | `openFirstChat()` | Set → 📌 可见 → Edit → 更新 → Clear → 消失 | ~500ms | `getChat`, `setPinnedMessage`, `clearPinnedMessage` |
| 6 | `logout returns to login` | 70 | 登出 | `mockLogin()` | URL → `/login`, h1 → "Welcome back!" | ~350ms | `logout` |
| 7 | `mock create and rename group chat` | 80 | 创建群聊 + 改名 | `mockLogin()` | header 含 "MockGroup" → Rename → "RenamedGroup" | ~440ms | `createChat`, `renameChat` |
| 8 | `mock create DM via search` | 97 | 搜索用户 → DM | `mockLogin()` | 搜索 "user" → 点击 User → 跳转到 DM 聊天 | ~370ms | `searchUsers`, `createDM` |
| 9 | `mock edit and delete message` | 109 | 编辑 + 删除消息 | `openFirstChat()` | Edit → 改内容 → Save / Delete → 删除 | ~480ms | `editMessage`, `deleteMessage` |
| 10 | `mock delete chat from context menu` | 133 | 右键删除聊天 | `mockLogin()` | 右键 → Delete → 列表减少 | ~350ms | `deleteChat` |
| 11 | `mock member panel interaction` | 147 | 成员面板 | `openFirstChat()` | + Add member → 搜索输入 | ~1.5s | `addMember`, `removeMember` |
| 12 | `mock public channels search visible` | 161 | 公开频道 | `mockLogin()` | 搜索 "public channels" | ~320ms | `listPublicChats`, `joinChat` |
| 13 | `mock open settings and close` | 170 | 设置页 | `mockLogin()` | Settings 可见 → Close | ~690ms | `updateProfile` |
| 14 | `mock upload file to composer` | 178 | 文件上传 | `openFirstChat()` | 📎 → 选文件 → `.file-attach` 可见 | ~1.6s | `upload` |
| 15 | `mock upload avatar in settings` | 190 | 头像上传 | `mockLogin()` | 点头像 → 选文件 → Save | ~900ms | `uploadAvatar` |

**测试助手函数：**

```javascript
async function mockLogin(page) {
  await page.goto('/login');
  await page.waitForSelector('.form-box');
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');
  await page.waitForSelector('.sidebar');
}

async function openFirstChat(page) {
  await mockLogin(page);
  await page.waitForSelector('.chat-item', { timeout: 5000 });
  await page.locator('.chat-item').first().click();
}
```

#### Mock API 覆盖率详细清单

| 类别 | 方法 | 源码 | CI 测试覆盖 | 实现完整性 |
|------|------|------|-----------|----------|
| **Auth** (4/4) | `register` | ✅ mock.js:252 | ✅ CI #2-15（mockLogin 内部调用） | 返回 user + access_token |
| | `login` | ✅ mock.js:260 | ✅ CI #2-15（mockLogin 内部调用） | 返回 user + access_token |
| | `refresh` | ✅ mock.js:268 | 非 Mock 路径（真实模式靠 HttpOnly cookie） | 返回新 access_token |
| | `logout` | ✅ mock.js:276 | ✅ CI #6 | 返回 { ok: true } |
| **User** (3/3) | `me` | ✅ mock.js:280 | ✅ CI #2-15（ChatPage mount 触发） | 返回 mock user |
| | `updateProfile` | ✅ mock.js:119 | ✅ CI #13（Settings 改名） | 更新 username/avatar |
| | `searchUsers` | ✅ mock.js:110 | ✅ CI #8, #11（DM/成员搜索） | 模糊搜索返回用户列表 |
| **Chat** (7/7) | `listChats` | ✅ mock.js:20 | ✅ CI #2-15（ChatPage mount 触发） | 返回 10 个 mock 聊天 |
| | `listPublicChats` | ✅ mock.js:284 | ✅ CI #12 | 过滤 public 聊天 |
| | `createChat` | ✅ mock.js:49 | ✅ CI #7 | 创建新群聊 + 加入列表 |
| | `getChat` | ✅ mock.js:43 | ✅ CI #5（点击聊天触发） | 返回聊天详情含 pinnedMessage |
| | `deleteChat` | ✅ mock.js:65 | ✅ CI #10 | 从列表移除 |
| | `renameChat` | ✅ mock.js:289 | ✅ CI #7 | 更新聊天名 |
| | `createDM` | ✅ mock.js:71 | ✅ CI #8 | 创建 DM 或返回已有 |
| | `joinChat` | ✅ mock.js:239 | ✅ CI #12 | 加入公开聊天 |
| **Pin** (2/2) | `setPinnedMessage` | ✅ mock.js:219 | ✅ CI #5, E2E #8 | 设置公告栏 + store.onChatUpdate |
| | `clearPinnedMessage` | ✅ mock.js:227 | ✅ CI #5, E2E #8 | 清除公告栏 + store.onChatUpdate |
| **Member** (2/2) | `addMember` | ✅ mock.js:92 | ✅ CI #11 | 添加成员到 chat.members |
| | `removeMember` | ✅ mock.js:101 | ✅ CI #11 | 从 chat.members 移除 |
| **Message** (5/5) | `listMessages` | ✅ mock.js:24 | ✅ CI #4, #5, #9（点击聊天触发） | 分页返回 50 条消息 |
| | `sendMessage` | ✅ mock.js:136 | ✅ CI #4, E2E #6 | 创建消息 + AI 自动回复 |
| | `editMessage` | ✅ mock.js:187 | ✅ CI #9 | 更新消息内容 + store.onMessageUpdate |
| | `deleteMessage` | ✅ mock.js:195 | ✅ CI #9 | 标记删除 + store.onMessageDelete |
| | `markRead` | ✅ mock.js:235 | 间接（ChatView 打开聊天自动调） | 返回 { ok: true } |
| **Reaction** (2/2) | `addReaction` | ✅ mock.js:203 | ✅ CI #9 间接（消息自带 reactions） | store.onReaction |
| | `removeReaction` | ✅ mock.js:211 | ✅ CI #9 间接 | store.onReaction(false) |
| **Upload** (2/2) | `upload` | ✅ mock.js:296 | ✅ CI #14 | 返回 mock file ID |
| | `uploadAvatar` | ✅ mock.js:299 | ✅ CI #15 | 返回 mock URL |

**总计：28/28 = 100%**

### 3.4 Job 2: `full-e2e`

**用途：** 启动真实后端 + Vite，运行全链路 E2E 测试。

**执行条件：** `needs: mock-test`（mock-test 成功后才会执行）

**完整 YAML：**

```yaml
full-e2e:
  needs: mock-test
  runs-on: ubuntu-latest
  defaults:
    run:
      working-directory: client
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: '22'
    - run: npm ci
    - run: npx playwright install chromium --with-deps
    - run: npm run build
    - uses: actions/setup-go@v5
      with:
        go-version: '1.23'
    - name: Start servers and run tests
      shell: bash
      run: |
        cd ${{ github.workspace }}/server
        go mod download
        go build -o /tmp/chatd ./cmd/chatd/
        /tmp/chatd &
        cd ${{ github.workspace }}/client
        npx vite --port 5173 &
        for i in $(seq 1 30); do
          curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && echo "backend ready" && break
          sleep 1
        done
        echo "starting tests..."
        npx playwright test tests/e2e.spec.mjs --reporter=list 2>&1
```

**启动后端的关键细节：**

```bash
cd ${{ github.workspace }}/server    # 切换到 server 目录（注意：用了 ${{ github.workspace }} 避免 working-directory: client 的影响）
go mod download                        # 下载 Go 依赖
 go build -o /tmp/chatd ./cmd/chatd/   # 预编译 chatd
/tmp/chatd &                            # 后台运行
```

**后端启动日志：**
```
chatd: listening on :8080
```

**等待后端就绪：**

```bash
for i in $(seq 1 30); do
  curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && echo "backend ready" && break
  sleep 1
done
```

最多等待 30 秒，每个 1 秒轮询 `/healthz`。后端通常在 ~3-5 秒内就绪（编译已在之前完成）。

**Vite 代理配置：**
```javascript
// vite.config.js
server: {
  proxy: {
    '/api': 'http://localhost:8080',      // API 请求转发到后端
    '/ws': { target: 'ws://localhost:8080', ws: true },  // WebSocket
    '/uploads': 'http://localhost:8080',   // 上传文件
  },
},
```

**测试结果：8/8 通过 ✅**

| # | 测试名 | 源码行 | 测试场景 | 前置条件 | 核心断言 | 耗时 | 后端端点 |
|---|--------|-------|---------|---------|---------|------|---------|
| 1 | `home redirects to login` | 4 | 未认证重定向 | `goto('/')` | URL 包含 `/login` | ~800ms | 前端路由 |
| 2 | `login form renders correctly` | 9 | 登录页 UI | `goto('/login')` | h1="Welcome back!" + email/password input + Log In 按钮 | ~250ms | 前端路由 |
| 3 | `register form renders correctly` | 17 | 注册页 UI | `goto('/register')` | h1="Create an account" + email/text/password input | ~230ms | 前端路由 |
| 4 | `full auth flow` | 25 | 注册 → 自动登录 | 填 email/username/password → Continue | URL → `/` + `.sidebar` 可见 | ~400ms | `POST /api/auth/register` |
| 5 | `create group chat` | 36 | 创建群聊 | 注册 → Create Group → 填名 → Create | `.chat-header` 含 "E2E Group" | ~510ms | `POST /api/auth/register`, `POST /api/chats` |
| 6 | `send and receive message` | 52 | 消息收发 | 注册 → 创建群聊 → 输入 → Send | `.msg-content` 含 "Hello E2E" | ~600ms | `POST /api/auth/register`, `POST /api/chats`, `POST */messages` |
| 7 | `responsive layout on mobile` | 72 | 手机视口适配 | `setViewportSize(375, 667)` → goto('/login') | `form.form-box` 可见 | ~220ms | 纯 CSS |
| 8 | `notice board as owner` | 78 | Owner 公告栏 CRUD | 注册 → 创建群聊 → Set/Edit/Clear | 📌 可见 → 更新 → 消失 | ~500ms | `POST /api/auth/register`, `POST /api/chats`, `PUT/PATCH */pin`, `DELETE */pin` |

**后端端点覆盖：** 5/32 ≈ 16%

| 端点 | 覆盖情况 | 被哪些测试调用 |
|------|---------|-------------|
| `POST /api/auth/register` | ✅ | #4, #5, #6, #8 |
| `POST /api/chats` | ✅ | #5, #6, #8 |
| `POST */messages` | ✅ | #6 |
| `PUT/PATCH */pin` | ✅ | #8 |
| `DELETE */pin` | ✅ | #8 |

> **说明：** 详细的 API 层后端测试（27/29 端点）由 `internal/testutil/` 包的 ~68 个集成测试覆盖。前端 E2E 主要验证完整的用户旅程。

---

## 四、测试文件结构

```
client/
├── tests/
│   ├── ci.spec.mjs       # 15 个 Mock API 测试（无后端依赖）
│   ├── e2e.spec.mjs      # 8 个全链路 E2E 测试（需后端）
│   └── README.md         # 测试说明
├── playwright.config.js   # Playwright 配置
│   timeout: 30000
│   retries: 0
│   use.baseURL: 'http://localhost:5173'
│   use.headless: true
├── package.json
│   scripts:
│     test:        playwright test tests/ci.spec.mjs
│     test:full:   playwright test tests/e2e.spec.mjs
│     test:all:    playwright test
└── src/
    ├── api/
    │   ├── client.js     # request() + api 对象 + MOCKABLE 28 对 + enableMock/disableMock
    │   └── mock.js       # 28 个 Mock 函数实现 + resetMockData()
    ├── store/
    │   └── auth.js       # mockLogin (enableMock + setMode + save mock-token)
    └── dev/
        ├── dummy.js      # generateDummyData({ chatCount: 10, msgPerChat: 150 })
        └── stream-source.js  # AI 流式回复模拟

server/
├── internal/
│   ├── auth/
│   │   └── auth_test.go       # JWT/密码测试（~5 tests）
│   ├── db/
│   │   ├── db_test.go         # DAO 层单元测试（~66 tests）
│   │   └── messages_test.go   # 消息层测试
│   ├── testutil/
│   │   ├── testutil.go        # Fixture 骨架
│   │   ├── handler_test.go    # HTTP handler 集成测试
│   │   ├── auth_flow_test.go  # 认证流程测试
│   │   ├── integration_test.go # 综合测试
│   │   └── ws_test.go         # WebSocket 测试
│   └── ws/
│       └── ws_test.go         # WS 功能测试
├── cmd/
│   └── chatd/
│       └── main.go            # 后端入口
└── .github/
    └── workflows/
        ├── ci.yml             # CI 流水线
        └── frontend-ci.yml    # 前端 CI 流水线
```

---

## 五、测试总数统计

| 层 | 测试文件 | 测试数 | 运行方式 | 依赖 | 预估耗时 |
|----|---------|--------|---------|------|---------|
| 前端 CI | `ci.spec.mjs` | 15 | `npm test` | 无 | ~30s |
| 前端 E2E | `e2e.spec.mjs` | 8 | `npm run test:full` | Go 后端 | ~60s |
| 前端合计 | 2 文件 | 23 | `npm run test:all` | | ~90s |
| 后端 Go | 5 文件 | ~134 | `go test ./...` | 无 | ~11s |
| **总计** | **7 文件** | **~157** | | | ~100s |

---

## 六、运行方法

```bash
# ── 前端 Mock 测试（推荐本地开发快速验证）──
cd client
npm test
# 等价于: npx playwright test tests/ci.spec.mjs
# 不依赖后端，15 个测试

# ── 前端 E2E 测试（需后端运行）──
cd client
npm run test:full
# 等价于: npx playwright test tests/e2e.spec.mjs
# 需 Go 后端在 localhost:8080 运行

# ── 前端全部测试 ──
cd client
npm run test:all
# 串行执行 CI + E2E，23 个测试

# ── 后端 Go 测试 ──
cd server
go test ./... -count=1 -timeout 120s
# ~134 个测试，~11 秒

# 仅 DB 层单元测试（更快）
go test ./internal/db/ -count=1 -v
# ~66 个测试，~1.7 秒

# 仅集成测试
go test ./internal/testutil/ -count=1 -v
# ~68 个测试，~9 秒

# 运行单个测试
go test ./internal/testutil/ -run TestCreateChat_InvalidType
```

---

## 七、运行时特征（实际 CI 数据）

来自最新成功运行 `f291423`（2026-07-09 21:42 UTC）：

### Job: mock-test

| 步骤 | 耗时 | 说明 |
|------|------|------|
| checkout | ~1s | git clone |
| setup-node | ~1s | 缓存命中 |
| npm ci | ~3s | 172 packages |
| playwright install | ~5s | chromium + deps |
| npm run build | ~2s | 67 modules |
| Start Vite + tests | ~15s | vite 启动 1.3s + 15 个测试 |
| **总计** | **~27s** | |

### Job: full-e2e

| 步骤 | 耗时 | 说明 |
|------|------|------|
| checkout | ~1s | |
| setup-node | ~1s | |
| npm ci | ~3s | |
| playwright install | ~5s | |
| npm run build | ~2s | |
| setup-go | ~1s | |
| go mod download + go build | ~30s | go 首次编译较慢 |
| start vite + wait healthz + tests | ~30s | 后端启动 ~3s + 8 个测试 |
| **总计** | **~73s** | |

### 完整流水线时间线

```
 0s  ─── Push to main
 5s  ├── CI: go-test 启动
20s  │   └── go-test 完成（134 tests ✅）
25s  ├── Frontend: mock-test 启动
     ├── CI: frontend-build 启动
55s  │   ├── mock-test 完成（15/15 ✅）
     │   └── frontend-build 完成
60s  ├── Frontend: full-e2e 启动
     ├── CI: go-build + release 启动
90s  │   ├── go-build 完成
100s │   ├── release 完成
130s │   └── full-e2e 完成（8/8 ✅）
```

**总运行时间：~2-3 分钟**（大部分时间花在 Go 编译）

---

## 八、问题修复历史

整个 CI/CD 搭建过程中遇到的 10 个问题：

| # | 提交 | 问题 | 症状 | 根因 | 修复方法 |
|---|------|------|------|------|---------|
| 1 | `96c5e9f` | Playwright 版本冲突 | `require` 报错 "ES module scope" | `package.json` 写 `^1.50.0` 但 lockfile 是 1.60.0，`npx playwright` 用了不同版本 | 锁定 `@playwright/test: 1.60.0`，删 lockfile 重新 `npm install` |
| 2 | `ca8dfec` | ESM vs CJS 冲突 | `require is not defined` | `package.json` 有 `"type": "module"` 但测试文件用 `require` | `.cjs`（再用 `.mjs`） + `require` → `import` |
| 3 | `59ac7d0` | npm install 不稳定 | CI 行为不一致 | `npm install` 可能更新 lockfile | `npm install` → `npm ci` |
| 4 | `bea83fc` | 8 个 CI 测试全失败 | `.chat-list` 不可见、`.chat-item` 超时、`.msg-content` 解析到旧内容 | 选择器太脆弱，`toBeVisible` 用在可能不存在的元素上 | 提取 `mockLogin`/`openFirstChat` 助手，用 `hasText` 筛选器 |
| 5 | `c235977` | strict mode violation | `.file-attach` 解析到 16 个元素 | `.first()` 没加 | 加 `.first()` |
| 6 | `6b41e49` | 刷新测试不稳定 | `.sidebar` 刷新后 20s 超时 | `page.reload()` 后 HMR 重新加载慢 | `page.reload` → `page.goto('/')` |
| 7 | `710f5db` | 后端启不动 | `http proxy error: /api/auth/register` ECONNREFUSED | `go run ./cmd/server/` 目录名错（实际是 `./cmd/chatd/`） | `./cmd/server/` → `./cmd/chatd/` |
| 8 | `79fc8f1` | sleep 不够 | 后端编译需要 >3s 但 `sleep 3` | Go 首次编译需要 10-20s | `sleep 3` → `go build` 预编译 + `sleep 10` |
| 9 | `b464794` | 后端路径错误 | `cd server` 走到了 `client/server/` | `defaults.run.working-directory: client` 导致相对路径错 | `cd server` → `cd ${{ github.workspace }}/server` |
| 10 | `f291423` | 后端启动时序 | 后端在测试开始后才启动 | `sleep` 是固定等待，不可靠 | `sleep 15` → `for curl healthz` 循环等待（最多 30s） |

---

## 九、当前状态

| 流水线 | Job | 测试数 | 状态 | 上次运行 |
|--------|-----|--------|------|---------|
| CI | go-test | ~134 | ✅ | 2026-07-09 21:42 |
| CI | frontend-build | — | ✅ | 2026-07-09 21:42 |
| CI | go-build | — | ✅ | 2026-07-09 21:42 |
| CI | release | — | ✅ | 2026-07-09 21:42 |
| Frontend CI | mock-test | 15 | ✅ | 2026-07-09 21:42 |
| Frontend CI | full-e2e | 8 | ✅ | 2026-07-09 21:42 |

**总测试数：23 前端 + ~134 后端 = ~157 个测试全部通过 🟢**

**前端 Mock API 覆盖：28/28 = 100%**

**CI/CD 状态：双流水线全绿 ✅**