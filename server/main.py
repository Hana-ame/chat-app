import asyncio
import uuid
import time
import socket
import random
from contextlib import asynccontextmanager
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Request, Query
from fastapi.responses import JSONResponse, FileResponse
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware

from db import db
from room_manager import room_manager

try:
    import orjson as json_mod
except ImportError:
    import json as json_mod

@asynccontextmanager
async def lifespan(app: FastAPI):
    await db.init()
    await load_bot_cache()
    cleanup = asyncio.create_task(_cleanup_task())
    yield
    cleanup.cancel()
    await db.close()

async def load_bot_cache():
    bots = await db.load_all_bot_tokens()
    for b in bots:
        bot_tokens_cache[b["token"]] = {
            "user_id": b["user_id"],
            "username": b["username"],
            "avatar_color": b["avatar_color"],
            "bot_id": b["bot_id"],
            "bot_name": b["bot_name"],
            "is_bot": True,
        }

async def _cleanup_task():
    while True:
        await asyncio.sleep(600)
        expired = [t for t, v in active_tokens.items() if time.time() - v.get("ts", 0) > 86400]
        for t in expired:
            active_tokens.pop(t, None)

app = FastAPI(lifespan=lifespan)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

STATIC_DIR = "../client/dist"
import os as _os
if not _os.path.isdir(STATIC_DIR):
    STATIC_DIR = "client/dist"
app.mount("/assets", StaticFiles(directory=f"{STATIC_DIR}/assets"), name="assets")

active_tokens: dict[str, dict] = {}
bot_tokens_cache: dict[str, dict] = {}

def get_user(token: str) -> dict | None:
    """Resolve token from sessions or bot cache."""
    if not token:
        return None
    u = active_tokens.get(token)
    if u:
        return {**u, "is_bot": False}
    b = bot_tokens_cache.get(token)
    if b:
        return b
    return None

# ── Login ──────────────────────────────────────────────────────────────

COLORS = ["#e74c3c", "#3498db", "#2ecc71", "#f39c12", "#9b59b6", "#1abc9c"]

@app.post("/api/login")
async def login(request: Request):
    data = await request.json()
    username = data.get("username", "").strip()[:20]
    password = data.get("password", "")
    if not username:
        return JSONResponse({"error": "username is required"}, 400)

    user = await db.fetch_one(
        "SELECT id, username, password, avatar_color FROM users WHERE username = ?",
        (username,),
    )

    if user and user["password"] == password:
        pass
    elif not user:
        color = random.choice(COLORS)
        future = await db.execute_write(
            "INSERT INTO users (username, password, avatar_color) VALUES (?, ?, ?)",
            (username, password, color),
        )
        uid = await future
        user = {"id": uid, "username": username, "avatar_color": color}
    else:
        return JSONResponse({"error": "wrong password"}, 401)

    token = uuid.uuid4().hex
    active_tokens[token] = {
        "user_id": user["id"],
        "username": user["username"],
        "avatar_color": user["avatar_color"],
        "ts": time.time(),
    }
    return {
        "token": token,
        "user_id": user["id"],
        "username": user["username"],
        "avatar_color": user["avatar_color"],
    }

# ── Rooms ──────────────────────────────────────────────────────────────

@app.get("/api/rooms")
async def list_rooms(token: str = Query(...)):
    user = get_user(token)
    if not user:
        return JSONResponse({"error": "unauthorized"}, 401)
    rooms = await db.get_rooms()
    return {"rooms": rooms}

@app.post("/api/rooms")
async def create_room(request: Request):
    data = await request.json()
    user = get_user(data.get("token"))
    if not user:
        return JSONResponse({"error": "unauthorized"}, 401)
    name = data.get("name", "").strip()[:50]
    if not name:
        return JSONResponse({"error": "room name is required"}, 400)
    desc = data.get("description", "").strip()[:200]
    try:
        rid = await db.create_room(name, desc)
        return {"id": rid, "name": name, "description": desc}
    except Exception:
        return JSONResponse({"error": "room already exists"}, 409)

# ── Messages ───────────────────────────────────────────────────────────

@app.get("/api/history/{room_id}")
async def get_history(room_id: int, token: str = Query(...),
                      after_id: int = 0, limit: int = 50):
    user = get_user(token)
    if not user:
        return JSONResponse({"error": "unauthorized"}, 401)
    msgs = await db.get_messages(room_id, after_id, limit)
    for m in msgs:
        b = bot_tokens_cache.get(m.get("token", ""))
        m["is_bot"] = bool(b)
    return {"messages": msgs}

@app.post("/api/msg")
async def send_message_rest(request: Request):
    data = await request.json()
    user = get_user(data.get("token"))
    if not user:
        return JSONResponse({"error": "unauthorized"}, 401)
    content = data.get("content", "").strip()[:2000]
    msg_type = data.get("msg_type", "text")
    if not content:
        return JSONResponse({"error": "empty message"}, 400)
    room_id = data.get("room_id", 1)
    display_name = user.get("bot_name", user["username"])
    msg = await room_manager.send_message(
        room_id, user["user_id"], display_name, content,
        msg_type=msg_type, is_bot=user.get("is_bot", False)
    )
    return msg

# ── Bots ───────────────────────────────────────────────────────────────

@app.post("/api/bot/create")
async def create_bot(request: Request):
    data = await request.json()
    user = get_user(data.get("token"))
    if not user or user.get("is_bot"):
        return JSONResponse({"error": "unauthorized or bot cannot create bots"}, 401)
    name = data.get("name", "").strip()[:30]
    if not name:
        return JSONResponse({"error": "bot name is required"}, 400)

    token = "bot_" + uuid.uuid4().hex
    bid = await db.create_bot_token(user["user_id"], name, token)

    entry = {
        "user_id": user["user_id"],
        "username": user["username"],
        "avatar_color": user["avatar_color"],
        "bot_id": bid,
        "bot_name": name,
        "is_bot": True,
    }
    bot_tokens_cache[token] = entry
    return {"id": bid, "name": name, "token": token}

@app.get("/api/bot/list")
async def list_bots(token: str = Query(...)):
    user = get_user(token)
    if not user or user.get("is_bot"):
        return JSONResponse({"error": "unauthorized"}, 401)
    bots = await db.list_user_bots(user["user_id"])
    return {"bots": bots}

@app.delete("/api/bot/{bot_id}")
async def delete_bot(bot_id: int, token: str = Query(...)):
    user = get_user(token)
    if not user or user.get("is_bot"):
        return JSONResponse({"error": "unauthorized"}, 401)
    ok = await db.delete_bot(bot_id, user["user_id"])
    if not ok:
        return JSONResponse({"error": "bot not found"}, 404)
    for t, v in list(bot_tokens_cache.items()):
        if v.get("bot_id") == bot_id:
            del bot_tokens_cache[t]
    return {"ok": True}

# ── WebSocket ──────────────────────────────────────────────────────────

@app.websocket("/ws/{room_id}")
async def ws_endpoint(ws: WebSocket, room_id: int):
    token = ws.query_params.get("token")
    user = get_user(token) if token else None
    if not user:
        await ws.close(code=4001, reason="unauthorized")
        return

    await ws.accept()

    transport = getattr(ws, 'transport', None)
    if transport:
        sock = transport.get_extra_info("socket")
        if sock:
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_KEEPALIVE, 1)
            try:
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPIDLE, 60)
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPINTVL, 10)
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPCNT, 3)
            except (AttributeError, OSError):
                pass

    display_name = user.get("bot_name", user["username"])
    is_bot = user.get("is_bot", False)
    await room_manager.ws_join(room_id, ws, user["user_id"], display_name, is_bot)

    try:
        while True:
            raw = await asyncio.wait_for(ws.receive_text(), timeout=300)
            try:
                msg = json_mod.loads(raw)
            except Exception:
                continue

            if msg.get("type") == "ping":
                await ws.send_json({"type": "pong"})
            elif msg.get("type") == "message":
                content = msg.get("content", "").strip()[:2000]
                msg_type = msg.get("msg_type", "text")
                if content:
                    await room_manager.send_message(
                        room_id, user["user_id"], display_name, content,
                        msg_type=msg_type, is_bot=is_bot,
                    )
    except (asyncio.TimeoutError, WebSocketDisconnect, Exception):
        pass
    finally:
        await room_manager.ws_leave(room_id, ws)

# ── Long Poll ──────────────────────────────────────────────────────────

@app.get("/api/poll")
async def long_poll(room_id: int, token: str, after_id: int = 0, timeout: int = 30):
    user = get_user(token)
    if not user:
        return JSONResponse({"error": "unauthorized"}, 401)

    msgs = await db.get_messages(room_id, after_id, limit=50)
    if msgs:
        return {"messages": msgs, "last_id": msgs[-1]["id"]}

    client_id = uuid.uuid4().hex
    client = room_manager.poll_register(room_id, client_id)
    try:
        await asyncio.wait_for(client.event.wait(), timeout=min(timeout, 60))
        msgs = await db.get_messages(room_id, after_id, limit=50)
        return {"messages": msgs, "last_id": msgs[-1]["id"] if msgs else after_id}
    except asyncio.TimeoutError:
        return {"messages": [], "last_id": after_id}
    finally:
        room_manager.poll_unregister(room_id, client_id)

# ── SPA fallback ───────────────────────────────────────────────────────


@app.get("/")
async def serve_index():
    return FileResponse(f"{STATIC_DIR}/index.html")


@app.get("/{full_path:path}")
async def serve_spa(full_path: str):
    fp = f"{STATIC_DIR}/{full_path}"
    if _os.path.isfile(fp):
        return FileResponse(fp)
    return FileResponse(f"{STATIC_DIR}/index.html")
