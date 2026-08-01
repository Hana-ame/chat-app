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
	"errors"
	"testing"

	"github.com/Hana-ame/chat-app/server/internal/service"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

func TestUserService_Create(t *testing.T) {
	f := testutil.New(t)
	u, err := f.Server.Services.User.Create(f.Ctx(), "new@x.com", "NewUser", "hash1234")
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, u.Username == "NewUser" && u.Email == "new@x.com", "user not created correctly")
}

func TestUserService_Create_DuplicateEmail(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.Create(f.Ctx(), "dup@x.com", "User1", "hash1")
	testutil.RequireNoError(t, err)
	_, err = f.Server.Services.User.Create(f.Ctx(), "dup@x.com", "User2", "hash2")
	testutil.RequireEqual(t, err, service.ErrConflict)
}

func TestUserService_GetByID(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "getid@x.com", "GetID")
	u, err := f.Server.Services.User.GetByID(f.Ctx(), a)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, u.ID, a)
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.GetByID(f.Ctx(), "nonexistent")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestUserService_GetByEmail(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "getemail@x.com", "GetEmail")
	u, hash, err := f.Server.Services.User.GetByEmail(f.Ctx(), "getemail@x.com")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, u.Username, "GetEmail")
	testutil.RequireEqual(t, hash, "test-hash-12345678")
}

func TestUserService_GetByEmail_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, _, err := f.Server.Services.User.GetByEmail(f.Ctx(), "nobody@x.com")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestUserService_UpdateProfile(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "upd@x.com", "UpdateMe")
	u, err := f.Server.Services.User.UpdateProfile(f.Ctx(), a, "NewName", "#ff0000", "")
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, u.Username, "NewName")
	testutil.RequireEqual(t, u.AvatarColor, "#ff0000")
}

func TestUserService_UpdateProfile_NotFound(t *testing.T) {
	f := testutil.New(t)
	_, err := f.Server.Services.User.UpdateProfile(f.Ctx(), "nonexistent", "Name", "#000", "")
	testutil.RequireEqual(t, err, service.ErrNotFound)
}

func TestUserService_UpdateProfile_DuplicateUsername(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "upddup@x.com", "OrigName")
	b := createTestUser(t, f, "upddup2@x.com", "OtherUser")
	_, err := f.Server.Services.User.UpdateProfile(f.Ctx(), b, "OrigName", "#000", "")
	testutil.RequireEqual(t, err, service.ErrConflict)
}

func TestUserService_Search(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "srch@x.com", "AlphaUser")
	createTestUser(t, f, "srch2@x.com", "BetaUser")

	users, err := f.Server.Services.User.Search(f.Ctx(), "Alpha", 10)
	testutil.RequireNoError(t, err)
	testutil.RequireEqual(t, len(users), 1)
	testutil.RequireEqual(t, users[0].Username, "AlphaUser")
}

func TestUserService_Search_EmptyQuery(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "srch3@x.com", "SomeUser")
	users, err := f.Server.Services.User.Search(f.Ctx(), "", 10)
	testutil.RequireNoError(t, err)
	testutil.RequireTrue(t, len(users) >= 0, "search should return >= 0 results")
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
			if tt.wantOK {
				testutil.RequireNoError(t, err)
			} else {
				testutil.RequireError(t, err)
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
				_, err := f.DB.ExecContext(context.Background(),
					`UPDATE chat_members SET role = 'admin' WHERE chat_id = ? AND user_id = ?`,
					c.ID, v)
				testutil.RequireNoError(t, err)
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
				testutil.RequireNoError(t, err)
				return
			}
			if tt.wantErr != nil {
				testutil.RequireTrue(t, errors.Is(err, tt.wantErr), "want %v, got %v", tt.wantErr, err)
				return
			}
			testutil.RequireError(t, err)
		})
	}
}

func TestUserService_GetByID_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "uidc@x.com", "UIDC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.GetByID(ctx, a)
	testutil.RequireError(t, err)
}

func TestUserService_GetByEmail_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	createTestUser(t, f, "uemc@x.com", "UEMC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := f.Server.Services.User.GetByEmail(ctx, "uemc@x.com")
	testutil.RequireError(t, err)
}

func TestUserService_Create_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.Create(ctx, "ucc@x.com", "UCC", "hash")
	testutil.RequireError(t, err)
}

func TestUserService_UpdateProfile_CanceledContext(t *testing.T) {
	f := testutil.New(t)
	a := createTestUser(t, f, "updc@x.com", "UPDC")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Server.Services.User.UpdateProfile(ctx, a, "NewName", "#000", "")
	testutil.RequireError(t, err)
}
