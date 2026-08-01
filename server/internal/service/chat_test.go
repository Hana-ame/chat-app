// Package service_test 覆盖业务逻辑层:全部 service 方法、权限(成员/所有者/
// 管理员)、错误映射、context 取消传播、DB 错误注入(WithTx 回滚)、
// StreamService 全生命周期、并发场景。
//
// 运行方式: cd server && go test ./internal/service/
// 说明:AI 上游用 httptest 假 SSE server(见 stream_test.go),DB 为真实
// SQLite 临时库。
package service_test

import (
	"context"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestChatService_ListForUser(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "list_a@x.com", "ListA")
	b := createTestUser(t, f, "list_b@x.com", "ListB")
	createTestChat(t, f, "Chat1", a, []string{a, b})
	createTestChat(t, f, "Chat2", a, []string{a})

	chats, err := f.Server.Services.Chat.ListForUser(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, len(chats), 2)
}

func TestChatService_ListForUser_Empty(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "list_empty@x.com", "ListEmpty")
	chats, err := f.Server.Services.Chat.ListForUser(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, len(chats), 0)
}

func TestChatService_GetByID_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get@x.com", "GetUser")
	chat := createTestChat(t, f, "GetTest", a, []string{a})

	got, err := f.Server.Services.Chat.GetByID(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, got.ID, chat.ID)
}

func TestChatService_GetByID_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get_a@x.com", "GetA")
	b := createTestUser(t, f, "get_b@x.com", "GetB")
	chat := createTestChat(t, f, "GetTest", a, []string{a})

	_, err := f.Server.Services.Chat.GetByID(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_GetByID_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "get_c@x.com", "GetC")
	_, err := f.Server.Services.Chat.GetByID(f.Ctx(), "nonexistent", a)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_Create_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "create@x.com", "CreateUser")
	b := createTestUser(t, f, "create_b@x.com", "CreateB")

	chat, err := f.Server.Services.Chat.Create(f.Ctx(), a, "NewChat", "public", []string{b})
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, chat.Name, "NewChat")
	testutil.RequireEqual(t, chat.OwnerID, a)
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, a)
	testutil.RequireTrue(t, ok, "owner should be auto-added as member")
	ok, _ = f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	testutil.RequireTrue(t, ok, "invited user should be member")
}

func TestChatService_Create_EmptyName(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "c_empty@x.com", "CEmpty")
	_, err := f.Server.Services.Chat.Create(f.Ctx(), a, "", "public", nil)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestChatService_Create_OnlyWhitespaceName(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "c_ws@x.com", "CWS")
	_, err := f.Server.Services.Chat.Create(f.Ctx(), a, "  ", "public", nil)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestChatService_Rename_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren@x.com", "RenUser")
	chat := createTestChat(t, f, "OldName", a, []string{a})

	updated, err := f.Server.Services.Chat.Rename(f.Ctx(), chat.ID, a, "NewName")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, updated.Name, "NewName")
}

func TestChatService_Rename_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_dm@x.com", "RenDM")
	b := createTestUser(t, f, "ren_dm2@x.com", "RenDM2")
	dm := createTestDM(t, f, a, b)

	_, err := f.Server.Services.Chat.Rename(f.Ctx(), dm.ID, a, "NewName")
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestChatService_Rename_NotOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_no@x.com", "RenNo")
	b := createTestUser(t, f, "ren_no2@x.com", "RenNo2")
	chat := createTestChat(t, f, "Test", a, []string{a, b})

	_, err := f.Server.Services.Chat.Rename(f.Ctx(), chat.ID, b, "NewName")
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_Rename_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ren_nf@x.com", "RenNF")
	_, err := f.Server.Services.Chat.Rename(f.Ctx(), "nonexistent", a, "NewName")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestChatService_Delete_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del@x.com", "DelUser")
	chat := createTestChat(t, f, "DeleteMe", a, []string{a})

	err := f.Server.Services.Chat.Delete(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
	_, err = f.DB.GetChat(f.Ctx(), chat.ID)
	testutil.RequireEqual(t, err, db.ErrNotFound)
}

func TestChatService_Delete_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_dm@x.com", "DelDM")
	b := createTestUser(t, f, "del_dm2@x.com", "DelDM2")
	dm := createTestDM(t, f, a, b)

	err := f.Server.Services.Chat.Delete(f.Ctx(), dm.ID, a)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestChatService_Delete_NotOwner(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_no@x.com", "DelNo")
	b := createTestUser(t, f, "del_no2@x.com", "DelNo2")
	chat := createTestChat(t, f, "Test", a, []string{a, b})

	err := f.Server.Services.Chat.Delete(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_Delete_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "del_nf@x.com", "DelNF")
	err := f.Server.Services.Chat.Delete(f.Ctx(), "nonexistent", a)
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestChatService_ListPublic(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pub@x.com", "PubUser")
	createTestChat(t, f, "Public1", a, []string{a})
	createTestChat(t, f, "Public2", a, []string{a})

	chats, err := f.Server.Services.Chat.ListPublic(f.Ctx(), 1, 20)
	testutil.RequireNoError(t, err)
	_ = chats
}

func TestChatService_Join_Success(t *testing.T) {
	f := testutil.New(t)
	owner := createTestUser(t, f, "join_own@x.com", "JoinOwner")
	member := createTestUser(t, f, "join_mem@x.com", "JoinMember")
	chat, err := f.DB.CreateChat(f.Ctx(), "group", "PublicChat", "public", owner, []string{owner})
	testutil.RequireNoError(t, err)
	got, err := f.Server.Services.Chat.Join(f.Ctx(), chat.ID, member)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, got.ID, chat.ID)
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, member)
	testutil.RequireTrue(t, ok, "member should have joined")
}

func TestChatService_Join_PrivateChat(t *testing.T) {
	f := testutil.New(t)
	owner := createTestUser(t, f, "join_priv@x.com", "JoinPriv")
	member := createTestUser(t, f, "join_priv2@x.com", "JoinPriv2")
	chat, err := f.DB.CreateChat(f.Ctx(), "group", "PrivateChat", "private", owner, []string{owner})
	testutil.RequireNoError(t, err)
	_, err = f.Server.Services.Chat.Join(f.Ctx(), chat.ID, member)
	testutil.RequireError(t, err)
}

func TestChatService_Join_Nonexistent(t *testing.T) {
	f := testutil.New(t)
	member := createTestUser(t, f, "join_nf@x.com", "JoinNF")
	_, err := f.Server.Services.Chat.Join(f.Ctx(), "nonexistent", member)
	testutil.RequireError(t, err)
}

func TestChatService_MarkRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mr@x.com", "MRUser")
	chat := createTestChat(t, f, "MRTest", a, []string{a})
	err := f.Server.Services.Chat.MarkRead(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
}

func TestChatService_MarkRead_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mr2@x.com", "MR2")
	b := createTestUser(t, f, "mr3@x.com", "MR3")
	chat := createTestChat(t, f, "MRTest2", a, []string{a})

	err := f.Server.Services.Chat.MarkRead(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_SetPinned(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pin@x.com", "PinUser")
	chat := createTestChat(t, f, "PinTest", a, []string{a})

	err := f.Server.Services.Chat.SetPinned(f.Ctx(), chat.ID, a, true)
	testutil.RequireNoError(t, err)

	members, err := f.DB.GetChatMembers(f.Ctx(), chat.ID)
	testutil.RequireNoError(t, err)
	var found bool
	for _, m := range members {
		if m.ID == a {
			found = true
			break
		}
	}
	testutil.RequireTrue(t, found, "should be a member")
}

func TestChatService_SetPinned_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "pin2@x.com", "Pin2")
	b := createTestUser(t, f, "pin3@x.com", "Pin3")
	chat := createTestChat(t, f, "PinTest2", a, []string{a})

	err := f.Server.Services.Chat.SetPinned(f.Ctx(), chat.ID, b, true)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_SetAnnouncement(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann@x.com", "AnnUser")
	b := createTestUser(t, f, "ann2@x.com", "Ann2")
	c := createTestUser(t, f, "ann3@x.com", "Ann3")
	chat := createTestChat(t, f, "AnnTest", a, []string{a, b, c})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, a, "Important notice")
	testutil.RequireNoError(t, err)
	got, err := f.DB.GetChat(f.Ctx(), chat.ID)
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, got.PinnedMessage != nil && got.PinnedMessage.Content == "Important notice", "announcement not set")
}

func TestChatService_SetAnnouncement_NotOwnerOrAdmin(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_no@x.com", "AnnNo")
	b := createTestUser(t, f, "ann_no2@x.com", "AnnNo2")
	c := createTestUser(t, f, "ann_no3@x.com", "AnnNo3")
	chat := createTestChat(t, f, "AnnTest2", a, []string{a, b, c})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, b, "test")
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_SetAnnouncement_SmallGroup(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_lt@x.com", "AnnLt")
	chat := createTestChat(t, f, "AnnTest3", a, []string{a})

	err := f.Server.Services.Chat.SetAnnouncement(f.Ctx(), chat.ID, a, "test")
	testutil.RequireNoError(t, err)
}

func TestChatService_ClearAnnouncement(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_clr@x.com", "AnnClr")
	b := createTestUser(t, f, "ann_clr2@x.com", "AnnClr2")
	c := createTestUser(t, f, "ann_clr3@x.com", "AnnClr3")
	chat := createTestChat(t, f, "AnnClrTest", a, []string{a, b, c})
	f.DB.SetPinnedMessage(f.Ctx(), chat.ID, "notice")

	err := f.Server.Services.Chat.ClearAnnouncement(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
	got, _ := f.DB.GetChat(f.Ctx(), chat.ID)
	testutil.RequireNil(t, got.PinnedMessage)
}

func TestChatService_ClearAnnouncement_NotOwnerOrAdmin(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_clr4@x.com", "AnnClr4")
	b := createTestUser(t, f, "ann_clr5@x.com", "AnnClr5")
	c := createTestUser(t, f, "ann_clr6@x.com", "AnnClr6")
	chat := createTestChat(t, f, "AnnClrTest2", a, []string{a, b, c})

	err := f.Server.Services.Chat.ClearAnnouncement(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestChatService_MarkAnnouncementRead(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "ann_rd@x.com", "AnnRd")
	chat := createTestChat(t, f, "AnnRdTest", a, []string{a})
	err := f.Server.Services.Chat.MarkAnnouncementRead(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
}

func TestChatService_CreateOrGetDM_Create(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_c@x.com", "DMC")
	b := createTestUser(t, f, "dm_c2@x.com", "DMC2")

	chat, existed, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, b)
	testutil.RequireNoError(t, err)
	testutil.RequireFalse(t, existed, "should be new DM")
	testutil.RequireEqual(t, chat.Type, "dm")
}

func TestChatService_CreateOrGetDM_Existing(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_e@x.com", "DME")
	b := createTestUser(t, f, "dm_e2@x.com", "DME2")
	createTestDM(t, f, a, b)

	chat, existed, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, b)
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, existed, "DM should already exist")
	testutil.RequireEqual(t, chat.Type, "dm")
}

func TestChatService_CreateOrGetDM_UserNotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "dm_nf@x.com", "DMNF")

	_, _, err := f.Server.Services.Chat.CreateOrGetDM(f.Ctx(), a, "nonexistent")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestChatService_Create_UserAlreadyInList(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cal@x.com", "Cal")
	chat, err := f.Server.Services.Chat.Create(context.Background(), a, "TestChat", "public", []string{a})
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, chat.Name, "TestChat")
}

func TestChatService_Create_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cc@x.com", "CC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Create(ctx, a, "Test", "public", nil)
	testutil.RequireError(t, err)
}

func TestChatService_CreateOrGetDM_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cdm@x.com", "CDM")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := f.Server.Services.Chat.CreateOrGetDM(ctx, a, "otherid")
	testutil.RequireError(t, err)
}

func TestChatService_Rename_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rnc@x.com", "RNC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Rename(ctx, "chatid", a, "NewName")
	testutil.RequireError(t, err)
}

func TestChatService_Delete_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "delc@x.com", "DelC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.Delete(ctx, "chatid", a)
	testutil.RequireError(t, err)
}

func TestChatService_Join_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "jnc@x.com", "JNC")
	b := createTestUser(t, f, "jnc2@x.com", "JNC2")
	chat, _ := f.DB.CreateChat(context.Background(), "group", "JNCTest", "public", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Chat.Join(ctx, chat.ID, b)
	testutil.RequireError(t, err)
}

func TestChatService_SetAnnouncement_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "sae@x.com", "SAE")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.SetAnnouncement(ctx, "chatid", a, "test")
	testutil.RequireError(t, err)
}

func TestChatService_ClearAnnouncement_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "cae@x.com", "CAE")
	b := createTestUser(t, f, "cae2@x.com", "CAE2")
	chat := createTestChat(t, f, "CAETest", a, []string{a, b})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.ClearAnnouncement(ctx, chat.ID, a)
	testutil.RequireError(t, err)
}

func TestChatService_SetPinned_DBError(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "spe@x.com", "SPE")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.SetPinned(ctx, "chatid", a, true)
	testutil.RequireError(t, err)
}

func TestChatService_MarkRead_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "chmrc@x.com", "ChMrC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Chat.MarkRead(ctx, "chatid", a)
	testutil.RequireError(t, err)
}
