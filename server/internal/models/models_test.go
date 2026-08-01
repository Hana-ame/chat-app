// Package models_test 覆盖 models 层数据结构。
//
// 测试范围:
//   - 各结构体 JSON 序列化/反序列化 roundtrip
//   - omitempty 字段在零值时不出现在 JSON 中(前端契约依赖此行为)
//   - Deprecated 字段仍按原 key 输出(兼容旧前端)
//
// 运行方式: cd server && go test ./internal/models/
// 说明:models 是纯数据结构,测试即契约文档——改字段先改这里的预期。
package models_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/testutil"
)

// marshal 辅助:把结构体编码成 JSON map,便于断言字段存在性。
func marshalMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	testutil.RequireNoError(t, err)
	var m map[string]any
	testutil.RequireNoError(t, json.Unmarshal(raw, &m))
	return m
}

func TestUserJSONContract(t *testing.T) {
	// User 的 JSON key 是前端契约:username/avatar_color/status/created_at 必现,
	// email/avatar_url/role/notify_blocked 零值省略。
	u := models.User{
		ID:          "u1",
		Username:    "alice",
		AvatarColor: "#ff0000",
		Status:      "online",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	m := marshalMap(t, u)

	testutil.RequireEqual(t, m["id"], "u1")
	testutil.RequireEqual(t, m["username"], "alice")
	testutil.RequireEqual(t, m["avatar_color"], "#ff0000")
	testutil.RequireEqual(t, m["status"], "online")
	testutil.RequireEqual(t, m["created_at"], "2026-01-01T00:00:00Z")
	// omitempty 字段:零值必须消失。
	testutil.RequireTrue(t, m["email"] == nil, "empty email should be omitted")
	testutil.RequireTrue(t, m["avatar_url"] == nil, "empty avatar_url should be omitted")
	testutil.RequireTrue(t, m["role"] == nil, "empty role should be omitted")
	testutil.RequireTrue(t, m["notify_blocked"] == nil, "empty notify_blocked should be omitted")

	// 非零值必须输出(带值序列化)。
	u.Email = "a@b.c"
	u.Role = "owner"
	u.NotifyBlocked = []string{"u9"}
	u.LastSeen = time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)
	m = marshalMap(t, u)
	testutil.RequireEqual(t, m["email"], "a@b.c")
	testutil.RequireEqual(t, m["role"], "owner")
	testutil.RequireEqual(t, m["notify_blocked"], []any{"u9"})
}

func TestUserJSONRoundtrip(t *testing.T) {
	// 完整字段序列化后反序列化,字段值不丢失。
	u := models.User{
		ID: "u1", Email: "a@b.c", Username: "alice", AvatarColor: "#fff",
		AvatarURL: "/uploads/a.png", Status: "offline", Role: "admin",
		LastSeen: time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(u)
	testutil.RequireNoError(t, err)
	var got models.User
	testutil.RequireNoError(t, json.Unmarshal(raw, &got))
	testutil.RequireEqual(t, got, u)
}

func TestChatJSONContract(t *testing.T) {
	// Chat 的必现字段(id/type/created_at/last_message_at/member_count/banner_opacity/pinned/notify_enabled)。
	c := models.Chat{
		ID: "c1", Type: "group", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastMessageAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MemberCount:   3,
	}
	m := marshalMap(t, c)
	testutil.RequireEqual(t, m["id"], "c1")
	testutil.RequireEqual(t, m["type"], "group")
	testutil.RequireEqual(t, m["member_count"], float64(3))
	testutil.RequireEqual(t, m["banner_opacity"], float64(0))
	testutil.RequireEqual(t, m["pinned"], false)
	testutil.RequireEqual(t, m["notify_enabled"], false)
	// Deprecated 字段 unread_count/last_message 仍以原 key 输出。
	testutil.RequireEqual(t, m["unread_count"], float64(0))

	// 指针字段零值为 nil 时省略。
	testutil.RequireTrue(t, m["pinned_message"] == nil, "nil pinned_message should be omitted")
	testutil.RequireTrue(t, m["last_message"] == nil, "nil last_message should be omitted")
}

func TestChatRoundtripWithPointers(t *testing.T) {
	// PinnedMessage/PinnedUpdatedAt 等指针字段的完整 roundtrip。
	now := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC)
	c := models.Chat{
		ID: "c1", Type: "dm", Name: "D", IconColor: "#123",
		PinnedMessage:   &models.PinnedContent{Content: "announcement", PinnedAt: now},
		PinnedUpdatedAt: &now,
		LastActiveAt:    &now,
		LastMessageID:   "m9",
	}
	raw, err := json.Marshal(c)
	testutil.RequireNoError(t, err)
	var got models.Chat
	testutil.RequireNoError(t, json.Unmarshal(raw, &got))
	testutil.RequireEqual(t, got.ID, c.ID)
	testutil.RequireNotNil(t, got.PinnedMessage)
	testutil.RequireEqual(t, got.PinnedMessage.Content, "announcement")
	testutil.RequireEqual(t, got.PinnedMessage.PinnedAt, now)
	testutil.RequireNotNil(t, got.PinnedUpdatedAt)
	testutil.RequireEqual(t, *got.PinnedUpdatedAt, now)
	testutil.RequireNotNil(t, got.LastActiveAt)
	testutil.RequireEqual(t, got.LastMessageID, "m9")
}

func TestMessageJSONContract(t *testing.T) {
	// Message 必现字段:id/chat_id/user_id/content/created_at/attachment_count/mention_count/reaction_count。
	m := models.Message{
		ID: "m1", ChatID: "c1", UserID: "u1",
		Content:   "hello",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	mm := marshalMap(t, m)
	testutil.RequireEqual(t, mm["id"], "m1")
	testutil.RequireEqual(t, mm["chat_id"], "c1")
	testutil.RequireEqual(t, mm["user_id"], "u1")
	testutil.RequireEqual(t, mm["content"], "hello")
	testutil.RequireEqual(t, mm["attachment_count"], float64(0))
	testutil.RequireEqual(t, mm["mention_count"], float64(0))
	testutil.RequireEqual(t, mm["reaction_count"], float64(0))
	// 可空字段零值省略。
	testutil.RequireTrue(t, mm["type"] == nil, "empty type should be omitted")
	testutil.RequireTrue(t, mm["thinking"] == nil, "empty thinking should be omitted")
	testutil.RequireTrue(t, mm["attachments"] == nil, "nil attachments should be omitted")
	testutil.RequireTrue(t, mm["edited_at"] == nil, "nil edited_at should be omitted")
}

func TestMessageRoundtripWithRawJSON(t *testing.T) {
	// Attachments/Reactions/Mentions 是 RawMessage,roundtrip 保真。
	m := models.Message{
		ID: "m1", ChatID: "c1", UserID: "u1", Content: "x",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Attachments: json.RawMessage(`[{"id":"a1","size":42}]`),
		Reactions:   json.RawMessage(`[{"emoji":"👍","count":1}]`),
		ReplyTo:     "m0",
	}
	raw, err := json.Marshal(m)
	testutil.RequireNoError(t, err)
	var got models.Message
	testutil.RequireNoError(t, json.Unmarshal(raw, &got))
	testutil.RequireEqual(t, string(got.Attachments), `[{"id":"a1","size":42}]`)
	testutil.RequireEqual(t, string(got.Reactions), `[{"emoji":"👍","count":1}]`)
	testutil.RequireEqual(t, got.ReplyTo, "m0")
}

func TestRefreshTokenHidesHash(t *testing.T) {
	// TokenHash 标记 json:"-",绝不能出现在任何 JSON 输出里(安全约束)。
	rt := models.RefreshToken{
		ID: "rt1", UserID: "u1", TokenHash: "supersecret",
		ExpiresAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt: time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(rt)
	testutil.RequireNoError(t, err)
	if bytes.Contains(raw, []byte("supersecret")) {
		t.Fatalf("token hash leaked into json: %s", raw)
	}

	m := marshalMap(t, rt)
	testutil.RequireEqual(t, m["id"], "rt1")
	testutil.RequireEqual(t, m["user_id"], "u1")
	testutil.RequireEqual(t, m["expires_at"], "2026-01-01T00:00:00Z")
	testutil.RequireEqual(t, m["created_at"], "2025-12-31T00:00:00Z")
	testutil.RequireTrue(t, m["token_hash"] == nil, "token_hash must not be serialized")
}

func TestAttachmentAndReactionContract(t *testing.T) {
	// Attachment/Reaction 的小字段 roundtrip。
	a := models.Attachment{ID: "a1", MessageID: "m1", Filename: "f.txt", MimeType: "text/plain", Size: 10, URL: "/api/local/x"}
	raw, err := json.Marshal(a)
	testutil.RequireNoError(t, err)
	var ga models.Attachment
	testutil.RequireNoError(t, json.Unmarshal(raw, &ga))
	testutil.RequireEqual(t, ga, a)

	r := models.Reaction{Emoji: "👍", Count: 2, UserIDs: []string{"u1", "u2"}, Me: true}
	raw, err = json.Marshal(r)
	testutil.RequireNoError(t, err)
	var gr models.Reaction
	testutil.RequireNoError(t, json.Unmarshal(raw, &gr))
	testutil.RequireEqual(t, gr, r)
}
