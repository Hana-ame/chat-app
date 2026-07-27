package service

import (
	"context"
	"sync"
	"time"

	"github.com/Hana-ame/chat-app/server/internal/ai"
	"github.com/Hana-ame/chat-app/server/internal/logutil"
	"github.com/Hana-ame/chat-app/server/internal/models"
)

type StreamService struct {
	*Service
	liveMu     sync.Mutex
	liveChunks map[string][]string
	liveSubs   map[string][]chan struct{}
	liveDone   map[string]bool
	liveAuthor map[string]*models.User
}

func (s *StreamService) StartStream(ctx context.Context, chatID, userID, msgID string, src ai.Source, author *models.User) (<-chan ai.Chunk, error) {
	ch, err := ai.StreamFromSource(ctx, src)
	if err != nil {
		return nil, err
	}

	s.liveMu.Lock()
	s.liveChunks[msgID] = []string{}
	s.liveDone[msgID] = false
	s.liveAuthor[msgID] = author
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
	if s.Hub != nil {
		s.Hub.BroadcastMessageCreate(placeholder)
	}

	return ch, nil
}

func (s *StreamService) AppendChunk(msgID, content string) {
	s.liveMu.Lock()
	s.liveChunks[msgID] = append(s.liveChunks[msgID], content)
	subs := s.liveSubs[msgID]
	s.liveMu.Unlock()
	for _, sub := range subs {
		select {
		case sub <- struct{}{}:
		default:
		}
	}
}

func (s *StreamService) FinishStream(ctx context.Context, chatID, userID, msgID, content string) {
	s.liveMu.Lock()
	s.liveDone[msgID] = true
	author := s.liveAuthor[msgID]
	for _, ch := range s.liveSubs[msgID] {
		close(ch)
	}
	s.liveSubs[msgID] = nil
	s.liveMu.Unlock()

	if _, err := s.Message.SendAI(ctx, chatID, userID, content, msgID, author); err != nil {
		logutil.Error("ai: save message failed: %v", err)
	}

	time.AfterFunc(30*time.Second, func() {
		s.liveMu.Lock()
		delete(s.liveChunks, msgID)
		delete(s.liveSubs, msgID)
		delete(s.liveAuthor, msgID)
		// liveDone 保留 true，让后续 StreamStatus 能识别「流已结束」
		s.liveMu.Unlock()
	})
}

func (s *StreamService) StreamStatus(msgID string, idx int) (chunks []string, done bool, ok bool) {
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

// SubscribeFrom atomically reads chunks from fromIdx and subscribes for
// future notifications — no chunk can be lost between the read and subscribe.
func (s *StreamService) SubscribeFrom(msgID string, fromIdx int) (chunks []string, done bool, ok bool, notify chan struct{}) {
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
			s.liveSubs[msgID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

func (s *StreamService) GetMessage(ctx context.Context, msgID string) (*models.Message, error) {
	return s.DB.GetMessage(ctx, msgID)
}
