// Package service_test 覆盖业务逻辑层:全部 service 方法、权限(成员/所有者/
// 管理员)、错误映射、context 取消传播、DB 错误注入(WithTx 回滚)、
// StreamService 全生命周期、并发场景。
//
// 运行方式: cd server && go test ./internal/service/
// 说明:AI 上游用 httptest 假 SSE server(见 stream_test.go),DB 为真实
// SQLite 临时库。
package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/testkit"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func startMockAIStream(t *testing.T, chunks ...string) *httptest.Server {
	t.Helper()
	return testkit.NewMockAIServer(t, chunks...)
}

func TestStreamService_Lifecycle(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_life@x.com", "StrmLife")
	chat := createTestChat(t, f, "StrmLife", a, []string{a})

	mockAI := startMockAIStream(t, "Hello", " World")
	defer mockAI.Close()

	msgID := "strm-lifecycle-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	var got strings.Builder
	chunkCount := 0
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		chunkCount++
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	testutil.RequireEqual(t, got.String(), "Hello World")
	testutil.RequireEqual(t, chunkCount, 2)

	// StreamStatus before finish
	chunks, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should exist")
	testutil.RequireFalse(t, done, "stream should not be done yet")
	testutil.RequireEqual(t, len(chunks), 2)
	var joined string
	for _, ci := range chunks {
		joined += ci.Content
	}
	testutil.RequireEqual(t, joined, "Hello World")

	// FinishStream
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, got.String(), "")

	// done should be true
	_, done, ok = f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should still exist after finish")
	testutil.RequireTrue(t, done, "stream should be done after finish")

	// DB should have the message
	msg, err := f.DB.GetMessage(context.Background(), msgID)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.Content, "Hello World")
	testutil.RequireEqual(t, msg.Type, "stream")
}

func TestStreamService_StreamStatus_WithIdx(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idx@x.com", "StrmIdx")
	chat := createTestChat(t, f, "StrmIdx", a, []string{a})

	mockAI := startMockAIStream(t, "A", "B", "C")
	defer mockAI.Close()

	msgID := "strm-idx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// consume stream and append
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// read with idx
	chunks, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok && len(chunks) == 3, "want 3 chunks from idx 0, got %d ok=%v", len(chunks), ok)

	chunks, _, _ = f.Server.Services.Stream.StreamStatus(msgID, 1)
	testutil.RequireEqual(t, len(chunks), 2)

	chunks, _, _ = f.Server.Services.Stream.StreamStatus(msgID, 3)
	testutil.RequireEqual(t, len(chunks), 0)

	// after finish, StreamStatus should still be ok
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "ABC", "")
	_, done, ok = f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should still exist after finish (before cleanup)")
	testutil.RequireTrue(t, done, "stream should be done")
}

func TestStreamService_Subscribe_Notification(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_sub@x.com", "StrmSub")
	chat := createTestChat(t, f, "StrmSub", a, []string{a})

	mockAI := startMockAIStream(t, "X")
	defer mockAI.Close()

	msgID := "strm-sub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// subscribe before appending
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	// consume and append on a goroutine to avoid deadlock
	done := make(chan struct{})
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
		close(done)
	}()

	// should receive notification
	select {
	case <-notify:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for notification")
	}

	// verify chunk is there
	chunks, _, _ := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, len(chunks) == 1 && chunks[0].Content == "X", "want [X], got %v", chunks)

	<-done
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "X", "")
}

func TestStreamService_Finish_NotifiesSubscribers(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_fin@x.com", "StrmFin")
	chat := createTestChat(t, f, "StrmFin", a, []string{a})

	mockAI := startMockAIStream(t, "hello")
	defer mockAI.Close()

	msgID := "strm-fin-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "hello", "")

	// subscriber channel should be closed (nil after finish)
	select {
	case _, ok := <-notify:
		testutil.RequireFalse(t, ok, "channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("timeout: notify channel not closed")
	}
}

func TestStreamService_StreamStatus_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	_, done, ok := f.Server.Services.Stream.StreamStatus("nonexistent", 0)
	testutil.RequireFalse(t, ok, "should not exist")
	testutil.RequireFalse(t, done, "nonexistent should not be done")
}

func TestStreamService_Unsubscribe(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_unsub@x.com", "StrmUnsub")
	chat := createTestChat(t, f, "StrmUnsub", a, []string{a})

	mockAI := startMockAIStream(t, "d")
	defer mockAI.Close()

	msgID := "strm-unsub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	ch1 := f.Server.Services.Stream.Subscribe(msgID)
	ch2 := f.Server.Services.Stream.Subscribe(msgID)

	// unsubscribe ch1
	f.Server.Services.Stream.Unsubscribe(msgID, ch1)

	// append
	f.Server.Services.Stream.AppendChunk(msgID, "content", "data")

	// ch1 should not receive notification
	select {
	case <-ch1:
		t.Fatal("ch1 should not receive notification after unsubscribe")
	case <-time.After(100 * time.Millisecond):
	}

	// ch2 should receive
	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("ch2 should receive notification")
	}
}

func TestStreamService_EmptyContent(t *testing.T) {
	// AI returns no content chunks (only [DONE])
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_emp@x.com", "StrmEmp")
	chat := createTestChat(t, f, "StrmEmp", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-empty-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// FinishStream with empty content
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "", "")

	// done should be true
	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should exist")
	testutil.RequireTrue(t, done, "stream should be done")

	// DB should NOT have the message (empty content rejected by SendAI)
	_, err = f.DB.GetMessage(context.Background(), msgID)
	testutil.RequireError(t, err)
}

func TestStreamService_NonStreamingUpstream(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_ns@x.com", "StrmNS")
	chat := createTestChat(t, f, "StrmNS", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"non-stream response"}}]}`))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-ns-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	testutil.RequireEqual(t, got.String(), "non-stream response")

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, got.String(), "")

	msg, err := f.DB.GetMessage(context.Background(), msgID)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, msg.Content, "non-stream response")
}

func TestStreamService_MultipleSubscribers(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_multi@x.com", "StrmMulti")
	chat := createTestChat(t, f, "StrmMulti", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c")
	defer mockAI.Close()

	msgID := "strm-multi-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	sub1 := f.Server.Services.Stream.Subscribe(msgID)
	sub2 := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, sub1)
	defer f.Server.Services.Stream.Unsubscribe(msgID, sub2)

	// consume + append in goroutine
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
	}()

	// both subscribers should get notification
	for i, sub := range []chan struct{}{sub1, sub2} {
		select {
		case <-sub:
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d not notified", i)
		}
	}
}

func TestStreamService_UpstreamError(t *testing.T) {
	// HTTP 500 is rejected by StreamFromSource with an error.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_err@x.com", "StrmErr")
	chat := createTestChat(t, f, "StrmErr", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	src := ai.Source{
		Endpoint: mockAI.URL + "/error",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, "err-msg", src, nil)
	testutil.RequireError(t, err)
	testutil.RequireTrue(t, strings.Contains(err.Error(), "500"), "expected 500 in error, got: %v", err)
}

func TestStreamService_ContextCancelPropagation(t *testing.T) {
	// Context cancellation during body read should close resp.Body and end the stream.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_cc@x.com", "StrmCC")
	chat := createTestChat(t, f, "StrmCC", a, []string{a})

	ctx, cancel := context.WithCancel(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		// wait for context cancellation
		<-ctx.Done()
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(ctx, chat.ID, a, "cancel-msg", src, nil)
	testutil.RequireNoError(t, err)

	// cancel context to trigger resp.Body.Close
	cancel()

	// channel should close without blocking
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not close after context cancellation")
	}
}

func TestStreamService_ConcurrentAppendAndStatus(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_conc@x.com", "StrmConc")
	chat := createTestChat(t, f, "StrmConc", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c", "d", "e")
	defer mockAI.Close()

	msgID := "strm-conc-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// consume and append concurrently with StreamStatus reads
	done := make(chan struct{})
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
		close(done)
	}()

	// read status concurrently — should never panic or return inconsistent data
	for i := 0; i < 100; i++ {
		_, _, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
		if !ok {
			// stream might not have started yet
			continue
		}
	}
	<-done
}

func TestStreamService_FinishIdempotent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idem@x.com", "StrmIdem")
	chat := createTestChat(t, f, "StrmIdem", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-idem-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// calling FinishStream twice should not panic
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")

	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should exist")
	testutil.RequireTrue(t, done, "stream should be done")
}

func TestStreamService_SubscribeAfterFinish(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_saf@x.com", "StrmSaf")
	chat := createTestChat(t, f, "StrmSaf", a, []string{a})

	mockAI := startMockAIStream(t, "d")
	defer mockAI.Close()

	msgID := "strm-saf-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "d", "")

	// subscribe after finish — channel should close immediately
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	select {
	case _, ok := <-notify:
		testutil.RequireFalse(t, ok, "channel should be closed immediately")
	default:
		t.Fatal("channel should be closed immediately, not block")
	}
}

func TestStreamService_DoneBeforeChunks(t *testing.T) {
	// liveChunks[msgID] is set to empty slice, then FinishStream is called
	// before any AppendChunk. StreamStatus should return done=true, ok=true.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_dbc@x.com", "StrmDbc")
	chat := createTestChat(t, f, "StrmDbc", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-dbc-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
	}

	// no AppendChunk called, but FinishStream sets done
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "", "")

	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	testutil.RequireTrue(t, ok, "stream should still exist")
	testutil.RequireTrue(t, done, "stream should be done")
}

func TestStreamService_StreamStatusIndexBounds(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idxb@x.com", "StrmIdxb")
	chat := createTestChat(t, f, "StrmIdxb", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b")
	defer mockAI.Close()

	msgID := "strm-idxb-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// negative index should not panic (cast to uint)
	t.Run("negative idx", func(t *testing.T) {
		_, _, ok := f.Server.Services.Stream.StreamStatus(msgID, -1)
		testutil.RequireTrue(t, ok, "should be ok even with negative idx")
	})

	// idx far beyond should return empty slices
	t.Run("idx beyond len", func(t *testing.T) {
		chunks, _, ok := f.Server.Services.Stream.StreamStatus(msgID, 100)
		testutil.RequireTrue(t, ok, "should be ok")
		testutil.RequireEqual(t, len(chunks), 0)
	})
}

func TestStreamService_GetMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_get@x.com", "StrmGet")
	chat := createTestChat(t, f, "StrmGet", a, []string{a})

	// Create a message via the service, then retrieve via GetMessage
	msg, err := f.Server.Services.Message.Send(context.Background(), chat.ID, a, "stored content", nil)
	testutil.RequireNoError(t, err)

	got, err := f.Server.Services.Stream.GetMessage(context.Background(), msg.ID)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, got.Content, "stored content")
}

func TestStreamService_GetMessage_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.Stream.GetMessage(context.Background(), "nonexistent")
	testutil.RequireError(t, err)
}

func TestStreamService_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_csu@x.com", "StrmCSU")
	chat := createTestChat(t, f, "StrmCSU", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c", "d", "e")
	defer mockAI.Close()

	msgID := "strm-csu-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// subscribe/unsubscribe in parallel with append
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := f.Server.Services.Stream.Subscribe(msgID)
			// read or timeout
			select {
			case <-sub:
			case <-time.After(2 * time.Second):
			}
			f.Server.Services.Stream.Unsubscribe(msgID, sub)
		}()
	}

	// consume and append
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	wg.Wait()
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "abcde", "")
}

func TestStreamService_SubscribeThenAppend(t *testing.T) {
	// subscribe BEFORE any chunks are appended should still get notified
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_sta@x.com", "StrmSta")
	chat := createTestChat(t, f, "StrmSta", a, []string{a})

	mockAI := startMockAIStream(t, "z")
	defer mockAI.Close()

	msgID := "strm-sta-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// subscribe before any append
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	// consume and append
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
	}()

	select {
	case <-notify:
	case <-time.After(time.Second):
		t.Fatal("subscriber should be notified")
	}
}

func TestStreamService_AppendChunk_NonexistentMessage(t *testing.T) {
	// AppendChunk for a msgID that was never started should not panic
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_appx@x.com", "StrmAppX")
	chat := createTestChat(t, f, "StrmAppX", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-appx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// append for a different msgID that doesn't exist
	f.Server.Services.Stream.AppendChunk("nonexistent-msg", "content", "should-not-panic")

	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")
}

func TestStreamService_Unsubscribe_NonexistentMessage(t *testing.T) {
	// Unsubscribe for a msgID that doesn't exist should not panic
	f := testutil.New(t)
	ch := make(chan struct{})
	f.Server.Services.Stream.Unsubscribe("nonexistent", ch)
}

func TestStreamService_Unsubscribe_NonexistentChannel(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_unx@x.com", "StrmUnX")
	chat := createTestChat(t, f, "StrmUnX", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-unx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)

	// subscribe then unsubscribe a different channel
	sub := f.Server.Services.Stream.Subscribe(msgID)
	other := make(chan struct{})
	f.Server.Services.Stream.Unsubscribe(msgID, other) // not panics
	f.Server.Services.Stream.Unsubscribe(msgID, sub)   // actual removal succeeds

	// now try to unsubscribe again — should be a no-op
	f.Server.Services.Stream.Unsubscribe(msgID, sub)
}

func TestStreamService_StartStream_HubNil(t *testing.T) {
	// when Hub is nil (not set up), StartStream should still work
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_hub@x.com", "StrmHub")
	chat := createTestChat(t, f, "StrmHub", a, []string{a})

	mockAI := startMockAIStream(t, "data")
	defer mockAI.Close()

	msgID := "strm-hub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	testutil.RequireNoError(t, err)
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	testutil.RequireEqual(t, got.String(), "data")
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "data", "")
}
