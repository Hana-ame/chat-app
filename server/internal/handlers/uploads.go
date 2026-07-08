package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var allowedMime = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"video/mp4": true, "video/webm": true,
	"audio/mpeg": true, "audio/ogg": true, "audio/wav": true, "audio/webm": true,
	"application/pdf": true, "text/plain": true, "application/zip": true,
	"application/octet-stream": true,
}

func randomKey(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "_")
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

// Deprecated: frontend uploads directly to upload.moonchan.xyz.
// This handler is kept for fallback but no longer called.
// Remove in future version along with serveUpload, randomKey, sanitizeFilename, allowedMime.
//
// Upload godoc
// @Summary      Upload a file
// @Description  Upload a file via multipart form (validates mime type and size)
// @Tags         uploads
// @Security     BearerAuth
// @Param        file  formData  file  true  "File to upload"
// @Success      201  {object}  map[string]any
// @Failure      413  {object}  map[string]any
// @Failure      415  {object}  map[string]any
// @Router       /api/uploads [post]
func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(s.Cfg.MaxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("max %d bytes", s.Cfg.MaxUploadBytes))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "file field missing")
		return
	}
	defer file.Close()
	if header.Size > s.Cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "too_large", "")
		return
	}
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(header.Filename))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	mimeType = strings.SplitN(mimeType, ";", 2)[0]
	if !allowedMime[mimeType] {
		writeError(w, http.StatusUnsupportedMediaType, "bad_mime",
			"unsupported mime type "+mimeType)
		return
	}

	if err := os.MkdirAll(s.Cfg.UploadDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	ext := filepath.Ext(header.Filename)
	key := randomKey(16) + ext
	target := filepath.Join(s.Cfg.UploadDir, key)
	dst, err := os.Create(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	written, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		os.Remove(target)
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":        key,
		"url":       "/uploads/" + key,
		"filename":  sanitizeFilename(header.Filename),
		"mime_type": mimeType,
		"size":      written,
	})
}
