# cmd/chatd 入口规范

> 原始来源：`server/cmd/chatd/main.go`

---

## 一、原始代码

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/auth"
	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/handlers"
	"github.com/Hana-ame/chat-app/server/internal/ws"

	_ "github.com/Hana-ame/chat-app/server/docs/swagger"
)

func main() {
	cfg := config.Load()
	log.Printf("chatd: starting (db=%s addr=%s)", cfg.DBPath, cfg.Addr)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer database.Close()

	authSvc := auth.New(cfg.JWTSecret, cfg.AccessTokenTTL)
	hub := ws.NewHub(database)
	gateway := ws.NewGateway(hub, database, authSvc)
	srv := handlers.New(cfg, database, authSvc, hub)
	r := srv.Router(gateway)

	if err := os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		log.Fatalf("upload dir: %v", err)
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
				log.Printf("purge tokens: %v", err)
			}
		}
	}()

	idle := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("chatd: shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		close(idle)
	}()

	log.Printf("chatd: listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen: %v", err)
	}
	<-idle
	log.Println("chatd: bye")
}
```

---

## 二、初始化流程

```
main()
  ├─ config.Load() → cfg
  ├─ db.Open(cfg.DBPath) → database
  ├─ auth.New(secret, ttl) → authSvc
  ├─ ws.NewHub(database) → hub
  ├─ ws.NewGateway(hub, db, authSvc) → gateway
  ├─ handlers.New(cfg, db, authSvc, hub) → srv
  ├─ srv.Router(gateway) → http.Handler
  ├─ os.MkdirAll(uploadDir)
  ├─ http.Server{Handler: r, Timeouts: ...}
  │
  ├─ goroutine: 定时清理过期 refresh token（1h）
  ├─ goroutine: 信号监听 (SIGINT/SIGTERM)
  └─ server.ListenAndServe()
```

---

## 三、HTTP Server 配置

| 参数 | 值 | 说明 |
|------|-----|------|
| ReadHeaderTimeout | `10s` | 读取请求头超时 |
| ReadTimeout | `15s` | 读取请求体超时 |
| WriteTimeout | `15s` | 写入响应超时 |
| IdleTimeout | `120s` | keep-alive 空闲超时 |

---

## 四、后台任务

| 任务 | 间隔 | 说明 |
|------|------|------|
| `PurgeExpiredTokens` | `1h` | 清理已过期的 refresh token（防止 DB 膨胀） |

---

## 五、优雅关闭

```
SIGINT / SIGTERM
  ├─ log "shutting down"
  ├─ http.Server.Shutdown(ctx) (10s timeout)
  └─ close(idle) → main 退出
```

---

## 六、依赖链

```
main
  ├─ config.Load
  ├─ db.Open
  │   └─ d.Migrate
  ├─ auth.New
  ├─ ws.NewHub
  ├─ ws.NewGateway
  ├─ handlers.New
  │   └─ srv.Router(gateway)
  │       ├─ chi.NewRouter
  │       └─ route registration
  ├─ http.Server
  └─ server.ListenAndServe
```

---

## 七、约束汇总

| 约束 | 说明 |
|------|------|
| 迁移 | 每次启动自动执行 `init.sql` |
| Swagger | 导入 `docs/swagger` 包注册路由 |
| 优雅关闭 | 支持 SIGINT/SIGTERM，最多等待 10s |
| 定时清理 | refresh token 每小时清理一次 |
| UploadDir | 启动时创建，不存在则 Fatal |