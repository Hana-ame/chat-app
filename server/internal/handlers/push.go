package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/service"
)

// VAPIDPublicKey godoc
// @Summary      Get the VAPID public key for Web Push subscriptions
// @Description  Return the server's VAPID public key so the client can call
//               PushManager.subscribe. Returns 503 when Web Push is not
//               configured (no VAPID env keys) — the client then silently
//               skips push registration.
// @Tags         push
// @Security     BearerAuth
// @Success      200  {object}  map[string]any
// @Failure      503  {object}  map[string]any
// @Router       /api/push/vapid-public-key [get]
func (s *Server) VAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.Services.Push.VAPIDPublicKey()
	if err != nil {
		if errors.Is(err, service.ErrPushNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "push_not_configured", "web push not configured")
			return
		}
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"vapid_public_key": key})
}

// SubscribePush godoc
// @Summary      Register a browser push subscription
// @Description  Persist the browser's PushSubscription (endpoint + p256dh +
//               auth) for the current user. Same endpoint re-registration
//               overwrites ownership. Returns 503 when Web Push is not
//               configured.
// @Tags         push
// @Security     BearerAuth
// @Param        body  body  object  true  "PushSubscription: {endpoint, p256dh, auth}"
// @Success      200  {object}  map[string]any
// @Failure      503  {object}  map[string]any
// @Router       /api/push/subscribe [post]
func (s *Server) SubscribePush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		P256DH   string `json:"p256dh"`
		Auth     string `json:"auth"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.Endpoint == "" || req.P256DH == "" || req.Auth == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "endpoint, p256dh and auth are required")
		return
	}
	u := userFrom(r.Context())
	created, err := s.Services.Push.Subscribe(r.Context(), u.ID, req.Endpoint, req.P256DH, req.Auth, time.Now())
	if err != nil {
		if errors.Is(err, service.ErrPushNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "push_not_configured", "web push not configured")
			return
		}
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"subscribed": true, "created": created})
}

// UnsubscribePush godoc
// @Summary      Remove a browser push subscription
// @Description  Delete the subscription identified by the endpoint, but only
//               if it belongs to the current user (idempotent when absent).
// @Tags         push
// @Security     BearerAuth
// @Param        endpoint  body  string  true  "Subscription endpoint to remove"
// @Success      200  {object}  map[string]any
// @Router       /api/push/subscribe [delete]
func (s *Server) UnsubscribePush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if req.Endpoint == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "endpoint is required")
		return
	}
	u := userFrom(r.Context())
	if err := s.Services.Push.Unsubscribe(r.Context(), u.ID, req.Endpoint); err != nil {
		status, code := mapServiceError(err)
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"unsubscribed": true})
}