package service

import (
	"context"
	"database/sql"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type Service struct {
	db       *db.DB
	hub      *ws.Hub
	Cfg      *config.Config
	Authz    *Authz
	Chat     *ChatService
	User     *UserService
	Message  *MessageService
	Member   *MemberService
	Reaction *ReactionService
	Stream   *StreamService
	// 【本地改动 2026-08-31】持久化通知服务（occurrence 存储 + 触发 + 清理），
	// 移植 chatto 的通知机制；Message 发送后经它触发提及/回复通知。
	Notification *NotificationService
}

func New(database *db.DB, hub *ws.Hub, cfg *config.Config) *Service {
	s := &Service{db: database, hub: hub, Cfg: cfg, Authz: &Authz{DB: database}}
	s.Chat = &ChatService{Service: s}
	s.User = &UserService{Service: s}
	s.Message = &MessageService{Service: s}
	s.Member = &MemberService{Service: s}
	s.Reaction = &ReactionService{Service: s}
	s.Stream = &StreamService{
		Service: s,
		live:    map[string]*liveStream{},
	}
	s.Notification = &NotificationService{Service: s}
	return s
}

func (s *Service) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateRefreshToken creates a new refresh token in the database.
func (s *Service) CreateRefreshToken(ctx context.Context, userID, hash string, ttl time.Duration) (*models.RefreshToken, error) {
	return s.db.CreateRefreshToken(ctx, userID, hash, ttl)
}

// FindAndDeleteRefreshToken atomically finds and deletes a refresh token.
func (s *Service) FindAndDeleteRefreshToken(ctx context.Context, hash string) (*models.RefreshToken, error) {
	return s.db.FindAndDeleteRefreshToken(ctx, hash)
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user.
func (s *Service) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	return s.db.DeleteUserRefreshTokens(ctx, userID)
}

// TrackLastActive updates the last_active_at timestamp for a chat member.
func (s *Service) TrackLastActive(ctx context.Context, chatID, userID string) error {
	return s.db.UpdateLastActiveAt(ctx, chatID, userID)
}

// BroadcastUserUpdate broadcasts a user update to all connected clients.
func (s *Service) BroadcastUserUpdate(u *models.User) {
	s.hub.BroadcastUserUpdate(u)
}

// OnlineUserIDs returns the list of currently connected user IDs.
func (s *Service) OnlineUserIDs() []string {
	return s.hub.OnlineUserIDs()
}

// SSERegister registers an SSE channel for a user.
func (s *Service) SSERegister(userID string, ch chan []byte) {
	s.hub.SSERegister(userID, ch)
}

// SSEUnregister unregisters an SSE channel for a user.
func (s *Service) SSEUnregister(userID string) {
	s.hub.SSEUnregister(userID)
}

// BroadcastMessageUpdate broadcasts a message update to all chat members.
func (s *Service) BroadcastMessageUpdate(m *models.Message) {
	s.hub.BroadcastMessageUpdate(m)
}
