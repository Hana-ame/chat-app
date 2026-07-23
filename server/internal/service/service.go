package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/config"
	"github.com/Hana-ame/chat-app/server/internal/db"
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
}

func New(database *db.DB, hub *ws.Hub, cfg *config.Config) *Service {
	s := &Service{DB: database, Hub: hub, Cfg: cfg, Authz: &Authz{DB: database}}
	s.Chat = &ChatService{Service: s}
	s.User = &UserService{Service: s}
	s.Message = &MessageService{Service: s}
	s.Member = &MemberService{Service: s}
	s.Reaction = &ReactionService{Service: s}
	return s
}

// WithTx is reserved for future cross-table transactions.
// DB methods (CreateChat, CreateMessage, etc.) handle their own internal txn.
func (s *Service) WithTx(ctx context.Context, fn func() error) error {
	return fn()
}
