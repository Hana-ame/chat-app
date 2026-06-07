package models

import "time"

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email,omitempty"`
	Username    string    `json:"username"`
	AvatarColor string    `json:"avatar_color"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

type Chat struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Name          string     `json:"name,omitempty"`
	IconColor     string     `json:"icon_color,omitempty"`
	OwnerID       string     `json:"owner_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	Members       []User     `json:"members,omitempty"`
	UnreadCount   int        `json:"unread_count"`
	LastMessage   *Message   `json:"last_message,omitempty"`
}

type ChatMember struct {
	ChatID            string    `json:"chat_id"`
	UserID            string    `json:"user_id"`
	JoinedAt          time.Time `json:"joined_at"`
	LastReadMessageID string    `json:"last_read_message_id,omitempty"`
}

type Message struct {
	ID          string       `json:"id"`
	ChatID      string       `json:"chat_id"`
	UserID      string       `json:"user_id"`
	Author      *User        `json:"author,omitempty"`
	Content     string       `json:"content"`
	CreatedAt   time.Time    `json:"created_at"`
	EditedAt    *time.Time   `json:"edited_at,omitempty"`
	Deleted     bool         `json:"deleted"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Reactions   []Reaction   `json:"reactions,omitempty"`
	Mentions    []string     `json:"mentions,omitempty"`
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
	UserIDs []string `json:"user_ids"`
	Me      bool     `json:"me"`
}

type RefreshToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
