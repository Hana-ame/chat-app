package service_test

import (
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

// 【本地改动 2026-08-31】线程移植回归测试。发现背景：chat-app 原本没有线程，
// 用户的回复只能以「引用另一条消息」的扁平方式存在，没有线程聚合。实现
// 的线程模型（thread_root_message_id/reply_to + ThreadWatchs + thread
// notifications）时新增本文件，覆盖：
//   - StartThread=true 时发送的消息自引用 thread_root_message_id 成为线程根；
//   - 对根消息的回复自动继承 thread_root_message_id，reply_to 保留为父消息；
//   - 顶层消息（既非根也非回复）thread_root_message_id 为空；
//   - 嵌套回复继承祖先的 thread_root，不指向父消息；
//   - ListMessages?in_thread=X 只返回该线程内的消息（含根本身）；
//   - Follow / Unfollow / ListThreadSummarys 端到端工作；
//   - MarkThreadRead 推进已读游标到最新回复；
//   - 线程内新回复触发 reply_in_thread 通知给关注者（除作者本人）。
func threadsFixture(t *testing.T) *testutil.Fixture {
	t.Helper()
	return testutil.New(t)
}

func makeChatAndUsers(f *testutil.Fixture, t *testing.T) (string, string, string) {
	t.Helper()
	a := createTestUser(t, f, "a@x.test", "Alice")
	b := createTestUser(t, f, "b@x.test", "Bob")
	chat := createTestChat(t, f, "thread-chat", a, []string{a, b})
	return chat.ID, a, b
}

func TestThreads_StartThreadSelfRoot(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	msg, err := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root topic", nil, "", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ThreadRootMessageID != msg.ID {
		t.Fatalf("self-root expected msg.ID=%s, got %s", msg.ID, msg.ThreadRootMessageID)
	}
}

func TestThreads_TopLevelMessage(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	msg, err := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "plain msg", nil, "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ThreadRootMessageID != "" {
		t.Fatalf("top-level expected empty thread_root, got %s", msg.ThreadRootMessageID)
	}
}

func TestThreads_ReplyInheritsThreadRoot(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root", nil, "", "", true)
	reply, err := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "reply to root", nil, root.ID, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ThreadRootMessageID != root.ID {
		t.Fatalf("reply expected thread_root=%s, got %s", root.ID, reply.ThreadRootMessageID)
	}
	if reply.ReplyTo != root.ID {
		t.Fatalf("reply expected reply_to=%s, got %s", root.ID, reply.ReplyTo)
	}
}

func TestThreads_NestedReplyInheritsThreadRoot(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root", nil, "", "", true)
	first, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "first reply", nil, root.ID, "", false)

	second, err := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "second reply", nil, first.ID, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if second.ThreadRootMessageID != root.ID {
		t.Fatalf("nested reply expected thread_root=%s, got %s", root.ID, second.ThreadRootMessageID)
	}
	if second.ReplyTo != first.ID {
		t.Fatalf("nested reply expected reply_to=%s, got %s", first.ID, second.ReplyTo)
	}
}

func TestThreads_ExplicitThreadRootHonored(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root", nil, "", "", true)
	// 显式指定 thread_root，即使没给 reply_to 也应使用该根。
	reply, err := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "explicit reply", nil, "", root.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if reply.ThreadRootMessageID != root.ID {
		t.Fatalf("explicit thread_root expected %s, got %s", root.ID, reply.ThreadRootMessageID)
	}
}

func TestThreads_ListMessagesInThreadFilter(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "top level 1", nil, "", "", false)
	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "thread root", nil, "", "", true)
	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "reply 1", nil, root.ID, "", false)
	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "top level 2", nil, "", "", false)
	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "reply 2", nil, root.ID, "", false)

	msgs, err := f.Server.Services.Message.List(f.Ctx(), chatID, a, "", 100, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages in thread, got %d", len(msgs))
	}
	for _, m := range msgs {
		if m.ThreadRootMessageID != root.ID {
			t.Fatalf("message %s has unexpected thread_root %s", m.ID, m.ThreadRootMessageID)
		}
	}

	all, err := f.Server.Services.Message.List(f.Ctx(), chatID, a, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 messages total, got %d", len(all))
	}
}

func TestThreads_FollowUnfollow(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "follow topic", nil, "", "", true)

	if err := f.DB.FollowThread(f.Ctx(), a, root.ID); err != nil {
		t.Fatal(err)
	}
	following, err := f.DB.IsFollowingThread(f.Ctx(), a, root.ID)
	if err != nil || !following {
		t.Fatalf("expected following=true, got %v err=%v", following, err)
	}
	// 幂等关注。
	if err := f.DB.FollowThread(f.Ctx(), a, root.ID); err != nil {
		t.Fatal(err)
	}

	if err := f.DB.UnfollowThread(f.Ctx(), a, root.ID); err != nil {
		t.Fatal(err)
	}
	following, _ = f.DB.IsFollowingThread(f.Ctx(), a, root.ID)
	if following {
		t.Fatal("expected following=false after unfollow")
	}
}

func TestThreads_ListThreadSummarys(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root1, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root1", nil, "", "", true)
	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "r1-reply", nil, root1.ID, "", false)
	root2, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "root2", nil, "", "", true)

	if err := f.DB.FollowThread(f.Ctx(), a, root1.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.FollowThread(f.Ctx(), a, root2.ID); err != nil {
		t.Fatal(err)
	}

	list, err := f.DB.ListThreadSummarys(f.Ctx(), a, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 followed threads, got %d", len(list))
	}
}

func TestThreads_MarkThreadRead(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, _ := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "read root", nil, "", "", true)
	latest, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "unread reply", nil, root.ID, "", false)

	if err := f.DB.FollowThread(f.Ctx(), a, root.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.SetThreadRead(f.Ctx(), a, root.ID, latest.ID); err != nil {
		t.Fatal(err)
	}

	cursor, err := f.DB.GetThreadReadCursor(f.Ctx(), a, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != latest.ID {
		t.Fatalf("expected cursor=%s, got %s", latest.ID, cursor)
	}
}

func TestThreads_ReplyTriggersNotificationForFollower(t *testing.T) {
	f := threadsFixture(t)
	chatID, a, b := makeChatAndUsers(f, t)

	root, _ := f.Server.Services.Message.Send(f.Ctx(), chatID, a, "notif root", nil, "", "", true)
	if err := f.DB.FollowThread(f.Ctx(), b, root.ID); err != nil {
		t.Fatal(err)
	}

	// alice 在 bob 关注的线程里发回复。
	_, _ = f.Server.Services.Message.Send(f.Ctx(), chatID, a, "notif reply", nil, root.ID, "", false)

	occB, err := f.Server.Services.Notification.List(f.Ctx(), b, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	foundForB := false
	for _, o := range occB {
		if o.Kind == "reply_in_thread" && o.ChatID == chatID && o.ActorID == a {
			foundForB = true
			break
		}
	}
	if !foundForB {
		t.Fatalf("bob should have received a reply_in_thread notification")
	}

	occA, err := f.Server.Services.Notification.List(f.Ctx(), a, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range occA {
		if o.Kind == "reply_in_thread" {
			t.Fatalf("alice should not receive reply_in_thread for her own reply, got %v", o)
		}
	}
}
