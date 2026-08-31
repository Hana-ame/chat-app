package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email,omitempty"`
	Username      string    `json:"username"`
	AvatarColor   string    `json:"avatar_color"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	Status        string    `json:"status"`
	Role          string    `json:"role,omitempty"`
	LastSeen      time.Time `json:"last_seen,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	NotifyBlocked []string  `json:"notify_blocked,omitempty"`
}

type PinnedContent struct {
	Content  string    `json:"content"`
	PinnedAt time.Time `json:"pinned_at"`
}

// 【本地改动 2026-09-02】PinEntry = 一条聊天消息的置顶记录（区别于聊天公告 pinned_message）。
// 由 chat_pins 表承载关联元数据，Message 字段为指向消息的投影（前端用于"跳转到消息"）。
type PinEntry struct {
	ChatID    string    `json:"chat_id"`
	MessageID string    `json:"message_id"`
	PinnedBy  string    `json:"pinned_by"`
	PinnedAt  time.Time `json:"pinned_at"`
	Message   Message   `json:"message"`
}

type Chat struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Name          string    `json:"name,omitempty"`
	IconColor     string    `json:"icon_color,omitempty"`
	AvatarURL     string    `json:"avatar_url,omitempty"`
	BannerURL     string    `json:"banner_url,omitempty"`
	BannerOpacity float64   `json:"banner_opacity"`
	BackgroundURL string    `json:"background_url,omitempty"`
	Visibility    string    `json:"visibility,omitempty"`
	OwnerID       string    `json:"owner_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	MemberCount   int       `json:"member_count"`
	// Deprecated.
	UnreadCount      int            `json:"unread_count"`
	PinnedMessage    *PinnedContent `json:"pinned_message,omitempty"`
	PinnedUpdatedAt  *time.Time     `json:"pinned_updated_at,omitempty"`
	PinnedLastReadAt *time.Time     `json:"pinned_last_read_at,omitempty"`
	Pinned           bool           `json:"pinned"`
	NotifyEnabled    bool           `json:"notify_enabled"`
	LastActiveAt     *time.Time     `json:"last_active_at,omitempty"`
	LastMessageID    string         `json:"last_message_id,omitempty"`
	// Deprecated.
	LastMessage *Message `json:"last_message,omitempty"`
}

type ChatMember struct {
	ChatID       string     `json:"chat_id"`
	UserID       string     `json:"user_id"`
	Role         string     `json:"role"`
	JoinedAt     time.Time  `json:"joined_at"`
	LastActiveAt *time.Time `json:"last_active_at,omitempty"`
	// Deprecated.
	LastReadMessageID string     `json:"last_read_message_id,omitempty"`
	Pinned            bool       `json:"pinned"`
	PinnedLastReadAt  *time.Time `json:"pinned_last_read_at,omitempty"`
}

type Message struct {
	ID     string `json:"id"`
	ChatID string `json:"chat_id"`
	UserID string `json:"user_id"`
	Type   string `json:"type,omitempty"`
	// Deprecated.
	Author          *User           `json:"author,omitempty"`
	Content         string          `json:"content"`
	Thinking        string          `json:"thinking,omitempty"`
	StreamURL       string          `json:"stream_url,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	EditedAt        *time.Time      `json:"edited_at,omitempty"`
	DeletedAt       *time.Time      `json:"deleted_at,omitempty"`
	AttachmentCount int             `json:"attachment_count"`
	MentionCount    int             `json:"mention_count"`
	ReactionCount   int             `json:"reaction_count"`
	Attachments     json.RawMessage `json:"attachments,omitempty"`
	Reactions       json.RawMessage `json:"reactions,omitempty"`
	Mentions        json.RawMessage `json:"mentions,omitempty"`
	ReplyTo               string          `json:"reply_to,omitempty"`
	RepliedTo             *Message        `json:"replied_to,omitempty"`
	// 【本地改动 2026-08-31】线程语义：thread_root_message_id / reply_to：
	// thread_root_message_id 是自引用外键。空串 = 顶层消息；== id = 该消息
	// 即线程根（start_thread=true 时写入）；其他非空 = 回复在该根下。
	// reply_to_message_id 语义不变（父消息，用于线程内的嵌套回复）。
	ThreadRootMessageID string `json:"thread_root_message_id,omitempty"`
}

type Attachment struct {
	ID        string `json:"id"`
	MessageID string `json:"message_id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	URL       string `json:"url"`
}

type Reaction struct {
	Emoji   string   `json:"emoji"`
	Count   int      `json:"count"`
	UserIDs []string `json:"user_ids,omitempty"`
	Me      bool     `json:"me"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationOccurrence 是持久化通知的一条记录（移植 持久化通知机制的
// occurrence 语义）。行身份由 (user_id, kind, chat_id, message_id) 唯一，
// 同源事件重复触发不重复插行；read 标记已读；expires_at 为 TTL 由清理
// worker 删除。
type NotificationOccurrence struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Kind      string    `json:"kind"`
	ChatID    string    `json:"chat_id"`
	MessageID string    `json:"message_id"`
	ActorID   string    `json:"actor_id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Read      bool      `json:"read"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PushSubscription 是 Web Push 的一条浏览器订阅
// subscription 语义）。endpoint 由浏览器的 PushManager 签发、全局唯一；
// P256DH/Auth 为 RFC 8291 的订阅加密密钥，发送时用于加密 payload。订阅
// 不设 TTL：失效由发送时的 404/410 响应即时清除，用户注销由 FK CASCADE
// 清空。
type PushSubscription struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Endpoint  string    `json:"endpoint"`
	P256DH    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	CreatedAt time.Time `json:"created_at"`
}

// 【本地改动 2026-08-31】线程摘要紧凑模型。
// ThreadMeta 以嵌入方式展平到 ThreadSummary 顶层，与根消息并列，
// 避免客户端需要解一层嵌套结构。
type ThreadSummary struct {
	ThreadMeta
	RootMessage *Message `json:"root_message,omitempty"`
}

type ThreadMeta struct {
	ThreadRootMessageID string    `json:"thread_root_message_id"`
	ChatID              string    `json:"chat_id"`
	ReplyCount          int       `json:"reply_count"`
	LastReplyAt         time.Time `json:"last_reply_at"`
	LatestReplyID       string    `json:"latest_reply_id,omitempty"`
	IsFollowing         bool      `json:"is_following"`
	HasUnread           bool      `json:"has_unread"`
}
