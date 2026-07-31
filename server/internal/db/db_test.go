package db_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/testutil"

	_ "modernc.org/sqlite"
)

func TestNewID(t *testing.T) {
	id1 := db.NewID()
	id2 := db.NewID()
	if id1 == "" || id2 == "" {
		t.Fatal("NewID returned empty")
	}
	if id1 == id2 {
		t.Fatal("NewID not unique")
	}
}

func TestPickColor(t *testing.T) {
	if c := db.PickColor(""); c != "#5865F2" {
		t.Fatalf("empty seed: want #5865F2 got %s", c)
	}
	if c := db.PickColor("not-a-uuid"); c != "#5865F2" {
		t.Fatalf("non-uuid seed: want #5865F2 got %s", c)
	}
	// Same UUID always picks same color
	u := db.NewID()
	c1 := db.PickColor(u)
	c2 := db.PickColor(u)
	if c1 != c2 {
		t.Fatalf("PickColor should be deterministic: %s != %s", c1, c2)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.GetUserByID(f.Ctx(), "nonexistent")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUpdateUserProfile_Conflict(t *testing.T) {
	f := testutil.New(t)
	f.DB.CreateUser(f.Ctx(), "c1@x.com", "ConflictA", "pw")
	f.DB.CreateUser(f.Ctx(), "c2@x.com", "ConflictB", "pw")
	ua, _, _ := f.DB.GetUserByEmail(f.Ctx(), "c1@x.com")
	_, err := f.DB.UpdateUserProfile(f.Ctx(), ua.ID, "ConflictB", "", "")
	if err != db.ErrConflict {
		t.Fatalf("username conflict: want ErrConflict got %v", err)
	}
}

func TestUpdateUserStatus_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.UpdateUserStatus(f.Ctx(), "nobody", "online")
	if err != nil {
		t.Fatal("nonexistent user should not error")
	}
}

func TestSearchUsers_EmptyQuery(t *testing.T) {
	f := testutil.New(t)
	users, err := f.DB.SearchUsers(f.Ctx(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Fatalf("empty query: want 0 got %d", len(users))
	}
	users, _ = f.DB.SearchUsers(f.Ctx(), "  ", 10)
	if len(users) != 0 {
		t.Fatalf("whitespace query: want 0 got %d", len(users))
	}
}

func TestCreateChat_InvalidType(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "invtype@x.com", "InvType", "pw")
	_, err := f.DB.CreateChat(f.Ctx(), "invalid", "", "", a.ID, []string{a.ID})
	if err == nil {
		t.Fatal("invalid type should fail")
	}
}

func TestCreateChat_GroupEmptyName(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "emptyname@x.com", "EmptyName", "pw")
	_, err := f.DB.CreateChat(f.Ctx(), "group", "", "", a.ID, []string{a.ID})
	if err == nil {
		t.Fatal("group with empty name should fail")
	}
}

func TestCreateChat_DMWrongMemberCount(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "dmcnt@x.com", "DmCnt", "pw")
	_, err := f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID})
	if err == nil {
		t.Fatal("dm with 1 member should fail")
	}
}

func TestCreateChat_EmptyMembers(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "nomem@x.com", "NoMem", "pw")
	_, err := f.DB.CreateChat(f.Ctx(), "group", "NoMembers", "", a.ID, []string{})
	if err == nil {
		t.Fatal("empty members should fail")
	}
}

func TestGetChat_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.GetChat(f.Ctx(), "nonexistent")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetChatMembers_Empty(t *testing.T) {
	f := testutil.New(t)
	members, err := f.DB.GetChatMembers(f.Ctx(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatal("nonexistent chat should return empty list")
	}
}

func TestGetChatMemberRole_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.GetChatMemberRole(f.Ctx(), "nochat", "nouser")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestIsChatMember_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	ok, err := f.DB.IsChatMember(f.Ctx(), "nochat", "nouser")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("should be false for nonexistent chat")
	}
}

func TestAddRemoveMember_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "armem@x.com", "ArMem", "pw")
	err := f.DB.RemoveChatMember(f.Ctx(), "nonexistent", a.ID)
	if err != nil {
		t.Fatal("removing from nonexistent chat should not error")
	}
	// Add to a real chat then try removing non-member
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "RealChat", "", a.ID, []string{a.ID})
	err = f.DB.RemoveChatMember(f.Ctx(), chat.ID, "nonexistent-user")
	if err != nil {
		t.Fatal("removing nonexistent user should not error")
	}
}

func TestDeleteChat_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.DeleteChat(f.Ctx(), "nonexistent")
	if err != nil {
		t.Fatal("deleting nonexistent chat should not error")
	}
}

func TestRenameChat_EmptyName(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "rnempty@x.com", "RnEmpty", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "RnMe", "", a.ID, []string{a.ID})
	err := f.DB.RenameChat(f.Ctx(), chat.ID, "")
	if err == nil {
		t.Fatal("empty name should fail")
	}
	err = f.DB.RenameChat(f.Ctx(), chat.ID, "  ")
	if err == nil {
		t.Fatal("whitespace name should fail")
	}
}

func TestRenameChat_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.RenameChat(f.Ctx(), "nonexistent", "NewName")
	if err != nil {
		t.Fatal("renaming nonexistent chat should not error")
	}
}

func TestUpdateLastRead_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.UpdateLastRead(f.Ctx(), "nochat", "nouser", "nomsg")
	if err != nil {
		t.Fatal("updating last read on nonexistent data should not error")
	}
}

func TestPurgeExpiredTokens_NoneExpired(t *testing.T) {
	f := testutil.New(t)
	n, err := f.DB.PurgeExpiredTokens(f.Ctx())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("no expired tokens: want 0 got %d", n)
	}
}

func TestFindDMBetween_Self(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "selfdm@x.com", "SelfDM", "pw")
	_, err := f.DB.FindDMBetween(f.Ctx(), a.ID, a.ID)
	if err == nil {
		t.Fatal("self DM should fail")
	}
}

func TestFindDMBetween_NotFound(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "dmnotfound@x.com", "DmNotFound", "pw")
	_, err := f.DB.FindDMBetween(f.Ctx(), a.ID, "nonexistent-user")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCreateRefreshToken_Duplicate(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "duprt@x.com", "DupRT", "pw")
	_, err := f.DB.CreateRefreshToken(f.Ctx(), a.ID, "same-hash", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.CreateRefreshToken(f.Ctx(), a.ID, "same-hash", time.Hour)
	if err != db.ErrConflict {
		t.Fatalf("duplicate hash: want ErrConflict got %v", err)
	}
}

func TestJoinChatByID_AlreadyJoined(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "join2@x.com", "Join2", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "Public2", "public", a.ID, []string{a.ID})
	err := f.DB.JoinChatByID(f.Ctx(), chat.ID, a.ID)
	if err != nil {
		t.Fatal("joining own chat should succeed (INSERT OR IGNORE)")
	}
}

func TestJoinChatByID_Private(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "joinpriv@x.com", "JoinPriv", "pw")
	b, _ := f.DB.CreateUser(f.Ctx(), "joinpriv2@x.com", "JoinPriv2", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "Private", "private", a.ID, []string{a.ID})
	err := f.DB.JoinChatByID(f.Ctx(), chat.ID, b.ID)
	if err == nil {
		t.Fatal("joining private chat without invite should fail")
	}
}

func TestJoinChatByID_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "joinnone@x.com", "JoinNone", "pw")
	err := f.DB.JoinChatByID(f.Ctx(), "nonexistent", a.ID)
	if err == nil {
		t.Fatal("joining nonexistent chat should fail")
	}
}

func TestSetAndClearPinnedMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pinunit@x.com", "PinUnit", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PinUnitTest", "", a.ID, []string{a.ID})
	if err := f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "pinned!"); err != nil {
		t.Fatal(err)
	}
	c, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	if c.PinnedMessage == nil || c.PinnedMessage.Content != "pinned!" {
		t.Fatal("pinned message not set")
	}
	if c.PinnedUpdatedAt == nil || c.PinnedUpdatedAt.IsZero() {
		t.Fatal("pinned_updated_at should be set")
	}
	if err := f.DB.ClearPinnedMessage(f.Ctx(), chat.ID); err != nil {
		t.Fatal(err)
	}
	c, _ = f.DB.GetChat(f.Ctx(), chat.ID)
	if c.PinnedMessage != nil {
		t.Fatal("pinned message not cleared")
	}
	if c.PinnedUpdatedAt != nil {
		t.Fatal("pinned_updated_at should be nil after clear")
	}
}

func TestSetPinnedMessage_MultipleUpdates(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pinmulti@x.com", "PinMulti", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PinMulti", "", a.ID, []string{a.ID})

	f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "v1")
	c, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	v1t := *c.PinnedUpdatedAt

	f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "v2")
	c, _ = f.DB.GetChat(f.Ctx(), chat.ID)
	if c.PinnedMessage.Content != "v2" {
		t.Fatal("content not updated")
	}
	if !c.PinnedUpdatedAt.After(v1t) {
		t.Fatal("pinned_updated_at should advance")
	}
}

func TestUpdatePinnedLastReadAt(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pread@x.com", "PRead", "pw")
	b, _ := f.DB.CreateUser(f.Ctx(), "pread2@x.com", "PRead2", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PReadTest", "", a.ID, []string{a.ID, b.ID})

	f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "notice")
	if err := f.DB.UpdatePinnedLastReadAt(f.Ctx(), chat.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.UpdatePinnedLastReadAt(f.Ctx(), chat.ID, b.ID); err != nil {
		t.Fatal(err)
	}

	chats, err := f.DB.ListUserChats(f.Ctx(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	var found *models.Chat
	for _, c := range chats {
		if c.ID == chat.ID {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatal("chat not found in list")
	}
	if found.PinnedLastReadAt == nil || found.PinnedLastReadAt.IsZero() {
		t.Fatal("pinned_last_read_at should be set for user a")
	}

	chats, _ = f.DB.ListUserChats(f.Ctx(), b.ID)
	found = nil
	for _, c := range chats {
		if c.ID == chat.ID {
			found = &c
			break
		}
	}
	if found == nil {
		t.Fatal("chat not found for user b")
	}
	if found.PinnedLastReadAt == nil || found.PinnedLastReadAt.IsZero() {
		t.Fatal("pinned_last_read_at should be set for user b")
	}
}

func TestUpdatePinnedLastReadAt_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.UpdatePinnedLastReadAt(f.Ctx(), "nochat", "nouser")
	if err != nil {
		t.Fatal("updating on nonexistent data should not error")
	}
}

func TestPinnedLastReadAt_NotSet(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pnil@x.com", "PNil", "pw")
	b, _ := f.DB.CreateUser(f.Ctx(), "pnil2@x.com", "PNil2", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PNil", "", a.ID, []string{a.ID, b.ID})

	chats, _ := f.DB.ListUserChats(f.Ctx(), a.ID)
	var found *models.Chat
	for _, c := range chats {
		if c.ID == chat.ID {
			found = &c
			break
		}
	}
	if found.PinnedLastReadAt != nil {
		t.Fatal("pinned_last_read_at should be nil before any read")
	}
}

func TestPinnedUpdatedAt_NotSet(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "punset@x.com", "PUnset", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PUnset", "", a.ID, []string{a.ID})

	c, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	if c.PinnedMessage != nil {
		t.Fatal("no pinned message expected")
	}
	if c.PinnedUpdatedAt != nil {
		t.Fatal("pinned_updated_at should be nil when no pin set")
	}
}

func TestSetPinnedMessage_ThreeMembers(t *testing.T) {
	// PinChat handler requires ≥3 members, test that DB layer allows it with 1
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pin3@x.com", "Pin3", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "Pin3", "", a.ID, []string{a.ID})
	if err := f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "works"); err != nil {
		t.Fatal("DB layer should allow pin with any member count")
	}
}

func TestListPublicChats_Empty(t *testing.T) {
	f := testutil.New(t)
	chats, err := f.DB.ListPublicChats(f.Ctx(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 0 {
		t.Fatal("no public chats should exist")
	}
}

func TestUnreadCount_FutureTimestamp(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "unread_future@x.com", "UnreadFuture", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "UnreadFuture", "", a.ID, []string{a.ID})
	f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "test", nil, nil)
	n := f.DB.UnreadCount(f.Ctx(), chat.ID, time.Now().UTC().Add(time.Hour))
	if n != 0 {
		t.Fatalf("future timestamp: want 0 got %d", n)
	}
}

func TestCreateMessage_DuplicateMentions(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "dupment@x.com", "DupMent", "pw")
	b, _ := f.DB.CreateUser(f.Ctx(), "dupment2@x.com", "DupMent2", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "DupMent", "", a.ID, []string{a.ID, b.ID})
	msg, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "mentions", []string{b.ID, b.ID, b.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var ments []string
	if err := json.Unmarshal(msg.Mentions, &ments); err != nil {
		t.Fatal(err)
	}
	if len(ments) != 1 {
		t.Fatalf("dedupe should produce 1 mention, got %d", len(ments))
	}
}

func TestCreateMessage_ContentTooLong(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "longmsg@x.com", "LongMsg", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "LongMsg", "", a.ID, []string{a.ID})
	buf := make([]byte, 4001)
	for i := range buf {
		buf[i] = 'a'
	}
	_, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, string(buf), nil, nil)
	if err == nil {
		t.Fatal("content over 4000 should fail")
	}
}

func TestGetMessage_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.GetMessage(f.Ctx(), "nonexistent")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetMessages_EmptyChat(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "emptymsg@x.com", "EmptyMsg", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "EmptyMsg", "", a.ID, []string{a.ID})
	msgs, err := f.DB.GetMessages(f.Ctx(), chat.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatal("empty chat should return no messages")
	}
}

func TestUpdateMessage_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.UpdateMessage(f.Ctx(), "nonexistent", "nobody", "new content")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDeleteMessage_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	err := f.DB.DeleteMessage(f.Ctx(), "nonexistent", "nobody", false)
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestAddReaction_EmptyEmoji(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "emptyrxn@x.com", "EmptyRxn", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "EmptyRxn", "", a.ID, []string{a.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "test", nil, nil)
	err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "")
	if err == nil {
		t.Fatal("empty emoji should fail")
	}
	err = f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "  ")
	if err == nil {
		t.Fatal("whitespace emoji should fail")
	}
}

func TestAddReaction_EmojiTooLong(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "longrxn@x.com", "LongRxn", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "LongRxn", "", a.ID, []string{a.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "test", nil, nil)
	err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Fatal("emoji over 32 chars should fail")
	}
}

func TestAddReaction_Duplicate(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "duprxn@x.com", "DupRxn", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "DupRxn", "", a.ID, []string{a.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "test", nil, nil)
	if err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "👍"); err != nil {
		t.Fatal(err)
	}
	if err := f.DB.AddReaction(f.Ctx(), msg.ID, a.ID, "👍"); err != nil {
		t.Fatal("duplicate reaction should succeed (INSERT OR IGNORE)")
	}
}

func TestRemoveReaction_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "norxn@x.com", "NoRxn", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "NoRxn", "", a.ID, []string{a.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "test", nil, nil)
	err := f.DB.RemoveReaction(f.Ctx(), msg.ID, a.ID, "👍")
	if err != nil {
		t.Fatal("removing nonexistent reaction should succeed (no-op)")
	}
}

func TestCreateMessage_AttachmentOnly(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "attonly_ut@x.com", "AttOnlyUt", "pw")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "AttOnlyUt", "", a.ID, []string{a.ID})
	atts := []models.Attachment{
		{Filename: "file.pdf", MimeType: "application/pdf", Size: 100, URL: "https://example.com/file.pdf"},
	}
	msg, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "", nil, atts)
	if err != nil {
		t.Fatalf("attachment-only should work: %v", err)
	}
	var attsOut []models.Attachment
	if err := json.Unmarshal(msg.Attachments, &attsOut); err != nil {
		t.Fatal(err)
	}
	if len(attsOut) != 1 {
		t.Fatal("attachment missing")
	}
}

func TestUserLastSeen_ZeroOnCreate(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "seenzero@x.com", "SeenZero", "pw")
	if !a.LastSeen.IsZero() {
		t.Fatal("last_seen should be zero for new user")
	}
}

func TestDBOpenAndMigrate(t *testing.T) {
	f := testutil.New(t)
	if f.DB == nil {
		t.Fatal("DB is nil")
	}
	ctx := f.Ctx()
	u, err := f.DB.CreateUser(ctx, "test1@x.com", "test-user", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.Username != "test-user" || u.AvatarColor == "" {
		t.Fatal("user creation incomplete")
	}
}

func TestUserCreateDuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.CreateUser(f.Ctx(), "same@x.com", "u1", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.CreateUser(f.Ctx(), "same@x.com", "u2", "hash12345678")
	if err != db.ErrConflict {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.DB.CreateUser(f.Ctx(), "getme@x.com", "getme", "hash12345678")
	if err != nil {
		t.Fatal(err)
	}
	u, hash, err := f.DB.GetUserByEmail(f.Ctx(), "getme@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "getme" || hash != "hash12345678" {
		t.Fatal("wrong user")
	}
	_, _, err = f.DB.GetUserByEmail(f.Ctx(), "nope@x.com")
	if err != db.ErrNotFound {
		t.Fatal("want not found")
	}
}

func TestSearchUsers(t *testing.T) {
	f := testutil.New(t)
	f.DB.CreateUser(f.Ctx(), "alpha@x.com", "Alpha", "pw12345678")
	f.DB.CreateUser(f.Ctx(), "beta@x.com", "Beta", "pw12345678")
	f.DB.CreateUser(f.Ctx(), "gamma@x.com", "gamma", "pw12345678")
	users, err := f.DB.SearchUsers(f.Ctx(), "alp", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Username != "Alpha" {
		t.Fatalf("want 1 result (Alpha), got %d", len(users))
	}
	users, err = f.DB.SearchUsers(f.Ctx(), "a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) < 2 {
		t.Fatalf("want at least 2 results for fuzzy 'a', got %d", len(users))
	}
}

func TestCreateChatGroupAndDM(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "a@g.com", "Alice", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "b@g.com", "Bob", "pw00000000")
	c, _ := f.DB.CreateUser(f.Ctx(), "c@g.com", "Carol", "pw00000000")

	chat, err := f.DB.CreateChat(f.Ctx(), "group", "TestGroup", "", a.ID, []string{a.ID, b.ID, c.ID})
	if err != nil {
		t.Fatal(err)
	}
	if chat.Name != "TestGroup" || chat.Type != "group" || chat.OwnerID != a.ID {
		t.Fatal("chat metadata wrong")
	}
	if chat.MemberCount != 3 {
		t.Fatalf("want 3 members, got %d", chat.MemberCount)
	}
	members, err := f.DB.GetChatMembers(f.Ctx(), chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.ID != a.ID && m.ID != b.ID && m.ID != c.ID {
			t.Fatalf("unexpected member: %s", m.ID)
		}
	}

	dm, err := f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if dm.Type != "dm" || dm.Name != "" {
		t.Fatal("DM metadata wrong")
	}
	if dm.OwnerID != "" {
		t.Fatal("DM should have no owner")
	}
}

func TestFindDMBetween(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "dma@x.com", "DMA", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "dmb@x.com", "DMB", "pw00000000")
	f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})

	dm, err := f.DB.FindDMBetween(f.Ctx(), a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if dm == nil || dm.Type != "dm" {
		t.Fatal("DM not found")
	}

	_, err = f.DB.FindDMBetween(f.Ctx(), a.ID, "nonexistent")
	if err != db.ErrNotFound {
		t.Fatal("want not found")
	}

	_, err = f.DB.FindDMBetween(f.Ctx(), a.ID, a.ID)
	if err == nil {
		t.Fatal("should error on self-DM")
	}
}

func TestListUserChats(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "list1@x.com", "List1", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "list2@x.com", "List2", "pw00000000")
	f.DB.CreateChat(f.Ctx(), "group", "Chat1", "", a.ID, []string{a.ID, b.ID})
	f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{a.ID, b.ID})

	chats, err := f.DB.ListUserChats(f.Ctx(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Fatalf("want 2 chats, got %d", len(chats))
	}
	foundGroup := false
	foundDM := false
	for _, c := range chats {
		if c.Type == "group" {
			foundGroup = true
		}
		if c.Type == "dm" {
			foundDM = true
		}
	}
	if !foundGroup || !foundDM {
		t.Fatal("missing chat type in list")
	}
}

func TestAddRemoveMember(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "mem1@x.com", "Mem1", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "mem2@x.com", "Mem2", "pw00000000")
	c, _ := f.DB.CreateUser(f.Ctx(), "mem3@x.com", "Mem3", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "MemTest", "", a.ID, []string{a.ID, b.ID})

	if err := f.DB.AddChatMember(f.Ctx(), chat.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, c.ID)
	if !ok {
		t.Fatal("should be member")
	}
	if err := f.DB.AddChatMember(f.Ctx(), chat.ID, c.ID); err != db.ErrConflict {
		t.Fatal("double add should conflict")
	}
	if err := f.DB.RemoveChatMember(f.Ctx(), chat.ID, c.ID); err != nil {
		t.Fatal(err)
	}
	ok, _ = f.DB.IsChatMember(f.Ctx(), chat.ID, c.ID)
	if ok {
		t.Fatal("should be removed")
	}
}

func TestDeleteChat(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "del1@x.com", "Del1", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "DeleteMe", "", a.ID, []string{a.ID})
	if err := f.DB.DeleteChat(f.Ctx(), chat.ID); err != nil {
		t.Fatal(err)
	}
	_, err := f.DB.GetChat(f.Ctx(), chat.ID)
	if err != db.ErrNotFound {
		t.Fatal("should be gone")
	}
}

func TestRenameChat(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "r1@x.com", "Renamer", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "OldName", "", a.ID, []string{a.ID})
	if err := f.DB.RenameChat(f.Ctx(), chat.ID, "NewName"); err != nil {
		t.Fatal(err)
	}
	updated, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	if updated.Name != "NewName" {
		t.Fatal("rename didn't stick")
	}
}

func TestFindAndDeleteRefreshToken(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "fadrt@x.com", "FadRT", "pw00000000")

	// Create token
	rt, err := f.DB.CreateRefreshToken(f.Ctx(), a.ID, "hash-fad", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if rt.ID == "" {
		t.Fatal("empty refresh token ID")
	}

	// Find and delete — atomically consumed
	found, err := f.DB.FindAndDeleteRefreshToken(f.Ctx(), "hash-fad")
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != rt.ID {
		t.Fatal("wrong token returned")
	}

	// Second call → ErrNotFound (prevents replay)
	_, err = f.DB.FindAndDeleteRefreshToken(f.Ctx(), "hash-fad")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// Nonexistent hash → ErrNotFound
	_, err = f.DB.FindAndDeleteRefreshToken(f.Ctx(), "no-such-hash")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDeleteUserRefreshTokens(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "durt@x.com", "DurT", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "durt2@x.com", "DurT2", "pw00000000")

	if _, err := f.DB.CreateRefreshToken(f.Ctx(), a.ID, "a1", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.CreateRefreshToken(f.Ctx(), a.ID, "a2", time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := f.DB.CreateRefreshToken(f.Ctx(), b.ID, "b1", time.Hour); err != nil {
		t.Fatal(err)
	}

	// Delete all for user a
	if err := f.DB.DeleteUserRefreshTokens(f.Ctx(), a.ID); err != nil {
		t.Fatal(err)
	}

	// Verify a's tokens are gone (FindAndDeleteRefreshToken returns ErrNotFound on replay)
	_, err := f.DB.FindAndDeleteRefreshToken(f.Ctx(), "a1")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	_, err = f.DB.FindAndDeleteRefreshToken(f.Ctx(), "a2")
	if err != db.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	// Verify b's token unaffected
	if _, err = f.DB.FindAndDeleteRefreshToken(f.Ctx(), "b1"); err != nil {
		t.Fatal(err)
	}

	// Calling on user with no tokens should not error
	if err := f.DB.DeleteUserRefreshTokens(f.Ctx(), "no-such-user"); err != nil {
		t.Fatal(err)
	}
}

func TestListReactions(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lrx1@x.com", "LRx1", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "lrx2@x.com", "LRx2", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "LRx", "", a.ID, []string{a.ID, b.ID})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "rx", nil, nil)

	f.DB.AddReaction(context.Background(), msg.ID, a.ID, "👍")
	f.DB.AddReaction(context.Background(), msg.ID, b.ID, "👍")
	f.DB.AddReaction(context.Background(), msg.ID, a.ID, "❤️")

	// Viewer = a (who reacted 👍 and ❤️)
	rxs, err := f.DB.ListReactions(context.Background(), msg.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rxs) != 2 {
		t.Fatalf("want 2 reaction groups, got %d", len(rxs))
	}
	for _, rx := range rxs {
		if rx.Count == 0 {
			t.Fatal("reaction count should be > 0")
		}
		if !rx.Me {
			t.Fatal("viewer a should have Me=true for all reactions they left")
		}
	}

	// Viewer = some third party (not reacted)
	c, _ := f.DB.CreateUser(f.Ctx(), "lrx3@x.com", "LRx3", "pw00000000")
	rxs2, _ := f.DB.ListReactions(context.Background(), msg.ID, c.ID)
	for _, rx := range rxs2 {
		if rx.Me {
			t.Fatal("third party should have Me=false")
		}
	}

	// No reactions
	msg2, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, "norx", nil, nil)
	rxs3, _ := f.DB.ListReactions(context.Background(), msg2.ID, a.ID)
	if len(rxs3) != 0 {
		t.Fatalf("want 0 reactions, got %d", len(rxs3))
	}
}

func TestSetPinned(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "pin@x.com", "Pin", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "pin2@x.com", "Pin2", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PinTest", "", a.ID, []string{a.ID, b.ID})

	if err := f.DB.SetPinned(context.Background(), chat.ID, a.ID, true); err != nil {
		t.Fatal(err)
	}
	chats, _ := f.DB.ListUserChats(context.Background(), a.ID)
	var foundA bool
	for _, c := range chats {
		if c.ID == chat.ID {
			foundA = true
			if !c.Pinned {
				t.Fatal("a should have chat pinned")
			}
		}
	}
	if !foundA {
		t.Fatal("chat not in a's list")
	}
	chats, _ = f.DB.ListUserChats(context.Background(), b.ID)
	for _, c := range chats {
		if c.ID == chat.ID {
			if c.Pinned {
				t.Fatal("b should NOT have chat pinned")
			}
		}
	}

	if err := f.DB.SetPinned(context.Background(), chat.ID, a.ID, false); err != nil {
		t.Fatal(err)
	}
	chats, _ = f.DB.ListUserChats(context.Background(), a.ID)
	for _, c := range chats {
		if c.ID == chat.ID {
			if c.Pinned {
				t.Fatal("a should no longer have chat pinned")
			}
		}
	}
}


func TestGetChatMemberRole_Success(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "gcmr@x.com", "GCMR", "pw00000000")
	b, _ := f.DB.CreateUser(f.Ctx(), "gcmr2@x.com", "GCMR2", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "GCMRTest", "", a.ID, []string{a.ID, b.ID})

	role, err := f.DB.GetChatMemberRole(f.Ctx(), chat.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "owner" {
		t.Fatalf("owner role should be 'owner', got %q", role)
	}

	role, err = f.DB.GetChatMemberRole(f.Ctx(), chat.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "" {
		t.Fatalf("regular member role should be empty, got %q", role)
	}
}

func TestListPublicChats_Pagination(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lpc@x.com", "LPC", "pw00000000")
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("PubChat_%d", i)
		if _, err := f.DB.CreateChat(f.Ctx(), "group", name, "public", a.ID, []string{a.ID}); err != nil {
			t.Fatal(err)
		}
	}

	// page = 0 (clamped to 1)
	chats, err := f.DB.ListPublicChats(f.Ctx(), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Fatalf("want 2 chats, got %d", len(chats))
	}

	// page = 2
	chats, err = f.DB.ListPublicChats(f.Ctx(), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) < 1 {
		t.Fatal("expected at least 1 chat on page 2")
	}

	// limit = 0 (clamped to 20)
	chats, err = f.DB.ListPublicChats(f.Ctx(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListPublicChats_WithMessages(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lpcm@x.com", "LPCM", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PubWithMsg", "public", a.ID, []string{a.ID})

	longContent := strings.Repeat("x", 150)
	if _, err := f.DB.CreateMessage(f.Ctx(), chat.ID, a.ID, longContent, nil, nil); err != nil {
		t.Fatal(err)
	}

	chats, err := f.DB.ListPublicChats(f.Ctx(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("want 1 chat, got %d", len(chats))
	}
	if chats[0].LastMessageAt.IsZero() {
		t.Fatal("LastMessageAt should be set")
	}
}

func TestListPublicChats_WithPinnedMessage(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lpcp@x.com", "LPCP", "pw00000000")
	chat, _ := f.DB.CreateChat(f.Ctx(), "group", "PubPinned", "public", a.ID, []string{a.ID})
	if err := f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "important notice"); err != nil {
		t.Fatal(err)
	}

	chats, err := f.DB.ListPublicChats(f.Ctx(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 {
		t.Fatalf("want 1 chat, got %d", len(chats))
	}
	if chats[0].PinnedMessage == nil || chats[0].PinnedMessage.Content != "important notice" {
		t.Fatal("pinned message content mismatch")
	}
}

func TestListPublicChats_PrivateExcluded(t *testing.T) {
	f := testutil.New(t)
	a, _ := f.DB.CreateUser(f.Ctx(), "lpcpriv@x.com", "LPCPriv", "pw00000000")
	if _, err := f.DB.CreateChat(f.Ctx(), "group", "PrivateChat", "private", a.ID, []string{a.ID}); err != nil {
		t.Fatal(err)
	}

	chats, err := f.DB.ListPublicChats(f.Ctx(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 0 {
		t.Fatalf("private chats should be excluded, got %d", len(chats))
	}
}
