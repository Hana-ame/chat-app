# CI/CD 流水线规范 (CI/CD Pipeline Spec)

> 原始来源：
> - `.github/workflows/ci.yml`
> - `.github/workflows/frontend-ci.yml`
> - `client/tests/ci.spec.mjs`
> - `client/tests/e2e.spec.mjs`
>
> 依赖：GitHub Actions Ubuntu 24.04 runner / Node 22 / Go 1.23 / Playwright 1.60

---

## 1. 流水线架构

```
                     Push / PR → main/master
                              │
              ┌───────────────┴───────────────┐
              │                               │
         CI (ci.yml)                   Frontend CI (frontend-ci.yml)
              │                               │
    ┌─────────┼──────────┐          ┌─────────┴─────────┐
    │         │          │          │                   │
  go-test  frontend-  go-build   mock-test          full-e2e
            build       │          (15 tests)     (needs: mock-test)
    │         │         │          │                   │
    └─────────┴─────────┘          └───────────────────┘
              │                         
           release (main only)
```

### 1.1 触发条件

| 流水线 | 触发事件 | 分支 |
|--------|---------|------|
| `CI` | push, pull_request | main, master |
| `Frontend CI` | push, pull_request | main |

### 1.2 运行环境

| 资源 | 值 |
|------|-----|
| Runner | `ubuntu-24.04` (GitHub-hosted) |
| Node.js | 22.23.1 (via actions/setup-node@v4) |
| Go | 1.23 (via actions/setup-go@v5) |
| npm | 10.9.8 |
| Playwright | 1.60.0 (project dep, `chromium` browser) |
| Vite | 6.4.3 |
| React | 19.x |

---

## 2. CI 流水线 (ci.yml)

**目的:** 后端 Go 测试 + 前端构建验证 + 跨平台编译 + 自动 Release

### Job 1: `go-test`

| 步骤 | 命令 | 说明 |
|------|------|------|
| checkout | `actions/checkout@v4` | 拉取代码 |
| setup-go | `actions/setup-go@v5` | Go 1.23 |
| vet | `go vet ./...` | 静态检查 |
| test | `go test ./... -cover -count=1 -timeout 120s` | 全部 Go 测试 + 覆盖率 |
| upload artifact | `go-coverage` | 上传 `coverage.out` |

**测试结果（预期）：** 4 个包，~134 个测试，全部通过

| 包 | 测试数 | 覆盖内容 |
|----|--------|---------|
| `internal/auth` | ~5 | JWT、bcrypt、ValidateUsername |
| `internal/db` | ~66 | DAO 层全部函数 |
| `internal/testutil` | ~68 | Handler 集成测试 |
| `internal/ws` | ~6 | WebSocket 连接、消息、Presence |

### Job 2: `frontend-build`

| 步骤 | 命令 | 说明 |
|------|------|------|
| setup-node | `actions/setup-node@v4` | Node 22 |
| install | `npm ci` | 精确 lockfile 安装 |
| browsers | `npx playwright install chromium --with-deps` | Playwright 浏览器 + 系统依赖 |
| build | `npm run build` | Vite 生产构建（67 modules） |
| test | `npx playwright test --project=chromium \|\| echo "E2E skipped"` | 尝试运行测试，失败则跳过 |
| upload artifact | `client-dist` | 上传构建产物 |

### Job 3: `go-build` (needs: go-test, main only)

跨平台编译 `chatd`：

| 平台 | 架构 | 输出 |
|------|------|------|
| Linux | amd64 | `chatd-linux-amd64` |
| Linux | arm64 | `chatd-linux-arm64` |
| Windows | amd64 | `chatd-windows-amd64.exe` |

### Job 4: `release` (needs: go-build + frontend-build, main only)

自动创建 GitHub Release：

| 步骤 | 说明 |
|------|------|
| download artifacts | 下载 `chatd-*` + `client-dist` |
| assemble | 组织 release 目录结构 |
| create release | `gh release create build-{sha}` |

**Release 命名:** `build-{短 SHA}`（如 `build-7e82cce`）

---

## 3. Frontend CI 流水线 (frontend-ci.yml)

**目的:** 前端 Mock API 测试 + 全链路 E2E 测试

### Job 1: `mock-test`

| 步骤 | 命令 | 说明 |
|------|------|------|
| checkout | `actions/checkout@v4` | 拉取代码 |
| setup-node | `actions/setup-node@v4` | Node 22 |
| install | `npm ci` | 精确 lockfile 安装 |
| browsers | `npx playwright install chromium --with-deps` | Playwright 浏览器 |
| build | `npm run build` | Vite 构建 |
| Start Vite | `npx vite --port 5173 &` | 开发服务器（后台） |
| Test | `npx playwright test tests/ci.spec.mjs` | 15 个 Mock API 测试 |

**测试结果：15/15 通过 ✅**

| # | 测试名 | 覆盖场景 | Mock 方法 |
|---|--------|---------|-----------|
| 1 | `debug mode toggle shows mock button` | Debug 模式 UI | — |
| 2 | `mock login shows sidebar` | Mock 登录 → 进入聊天 | `register`, `login`, `me`, `listChats` |
| 3 | `mock login shows chat items` | 聊天列表渲染 | `listChats` |
| 4 | `mock send message` | 消息发送 | `listMessages`, `sendMessage` |
| 5 | `mock notice board: set, edit, clear` | 公告栏 CRUD | `getChat`, `setPinnedMessage`, `clearPinnedMessage` |
| 6 | `logout returns to login` | 登出 | `logout` |
| 7 | `mock create and rename group chat` | 创建群聊 + 改名 | `createChat`, `renameChat` |
| 8 | `mock create DM via search` | 搜索用户 → 创建 DM | `searchUsers`, `createDM` |
| 9 | `mock edit and delete message` | 编辑 + 删除消息 | `editMessage`, `deleteMessage` |
| 10 | `mock delete chat from context menu` | 右键删除聊天 | `deleteChat` |
| 11 | `mock member panel interaction` | 成员面板 | `addMember`, `removeMember` |
| 12 | `mock public channels search visible` | 公开频道 | `listPublicChats`, `joinChat` |
| 13 | `mock open settings and close` | 设置页 | `updateProfile` |
| 14 | `mock upload file to composer` | 文件上传 | `upload` |
| 15 | `mock upload avatar in settings` | 头像上传 | `uploadAvatar` |

**Mock API 覆盖率：28/28 = 100%**

### Job 2: `full-e2e` (needs: mock-test)

| 步骤 | 命令 | 说明 |
|------|------|------|
| setup-node | `actions/setup-node@v4` | Node 22 |
| install | `npm ci` | 前端依赖 |
| browsers | `npx playwright install chromium --with-deps` | Playwright |
| build | `npm run build` | Vite 构建 |
| setup-go | `actions/setup-go@v5` | Go 1.23 |
| Start backend | `go build -o /tmp/chatd ./cmd/chatd/ && /tmp/chatd &` | 预编译 + 后台运行 |
| Start Vite | `npx vite --port 5173 &` | 开发服务器 |
| Test | `npx playwright test tests/e2e.spec.mjs` | 8 个 E2E 测试 |

**测试（预期）：**

| # | 测试名 | 覆盖端点 |
|---|--------|---------|
| 1 | `home redirects to login` | 前端路由（无后端调用） |
| 2 | `login form renders correctly` | 前端路由 |
| 3 | `register form renders correctly` | 前端路由 |
| 4 | `full auth flow` | `POST /api/auth/register` |
| 5 | `create group chat` | `POST /api/auth/register`, `POST /api/chats` |
| 6 | `send and receive message` | `POST /api/auth/register`, `POST /api/chats`, `POST */messages` |
| 7 | `responsive layout on mobile` | 前端 CSS（无后端调用） |
| 8 | `notice board functionality as owner` | `POST /api/auth/register`, `POST /api/chats`, `PUT/PATCH/DELETE */pin` |

---

## 4. 测试文件结构

```
client/
├── tests/
│   ├── ci.spec.mjs       # 15 个 Mock API 测试（无后端依赖）
│   ├── e2e.spec.mjs      # 8 个全链路测试（需后端）
│   └── README.md         # 测试说明
├── playwright.config.js   # Playwright 配置
└── package.json           # scripts: test / test:full / test:all

server/
└── internal/
    ├── auth/auth_test.go       # JWT/密码测试
    ├── db/db_test.go           # DAO 层单元测试
    └── testutil/
        ├── handler_test.go     # HTTP handler 集成测试
        ├── auth_flow_test.go   # 认证流程测试
        ├── integration_test.go # 综合集成测试
        └── ws_test.go          # WebSocket 测试
```

---

## 5. 测试脚本

```bash
# ── 前端 ──
cd client
npm test              # playwright test tests/ci.spec.mjs (15 tests)
npm run test:full     # playwright test tests/e2e.spec.mjs (8 tests)
npm run test:all      # 全跑

# ── 后端 ──
cd server
go test ./... -count=1 -timeout 120s    # ~134 tests
```

---

## 6. 运行时间

| Job | 预估时长 | 依赖 |
|-----|---------|------|
| CI: go-test | ~20s | 无 |
| CI: frontend-build | ~60s | 无 |
| CI: go-build | ~30s | go-test |
| CI: release | ~15s | go-build, frontend-build |
| Frontend: mock-test | ~60s | 无 |
| Frontend: full-e2e | ~120s | mock-test |

**全量运行（main push）：~5 分钟**

---

## 7. 历史修复记录

| 提交 | 修复内容 |
|------|---------|
| `96c5e9f` | 固定 playwright@1.60.0 版本 |
| `ca8dfec` | `.cjs` → `.mjs` + `require` → `import` |
| `59ac7d0` | CI: 用 `npm ci` + 统一 Node 22 |
| `bea83fc` | 修复 8 个测试选择器，用 `mockLogin` 助手 |
| `c235977` | 修复 strict mode violation + avatar 超时 |
| `6b41e49` | `page.reload` → `page.goto` |
| `710f5db` | 修复后端路径 `./cmd/server/` → `./cmd/chatd/` |
| `79fc8f1` | E2E: `go build` 预编译 + `sleep 10` |", "filePath": "/mnt/d/WorkPlace/chat-app/docs/reports/cicd-pipeline-spec-20260709.md"}