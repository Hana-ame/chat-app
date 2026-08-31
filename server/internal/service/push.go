package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/SherClockHolmes/webpush-go"

	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

// PushService 提供 Web Push（VAPID）的订阅管理、离线投递与失效清理，
// Web Push 机制落到 SQLite 栈：
//   - 订阅：浏览器 PushManager 签发 endpoint（+p256dh/auth 加密密钥），
//     前端登录后经 /api/push/subscribe 存入；endpoint 全局唯一，重复注册
//     直接覆盖（数据层 UNIQUE 兜底）。
//   - 离线投递：用户不在线（hub 无 WS 连接）且发生通知时，对每个订阅
//     发送加密 push；在线用户走实时广播，不再重复推送。
//   - 失效清理：推送服务返回 404/410（订阅过期/被撤销）时删除该订阅；
//     其余错误只记日志（尽力而为，不拖垮消息链路）。
//   - 开关：VAPID 三件套未配置（env 缺 key）时 IsConfigured() 为 false，
//     订阅端点返回 503、发送直接跳过——Web Push 整体不可用但不报错。
type PushService struct {
	*Service
}

// ErrPushNotConfigured 表示 VAPID 未配置时对订阅/推送操作的拒绝原因。
var ErrPushNotConfigured = errors.New("web push not configured")

// pushPayloadTTLSeconds 是推送消息在推送服务侧的保留时长（1 小时）：
// 离线用户在 1 小时内回来看见即可，避免通知在第三方服务堆积。
const pushPayloadTTLSeconds = 3600

// IsConfigured 返回 VAPID 三件套是否齐全（公钥/私钥/subject 都非空）。
// 未启用时不接触任何第三方推送服务（默认 opt-in）。
func (s *PushService) IsConfigured() bool {
	c := s.Cfg
	return c.PushVAPIDPublicKey != "" && c.PushVAPIDPrivateKey != "" && c.PushVAPIDSubject != ""
}

// VAPIDPublicKey 返回配置的公钥（前端拉取后交给 PushManager 注册）。
// 未配置时返回 ErrPushNotConfigured。
func (s *PushService) VAPIDPublicKey() (string, error) {
	if !s.IsConfigured() {
		return "", ErrPushNotConfigured
	}
	return s.Cfg.PushVAPIDPublicKey, nil
}

// Subscribe 保存一条浏览器订阅（重复 endpoint 覆盖归属）。返回 created
// 表示新插入。未配置 VAPID 时拒绝。
func (s *PushService) Subscribe(ctx context.Context, userID, endpoint, p256dh, auth string, now time.Time) (created bool, err error) {
	if !s.IsConfigured() {
		return false, ErrPushNotConfigured
	}
	return s.db.SavePushSubscription(ctx, userID, endpoint, p256dh, auth, now)
}

// Unsubscribe 删除一条订阅；只允许订阅 owner 删除自己的（user id+endpoint
// 双条件，防越权）。不存在时静默成功（幂等退订）。
func (s *PushService) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	return s.db.DeletePushSubscriptionByUserAndEndpoint(ctx, userID, endpoint)
}

// PushForOfflineUser 向离线用户的所有订阅发送一条 Web Push（payload 为
// 通知的 {title, body}）。在线时不调用（实时广播已覆盖）。单个订阅失败
// 不中断：404/410 即删该订阅，其余只记日志。
func (s *PushService) PushForOfflineUser(ctx context.Context, userID string, title, body string) {
	if !s.IsConfigured() {
		return
	}
	subs, err := s.db.ListPushSubscriptionsByUser(ctx, userID)
	if err != nil {
		logutil.Warn("push: list subscriptions for %s: %v", logutil.SafeID(userID), err)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		logutil.Warn("push: marshal payload: %v", err)
		return
	}
	opts := &webpush.Options{
		Subscriber:      s.Cfg.PushVAPIDSubject,
		VAPIDPublicKey:  s.Cfg.PushVAPIDPublicKey,
		VAPIDPrivateKey: s.Cfg.PushVAPIDPrivateKey,
		TTL:             pushPayloadTTLSeconds,
	}
	for _, sub := range subs {
		s.sendOne(ctx, userID, sub, payload, opts)
	}
}

// sendOne 向单条订阅发送并处理结果（410/404 → 删订阅）。
func (s *PushService) sendOne(ctx context.Context, userID string, sub models.PushSubscription, payload []byte, opts *webpush.Options) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	resp, err := webpush.SendNotificationWithContext(reqCtx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256DH,
			Auth:   sub.Auth,
		},
	}, opts)
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			logutil.Warn("push: send to %s endpoint=%s: %v", logutil.SafeID(userID), logutil.SafeID(sub.Endpoint), err)
		}
		return
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200, 201, 202:
		// 成功：推送服务已接收。
	case 404, 410:
		// 订阅已失效（端点被浏览器服务端撤销/过期）→ 删除，避免反复打空。
		if err := s.db.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); err != nil {
			logutil.Warn("push: delete stale subscription %s: %v", logutil.SafeID(sub.Endpoint), err)
		} else {
			logutil.Info("push: subscription %s gone (410/404), deleted", logutil.SafeID(sub.Endpoint))
		}
	default:
		logutil.Warn("push: unexpected status %d for %s", resp.StatusCode, logutil.SafeID(sub.Endpoint))
	}
}