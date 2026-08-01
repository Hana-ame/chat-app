// Package handlers 覆盖 handler 层函数级单测(内部包,可直接测私有函数)。
//
// 测试范围:
//   - 上传相关纯函数:normalizeContentType / validateContentType /
//     sanitizeUploadFilename / contentTypeToExt(安全关键:危险类型必须被拒绝)
//   - aapiHash / aapiRequestBaseURL(上传响应 URL 构建)
//   - VersionHandler
//   - authMiddleware 的 401 分支(缺 token / 无效 token / 过期 token)
//
// 运行方式: cd server && go test ./internal/handlers/
// 说明:黑盒 HTTP 集成测试见 internal/testutil/handler_test.go,这里只测
// 函数级行为;涉及 DB/Service 的中间件路径由集成测试覆盖。
package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/testkit"
)

func TestNormalizeContentType(t *testing.T) {
	// 参数剥离、去空格、转小写:三种归一化必须同时生效。
	tests := []struct {
		input string
		want  string
	}{
		{"text/plain", "text/plain"},
		{"text/plain; charset=utf-8", "text/plain"},
		{"  IMAGE/PNG  ", "image/png"},
		{"", ""},
		{"application/json; boundary=xyz", "application/json"},
	}
	for _, tc := range tests {
		testkit.RequireEqual(t, normalizeContentType(tc.input), tc.want)
	}
}

func TestValidateContentType(t *testing.T) {
	// 白名单安全约束:危险可执行类型必须被拒绝,允许类型必须通过。
	allowed := []string{
		"text/plain", "application/pdf", "image/png", "image/jpeg",
		"video/mp4", "audio/mpeg", "application/octet-stream",
		"application/json", "application/zip",
	}
	for _, ct := range allowed {
		testkit.RequireNoError(t, validateContentType(ct))
	}
	dangerous := []string{
		"text/html", "application/xhtml+xml", "image/svg+xml",
		"text/xml", "application/xml", "text/javascript",
		"application/javascript", "application/x-javascript", "text/x-javascript",
		"", // 空类型必须报错(要求显式声明)
	}
	for _, ct := range dangerous {
		testkit.RequireError(t, validateContentType(ct))
	}
}

func TestSanitizeUploadFilename(t *testing.T) {
	// 上传文件名净化:原始扩展名必须被剥离,改由 content-type 推导的扩展名
	// 决定,防止存储的扩展名暗示可执行类型(serve 时 extension-derived)。
	tests := []struct {
		name string // 用例名
		fn   string // 原始文件名
		ct   string // 声明的 content type
		want string // 期望的净化结果
	}{
		{"plain file", "report.txt", "text/plain", "report.txt"},
		{"executable extension stripped", "shell.html", "text/plain", "shell.txt"},
		{"svg renamed to png", "image.svg", "image/png", "image.png"},
		{"path traversal basename", "../../../etc/passwd", "text/plain", "passwd.txt"},
		{"no extension", "README", "text/plain", "README.txt"},
		{"unknown type gets bin", "foo.xyz", "application/x-unknown", "foo.bin"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testkit.RequireEqual(t, sanitizeUploadFilename(tc.fn, tc.ct), tc.want)
		})
	}
}

func TestSanitizeUploadFilenameEmptyFallback(t *testing.T) {
	// 空/非法文件名回退到 uuid 前缀 + 合法扩展名。
	got := sanitizeUploadFilename("", "text/plain")
	testkit.RequireTrue(t, len(got) == 8+len(".txt") && got[len(got)-4:] == ".txt",
		"expected uuid(8)+.txt, got: "+got)
	got2 := sanitizeUploadFilename("/", "image/png")
	testkit.RequireTrue(t, len(got2) == 8+len(".png") && got2[len(got2)-4:] == ".png",
		"expected uuid(8)+.png, got: "+got2)
}

func TestContentTypeToExt(t *testing.T) {
	// content-type → 扩展名映射表。
	tests := []struct {
		ct   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"video/mp4", ".mp4"},
		{"audio/mpeg", ".mp3"},
		{"application/pdf", ".pdf"},
		{"text/plain", ".txt"},
		{"application/octet-stream", ".bin"},
		{"application/x-unknown", ".bin"},
		{"", ".bin"},
		{"IMAGE/PNG", ".png"}, // 大小写不敏感
	}
	for _, tc := range tests {
		testkit.RequireEqual(t, contentTypeToExt(tc.ct), tc.want)
	}
}

func TestAAPIHashDeterministic(t *testing.T) {
	// 同一路径在同一 salt 下哈希必须稳定(删除链接的可验证依据)。
	s := &Server{Cfg: &config.Config{UploadSalt: "test-salt"}}
	h1 := s.aapiHash("2026/08/01/x.png")
	h2 := s.aapiHash("2026/08/01/x.png")
	testkit.RequireEqual(t, h1, h2)

	// 不同路径必须产生不同哈希。
	h3 := s.aapiHash("2026/08/01/y.png")
	testkit.RequireNotEqual(t, h1, h3)

	// 输出为 16 位十六进制(8 字节)。
	testkit.RequireEqual(t, len(h1), 16)
	for _, c := range h1 {
		testkit.RequireTrue(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'), "hex char: "+string(c))
	}
}

func TestAAPIRequestBaseURL(t *testing.T) {
	// URL 构建优先级:显式 BaseURL 配置 > X-Forwarded-Proto + Host > http 兜底。
	s := &Server{Cfg: &config.Config{UploadSalt: "s"}}

	// 无配置:从请求头推导(X-Forwarded-Proto 优先)。
	r := httptest.NewRequest(http.MethodPost, "/api/upload/x", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Host = "chat.moonchan.xyz"
	testkit.RequireEqual(t, s.aapiRequestBaseURL(r), "https://chat.moonchan.xyz")

	// 无 X-Forwarded-Proto:默认 http。
	r2 := httptest.NewRequest(http.MethodPost, "/api/upload/x", nil)
	r2.Host = "localhost:8080"
	testkit.RequireEqual(t, s.aapiRequestBaseURL(r2), "http://localhost:8080")

	// 配置了 BaseURL:配置优先,忽略请求头(部署拓扑变化时的稳定依据)。
	s2 := &Server{Cfg: &config.Config{UploadSalt: "s", BaseURL: "https://files.example.com"}}
	testkit.RequireEqual(t, s2.aapiRequestBaseURL(r2), "https://files.example.com")
}

func TestVersionHandler(t *testing.T) {
	// GET /api/version 返回当前构建版本。
	s := &Server{Version: "v9.9.9-test"}
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	s.VersionHandler(rec, req)

	testkit.RequireEqual(t, rec.Code, http.StatusOK)
	testkit.RequireBodyContains(t, rec.Result(), `"version":"v9.9.9-test"`)
}

func TestAuthMiddlewareRejectsBadTokens(t *testing.T) {
	// authMiddleware 的 401 分支:缺 token / 垃圾 token / 过期 token。
	// 这三个分支在触达 DB 之前就返回,只需构造 Auth,不需要 Service。
	authSvc := auth.New([]byte("unit-test-secret-32bytes-long!!"), 5*time.Minute)

	// 过期 token:用负 TTL 签发一个已过期的 token。
	expAuth := auth.New([]byte("unit-test-secret-32bytes-long!!"), -1*time.Minute)
	expTok, _, err := expAuth.IssueAccessToken("u1")
	testkit.RequireNoError(t, err)

	cases := []struct {
		name     string
		token    string
		wantCode string
	}{
		{"missing token", "", "unauthorized"},
		{"garbage token", "not-a-jwt", "token_invalid"},
		{"expired token", expTok, "token_expired"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{Auth: authSvc}
			req := httptest.NewRequest(http.MethodGet, "/api/x", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			rec := httptest.NewRecorder()
			s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("middleware should not reach the next handler")
			})).ServeHTTP(rec, req)

			testkit.RequireEqual(t, rec.Code, http.StatusUnauthorized)
			testkit.RequireBodyContains(t, rec.Result(), tc.wantCode)
		})
	}
}
