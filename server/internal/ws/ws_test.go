// Package ws_test 覆盖 WebSocket 网关的端到端行为。
//
// 测试范围:
//   - 连接握手与 ready 事件(user 信息)
//   - ping/pong 心跳
//   - 订阅后实时接收消息(message_create)
//   - typing 广播(仅发给聊天内其他成员,不含发送者)
//   - 未授权连接被拒绝(401)
//   - presence 在线状态广播
//
// 运行方式: cd server && go test ./internal/ws/
// 说明:真实 WebSocket 拨号(gorilla/websocket),走 testutil 装配的完整
// HTTP + Hub 栈,不 mock。CI 中默认启用,不再需要 WS_ENABLED 门控
// (WS_ENABLED 仅作为生产环境显式关闭 WS 的开关,见 gateway.go)。
package ws_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
	"github.com/Hana-ame/chat-app/server/internal/ws"
	"github.com/gorilla/websocket"
)

// wsEnvelope 是服务端下发的统一消息信封:op 为操作类型,payload 为负载。
type wsEnvelope struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// wsClient 封装一条已连接的 WebSocket,提供带超时的读写辅助。
type wsClient struct {
	conn *websocket.Conn
}

// dialWS 建立 WebSocket 连接;401 视为测试失败(调用方若测未授权应直接拨号)。
// token 通过 Authorization 头传递(WSURL 刻意不带 token,见 testutil/client.go)。
func dialWS(t *testing.T, url, token string) *wsClient {
	t.Helper()
	header := http.Header{}
	header.Set("Authorization", "Bearer "+token)
	conn, resp, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("unauthorized: %v", err)
		}
		t.Fatalf("dial: %v", err)
	}
	return &wsClient{conn: conn}
}

func (c *wsClient) close() { c.conn.Close() }

// read 读取一条消息,超时即失败。供 expectOp 内部使用。
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

// expectOp 轮询读取,直到收到 ops 中任一操作类型;未知操作(如 presence_update)
// 自动跳过,保证对事件顺序不敏感。超时(约 10 秒)则失败。
func (c *wsClient) expectOp(t *testing.T, ops ...string) wsEnvelope {
	t.Helper()
	for attempt := 0; attempt < 10; attempt++ {
		env := c.read(t, 5*time.Second)
		for _, want := range ops {
			if env.Op == want {
				return env
			}
		}
		if attempt > 8 {
			t.Fatalf("expected one of %v, got %s", ops, env.Op)
		}
	}
	return wsEnvelope{}
}

// write 发送一条 JSON 信封(客户端 → 服务端方向)。
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

// createGroupChat 走 HTTP API 建群,返回 chatID。
func createGroupChat(t *testing.T, f *testutil.Fixture, token string, name string, members []string) string {
	t.Helper()
	res := f.Do(t, "POST", "/api/chats", token, map[string]any{
		"type": "group", "name": name, "member_ids": members,
	})
	testutil.RequireStatus(t, res, 201)
	var c struct {
		ID string `json:"id"`
	}
	testutil.RequireJSONBody(t, res, &c)
	res.Body.Close()
	testutil.RequireNotEqual(t, c.ID, "")
	return c.ID
}

func TestWSConnectAndReady(t *testing.T) {
	// 连接成功后应收到 ready,payload 里带当前用户信息。
	f := testutil.New(t)
	alice := f.Register(t, "ws1@w.t", "AliceWS", "testtest123")

	ws := dialWS(t, f.WSURL(alice.AccessToken), alice.AccessToken)
	defer ws.close()

	env := ws.expectOp(t, "ready")
	var ready struct {
		User map[string]any `json:"user"`
	}
	testutil.RequireNoError(t, json.Unmarshal(env.Payload, &ready))
	testutil.RequireEqual(t, ready.User["username"], "AliceWS")
}

func TestWSPingPong(t *testing.T) {
	// 客户端发 ping,服务端必须回 pong(心跳保活链路)。
	f := testutil.New(t)
	alice := f.Register(t, "ping@w.t", "Pinger", "testtest123")

	ws := dialWS(t, f.WSURL(alice.AccessToken), alice.AccessToken)
	defer ws.close()
	ws.expectOp(t, "ready")

	ws.write(t, map[string]string{"op": "ping"})
	env := ws.expectOp(t, "pong")
	testutil.RequireEqual(t, env.Op, "pong")
}

// expectPresence 读取直到收到指定用户的 presence_update;自动跳过其他
// presence_update(如自己上线广播),对事件顺序不敏感。
func (c *wsClient) expectPresence(t *testing.T, wantUserID string) wsEnvelope {
	t.Helper()
	for attempt := 0; attempt < 10; attempt++ {
		env := c.expectOp(t, "presence_update")
		var pres struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal(env.Payload, &pres); err != nil {
			continue
		}
		if pres.UserID == wantUserID {
			return env
		}
	}
	t.Fatalf("no presence_update for user %s", wantUserID)
	return wsEnvelope{}
}

func TestWSSubscribeAndReceiveMessage(t *testing.T) {
	// 订阅聊天后,其他成员发消息应实时收到 message_create。
	// 注:网关在连接时已自动订阅用户的所有聊天,显式 subscribe 用于验证协议格式。
	f := testutil.New(t)
	alice := f.Register(t, "sub1@w.t", "SubAlice", "testtest123")
	bob := f.Register(t, "sub2@w.t", "SubBob", "testtest123")

	chatID := createGroupChat(t, f, alice.AccessToken, "WS Chat", []string{bob.UserID})

	ws := dialWS(t, f.WSURL(alice.AccessToken), alice.AccessToken)
	defer ws.close()
	ws.expectOp(t, "ready")

	ws.write(t, map[string]any{"op": "subscribe", "payload": map[string]any{"chat_id": chatID}})
	time.Sleep(50 * time.Millisecond)

	sendRes := f.Do(t, "POST", "/api/chats/"+chatID+"/messages", bob.AccessToken, map[string]string{
		"content": "ws test message",
	})
	defer sendRes.Body.Close()
	testutil.RequireStatus(t, sendRes, 201)

	env := ws.expectOp(t, "message_create", "presence_update")
	if env.Op == "presence_update" {
		env = ws.expectOp(t, "message_create")
	}
	testutil.RequireEqual(t, env.Op, "message_create")
	var msg struct {
		Content string `json:"content"`
	}
	testutil.RequireNoError(t, json.Unmarshal(env.Payload, &msg))
	testutil.RequireEqual(t, msg.Content, "ws test message")
}

func TestWSTyping(t *testing.T) {
	// typing 广播只发给聊天内其他成员(实现上排除发送者自己)。
	f := testutil.New(t)
	alice := f.Register(t, "type1@w.t", "Typer", "testtest123")
	bob := f.Register(t, "type2@w.t", "Typed", "testtest123")

	chatID := createGroupChat(t, f, alice.AccessToken, "Typing Chat", []string{bob.UserID})

	aliceWS := dialWS(t, f.WSURL(alice.AccessToken), alice.AccessToken)
	defer aliceWS.close()
	aliceWS.expectOp(t, "ready")

	bobWS := dialWS(t, f.WSURL(bob.AccessToken), bob.AccessToken)
	defer bobWS.close()
	bobWS.expectOp(t, "ready")

	bobWS.write(t, map[string]any{"op": "typing", "payload": map[string]any{"chat_id": chatID}})

	env := aliceWS.expectOp(t, "typing")
	testutil.RequireEqual(t, env.Op, "typing")
	var typing struct {
		ChatID string `json:"chat_id"`
		UserID string `json:"user_id"`
	}
	testutil.RequireNoError(t, json.Unmarshal(env.Payload, &typing))
	testutil.RequireEqual(t, typing.ChatID, chatID)
	testutil.RequireEqual(t, typing.UserID, bob.UserID)
}

func TestWSUnauthorized(t *testing.T) {
	// 无效 token 必须被拒绝(HTTP 401),而不是升级成功。
	f := testutil.New(t)
	conn, resp, err := websocket.DefaultDialer.Dial(f.WSURL("bad-token"), nil)
	if err != nil {
		testutil.RequireNotNil(t, resp)
		testutil.RequireEqual(t, resp.StatusCode, http.StatusUnauthorized)
		return
	}
	defer conn.Close()
	t.Fatal("should have failed 401")
}

func TestWSPresence(t *testing.T) {
	// 新用户上线时,已连接用户应收到 presence_update(online)。
	f := testutil.New(t)
	alice := f.Register(t, "pres1@w.t", "PresAlice", "testtest123")
	bob := f.Register(t, "pres2@w.t", "PresBob", "testtest123")

	aliceWS := dialWS(t, f.WSURL(alice.AccessToken), alice.AccessToken)
	defer aliceWS.close()
	aliceWS.expectOp(t, "ready")

	bobWS := dialWS(t, f.WSURL(bob.AccessToken), bob.AccessToken)
	defer bobWS.close()
	bobWS.expectOp(t, "ready")

	env := aliceWS.expectPresence(t, bob.UserID)
	testutil.RequireEqual(t, env.Op, "presence_update")
	var pres struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
	}
	testutil.RequireNoError(t, json.Unmarshal(env.Payload, &pres))
	testutil.RequireEqual(t, pres.UserID, bob.UserID)
	testutil.RequireEqual(t, pres.Status, "online")
}

// TestWSOriginRejected 验证 CheckOrigin 按 CORS 白名单拒绝跨站页面发起的
// WS 握手:白名单外的 Origin 握手失败,白名单内与无 Origin(非浏览器
// 客户端)成功。独立装配最小栈,不依赖 testutil 的 "*" 默认配置。
func TestWSOriginRejected(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Addr:                    ":0",
		DBPath:                  filepath.Join(dir, "test.db"),
		UploadDir:               filepath.Join(dir, "uploads"),
		JWTSecret:               []byte("test-secret-very-secret-test-secret-very-secret"),
		AccessTokenTTL:          15 * time.Minute,
		RefreshTokenTTL:         24 * time.Hour,
		MaxUploadBytes:          5 << 20,
		UploadSalt:              "test-salt",
		AllowOrigins:            []string{"https://trusted.example"},
		MaxMessageContentLength: 4000,
		APITimeout:              30 * time.Second,
		UploadTimeout:           5 * time.Minute,
		ReadTimeout:             10 * time.Minute,
	}
	database, err := db.Open(cfg.DBPath, cfg.MaxMessageContentLength)
	testutil.RequireNoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL)
	gateway := ws.NewGateway(ws.NewHub(database), database, authSvc, 1<<16, cfg)
	httpSrv := httptest.NewServer(gateway)
	t.Cleanup(httpSrv.Close)

	user, err := database.CreateUser(t.Context(), "origin@w.t", "OriginAlice", "testtest123")
	testutil.RequireNoError(t, err)
	token, _, err := authSvc.IssueAccessToken(user.ID)
	testutil.RequireNoError(t, err)

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	dialWithOrigin := func(origin string) error {
		header := http.Header{}
		header.Set("Authorization", "Bearer "+token)
		if origin != "" {
			header.Set("Origin", origin)
		}
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			if resp != nil {
				return fmt.Errorf("dial failed: %v (status %d)", err, resp.StatusCode)
			}
			return fmt.Errorf("dial failed: %v", err)
		}
		conn.Close()
		return nil
	}

	// 白名单外 Origin:握手被拒绝。
	err = dialWithOrigin("https://evil.example")
	testutil.RequireError(t, err)

	// 白名单内 Origin:握手成功。
	testutil.RequireNoError(t, dialWithOrigin("https://trusted.example"))

	// 未显式携带 Origin 的客户端:gorilla Dialer 会自动补
	// "http://<ws-host>",该值不在白名单 → 同样拒绝(与浏览器 CSWSH 同权)。
	err = dialWithOrigin("")
	testutil.RequireError(t, err)
}
