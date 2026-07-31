# 快速开始

5 分钟内本地跑起来。

## 环境要求

- Go 1.23+（`go.mod` 声明版本）
- Node.js 20+（Vite 6）
- Python 3（可选，用于 `scripts/deploy_local.py`）

## 1. 配置环境变量

复制 `.env.example`（如存在）为 `.env`，或在 shell 中导出。最小配置：

```bash
CHAT_JWT_SECRET=请设置一个长随机字符串
CHAT_DB_PATH=chat.db
CHAT_BASE_URL=http://localhost:8080   # 上传等接口返回的绝对 URL 前缀
```

未设置 `CHAT_JWT_SECRET` 时服务会生成随机密钥（重启后所有会话失效）。完整变量表见 [guide/deployment.md](deployment.md#环境变量)。

## 2. 构建 + 启动

一键脚本（编译前端 + 后端，捕获日志到 `server.log`）：

```bash
python scripts/deploy_local.py all      # 构建 + 启动
python scripts/deploy_local.py restart  # 重启（不编译）
python scripts/deploy_local.py kill     # 停止
```

手动方式：

```bash
# 后端（需先配 .env）
cd server && go build -o ../chatd.exe ./cmd/chatd/ && ../chatd.exe

# 前端
cd client && npm ci && npm run build
# 开发模式（Vite dev server，默认 5173 端口，代理 /api 到后端）
cd client && npx vite --port 5173
```

## 3. 验证

```bash
curl http://localhost:8080/healthz        # {"status":"ok",...}
curl http://localhost:8080/api/version    # {"version":"0.9.4",...}
```

浏览器访问 `http://localhost:5173`，注册一个账号即可使用。

## 常见问题

- **8080 端口被占用**：`CHAT_ADDR=:8081 ./chatd.exe`
- **登录接口 401**：检查 `CHAT_JWT_SECRET` 是否变化（旧会话会失效）
- **上传返回的 url 无法访问**：确认 `CHAT_BASE_URL` 指向可从浏览器访问的地址
- **AI 流式 400（endpoint 拒绝）**：AI 端点为私有 IP 时需 `CHAT_AI_ALLOW_PRIVATE=1`（SSRF 防护默认拦截，见 [security.md](../security.md)）
