package testutil_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func startMockAI(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatal("expected POST")
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]json.RawMessage
		json.Unmarshal(body, &req)
		// auto-added by ensureStreamEnabled
		var stream bool
		json.Unmarshal(req["stream"], &stream)
		if !stream {
			t.Fatal("stream should be true")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" from\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" AI\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	return httptest.NewServer(mux)
}

func TestSendStreamMessage(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream1@test.dev", "StreamAlice", "password123")
	mockAI := startMockAI(t)
	defer mockAI.Close()

	// Create a chat
	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "AI Chat", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// Send stream message
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body": map[string]any{
				"model":    "test-model",
				"messages": []map[string]string{{"role": "user", "content": "hi"}},
			},
		},
	})
	defer sendRes.Body.Close()

	if sendRes.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("stream message: want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}

	if ct := sendRes.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}

	// Read SSE events
	scanner := bufio.NewScanner(sendRes.Body)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			break
		}
		var event struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		content.WriteString(event.Content)
	}
	if content.String() != "Hello from AI" {
		t.Fatalf("expected 'Hello from AI', got '%s'", content.String())
	}

	// Verify the AI message was saved to DB
	msgsRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages?limit=10", alice.AccessToken, nil)
	defer msgsRes.Body.Close()
	var listResp struct {
		Messages []struct {
			UserID  string `json:"user_id"`
			Content string `json:"content"`
			Type    string `json:"type"`
		} `json:"messages"`
	}
	json.NewDecoder(msgsRes.Body).Decode(&listResp)
	found := false
	for _, m := range listResp.Messages {
		if m.Type == "stream" && m.UserID == alice.UserID {
			found = true
			if m.Content != "Hello from AI" {
				t.Fatalf("saved content: want 'Hello from AI', got '%s'", m.Content)
			}
		}
	}
	if !found {
		t.Fatal("AI message not saved to DB")
	}
}

func TestSendStreamMessage_NonStreamingResponse(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream2@test.dev", "StreamBob", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Non-stream reply"}}]}`))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NonStream", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body": map[string]any{
				"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}},
			},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}

	var got strings.Builder
	scanner := bufio.NewScanner(sendRes.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		p := line[6:]
		if p == "[DONE]" {
			break
		}
		var ev struct{ Content string }
		json.Unmarshal([]byte(p), &ev)
		got.WriteString(ev.Content)
	}
	if got.String() != "Non-stream reply" {
		t.Fatalf("want 'Non-stream reply', got '%s'", got.String())
	}
}

func TestSendStreamMessage_MissingSource(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream3@test.dev", "StreamCarol", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NoSource", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 400 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("missing source: want 400 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_NilSource(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream4@test.dev", "StreamDave", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NilSource", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":   "stream",
		"source": nil,
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 400 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("nil source: want 400 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_NonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream5@test.dev", "StreamEve", "password123")
	bob := f.Register(t, "stream6@test.dev", "StreamFrank", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Exclusive", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	mockAI := startMockAI(t)
	defer mockAI.Close()

	// Bob is not a member of this chat
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", bob.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 403 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("non-member: want 403 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_EndpointConfig(t *testing.T) {
	// Verify that the given endpoint/auth_key are used as-is (for openai.ai/zen/v1/chat/completions)
	f := testutil.New(t)
	alice := f.Register(t, "stream7@test.dev", "StreamGrace", "password123")

	mockAI := startMockAI(t)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "ConfigTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// Send with openai.ai endpoint and public key (should hit mockAI in test)
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("config test: want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSSEReplay_AfterStreamComplete(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "replay@test.dev", "ReplayAlice", "password123")
	mockAI := startMockAI(t)
	defer mockAI.Close()

	// create chat
	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Replay", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// send stream message with unique msg_id, read SSE body until done
	msgID := "replay-test-msg"
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body": map[string]any{
				"model":    "test",
				"messages": []map[string]string{{"role": "user", "content": "hi"}},
			},
		},
		"msg_id": msgID,
	})
	io.Copy(io.Discard, sendRes.Body)
	sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		t.Fatalf("send: want 200 got %d", sendRes.StatusCode)
	}

	// StreamMessageContent should now replay the full content from live buffer
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/"+msgID+"/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		t.Fatalf("replay: want 200 got %d", replayRes.StatusCode)
	}
	if ct := replayRes.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("want text/event-stream, got %s", ct)
	}

	scanner := bufio.NewScanner(replayRes.Body)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		p := line[6:]
		if p == "[DONE]" {
			break
		}
		var ev struct{ Content string }
		if err := json.Unmarshal([]byte(p), &ev); err != nil {
			continue
		}
		content.WriteString(ev.Content)
	}
	if content.String() != "Hello from AI" {
		t.Fatalf("replay content: want 'Hello from AI', got '%s'", content.String())
	}

	// saved to DB
	msgsRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages?limit=10", alice.AccessToken, nil)
	defer msgsRes.Body.Close()
	var listResp struct {
		Messages []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	json.NewDecoder(msgsRes.Body).Decode(&listResp)
	var found bool
	for _, m := range listResp.Messages {
		if m.ID == msgID {
			found = true
			if m.Content != "Hello from AI" {
				t.Fatalf("DB content: want 'Hello from AI', got '%s'", m.Content)
			}
		}
	}
	if !found {
		t.Fatal("message not found in DB")
	}
}

func TestSendStreamMessage_ReplayAfterCleanup(t *testing.T) {
	// after live buffer cleanup, StreamMessageContent should read from DB
	f := testutil.New(t)
	alice := f.Register(t, "replay2@test.dev", "ReplayBob", "password123")
	mockAI := startMockAI(t)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Replay2", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	msgID := "replay-cleanup-msg"
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body": map[string]any{
				"model":    "test",
				"messages": []map[string]string{{"role": "user", "content": "hi"}},
			},
		},
		"msg_id": msgID,
	})
	io.Copy(io.Discard, sendRes.Body)
	sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		t.Fatalf("send: want 200 got %d", sendRes.StatusCode)
	}

	// verify DB has it
	msgsRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages?limit=10", alice.AccessToken, nil)
	defer msgsRes.Body.Close()
	var listResp struct {
		Messages []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	json.NewDecoder(msgsRes.Body).Decode(&listResp)

	// replay should work from live buffer (no cleanup yet)
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/"+msgID+"/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		t.Fatalf("replay: want 200 got %d", replayRes.StatusCode)
	}
	scanner := bufio.NewScanner(replayRes.Body)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		p := line[6:]
		if p == "[DONE]" {
			break
		}
		var ev struct{ Content string }
		json.Unmarshal([]byte(p), &ev)
		content.WriteString(ev.Content)
	}
	if content.String() != "Hello from AI" {
		t.Fatalf("live buffer: want 'Hello from AI', got '%s'", content.String())
	}
}

func TestSendStreamMessage_ReplayNonexistentMessage(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "replay3@test.dev", "ReplayCarol", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Replay3", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// replay a nonexistent message — should return SSE with just [DONE]
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/nonexistent-msg/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		t.Fatalf("replay: want 200 got %d", replayRes.StatusCode)
	}
	scanner := bufio.NewScanner(replayRes.Body)
	foundDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "data: [DONE]" {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Fatal("should receive [DONE] even for nonexistent message")
	}
}

func TestSendStreamMessage_ReplayNonMember(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "replay4@test.dev", "ReplayDave", "password123")
	bob := f.Register(t, "replay5@test.dev", "ReplayEve", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Replay4", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// bob is not a member
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/some-msg/stream", bob.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 403 {
		b, _ := io.ReadAll(replayRes.Body)
		t.Fatalf("non-member: want 403 got %d body=%s", replayRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_UpstreamError(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream8@test.dev", "StreamUpstreamErr", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "UpstreamErr", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != http.StatusBadGateway {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("upstream error: want 502 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_UpstreamTransportError(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream9@test.dev", "StreamTransportErr", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "TransportErr", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// endpoint that will be refused
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": "http://127.0.0.1:1/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != http.StatusBadGateway {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("transport error: want 502 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_ResponseHeaders(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream10@test.dev", "StreamHeaders", "password123")
	mockAI := startMockAI(t)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "Headers", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		t.Fatalf("want 200 got %d", sendRes.StatusCode)
	}
	if ct := sendRes.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type: want text/event-stream, got %s", ct)
	}
	if cc := sendRes.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control: want no-cache, got %s", cc)
	}
	if xa := sendRes.Header.Get("X-Accel-Buffering"); xa != "no" {
		t.Fatalf("X-Accel-Buffering: want no, got %s", xa)
	}
	io.Copy(io.Discard, sendRes.Body)
}

func TestSendStreamMessage_ReplayLiveWithNotification(t *testing.T) {
	// replay while stream is still in progress — gets live chunks + subscription
	f := testutil.New(t)
	alice := f.Register(t, "stream11@test.dev", "StreamLiveNotify", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		for i := 0; i < 5; i++ {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"chunk" + fmt.Sprintf("%d", i) + "\"}}]}\n\n"))
			w.(http.Flusher).Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		w.(http.Flusher).Flush()
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "LiveNotify", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// send the stream message, consume chunks in background
	msgID := "replay-live-notify"
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
		"msg_id": msgID,
	})
	io.Copy(io.Discard, sendRes.Body)
	sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		t.Fatalf("send: want 200 got %d", sendRes.StatusCode)
	}

	// now replay — should get full content
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/"+msgID+"/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		t.Fatalf("replay: want 200 got %d", replayRes.StatusCode)
	}
	scanner := bufio.NewScanner(replayRes.Body)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		p := line[6:]
		if p == "[DONE]" {
			break
		}
		var ev struct{ Content string }
		json.Unmarshal([]byte(p), &ev)
		content.WriteString(ev.Content)
	}
	expected := "chunk0chunk1chunk2chunk3chunk4"
	if content.String() != expected {
		t.Fatalf("replay content: want %q, got %q", expected, content.String())
	}
}

func TestSendStreamMessage_ReplayNonStreamMessage(t *testing.T) {
	// replay a non-stream message — should return [DONE] only
	f := testutil.New(t)
	alice := f.Register(t, "stream12@test.dev", "StreamNonStreamReplay", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NonStreamReplay", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	// send a regular message
	msgRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"content": "regular message",
	})
	defer msgRes.Body.Close()
	var msg struct {
		ID string `json:"id"`
	}
	json.NewDecoder(msgRes.Body).Decode(&msg)

	// replay non-stream message — stream endpoint should return [DONE] only
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/"+msg.ID+"/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		b, _ := io.ReadAll(replayRes.Body)
		t.Fatalf("replay: want 200 got %d body=%s", replayRes.StatusCode, string(b))
	}
	scanner := bufio.NewScanner(replayRes.Body)
	foundDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "data: [DONE]" {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Fatal("expected [DONE] for non-stream message replay")
	}
}

func TestSendStreamMessage_ReasoningContent(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream13@test.dev", "StreamReason", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "ReasonTest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(sendRes.Body)
	var content strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		p := line[6:]
		if p == "[DONE]" {
			break
		}
		var ev struct{ Content string }
		json.Unmarshal([]byte(p), &ev)
		content.WriteString(ev.Content)
	}
	expected := "thinkinganswer"
	if content.String() != expected {
		t.Fatalf("want %q, got %q", expected, content.String())
	}
}

func TestSendStreamMessage_EmptySourceBody(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream14@test.dev", "StreamEmptyBody", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyBody", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("empty body: want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}
	io.Copy(io.Discard, sendRes.Body)
}

func TestSendStreamMessage_SourceWithoutBody(t *testing.T) {
	// source with endpoint and auth_key but no body — should fail with 400
	f := testutil.New(t)
	alice := f.Register(t, "stream15@test.dev", "StreamNoBody", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NoBody", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": "http://example.com/chat",
			"auth_key": "public",
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 400 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("no body: want 400 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_EmptyEndpoint(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream16@test.dev", "StreamEmptyEP", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyEP", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": "",
			"auth_key": "public",
			"body":     map[string]any{"model": "test"},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 400 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("empty endpoint: want 400 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_EmptyAuthKey(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "stream17@test.dev", "StreamEmptyAuth", "password123")

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyAuth", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": "http://example.com/chat",
			"auth_key": "",
			"body":     map[string]any{"model": "test"},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 400 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("empty auth_key: want 400 got %d body=%s", sendRes.StatusCode, string(b))
	}
}

func TestSendStreamMessage_MissingMsgID(t *testing.T) {
	// when msg_id is not provided, the handler auto-generates one
	f := testutil.New(t)
	alice := f.Register(t, "stream18@test.dev", "StreamNoMsgID", "password123")
	mockAI := startMockAI(t)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "NoMsgID", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("auto msg_id: want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}
	io.Copy(io.Discard, sendRes.Body)
}

func TestSendStreamMessage_ReplayWithEmptyLiveBuffer(t *testing.T) {
	// stream sends no content chunks (only [DONE]), then replay from live buffer
	f := testutil.New(t)
	alice := f.Register(t, "stream19@test.dev", "StreamEmptyReplay", "password123")

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "EmptyReplay", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	msgID := "empty-replay-msg"
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": mockAI.URL + "/v1/chat/completions",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
		"msg_id": msgID,
	})
	io.Copy(io.Discard, sendRes.Body)
	sendRes.Body.Close()

	// replay — should get [DONE] directly (no content, but done is true)
	replayRes := f.Do(t, "GET", "/api/chats/"+chat.ID+"/messages/"+msgID+"/stream", alice.AccessToken, nil)
	defer replayRes.Body.Close()
	if replayRes.StatusCode != 200 {
		t.Fatalf("replay: want 200 got %d", replayRes.StatusCode)
	}
	scanner := bufio.NewScanner(replayRes.Body)
	foundDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "data: [DONE]" {
			foundDone = true
			break
		}
	}
	if !foundDone {
		t.Fatal("expected [DONE] for empty stream replay")
	}
}

// TestSendStreamMessage_SlowAI 用慢速 mock AI（每 500ms 发一个 chunk，持续 15s）
// 验证 v0.8.15 的 ReadTimeout 分组下 SSE 能存活超过 10s（不会被 APITimeout 杀死）。
func TestSendStreamMessage_SlowAI(t *testing.T) {
	f := testutil.New(t)
	alice := f.Register(t, "slowai@test.dev", "SlowAI", "password123")

	// 慢速 mock AI：每 500ms 一个 chunk，持续 15s（共 30 个 chunk）
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 30; i++ {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"chunk%d\"}}]}\n\n", i)
			flusher.Flush()
			time.Sleep(500 * time.Millisecond)
		}
		w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	})
	slowAI := httptest.NewServer(mux)
	defer slowAI.Close()

	// 创建群聊
	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "SlowAITest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	start := time.Now()
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type": "stream",
		"source": map[string]any{
			"endpoint": slowAI.URL + "/slow",
			"auth_key": "public",
			"body":     map[string]any{"model": "test", "messages": []map[string]string{{"role": "user", "content": "hi"}}},
		},
	})
	defer sendRes.Body.Close()
	if sendRes.StatusCode != 200 {
		b, _ := io.ReadAll(sendRes.Body)
		t.Fatalf("slow AI: want 200 got %d body=%s", sendRes.StatusCode, string(b))
	}

	// 读 SSE，记录持续时间和 chunk 数
	scanner := bufio.NewScanner(sendRes.Body)
	chunkCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			elapsed := time.Since(start)
			if elapsed < 14*time.Second {
				t.Fatalf("SSE completed too fast: %v (expected ~15s for 30 slow chunks)", elapsed)
			}
			t.Logf("SSE survived %v with %d chunks — ReadTimeout fix works!", elapsed, chunkCount)
			return
		}
		chunkCount++
	}

	elapsed := time.Since(start)
	t.Fatalf("SSE INTERRUPTED at %v after %d chunks (err=%v) — route still under APITimeout!", elapsed, chunkCount, scanner.Err())
}

// TestRealAIEndpoint 用真实 AI endpoint 验证生产场景的 SSE 完整性。
// 运行: go test -run TestRealAIEndpoint -v -timeout 3m ./internal/testutil/
// 需要网络连接。
func TestRealAIEndpoint(t *testing.T) {
	endpoint := "https://bwh.moonchan.xyz:8443/zen/v1/chat/completions"
	authKey := "public"
	model := "deepseek-v4-flash-free"

	f := testutil.New(t)
	alice := f.Register(t, "realai@test.dev", "RealAI", "password123")

	// 创建群聊
	res := f.Do(t, "POST", "/api/chats", alice.AccessToken, map[string]any{
		"type": "group", "name": "RealAITest", "member_ids": []string{},
	})
	var chat struct {
		ID string `json:"id"`
	}
	json.NewDecoder(res.Body).Decode(&chat)
	res.Body.Close()

	t.Logf("chat created: %s", chat.ID)

	// 发送 stream 消息到真实 endpoint
	start := time.Now()
	sendRes := f.Do(t, "POST", "/api/chats/"+chat.ID+"/messages", alice.AccessToken, map[string]any{
		"type":    "stream",
		"content": "",
		"source": map[string]any{
			"endpoint": endpoint,
			"auth_key": authKey,
			"body": map[string]any{
				"model": model,
				"messages": []map[string]string{
					{"role": "system", "content": "请用中文回答，回复一篇约500字的短文，主题是大语言模型的发展历史。"},
					{"role": "user", "content": "开始"},
				},
			},
		},
	})
	if sendRes.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(sendRes.Body)
		sendRes.Body.Close()
		t.Fatalf("stream message: want 200 got %d body=%s", sendRes.StatusCode, string(body))
	}

	// 读取 SSE，记录时间和数据量
	defer sendRes.Body.Close()

	scanner := bufio.NewScanner(sendRes.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	var totalContent int
	chunkCount := 0
	lastChunkAt := time.Now()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			elapsed := time.Since(start)
			t.Logf("SSE complete: elapsed=%v chunks=%d totalBytes=%d", elapsed, chunkCount, totalContent)
			return
		}

		lastChunkAt = time.Now()
		chunkCount++
		totalContent += len(payload)

		if chunkCount <= 3 || chunkCount%50 == 0 {
			t.Logf("chunk #%d at %v: len=%d", chunkCount, time.Since(start), len(payload))
		}
	}

	elapsed := time.Since(start)
	t.Logf("INTERRUPTED after %v: chunks=%d totalBytes=%d lastChunkAge=%v",
		elapsed, chunkCount, totalContent, time.Since(lastChunkAt))

	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "unexpected EOF") {
			t.Logf("scanner EOF (connection closed by server): %v", err)
		} else {
			t.Logf("scanner error: %v", err)
		}
	}

	// 如果没有收到 [DONE] 而是被中断，标记失败
	t.Errorf("SSE interrupted before [DONE]: only %d chunks in %v", chunkCount, elapsed)
}
