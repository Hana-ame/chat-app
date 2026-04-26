import asyncio
import time
from dataclasses import dataclass
from fastapi import WebSocket
from db import db

try:
    import orjson as json_mod
except ImportError:
    import json as json_mod


@dataclass
class WSConnection:
    ws: WebSocket
    user_id: int
    username: str
    is_bot: bool = False


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

    async def ws_join(self, room_id: int, ws: WebSocket, user_id: int,
                      username: str, is_bot: bool = False):
        room = self.get_room(room_id)
        room.ws_connections[ws] = WSConnection(ws, user_id, username, is_bot)
        tag = "[Bot] " if is_bot else ""
        await self._broadcast_system(room, f"{tag}{username} joined the chat")

    async def ws_leave(self, room_id: int, ws: WebSocket):
        room = self.rooms.get(room_id)
        if not room:
            return
        conn = room.ws_connections.pop(ws, None)
        if conn:
            tag = "[Bot] " if conn.is_bot else ""
            await self._broadcast_system(room, f"{tag}{conn.username} left the chat")

    def poll_register(self, room_id: int, client_id: str) -> PollClient:
        room = self.get_room(room_id)
        client = PollClient(event=asyncio.Event(), last_seen_id=room.last_msg_id)
        room.poll_clients[client_id] = client
        return client

    def poll_unregister(self, room_id: int, client_id: str):
        room = self.rooms.get(room_id)
        if room:
            room.poll_clients.pop(client_id, None)

    async def send_message(self, room_id: int, user_id: int, username: str,
                           content: str, msg_type: str = "text",
                           is_bot: bool = False) -> dict:
        msg_id = await db.save_message(room_id, user_id, content, msg_type)
        msg = {
            "id": msg_id,
            "room_id": room_id,
            "user_id": user_id,
            "username": username,
            "content": content,
            "msg_type": msg_type,
            "is_bot": is_bot,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        room = self.get_room(room_id)
        room.last_msg_id = msg_id
        await self._broadcast(room, msg)
        return msg

    async def _broadcast(self, room: Room, msg: dict):
        data = json_mod.dumps(msg) if hasattr(json_mod, 'dumps') else json_mod.dumps(msg)
        if isinstance(data, str):
            data = data.encode()
        dead_ws = [ws for ws in room.ws_connections if not self._safe_send(ws, data)]
        for ws in dead_ws:
            room.ws_connections.pop(ws, None)
        for client in room.poll_clients.values():
            client.event.set()

    async def _broadcast_system(self, room: Room, text: str):
        msg = {
            "type": "system",
            "content": text,
            "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        data = json_mod.dumps(msg) if hasattr(json_mod, 'dumps') else json_mod.dumps(msg)
        if isinstance(data, str):
            data = data.encode()
        dead_ws = [ws for ws in room.ws_connections if not self._safe_send(ws, data)]
        for ws in dead_ws:
            room.ws_connections.pop(ws, None)
        for client in room.poll_clients.values():
            client.event.set()

    def _safe_send(self, ws: WebSocket, data: bytes) -> bool:
        try:
            asyncio.create_task(ws.send_bytes(data))
            return True
        except Exception:
            return False


room_manager = RoomManager()
