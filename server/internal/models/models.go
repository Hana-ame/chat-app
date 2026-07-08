package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email,omitempty"`
	Username    string    `json:"username"`
	AvatarColor string    `json:"avatar_color"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Status      string    `json:"status"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type PinnedContent struct {
	Content  string    `json:"content"`
	PinnedAt time.Time `json:"pinned_at"`
}

type Chat struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Name          string     `json:"name,omitempty"`
	IconColor     string     `json:"icon_color,omitempty"`
	Visibility    string     `json:"visibility,omitempty"`
	OwnerID       string     `json:"owner_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	MemberCount   int        `json:"member_count"`
	// Deprecated.
	UnreadCount   int        `json:"unread_count"`
	PinnedMessage  *PinnedContent `json:"pinned_message,omitempty"`
	// Deprecated.
	LastMessage   *Message   `json:"last_message,omitempty"`
}

type ChatMember struct {
	ChatID   string    `json:"chat_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	LastSeen time.Time `json:"last_seen,omitempty"`
	JoinedAt time.Time `json:"joined_at"`
	// Deprecated.
	LastReadMessageID string `json:"last_read_message_id,omitempty"`
	// Deprecated.
	Pinned bool `json:"pinned"`
}

type Message struct {
	ID              string       `json:"id"`
	ChatID          string       `json:"chat_id"`
	UserID          string       `json:"user_id"`
	// Deprecated.
	Author          *User        `json:"author,omitempty"`
	Content         string       `json:"content"`
	CreatedAt       time.Time    `json:"created_at"`
	EditedAt        *time.Time   `json:"edited_at,omitempty"`
	DeletedAt       *time.Time   `json:"deleted_at,omitempty"`
	AttachmentCount int          `json:"attachment_count"`
	MentionCount    int          `json:"mention_count"`
	ReactionCount   int          `json:"reaction_count"`
	Attachments     json.RawMessage `json:"attachments,omitempty"`
	Reactions       json.RawMessage `json:"reactions,omitempty"`
	Mentions        json.RawMessage `json:"mentions,omitempty"`
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
	Emoji   string `json:"emoji"`
	Count   int    `json:"count"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}