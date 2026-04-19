import asyncio
import orjson
import time
from dataclasses import dataclass
from fastapi import WebSocket
from db import db

@dataclass
class WSConnection:
    ws: WebSocket
    user_id: int
    username: str

@dataclass
class PollClient:
    event: asyncio.Event
    last_seen_id: int = 0

class Room:
    def __init__(self, room_id: int):
        self.room_id = room_id
        self.ws_connections: dict[WebSocket, WSConnection] = {}
        self.poll_clients: dict[str, PollClient] = {}
        self.last_msg_id: int = 0

    @property
    def online_count(self) -> int:
        return len(self.ws_connections) + len(self.poll_clients)

class RoomManager:
    def __init__(self):
        self.rooms: dict[int, Room] = {}

    def get_room(self, room_id: int) -> Room:
        if room_id not in self.rooms:
            self.rooms[room_id] = Room(room_id)
        return self.rooms[room_id]

    async def ws_join(self, room_id: int, ws: WebSocket, user_id: int, username: str):
        room = self.get_room(room_id)
        room.ws_connections[ws] = WSConnection(ws, user_id, username)
        await self._broadcast_system(room, f"🟢 {username} 加入了聊天")
        await self._send_online_count(room)

    async def ws_leave(self, room_id: int, ws: WebSocket):
        room = self.rooms.get(room_id)
        if not room: return
        conn = room.ws_connections.pop(ws, None)
        if conn:
            await self._broadcast_system(room, f"🔴 {conn.username} 离开了聊天")
            await self._send_online_count(room)

    def poll_register(self, room_id: int, client_id: str) -> PollClient:
        room = self.get_room(room_id)
        client = PollClient(event=asyncio.Event(), last_seen_id=room.last_msg_id)
        room.poll_clients[client_id] = client
        return client

    def poll_unregister(self, room_id: int, client_id: str):
        room = self.rooms.get(room_id)
        if room: room.poll_clients.pop(client_id, None)

    async def send_message(self, room_id: int, user_id: int, username: str, content: str, msg_type: str = "text") -> dict:
        msg_id = await db.save_message(room_id, user_id, content, msg_type)
        msg = {
            "id": msg_id, "room_id": room_id, "user_id": user_id,
            "username": username, "content": content, "msg_type": msg_type,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        room = self.get_room(room_id)
        room.last_msg_id = msg_id
        await self._broadcast(room, msg)
        return msg

    async def _broadcast(self, room: Room, msg: dict):
        data = orjson.dumps(msg)
        dead_ws = [ws for ws, conn in room.ws_connections.items() if not self._safe_send(ws, data)]
        for ws in dead_ws: room.ws_connections.pop(ws, None)
        for client in room.poll_clients.values(): client.event.set()

    async def _broadcast_system(self, room: Room, text: str):
        msg = {"type": "system", "content": text, "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}
        data = orjson.dumps(msg)
        dead_ws = [ws for ws in room.ws_connections.keys() if not self._safe_send(ws, data)]
        for ws in dead_ws: room.ws_connections.pop(ws, None)
        for client in room.poll_clients.values(): client.event.set()

    async def _send_online_count(self, room: Room):
        msg = {"type": "online_count", "count": room.online_count}
        data = orjson.dumps(msg)
        for ws in list(room.ws_connections.keys()): self._safe_send(ws, data)

    def _safe_send(self, ws: WebSocket, data: bytes) -> bool:
        try:
            asyncio.create_task(ws.send_bytes(data))
            return True
        except Exception:
            return False

room_manager = RoomManager()