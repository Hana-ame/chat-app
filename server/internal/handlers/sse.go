package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
)

// SSE godoc
// @Summary      SSE event stream
// @Description  Connect to server-sent events for real-time updates
// @Tags         sse
// @Security     BearerAuth
// @Success      200  {string}  string  "text/event-stream"
// @Router       /api/events [get]
func (s *Server) SSE(w http.ResponseWriter, r *http.Request) {
	tok := bearerToken(r)
	if tok == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}
	claims, err := s.Auth.ParseAccessToken(tok)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
		return
	}

	userID := claims.UserID
	user, err := s.Services.User.GetByID(r.Context(), userID)
	if err != nil {
		status, code := mapServiceError(err)
		if status >= 500 {
			w.Header().Set("X-Error", err.Error())
		}
		writeError(w, status, code, err.Error())
		return
	}

	chats, err := s.Services.Chat.ListForUser(r.Context(), userID)
	var xErr string
	if err != nil {
		xErr = err.Error()
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		logutil.Error("SSE not supported for %s", logutil.SafeID(userID))
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if xErr != "" {
		w.Header().Set("X-Error", xErr)
	}
	w.WriteHeader(http.StatusOK)

	ready, _ := json.Marshal(map[string]any{
		"user": user, "chats": chats,
		"online_user_ids": s.Services.OnlineUserIDs(),
	})
	fmt.Fprintf(w, "id: 0\nevent: ready\ndata: %s\n\n", ready)
	flusher.Flush()

	logutil.Info("SSE connected: user=%s", logutil.SafeID(userID))

	ch := make(chan []byte, 64)
	s.Services.SSERegister(userID, ch)
	defer s.Services.SSEUnregister(userID)

	notify := r.Context().Done()
	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-notify:
			logutil.Info("SSE disconnected: user=%s", logutil.SafeID(userID))
			return
		case <-keepalive.C:
			fmt.Fprintf(w, ":keepalive\n\n")
			flusher.Flush()
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
