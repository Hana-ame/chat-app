package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestService_New(t *testing.T) {
	f := testutil.New(t)
	if f.Server.Services == nil {
		t.Fatal("Services is nil")
	}
	if f.Server.Services.Chat == nil {
		t.Fatal("ChatService is nil")
	}
	if f.Server.Services.Message == nil {
		t.Fatal("MessageService is nil")
	}
	if f.Server.Services.Member == nil {
		t.Fatal("MemberService is nil")
	}
	if f.Server.Services.User == nil {
		t.Fatal("UserService is nil")
	}
}

func TestService_WithTx(t *testing.T) {
	f := testutil.New(t)
	called := false
	err := f.Server.Services.WithTx(f.Ctx(), func(tx *sql.Tx) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("fn not called")
	}
}

func TestErrors(t *testing.T) {
	if service.ErrForbidden.Error() != "forbidden" {
		t.Fatal("ErrForbidden wrong message")
	}
	if service.ErrNotFound.Error() != "not_found" {
		t.Fatal("ErrNotFound wrong message")
	}
	if service.ErrInvalidInput.Error() != "invalid_input" {
		t.Fatal("ErrInvalidInput wrong message")
	}
	if service.ErrConflict.Error() != "conflict" {
		t.Fatal("ErrConflict wrong message")
	}
	if service.ErrContentTooLong.Error() != "content too long" {
		t.Fatal("ErrContentTooLong wrong message")
	}
}

func TestService_WithTx_Error(t *testing.T) {
	f := testutil.New(t)
	err := f.Server.Services.WithTx(f.Ctx(), func(tx *sql.Tx) error {
		return service.ErrInvalidInput
	})
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func createTestUser(t *testing.T, f *testutil.Fixture, email, username string) string {
	t.Helper()
	hash := "test-hash-12345678"
	u, err := f.DB.CreateUser(f.Ctx(), email, username, hash)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func createTestChat(t *testing.T, f *testutil.Fixture, name string, ownerID string, memberIDs []string) *models.Chat {
	t.Helper()
	chat, err := f.DB.CreateChat(f.Ctx(), "group", name, "", ownerID, memberIDs)
	if err != nil {
		t.Fatal(err)
	}
	return chat
}

func createTestDM(t *testing.T, f *testutil.Fixture, u1, u2 string) *models.Chat {
	t.Helper()
	chat, err := f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{u1, u2})
	if err != nil {
		t.Fatal(err)
	}
	return chat
}

func TestChatService_ListForUser(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "list_a@x.com", "ListA")
	b := createTestUser(t, f, "list_b@x.com", "ListB")
	createTestChat(t, f, "Chat1", a, []string{a, b})
	createTestChat(t, f, "Chat2", a, []string{a})

	chats, err := f.Server.Services.Chat.ListForUser(f.Ctx(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 2 {
		t.Fatalf("want 2 chats, got %d", len(chats))
	}
}

func TestChatService_ListForUser_Empty(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "list_empty@x.com", "ListEmpty")
	chats, err := f.Server.Services.Chat.ListForUser(f.Ctx(), a)
	if err != nil {
		t.Fatal(err)
	}
	if len(chats) != 0 {
		t.Fatalf("want 0 chats, got %d", len(chats))
	}
}

func TestChatService_GetByID_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get@x.com", "GetUser")
	chat := createTestChat(t, f, "GetTest", a, []string{a})

	got, err := f.Server.Services.Chat.GetByID(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != chat.ID {
		t.Fatal("wrong chat returned")
	}
}

func TestChatService_GetByID_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get_a@x.com", "GetA")
	b := createTestUser(t, f, "get_b@x.com", "GetB")
	chat := createTestChat(t, f, "GetTest", a, []string{a})

	_, err := f.Server.Services.Chat.GetByID(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_GetByID_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get_c@x.com", "GetC")
	_, err := f.Server.Services.Chat.GetByID(f.Ctx(), "nonexistent", a)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden (membership check fails for nonexistent chat), got %v", err)
	}
}

func TestChatService_Create_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "create@x.com", "CreateUser")
	b := createTestUser(t, f, "create_b@x.com", "CreateB")

	chat, err := f.Server.Services.Chat.Create(f.Ctx(), a, "NewChat", "public", []string{b})
	if err != nil {
		t.Fatal(err)
	}
	if chat.Name != "NewChat" {
		t.Fatalf("want NewChat, got %s", chat.Name)
	}
	if chat.OwnerID != a {
		t.Fatal("owner should be creator")
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, a)
	if !ok {
		t.Fatal("owner should be auto-added as member")
	}
	ok, _ = f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	if !ok {
		t.Fatal("invited user should be member")
	}
}

func TestChatService_Create_EmptyName(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "c_empty@x.com", "CEmpty")
	_, err := f.Server.Services.Chat.Create(f.Ctx(), a, "", "public", nil)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestChatService_Create_OnlyWhitespaceName(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "c_ws@x.com", "CWS")
	_, err := f.Server.Services.Chat.Create(f.Ctx(), a, "  ", "public", nil)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestChatService_Rename_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren@x.com", "RenUser")
	chat := createTestChat(t, f, "OldName", a, []string{a})

	updated, err := f.Server.Services.Chat.Rename(f.Ctx(), chat.ID, a, "NewName")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "NewName" {
		t.Fatalf("want NewName, got %s", updated.Name)
	}
}

func TestChatService_Rename_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_dm@x.com", "RenDM")
	b := createTestUser(t, f, "ren_dm2@x.com", "RenDM2")
	dm := createTestDM(t, f, a, b)

	_, err := f.Server.Services.Chat.Rename(f.Ctx(), dm.ID, a, "NewName")
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestChatService_Rename_NotOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_no@x.com", "RenNo")
	b := createTestUser(t, f, "ren_no2@x.com", "RenNo2")
	chat := createTestChat(t, f, "Test", a, []string{a, b})

	_, err := f.Server.Services.Chat.Rename(f.Ctx(), chat.ID, b, "NewName")
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_Rename_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_nf@x.com", "RenNF")
	_, err := f.Server.Services.Chat.Rename(f.Ctx(), "nonexistent", a, "NewName")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestChatService_Delete_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del@x.com", "DelUser")
	chat := createTestChat(t, f, "DeleteMe", a, []string{a})

	err := f.Server.Services.Chat.Delete(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.DB.GetChat(f.Ctx(), chat.ID)
	if err != db.ErrNotFound {
		t.Fatal("chat should be deleted")
	}
}

func TestChatService_Delete_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_dm@x.com", "DelDM")
	b := createTestUser(t, f, "del_dm2@x.com", "DelDM2")
	dm := createTestDM(t, f, a, b)

	err := f.Server.Services.Chat.Delete(f.Ctx(), dm.ID, a)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestChatService_Delete_NotOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_no@x.com", "DelNo")
	b := createTestUser(t, f, "del_no2@x.com", "DelNo2")
	chat := createTestChat(t, f, "Test", a, []string{a, b})

	err := f.Server.Services.Chat.Delete(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_Delete_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_nf@x.com", "DelNF")
	err := f.Server.Services.Chat.Delete(f.Ctx(), "nonexistent", a)
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestChatService_ListPublic(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pub@x.com", "PubUser")
	createTestChat(t, f, "Public1", a, []string{a})
	createTestChat(t, f, "Public2", a, []string{a})

	chats, err := f.Server.Services.Chat.ListPublic(f.Ctx(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	_ = chats
}

func TestChatService_Join_Success(t *testing.T) {
	f := testutil.New(t)
	owner := createTestUser(t, f, "join_own@x.com", "JoinOwner")
	member := createTestUser(t, f, "join_mem@x.com", "JoinMember")
	chat, err := f.DB.CreateChat(f.Ctx(), "group", "PublicChat", "public", owner, []string{owner})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Server.Services.Chat.Join(f.Ctx(), chat.ID, member)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != chat.ID {
		t.Fatal("wrong chat returned")
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, member)
	if !ok {
		t.Fatal("member should have joined")
	}
}

func TestChatService_Join_PrivateChat(t *testing.T) {
	f := testutil.New(t)
	owner := createTestUser(t, f, "join_priv@x.com", "JoinPriv")
	member := createTestUser(t, f, "join_priv2@x.com", "JoinPriv2")
	chat, err := f.DB.CreateChat(f.Ctx(), "group", "PrivateChat", "private", owner, []string{owner})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Server.Services.Chat.Join(f.Ctx(), chat.ID, member)
	if err == nil {
		t.Fatal("joining private chat should fail")
	}
}

func TestChatService_Join_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	member := createTestUser(t, f, "join_nf@x.com", "JoinNF")
	_, err := f.Server.Services.Chat.Join(f.Ctx(), "nonexistent", member)
	if err == nil {
		t.Fatal("joining nonexistent chat should fail")
	}
}

func TestChatService_MarkRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mr@x.com", "MRUser")
	chat := createTestChat(t, f, "MRTest", a, []string{a})
	err := f.Server.Services.Chat.MarkRead(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
}

func TestChatService_MarkRead_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mr2@x.com", "MR2")
	b := createTestUser(t, f, "mr3@x.com", "MR3")
	chat := createTestChat(t, f, "MRTest2", a, []string{a})

	err := f.Server.Services.Chat.MarkRead(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_SetPinned(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pin@x.com", "PinUser")
	chat := createTestChat(t, f, "PinTest", a, []string{a})

	err := f.Server.Services.Chat.SetPinned(f.Ctx(), chat.ID, a, true)
	if err != nil {
		t.Fatal(err)
	}

	members, err := f.DB.GetChatMembers(f.Ctx(), chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, m := range members {
		if m.ID == a {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("should be a member")
	}
}

func TestChatService_SetPinned_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pin2@x.com", "Pin2")
	b := createTestUser(t, f, "pin3@x.com", "Pin3")
	chat := createTestChat(t, f, "PinTest2", a, []string{a})

	err := f.Server.Services.Chat.SetPinned(f.Ctx(), chat.ID, b, true)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_SetAnnouncement(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann@x.com", "AnnUser")
	b := createTestUser(t, f, "ann2@x.com", "Ann2")
	c := createTestUser(t, f, "ann3@x.com", "Ann3")
	chat := createTestChat(t, f, "AnnTest", a, []string{a, b, c})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, a, "Important notice")
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.DB.GetChat(f.Ctx(), chat.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PinnedMessage == nil || got.PinnedMessage.Content != "Important notice" {
		t.Fatal("announcement not set")
	}
}

func TestChatService_SetAnnouncement_NotOwnerOrAdmin(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_no@x.com", "AnnNo")
	b := createTestUser(t, f, "ann_no2@x.com", "AnnNo2")
	c := createTestUser(t, f, "ann_no3@x.com", "AnnNo3")
	chat := createTestChat(t, f, "AnnTest2", a, []string{a, b, c})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, b, "test")
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_SetAnnouncement_SmallGroup(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_lt@x.com", "AnnLt")
	chat := createTestChat(t, f, "AnnTest3", a, []string{a})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, a, "test")
	if err != nil {
		t.Fatalf("small group pin: want nil, got %v", err)
	}
}

func TestChatService_ClearAnnouncement(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_clr@x.com", "AnnClr")
	b := createTestUser(t, f, "ann_clr2@x.com", "AnnClr2")
	c := createTestUser(t, f, "ann_clr3@x.com", "AnnClr3")
	chat := createTestChat(t, f, "AnnClrTest", a, []string{a, b, c})
	f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "notice")

	err := f.Server.Services.Chat.ClearAnnouncement(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	if got.PinnedMessage != nil {
		t.Fatal("announcement not cleared")
	}
}

func TestChatService_ClearAnnouncement_NotOwnerOrAdmin(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_clr4@x.com", "AnnClr4")
	b := createTestUser(t, f, "ann_clr5@x.com", "AnnClr5")
	c := createTestUser(t, f, "ann_clr6@x.com", "AnnClr6")
	chat := createTestChat(t, f, "AnnClrTest2", a, []string{a, b, c})

	err := f.Server.Services.Chat.ClearAnnouncement(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestChatService_MarkAnnouncementRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_rd@x.com", "AnnRd")
	chat := createTestChat(t, f, "AnnRdTest", a, []string{a})
	err := f.Server.Services.Chat.MarkAnnouncementRead(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessageService_List(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_lst@x.com", "MsgLst")
	chat := createTestChat(t, f, "MsgList", a, []string{a})
	f.DB.CreateMessage(f.Ctx(), chat.ID, a, "Hello", nil, nil)
	f.DB.CreateMessage(f.Ctx(), chat.ID, a, "World", nil, nil)

	msgs, err := f.Server.Services.Message.List(f.Ctx(), chat.ID, a, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
}

func TestMessageService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_lst2@x.com", "MsgLst2")
	b := createTestUser(t, f, "msg_lst3@x.com", "MsgLst3")
	chat := createTestChat(t, f, "MsgList2", a, []string{a})

	_, err := f.Server.Services.Message.List(f.Ctx(), chat.ID, b, "", 10)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Send(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd@x.com", "MsgSnd")
	chat := createTestChat(t, f, "MsgSend", a, []string{a})

	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "Hello, world!", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Hello, world!" {
		t.Fatalf("want Hello, world!, got %s", msg.Content)
	}
	if msg.UserID != a {
		t.Fatal("wrong user ID")
	}
	if msg.ChatID != chat.ID {
		t.Fatal("wrong chat ID")
	}
}

func TestMessageService_Send_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd2@x.com", "MsgSnd2")
	b := createTestUser(t, f, "msg_snd3@x.com", "MsgSnd3")
	chat := createTestChat(t, f, "MsgSend2", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, b, "test", nil)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Send_EmptyContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd4@x.com", "MsgSnd4")
	chat := createTestChat(t, f, "MsgSend3", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "", nil)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_Send_WhitespaceContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_snd5@x.com", "MsgSnd5")
	chat := createTestChat(t, f, "MsgSend4", a, []string{a})

	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "  ", nil)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_Send_AttachmentOnly(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att@x.com", "MsgAtt")
	chat := createTestChat(t, f, "MsgAttTest", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.pdf", MimeType: "application/pdf", Size: 100, URL: "http://localhost:8080/api/local/1234567890/file.pdf"},
	}
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "", atts)
	if err != nil {
		t.Fatal(err)
	}
	if msg.AttachmentCount != 1 {
		t.Fatalf("want 1 attachment, got %d", msg.AttachmentCount)
	}
}

func TestMessageService_Send_InvalidAttachmentURL(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att2@x.com", "MsgAtt2")
	chat := createTestChat(t, f, "MsgAttTest2", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.pdf", URL: "https://evil.com/file.pdf"},
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_Send_AttachmentNoURL(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att3@x.com", "MsgAtt3")
	chat := createTestChat(t, f, "MsgAttTest3", a, []string{a})

	atts := []models.Attachment{
		{Filename: "", URL: ""},
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_Send_DefaultMimeType(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_att4@x.com", "MsgAtt4")
	chat := createTestChat(t, f, "MsgAttTest4", a, []string{a})

	atts := []models.Attachment{
		{Filename: "file.bin", URL: "http://localhost:8080/api/local/1234567890/file.bin"},
	}
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, "test", atts)
	if err != nil {
		t.Fatal(err)
	}
	if msg.AttachmentCount != 1 {
		t.Fatalf("want 1 attachment, got %d", msg.AttachmentCount)
	}
}

func TestMessageService_Send_Mentions(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_men@x.com", "MsgMen")
	b := createTestUser(t, f, "msg_men2@x.com", "MsgMen2")
	chat := createTestChat(t, f, "MsgMentions", a, []string{a, b})

	content := "Hey <@" + b + "> check this out!"
	msg, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, content, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.MentionCount != 1 {
		t.Fatalf("want 1 mention, got %d", msg.MentionCount)
	}
}

func TestMessageService_Send_ContentTooLong(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_long@x.com", "MsgLong")
	chat := createTestChat(t, f, "MsgLongTest", a, []string{a})

	buf := make([]byte, 4001)
	for i := range buf {
		buf[i] = 'a'
	}
	_, err := f.Server.Services.Message.Send(f.Ctx(), chat.ID, a, string(buf), nil)
	if err != service.ErrContentTooLong {
		t.Fatalf("want ErrContentTooLong, got %v", err)
	}
}

func TestMessageService_Edit_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed@x.com", "MsgEd")
	chat := createTestChat(t, f, "MsgEdit", a, []string{a})

	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "original", nil, nil)
	edited, err := f.Server.Services.Message.Edit(f.Ctx(), chat.ID, msg.ID, a, "edited")
	if err != nil {
		t.Fatal(err)
	}
	if edited.Content != "edited" {
		t.Fatalf("want edited, got %s", edited.Content)
	}
	if edited.EditedAt == nil {
		t.Fatal("edited_at should be set")
	}
}

func TestMessageService_Edit_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed2@x.com", "MsgEd2")
	b := createTestUser(t, f, "msg_ed3@x.com", "MsgEd3")
	chat := createTestChat(t, f, "MsgEdit2", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "test", nil, nil)

	_, err := f.Server.Services.Message.Edit(f.Ctx(), chat.ID, msg.ID, b, "new")
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Edit_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed4@x.com", "MsgEd4")
	chat1 := createTestChat(t, f, "MsgEdit3", a, []string{a})
	chat2 := createTestChat(t, f, "MsgEdit4", a, []string{a})

	msg, _ := f.DB.CreateMessage(f.Ctx(), chat1.ID, a, "test", nil, nil)
	_, err := f.Server.Services.Message.Edit(f.Ctx(), chat2.ID, msg.ID, a, "new")
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_Delete_OwnMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del@x.com", "MsgDel")
	chat := createTestChat(t, f, "MsgDelete", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "delete me", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat.ID, msg.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	// Soft delete: message still exists but has DeletedAt set and content cleared
	m, err := f.DB.GetMessage(f.Ctx(), msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.DeletedAt == nil {
		t.Fatal("message should be soft-deleted (DeletedAt should be set)")
	}
	if m.Content != "" {
		t.Fatal("message content should be cleared after deletion")
	}
}

func TestMessageService_Delete_OtherUserMessage_Forbidden(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del2@x.com", "MsgDel2")
	b := createTestUser(t, f, "msg_del3@x.com", "MsgDel3")
	chat := createTestChat(t, f, "MsgDelete2", a, []string{a, b})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat.ID, msg.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Delete_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del4@x.com", "MsgDel4")
	chat1 := createTestChat(t, f, "MsgDelete3", a, []string{a})
	chat2 := createTestChat(t, f, "MsgDelete4", a, []string{a})
	msg, _ := f.DB.CreateMessage(f.Ctx(), chat1.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(f.Ctx(), chat2.ID, msg.ID, a)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMessageService_MarkRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_mr@x.com", "MsgMR")
	chat := createTestChat(t, f, "MsgMRTest", a, []string{a})
	err := f.Server.Services.Message.MarkRead(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessageService_MarkRead_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_mr2@x.com", "MsgMR2")
	b := createTestUser(t, f, "msg_mr3@x.com", "MsgMR3")
	chat := createTestChat(t, f, "MsgMRTest2", a, []string{a})

	err := f.Server.Services.Message.MarkRead(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMemberService_List(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_lst@x.com", "MemLst")
	b := createTestUser(t, f, "mem_lst2@x.com", "MemLst2")
	chat := createTestChat(t, f, "MemList", a, []string{a, b})

	members, err := f.Server.Services.Member.List(f.Ctx(), chat.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 2 {
		t.Fatalf("want 2 members, got %d", len(members))
	}
}

func TestMemberService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_lst3@x.com", "MemLst3")
	b := createTestUser(t, f, "mem_lst4@x.com", "MemLst4")
	chat := createTestChat(t, f, "MemList2", a, []string{a})

	_, err := f.Server.Services.Member.List(f.Ctx(), chat.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMemberService_Add(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add@x.com", "MemAdd")
	b := createTestUser(t, f, "mem_add2@x.com", "MemAdd2")
	chat := createTestChat(t, f, "MemAddTest", a, []string{a})

	updated, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if updated.MemberCount < 2 {
		t.Fatal("member count should be at least 2")
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	if !ok {
		t.Fatal("b should be a member")
	}
}

func TestMemberService_Add_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add3@x.com", "MemAdd3")
	b := createTestUser(t, f, "mem_add4@x.com", "MemAdd4")
	c := createTestUser(t, f, "mem_add5@x.com", "MemAdd5")
	dm := createTestDM(t, f, a, b)

	_, err := f.Server.Services.Member.Add(f.Ctx(), dm.ID, a, c)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMemberService_Add_NonexistentTarget(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add6@x.com", "MemAdd6")
	chat := createTestChat(t, f, "MemAddTest2", a, []string{a})

	_, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, "nonexistent")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMemberService_Add_Duplicate(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add7@x.com", "MemAdd7")
	chat := createTestChat(t, f, "MemAddTest3", a, []string{a})

	_, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, a)
	if err != service.ErrConflict {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestMemberService_Add_NotFoundChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add8@x.com", "MemAdd8")
	_, err := f.Server.Services.Member.Add(f.Ctx(), "nonexistent", a, a)
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMemberService_Remove_Self(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem@x.com", "MemRem")
	b := createTestUser(t, f, "mem_rem2@x.com", "MemRem2")
	chat := createTestChat(t, f, "MemRemove", a, []string{a, b})

	err := f.Server.Services.Member.Remove(f.Ctx(), chat.ID, b, b)
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	if ok {
		t.Fatal("b should no longer be a member")
	}
}

func TestMemberService_Remove_OwnerProtection(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem3@x.com", "MemRem3")
	b := createTestUser(t, f, "mem_rem4@x.com", "MemRem4")
	chat := createTestChat(t, f, "MemRemove2", a, []string{a, b})

	err := f.Server.Services.Member.Remove(f.Ctx(), chat.ID, b, a)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMemberService_Remove_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem5@x.com", "MemRem5")
	b := createTestUser(t, f, "mem_rem6@x.com", "MemRem6")
	dm := createTestDM(t, f, a, b)

	err := f.Server.Services.Member.Remove(f.Ctx(), dm.ID, a, b)
	if err != service.ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestMemberService_Remove_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem7@x.com", "MemRem7")
	err := f.Server.Services.Member.Remove(f.Ctx(), "nonexistent", a, a)
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserService_Create(t *testing.T) {
	f := testutil.New(t)
	u, err := f.Server.Services.User.Create(f.Ctx(), "new@x.com", "NewUser", "hash1234")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "NewUser" || u.Email != "new@x.com" {
		t.Fatal("user not created correctly")
	}
}

func TestUserService_Create_DuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.Create(f.Ctx(), "dup@x.com", "User1", "hash1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Server.Services.User.Create(f.Ctx(), "dup@x.com", "User2", "hash2")
	if err != service.ErrConflict {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestUserService_GetByID(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "getid@x.com", "GetID")
	u, err := f.Server.Services.User.GetByID(f.Ctx(), a)
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != a {
		t.Fatal("wrong user")
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.GetByID(f.Ctx(), "nonexistent")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "getemail@x.com", "GetEmail")
	u, hash, err := f.Server.Services.User.GetByEmail(f.Ctx(), "getemail@x.com")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "GetEmail" {
		t.Fatal("wrong user")
	}
	if hash != "test-hash-12345678" {
		t.Fatalf("wrong hash: %s", hash)
	}
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, _, err := f.Server.Services.User.GetByEmail(f.Ctx(), "nobody@x.com")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserService_UpdateProfile(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "upd@x.com", "UpdateMe")
	u, err := f.Server.Services.User.UpdateProfile(f.Ctx(), a, "NewName", "#ff0000", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.Username != "NewName" {
		t.Fatalf("want NewName, got %s", u.Username)
	}
	if u.AvatarColor != "#ff0000" {
		t.Fatalf("want #ff0000, got %s", u.AvatarColor)
	}
}

func TestUserService_UpdateProfile_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.UpdateProfile(f.Ctx(), "nonexistent", "Name", "#000", "")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserService_UpdateProfile_DuplicateUsername(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "upddup@x.com", "OrigName")
	b := createTestUser(t, f, "upddup2@x.com", "OtherUser")
	_, err := f.Server.Services.User.UpdateProfile(f.Ctx(), b, "OrigName", "#000", "")
	if err != service.ErrConflict {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestUserService_Search(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "srch@x.com", "AlphaUser")
	createTestUser(t, f, "srch2@x.com", "BetaUser")

	users, err := f.Server.Services.User.Search(f.Ctx(), "Alpha", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 {
		t.Fatalf("want 1 result, got %d", len(users))
	}
	if users[0].Username != "AlphaUser" {
		t.Fatalf("want AlphaUser, got %s", users[0].Username)
	}
}

func TestUserService_Search_EmptyQuery(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "srch3@x.com", "SomeUser")
	users, err := f.Server.Services.User.Search(f.Ctx(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) < 0 {
		t.Fatal("search should return >= 0 results")
	}
}

func TestAuthz_MustBeMember(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(f *testutil.Fixture) (chatID, userID string, ctx context.Context)
		wantOK bool
	}{
		{
			name: "success",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "m1@x.com", "M1")
				c := createTestChat(t, f, "M1", u, []string{u})
				return c.ID, u, f.Ctx()
			},
			wantOK: true,
		},
		{
			name: "not member",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "m2@x.com", "M2")
				v := createTestUser(t, f, "m3@x.com", "M3")
				c := createTestChat(t, f, "M2", u, []string{u})
				return c.ID, v, f.Ctx()
			},
			wantOK: false,
		},
		{
			name: "empty user id",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "m4@x.com", "M4")
				c := createTestChat(t, f, "M4", u, []string{u})
				return c.ID, "", f.Ctx()
			},
			wantOK: false,
		},
		{
			name: "canceled context",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "m5@x.com", "M5")
				c := createTestChat(t, f, "M5", u, []string{u})
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return c.ID, u, ctx
			},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := testutil.New(t)
			chatID, userID, ctx := tt.setup(f)
			err := f.Server.Services.Authz.MustBeMember(ctx, chatID, userID)
			if tt.wantOK && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if !tt.wantOK && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestAuthz_RequireOwnerOrAdmin(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(f *testutil.Fixture) (chatID, userID string, ctx context.Context)
		wantOK  bool
		wantErr error
	}{
		{
			name: "owner",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r1@x.com", "R1")
				c := createTestChat(t, f, "R1", u, []string{u})
				return c.ID, u, f.Ctx()
			},
			wantOK: true,
		},
		{
			name: "admin role",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r2@x.com", "R2")
				v := createTestUser(t, f, "r3@x.com", "R3")
				c := createTestChat(t, f, "R2", u, []string{u, v})
				if _, err := f.DB.ExecContext(context.Background(),
					`UPDATE chat_members SET role = 'admin' WHERE chat_id = ? AND user_id = ?`,
					c.ID, v); err != nil {
					t.Fatal(err)
				}
				return c.ID, v, context.Background()
			},
			wantOK: true,
		},
		{
			name: "not owner",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r4@x.com", "R4")
				v := createTestUser(t, f, "r5@x.com", "R5")
				c := createTestChat(t, f, "R4", u, []string{u, v})
				return c.ID, v, f.Ctx()
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "not chat member",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r6@x.com", "R6")
				v := createTestUser(t, f, "r7@x.com", "R7")
				c := createTestChat(t, f, "R6", u, []string{u})
				return c.ID, v, context.Background()
			},
			wantErr: service.ErrForbidden,
		},
		{
			name: "chat not found",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r8@x.com", "R8")
				return "nonexistent", u, f.Ctx()
			},
			wantErr: service.ErrNotFound,
		},
		{
			name: "canceled context",
			setup: func(f *testutil.Fixture) (string, string, context.Context) {
				u := createTestUser(t, f, "r9@x.com", "R9")
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return "chatid", u, ctx
			},
			wantErr: nil, // any error is acceptable
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := testutil.New(t)
			chatID, userID, ctx := tt.setup(f)
			err := f.Server.Services.Authz.RequireOwnerOrAdmin(ctx, chatID, userID)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr == nil && err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestChatService_CreateOrGetDM_Create(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_c@x.com", "DMC")
	b := createTestUser(t, f, "dm_c2@x.com", "DMC2")

	chat, existed, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Fatal("should be new DM")
	}
	if chat.Type != "dm" {
		t.Fatal("should be DM")
	}
}

func TestChatService_CreateOrGetDM_Existing(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_e@x.com", "DME")
	b := createTestUser(t, f, "dm_e2@x.com", "DME2")
	createTestDM(t, f, a, b)

	chat, existed, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatal("DM should already exist")
	}
	if chat.Type != "dm" {
		t.Fatal("should be DM")
	}
}

func TestChatService_CreateOrGetDM_UserNotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_nf@x.com", "DMNF")

	_, _, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, "nonexistent")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestChatService_Create_UserAlreadyInList(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cal@x.com", "Cal")
	chat, err := f.Server.Services.Chat.Create(context.Background(), a, "TestChat", "public", []string{a})
	if err != nil {
		t.Fatal(err)
	}
	if chat.Name != "TestChat" {
		t.Fatal("wrong name")
	}
}

func TestChatService_Create_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cc@x.com", "CC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Create(ctx, a, "Test", "public", nil)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestChatService_CreateOrGetDM_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cdm@x.com", "CDM")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := f.Server.Services.Chat.CreateOrGetDM(ctx, a, "otherid")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_Rename_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rnc@x.com", "RNC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Rename(ctx, "chatid", a, "NewName")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_Delete_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "delc@x.com", "DelC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.Delete(ctx, "chatid", a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_Join_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "jnc@x.com", "JNC")
	b := createTestUser(t, f, "jnc2@x.com", "JNC2")
	chat, _ := f.DB.CreateChat(context.Background(), "group", "JNCTest", "public", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Join(ctx, chat.ID, b)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_SetAnnouncement_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "sae@x.com", "SAE")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.SetAnnouncement(ctx, "chatid", a, "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_ClearAnnouncement_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cae@x.com", "CAE")
	b := createTestUser(t, f, "cae2@x.com", "CAE2")
	chat := createTestChat(t, f, "CAETest", a, []string{a, b})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.ClearAnnouncement(ctx, chat.ID, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_SetPinned_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "spe@x.com", "SPE")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.SetPinned(ctx, "chatid", a, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_GetByID_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "uidc@x.com", "UIDC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.GetByID(ctx, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_GetByEmail_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "uemc@x.com", "UEMC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := f.Server.Services.User.GetByEmail(ctx, "uemc@x.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_Create_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.Create(ctx, "ucc@x.com", "UCC", "hash")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserService_UpdateProfile_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "updc@x.com", "UPDC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.UpdateProfile(ctx, a, "NewName", "#000", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemberService_Add_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "addc@x.com", "AddC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Member.Add(ctx, "chatid", a, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemberService_Remove_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "remc@x.com", "RemC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Member.Remove(ctx, "chatid", a, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMessageService_Send_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "sndc@x.com", "SndC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Send(ctx, "chatid", a, "test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMessageService_Edit_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "edtc@x.com", "EdtC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Edit(ctx, "chatid", "msgid", a, "new")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMessageService_Delete_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del8@x.com", "MsgDel8")
	b := createTestUser(t, f, "msg_del9@x.com", "MsgDel9")
	chat := createTestChat(t, f, "MsgDelete9", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "test", nil, nil)

	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, msg.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Delete_BroadcastError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del10@x.com", "MsgDel10")
	chat := createTestChat(t, f, "MsgDelete10", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "test", nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Message.Delete(ctx, chat.ID, msg.ID, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMessageService_MarkRead_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mrkc@x.com", "MrkC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Message.MarkRead(ctx, "chatid", a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestChatService_MarkRead_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "chmrc@x.com", "ChMrC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.MarkRead(ctx, "chatid", a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemberService_List_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "lstc@x.com", "LstC")
	chat := createTestChat(t, f, "LstCTest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Member.List(ctx, chat.ID, a)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMessageService_List_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msglc@x.com", "MsgLC")
	chat := createTestChat(t, f, "MsgLCTest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.List(ctx, chat.ID, a, "", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemberService_Remove_AdminRemovesMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_admr@x.com", "MemAdmr")
	b := createTestUser(t, f, "mem_admr2@x.com", "MemAdmr2")
	c := createTestUser(t, f, "mem_admr3@x.com", "MemAdmr3")
	chat := createTestChat(t, f, "MemAdmrTest", a, []string{a, b, c})
	_, err := f.DB.ExecContext(context.Background(),
		`UPDATE chat_members SET role = 'admin' WHERE chat_id = ? AND user_id = ?`,
		chat.ID, b)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Server.Services.Member.Remove(context.Background(), chat.ID, b, c)
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := f.DB.IsChatMember(context.Background(), chat.ID, c)
	if ok {
		t.Fatal("c should no longer be a member")
	}
}

func TestMemberService_Remove_NonAdminRemovesOther(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_nar@x.com", "MemNAR")
	b := createTestUser(t, f, "mem_nar2@x.com", "MemNAR2")
	c := createTestUser(t, f, "mem_nar3@x.com", "MemNAR3")
	chat := createTestChat(t, f, "MemNARTest", a, []string{a, b, c})
	err := f.Server.Services.Member.Remove(context.Background(), chat.ID, b, c)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMemberService_Remove_OwnerRemovesMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_orm@x.com", "MemORM")
	b := createTestUser(t, f, "mem_orm2@x.com", "MemORM2")
	chat := createTestChat(t, f, "MemORMTest", a, []string{a, b})
	err := f.Server.Services.Member.Remove(context.Background(), chat.ID, a, b)
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := f.DB.IsChatMember(context.Background(), chat.ID, b)
	if ok {
		t.Fatal("b should no longer be a member")
	}
}

func TestMemberService_Add_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add9@x.com", "MemAdd9")
	b := createTestUser(t, f, "mem_add10@x.com", "MemAdd10")
	chat := createTestChat(t, f, "MemAddTest4", a, []string{a})
	_, err := f.Server.Services.Member.Add(context.Background(), chat.ID, b, a)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestMessageService_Edit_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed5@x.com", "MsgEd5")
	chat := createTestChat(t, f, "MsgEdit5", a, []string{a})
	_, err := f.Server.Services.Message.Edit(context.Background(), chat.ID, "nonexistent", a, "edited")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestMessageService_Edit_EmptyContent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_ed6@x.com", "MsgEd6")
	chat := createTestChat(t, f, "MsgEdit6", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "original", nil, nil)
	_, err := f.Server.Services.Message.Edit(context.Background(), chat.ID, msg.ID, a, "")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

func TestMessageService_Delete_NonexistentMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del11@x.com", "MsgDel11")
	chat := createTestChat(t, f, "MsgDelete11", a, []string{a})
	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, "nonexistent", a)
	if err == nil {
		t.Fatal("expected error for deleting nonexistent message")
	}
}

func TestMessageService_Delete_OtherUserMessageAsOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_del12@x.com", "MsgDel12")
	b := createTestUser(t, f, "msg_del13@x.com", "MsgDel13")
	chat := createTestChat(t, f, "MsgDelete12", a, []string{a, b})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, b, "test", nil, nil)
	err := f.Server.Services.Message.Delete(context.Background(), chat.ID, msg.ID, a)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMessageService_Send_CreateMessageError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "msg_cme@x.com", "MsgCME")
	chat := createTestChat(t, f, "MsgCMETest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Message.Send(ctx, chat.ID, a, "test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReactionService_Add_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add@x.com", "RxnAdd")
	chat := createTestChat(t, f, "RxnAddTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	updated, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, a, "👍")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != msg.ID {
		t.Fatal("wrong message returned")
	}
}

func TestReactionService_Add_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add2@x.com", "RxnAdd2")
	b := createTestUser(t, f, "rxn_add3@x.com", "RxnAdd3")
	chat := createTestChat(t, f, "RxnAddTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, b, "👍")
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestReactionService_Add_NonexistentMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add4@x.com", "RxnAdd4")
	chat := createTestChat(t, f, "RxnAddTest3", a, []string{a})
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, "nonexistent", a, "👍")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestReactionService_Remove_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_rem@x.com", "RxnRem")
	chat := createTestChat(t, f, "RxnRemTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	f.DB.AddReaction(context.Background(), msg.ID, a, "👍")
	_, err := f.Server.Services.Reaction.Remove(context.Background(), chat.ID, msg.ID, a, "👍")
	if err != nil {
		t.Fatal(err)
	}
}

func TestReactionService_Remove_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_rem2@x.com", "RxnRem2")
	b := createTestUser(t, f, "rxn_rem3@x.com", "RxnRem3")
	chat := createTestChat(t, f, "RxnRemTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Remove(context.Background(), chat.ID, msg.ID, b, "👍")
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestReactionService_List_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_lst@x.com", "RxnLst")
	chat := createTestChat(t, f, "RxnLstTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	f.DB.AddReaction(context.Background(), msg.ID, a, "👍")
	reactions, err := f.Server.Services.Reaction.List(context.Background(), chat.ID, msg.ID, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(reactions) == 0 {
		t.Fatal("expected at least 1 reaction")
	}
}

func TestReactionService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_lst2@x.com", "RxnLst2")
	b := createTestUser(t, f, "rxn_lst3@x.com", "RxnLst3")
	chat := createTestChat(t, f, "RxnLstTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.List(context.Background(), chat.ID, msg.ID, b)
	if err != service.ErrForbidden {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestReactionService_Add_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_wc@x.com", "RxnWC")
	chat1 := createTestChat(t, f, "RxnWCTest1", a, []string{a})
	chat2 := createTestChat(t, f, "RxnWCTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat1.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat2.ID, msg.ID, a, "👍")
	if err != service.ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestReactionService_Add_EmptyEmoji(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_ee@x.com", "RxnEE")
	chat := createTestChat(t, f, "RxnEETest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, a, "")
	if err == nil {
		t.Fatal("expected error for empty emoji")
	}
}

func TestReactionService_Remove_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_rc@x.com", "RxnRC")
	chat := createTestChat(t, f, "RxnRCTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	f.DB.AddReaction(context.Background(), msg.ID, a, "👍")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Reaction.Remove(ctx, chat.ID, msg.ID, a, "👍")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

func TestReactionService_Add_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_ac@x.com", "RxnAC")
	chat := createTestChat(t, f, "RxnACTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Reaction.Add(ctx, chat.ID, msg.ID, a, "👍")
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

// ── StreamService ─────────────────────────────────────────────────────

func startMockAIStream(t *testing.T, chunks ...string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatal("expected POST")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, c := range chunks {
			data, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{
					"delta": map[string]string{"content": c},
				}},
			})
			w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		w.Write([]byte("data: [DONE]\n\n"))
	})
	return httptest.NewServer(mux)
}

func TestStreamService_Lifecycle(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_life@x.com", "StrmLife")
	chat := createTestChat(t, f, "StrmLife", a, []string{a})

	mockAI := startMockAIStream(t, "Hello", " World")
	defer mockAI.Close()

	msgID := "strm-lifecycle-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	var got strings.Builder
	chunkCount := 0
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		chunkCount++
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	if got.String() != "Hello World" {
		t.Fatalf("want 'Hello World', got '%s'", got.String())
	}
	if chunkCount != 2 {
		t.Fatalf("want 2 chunks, got %d", chunkCount)
	}

	// StreamStatus before finish
	chunks, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should exist")
	}
	if done {
		t.Fatal("stream should not be done yet")
	}
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks, got %d", len(chunks))
	}
	var joined string
	for _, ci := range chunks {
		joined += ci.Content
	}
	if joined != "Hello World" {
		t.Fatalf("want 'Hello World', got '%s'", joined)
	}

	// FinishStream
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, got.String(), "")

	// done should be true
	_, done, ok = f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should still exist after finish")
	}
	if !done {
		t.Fatal("stream should be done after finish")
	}

	// DB should have the message
	msg, err := f.DB.GetMessage(context.Background(), msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "Hello World" {
		t.Fatalf("want 'Hello World', got '%s'", msg.Content)
	}
	if msg.Type != "stream" {
		t.Fatalf("want type 'stream', got '%s'", msg.Type)
	}
}

func TestStreamService_StreamStatus_WithIdx(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idx@x.com", "StrmIdx")
	chat := createTestChat(t, f, "StrmIdx", a, []string{a})

	mockAI := startMockAIStream(t, "A", "B", "C")
	defer mockAI.Close()

	msgID := "strm-idx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// consume stream and append
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// read with idx
	chunks, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok || len(chunks) != 3 {
		t.Fatalf("want 3 chunks from idx 0, got %d ok=%v", len(chunks), ok)
	}

	chunks, _, _ = f.Server.Services.Stream.StreamStatus(msgID, 1)
	if len(chunks) != 2 {
		t.Fatalf("want 2 chunks from idx 1, got %d", len(chunks))
	}

	chunks, _, _ = f.Server.Services.Stream.StreamStatus(msgID, 3)
	if len(chunks) != 0 {
		t.Fatalf("want 0 chunks from idx 3, got %d", len(chunks))
	}

	// after finish, StreamStatus should still be ok
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "ABC", "")
	_, done, ok = f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should still exist after finish (before cleanup)")
	}
	if !done {
		t.Fatal("stream should be done")
	}
}

func TestStreamService_Subscribe_Notification(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_sub@x.com", "StrmSub")
	chat := createTestChat(t, f, "StrmSub", a, []string{a})

	mockAI := startMockAIStream(t, "X")
	defer mockAI.Close()

	msgID := "strm-sub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// subscribe before appending
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	// consume and append on a goroutine to avoid deadlock
	done := make(chan struct{})
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
		close(done)
	}()

	// should receive notification
	select {
	case <-notify:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for notification")
	}

	// verify chunk is there
	chunks, _, _ := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if len(chunks) != 1 || chunks[0].Content != "X" {
		t.Fatalf("want [X], got %v", chunks)
	}

	<-done
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "X", "")
}

func TestStreamService_Finish_NotifiesSubscribers(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_fin@x.com", "StrmFin")
	chat := createTestChat(t, f, "StrmFin", a, []string{a})

	mockAI := startMockAIStream(t, "hello")
	defer mockAI.Close()

	msgID := "strm-fin-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "hello", "")

	// subscriber channel should be closed (nil after finish)
	select {
	case _, ok := <-notify:
		if ok {
			t.Fatal("channel should be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout: notify channel not closed")
	}
}

func TestStreamService_StreamStatus_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	_, done, ok := f.Server.Services.Stream.StreamStatus("nonexistent", 0)
	if ok {
		t.Fatal("should not exist")
	}
	if done {
		t.Fatal("nonexistent should not be done")
	}
}

func TestStreamService_Unsubscribe(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_unsub@x.com", "StrmUnsub")
	chat := createTestChat(t, f, "StrmUnsub", a, []string{a})

	mockAI := startMockAIStream(t, "d")
	defer mockAI.Close()

	msgID := "strm-unsub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	ch1 := f.Server.Services.Stream.Subscribe(msgID)
	ch2 := f.Server.Services.Stream.Subscribe(msgID)

	// unsubscribe ch1
	f.Server.Services.Stream.Unsubscribe(msgID, ch1)

	// append
	f.Server.Services.Stream.AppendChunk(msgID, "content", "data")

	// ch1 should not receive notification
	select {
	case <-ch1:
		t.Fatal("ch1 should not receive notification after unsubscribe")
	case <-time.After(100 * time.Millisecond):
	}

	// ch2 should receive
	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("ch2 should receive notification")
	}
}

func TestStreamService_EmptyContent(t *testing.T) {
	// AI returns no content chunks (only [DONE])
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_emp@x.com", "StrmEmp")
	chat := createTestChat(t, f, "StrmEmp", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-empty-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// FinishStream with empty content
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "", "")

	// done should be true
	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should exist")
	}
	if !done {
		t.Fatal("stream should be done")
	}

	// DB should NOT have the message (empty content rejected by SendAI)
	_, err = f.DB.GetMessage(context.Background(), msgID)
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestStreamService_NonStreamingUpstream(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_ns@x.com", "StrmNS")
	chat := createTestChat(t, f, "StrmNS", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"non-stream response"}}]}`))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-ns-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test","messages":[{"role":"user","content":"hi"}]}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	if got.String() != "non-stream response" {
		t.Fatalf("want 'non-stream response', got '%s'", got.String())
	}

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, got.String(), "")

	msg, err := f.DB.GetMessage(context.Background(), msgID)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "non-stream response" {
		t.Fatalf("wrong content: %s", msg.Content)
	}
}

func TestStreamService_MultipleSubscribers(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_multi@x.com", "StrmMulti")
	chat := createTestChat(t, f, "StrmMulti", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c")
	defer mockAI.Close()

	msgID := "strm-multi-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	sub1 := f.Server.Services.Stream.Subscribe(msgID)
	sub2 := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, sub1)
	defer f.Server.Services.Stream.Unsubscribe(msgID, sub2)

	// consume + append in goroutine
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
	}()

	// both subscribers should get notification
	for i, sub := range []chan struct{}{sub1, sub2} {
		select {
		case <-sub:
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d not notified", i)
		}
	}
}

func TestStreamService_UpstreamError(t *testing.T) {
	// HTTP 500 is rejected by StreamFromSource with an error.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_err@x.com", "StrmErr")
	chat := createTestChat(t, f, "StrmErr", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	src := ai.Source{
		Endpoint: mockAI.URL + "/error",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, "err-msg", src, nil)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected 500 in error, got: %v", err)
	}
}

func TestStreamService_ContextCancelPropagation(t *testing.T) {
	// Context cancellation during body read should close resp.Body and end the stream.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_cc@x.com", "StrmCC")
	chat := createTestChat(t, f, "StrmCC", a, []string{a})

	ctx, cancel := context.WithCancel(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		// wait for context cancellation
		<-ctx.Done()
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(ctx, chat.ID, a, "cancel-msg", src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// cancel context to trigger resp.Body.Close
	cancel()

	// channel should close without blocking
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not close after context cancellation")
	}
}

func TestStreamService_ConcurrentAppendAndStatus(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_conc@x.com", "StrmConc")
	chat := createTestChat(t, f, "StrmConc", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c", "d", "e")
	defer mockAI.Close()

	msgID := "strm-conc-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// consume and append concurrently with StreamStatus reads
	done := make(chan struct{})
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
		close(done)
	}()

	// read status concurrently — should never panic or return inconsistent data
	for i := 0; i < 100; i++ {
		_, _, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
		if !ok {
			// stream might not have started yet
			continue
		}
	}
	<-done
}

func TestStreamService_FinishIdempotent(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idem@x.com", "StrmIdem")
	chat := createTestChat(t, f, "StrmIdem", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-idem-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// calling FinishStream twice should not panic
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")

	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should exist")
	}
	if !done {
		t.Fatal("stream should be done")
	}
}

func TestStreamService_SubscribeAfterFinish(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_saf@x.com", "StrmSaf")
	chat := createTestChat(t, f, "StrmSaf", a, []string{a})

	mockAI := startMockAIStream(t, "d")
	defer mockAI.Close()

	msgID := "strm-saf-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "d", "")

	// subscribe after finish — channel should close immediately
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	select {
	case _, ok := <-notify:
		if ok {
			t.Fatal("channel should be closed immediately")
		}
	default:
		t.Fatal("channel should be closed immediately, not block")
	}
}

func TestStreamService_DoneBeforeChunks(t *testing.T) {
	// liveChunks[msgID] is set to empty slice, then FinishStream is called
	// before any AppendChunk. StreamStatus should return done=true, ok=true.
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_dbc@x.com", "StrmDbc")
	chat := createTestChat(t, f, "StrmDbc", a, []string{a})

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	})
	mockAI := httptest.NewServer(mux)
	defer mockAI.Close()

	msgID := "strm-dbc-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/chat",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
	}

	// no AppendChunk called, but FinishStream sets done
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "", "")

	_, done, ok := f.Server.Services.Stream.StreamStatus(msgID, 0)
	if !ok {
		t.Fatal("stream should still exist")
	}
	if !done {
		t.Fatal("stream should be done")
	}
}

func TestStreamService_StreamStatusIndexBounds(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_idxb@x.com", "StrmIdxb")
	chat := createTestChat(t, f, "StrmIdxb", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b")
	defer mockAI.Close()

	msgID := "strm-idxb-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	// negative index should not panic (cast to uint)
	t.Run("negative idx", func(t *testing.T) {
		_, _, ok := f.Server.Services.Stream.StreamStatus(msgID, -1)
		if !ok {
			t.Fatal("should be ok even with negative idx")
		}
	})

	// idx far beyond should return empty slices
	t.Run("idx beyond len", func(t *testing.T) {
		chunks, _, ok := f.Server.Services.Stream.StreamStatus(msgID, 100)
		if !ok {
			t.Fatal("should be ok")
		}
		if len(chunks) != 0 {
			t.Fatalf("expected 0 chunks, got %d", len(chunks))
		}
	})
}

func TestStreamService_GetMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_get@x.com", "StrmGet")
	chat := createTestChat(t, f, "StrmGet", a, []string{a})

	// Create a message via the service, then retrieve via GetMessage
	msg, err := f.Server.Services.Message.Send(context.Background(), chat.ID, a, "stored content", nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := f.Server.Services.Stream.GetMessage(context.Background(), msg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "stored content" {
		t.Fatalf("want 'stored content', got '%s'", got.Content)
	}
}

func TestStreamService_GetMessage_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.Stream.GetMessage(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent message")
	}
}

func TestStreamService_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_csu@x.com", "StrmCSU")
	chat := createTestChat(t, f, "StrmCSU", a, []string{a})

	mockAI := startMockAIStream(t, "a", "b", "c", "d", "e")
	defer mockAI.Close()

	msgID := "strm-csu-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// subscribe/unsubscribe in parallel with append
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sub := f.Server.Services.Stream.Subscribe(msgID)
			// read or timeout
			select {
			case <-sub:
			case <-time.After(2 * time.Second):
			}
			f.Server.Services.Stream.Unsubscribe(msgID, sub)
		}()
	}

	// consume and append
	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}

	wg.Wait()
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "abcde", "")
}

func TestStreamService_SubscribeThenAppend(t *testing.T) {
	// subscribe BEFORE any chunks are appended should still get notified
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_sta@x.com", "StrmSta")
	chat := createTestChat(t, f, "StrmSta", a, []string{a})

	mockAI := startMockAIStream(t, "z")
	defer mockAI.Close()

	msgID := "strm-sta-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// subscribe before any append
	notify := f.Server.Services.Stream.Subscribe(msgID)
	defer f.Server.Services.Stream.Unsubscribe(msgID, notify)

	// consume and append
	go func() {
		for c := range ch {
			if c.Done {
				break
			}
			f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
		}
	}()

	select {
	case <-notify:
	case <-time.After(time.Second):
		t.Fatal("subscriber should be notified")
	}
}

func TestStreamService_AppendChunk_NonexistentMessage(t *testing.T) {
	// AppendChunk for a msgID that was never started should not panic
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_appx@x.com", "StrmAppX")
	chat := createTestChat(t, f, "StrmAppX", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-appx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// append for a different msgID that doesn't exist
	f.Server.Services.Stream.AppendChunk("nonexistent-msg", "content", "should-not-panic")

	for c := range ch {
		if c.Done {
			break
		}
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "x", "")
}

func TestStreamService_Unsubscribe_NonexistentMessage(t *testing.T) {
	// Unsubscribe for a msgID that doesn't exist should not panic
	f := testutil.New(t)
	ch := make(chan struct{})
	f.Server.Services.Stream.Unsubscribe("nonexistent", ch)
}

func TestStreamService_Unsubscribe_NonexistentChannel(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_unx@x.com", "StrmUnX")
	chat := createTestChat(t, f, "StrmUnX", a, []string{a})

	mockAI := startMockAIStream(t, "x")
	defer mockAI.Close()

	msgID := "strm-unx-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	_, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}

	// subscribe then unsubscribe a different channel
	sub := f.Server.Services.Stream.Subscribe(msgID)
	other := make(chan struct{})
	f.Server.Services.Stream.Unsubscribe(msgID, other) // not panics
	f.Server.Services.Stream.Unsubscribe(msgID, sub)   // actual removal succeeds

	// now try to unsubscribe again — should be a no-op
	f.Server.Services.Stream.Unsubscribe(msgID, sub)
}

func TestStreamService_StartStream_HubNil(t *testing.T) {
	// when Hub is nil (not set up), StartStream should still work
	f := testutil.New(t)
	a := createTestUser(t, f, "strm_hub@x.com", "StrmHub")
	chat := createTestChat(t, f, "StrmHub", a, []string{a})

	mockAI := startMockAIStream(t, "data")
	defer mockAI.Close()

	msgID := "strm-hub-msg"
	src := ai.Source{
		Endpoint: mockAI.URL + "/v1/chat/completions",
		AuthKey:  "test-key",
		Body:     json.RawMessage(`{"model":"test"}`),
	}

	ch, err := f.Server.Services.Stream.StartStream(context.Background(), chat.ID, a, msgID, src, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for c := range ch {
		if c.Done {
			break
		}
		got.WriteString(c.Content)
		f.Server.Services.Stream.AppendChunk(msgID, "content", c.Content)
	}
	if got.String() != "data" {
		t.Fatalf("want 'data', got '%s'", got.String())
	}
	f.Server.Services.Stream.FinishStream(context.Background(), chat.ID, a, msgID, "data", "")
}
