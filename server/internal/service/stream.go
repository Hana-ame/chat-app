package service

import (
	"context"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type ChunkInfo struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type StreamService struct {
	*Service
	liveMu     sync.Mutex
	liveChunks map[string][]ChunkInfo
	liveSubs   map[string][]chan struct{}
	liveDone   map[string]bool
	liveAuthor map[string]*models.User
	liveChat   map[string]string
}

func (s *StreamService) StartStream(ctx context.Context, chatID, userID, msgID string, src ai.Source, author *models.User) (<-chan ai.Chunk, error) {
	if err := ai.ValidateEndpoint(src.Endpoint, s.Cfg.AIAllowPrivateIPs); err != nil {
		return nil, err
	}
	ch, err := ai.StreamFromSource(ctx, src)
	if err != nil {
		return nil, err
	}

	s.liveMu.Lock()
	s.liveChunks[msgID] = []ChunkInfo{}
	s.liveDone[msgID] = false
	s.liveAuthor[msgID] = author
	s.liveChat[msgID] = chatID
	s.liveMu.Unlock()

	streamURL := "/api/chats/" + chatID + "/messages/" + msgID + "/stream"
	placeholder := &models.Message{
		ID:        msgID,
		ChatID:    chatID,
		UserID:    userID,
		Type:      "stream",
		Content:   "",
		StreamURL: streamURL,
		CreatedAt: time.Now().UTC(),
		Author:    author,
	}
	if s.hub != nil {
		s.hub.BroadcastMessageCreate(placeholder)
	}

	return ch, nil
}

func (s *StreamService) AppendChunk(msgID, chunkType, content string) {
	s.liveMu.Lock()
	s.liveChunks[msgID] = append(s.liveChunks[msgID], ChunkInfo{Type: chunkType, Content: content})
	for _, sub := range s.liveSubs[msgID] {
		select {
		case sub <- struct{}{}:
		default:
		}
	}
	s.liveMu.Unlock()
}

func (s *StreamService) FinishStream(ctx context.Context, chatID, userID, msgID, content, thinking string) {
	s.liveMu.Lock()
	s.liveDone[msgID] = true
	author := s.liveAuthor[msgID]
	for _, ch := range s.liveSubs[msgID] {
		close(ch)
	}
	s.liveSubs[msgID] = nil
	s.liveMu.Unlock()

	if _, err := s.Message.SendAI(ctx, chatID, userID, content, thinking, msgID, author); err != nil {
		logutil.Error("ai: save message failed: %v", err)
	}

	time.AfterFunc(30*time.Second, func() {
		s.liveMu.Lock()
		delete(s.liveChunks, msgID)
		delete(s.liveSubs, msgID)
		delete(s.liveAuthor, msgID)
		delete(s.liveChat, msgID)
		s.liveMu.Unlock()
	})
}

func (s *StreamService) StreamStatus(msgID string, idx int) (chunks []ChunkInfo, done bool, ok bool) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	done = s.liveDone[msgID]
	chunks, ok = s.liveChunks[msgID]
	if !ok {
		return nil, done, false
	}
	if idx >= len(chunks) {
		return nil, done, true
	}
	if idx < 0 {
		idx = 0
	}
	return chunks[idx:], done, true
}

func (s *StreamService) Subscribe(msgID string) chan struct{} {
	ch := make(chan struct{}, 8)
	s.liveMu.Lock()
	if s.liveDone[msgID] {
		close(ch)
	} else {
		s.liveSubs[msgID] = append(s.liveSubs[msgID], ch)
	}
	s.liveMu.Unlock()
	return ch
}

func (s *StreamService) SubscribeFrom(msgID string, fromIdx int) (chunks []ChunkInfo, done bool, ok bool, notify chan struct{}) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	done = s.liveDone[msgID]
	allChunks, ok := s.liveChunks[msgID]
	if !ok {
		return nil, done, false, nil
	}

	if fromIdx >= len(allChunks) {
		chunks = nil
	} else {
		if fromIdx < 0 {
			fromIdx = 0
		}
		chunks = allChunks[fromIdx:]
	}

	notify = make(chan struct{}, 8)
	if done {
		close(notify)
	} else {
		s.liveSubs[msgID] = append(s.liveSubs[msgID], notify)
	}
	return chunks, done, true, notify
}

func (s *StreamService) Unsubscribe(msgID string, sub chan struct{}) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	subs := s.liveSubs[msgID]
	for i, c := range subs {
		if c == sub {
			subs = append(subs[:i:i], subs[i+1:]...)
			s.liveSubs[msgID] = subs
			return
		}
	}
}

func (s *StreamService) GetMessage(ctx context.Context, msgID string) (*models.Message, error) {
	return s.db.GetMessage(ctx, msgID)
}

// LiveChatID returns the chat a live (in-buffer) stream message belongs to.
func (s *StreamService) LiveChatID(msgID string) (string, bool) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	chatID, ok := s.liveChat[msgID]
	return chatID, ok
}
