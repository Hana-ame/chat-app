// @title           Chat App API
// @version         0.1.0-beta
// @description     Real-time chat server with WebSocket and SSE
// @host            localhost:8080
// @BasePath        /api
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/handlers"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/ws"

	_ "github.com/Hana-ame/chat-app/server/docs/swagger"
)

// Version is set at build time via -ldflags -X main.Version=build-xxxxx.
// Default "dev" for local builds.
var Version = "dev"

func main() {
	cfg := config.Load()
	logutil.Info("chatd: %s starting (db=%s addr=%s)", Version, cfg.DBPath, cfg.Addr)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logutil.Fatal("db open: %v", err)
	}
	defer database.Close()

	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL)
	hub := ws.NewHub(database)
	gateway := ws.NewGateway(hub, database, authSvc)
	srv := handlers.New(cfg, database, authSvc, hub)
	srv.Version = Version
	r := srv.Router(gateway)

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil { // Deprecated: frontend uploads directly to upload.moonchan.xyz. Remove in future version.
		logutil.Fatal("upload dir: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := database.PurgeExpiredTokens(context.Background()); err != nil {
				logutil.Warn("purge tokens: %v", err)
			}
		}
	}()

	idle := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		logutil.Info("chatd: shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		close(idle)
	}()

	logutil.Info("chatd: listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logutil.Fatal("listen: %v", err)
	}
	<-idle
	logutil.Info("chatd: bye")
}
