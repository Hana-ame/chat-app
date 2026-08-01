package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type updateAvatarReq struct {
	AvatarURL string `json:"avatar_url"`
}

type updateBannerReq struct {
	BannerURL     string  `json:"banner_url"`
	BannerOpacity float64 `json:"banner_opacity"`
}

type updateBackgroundReq struct {
	BackgroundURL string `json:"background_url"`
}

func (s *Server) UpdateChatAvatar(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	var req updateAvatarReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, err := s.Services.Chat.UpdateAvatar(r.Context(), chatID, u.ID, req.AvatarURL)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) UpdateChatBanner(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	var req updateBannerReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, err := s.Services.Chat.UpdateBanner(r.Context(), chatID, u.ID, req.BannerURL, req.BannerOpacity)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) UpdateChatBackground(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r.Context())
	if u == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	chatID := chi.URLParam(r, "chatID")
	var req updateBackgroundReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	updated, err := s.Services.Chat.UpdateBackground(r.Context(), chatID, u.ID, req.BackgroundURL)
	if err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated)
}
