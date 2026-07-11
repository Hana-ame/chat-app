# WebChat 部署指南

> 版本: v0.1.0-beta

---

## 部署方式选择

| 方式 | 适用场景 | 复杂度 |
|------|---------|--------|
| **一体化部署** | 小范围测试、单机部署 | ⭐ 低 |
| **前后端分离** | 生产环境、CDN 加速 | ⭐⭐ 中 |

---

## 方式一：一体化部署（推荐小范围测试）

Go 后端直接托管前端静态文件，一个进程搞定。

### 1. 构建

```bash
# 构建前端
cd client && npm ci && npm run build

# 构建后端
cd server && go build -o ../chatd ./cmd/chatd/

# 或用 Makefile
make build
```

产物：
- `chatd` — 后端二进制
- `client/dist/` — 前端静态文件（Go 会自动提供服务）

### 2. 配置环境变量

```bash
# 最小配置
export CHAT_JWT_SECRET=你的固定密钥           # 必填！不设则每次重启 token 失效
export CHAT_STATIC_DIR=./client/dist          # 前端静态文件目录
export CHAT_ADDR=:8080                        # 监听端口

# 可选
export CHAT_DB_PATH=./chat.db                 # SQLite 路径
export CHAT_ACCESS_TTL=30m                    # Access token 有效期
export CHAT_REFRESH_TTL=8760h                 # Refresh token 有效期
export CHAT_BASE_URL=https://chat.example.com # 回调地址
export WS_ENABLED=false                       # 是否启用 WebSocket
```

或使用 `.env` 文件：

```bash
cp server/.env.example server/.env
# 编辑 .env 修改 CHAT_JWT_SECRET 等配置
```

### 3. 启动

```bash
# 直接运行
./chatd

# 或用 start.sh（自动设置 ulimit + 日志）
cd server && bash start.sh
```

### 4. nginx 反向代理（可选）

```nginx
server {
    listen 443 ssl;
    server_name chat.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

---

## 方式二：前后端分离（生产环境）

前端部署在 Cloudflare Pages，后端部署在 VPS。

### 后端（VPS）

```bash
# 构建（仅后端）
cd server && go build -ldflags="-s -w" -o chatd ./cmd/chatd/

# 传输到服务器
scp chatd user@your-server:~/

# 登录服务器启动
export CHAT_JWT_SECRET=你的固定密钥
export CHAT_ADDR=:8080
export CHAT_STATIC_DIR=""  # 不托管前端文件
./chatd
```

CORS 已在 Go 中配置 `AllowOrigins: *`，无需额外设置。

### 前端（Cloudflare Pages）

1. Fork 或连接到 GitHub 仓库
2. 在 Cloudflare Pages 中创建项目：
   - **构建命令**: `cd client && npm ci && npm run build`
   - **输出目录**: `client/dist`
3. 设置环境变量：
   - `VITE_API_BASE`: 后端地址（如 `https://chat-api.example.com`）
4. 配置自定义域名

> **注意**: 每次 `git push` 到 main 分支，Cloudflare Pages 自动重新部署前端。

---

## 环境变量参考

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `CHAT_ADDR` | `:8080` | 监听地址 |
| `CHAT_DB_PATH` | `chat.db` | SQLite 数据库路径 |
| `CHAT_JWT_SECRET` | 随机值 | **生产必设！** JWT 签名密钥 |
| `CHAT_ACCESS_TTL` | `30m` | Access token 有效期 |
| `CHAT_REFRESH_TTL` | `8760h` (1年) | Refresh token 有效期 |
| `CHAT_STATIC_DIR` | `../client/dist` | 前端静态文件目录。留空则不托管前端 |
| `CHAT_BASE_URL` | `""` | 回调地址（用于 Cookie 的 Domain） |
| `CHAT_MAX_UPLOAD` | `20971520` | 最大上传字节数（已废弃，前端直传外部服务） |
| `WS_ENABLED` | `""` (禁用) | 设为 `true` 以启用 WebSocket |

---

## 生产 Checklist

- [ ] `CHAT_JWT_SECRET` 设为固定值（`openssl rand -hex 32`）
- [ ] 数据库路径使用绝对路径
- [ ] 文件描述符限制已提升（`ulimit -n 65535`）
- [ ] HTTPS 已启用（Cloudflare / nginx certbot）
- [ ] `WS_ENABLED` 按需配置（未修复安全问题时建议 `false`）
- [ ] 日志轮转已配置（或使用 systemd journal）
- [ ] 定期备份 `chat.db`

---

## 常见问题

### WebSocket 连接失败
确认 `WS_ENABLED=true` 已设置。如果前端使用 SSE 或轮询模式，不需要 WebSocket。

### 首次编译很慢
`modernc.org/sqlite` 首次编译生成大量 Go 代码，需 3-5 分钟。后续编译有缓存会快很多。

### 所有用户 token 失效
`CHAT_JWT_SECRET` 被更改或未设置（每次启动随机生成）。生产务必固定此值。

### 一键部署到新服务器

```bash
# 在本地构建
make build

# 上传到服务器
scp chatd user@host:~/
scp -r client/dist user@host:~/client-dist/

# 在服务器启动
ssh user@host '
  export CHAT_JWT_SECRET=$(openssl rand -hex 32)
  export CHAT_STATIC_DIR=./client-dist
  export CHAT_DB_PATH=/var/data/chat.db
  nohup ./chatd > chatd.log 2>&1 &
'
```
