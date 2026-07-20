package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

type aiSourceProvider struct {
	cfg  ai.SourceConfig
	prov ai.Provider
}

type AIHandler struct {
	mu       sync.RWMutex
	providers map[string]*aiSourceProvider
}

func NewAIHandler(sources []ai.SourceConfig) *AIHandler {
	h := &AIHandler{providers: make(map[string]*aiSourceProvider, len(sources))}
	for _, s := range sources {
		h.providers[s.Name] = &aiSourceProvider{cfg: s, prov: ai.NewOpenAI(s)}
	}
	return h
}

func (h *AIHandler) Get(name string) *aiSourceProvider {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.providers[name]
}

func (s *Server) AIChat(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	if s.aiHandler == nil {
		writeError(w, http.StatusServiceUnavailable, "ai_not_configured", "AI chat is not configured")
		return
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "cannot read body")
		return
	}

	var req struct {
		Source   string          `json:"source"`
		Messages json.RawMessage `json:"messages"`
		Stream   bool            `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil || len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "messages required")
		return
	}

	sourceName := req.Source
	if sourceName == "" {
		sourceName = "default"
		for name := range s.aiHandler.providers {
			sourceName = name
			break
		}
	}
	p := s.aiHandler.Get(sourceName)
	if p == nil {
		writeError(w, http.StatusBadRequest, "unknown_source", "unknown AI source: "+sourceName)
		return
	}

	ch, err := p.prov.ChatStream(r.Context(), ai.ChatRequest{
		Messages: req.Messages,
		Stream:   req.Stream,
		Raw:      body,
	})
	if err != nil {
		logutil.Error("ai: source %s failed for user %s: %v", sourceName, u.ID[:8], err)
		writeError(w, http.StatusBadGateway, "upstream_error", "AI service error")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	for chunk := range ch {
		if chunk.Done {
			break
		}
		data, _ := json.Marshal(map[string]string{"content": chunk.Content})
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		flusher.Flush()
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}


