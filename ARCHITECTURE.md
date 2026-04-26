# 架构与技术文档

## 概述

Chat App 是一个类 IM（QQ / Line / WeChat / Discord）的聊天服务端。基于房间的实时消息 + Bot API。两套前端共享同一后端：

| 前端 | 入口 | 技术 |
|------|------|------|
| SPA（独立页面） | `/` | React + Vite |
| 内联 Widget | `<script src=".../widget.js">` | 纯 JS，无依赖 |

后端：**FastAPI** + **aiosqlite**（异步 SQLite + 写攒批）。单进程同时处理 REST、WebSocket、静态文件——开发环境无需 nginx。

## 目录结构

```
chat-app/
├── server/
│   ├── main.py           # FastAPI 应用，全部路由，鉴权
│   ├── db.py             # 数据库层（aiosqlite，写攒批）
│   ├── room_manager.py   # WebSocket 房间管理 + 长轮询
│   ├── schema.sql        # DDL + 种子数据
│   ├── run.sh            # 启动脚本
│   ├── requirements.txt
│   └── test_server.py    # Pytest 测试（17 个用例）
├── client/               # React SPA
│   ├── src/
│   │   ├── components/   # Login.jsx, Chat.jsx, MessageItem.jsx
│   │   └── hooks/        # useChat.js（WS + 轮询混合）
│   ├── dist/             # Vite 构建产物
│   └── vite.config.js    # 开发代理到 :8000
├── widget.js             # 内联聊天 Widget（CDN 直接访问）
├── build_client.sh
├── guide.md              # Bot 开发指南
├── ARCHITECTURE.md       # 本文件
└── README.md
```

## 数据库（SQLite）

```
users        ─── id, username, password, avatar_color, created_at
rooms        ─── id, name, description, created_at
messages     ─── id, room_id→rooms, user_id→users, content, msg_type, created_at
bot_tokens   ─── id, user_id→users, name, token, created_at
```

设计要点：
- **WAL 模式** — 读写并发
- **写攒批** — INSERT 排队，每 200ms 批量提交（最多 50 条/批），~5000 写/秒
- **外键** — 强制引用完整性
- **种子数据** — 房间"大厅"和用户"系统通知"首次启动自动创建

## 后端架构

### 1. 服务生命周期 (`lifespan`)

```
启动:
  db.init()           → 打开 SQLite, 执行 schema.sql, 启动攒批循环
  load_bot_cache()    → 将所有 bot token 加载到内存
  _cleanup_task()     → 后台任务: 每 10 分钟清理过期 session

关闭:
  _cleanup_task.cancel()
  db.close()          → 排空写队列, 提交, 关闭连接
```

### 2. 鉴权

**普通用户**: `POST /api/login` → 返回 `token`（UUID hex）。存入 `active_tokens` 字典，24 小时过期。

**Bot**: `POST /api/bot/create` → 返回带 `bot_` 前缀的 token。存入 `bot_tokens_cache` 字典（启动时从 DB 加载到内存，创建时即时添加）。

`get_user(token)` 同时检查两个字典：
```python
def get_user(token):
    u = active_tokens.get(token)        # session token
    if u: return {**u, "is_bot": False}
    b = bot_tokens_cache.get(token)     # bot token
    if b: return b                      # 自带 is_bot=True
    return None
```

### 3. 消息流

```
客户端发消息:
  ┌─ REST  → POST /api/msg       → room_manager.send_message()
  └─ WS    → {"type":"message"}  → room_manager.send_message()
                                    │
                                    ├─ db.save_message() → SQLite INSERT
                                    │
                                    └─ room._broadcast()
                                       ├─ 所有 WS 客户端    → send_bytes()
                                       └─ 所有 poll 客户端  → event.set()
```

### 4. 连接模式

**WebSocket**（主要）：
- 连接：`ws://host:8000/ws/{room_id}?token=...`
- TCP keepalive：空闲 60s，间隔 10s，探测 3 次
- 5 分钟无活动自动断开
- 心跳：客户端发 `{"type":"ping"}`，服务端回 `{"type":"pong"}`

**长轮询**（降级）：
- `GET /api/poll?room_id=&token=&after_id=&timeout=30`
- 最多阻塞 30 秒等待新消息
- 使用 `asyncio.Event` 实现有新消息时即时唤醒
- 超时返回空数组

### 5. 房间管理器

```
RoomManager
  ├── rooms: dict[int, Room]
  │
  └── Room (每个 room_id)
      ├── ws_connections: dict[WebSocket, WSConnection]
      ├── poll_clients: dict[str, PollClient]
      ├── last_msg_id: int
      └── online_count: len(ws) + len(poll)
```

- `ws_join()` / `ws_leave()` — 广播进出系统消息
- `send_message()` — 存 DB，设 `last_msg_id`，广播给所有对端
- 断开的 WebSocket 在广播时惰性清理

### 6. Bot 系统

Bot 本质是绑定到用户账号的 API key：

```python
# 创建
POST /api/bot/create  {"token": "<用户token>", "name": "MyBot"}
→ {"id": 1, "name": "MyBot", "token": "bot_<hex>"}

# 列出
GET /api/bot/list?token=<用户token>
→ {"bots": [...]}

# 删除
DELETE /api/bot/{id}?token=<用户token>
→ {"ok": true}

# 使用（发消息）
POST /api/msg {"token": "bot_<hex>", "content": "hello"}
→ {..., "is_bot": true, "username": "MyBot"}
```

关键特性：
- Bot token **永久有效**（存在 DB，缓存在内存）
- 前端显示机器人图标
- Bot 共享所属用户的 `avatar_color`
- Bot **不能**创建子 bot（防止递归）
- 删除 bot 后内存缓存即时清理

### 7. 内联 Widget

`widget.js` 在仓库根目录 — 单个 JS 文件，通过 `<script>` 注入：

```html
<script src="https://cdn.jsdelivr.net/gh/Hana-ame/chat-app@main/widget.js"></script>
```

纯 JavaScript（无 React/Babel/CDN 依赖）。加载后：
1. 将 CSS 注入 `<head>`
2. 创建浮动球 + 聊天窗口作为 DOM 元素
3. 处理登录、轮询、消息渲染、拖拽移动

API 地址 `wsl-8000.moonchan.xyz` 硬编码在文件中。后端 CORS 中间件允许 Widget 从任何页面跨域请求。

## 如何运行

```bash
# 1. 安装依赖
pip install -r server/requirements.txt

# 2. 构建前端
bash build_client.sh

# 3. 启动
bash server/run.sh
# → http://localhost:8000

# 4. 测试
pytest server/test_server.py -v
```

## API 参考

所有端点需要 `token`（除 `/api/login`），支持 query 参数或 JSON body。

| 方法 | 路径 | 鉴权 | 说明 |
|------|------|------|------|
| POST | `/api/login` | — | 登录 / 自动注册 |
| GET | `/api/rooms` | token | 列出房间 |
| POST | `/api/rooms` | token | 创建房间 |
| GET | `/api/history/{room_id}` | token | 消息历史 |
| POST | `/api/msg` | token | 发消息 |
| GET | `/api/poll` | token | 长轮询新消息 |
| WS | `/ws/{room_id}` | token | WebSocket 实时 |
| POST | `/api/bot/create` | token | 创建 bot |
| GET | `/api/bot/list` | token | 列出 bot |
| DELETE | `/api/bot/{id}` | token | 删除 bot |

## 测试

```bash
pytest server/test_server.py -v    # 17 个测试
```

测试架构：
- **Starlette TestClient**（同步，自动处理 lifespan）
- **Session 级 fixture** — 所有测试共享一次 DB 初始化
- **CHAT_DB_PATH** 环境变量 → 临时文件（隔离生产数据）
- 覆盖：登录/注册、鉴权错误、房间增删、消息发送/历史、bot 增删、bot 发消息、bot 鉴权、bot 删除+缓存清理、长轮询超时

## 部署

### 开发
```bash
pip install -r server/requirements.txt
bash build_client.sh
bash server/run.sh          # → http://0.0.0.0:8000
```

### 生产

```bash
# 构建前端
bash build_client.sh

# 放 nginx 后面或者 Cloudflare Tunnel
# 服务端直接 serve client/dist/
# 环境变量:
#   CHAT_DB_PATH=/data/chat.db   (默认: server/chat.db)
```

### Cloudflare Pages 部署

根据 `其他设置.txt`：push 即部署到 `https://chat-app-fastapi.pages.dev/`

SPA 静态文件在 `client/dist/`，Cloudflare Pages 自动部署。Python 后端需单独运行（Pages 不支持 Python）。

### 反向代理

```nginx
# nginx 配置示例（用于 wsl-8000.moonchan.xyz）
server {
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

`Upgrade` + `Connection` 头实现 WebSocket 穿透。
