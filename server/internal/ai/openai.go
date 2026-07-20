package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

type OpenAIProvider struct {
	cfg SourceConfig
}

func NewOpenAI(cfg SourceConfig) *OpenAIProvider {
	return &OpenAIProvider{cfg: cfg}
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan Chunk, error) {
	body := p.buildBody(req)
	upstreamReq, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimRight(p.cfg.BaseURL, "/")+"/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("Authorization", "Bearer "+p.cfg.Key)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}

	ch := make(chan Chunk, 64)

	if isStreaming(resp.Header) {
		go p.readStream(resp, ch)
	} else {
		go p.readOnce(resp, ch)
	}

	return ch, nil
}

func (p *OpenAIProvider) buildBody(req ChatRequest) []byte {
	// use model from config, override if request specifies one
	var raw map[string]json.RawMessage
	json.Unmarshal(req.Raw, &raw)
	raw["model"] = mustJSON(p.cfg.Model)
	if raw["stream"] == nil {
		raw["stream"] = mustJSON(true)
	}
	b, _ := json.Marshal(raw)
	return b
}

func (p *OpenAIProvider) readStream(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := line[6:]
		if payload == "[DONE]" {
			ch <- Chunk{Done: true}
			return
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content         string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		for _, c := range event.Choices {
			if c.Delta.ReasoningContent != "" {
				ch <- Chunk{Content: c.Delta.ReasoningContent}
			} else if c.Delta.Content != "" {
				ch <- Chunk{Content: c.Delta.Content}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logutil.Error("ai openai stream read: %v", err)
	}
	ch <- Chunk{Done: true}
}

func (p *OpenAIProvider) readOnce(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logutil.Error("ai openai read: %v", err)
		ch <- Chunk{Done: true}
		return
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		for _, c := range result.Choices {
			if c.Message.Content != "" {
				ch <- Chunk{Content: c.Message.Content}
			}
		}
	}
	ch <- Chunk{Done: true}
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func isStreaming(h http.Header) bool {
	ct := h.Get("Content-Type")
	return strings.Contains(ct, "text/event-stream") || strings.Contains(ct, "application/x-ndjson")
}
