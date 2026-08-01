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

	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestMemberService_List(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_lst@x.com", "MemLst")
	b := createTestUser(t, f, "mem_lst2@x.com", "MemLst2")
	chat := createTestChat(t, f, "MemList", a, []string{a, b})

	members, err := f.Server.Services.Member.List(f.Ctx(), chat.ID, a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, len(members), 2)
}

func TestMemberService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_lst3@x.com", "MemLst3")
	b := createTestUser(t, f, "mem_lst4@x.com", "MemLst4")
	chat := createTestChat(t, f, "MemList2", a, []string{a})

	_, err := f.Server.Services.Member.List(f.Ctx(), chat.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMemberService_Add(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add@x.com", "MemAdd")
	b := createTestUser(t, f, "mem_add2@x.com", "MemAdd2")
	chat := createTestChat(t, f, "MemAddTest", a, []string{a})

	updated, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, b)
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, updated.MemberCount >= 2, "member count should be at least 2")
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	testutil.RequireTrue(t, ok, "b should be a member")
}

func TestMemberService_Add_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add3@x.com", "MemAdd3")
	b := createTestUser(t, f, "mem_add4@x.com", "MemAdd4")
	c := createTestUser(t, f, "mem_add5@x.com", "MemAdd5")
	dm := createTestDM(t, f, a, b)

	_, err := f.Server.Services.Member.Add(f.Ctx(), dm.ID, a, c)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMemberService_Add_NonexistentTarget(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add6@x.com", "MemAdd6")
	chat := createTestChat(t, f, "MemAddTest2", a, []string{a})

	_, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, "nonexistent")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestMemberService_Add_Duplicate(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add7@x.com", "MemAdd7")
	chat := createTestChat(t, f, "MemAddTest3", a, []string{a})

	_, err := f.Server.Services.Member.Add(f.Ctx(), chat.ID, a, a)
	testutil.RequireEqual(t, err, service.ErrConflict)
}

func TestMemberService_Add_NotFoundChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add8@x.com", "MemAdd8")
	_, err := f.Server.Services.Member.Add(f.Ctx(), "nonexistent", a, a)
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestMemberService_Remove_Self(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem@x.com", "MemRem")
	b := createTestUser(t, f, "mem_rem2@x.com", "MemRem2")
	chat := createTestChat(t, f, "MemRemove", a, []string{a, b})

	err := f.Server.Services.Member.Remove(f.Ctx(), chat.ID, b, b)
	testutil.RequireNoError(t, err)
	ok, _ := f.DB.IsChatMember(f.Ctx(), chat.ID, b)
	testutil.RequireFalse(t, ok, "b should no longer be a member")
}

func TestMemberService_Remove_OwnerProtection(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem3@x.com", "MemRem3")
	b := createTestUser(t, f, "mem_rem4@x.com", "MemRem4")
	chat := createTestChat(t, f, "MemRemove2", a, []string{a, b})

	err := f.Server.Services.Member.Remove(f.Ctx(), chat.ID, b, a)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMemberService_Remove_DM(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem5@x.com", "MemRem5")
	b := createTestUser(t, f, "mem_rem6@x.com", "MemRem6")
	dm := createTestDM(t, f, a, b)

	err := f.Server.Services.Member.Remove(f.Ctx(), dm.ID, a, b)
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func TestMemberService_Remove_NotFound(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_rem7@x.com", "MemRem7")
	err := f.Server.Services.Member.Remove(f.Ctx(), "nonexistent", a, a)
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestMemberService_Add_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "addc@x.com", "AddC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Member.Add(ctx, "chatid", a, a)
	testutil.RequireError(t, err)
}

func TestMemberService_Remove_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "remc@x.com", "RemC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := f.Server.Services.Member.Remove(ctx, "chatid", a, a)
	testutil.RequireError(t, err)
}

func TestMemberService_List_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "lstc@x.com", "LstC")
	chat := createTestChat(t, f, "LstCTest", a, []string{a})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Member.List(ctx, chat.ID, a)
	testutil.RequireError(t, err)
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
	testutil.RequireNoError(t, err)
	err = f.Server.Services.Member.Remove(context.Background(), chat.ID, b, c)
	testutil.RequireNoError(t, err)
	ok, _ := f.DB.IsChatMember(context.Background(), chat.ID, c)
	testutil.RequireFalse(t, ok, "c should no longer be a member")
}

func TestMemberService_Remove_NonAdminRemovesOther(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_nar@x.com", "MemNAR")
	b := createTestUser(t, f, "mem_nar2@x.com", "MemNAR2")
	c := createTestUser(t, f, "mem_nar3@x.com", "MemNAR3")
	chat := createTestChat(t, f, "MemNARTest", a, []string{a, b, c})
	err := f.Server.Services.Member.Remove(context.Background(), chat.ID, b, c)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestMemberService_Remove_OwnerRemovesMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_orm@x.com", "MemORM")
	b := createTestUser(t, f, "mem_orm2@x.com", "MemORM2")
	chat := createTestChat(t, f, "MemORMTest", a, []string{a, b})
	err := f.Server.Services.Member.Remove(context.Background(), chat.ID, a, b)
	testutil.RequireNoError(t, err)
	ok, _ := f.DB.IsChatMember(context.Background(), chat.ID, b)
	testutil.RequireFalse(t, ok, "b should no longer be a member")
}

func TestMemberService_Add_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "mem_add9@x.com", "MemAdd9")
	b := createTestUser(t, f, "mem_add10@x.com", "MemAdd10")
	chat := createTestChat(t, f, "MemAddTest4", a, []string{a})
	_, err := f.Server.Services.Member.Add(context.Background(), chat.ID, b, a)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}
