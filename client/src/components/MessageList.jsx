import { useEffect, useRef, useCallback } from 'react';
import MessageItem from './MessageItem';

export default function MessageList({ messages, hasMore, loading, onLoadMore, chatId, backgroundStyle, hasBackground, onReply }) {
  const bodyRef = useRef(null);
  const snapshotRef = useRef(null);
  const loadingMoreRef = useRef(false);
  const prevChatIdRef = useRef(null);

  const handleLoadMore = useCallback(() => {
    if (loadingMoreRef.current || loading) return;
    if (bodyRef.current) {
      snapshotRef.current = {
        scrollHeight: bodyRef.current.scrollHeight,
        scrollTop: bodyRef.current.scrollTop,
      };
    }
    loadingMoreRef.current = true;
    onLoadMore();
    loadingMoreRef.current = false;
  }, [onLoadMore, loading]);

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

    if (messages.length > 0 && bodyRef.current && !loadingMoreRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = bodyRef.current;
      const isNewChat = prevChatIdRef.current !== chatId;
      if (isNewChat || (scrollHeight - scrollTop - clientHeight < 300)) {
        requestAnimationFrame(() => {
          if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
          }
        });
      }
      prevChatIdRef.current = chatId;
    }
  }, [chatId, messages, loading]);

  return (
    <div className={'chat-body' + (hasBackground ? ' has-bg' : '')} ref={bodyRef} style={backgroundStyle}>
      <div>
        {hasMore && !loading && messages.length > 0 && (
          <div style={{textAlign:'center',padding:8}}>
            <button className="btn-ghost" onClick={handleLoadMore} style={{fontSize:13}}>Load older messages</button>
          </div>
        )}
        {loading && <div style={{textAlign:'center',padding:8,color:'var(--text-muted)',fontSize:13}}>Loading...</div>}
        {!loading && messages.length === 0 && (
          <div style={{textAlign:'center',padding: '40px 20px',color:'var(--text-muted)',fontSize:14,lineHeight:1.6}}>
            <div style={{fontSize:24,marginBottom:8}}>💬</div>
            <div>No messages yet. Start the conversation!</div>
          </div>
        )}
        {messages.map((msg, i) => {
          const prev = i > 0 ? messages[i - 1] : null;
          const sameAuthor = prev && prev.user_id === msg.user_id && !prev.deleted && !msg.deleted;
          return <MessageItem key={msg.id || `msg-${i}`} msg={msg} sameAuthor={sameAuthor} chatId={chatId} onReply={onReply} />;
        })}
      </div>
    </div>
  );
}
