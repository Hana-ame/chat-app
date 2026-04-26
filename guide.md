# Bot 开发指南

## 如何运行

```bash
# 1. 安装依赖
pip install -r server/requirements.txt

# 2. 构建前端
bash build_client.sh

# 3. 启动服务
bash server/run.sh
# → http://localhost:8000

# 4. 运行测试
pytest server/test_server.py -v
```

## 创建 Bot

### 步骤 1 — 注册用户账号

```bash
curl -X POST http://localhost:8000/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"myuser","password":"mypass"}'
```

返回：
```json
{"token":"<会话token>","user_id":2,"username":"myuser","avatar_color":"#3498db"}
```

### 步骤 2 — 创建 Bot

```bash
curl -X POST http://localhost:8000/api/bot/create \
  -H 'Content-Type: application/json' \
  -d '{"token":"<会话token>","name":"WeatherBot"}'
```

返回：
```json
{"id":1,"name":"WeatherBot","token":"bot_<hex>"}
```

保存 `token`（以 `bot_` 开头）—— 这是 bot 的永久 API 密钥。

### 步骤 3 — 删除 Bot

```bash
curl -X DELETE "http://localhost:8000/api/bot/1?token=<会话token>"
```

## Bot 鉴权

所有 API 端点都需要 `token` 参数。把你的 bot token 填在需要 `token` 的地方即可。

- **REST**：JSON body 或 query string 中带 `token`
- **WebSocket**：`ws://host:8000/ws/1?token=bot_<hex>`

## 发送消息

```bash
curl -X POST http://localhost:8000/api/msg \
  -H 'Content-Type: application/json' \
  -d '{"token":"bot_<token>","room_id":1,"content":"Hello!"}'
```

Bot 消息会带 `"is_bot": true`，前端会显示机器人图标。

## 接收消息（轮询）

```bash
curl "http://localhost:8000/api/poll?room_id=1&token=bot_<token>&after_id=0&timeout=30"
```

## Python Bot 示例

### 轮询模式

```python
import time
import requests

BASE = "http://localhost:8000"
BOT_TOKEN = "bot_your_token_here"
ROOM_ID = 1
last_id = 0

def send_message(text):
    r = requests.post(f"{BASE}/api/msg", json={
        "token": BOT_TOKEN,
        "room_id": ROOM_ID,
        "content": text,
    })
    return r.json()

def poll_messages():
    global last_id
    r = requests.get(f"{BASE}/api/poll", params={
        "room_id": ROOM_ID,
        "token": BOT_TOKEN,
        "after_id": last_id,
        "timeout": 30,
    })
    data = r.json()
    for msg in data.get("messages", []):
        last_id = msg["id"]
        yield msg

# Echo bot
if __name__ == "__main__":
    print("Echo bot 运行中...")
    for msg in poll_messages():
        if msg.get("is_bot"):
            continue  # 不回复 bot 消息
        content = msg.get("content", "")
        if content:
            send_message(f"你说: {content}")
```

### WebSocket 模式（Python）

```python
import asyncio
import json
import websockets

BOT_TOKEN = "bot_your_token_here"
WS_URL = f"ws://localhost:8000/ws/1?token={BOT_TOKEN}"

async def bot():
    async with websockets.connect(WS_URL, ping_interval=30) as ws:
        while True:
            raw = await ws.recv()
            msg = json.loads(raw)
            if msg.get("type") == "system":
                continue
            if msg.get("is_bot"):
                continue
            content = msg.get("content", "")
            if content.startswith("/echo "):
                await ws.send(json.dumps({
                    "type": "message",
                    "content": content[6:],
                }))

asyncio.run(bot())
```

### 命令行回复 Bot（curl 版）

```bash
#!/bin/bash
TOKEN="bot_your_token"
LAST_ID=0
while true; do
  RESP=$(curl -s "http://localhost:8000/api/poll?room_id=1&token=$TOKEN&after_id=$LAST_ID&timeout=30")
  MSGS=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); [print(m['id'],m['content']) for m in d.get('messages',[])]")
  while read -r mid content; do
    LAST_ID=$mid
    echo "收到: $content"
  done <<< "$MSGS"
  sleep 1
done
```

## API 参考

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录 / 自动注册 |
| GET | `/api/rooms?token=` | 列出所有房间 |
| POST | `/api/rooms` | 创建房间 |
| GET | `/api/history/{id}?token=` | 消息历史 |
| POST | `/api/msg` | 发送消息 (REST) |
| GET | `/api/poll?room_id=&token=&after_id=&timeout=` | 长轮询新消息 |
| WS | `/ws/{room_id}?token=` | WebSocket 实时消息 |
| POST | `/api/bot/create` | 创建 bot |
| GET | `/api/bot/list?token=` | 列出你的 bot |
| DELETE | `/api/bot/{id}?token=` | 删除 bot |

## 注意事项

- Bot token 永久有效（不像会话 token 24 小时后过期）
- Bot 可以看到和普通用户一样的消息
- Bot 消息前端显示机器人图标
- Bot 不能再创建子 bot
- 轮询超时 30 秒，没有新消息时返回空数组
- 跨域请求需带 CORS 头（服务端已配置 `*`）
