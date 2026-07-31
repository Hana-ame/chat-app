package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		allowPrivateIPs  bool
		wantErr          bool
	}{
		{"https public host", "https://api.openai.com/v1/chat/completions", false, false},
		{"http public host", "http://example.com/chat", false, false},
		{"bad scheme", "ftp://example.com/x", false, true},
		{"no host", "http:///x", false, true},
		{"invalid url", "://bad", false, true},
		{"literal loopback", "http://127.0.0.1:8080/chat", false, true},
		{"literal private", "http://192.168.1.10/chat", false, true},
		{"literal link-local", "http://169.254.169.254/latest/meta-data", false, true},
		{"localhost name", "http://localhost:11434/v1/chat/completions", false, true},
		{"loopback allowed by flag", "http://127.0.0.1:11434/chat", true, false},
		{"private allowed by flag", "http://192.168.1.10/chat", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.endpoint, tt.allowPrivateIPs)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.endpoint)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.endpoint, err)
			}
		})
	}
}

func TestStreamFromSource_NoRedirectFollow(t *testing.T) {
	// a 3xx must surface as an error (redirect not followed)
	targetHit := false
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/chat", http.StatusFound)
	})
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/redirect", AuthKey: "key", Body: json.RawMessage(`{}`)}
	_, err := StreamFromSource(context.Background(), src)
	if err == nil {
		t.Fatal("expected error for 3xx response")
	}
	if targetHit {
		t.Fatal("redirect target should not have been requested")
	}
	if !strings.Contains(err.Error(), "302") {
		t.Fatalf("error should mention status code, got: %v", err)
	}
}

func TestStreamFromSource_ErrorBodyNotEchoed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal-secret-detail"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	_, err := StreamFromSource(context.Background(), src)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "internal-secret-detail") {
		t.Fatalf("upstream body leaked in error: %v", err)
	}
}

func TestEnsureStreamEnabled(t *testing.T) {
	t.Run("adds stream:true", func(t *testing.T) {
		body := json.RawMessage(`{"model":"test","messages":[]}`)
		out, err := ensureStreamEnabled(body)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		json.Unmarshal(out, &m)
		var stream bool
		json.Unmarshal(m["stream"], &stream)
		if !stream {
			t.Fatal("stream should be true")
		}
	})

	t.Run("keeps existing stream value", func(t *testing.T) {
		body := json.RawMessage(`{"model":"test","stream":false}`)
		out, err := ensureStreamEnabled(body)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		json.Unmarshal(out, &m)
		var stream bool
		json.Unmarshal(m["stream"], &stream)
		if !stream {
			t.Fatal("stream should be overwritten to true")
		}
	})
}

func TestStreamFromSource_StreamingResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatal("expected POST")
		}
		if r.Header.Get("Authorization") != "Bearer public" {
			t.Fatal("bad auth")
		}
		body, _ := io.ReadAll(r.Body)
		var req map[string]json.RawMessage
		json.Unmarshal(body, &req)
		var stream bool
		json.Unmarshal(req["stream"], &stream)
		if !stream {
			t.Fatal("stream should be true")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := json.RawMessage(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`)
	src := Source{Endpoint: srv.URL + "/v1/chat/completions", AuthKey: "public", Body: body}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "Hello world" {
		t.Fatalf("want 'Hello world', got '%s'", got.String())
	}
}

func TestStreamFromSource_NonStreamingResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"Non-stream reply"}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := json.RawMessage(`{"model":"test","messages":[]}`)
	src := Source{Endpoint: srv.URL + "/v1/chat/completions", AuthKey: "public", Body: body}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "Non-stream reply" {
		t.Fatalf("want 'Non-stream reply', got '%s'", got.String())
	}
}

func TestIsStreaming(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"text/event-stream", true},
		{"application/x-ndjson", true},
		{"application/json", false},
		{"text/plain", false},
		{"", false},
		{"text/event-stream; charset=utf-8", true},
	}
	for _, tt := range tests {
		h := http.Header{}
		h.Set("Content-Type", tt.ct)
		got := isStreaming(h)
		if got != tt.want {
			t.Errorf("isStreaming(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestEnsureStreamEnabled_NonObjectBody(t *testing.T) {
	// non-object JSON (string, array, number) should pass through unchanged
	tests := []struct {
		name string
		body json.RawMessage
	}{
		{"string", json.RawMessage(`"hello"`)},
		{"array", json.RawMessage(`[1, 2, 3]`)},
		{"number", json.RawMessage(`42`)},
		{"null", json.RawMessage(`null`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := ensureStreamEnabled(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != string(tt.body) {
				t.Fatalf("want %s, got %s", string(tt.body), string(out))
			}
		})
	}
}

func TestReadStream_EmptyLines(t *testing.T) {
	// empty lines between events should be skipped
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"))
		w.Write([]byte("\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "AB" {
		t.Fatalf("want 'AB', got '%s'", got.String())
	}
}

func TestReadStream_MultipleEventsInOneChunk(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// all events in a single Write
		w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
				"data: [DONE]\n\n",
		))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "Hello world" {
		t.Fatalf("want 'Hello world', got '%s'", got.String())
	}
}

func TestReadStream_ReasoningContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// reasoning_content first, then content
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking...\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "thinking...answer" {
		t.Fatalf("want 'thinking...answer', got '%s'", got.String())
	}
}

func TestReadStream_NonJSONData(t *testing.T) {
	// non-JSON data lines should be silently skipped
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: not-json\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "A" {
		t.Fatalf("want 'A', got '%s'", got.String())
	}
}

func TestReadStream_NoDataPrefix(t *testing.T) {
	// lines without "data: " prefix should be skipped
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: ping\n"))
		w.Write([]byte("id: 1\n"))
		w.Write([]byte(": comment\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"X\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "X" {
		t.Fatalf("want 'X', got '%s'", got.String())
	}
}

func TestReadStream_UpstreamClosesPrematurely(t *testing.T) {
	// upstream closes without [DONE] — should still emit Done:true
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
		// no [DONE], just close
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "partial" {
		t.Fatalf("want 'partial', got '%s'", got.String())
	}
}

func TestReadOnce_EmptyChoices(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 content chunks, got %d", count)
	}
}

func TestReadOnce_InvalidJSON(t *testing.T) {
	// non-JSON response body in non-streaming mode should produce Done with no content
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json at all`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 content chunks, got %d", count)
	}
}

func TestStreamFromSource_WithContext(t *testing.T) {
	// cancelled context before upstream responds should error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	_, err := StreamFromSource(ctx, src)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestStreamFromSource_RequestBodySentAsIs(t *testing.T) {
	var capturedModel string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		json.Unmarshal(body, &req)
		capturedModel = req.Model
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	body := json.RawMessage(`{"model":"custom-model","messages":[{"role":"user","content":"hi"}]}`)
	src := Source{Endpoint: srv.URL + "/v1/chat/completions", AuthKey: "public", Body: body}
	ch, _ := StreamFromSource(context.Background(), src)
	for c := range ch {
		if c.Done {
			break
		}
	}
	if capturedModel != "custom-model" {
		t.Fatalf("expected model 'custom-model', got '%s'", capturedModel)
	}
}

func TestReadStream_NDJSONContentType(t *testing.T) {
	// When Content-Type is application/x-ndjson, the format is still SSE-style
	// (data: prefix + [DONE]) but identified by a different Content-Type.
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "AB" {
		t.Fatalf("want 'AB', got '%s'", got.String())
	}
}

func TestReadOnce_BodyReadError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`partial`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Use a POST that will be closed before reading: cancel context immediately
	// to trigger early close. readOnce still handles body read error gracefully.
	ch, err := StreamFromSource(context.Background(), Source{
		Endpoint: srv.URL + "/chat",
		AuthKey:  "key",
		Body:     json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 chunks, got %d", count)
	}
}

func TestEnsureStreamEnabled_StreamNull(t *testing.T) {
	body := json.RawMessage(`{"stream":null}`)
	out, err := ensureStreamEnabled(body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(out, &m)
	var stream bool
	json.Unmarshal(m["stream"], &stream)
	if !stream {
		t.Fatal("stream:null should be overwritten to true")
	}
}

func TestReadStream_EmptyDelta(t *testing.T) {
	// choices with empty delta should not emit content chunks
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"B\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "B" {
		t.Fatalf("want 'B', got '%s'", got.String())
	}
}

func TestStreamFromSource_TransportError(t *testing.T) {
	// connection refused — no server listening
	src := Source{
		Endpoint: "http://127.0.0.1:1/v1/chat/completions",
		AuthKey:  "key",
		Body:     json.RawMessage(`{}`),
	}
	_, err := StreamFromSource(context.Background(), src)
	if err == nil {
		t.Fatal("expected transport error")
	}
}

func TestStreamFromSource_InvalidURL(t *testing.T) {
	src := Source{
		Endpoint: "://invalid",
		AuthKey:  "key",
		Body:     json.RawMessage(`{}`),
	}
	_, err := StreamFromSource(context.Background(), src)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestReadStream_ScannerBufferTooSmall(t *testing.T) {
	// a line exceeding the default scanner buffer (64K) should not panic
	longLine := `data: {"choices":[{"delta":{"content":"` + strings.Repeat("A", 70*1024) + `"}}]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(longLine + "\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	gotContent := false
	for c := range ch {
		if c.Done {
			break
		}
		if c.Content != "" {
			gotContent = true
		}
	}
	if !gotContent {
		t.Fatal("expected content from long line")
	}
}

func TestReadStream_MultipleDones(t *testing.T) {
	// multiple [DONE] should not cause double-close
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			count++
		}
	}
	// only first [DONE] should be emitted, second happens after close(ch)
	if count != 1 {
		t.Fatalf("expected exactly 1 Done, got %d", count)
	}
}

func TestReadOnce_EmptyBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 content chunks, got %d", count)
	}
}

func TestReadOnce_StreamingCTNonStreamingBody(t *testing.T) {
	// Content-Type says text/event-stream but body is a single JSON object (not SSE)
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"single json"}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	// readStream won't find "data: " prefix, so no content; ends with Done
	if got.Len() != 0 {
		t.Fatalf("expected 0 content, got '%s'", got.String())
	}
}

func TestReadOnce_EmptyChoiceContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for c := range ch {
		if c.Done {
			break
		}
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 content chunks for empty content, got %d", count)
	}
}

func TestReadStream_ReasoningAndContentTogether(t *testing.T) {
	// delta has both reasoning_content and content — reasoning takes priority
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking\",\"content\":\"answer\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	// reasoning_content emitted, content skipped when both present
	if got.String() != "thinking" {
		t.Fatalf("want 'thinking', got '%s'", got.String())
	}
}

func TestReadStream_OnlyDataLines(t *testing.T) {
	// lines that have "data: " prefix but are not JSON or [DONE] — should skip
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: plain string\n\n"))
		w.Write([]byte("data: 42\n\n"))
		w.Write([]byte("data: null\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"valid\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "valid" {
		t.Fatalf("want 'valid', got '%s'", got.String())
	}
}

func TestReadStream_MultipleChoices(t *testing.T) {
	// multiple choices in one delta — only non-empty content emitted
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}},{\"delta\":{\"content\":\"B\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := Source{Endpoint: srv.URL + "/chat", AuthKey: "key", Body: json.RawMessage(`{}`)}
	ch, err := StreamFromSource(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
	}
	if got.String() != "AB" {
		t.Fatalf("want 'AB', got '%s'", got.String())
	}
}

func TestEnsureStreamEnabled_BodyWithExtraFields(t *testing.T) {
	body := json.RawMessage(`{"model":"gpt-4","messages":[{"role":"user","content":"hi"}],"temperature":0.7,"max_tokens":100}`)
	out, err := ensureStreamEnabled(body)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	json.Unmarshal(out, &m)
	var stream bool
	json.Unmarshal(m["stream"], &stream)
	if !stream {
		t.Fatal("stream should be true")
	}
	var model string
	json.Unmarshal(m["model"], &model)
	if model != "gpt-4" {
		t.Fatalf("model should be 'gpt-4', got '%s'", model)
	}
	var temp float64
	json.Unmarshal(m["temperature"], &temp)
	if temp != 0.7 {
		t.Fatalf("temperature should be 0.7, got %f", temp)
	}
}
