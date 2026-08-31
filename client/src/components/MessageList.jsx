import { useEffect, useRef, useCallback, useState } from 'react';
import MessageItem from './MessageItem';

// 【本地改动 2026-09-03】Jump to present（对齐 chatto FDR-014 行为）：
// - 用户接近底部（<300px）时，新消息自动滚动到底（保留原逻辑）。
// - 用户离开底部往上翻历史时，新消息到达【不强制滚动】，底部浮出
//   「↓ N 条新消息」胶囊；点击立即回到底部并清零。
// - 用户自行滚回底部也清零。
// 纯前端增强，无后端改动。

const NEAR_BOTTOM_PX = 300;

export default function MessageList({ messages, hasMore, loading, onLoadMore, chatId, backgroundStyle, hasBackground, onReply }) {
  const bodyRef = useRef(null);
  const snapshotRef = useRef(null);
  const loadingMoreRef = useRef(false);
  const prevChatIdRef = useRef(null);
  const nearBottomRef = useRef(true);      // 用户最近一次滚动是否接近底部
  const newestIdRef = useRef(null);        // 已计入「新消息数」的最新消息 id
  const [jumpCount, setJumpCount] = useState(0);

  const handleLoadMore = useCallback(() => {
    if (loadingMoreRef.current || loading) return;
    if (bodyRef.current) {
      snapshotRef.current = {
        scrollHeight: bodyRef.current.scrollHeight,
        scrollTop: bodyRef.current.scrollTop,
      };
    }
    loadingMoreRef.current = true;
    Promise.resolve(onLoadMore()).finally(() => {
      loadingMoreRef.current = false;
    });
  }, [onLoadMore, loading]);

  // 滚动监听：离开/回到底部时更新 nearBottom 状态
  useEffect(() => {
    const el = bodyRef.current;
    if (!el) return;
    const onScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = el;
      const near = scrollHeight - scrollTop - clientHeight < NEAR_BOTTOM_PX;
      nearBottomRef.current = near;
      if (near) setJumpCount(0);
    };
    el.addEventListener('scroll', onScroll);
    return () => el.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => {
    if (snapshotRef.current && !loading && bodyRef.current) {
      const snap = snapshotRef.current;
      snapshotRef.current = null;
      requestAnimationFrame(() => {
        if (bodyRef.current) {
          bodyRef.current.scrollTop = bodyRef.current.scrollHeight - snap.scrollHeight + snap.scrollTop;
        }
      });
      return;
    }

    const isNewChat = prevChatIdRef.current !== chatId;
    if (isNewChat) {
      prevChatIdRef.current = chatId;
      nearBottomRef.current = true;
      newestIdRef.current = messages[messages.length - 1]?.id || null;
      setJumpCount(0);
    }

    if (messages.length > 0 && bodyRef.current && !loadingMoreRef.current) {
      const newestId = messages[messages.length - 1]?.id || null;
      if (isNewChat || nearBottomRef.current) {
        requestAnimationFrame(() => {
          if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
          }
        });
        nearBottomRef.current = true;
        setJumpCount(0);
      } else if (newestId && newestId !== newestIdRef.current) {
        // 用户离开底部时来了新消息 → 计数，不滚动
        setJumpCount(c => c + 1);
      }
      newestIdRef.current = newestId;
    }
  }, [chatId, messages, loading]);

  const handleJumpToPresent = () => {
    if (bodyRef.current) {
      bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
    }
    nearBottomRef.current = true;
    setJumpCount(0);
  };

  return (
    <div className={'chat-body' + (hasBackground ? ' has-bg' : '')} ref={bodyRef} style={{ ...backgroundStyle, position: 'relative' }}>
      <div>
        {hasMore && !loading && messages.length > 0 && (
          <div style={{ textAlign: 'center', padding: 8 }}>
            <button className="btn-ghost" onClick={handleLoadMore} style={{ fontSize: 13 }}>Load older messages</button>
          </div>
        )}
        {loading && <div style={{ textAlign: 'center', padding: 8, color: 'var(--text-muted)', fontSize: 13 }}>Loading...</div>}
        {!loading && messages.length === 0 && (
          <div style={{ textAlign: 'center', padding: '40px 20px', color: 'var(--text-muted)', fontSize: 14, lineHeight: 1.6 }}>
            <div style={{ fontSize: 24, marginBottom: 8 }}>💬</div>
            <div>No messages yet. Start the conversation!</div>
          </div>
        )}
        {messages.map((msg, i) => {
          const prev = i > 0 ? messages[i - 1] : null;
          const sameAuthor = prev && prev.user_id === msg.user_id && !prev.deleted && !msg.deleted;
          return <MessageItem key={msg.id || `msg-${i}`} msg={msg} sameAuthor={sameAuthor} chatId={chatId} onReply={onReply} />;
        })}
      </div>
      {jumpCount > 0 && (
        <div style={{ position: 'sticky', bottom: 12, display: 'flex', justifyContent: 'center', zIndex: 10, pointerEvents: 'none' }}>
          <button
            className="btn btn-primary"
            onClick={handleJumpToPresent}
            style={{
              pointerEvents: 'auto',
              fontSize: 13,
              padding: '7px 16px',
              borderRadius: 999,
              boxShadow: '0 2px 12px rgba(0,0,0,0.35)',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <line x1="12" y1="5" x2="12" y2="19"/>
              <polyline points="19 12 12 19 5 12"/>
            </svg>
            {jumpCount} 条新消息
          </button>
        </div>
      )}
    </div>
  );
}
