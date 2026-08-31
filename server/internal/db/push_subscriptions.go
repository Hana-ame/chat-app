package db

import (
	"context"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/models"
)

// PushSubscription 相关 SQLite 访问层（移植 chatto 的 Web Push 订阅语义）：
//   - 每行 = 一个用户在某浏览器/设备上的一条订阅；endpoint 由浏览器
//     PushManager 签发且全局唯一。
//   - p256dh/auth 是浏览器订阅的加密密钥（RFC 8291），发送时用于加密
//     payload；VAPID 私钥在后端持有并签名请求，推送服务验签确认发送者。
//   - 发送返回 404/410（订阅失效）或用户注销时由 service 层调用删除原语
//     清行；删用户通过 FK ON DELETE CASCADE 自动清空其全部订阅。

// newPushSubscription 构造一条待持久化的订阅。id 由调用方通过 NewID 分配。
func newPushSubscription(userID, endpoint, p256dh, auth string, now time.Time) *models.PushSubscription {
	return &models.PushSubscription{
		ID:        NewID(),
		UserID:    userID,
		Endpoint:  endpoint,
		P256DH:    p256dh,
		Auth:      auth,
		CreatedAt: now.UTC(),
	}
}

// SavePushSubscription 幂等保存一条订阅：endpoint 已存在则覆盖为该用户的数据
// （依赖迁移 006 的 UNIQUE(endpoint)，同一端点重复注册直接更新，无需应用层
// 锁）；否则插入。返回 created 表示这次是新插入。
//
// 实现注意（踩坑 2026-08-31）：SQLite 的 INSERT ... ON CONFLICT DO UPDATE
// 在「更新已有行」时 sqlite3_changes 也计 1，RowsAffected 无法区分新插入与
// 覆盖；因此改用两步：先 DO NOTHING（新插入计数 1、已存在计数 0）区分
// created，再对已存在的行单独 UPDATE 覆盖。并发下两条路径的 UPDATE 均幂等，
// 不产生重复行。
func (d *DB) SavePushSubscription(ctx context.Context, userID, endpoint, p256dh, auth string, now time.Time) (created bool, err error) {
	sub := newPushSubscription(userID, endpoint, p256dh, auth, now)
	res, err := d.ExecContext(ctx,
		`INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(endpoint) DO NOTHING`,
		sub.ID, sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth,
		sub.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 1 {
		return true, nil // 新插入
	}
	// 已存在（endpoint 唯一键）：覆盖归属与密钥，保留原 created_at。
	_, err = d.ExecContext(ctx,
		`UPDATE push_subscriptions SET user_id = ?, p256dh = ?, auth = ?
		 WHERE endpoint = ?`,
		userID, p256dh, auth, endpoint)
	return false, err
}

// ListPushSubscriptionsByUser 返回某用户的全部订阅（按创建时间正序，保证
// 推送按注册顺序送达各设备）。
func (d *DB) ListPushSubscriptionsByUser(ctx context.Context, userID string) ([]models.PushSubscription, error) {
	const q = `SELECT id, user_id, endpoint, p256dh, auth, created_at
	           FROM push_subscriptions
	           WHERE user_id = ?
	           ORDER BY created_at`
	rows, err := d.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.PushSubscription, 0)
	for rows.Next() {
		var s models.PushSubscription
		var createdAt string
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256DH, &s.Auth, &createdAt); err != nil {
			return nil, err
		}
		s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, s)
	}
	return out, rows.Err()
}

// DeletePushSubscriptionByEndpoint 按 endpoint 删除一条订阅；返回 ErrNotFound
// 当不存在。endpoint 由 service 层校验归属后传入。
func (d *DB) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePushSubscriptionByUserAndEndpoint 按 (user_id, endpoint) 删除，只允许
// 订阅的 owner 删除自己的订阅（防越权）；不存在时不报错（幂等删除）。
func (d *DB) DeletePushSubscriptionByUserAndEndpoint(ctx context.Context, userID, endpoint string) error {
	_, err := d.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`,
		userID, endpoint)
	return err
}

// DeleteAllPushSubscriptionsByUser 删除某用户的全部订阅（用户注销设备时
// 调用；FK CASCADE 本可兜底，但注销流程需要显式清行）。
func (d *DB) DeleteAllPushSubscriptionsByUser(ctx context.Context, userID string) error {
	_, err := d.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE user_id = ?`, userID)
	return err
}

// CountPushSubscriptionsByUser 返回某用户的订阅数（测试断言与配额上限用）。
func (d *DB) CountPushSubscriptionsByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM push_subscriptions WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}