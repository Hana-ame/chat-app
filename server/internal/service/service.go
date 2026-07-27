package service

import (
	"context"
	"database/sql"

	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/models"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type Service struct {
	DB       *db.DB
	Hub      *ws.Hub
	Cfg      *config.Config
	Authz    *Authz
	Chat     *ChatService
	User     *UserService
	Message  *MessageService
	Member   *MemberService
	Reaction *ReactionService
	Stream   *StreamService
}

func New(database *db.DB, hub *ws.Hub, cfg *config.Config) *Service {
	s := &Service{DB: database, Hub: hub, Cfg: cfg, Authz: &Authz{DB: database}}
	s.Chat = &ChatService{Service: s}
	s.User = &UserService{Service: s}
	s.Message = &MessageService{Service: s}
	s.Member = &MemberService{Service: s}
	s.Reaction = &ReactionService{Service: s}
	s.Stream = &StreamService{
		Service:    s,
		liveChunks: map[string][]string{},
		liveSubs:   map[string][]chan struct{}{},
		liveDone:   map[string]bool{},
		liveAuthor: map[string]*models.User{},
	}
	return s
}

// WithTx runs fn inside a database transaction. If DB doesn't support
// transactions (e.g., in tests), calls fn directly.
func (s *Service) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
