package service

import (
	"context"

	"github.com/Hana-ame/chat-app/server/internal/db"
	"github.com/Hana-ame/chat-app/server/internal/ws"
)

type Service struct {
	DB      *db.DB
	Hub     *ws.Hub
	Chat    *ChatService
	Message *MessageService
	Member  *MemberService
}

func New(database *db.DB, hub *ws.Hub) *Service {
	s := &Service{DB: database, Hub: hub}
	s.Chat = &ChatService{Service: s}
	s.Message = &MessageService{Service: s}
	s.Member = &MemberService{Service: s}
	return s
}

// WithTx is reserved for future cross-table transactions.
// DB methods (CreateChat, CreateMessage, etc.) handle their own internal txn.
func (s *Service) WithTx(ctx context.Context, fn func() error) error {
	return fn()
}
