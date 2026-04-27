import os
import asyncio
import aiosqlite
from pathlib import Path
from dataclasses import dataclass

DB_PATH = Path(os.environ.get("CHAT_DB_PATH", str(Path(__file__).parent / "chat.db")))
SCHEMA_PATH = Path(__file__).parent / "schema.sql"

@dataclass
class PendingWrite:
    sql: str
    params: tuple
    future: asyncio.Future

class Database:
    def __init__(self):
        self._db = None
        self._queue: asyncio.Queue[PendingWrite] = asyncio.Queue()
        self._flush_task = None
        self._max_batch = 50
        self._flush_interval = 0.2

    async def init(self):
        if self._db is not None:
            return
        self._db = await aiosqlite.connect(str(DB_PATH))
        self._db.row_factory = aiosqlite.Row
        with open(SCHEMA_PATH) as f:
            await self._db.executescript(f.read())
        await self._db.commit()
        self._flush_task = asyncio.create_task(self._flush_loop())

    async def close(self):
        if self._flush_task:
            self._flush_task.cancel()
        await self._drain()
        if self._db:
            await self._db.close()

    async def fetch_one(self, sql: str, params: tuple = ()) -> dict | None:
        cursor = await self._db.execute(sql, params)
        row = await cursor.fetchone()
        return dict(row) if row else None

    async def fetch_all(self, sql: str, params: tuple = ()) -> list[dict]:
        cursor = await self._db.execute(sql, params)
        rows = await cursor.fetchall()
        return [dict(r) for r in rows]

    async def execute_write(self, sql: str, params: tuple = ()) -> asyncio.Future:
        loop = asyncio.get_running_loop()
        future = loop.create_future()
        await self._queue.put(PendingWrite(sql, params, future))
        return future

    async def save_message(self, room_id: int, user_id: int, content: str, msg_type: str = "text") -> int:
        future = await self.execute_write(
            "INSERT INTO messages (room_id, user_id, content, msg_type) VALUES (?, ?, ?, ?)",
            (room_id, user_id, content, msg_type)
        )
        return await future

    async def get_messages(self, room_id: int, after_id: int = 0, limit: int = 50) -> list[dict]:
        return await self.fetch_all(
            "SELECT m.id, m.user_id, m.content, m.msg_type, m.created_at, u.username, u.avatar_color "
            "FROM messages m JOIN users u ON m.user_id = u.id "
            "WHERE m.room_id = ? AND m.id > ? ORDER BY m.id ASC LIMIT ?",
            (room_id, after_id, limit)
        )

    async def create_bot_token(self, user_id: int, name: str, token: str) -> int:
        future = await self.execute_write(
            "INSERT INTO bot_tokens (user_id, name, token) VALUES (?, ?, ?)",
            (user_id, name, token)
        )
        return await future

    async def get_bot_by_token(self, token: str) -> dict | None:
        row = await self.fetch_one(
            "SELECT bt.id as bot_id, bt.name as bot_name, bt.token, bt.user_id, "
            "u.username, u.avatar_color "
            "FROM bot_tokens bt JOIN users u ON bt.user_id = u.id "
            "WHERE bt.token = ?", (token,)
        )
        return dict(row) if row else None

    async def list_user_bots(self, user_id: int) -> list[dict]:
        return await self.fetch_all(
            "SELECT id, name, token, created_at FROM bot_tokens WHERE user_id = ?",
            (user_id,)
        )

    async def delete_bot(self, bot_id: int, user_id: int) -> bool:
        future = await self.execute_write(
            "DELETE FROM bot_tokens WHERE id = ? AND user_id = ?",
            (bot_id, user_id)
        )
        result = await future
        cursor = await self._db.execute("SELECT changes()")
        row = await cursor.fetchone()
        return row[0] > 0

    async def load_all_bot_tokens(self) -> list[dict]:
        return await self.fetch_all(
            "SELECT bt.id as bot_id, bt.name as bot_name, bt.token, bt.user_id, "
            "u.username, u.avatar_color "
            "FROM bot_tokens bt JOIN users u ON bt.user_id = u.id"
        )

    async def get_rooms(self) -> list[dict]:
        return await self.fetch_all(
            "SELECT id, name, description, created_at FROM rooms ORDER BY id"
        )

    async def create_room(self, name: str, description: str = "") -> int:
        future = await self.execute_write(
            "INSERT INTO rooms (name, description) VALUES (?, ?)",
            (name, description)
        )
        return await future

    async def _flush_loop(self):
        while True:
            try:
                batch = []
                first = await self._queue.get()
                batch.append(first)

                deadline = asyncio.get_event_loop().time() + self._flush_interval
                while len(batch) < self._max_batch:
                    remaining = deadline - asyncio.get_event_loop().time()
                    if remaining <= 0:
                        break
                    try:
                        item = await asyncio.wait_for(self._queue.get(), timeout=remaining)
                        batch.append(item)
                    except asyncio.TimeoutError:
                        break

                await self._execute_batch(batch)
            except asyncio.CancelledError:
                break
            except Exception as e:
                for pw in batch:
                    if not pw.future.done():
                        pw.future.set_exception(e)

    async def _execute_batch(self, batch: list[PendingWrite]):
        try:
            lastrowids = []
            for pw in batch:
                cursor = await self._db.execute(pw.sql, pw.params)
                lastrowids.append(cursor.lastrowid)
            await self._db.commit()

            for pw, rowid in zip(batch, lastrowids):
                if not pw.future.done():
                    pw.future.set_result(rowid)
        except Exception as e:
            await self._db.rollback()
            for pw in batch:
                if not pw.future.done():
                    pw.future.set_exception(e)

    async def _drain(self):
        while not self._queue.empty():
            batch = []
            while not self._queue.empty() and len(batch) < self._max_batch:
                try:
                    batch.append(self._queue.get_nowait())
                except asyncio.QueueEmpty:
                    break
            if batch:
                await self._execute_batch(batch)

db = Database()
