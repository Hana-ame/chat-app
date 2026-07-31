package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

var aiHTTPClient = &http.Client{
	Timeout: 5 * time.Minute,
	// Never follow redirects: an attacker-controlled endpoint must not be
	// able to redirect the server to an internal address.
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type Source struct {
	Endpoint string          `json:"endpoint"`
	AuthKey  string          `json:"auth_key"`
	Body     json.RawMessage `json:"body"`
}

type Chunk struct {
	Type    string // "content" or "reasoning"
	Content string
	Done    bool
}

func StreamFromSource(ctx context.Context, src Source) (<-chan Chunk, error) {
	body, err := ensureStreamEnabled(src.Body)
	if err != nil {
		return nil, fmt.Errorf("invalid body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", src.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+src.AuthKey)

	resp, err := aiHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		// Do not echo the upstream response body: it may leak internal
		// service details when probing internal endpoints.
		return nil, fmt.Errorf("upstream returned HTTP %d", resp.StatusCode)
	}

	ctx, cancel := context.WithCancel(ctx)
	ch := make(chan Chunk, 64)

	go func() {
		<-ctx.Done()
		resp.Body.Close()
	}()

	if isStreaming(resp.Header) {
		go func() {
			readStream(resp, ch)
			cancel()
		}()
	} else {
		go func() {
			readOnce(resp, ch)
			cancel()
		}()
	}

	return ch, nil
}

// ValidateEndpoint rejects endpoints that are not http/https or that resolve
// to private / loopback / link-local / reserved addresses, preventing the
// server from being used as an SSRF proxy into internal networks.
func ValidateEndpoint(endpoint string, allowPrivateIPs bool) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("endpoint must use http or https scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("endpoint host is required")
	}
	if allowPrivateIPs {
		return nil
	}
	ips, err := net.LookupHost(u.Hostname())
	if err != nil {
		return fmt.Errorf("cannot resolve endpoint host: %w", err)
	}
	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		if isRestrictedIP(parsed) {
			return fmt.Errorf("endpoint host %q resolves to a private or reserved address", u.Host)
		}
	}
	return nil
}

func isRestrictedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

func ensureStreamEnabled(body json.RawMessage) (json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return body, nil
	}
	raw["stream"] = mustJSON(true)
	b, _ := json.Marshal(raw)
	return b, nil
}

func readStream(resp *http.Response, ch chan<- Chunk) {
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
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		for _, c := range event.Choices {
			if c.Delta.ReasoningContent != "" {
				ch <- Chunk{Type: "reasoning", Content: c.Delta.ReasoningContent}
			} else if c.Delta.Content != "" {
				ch <- Chunk{Type: "content", Content: c.Delta.Content}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logutil.Error("ai stream read: %v", err)
	}
	ch <- Chunk{Done: true}
}

func readOnce(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logutil.Error("ai read: %v", err)
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
