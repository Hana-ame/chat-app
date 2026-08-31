import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import MessageItem from './MessageItem';
import { dateKey, formatDateDivider } from '../utils/messageDates';
import { computeUnreadIndex } from '../utils/unreadBoundary';

// 【本地改动 2026-09-03】Jump to present（对齐 chatto FDR-014 行为）：
// - 用户接近底部（<300px）时，新消息自动滚动到底（保留原逻辑）。
// - 用户离开底部往上翻历史时，新消息到达【不强制滚动】，底部浮出
//   「↓ N 条新消息」胶囊；点击立即回到底部并清零。
// - 用户自行滚回底部也清零。
// 纯前端增强，无后端改动。

const NEAR_BOTTOM_PX = 300;

export default function MessageList({ messages, hasMore, loading, onLoadMore, chatId, backgroundStyle, hasBackground, onReply, unreadSince }) {
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

  // 【本地改动 2026-09-03】日期分隔：相邻消息跨日时插入 Today/Yesterday/日期分隔线。
  // 用 flatMap 展平为 [divider?, item, divider?, item, ...]；分隔线不占 MessageItem 语义。
  const dateDividers = useMemo(() => {
    const keys = messages.map(m => dateKey(m.created_at));
    const divs = [];
    let prevKey = null;
    for (let i = 0; i < messages.length; i++) {
      const k = keys[i];
      if (k && k !== prevKey) divs.push(i);
      prevKey = k || prevKey;
    }
    return divs;
  }, [messages]);

  // 【本地改动 2026-09-03】未读分隔线：在"最后活跃时间"后的第一条消息上方插入
  // 「未读消息」标记。语义与后端 UnreadCount 完全一致（created_at > last_active_at）。
  // 只读计算，不改变消息数组。排序要求 messages 为时间升序（store 保证）。
  const unreadIdx = useMemo(() => computeUnreadIndex(messages, unreadSince), [messages, unreadSince]);

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
        {messages.flatMap((msg, i) => {
          const prev = i > 0 ? messages[i - 1] : null;
          const sameAuthor = prev && prev.user_id === msg.user_id && !prev.deleted && !msg.deleted;
          const unreadDivider = i === unreadIdx ? (
            <div key={`unread-${i}`} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', userSelect: 'none' }}>
              <div style={{ flex: 1, height: 1, background: 'var(--accent)', opacity: 0.4 }} />
              <span style={{ fontSize: 11, fontWeight: 600, color: 'var(--accent)' }}>未读消息</span>
              <div style={{ flex: 1, height: 1, background: 'var(--accent)', opacity: 0.4 }} />
            </div>
          ) : null;
          const divider = dateDividers.includes(i) ? (
            <div key={`div-${i}`} style={{ textAlign: 'center', padding: '10px 0 4px', fontSize: 11, color: 'var(--text-muted)', userSelect: 'none' }}>
              <span style={{ background: 'var(--bg-secondary)', padding: '3px 10px', borderRadius: 999 }}>
                {formatDateDivider(msg.created_at)}
              </span>
            </div>
          ) : null;
          return [
            unreadDivider,
            divider,
            <MessageItem key={msg.id || `msg-${i}`} msg={msg} sameAuthor={sameAuthor} chatId={chatId} onReply={onReply} />,
          ];
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
