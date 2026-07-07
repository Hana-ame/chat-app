package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ws"
	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// Router builds the HTTP handler with all routes and middleware.
func (s *Server) Router(gateway *ws.Gateway) http.Handler {
	r := chi.NewRouter()
	r.Use(chimid.RealIP)
	r.Use(chimid.RequestID)
	r.Use(chimid.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(chimid.Timeout(30 * time.Second))
		r.Post("/auth/register", s.Register)
		r.Post("/auth/login", s.Login)
		r.Post("/auth/refresh", s.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/auth/logout", s.Logout)
			r.Get("/users/me", s.Me)
			r.Patch("/users/me", s.UpdateMe)
			r.Get("/users", s.SearchUsers)

			r.Get("/chats", s.ListChats)
			r.Get("/chats/public", s.ListPublicChats)
			r.Post("/chats", s.CreateChat)
			r.Post("/dms", s.CreateOrGetDM)
			r.Route("/chats/{chatID}", func(r chi.Router) {
				r.Get("/", s.GetChat)
				r.Patch("/", s.RenameChat)
				r.Delete("/", s.DeleteChat)
				r.Get("/members", s.ListMembers)
				r.Post("/members", s.AddMember)
				r.Delete("/members/{userID}", s.RemoveMember)
				r.Post("/read", s.MarkRead)
				r.Get("/messages", s.ListMessages)
				r.Post("/messages", s.SendMessage)
				r.Patch("/messages/{messageID}", s.EditMessage)
				r.Delete("/messages/{messageID}", s.DeleteMessage)
				r.Put("/messages/{messageID}/reactions/{emoji}", s.AddReaction)
				r.Delete("/messages/{messageID}/reactions/{emoji}", s.RemoveReaction)
				r.Post("/join", s.JoinChat)
				r.Post("/pin", s.PinChat)
				r.Post("/unpin", s.UnpinChat)
			})

			r.Post("/uploads", s.Upload)
		})
	})

	if gateway != nil {
		r.Get("/ws", gateway.ServeHTTP)
	}
	r.Get("/api/events", s.SSE)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("https://wsl-8080.moonchan.xyz/swagger/doc.json"),
	))

	r.Get("/uploads/*", s.serveUpload)
	if s.Cfg.StaticDir != "" {
		r.NotFound(s.serveStatic)
	}
	return r
}

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/uploads/")
	if rel == "" || strings.Contains(rel, "..") {
		http.NotFound(w, r)
		return
	}
	p := filepath.Join(s.Cfg.UploadDir, rel)
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	http.ServeFile(w, r, p)
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws") {
		http.NotFound(w, r)
		return
	}
	if s.Cfg.StaticDir == "" {
		http.NotFound(w, r)
		return
	}
	clean := strings.TrimPrefix(r.URL.Path, "/")
	if clean == "" {
		clean = "index.html"
	}
	p := filepath.Join(s.Cfg.StaticDir, clean)
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		http.ServeFile(w, r, p)
		return
	}
	idx := filepath.Join(s.Cfg.StaticDir, "index.html")
	if _, err := os.Stat(idx); err == nil {
		http.ServeFile(w, r, idx)
		return
	}
	http.NotFound(w, r)
}
