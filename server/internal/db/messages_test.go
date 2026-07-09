package db_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestCreateGetMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "msg1@x.com", "MsgUser", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "General", "", a.ID, []string{a.ID})

	msg, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "Hello World!", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Hello World!" || msg.ChatID != chat.ID || msg.UserID != a.ID {
		t.Fatal("message metadata wrong")
	}
	if msg.Author == nil || msg.Author.Username != "MsgUser" {
		t.Fatal("author not populated")
	}
	if msg.DeletedAt != nil {
		t.Fatal("new message marked deleted")
	}
	if msg.EditedAt != nil {
		t.Fatal("new message has edited timestamp")
	}
}

func TestGetMessagesWithPagination(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "page@x.com", "Pager", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "Pagination", "", a.ID, []string{a.ID})

	ids := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		msg, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "msg-", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, msg.ID)
	}

	msgs, err := f.DB.GetMessages(f.Ctx(), chat.ID, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("first page: want 3 got %d", len(msgs))
	}
	msgs2, err := f.DB.GetMessages(f.Ctx(), chat.ID, msgs[0].ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs2) != 2 {
		t.Fatalf("second page: want 2 got %d", len(msgs2))
	}
}

func TestUpdateMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "edit@x.com", "Editor", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "other@x.com", "Other", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "EditTest", "", a.ID, []string{a.ID, b.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "original", nil, nil)

	updated, err := f.DB.UpdateMessage(f.Ctx(), msg.ID, a.ID, "edited")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != "edited" {
		t.Fatal("content not updated")
	}
	if updated.EditedAt == nil {
		t.Fatal("edited_at not set")
	}
	_, err = f.DB.UpdateMessage(f.Ctx(), msg.ID, b.ID, "hack")
	if err != db.ErrNotFound {
		t.Fatal("other user should not be able to edit")
	}
}

func TestDeleteMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "delmsg@x.com", "DelMsg", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "delother@x.com", "DelOther", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "DelTest", "", a.ID, []string{a.ID, b.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "delete me", nil, nil)

	if err := f.DB.DeleteMessage(f.Ctx(), msg.ID, a.ID, false); err != nil {
		t.Fatal(err)
	}
	m, _ := f.DB.GetMessage(f.Ctx(), msg.ID)
	if m.DeletedAt == nil {
		t.Fatal("should be deleted")
	}
	if m.Content != "" {
		t.Fatal("deleted messages should have empty content")
	}

	msg2, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, b.ID, "delete me 2", nil, nil)
	err := f.DB.DeleteMessage(f.Ctx(), msg2.ID, a.ID, false)
	if err != db.ErrNotFound {
		t.Fatalf("non-owner delete without allowAny should fail, got %v", err)
	}
	err = f.DB.DeleteMessage(f.Ctx(), msg2.ID, a.ID, true)
	if err != nil {
		t.Fatalf("admin delete with allowAny should work: %v", err)
	}
}

func TestReactionsAddRemove(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "rxn1@x.com", "RxnUser", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "rxn2@x.com", "RxnOther", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "RxnTest", "", a.ID, []string{a.ID, b.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "reaction test", nil, nil)

	if err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.AddReaction(f.Ctx(), msg.ID, b.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "❤️"); err != nil {
		t.Fatal(err)
	}
	m, err := f.DB.GetMessage(f.Ctx(), msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	var rxs []models.Reaction
	if err := json.Unmarshal(m.Reactions, &rxs); err != nil {
		t.Fatal(err)
	}
	if len(rxs) != 2 {
		t.Fatalf("want 2 reactions, got %d", len(rxs))
	}
	reactionMap := map[string]int{}
	for _, r := range rxs {
		reactionMap[r.Emoji] = r.Count
	}
	if reactionMap["👍"] != 2 {
		t.Fatalf("thumbs up count: want 2 got %d", reactionMap["👍"])
	}
	if reactionMap["❤️"] != 1 {
		t.Fatalf("heart count: want 1 got %d", reactionMap["❤️"])
	}
	if err := f.DB.RemoveReaction(f.Ctx(), msg.ID, a.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	m, _ = f.DB.GetMessage(f.Ctx(), msg.ID)
	var rxs2 []models.Reaction
	if err := json.Unmarshal(m.Reactions, &rxs2); err != nil {
		t.Fatal(err)
	}
	reactionMap = map[string]int{}
	for _, r := range rxs2 {
		reactionMap[r.Emoji] = r.Count
	}
	if reactionMap["👍"] != 1 {
		t.Fatalf("after remove: want 1 got %d", reactionMap["👍"])
	}
}

func TestMessageWithMentions(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "ment1@x.com", "MentA", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "ment2@x.com", "MentB", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "MentionTest", "", a.ID, []string{a.ID, b.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "Hey <@there>", []string{b.ID}, nil)
	var ments []string
	if err := json.Unmarshal(msg.Mentions, &ments); err != nil {
		t.Fatal(err)
	}
	if len(ments) != 1 || ments[0] != b.ID {
		t.Fatalf("mentions: %v", ments)
	}
}

func TestMessageWithAttachments(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "att@x.com", "AttUser", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "AttTest", "", a.ID, []string{a.ID})
	atts := []models.Attachment{
		{Filename: "foo.png", MimeType: "image/png", Size: 1024, URL: "/uploads/foo.png"},
	}
	msg, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "with attachment", nil, atts)
	if err != nil {
		t.Fatal(err)
	}
	var attsOut []models.Attachment
	if err := json.Unmarshal(msg.Attachments, &attsOut); err != nil {
		t.Fatal(err)
	}
	if len(attsOut) != 1 {
		t.Fatalf("want 1 attachment, got %d", len(attsOut))
	}
	if attsOut[0].Filename != "foo.png" {
		t.Fatal("attachment filename wrong")
	}
}

func TestUnreadCount(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "unread@x.com", "UnreadUser", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "UnreadTest", "", a.ID, []string{a.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "msg1", nil, nil)
	n, _ := f.DB.UnreadCount(f.Ctx(), chat.ID, "")
	if n == 0 {
		t.Fatal("should have unread")
	}
	f.DB.UpdateLastRead(f.Ctx(), chat.ID, a.ID, msg.ID)
	n, _ = f.DB.UnreadCount(f.Ctx(), chat.ID, msg.ID)
	if n != 0 {
		t.Fatalf("after reading, unread should be 0, got %d", n)
	}
	time.Sleep(10 * time.Millisecond)
	f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "msg2", nil, nil)
	n, _ = f.DB.UnreadCount(f.Ctx(), chat.ID, msg.ID)
	if n == 0 {
		t.Fatal("new message should register unread")
	}
}

func TestLastMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lastmsg@x.com", "LastMsg", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "LastMsgTest", "", a.ID, []string{a.ID})

	_, err := f.DB.LastMessage(f.Ctx(), chat.ID)
	if err != db.ErrNotFound {
		t.Fatal("no messages yet: want ErrNotFound")
	}

	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "first", nil, nil)
	f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "last", nil, nil)
	last, err := f.DB.LastMessage(f.Ctx(), chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last.Content != "last" {
		t.Fatalf("want 'last' got '%s'", last.Content)
	}
	if last.ID == msg.ID {
		t.Fatal("should be different message")
	}
}

func TestUpdateUserProfile(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "prof@x.com", "OldName", "pw00000000")
	u, err := f.DB.UpdateUserProfile(f.Ctx(), a.ID, "NewName", "#FF0000", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "NewName" || u.AvatarColor != "#FF0000" {
		t.Fatal("profile not updated")
	}
}

func TestUserStatus(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "status@x.com", "StatusUser", "pw00000000")
	if err := f.DB.UpdateUserStatus(f.Ctx(), a.ID, "online"); err != nil {
		t.Fatal(err)
	}
	u, _ := f.DB.GetUserByID(f.Ctx(), a.ID)
	if u.Status != "online" {
		t.Fatalf("want online, got %s", u.Status)
	}
	if err := f.DB.UpdateUserStatus(f.Ctx(), a.ID, "offline"); err != nil {
		t.Fatal(err)
	}
	u, _ = f.DB.GetUserByID(f.Ctx(), a.ID)
	if u.Status != "offline" {
		t.Fatalf("want offline, got %s", u.Status)
	}
}

func TestUpdateUserLastSeen(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "seen@x.com", "SeenUser", "pw00000000")
	if err := f.DB.UpdateUserLastSeen(f.Ctx(), a.ID); err != nil {
		t.Fatal(err)
	}
	// Create a message (which also calls UpdateUserLastSeen), then verify timestamp exists
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "SeenTest", "", a.ID, []string{a.ID})
	if err := f.DB.UpdateUserLastSeen(f.Ctx(), a.ID); err != nil {
		t.Fatal(err)
	}
	f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "triggers last_seen", nil, nil)
	u, _ := f.DB.GetUserByID(f.Ctx(), a.ID)
	if u.LastSeen.IsZero() {
		t.Fatal("last_seen should be set after message creation")
	}
}

func TestRefreshTokenCRUD(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "tok@x.com", "Tok", "pw00000000")
	rt, err := f.DB.CreateRefreshToken(f.Ctx(), a.ID, "abc-hash", 1)
	if err != nil {
		t.Fatal(err)
	}
	found, err := f.DB.FindRefreshToken(f.Ctx(), "abc-hash")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != rt.ID {
		t.Fatal("token mismatch")
	}
	if err := f.DB.DeleteRefreshToken(f.Ctx(), rt.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.FindRefreshToken(f.Ctx(), "abc-hash")
	if err != db.ErrNotFound {
		t.Fatal("should be gone")
	}
	f.DB.CreateRefreshToken(f.Ctx(), a.ID, "expired-hash", -1)
	_, err = f.DB.FindRefreshToken(f.Ctx(), "expired-hash")
	if err != nil {
		t.Fatal("expired token not found (TTL enforcement is at handler level)")
	}
	n, _ := f.DB.PurgeExpiredTokens(f.Ctx())
	if n != 1 {
		t.Fatalf("purge should clean 1 expired token, got %d", n)
	}
}

func TestEmptyMessageRejected(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "emptymsg@x.com", "Empty", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "EmptyTest", "", a.ID, []string{a.ID})
	_, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "", nil, nil)
	if err == nil {
		t.Fatal("empty message should fail")
	}
}

