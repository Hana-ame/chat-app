import asyncio
import uuid
import time
import socket
from contextlib import asynccontextmanager
from fastapi import FastAPI, WebSocket, WebSocketDisconnect, Request
from fastapi.responses import JSONResponse
from fastapi.staticfiles import StaticFiles

from db import db
from room_manager import room_manager

@asynccontextmanager
async def lifespan(app: FastAPI):
    await db.init()
    cleanup = asyncio.create_task(cleanup_task())
    yield
    cleanup.cancel()
    await db.close()

async def cleanup_task():
    while True:
        await asyncio.sleep(600)
        expired = [t for t, v in active_tokens.items() if time.time() - v.get("ts", 0) > 86400]
        for t in expired: active_tokens.pop(t, None)
        empty = [rid for rid, room in room_manager.rooms.items() if room.online_count == 0]
        for rid in empty: room_manager.rooms.pop(rid, None)

app = FastAPI(lifespan=lifespan)

# 生产环境托管前端 build 产物
app.mount("/assets", StaticFiles(directory="../client/dist/assets"), name="assets")

active_tokens: dict[str, dict] = {}

@app.post("/api/login")
async def login(request: Request):
    data = await request.json()
    username = data.get("username", "").strip()[:20]
    password = data.get("password", "")
    if not username: return JSONResponse({"error": "用户名不能为空"}, 400)

    user = await db.fetch_one("SELECT id, username, password, avatar_color FROM users WHERE username = ?", (username,))
    if user and user["password"] == password:
        pass
    elif not user:
        import random
        colors = ["#e74c3c", "#3498db", "#2ecc71", "#f39c12", "#9b59b6", "#1abc9c"]
        future = await db.execute_write("INSERT INTO users (username, password, avatar_color) VALUES (?, ?, ?)", (username, password, random.choice(colors)))
        uid = await future
        user = {"id": uid, "username": username, "avatar_color": colors[0]}
    else:
        return JSONResponse({"error": "密码错误"}, 401)

    token = uuid.uuid4().hex
    active_tokens[token] = {"user_id": user["id"], "username": user["username"], "avatar_color": user["avatar_color"], "ts": time.time()}
    return {"token": token, "user_id": user["id"], "username": user["username"], "avatar_color": user["avatar_color"]}

def get_user(token: str) -> dict | None:
    return active_tokens.get(token)

@app.get("/api/history/{room_id}")
async def get_history(room_id: int, after_id: int = 0, limit: int = 50):
    msgs = await db.get_messages(room_id, after_id, limit)
    return {"messages": msgs}

@app.websocket("/ws/{room_id}")
async def ws_endpoint(ws: WebSocket, room_id: int):
    token = ws.query_params.get("token")
    user = get_user(token) if token else None
    if not user:
        await ws.close(code=4001, reason="未认证"); return

    await ws.accept()
    
    # TCP Keepalive (Doze 友好)
    transport = ws.transport
    if transport:
        sock = transport.get_extra_info("socket")
        if sock:
            sock.setsockopt(socket.SOL_SOCKET, socket.SO_KEEPALIVE, 1)
            try:
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPIDLE, 60)
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPINTVL, 10)
                sock.setsockopt(socket.IPPROTO_TCP, socket.TCP_KEEPCNT, 3)
            except (AttributeError, OSError): pass

    await room_manager.ws_join(room_id, ws, user["user_id"], user["username"])
    try:
        while True:
            data = await asyncio.wait_for(ws.receive_text(), timeout=300) # 5分钟无活动断开
            try:
                msg = __import__('orjson').loads(data)
            except: continue

            if msg.get("type") == "ping":
                await ws.send_json({"type": "pong"})
            elif msg.get("type") == "message":
                content = msg.get("content", "").strip()[:500]
                if content: await room_manager.send_message(room_id, user["user_id"], user["username"], content)
    except (asyncio.TimeoutError, WebSocketDisconnect, Exception): pass
    finally:
        await room_manager.ws_leave(room_id, ws)

@app.get("/api/poll")
async def long_poll(room_id: int, token: str, after_id: int = 0, timeout: int = 30):
    user = get_user(token)
    if not user: return JSONResponse({"error": "未认证"}, 401)

    msgs = await db.get_messages(room_id, after_id, limit=50)
    if msgs: return {"messages": msgs, "last_id": msgs[-1]["id"]}

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

@app.post("/api/msg")
async def send_message(request: Request):
    data = await request.json()
    user = get_user(data.get("token"))
    if not user: return JSONResponse({"error": "未认证"}, 401)
    content = data.get("content", "").strip()[:500]
    if not content: return JSONResponse({"error": "空消息"}, 400)
    msg = await room_manager.send_message(data.get("room_id", 1), user["user_id"], user["username"], content)
    return msg

# 前端页面路由（必须放最后）
from fastapi.responses import FileResponse
@app.get("/")
async def serve_index():
    return FileResponse("../client/dist/index.html")