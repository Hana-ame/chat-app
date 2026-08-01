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

func TestReactionService_Add_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add@x.com", "RxnAdd")
	chat := createTestChat(t, f, "RxnAddTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	updated, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, a, "👍")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, updated.ID, msg.ID)
}

func TestReactionService_Add_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add2@x.com", "RxnAdd2")
	b := createTestUser(t, f, "rxn_add3@x.com", "RxnAdd3")
	chat := createTestChat(t, f, "RxnAddTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, b, "👍")
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestReactionService_Add_NonexistentMessage(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_add4@x.com", "RxnAdd4")
	chat := createTestChat(t, f, "RxnAddTest3", a, []string{a})
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, "nonexistent", a, "👍")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestReactionService_Remove_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_rem@x.com", "RxnRem")
	chat := createTestChat(t, f, "RxnRemTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	f.DB.AddReaction(context.Background(), msg.ID, a, "👍")
	_, err := f.Server.Services.Reaction.Remove(context.Background(), chat.ID, msg.ID, a, "👍")
	testutil.RequireNoError(t, err)
}

func TestReactionService_Remove_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_rem2@x.com", "RxnRem2")
	b := createTestUser(t, f, "rxn_rem3@x.com", "RxnRem3")
	chat := createTestChat(t, f, "RxnRemTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Remove(context.Background(), chat.ID, msg.ID, b, "👍")
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestReactionService_List_Success(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_lst@x.com", "RxnLst")
	chat := createTestChat(t, f, "RxnLstTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	f.DB.AddReaction(context.Background(), msg.ID, a, "👍")
	reactions, err := f.Server.Services.Reaction.List(context.Background(), chat.ID, msg.ID, a)
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, len(reactions) > 0, "expected at least 1 reaction")
}

func TestReactionService_List_NotMember(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_lst2@x.com", "RxnLst2")
	b := createTestUser(t, f, "rxn_lst3@x.com", "RxnLst3")
	chat := createTestChat(t, f, "RxnLstTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.List(context.Background(), chat.ID, msg.ID, b)
	testutil.RequireEqual(t, err, service.ErrForbidden)
}

func TestReactionService_Add_WrongChat(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_wc@x.com", "RxnWC")
	chat1 := createTestChat(t, f, "RxnWCTest1", a, []string{a})
	chat2 := createTestChat(t, f, "RxnWCTest2", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat1.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat2.ID, msg.ID, a, "👍")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestReactionService_Add_EmptyEmoji(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_ee@x.com", "RxnEE")
	chat := createTestChat(t, f, "RxnEETest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	_, err := f.Server.Services.Reaction.Add(context.Background(), chat.ID, msg.ID, a, "")
	testutil.RequireError(t, err)
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
	testutil.RequireError(t, err)
}

func TestReactionService_Add_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "rxn_ac@x.com", "RxnAC")
	chat := createTestChat(t, f, "RxnACTest", a, []string{a})
	msg, _ := f.DB.CreateMessage(context.Background(), chat.ID, a, "hello", nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.Reaction.Add(ctx, chat.ID, msg.ID, a, "👍")
	testutil.RequireError(t, err)
}

// ── StreamService ─────────────────────────────────────────────────────

// startMockAIStream 是 testkit.NewMockAIServer 的本地别名,见 testkit/mockai.go。
