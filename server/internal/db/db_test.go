package db_test

import (
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/testutil"

	_ "modernc.org/sqlite"
)

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