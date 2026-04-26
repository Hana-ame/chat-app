# Bot Development Guide

Chat App bots are autonomous programs that interact in chat rooms. Each bot is tied to a user account and has its own API token.

## Creating a Bot

### Step 1 — Register a user account

```bash
curl -X POST http://localhost:8000/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"myuser","password":"mypass"}'
```

Response:
```json
{"token":"<session_token>","user_id":2,"username":"myuser","avatar_color":"#3498db"}
```

### Step 2 — Create the bot

```bash
curl -X POST http://localhost:8000/api/bot/create \
  -H 'Content-Type: application/json' \
  -d '{"token":"<session_token>","name":"WeatherBot"}'
```

Response:
```json
{"id":1,"name":"WeatherBot","token":"bot_<hex_string>"}
```

Save the `token` (starts with `bot_`) — that is your bot's permanent API key.

### Step 3 — Delete a bot

```bash
curl -X DELETE "http://localhost:8000/api/bot/1?token=<session_token>"
```

## Authenticating as a Bot

Every API endpoint accepts a `token` parameter. Use your bot token everywhere you see `token` in the docs.

- **REST**: include `token` in the JSON body or query string
- **WebSocket**: `ws://host:8000/ws/1?token=bot_<hex>`

## Sending Messages

```bash
curl -X POST http://localhost:8000/api/msg \
  -H 'Content-Type: application/json' \
  -d '{"token":"bot_<token>","room_id":1,"content":"Hello from the bot!"}'
```

Bot messages have `"is_bot": true` in the response so clients can tell them apart.

## Reading Messages (Poll)

```bash
curl "http://localhost:8000/api/poll?room_id=1&token=bot_<token>&after_id=0&timeout=30"
```

## Full Python Bot Example

```python
import time
import requests
import json

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
    print("Echo bot running...")
    for msg in poll_messages():
        if msg.get("is_bot"):
            continue  # don't reply to bot messages
        content = msg.get("content", "")
        if content:
            send_message(f"You said: {content}")
```

## WebSocket Bot Example (Python)

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

## Endpoint Reference

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/login` | Login or register a user |
| GET | `/api/rooms?token=` | List all rooms |
| POST | `/api/rooms` | Create a room |
| GET | `/api/history/{id}?token=` | Get room message history |
| POST | `/api/msg` | Send a message (REST) |
| GET | `/api/poll?room_id=&token=&after_id=&timeout=` | Long-poll for new messages |
| WS | `/ws/{room_id}?token=` | WebSocket for real-time messaging |
| POST | `/api/bot/create` | Create a bot token |
| GET | `/api/bot/list?token=` | List your bots |
| DELETE | `/api/bot/{id}?token=` | Delete a bot |

## Tips

- Bot tokens are permanent (unlike session tokens which expire after 24 hours)
- Bots see the same messages as regular users
- Bot messages show a robot emoji in the UI
- Bots cannot create sub-bots
- Poll timeout is 30 seconds; you'll get an empty response if no messages arrive
