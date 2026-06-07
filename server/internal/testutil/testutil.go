// Package testutil provides shared helpers for backend tests.
package testutil

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/handlers"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type Fixture struct {
	Cfg     *config.Config
	DB      *db.DB
	Auth    *auth.Service
	Hub     *ws.Hub
	Gateway *ws.Gateway
	Server  *handlers.Server
	HTTP    *httptest.Server
}

func New(t *testing.T) *Fixture {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Addr:            ":0",
		DBPath:          filepath.Join(dir, "test.db"),
		UploadDir:       filepath.Join(dir, "uploads"),
		JWTSecret:       []byte("test-secret-very-secret-test-secret-very-secret"),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		MaxUploadBytes:  5 << 20,
		StaticDir:       "",
		AllowOrigins:    []string{"*"},
	}
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL)
	hub := ws.NewHub(database)
	gateway := ws.NewGateway(hub, database, authSvc)
	srv := handlers.New(cfg, database, authSvc, hub)

	httpSrv := httptest.NewServer(srv.Router(gateway))
	t.Cleanup(httpSrv.Close)

	return &Fixture{
		Cfg:     cfg,
		DB:      database,
		Auth:    authSvc,
		Hub:     hub,
		Gateway: gateway,
		Server:  srv,
		HTTP:    httpSrv,
	}
}

func (f *Fixture) Ctx() context.Context { return context.Background() }
