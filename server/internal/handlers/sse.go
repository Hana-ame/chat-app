package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// SSE godoc
// @Summary      SSE event stream
// @Description  Connect to server-sent events for real-time updates
// @Tags         sse
// @Security     BearerAuth
// @Param        access_token  query  string  true  "JWT access token"
// @Success      200  {string}  string  "text/event-stream"
// @Router       /api/events [get]
func (s *Server) SSE(w http.ResponseWriter, r *http.Request) {
	// Deprecated: URL query token leaks via server logs, browser history, and Referer headers.
	// Frontend should use Authorization header or cookie for the initial request.
	// Query string fallback is kept for EventSource API compatibility and will be removed in a future version.
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)

	userID := claims.UserID
	user, err := s.DB.GetUserByID(r.Context(), userID)
	if err != nil {
		return
	}

	chats, _ := s.DB.ListUserChats(r.Context(), userID)
	ready, _ := json.Marshal(map[string]any{
		"user": user, "chats": chats,
		"online_user_ids": s.Hub.OnlineUserIDs(),
	})
	fmt.Fprintf(w, "id: 0\nevent: ready\ndata: %s\n\n", ready)
	flusher.Flush()

	ch := make(chan []byte, 64)
	s.Hub.SSERegister(userID, ch)
	defer s.Hub.SSEUnregister(userID)

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
