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

// liveStream 是一条正在进行中的 AI 流式消息的全部内存状态。
// 之前用 5 个平行 map(liveChunks/liveSubs/liveDone/liveAuthor/liveChat)
// 分别跟踪同一 msgID 的片段,生命周期必须同步增删,漏一处就产生
// 悬空状态;聚合为单一结构后按 msgID 整体创建/删除。
type liveStream struct {
	chunks []ChunkInfo
	subs   []chan struct{}
	done   bool
	author *models.User
	chatID string
}

type StreamService struct {
	*Service
	liveMu sync.Mutex
	live   map[string]*liveStream
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
	s.live[msgID] = &liveStream{
		chunks: []ChunkInfo{},
		author: author,
		chatID: chatID,
	}
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
	st := s.live[msgID]
	if st != nil {
		st.chunks = append(st.chunks, ChunkInfo{Type: chunkType, Content: content})
		for _, sub := range st.subs {
			select {
			case sub <- struct{}{}:
			default:
			}
		}
	}
	s.liveMu.Unlock()
}

func (s *StreamService) FinishStream(ctx context.Context, chatID, userID, msgID, content, thinking string) {
	s.liveMu.Lock()
	st := s.live[msgID]
	if st != nil {
		st.done = true
		for _, ch := range st.subs {
			close(ch)
		}
		st.subs = nil
	}
	s.liveMu.Unlock()
	author := (*models.User)(nil)
	if st != nil {
		author = st.author
	}

	if _, err := s.Message.SendAI(ctx, chatID, userID, content, thinking, msgID, author); err != nil {
		logutil.Error("ai: save message failed: %v", err)
	}

	time.AfterFunc(30*time.Second, func() {
		s.liveMu.Lock()
		delete(s.live, msgID)
		s.liveMu.Unlock()
	})
}

func (s *StreamService) StreamStatus(msgID string, idx int) (chunks []ChunkInfo, done bool, ok bool) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	st, ok := s.live[msgID]
	if !ok {
		return nil, false, false
	}
	done = st.done
	if idx >= len(st.chunks) {
		return nil, done, true
	}
	if idx < 0 {
		idx = 0
	}
	return st.chunks[idx:], done, true
}

func (s *StreamService) Subscribe(msgID string) chan struct{} {
	ch := make(chan struct{}, 8)
	s.liveMu.Lock()
	if st := s.live[msgID]; st != nil && !st.done {
		st.subs = append(st.subs, ch)
	} else {
		close(ch)
	}
	s.liveMu.Unlock()
	return ch
}

func (s *StreamService) SubscribeFrom(msgID string, fromIdx int) (chunks []ChunkInfo, done bool, ok bool, notify chan struct{}) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	st, ok := s.live[msgID]
	if !ok {
		return nil, false, false, nil
	}
	done = st.done

	if fromIdx >= len(st.chunks) {
		chunks = nil
	} else {
		if fromIdx < 0 {
			fromIdx = 0
		}
		chunks = st.chunks[fromIdx:]
	}

	notify = make(chan struct{}, 8)
	if done {
		close(notify)
	} else {
		st.subs = append(st.subs, notify)
	}
	return chunks, done, true, notify
}

func (s *StreamService) Unsubscribe(msgID string, sub chan struct{}) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	st := s.live[msgID]
	if st == nil {
		return
	}
	for i, c := range st.subs {
		if c == sub {
			st.subs = append(st.subs[:i:i], st.subs[i+1:]...)
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
	st, ok := s.live[msgID]
	if !ok {
		return "", false
	}
	return st.chatID, true
}
