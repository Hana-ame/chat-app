package handlers

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	"text/html":               true,
	"application/xhtml+xml":   true,
	"image/svg+xml":           true,
	"text/xml":                true,
	"application/xml":         true,
	"text/javascript":         true,
	"application/javascript":  true,
	"application/x-javascript": true,
	"text/x-javascript":       true,
}

// File extensions that browsers may render/execute. Served as
// application/octet-stream with attachment disposition if ever found on disk
// (legacy files uploaded before the upload-time guard).
var dangerousExtensions = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true,
	".svg": true, ".xml": true,
	".js": true, ".mjs": true,
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

func (s *Server) aapiUploadResp(w http.ResponseWriter, r *http.Request, path string) {
	h := s.aapiHash(path)
	reqBase := s.aapiRequestBaseURL(r)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         h,
		"path":       "/" + path,
		"url":        reqBase + "/api/local/" + path,
		"delete_url": reqBase + "/api/local/" + path + "?delete=" + h,
	})
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

		fileCT := normalizeContentType(header.Header.Get("Content-Type"))
		if fileCT == "" {
			fileCT = "application/octet-stream"
		}
		if err := validateContentType(fileCT); err != nil {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
			return
		}

		fn := sanitizeUploadFilename(header.Filename, fileCT)

		ts := strconv.Itoa(int(time.Now().Unix()))
		path := ts + "/" + fn

		result, err := drv.Put(path, fileCT, file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		if result != nil && result.Path != "" {
			path = result.Path
		}

		s.aapiUploadResp(w, r, path)
		return
	}

	body := http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes)
	defer body.Close()

	ts := strconv.Itoa(int(time.Now().Unix()))
	fn := sanitizeUploadFilename(chi.URLParam(r, "*"), ct)
	path := ts + "/" + fn

	result, err := drv.Put(path, ct, body)
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
		path = result.Path
	}

	s.aapiUploadResp(w, r, path)
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

func (s *Server) AAPILocalFile(w http.ResponseWriter, r *http.Request) {
	drv := s.aapiUploadDriver()
	if drv == nil {
		http.NotFound(w, r)
		return
	}

	raw := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	if raw == "" {
		http.NotFound(w, r)
		return
	}

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
