// 可复用的 httptest 假 AI 上游(OpenAI 风格 SSE)。
//
// 用途:service 层与 testutil 集成测试都需要一个"看起来像 OpenAI
// /v1/chat/completions"的假上游。历史上 service_test.go 与
// testutil/ai_stream_test.go 各写了一份,现已统一到这里:
//   - 校验方法必须是 POST
//   - 按 chunks 逐个返回 data: {choices:[{delta:{content}}]} 行
//   - 最后追加 data: [DONE]
//
// 用 t.Cleanup 自动关闭 server,不需要手动 Close。
package testkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// NewMockAIServer 返回一个模拟 AI 流式补全的 httptest server。
// chunks 中的每个字符串会作为一个 SSE data 行下发(choices/delta/content),
// 全部发完后追加 data: [DONE]。测试结束时自动关闭。
func NewMockAIServer(t *testing.T, chunks ...string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("mock AI: expected POST, got %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, c := range chunks {
			data, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]string{"content": c},
				}},
			})
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
