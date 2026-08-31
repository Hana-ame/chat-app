package handlers

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	localfs "github.com/Hana-ame/chat-app/server/internal/storage/local"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

//go:embed upload.html
var uploadHTML string

var contentTypeExt = map[string]string{
	"image/jpeg":               ".jpg",
	"image/png":                ".png",
	"image/gif":                ".gif",
	"image/webp":               ".webp",
	"video/mp4":                ".mp4",
	"video/webm":               ".webm",
	"audio/mpeg":               ".mp3",
	"audio/ogg":                ".ogg",
	"audio/wav":                ".wav",
	"application/pdf":          ".pdf",
	"text/plain":               ".txt",
	"application/zip":          ".zip",
	"application/json":         ".json",
	"application/octet-stream": ".bin",
}

// Content types that browsers would render/execute from the same origin.
// Files with these types are rejected at upload time so they can never be
// served inline as script under /api/local/.
var dangerousContentTypes = map[string]bool{
	"text/html":                true,
	"application/xhtml+xml":    true,
	"image/svg+xml":            true,
	"text/xml":                 true,
	"application/xml":          true,
	"text/javascript":          true,
	"application/javascript":   true,
	"application/x-javascript": true,
	"text/x-javascript":        true,
}

// File extensions that browsers may render/execute. Served as
// application/octet-stream with attachment disposition if ever found on disk
// (legacy files uploaded before the upload-time guard).
var dangerousExtensions = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true,
	".svg": true, ".xml": true,
	".js": true, ".mjs": true,
}

// Content types that, when served inline, browsers may execute.
// CSP sandbox is applied as an extra guard.
var sandboxableContentTypes = map[string]bool{
	"text/html":          true,
	"application/xhtml+xml": true,
	"image/svg+xml":      true,
	"text/xml":           true,
	"application/xml":    true,
}

func (s *Server) aapiUploadDriver() *localfs.Driver {
	s.aapiLocalOnce.Do(func() {
		d, err := localfs.New(s.Cfg.UploadDir)
		if err != nil {
			logutil.Error("aapi local driver init: %v", err)
			return
		}
		s.aapiLocalDriver = d
	})
	return s.aapiLocalDriver
}

func (s *Server) aapiHash(path string) string {
	h := sha256.Sum256([]byte(path + s.Cfg.UploadSalt))
	return fmt.Sprintf("%x", h[:8])
}

func (s *Server) aapiRequestBaseURL(r *http.Request) string {
	if s.Cfg.BaseURL != "" {
		return s.Cfg.BaseURL
	}
	scheme := "http"
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// 【本地改动 2026-09-02】公开稳定 URL 上传响应：URL 直接嵌 UUID assetID（即凭证，
// 无 ticket、无成员校验，assetID 不可枚举 = 事实安全）；浏览器/CDN 可 long-term
// 缓存（max-age=31536000, immutable）。与旧 /api/local/{ts}/{fn} 并存，旧 URL
// 仍可通过 legacy handler 访问。
// 目的：给附件一个稳定、可缓存、CDN 友好的公开 URL，前端可直接嵌 <img src={url}>
// 无需签名计算；CDN 侧按 assetID 命中后可跨用户共享缓存。
// 思路：uuid 4 字节随机 ~1.8e19 空间，暴力枚举不可行，等价凭证。
// 边界：只影响上传后的新文件；旧 /api/local/ URL 不受影响（legacy handler 仍在）。
func (s *Server) aapiUploadResp(w http.ResponseWriter, r *http.Request, assetID, filename, path, mimeType string, size int64) {
	reqBase := s.aapiRequestBaseURL(r)
	stableURL := reqBase + "/assets/files/" + assetID + "/" + urlPathEscape(filename)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        assetID,
		"filename":  filename,
		"mime_type": mimeType,
		"size":      size,
		"url":       stableURL,
		// 【本地改动 2026-09-02】保留 delete_url 字段但改为 /api/files/{assetID} 的
		// Bearer-认证的 DELETE 端点（旧 ?delete=hash 是路径凭据，新 assetID 已是凭证，
		// 不再需要 hash）。
		"delete_url": reqBase + "/api/files/" + assetID,
	})
}

// 【本地改动 2026-09-02】公开稳定附件路径：/assets/files/{assetID}/{fn.ext}
// 构造。assetID 是 UUIDv4，作为公开凭证；尾部 {fn.ext} 仅用于可读性 + CDN
// cache-key 差异（服务端实际只按 assetID 解析，忽略文件名）。
// 踩坑：transformPath 只能传尾段，不能传完整路径（2026-08-23 部署后
// 双重前缀导致 404）。这里没有 transformPath 参数，保持简单。
func stableAssetPath(assetID, filename string) string {
	return "uploads/" + assetID + "/" + filename
}

func (s *Server) AAPIUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(uploadHTML))
		return
	}

	drv := s.aapiUploadDriver()
	if drv == nil {
		writeError(w, http.StatusInternalServerError, "internal", "upload storage not available")
		return
	}

	ct := normalizeContentType(r.Header.Get("Content-Type"))
	if err := validateContentType(ct); err != nil {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
		return
	}

	var fileCT, filename string
	var body io.ReadCloser
	var size int64

	if strings.HasPrefix(ct, "multipart/form-data") {
		if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "no file in form")
			return
		}
		defer file.Close()

		fileCT = normalizeContentType(header.Header.Get("Content-Type"))
		if fileCT == "" {
			fileCT = "application/octet-stream"
		}
		if err := validateContentType(fileCT); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		filename = sanitizeUploadFilename(header.Filename, fileCT)
		body = file
		size = int64(header.Size)
	} else {
		body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes)
		defer body.Close()

		filename = sanitizeUploadFilename(chi.URLParam(r, "*"), ct)
		fileCT = ct
		size = s.Cfg.MaxUploadBytes // 上限估算，实际可能更小；仅用于响应提示
	}

	// 【本地改动 2026-09-02】assetID 使用 UUIDv4：不可枚举 + 作为公开 URL 凭证。
	// 存盘路径 uploads/{assetID}/{filename}（uuid 目录隔离，文件名仍来自
	// Content-Type 推导，防止 extension-mismatch XSS）。
	assetID := uuid.New().String()
	diskPath := stableAssetPath(assetID, filename)

	result, err := drv.Put(diskPath, fileCT, body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", fmt.Sprintf("max %d bytes", s.Cfg.MaxUploadBytes))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if result != nil && result.Path != "" {
		diskPath = result.Path
	}

	s.aapiUploadResp(w, r, assetID, filename, diskPath, fileCT, size)
}

func normalizeContentType(ct string) string {
	ct = strings.SplitN(ct, ";", 2)[0]
	return strings.TrimSpace(strings.ToLower(ct))
}

func validateContentType(ct string) error {
	if ct == "" {
		return errors.New("missing content type")
	}
	if dangerousContentTypes[ct] {
		return fmt.Errorf("content type %q is not allowed", ct)
	}
	return nil
}

// sanitizeUploadFilename strips any original extension and appends one derived
// from the validated content type, so the stored extension can never imply an
// executable content type at serve time (extension-derived).
func sanitizeUploadFilename(fn, ct string) string {
	base := filepath.Base(fn)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" || base == "." || base == "/" {
		base = uuid.New().String()[:8]
	}
	return base + contentTypeToExt(ct)
}

func contentTypeToExt(ct string) string {
	ct = normalizeContentType(ct)
	if ext, ok := contentTypeExt[ct]; ok {
		return ext
	}
	exts, _ := mime.ExtensionsByType(ct)
	for _, ext := range exts {
		return ext
	}
	return ".bin"
}

// 【本地改动 2026-09-02】ServeAssetsFile 处理两个路由：
//
//   - GET /assets/files/{assetID}/{filename}  （公开稳定 URL，CDN 可缓存）
//   - GET /assets/files/{assetID}             （兜底，需知道实际文件名，
//                                               通过 ls 目录推断，仅兼容旧客户端）
//
// 无认证（assetID 即凭证）；ETag = assetID（不可变 URL 保证 ETag 稳定）；
// Cache-Control = public, max-age=31536000, immutable（CDN 永久缓存）。
// Accept-Ranges: none（本地顺序流，非 S3 字节范围）。
// 安全头：X-Content-Type-Options: nosniff；HTML/XML/SVG 加 CSP sandbox。
func (s *Server) ServeAssetsFile(w http.ResponseWriter, r *http.Request) {
	drv := s.aapiUploadDriver()
	if drv == nil {
		http.NotFound(w, r)
		return
	}

	assetID := chi.URLParam(r, "assetID")
	if assetID == "" {
		http.NotFound(w, r)
		return
	}
	// 验证 UUID 格式：拒绝路径遍历 + 非 UUID 输入（如 ..、sql、空串）
	if _, err := uuid.Parse(assetID); err != nil {
		http.NotFound(w, r)
		return
	}

	filename := chi.URLParam(r, "filename")
	if filename == "" {
		// 无文件名兜底：尝试从磁盘目录列出第一个文件
		listing, err := filepath.Glob(filepath.Join(s.Cfg.UploadDir, "uploads", assetID, "*"))
		if err != nil || len(listing) == 0 {
			http.NotFound(w, r)
			return
		}
		filename = filepath.Base(listing[0])
	}
	// 防止 filename 里带路径分隔符（chi 已吞掉 "/" 但保留 ".."）
	filename = filepath.Base(filename)

	diskPath := stableAssetPath(assetID, filename)
	file, _, contentType, err := drv.Get(diskPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	// 【本地改动 2026-09-02】公开 immutable 缓存（1 年）+ ETag 用 assetID
	// （URL 不可变 = ETag 稳定；CDN 用 ETag 校验可永久命中）。
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+assetID+`"`)
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	ct := normalizeContentType(contentType)
	if assetNeedsSandbox(ct) {
		w.Header().Set("Content-Security-Policy", "sandbox")
	}

	// 即使文件扩展名危险，也按真实 contentType 投递（CSP sandbox 兜底）。
	// 旧 /api/local 的 dangerousExtensions 拦截逻辑不适用于此公开路径。
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
}

// ServeAssetsFileLegacy handles the legacy /api/local/{path} GET for backwards
// compatibility with old messages whose attachments URL uses the old pattern.
// Delete via ?delete=hash is handled separately in AAPILocalFile below.
func (s *Server) ServeAssetsFileLegacy(w http.ResponseWriter, r *http.Request) {
	drv := s.aapiUploadDriver()
	if drv == nil {
		http.NotFound(w, r)
		return
	}

	// Legacy path: /api/local/{ts}/{fn.ext}
	raw := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	if raw == "" {
		http.NotFound(w, r)
		return
	}

	// Legacy delete via ?delete=hash
	if key := r.URL.Query().Get("delete"); key != "" {
		if key != s.aapiHash(raw) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "key is wrong")
			return
		}
		if err := drv.Delete(raw); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusGone, map[string]string{"status": "gone"})
		return
	}

	file, _, contentType, err := drv.Get(raw)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(raw))
	if dangerousExtensions[ext] {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
	} else {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "inline")
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
}

// 【本地改动 2026-09-02】DeleteAssetsFile 用 Bearer 鉴权的 DELETE 端点，
// 替代旧的 /api/local/{path}?delete=hash（hash 是路径凭据，新 assetID 已是凭证，
// 无需额外 hash）。用于消息删除时级联清理附件文件；也可被管理员手动调用。
func (s *Server) DeleteAssetsFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.NotFound(w, r)
		return
	}
	// 鉴权：需要 Bearer token（用户需已登录）
	if userFrom(r.Context()) == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "login required")
		return
	}
	assetID := chi.URLParam(r, "assetID")
	if assetID == "" {
		http.NotFound(w, r)
		return
	}
	if _, err := uuid.Parse(assetID); err != nil {
		http.NotFound(w, r)
		return
	}

	drv := s.aapiUploadDriver()
	if drv == nil {
		writeError(w, http.StatusInternalServerError, "internal", "upload storage not available")
		return
	}

	// 列出目录并删除所有文件（一个 assetID 目录下可能只有一个文件）
	dir := filepath.Join(s.Cfg.UploadDir, "uploads", assetID)
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var errs []error
	for _, e := range entries {
		if err := drv.Delete("uploads/" + assetID + "/" + filepath.Base(e)); err != nil {
			errs = append(errs, err)
		}
	}
	// 清理空目录
	if len(entries) == 0 {
		_ = os.Remove(dir)
	}
	if len(errs) > 0 && len(errs) == len(entries) {
		writeError(w, http.StatusNotFound, "not_found", "asset not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// assetNeedsSandbox checks whether the MIME type should be served with a
// sandbox CSP to prevent inline script execution (HTML/XML/SVG).
func assetNeedsSandbox(ct string) bool {
	if sandboxableContentTypes[ct] {
		return true
	}
	return strings.HasSuffix(ct, "+xml")
}

// urlPathEscape escapes a filename for use in a URL path segment.
func urlPathEscape(s string) string {
	// chi 支持通配符路由，filename 已不含 /；这里只防特殊字符。
	return strings.ReplaceAll(s, " ", "%20")
}

// 【本地改动 2026-09-02】assetIDFromURL 从旧 /api/local/ 或新 /assets/files/
// URL 中提取磁盘路径（用于消息删除级联清理附件）。
// 返回两个值：(diskPath, ok)；ok=false 表示无法识别的 URL 格式（跳过）。
// 踩坑：附件 URL 可能来自客户端构造（旧 /api/local 或新 /assets/files），
// 两种模式都要处理，保证删除消息时附件磁盘不泄漏。
func assetIDFromURL(url string) (string, string) {
	// 新 URL: /assets/files/{assetID}/{filename}
	re := regexp.MustCompile(`/assets/files/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})/(.+)$`)
	if m := re.FindStringSubmatch(url); m != nil {
		return "uploads/" + m[1] + "/" + m[2], m[1]
	}
	// 旧 URL: /api/local/{ts}/{filename}
	re2 := regexp.MustCompile(`/api/local/(.+)$`)
	if m := re2.FindStringSubmatch(url); m != nil {
		return m[1], ""
	}
	return "", ""
}

// 【本地改动 2026-09-02】cleanupAttachmentsOnDelete 在消息软删除前，
// 解析 attachments JSON 中的每条 URL，将磁盘上的文件删除（级联清理）。
// 目的：消息删除后附件文件不再可达，应同时清理磁盘，防止磁盘膨胀。
// 边界：只清理新 URL 模式的 assetID 文件和旧 URL 模式的 legacy 路径。
//       无法解析的 URL 跳过（不报错），不影响消息删除本身。
func (s *Server) cleanupAttachmentsOnDelete(ctx context.Context, messageID string) {
	msg, err := s.DB.GetMessage(ctx, messageID)
	if err != nil {
		return // 消息不存在/不可读，跳过
	}
	if len(msg.Attachments) == 0 {
		return
	}
	var attachments []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(msg.Attachments, &attachments); err != nil {
		return
	}
	drv := s.aapiUploadDriver()
	if drv == nil {
		return
	}
	for _, a := range attachments {
		diskPath, _ := assetIDFromURL(a.URL)
		if diskPath == "" {
			continue
		}
		_ = drv.Delete(diskPath)
	}
}
