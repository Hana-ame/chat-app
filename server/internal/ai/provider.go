package ai

import (
	"context"
	"encoding/json"
)

type SourceConfig struct {
	Name    string `json:"name"`
	Key     string `json:"key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

type ChatRequest struct {
	Model    string          `json:"model"`
	Messages json.RawMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Raw      json.RawMessage `json:"-"` // original request body forwarded as-is
}

type Chunk struct {
	Content string // delta content
	Done    bool
}

type Provider interface {
	ChatStream(ctx context.Context, req ChatRequest) (<-chan Chunk, error)
}
