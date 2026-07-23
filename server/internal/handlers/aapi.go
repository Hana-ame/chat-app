package handlers

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"io"
	"mime"
	"net/http"
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
	"image/jpeg":           ".jpg",
	"image/png":            ".png",
	"image/gif":            ".gif",
	"image/webp":           ".webp",
	"video/mp4":            ".mp4",
	"video/webm":           ".webm",
	"audio/mpeg":           ".mp3",
	"audio/ogg":            ".ogg",
	"audio/wav":            ".wav",
	"application/pdf":      ".pdf",
	"text/plain":           ".txt",
	"application/zip":      ".zip",
	"application/json":     ".json",
	"application/octet-stream": ".bin",
}

func (s *Server) aapiUploadDriver() *localfs.Driver {
	if s.aapiLocalDriver == nil {
		d, err := localfs.New(s.Cfg.UploadDir)
		if err != nil {
			logutil.Error("aapi local driver init: %v", err)
			return nil
		}
		s.aapiLocalDriver = d
	}
	return s.aapiLocalDriver
}

func (s *Server) aapiHash(path string) string {
	h := sha256.Sum256([]byte(path + s.Cfg.UploadSalt))
	return fmt.Sprintf("%x", h[:8])
}

func (s *Server) aapiRequestBaseURL(r *http.Request) string {
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

	publicURL := s.Cfg.UploadPublicURL
	if publicURL == "" {
		publicURL = reqBase
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":         h,
		"path":       "/" + path,
		"url":        publicURL + "/api/local/" + path,
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

	ct := r.Header.Get("Content-Type")

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

		fn := header.Filename
		if fn == "" {
			fn = uuid.New().String()[:8] + contentTypeToExt(header.Header.Get("Content-Type"))
		}

		ts := strconv.Itoa(int(time.Now().Unix()))
		path := ts + "/" + fn

		result, err := drv.Put(path, header.Header.Get("Content-Type"), file)
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

	body := r.Body
	defer body.Close()

	ts := strconv.Itoa(int(time.Now().Unix()))
	fn := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	if fn == "" || fn == "/" {
		fn = uuid.New().String()[:8] + contentTypeToExt(ct)
	}
	path := ts + "/" + fn

	result, err := drv.Put(path, ct, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if result != nil && result.Path != "" {
		path = result.Path
	}

	s.aapiUploadResp(w, r, path)
}

func contentTypeToExt(ct string) string {
	ct = strings.SplitN(ct, ";", 2)[0]
	ct = strings.TrimSpace(ct)
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

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, file)
}
