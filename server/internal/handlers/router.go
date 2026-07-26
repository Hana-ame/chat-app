package handlers

import (
	_ "embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/ws"
	"github.com/go-chi/chi/v5"
	chimid "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

//go:embed swagger.json
var swaggerJSON []byte

// Router builds the HTTP handler with all routes and middleware.
func (s *Server) Router(gateway *ws.Gateway) http.Handler {
	r := chi.NewRouter()
	r.Use(chimid.RealIP)
	r.Use(chimid.RequestID)
	r.Use(chimid.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimid.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			u := userFrom(r.Context())
			uid := ""
			if u != nil {
				uid = logutil.SafeID(u.ID)
			}
			logutil.Info("%s %s %d %s [user=%s]", r.Method, r.URL.Path, ww.Status(), time.Since(start).Round(time.Millisecond), uid)
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'wasm-unsafe-eval' https://esm.sh https://static.cloudflareinsights.com; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: blob:; "+
					"connect-src "+s.Cfg.CSPConnectSrc+" https://esm.sh; "+
					"font-src 'self' data:;")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		echo := map[string]string{}
		for k, v := range r.Header {
			echo[k] = strings.Join(v, ", ")
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "echo": echo})
	})

	r.Route("/api", func(r chi.Router) {
		// Upload — 5min timeout (big files on slow connections)
		r.Group(func(r chi.Router) {
			r.Use(chimid.Timeout(s.Cfg.UploadTimeout))
			r.Get("/upload", s.AAPIUpload)
			r.Put("/upload", s.AAPIUpload)
			r.Put("/upload/*", s.AAPIUpload)
			r.Post("/upload", s.AAPIUpload)
			r.Get("/local/*", s.AAPILocalFile)
		})

		// All other API endpoints — 10s timeout, 120 req/min per IP
		r.Group(func(r chi.Router) {
			r.Use(chimid.Timeout(s.Cfg.APITimeout))
			r.Use(httprate.LimitByIP(120, 1*time.Minute))

			r.Get("/version", s.VersionHandler)
			r.With(httprate.LimitByIP(10, 1*time.Minute)).Post("/auth/login", s.Login)
			r.With(httprate.LimitByIP(5, 1*time.Minute)).Post("/auth/register", s.Register)
			r.Post("/auth/refresh", s.Refresh)

			r.Group(func(r chi.Router) {
				r.Use(s.authMiddleware)
				r.Post("/auth/logout", s.Logout)
				r.Get("/users/me", s.Me)
				r.Patch("/users/me", s.UpdateMe)
				r.With(rateLimitByUser(30, 1*time.Minute)).Get("/users", s.SearchUsers)

				r.Get("/chats/my", s.ListChats)
				r.Get("/chats/public", s.ListPublicChats)
				r.Post("/chats", s.CreateChat)
				r.Post("/dms", s.CreateOrGetDM)
				r.Route("/chats/{chatID}", func(r chi.Router) {
					r.Use(s.trackLastActive)
					r.Get("/", s.GetChat)
					r.Patch("/", s.RenameChat)
					r.Delete("/", s.DeleteChat)
					r.Get("/members", s.ListMembers)
					r.Post("/members", s.AddMember)
					r.Delete("/members/{userID}", s.RemoveMember)
					r.Post("/read", s.MarkRead)
					r.Get("/messages", s.ListMessages)
					r.With(rateLimitByUser(30, 1*time.Minute)).Post("/messages", s.SendMessage)
					r.Patch("/messages/{messageID}", s.EditMessage)
					r.Delete("/messages/{messageID}", s.DeleteMessage)
					r.Get("/messages/{messageID}/stream", s.StreamMessageContent)
					r.Put("/messages/{messageID}/reactions/{emoji}", s.AddReaction)
					r.Delete("/messages/{messageID}/reactions/{emoji}", s.RemoveReaction)
					r.Get("/messages/{messageID}/reactions", s.ListReactions)
					r.Post("/join", s.JoinChat)
					r.Post("/announcement", s.PinChat)
					r.Patch("/announcement", s.PinChat)
					r.Delete("/announcement", s.DeletePinnedChat)
					r.Post("/announcement/read", s.MarkPinnedRead)
					r.Post("/pin", s.PinChatList)
					r.Post("/unpin", s.UnpinChatList)
					r.Put("/avatar", s.UpdateChatAvatar)
					r.Put("/banner", s.UpdateChatBanner)
					r.Put("/background", s.UpdateChatBackground)
					r.Put("/notify", s.UpdateNotify)
				})
			}) // end auth group

			r.Get("/events", s.SSE)
		})
	}) // end /api route

	if gateway != nil {
		r.Get("/ws", gateway.ServeHTTP)
	} else {
		logutil.Warn("WebSocket gateway is nil, /ws disabled")
	}

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/swagger.json"),
	))
	r.Get("/swagger/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(swaggerJSON)
	})

	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/icon.svg", http.StatusFound)
	})

	if s.Cfg.StaticDir != "" {
		s.logStaticInfo()
		r.NotFound(s.serveStatic)
	}
	return r
}

// rateLimitByUser returns an httprate middleware that keys on the authenticated
// user ID. Falls back to IP for unauthenticated requests.
func rateLimitByUser(limit int, window time.Duration) func(http.Handler) http.Handler {
	return httprate.Limit(limit, window, httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
		if u := userFrom(r.Context()); u != nil {
			return "user:" + u.ID, nil
		}
		return "ip:" + r.RemoteAddr, nil
	}))
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
	clean := filepath.Clean("/" + r.URL.Path)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "" {
		rel = "index.html"
	}
	p := filepath.Join(s.Cfg.StaticDir, rel)
	if !strings.HasPrefix(p, s.Cfg.StaticDir) {
		http.NotFound(w, r)
		return
	}
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
