// Package service_test 覆盖业务逻辑层:全部 service 方法、权限(成员/所有者/
// 管理员)、错误映射、context 取消传播、DB 错误注入(WithTx 回滚)、
// StreamService 全生命周期、并发场景。
//
// 运行方式: cd server && go test ./internal/service/
// 说明:AI 上游用 httptest 假 SSE server(见 stream_test.go),DB 为真实
// SQLite 临时库。
package service_test

import (
	"database/sql"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestService_New(t *testing.T) {
	f := testutil.New(t)
	testutil.RequireNotNil(t, f.Server.Services)
	testutil.RequireNotNil(t, f.Server.Services.Chat)
	testutil.RequireNotNil(t, f.Server.Services.Message)
	testutil.RequireNotNil(t, f.Server.Services.Member)
	testutil.RequireNotNil(t, f.Server.Services.User)
}

func TestService_WithTx(t *testing.T) {
	f := testutil.New(t)
	called := false
	err := f.Server.Services.WithTx(f.Ctx(), func(tx *sql.Tx) error {
		called = true
		return nil
	})
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, called, "fn not called")
}

func TestErrors(t *testing.T) {
	testutil.RequireEqual(t, service.ErrForbidden.Error(), "forbidden")
	testutil.RequireEqual(t, service.ErrNotFound.Error(), "not_found")
	testutil.RequireEqual(t, service.ErrInvalidInput.Error(), "invalid_input")
	testutil.RequireEqual(t, service.ErrConflict.Error(), "conflict")
	testutil.RequireEqual(t, service.ErrContentTooLong.Error(), "content too long")
}

func TestService_WithTx_Error(t *testing.T) {
	f := testutil.New(t)
	err := f.Server.Services.WithTx(f.Ctx(), func(tx *sql.Tx) error {
		return service.ErrInvalidInput
	})
	testutil.RequireEqual(t, err, service.ErrInvalidInput)
}

func createTestUser(t *testing.T, f *testutil.Fixture, email, username string) string {
	t.Helper()
	hash := "test-hash-12345678"
	u, err := f.DB.CreateUser(f.Ctx(), email, username, hash)
	testutil.RequireNoError(t, err)
	return u.ID
}

func createTestChat(t *testing.T, f *testutil.Fixture, name string, ownerID string, memberIDs []string) *models.Chat {
	t.Helper()
	chat, err := f.DB.CreateChat(f.Ctx(), "group", name, "", ownerID, memberIDs)
	testutil.RequireNoError(t, err)
	return chat
}

func createTestDM(t *testing.T, f *testutil.Fixture, u1, u2 string) *models.Chat {
	t.Helper()
	chat, err := f.DB.CreateChat(f.Ctx(), "dm", "", "", "", []string{u1, u2})
	testutil.RequireNoError(t, err)
	return chat
}
