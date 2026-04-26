# 测试文档

## 运行测试

```bash
pytest server/test_server.py -v
```

## 总览

**测试数**: 25（17 功能 + 8 CORS）  
**覆盖范围**: 登录认证、房间管理、消息收发、Bot 系统、长轮询  
**CORS**: 所有端点返回 `Access-Control-Allow-Origin: *`

---

## 测试列表

### 1. 登录注册

| 测试 | 用例 | 状态码 |
|------|------|--------|
| `test_login_register` | 新用户自动注册并返回 token | 200 |
| `test_login_existing` | 已注册用户输入正确密码再次登录 | 200 |
| `test_login_wrong_password` | 已存在用户使用错误密码 | 401 |
| `test_login_empty_username` | 空用户名 | 400 |

### 2. 房间管理

| 测试 | 用例 | 状态码 |
|------|------|--------|
| `test_rooms_list` | 列出房间，"大厅"默认存在 | 200 |
| `test_rooms_list_unauthorized` | 伪造 token 访问 | 401 |
| `test_create_room` | 创建新房间 | 200 |
| `test_create_room_duplicate` | 创建同名房间 | 409 |

### 3. 消息收发

| 测试 | 用例 | 状态码 |
|------|------|--------|
| `test_send_message_rest` | REST 发送消息，验证返回内容 | 200 |
| `test_history` | 获取历史消息，验证顺序和内容 | 200 |
| `test_empty_message_rejected` | 发送空消息 | 400 |

### 4. Bot 系统

| 测试 | 用例 | 状态码 |
|------|------|--------|
| `test_bot_create_list_delete` | 创建→列表→删除 Bot 完整流程 | 200 |
| `test_bot_auth_denied` | 无效 token 被拒，bot token 可访问 | 200 / 401 |
| `test_bot_send_message` | Bot 发消息，验证 `is_bot=true` | 200 |
| `test_bot_cannot_create_bot` | Bot token 不能创建子 bot | 401 |
| `test_bot_delete_removes_from_cache` | 删除 bot 后缓存立即清理，token 失效 | 200 / 401 |

### 5. 长轮询

| 测试 | 用例 | 状态码 |
|------|------|--------|
| `test_long_poll_timeout` | 无新消息时 1 秒超时返回空数组 | 200 |

---

## CORS 验证

8 个专属 CORS 测试覆盖所有端点，确保跨域访问正常：

| 测试 | 端点 | 验证内容 |
|------|------|---------|
| `test_cors_login_preflight` | `OPTIONS /api/login` | 预检返回 allow-origin + allow-methods |
| `test_cors_login_normal` | `POST /api/login` | 正常请求也带 CORS 头 |
| `test_cors_rooms` | `GET /api/rooms` | 跨域 + token 鉴权 |
| `test_cors_bot` | `POST /api/bot/create` | 写操作 + 跨域 |
| `test_cors_msg` | `POST /api/msg` | 消息发送 + 跨域 |
| `test_cors_history` | `GET /api/history/1` | 历史查询 + 跨域 |
| `test_cors_poll` | `GET /api/poll` | 长轮询 + 跨域 |
| `test_cors_unauthorized` | `GET /api/rooms?token=bad` | **401 也返回 CORS 头** |

后端 CORS 中间件配置：
```python
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)
```

- **所有响应**携带 `access-control-allow-origin: *`
- **OPTIONS 预检请求**由中间件自动响应，无需手动处理
- Widget (`widget.js`) 和 SPA (`chat-app-fastapi.pages.dev`) 均可跨域调用 API

---

## 测试架构

| 组件 | 说明 |
|------|------|
| 测试框架 | `pytest` (strict async mode) |
| HTTP 客户端 | `starlette.TestClient`（同步，自动处理 lifespan） |
| 数据库隔离 | `CHAT_DB_PATH` 环境变量 → tmp 文件 |
| Fixture 作用域 | `session` — 所有测试共享一次 DB 初始化 |

---

## API 端点覆盖

| 方法 | 路径 | 测试 |
|------|------|------|
| POST | `/api/login` | 4 个 |
| GET | `/api/rooms` | 2 个 |
| POST | `/api/rooms` | 2 个 |
| POST | `/api/msg` | 3 个 |
| GET | `/api/history/{room_id}` | 1 个 |
| GET | `/api/poll` | 1 个 |
| POST | `/api/bot/create` | 4 个 |
| GET | `/api/bot/list` | 2 个 |
| DELETE | `/api/bot/{bot_id}` | 2 个 |
