import os
import sys
import tempfile
import uuid

test_db = os.path.join(tempfile.gettempdir(), f"test_chat_{uuid.uuid4().hex}.db")
os.environ["CHAT_DB_PATH"] = test_db

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import pytest
from starlette.testclient import TestClient as _TestClient


@pytest.fixture(scope="session")
def client():
    from main import app
    with _TestClient(app) as c:
        yield c


def test_login_register(client):
    r = client.post("/api/login", json={"username": "alice", "password": "123"})
    assert r.status_code == 200
    data = r.json()
    assert "token" in data
    assert data["username"] == "alice"

def test_login_existing(client):
    r = client.post("/api/login", json={"username": "bob", "password": "456"})
    assert r.status_code == 200
    r2 = client.post("/api/login", json={"username": "bob", "password": "456"})
    assert r2.status_code == 200

def test_login_wrong_password(client):
    client.post("/api/login", json={"username": "carol", "password": "pw1"})
    r = client.post("/api/login", json={"username": "carol", "password": "wrong"})
    assert r.status_code == 401

def test_login_empty_username(client):
    r = client.post("/api/login", json={"username": "", "password": "x"})
    assert r.status_code == 400

def test_rooms_list(client):
    r = client.post("/api/login", json={"username": "dave", "password": "x"})
    token = r.json()["token"]
    r2 = client.get("/api/rooms", params={"token": token})
    assert r2.status_code == 200
    rooms = r2.json()["rooms"]
    assert any(r["name"] == "大厅" for r in rooms)

def test_rooms_list_unauthorized(client):
    r = client.get("/api/rooms", params={"token": "bad"})
    assert r.status_code == 401

def test_create_room(client):
    r = client.post("/api/login", json={"username": "eve", "password": "x"})
    token = r.json()["token"]
    r2 = client.post("/api/rooms", json={"token": token, "name": "my-room"})
    assert r2.status_code == 200

def test_create_room_duplicate(client):
    r = client.post("/api/login", json={"username": "frank", "password": "x"})
    token = r.json()["token"]
    client.post("/api/rooms", json={"token": token, "name": "secret"})
    r2 = client.post("/api/rooms", json={"token": token, "name": "secret"})
    assert r2.status_code == 409

def test_send_message_rest(client):
    r = client.post("/api/login", json={"username": "grace", "password": "x"})
    token = r.json()["token"]
    r2 = client.post("/api/msg", json={
        "token": token, "room_id": 1, "content": "hello world"
    })
    assert r2.status_code == 200
    assert r2.json()["content"] == "hello world"

def test_history(client):
    r = client.post("/api/login", json={"username": "heidi", "password": "x"})
    token = r.json()["token"]
    client.post("/api/msg", json={"token": token, "room_id": 1, "content": "msg1"})
    client.post("/api/msg", json={"token": token, "room_id": 1, "content": "msg2"})

    r2 = client.get("/api/history/1", params={"token": token, "limit": 50})
    assert r2.status_code == 200
    msgs = r2.json()["messages"]
    texts = [m["content"] for m in msgs if m.get("username") == "heidi"]
    assert "msg1" in texts and "msg2" in texts

def test_bot_create_list_delete(client):
    r = client.post("/api/login", json={"username": "ivan", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/bot/create", json={"token": token, "name": "MyBot"})
    assert r2.status_code == 200
    bot = r2.json()
    assert bot["name"] == "MyBot"
    assert bot["token"].startswith("bot_")

    r3 = client.get("/api/bot/list", params={"token": token})
    assert r3.status_code == 200
    assert len(r3.json()["bots"]) == 1

    r4 = client.delete(f"/api/bot/{bot['id']}", params={"token": token})
    assert r4.status_code == 200

    r5 = client.get("/api/bot/list", params={"token": token})
    assert len(r5.json()["bots"]) == 0

def test_bot_auth_denied(client):
    r = client.post("/api/login", json={"username": "judy", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/bot/create", json={"token": token, "name": "Helper"})
    bot_token = r2.json()["token"]

    r3 = client.get("/api/rooms", params={"token": "bad_token"})
    assert r3.status_code == 401

    r4 = client.get("/api/rooms", params={"token": bot_token})
    assert r4.status_code == 200

def test_bot_send_message(client):
    r = client.post("/api/login", json={"username": "ken", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/bot/create", json={"token": token, "name": "EchoBot"})
    bot_token = r2.json()["token"]

    r3 = client.post("/api/msg", json={
        "token": bot_token, "room_id": 1, "content": "beep boop"
    })
    assert r3.status_code == 200
    msg = r3.json()
    assert msg["is_bot"] is True
    assert msg["username"] == "EchoBot"

def test_bot_cannot_create_bot(client):
    r = client.post("/api/login", json={"username": "leo", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/bot/create", json={"token": token, "name": "B1"})
    bot_token = r2.json()["token"]

    r3 = client.post("/api/bot/create", json={"token": bot_token, "name": "B2"})
    assert r3.status_code == 401

def test_bot_delete_removes_from_cache(client):
    r = client.post("/api/login", json={"username": "mia", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/bot/create", json={"token": token, "name": "B"})
    bot_token = r2.json()["token"]
    bot_id = r2.json()["id"]

    r3 = client.get("/api/rooms", params={"token": bot_token})
    assert r3.status_code == 200

    client.delete(f"/api/bot/{bot_id}", params={"token": token})

    r4 = client.get("/api/rooms", params={"token": bot_token})
    assert r4.status_code == 401

def test_empty_message_rejected(client):
    r = client.post("/api/login", json={"username": "nora", "password": "x"})
    token = r.json()["token"]

    r2 = client.post("/api/msg", json={"token": token, "room_id": 1, "content": ""})
    assert r2.status_code == 400

def test_long_poll_timeout(client):
    r = client.post("/api/login", json={"username": "owen", "password": "x"})
    token = r.json()["token"]

    r2 = client.get("/api/poll", params={
        "room_id": 1, "token": token, "after_id": 0, "timeout": 1
    })
    assert r2.status_code == 200
    assert "messages" in r2.json()


# ── WebSocket integration ────────────────────────────────────────────────────

def test_ws_send_receive(client):
    r = client.post("/api/login", json={"username": "wx1", "password": "x"})
    token = r.json()["token"]

    with client.websocket_connect(f"/ws/1?token={token}") as ws:
        # 收到进房系统消息
        data = ws.receive_json()
        assert data["type"] == "system"

        # 发送消息
        ws.send_json({"type": "message", "content": "hello ws"})

        # 收到自己发的那条消息（广播回环）
        data = ws.receive_json()
        assert data["content"] == "hello ws"
        assert data["username"] == "wx1"

def test_ws_binary_not_sent(client):
    """确认服务端发的是文本帧而非二进制帧"""
    r = client.post("/api/login", json={"username": "wx2", "password": "x"})
    token = r.json()["token"]

    with client.websocket_connect(f"/ws/1?token={token}") as ws:
        ws.receive_json()  # 系统消息

        ws.send_json({"type": "message", "content": "text only"})

        # receive_json 内部只接受 text 帧，如果是 binary 会 TypeError
        data = ws.receive_json()
        assert data["content"] == "text only"

def test_ws_ping_pong(client):
    r = client.post("/api/login", json={"username": "wx3", "password": "x"})
    token = r.json()["token"]

    with client.websocket_connect(f"/ws/1?token={token}") as ws:
        ws.receive_json()  # 系统消息

        ws.send_json({"type": "ping"})
        data = ws.receive_json()
        assert data["type"] == "pong"

def test_ws_multiple(client):
    """两个连接互相收发消息"""
    r1 = client.post("/api/login", json={"username": "wx4", "password": "x"})
    t1 = r1.json()["token"]
    r2 = client.post("/api/login", json={"username": "wx5", "password": "x"})
    t2 = r2.json()["token"]

    with client.websocket_connect(f"/ws/1?token={t1}") as ws1:
        assert ws1.receive_json()["type"] == "system"  # wx4 joined

        with client.websocket_connect(f"/ws/1?token={t2}") as ws2:
            assert ws2.receive_json()["type"] == "system"  # wx5 joined
            assert ws1.receive_json()["type"] == "system"  # wx1 sees wx5 joined

            # wx5 发消息，双方都会收到（广播回环）
            ws2.send_json({"type": "message", "content": "from wx5"})
            # ws1 收到
            data = ws1.receive_json()
            assert data["content"] == "from wx5"
            assert data["username"] == "wx5"
            # ws2 也收到自己的（自环广播）
            echo = ws2.receive_json()
            assert echo["content"] == "from wx5"

            # wx4 发消息
            ws1.send_json({"type": "message", "content": "from wx4"})
            data = ws2.receive_json()
            assert data["content"] == "from wx4"
            assert data["username"] == "wx4"


# ── CORS tests ──────────────────────────────────────────────────────────────

def _assert_cors(response):
    """Every response must include Access-Control-Allow-Origin: *"""
    assert response.headers.get("access-control-allow-origin") == "*"

def test_cors_login_preflight(client):
    r = client.options("/api/login", headers={
        "origin": "https://example.com",
        "access-control-request-method": "POST",
    })
    assert r.status_code == 200
    assert r.headers["access-control-allow-origin"] == "*"
    assert r.headers["access-control-allow-methods"] is not None

def test_cors_login_normal(client):
    r = client.post("/api/login", json={"username": "corser1", "password": "x"},
                    headers={"origin": "https://evil.com"})
    assert r.status_code == 200
    _assert_cors(r)

def test_cors_rooms(client):
    r = client.post("/api/login", json={"username": "corser2", "password": "x"},
                    headers={"origin": "https://other.com"})
    token = r.json()["token"]
    r2 = client.get("/api/rooms", params={"token": token},
                    headers={"origin": "https://other.com"})
    _assert_cors(r2)

def test_cors_bot(client):
    r = client.post("/api/login", json={"username": "corser3", "password": "x"})
    token = r.json()["token"]
    r2 = client.post("/api/bot/create", json={"token": token, "name": "CorsBot"},
                     headers={"origin": "https://cross.example"})
    _assert_cors(r2)

def test_cors_msg(client):
    r = client.post("/api/login", json={"username": "corser4", "password": "x"})
    token = r.json()["token"]
    r2 = client.post("/api/msg", json={"token": token, "room_id": 1, "content": "cors!"},
                     headers={"origin": "https://abc.com"})
    _assert_cors(r2)

def test_cors_history(client):
    r = client.post("/api/login", json={"username": "corser5", "password": "x"})
    token = r.json()["token"]
    r2 = client.get("/api/history/1", params={"token": token},
                    headers={"origin": "https://foo.bar"})
    _assert_cors(r2)

def test_cors_poll(client):
    r = client.post("/api/login", json={"username": "corser6", "password": "x"})
    token = r.json()["token"]
    r2 = client.get("/api/poll", params={"room_id": 1, "token": token, "timeout": 1},
                    headers={"origin": "https://poll.cors"})
    _assert_cors(r2)

def test_cors_unauthorized(client):
    r = client.get("/api/rooms", params={"token": "bad"},
                   headers={"origin": "https://attacker.com"})
    _assert_cors(r)  # 即使 401 也要有 CORS 头
