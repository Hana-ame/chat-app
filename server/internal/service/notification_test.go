package service_test

import (
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

// 【本地改动 2026-08-31】发现背景：chat-app 原本只有客户端 browserNotify
// （页面打开才弹，无服务端持久化）。
// occurrence 存储与「提及 + 回复」触发，本文件是其回归测试：
//   - 提及触发只发给被 @ 的人（不发给发送者）；
//   - (user, kind, chat, 源消息) 唯一——重复触发不重复插行、不重置已读；
//   - 回复触发发给被回复消息的作者；
//   - 未读计数 / 单条已读 / 全部已读 / 删除 / 过期清理生命周期。
func TestNotificationService_MentionCreatesOccurrenceForRecipientOnly(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a@x.test", "Alice")
	b := createTestUser(t, f, "b@x.test", "Bob")
	chat := createTestChat(t, f, "Team", a, []string{a, b})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "<@"+a+"> hello from Bob", nil, "", "", false)
	testutil.RequireNoError(t, err)

	occA, err := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	testutil.RequireNoError(t, err)
	if len(occA) != 1 {
		t.Fatalf("Alice occurrences = %d, want 1", len(occA))
	}
	if occA[0].Kind != "mention" || occA[0].ChatID != chat.ID || occA[0].ActorID != b {
		t.Fatalf("occurrence = %+v, want mention from Bob in chat", occA[0])
	}
	if occA[0].Read {
		t.Fatalf("fresh occurrence must be unread")
	}

	// 发送者自身不产生通知。
	occB, err := f.Server.Services.Notification.List(f.Ctx(), b, "", 50)
	testutil.RequireNoError(t, err)
	if len(occB) != 0 {
		t.Fatalf("sender Bob occurrences = %d, want 0", len(occB))
	}

	n, err := f.Server.Services.Notification.UnreadCount(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	if n != 1 {
		t.Fatalf("Alice unread = %d, want 1", n)
	}
}

func TestNotificationService_MentionIsUniquePerSource(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a2@x.test", "Alice2")
	b := createTestUser(t, f, "b2@x.test", "Bob2")
	chat := createTestChat(t, f, "Team2", a, []string{a, b})

	// 用同一源消息直接触发两次（模拟真实链路里同一条消息被重复投递），
	// 唯一键 (user, kind, chat, message_id) 应保证只落一行。
	msg := &models.Message{ID: "m-same-source", ChatID: chat.ID, UserID: b, Content: "ping"}
	if err := f.Server.Services.Notification.CreateForMessage(f.Ctx(), chat.ID, b, []string{a}, nil, msg); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if err := f.Server.Services.Notification.CreateForMessage(f.Ctx(), chat.ID, b, []string{a}, nil, msg); err != nil {
		t.Fatalf("trigger twice: %v", err)
	}

	occA, err := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	testutil.RequireNoError(t, err)
	if len(occA) != 1 {
		t.Fatalf("Alice occurrences = %d, want 1 (same source message once)", len(occA))
	}

	// 另一条源消息 → 第二行；唯一性按源消息粒度，而不是按内容。
	msg2 := &models.Message{ID: "m-other-source", ChatID: chat.ID, UserID: b, Content: "ping"}
	if err := f.Server.Services.Notification.CreateForMessage(f.Ctx(), chat.ID, b, []string{a}, nil, msg2); err != nil {
		t.Fatalf("trigger other source: %v", err)
	}
	occA, err = f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	testutil.RequireNoError(t, err)
	if len(occA) != 2 {
		t.Fatalf("Alice occurrences = %d, want 2 (distinct source messages)", len(occA))
	}
	n, err := f.Server.Services.Notification.UnreadCount(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	if n != 2 {
		t.Fatalf("Alice unread = %d, want 2", n)
	}
}

func TestNotificationService_ReplyNotifiesRepliedAuthor(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a3@x.test", "Alice3")
	b := createTestUser(t, f, "b3@x.test", "Bob3")
	chat := createTestChat(t, f, "Team3", a, []string{a, b})

	original, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "question?", nil, "", "", false)
	testutil.RequireNoError(t, err)

	_, err = f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "answer!", nil, original.ID, "", false)
	testutil.RequireNoError(t, err)

	occA, err := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	testutil.RequireNoError(t, err)
	if len(occA) != 1 {
		t.Fatalf("Alice occurrences = %d, want 1 (reply)", len(occA))
	}
	if occA[0].Kind != "reply" || occA[0].ActorID != b {
		t.Fatalf("occurrence = %+v, want reply from Bob", occA[0])
	}
}

func TestNotificationService_ReadAllAndDeleteLifecycle(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a4@x.test", "Alice4")
	b := createTestUser(t, f, "b4@x.test", "Bob4")
	chat := createTestChat(t, f, "Team4", a, []string{a, b})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "<@"+a+"> hi", nil, "", "", false)
	testutil.RequireNoError(t, err)

	occ, err := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	testutil.RequireNoError(t, err)
	if len(occ) != 1 {
		t.Fatalf("occurrences = %d, want 1", len(occ))
	}

	if err := f.Server.Services.Notification.MarkRead(f.Ctx(), occ[0].ID, a); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	n, _ := f.Server.Services.Notification.UnreadCount(f.Ctx(), a)
	if n != 0 {
		t.Fatalf("unread after markRead = %d, want 0", n)
	}

	if err := f.Server.Services.Notification.Delete(f.Ctx(), occ[0].ID, a); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	after, _ := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	if len(after) != 0 {
		t.Fatalf("occurrences after delete = %d, want 0", len(after))
	}
}

func TestNotificationService_MarkAllRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a5@x.test", "Alice5")
	b := createTestUser(t, f, "b5@x.test", "Bob5")
	chat := createTestChat(t, f, "Team5", a, []string{a, b})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "<@"+a+"> one", nil, "", "", false)
	testutil.RequireNoError(t, err)
	_, err = f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "<@"+a+"> two", nil, "", "", false)
	testutil.RequireNoError(t, err)

	if err := f.Server.Services.Notification.MarkAllRead(f.Ctx(), a); err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	n, _ := f.Server.Services.Notification.UnreadCount(f.Ctx(), a)
	if n != 0 {
		t.Fatalf("unread after markAll = %d, want 0", n)
	}
	occ, _ := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	if len(occ) != 2 {
		t.Fatalf("occurrences = %d, want 2 kept", len(occ))
	}
	for _, o := range occ {
		if !o.Read {
			t.Fatalf("occurrence %s still unread after mark-all", o.ID)
		}
	}
}

func TestNotificationService_PruneExpired(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "a6@x.test", "Alice6")

	// 直接落一行已过期的 occurrence（绕过 90 天 TTL 的常规写入路径）。
	_, err := f.DB.ExecContext(f.Ctx(),
		`INSERT INTO notification_occurrences
		   (id, user_id, kind, chat_id, message_id, actor_id, title, body, read, created_at, expires_at)
		 VALUES (?, ?, 'system', 'c-expired', '', '', 't', 'b', 0, ?, ?)`,
		"ntf-expired", a, time.Now().UTC().Add(-2*time.Hour).Format(time.RFC3339Nano),
		time.Now().UTC().Add(-1*time.Hour).Format(time.RFC3339Nano),
	)
	testutil.RequireNoError(t, err)

	n, err := f.Server.Services.Notification.PruneExpired(f.Ctx())
	testutil.RequireNoError(t, err)
	if n != 1 {
		t.Fatalf("pruned = %d, want 1", n)
	}
	occ, _ := f.Server.Services.Notification.List(f.Ctx(), a, "", 50)
	if len(occ) != 0 {
		t.Fatalf("occurrences after prune = %d, want 0", len(occ))
	}
}
