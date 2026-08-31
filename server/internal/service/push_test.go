package service_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
	"github.com/SherClockHolmes/webpush-go"
)

// 【本地改动 2026-08-31】Web Push 移植回归测试。发现背景：chat-app 原本
// 没有服务端推送，离线用户收不到"提及/回复"通知；实现 push
// 机制（VAPID 订阅 + 离线投递 + 410 失效清理）时新增本文件，覆盖：
//   - 未配置 VAPID 时 Subscribe 拒绝（对应 503）、PushForOfflineUser 静默跳过；
//   - endpoint 为唯一键——同一端点重复注册覆盖、不产生重复订阅；
//   - 离线投递真实走到推送服务；推送服务返回 410（订阅失效）时订阅被删除，
//     不会反复打空。
//
// 说明：Push 方法读 s.Cfg（同一指针），测试里直接改 f.Cfg.PushVAPID* 即可
// 注入配置；VAPID 密钥必须是真实生成的（webpush.GenerateVAPIDKeys），否则
// webpush 库加密失败、请求根本发不到 mock 推送服务。

// generateP256DH 生成合法 RFC 8291 p256dh（P-256 非压缩公钥，65 字节）。
func generateP256DH(t *testing.T) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testutil.RequireNoError(t, err)
	return base64.RawURLEncoding.EncodeToString(
		elliptic.Marshal(elliptic.P256(), priv.X, priv.Y))
}

// generateAuth 生成合法 auth（16 字节随机）。
func generateAuth(t *testing.T) string {
	t.Helper()
	b := make([]byte, 16)
	_, err := rand.Read(b)
	testutil.RequireNoError(t, err)
	return base64.RawURLEncoding.EncodeToString(b)
}

func TestPushService_SubscribeRequiresConfiguredVAPID(t *testing.T) {
	f := testutil.New(t)
	alice := createTestUser(t, f, "push1@x.test", "PushAlice")

	// 未配置 VAPID：订阅必须被拒绝（对应 503）。
	_, err := f.Server.Services.Push.Subscribe(f.Ctx(), alice,
		"https://push.example/endpoint-1", generateP256DH(t), generateAuth(t), time.Now())
	if !errors.Is(err, service.ErrPushNotConfigured) {
		t.Fatalf("err = %v, want ErrPushNotConfigured", err)
	}
}

func TestPushService_SubscribeIdempotentPerEndpoint(t *testing.T) {
	f := testutil.New(t)
	alice := createTestUser(t, f, "push2@x.test", "PushAlice")

	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	testutil.RequireNoError(t, err)
	f.Cfg.PushVAPIDPublicKey = publicKey
	f.Cfg.PushVAPIDPrivateKey = privateKey
	f.Cfg.PushVAPIDSubject = "mailto:push@x.test"

	endpoint := "https://push.example/endpoint-2"
	p256dh, auth := generateP256DH(t), generateAuth(t)

	created, err := f.Server.Services.Push.Subscribe(f.Ctx(), alice, endpoint, p256dh, auth, time.Now())
	testutil.RequireNoError(t, err)
	if !created {
		t.Fatalf("first subscribe should create")
	}

	// 同一 endpoint 再注册：覆盖（created=false），不产生第二行。
	created, err = f.Server.Services.Push.Subscribe(f.Ctx(), alice, endpoint, p256dh, auth, time.Now())
	testutil.RequireNoError(t, err)
	if created {
		t.Fatalf("re-subscribe same endpoint must not create a new row")
	}
	n, err := f.DB.CountPushSubscriptionsByUser(f.Ctx(), alice)
	testutil.RequireNoError(t, err)
	if n != 1 {
		t.Fatalf("subscriptions = %d, want 1", n)
	}

	// 退订后清空（幂等：再退一次不报错）。
	err = f.Server.Services.Push.Unsubscribe(f.Ctx(), alice, endpoint)
	testutil.RequireNoError(t, err)
	err = f.Server.Services.Push.Unsubscribe(f.Ctx(), alice, endpoint)
	testutil.RequireNoError(t, err)
	n, err = f.DB.CountPushSubscriptionsByUser(f.Ctx(), alice)
	testutil.RequireNoError(t, err)
	if n != 0 {
		t.Fatalf("subscriptions after unsubscribe = %d, want 0", n)
	}
}

func TestPushService_PushForOfflineUser_410RemovesSubscription(t *testing.T) {
	f := testutil.New(t)
	alice := createTestUser(t, f, "push3@x.test", "PushAlice")

	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	testutil.RequireNoError(t, err)
	f.Cfg.PushVAPIDPublicKey = publicKey
	f.Cfg.PushVAPIDPrivateKey = privateKey
	f.Cfg.PushVAPIDSubject = "mailto:push@x.test"

	// 模拟推送服务：收到请求就回 410 Gone（订阅已被浏览器服务端撤销）。
	var hits int
	ps := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusGone)
	}))
	defer ps.Close()

	created, err := f.Server.Services.Push.Subscribe(f.Ctx(), alice, ps.URL, generateP256DH(t), generateAuth(t), time.Now())
	testutil.RequireNoError(t, err)
	if !created {
		t.Fatalf("subscribe should create")
	}

	// 离线触发推送：应发出请求，且 410 后订阅被删除。
	f.Server.Services.Push.PushForOfflineUser(f.Ctx(), alice, "New message", "hello")
	if hits == 0 {
		t.Fatalf("push service not hit")
	}
	n, err := f.DB.CountPushSubscriptionsByUser(f.Ctx(), alice)
	testutil.RequireNoError(t, err)
	if n != 0 {
		t.Fatalf("subscriptions after 410 = %d, want 0 (stale subscription removed)", n)
	}
}

func TestPushService_PushForOfflineUser_SkipsWhenNotConfigured(t *testing.T) {
	f := testutil.New(t)
	alice := createTestUser(t, f, "push4@x.test", "PushAlice")

	// 不配置 VAPID：无论有无订阅，PushForOfflineUser 都不得报错、不得发送。
	_, err := f.Server.Services.Push.Subscribe(f.Ctx(), alice,
		"https://push.example/endpoint-4", generateP256DH(t), generateAuth(t), time.Now())
	if !errors.Is(err, service.ErrPushNotConfigured) {
		t.Fatalf("err = %v, want ErrPushNotConfigured", err)
	}

	// 不应 panic、不应有任何副作用（无 key 时内部直接 return）。
	f.Server.Services.Push.PushForOfflineUser(f.Ctx(), alice, "New message", "hello")

	n, err := f.DB.CountPushSubscriptionsByUser(f.Ctx(), alice)
	testutil.RequireNoError(t, err)
	if n != 0 {
		t.Fatalf("subscriptions = %d, want 0 (subscribe was rejected)", n)
	}
}