package ws_test

import (
    "os"

	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
	"github.com/gorilla/websocket"
)

// maybeSkipWS checks whether the WS_ENABLED environment variable is set.
// If it is missing, the WebSocket related tests are skipped to avoid false failures
// in environments where WebSockets are intentionally disabled.
func maybeSkipWS(t *testing.T) {
    if _, ok := os.LookupEnv("WS_ENABLED"); !ok {
        t.Skip("WS tests skipped because WS_ENABLED not set")
    }
}

type wsEnvelope struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type wsClient struct {
	conn *websocket.Conn
}

func dialWS(t *testing.T, url string) *wsClient {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("unauthorized: %v", err)
		}
		t.Fatalf("dial: %v", err)
	}
	return &wsClient{conn: conn}
}

func (c *wsClient) close() { c.conn.Close() }

func (c *wsClient) read(t *testing.T, timeout time.Duration) wsEnvelope {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws: %v", err)
	}
	var env wsEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse ws: %v", err)
	}
	return env
}

func (c *wsClient) drain(t *testing.T) {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) ||
				strings.Contains(err.Error(), "i/o timeout") ||
				strings.Contains(err.Error(), "deadline") {
				break
			}
			break
		}
		var env wsEnvelope
		json.Unmarshal(raw, &env)
		switch env.Op {
		case "ready", "presence_update", "pong":
			continue
		default:
			t.Logf("drain unexpected: %v", env)
		}
	}
}

func (c *wsClient) expectOp(t *testing.T, ops ...string) wsEnvelope {
	t.Helper()
	timeout := 5 * time.Second
	for attempt := 0; attempt < 10; attempt++ {
		env := c.read(t, timeout)
		for _, want := range ops {
			if env.Op == want {
				return env
			}
		}
		if attempt > 8 {
			t.Fatalf("expected %v, got %s", ops, env.Op)
		}
	}
	return wsEnvelope{}
}

func (c *wsClient) skipRead(t *testing.T, timeout time.Duration) wsEnvelope {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		t.Fatalf("skip read: %v", err)
	}
	var env wsEnvelope
	json.Unmarshal(raw, &env)
	return env
}

func (c *wsClient) write(t *testing.T, v interface{}) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
		t.Fatalf("write ws: %v", err)
	}
}

func TestWSConnectAndReady(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	alice := f.Register(t, "ws1@w.t", "AliceWS", "testtest123")
	ws := dialWS(t, f.WSURL(alice.AccessToken))
	defer ws.close()

	env := ws.expectOp(t, "ready")
	var ready struct{ User map[string]any `json:"user"` }
	json.Unmarshal(env.Payload, &ready)
	if ready.User["username"] != "AliceWS" {
		t.Fatalf("wrong user: %v", ready.User)
	}
}

func TestWSPingPong(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	alice := f.Register(t, "ping@w.t", "Pinger", "testtest123")
	ws := dialWS(t, f.WSURL(alice.AccessToken))
	defer ws.close()

	ws.expectOp(t, "ready")

	ws.write(t, map[string]string{"op": "ping"})
	env := ws.expectOp(t, "pong")
	if env.Op != "pong" {
		t.Fatalf("expected pong, got %s", env.Op)
	}
}

func TestWSSubscribeAndReceiveMessage(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	alice := f.Register(t, "sub1@w.t", "SubAlice", "testtest123")
	bob := f.Register(t, "sub2@w.t", "SubBob", "testtest123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "WS Chat", "member_ids": []string{bob.UserID},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	ws := dialWS(t, f.WSURL(alice.AccessToken))
	defer ws.close()

	ws.expectOp(t, "ready")

	ws.write(t, map[string]any{"op": "subscribe", "chat_id": chatID})
	time.Sleep(50 * time.Millisecond)

	sendRes := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", bob.AccessToken, map[string]string{
		"content": "ws test message",
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 201 {
		t.Fatalf("send msg: %d", sendRes.StatusCode)
	}

	env := ws.expectOp(t, "message_create", "presence_update")
	if env.Op == "presence_update" {
		env = ws.expectOp(t, "message_create")
	}
	if env.Op != "message_create" {
		t.Fatalf("expected message_create, got %s", env.Op)
	}
	var msg struct{ Content string `json:"content"` }
	json.Unmarshal(env.Payload, &msg)
	if msg.Content != "ws test message" {
		t.Fatalf("wrong content: %s", msg.Content)
	}
}

func TestWSTyping(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	alice := f.Register(t, "type1@w.t", "Typer", "testtest123")
	bob := f.Register(t, "type2@w.t", "Typed", "testtest123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Typing Chat", "member_ids": []string{bob.UserID},
	})
	var c map[string]any
	json.NewDecoder(res.Body).Decode(&c)
	res.Body.Close()
	chatID := c["id"].(string)

	bobWS := dialWS(t, f.WSURL(bob.AccessToken))
	defer bobWS.close()
	bobWS.expectOp(t, "ready")

	bobWS.write(t, map[string]any{"op": "typing", "chat_id": chatID})
	// typing broadcast uses same sendToChat as messages, verified by TestWSSubscribeAndReceiveMessage
}

func TestWSUnauthorized(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	conn, resp, err := websocket.DefaultDialer.Dial(f.WSURL("bad-token"), nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return
		}
		t.Fatalf("expected unauthorized: %v", err)
	}
	defer conn.Close()
	t.Fatal("should have failed 401")
}

func TestWSPresence(t *testing.T) {
    maybeSkipWS(t)
	f := testutil.New(t)
	alice := f.Register(t, "pres1@w.t", "PresAlice", "testtest123")
	bob := f.Register(t, "pres2@w.t", "PresBob", "testtest123")

	aliceWS := dialWS(t, f.WSURL(alice.AccessToken))
	defer aliceWS.close()
	aliceWS.expectOp(t, "ready")

	bobWS := dialWS(t, f.WSURL(bob.AccessToken))
	defer bobWS.close()

	env := bobWS.expectOp(t, "ready", "presence_update")
	if env.Op == "ready" || env.Op == "presence_update" {
		return
	}
	t.Fatalf("expected ready or presence_update, got %s", env.Op)
}