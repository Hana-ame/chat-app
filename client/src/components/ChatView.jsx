import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import MessageItem from './MessageItem';
import Composer from './Composer';

function getDMName(chat, currentUserId) {
  if (chat.type !== 'dm') return chat.name;
  const other = chat.members?.find(m => m.id !== currentUserId);
  return other ? other.username : 'Unknown';
}

export default function ChatView({ chatId, onBack }) {
  const { user, accessToken } = useAuthStore();
  const { chats, messages, loadMessages, subscribe, markRead } = useChatStore();
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const bodyRef = useRef(null);

  const chat = useMemo(() => chats.find(c => c.id === chatId), [chats, chatId]);

  useEffect(() => {
    if (chatId && accessToken) {
      subscribe(chatId);
      loadMessages(accessToken, chatId);
      setHasMore(true);
    }
  }, [chatId, accessToken]);

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    const msgs = await api.listMessages(accessToken, chatId, messages[0]?.id, 50);
    const list = msgs.messages || [];
    if (list.length < 50) setHasMore(false);
    setLoading(false);
  }, [loading, hasMore, chatId, accessToken, messages]);

  const filtered = messages.filter(m => m.chat_id === chatId);

  useEffect(() => {
    if (filtered.length > 0) {
      const last = filtered[filtered.length - 1];
    }
  }, [filtered.length]);

  const name = chat ? getDMName(chat, user.id) : 'Loading...';
  const memberCount = chat?.members?.length || 0;
  const onlineCount = chat?.members?.filter(m => m.status === 'online')?.length || 0;

  return (
    <div className="main">
      <div className="chat-header">
        {onBack && <button className="btn-ghost" onClick={onBack} style={{fontSize:18}}>←</button>}
        <div style={{flex:1}}>
          <div style={{fontWeight:600}}>{name}</div>
          <div style={{fontSize:12,color:'var(--text-muted)'}}>
            {memberCount} member{memberCount !== 1 ? 's' : ''}{' '}
            {chat?.type !== 'dm' && <>{onlineCount} online</>}
          </div>
        </div>
      </div>
      <div className="chat-body" ref={bodyRef}>
        <div>
          {hasMore && !loading && (
            <div style={{textAlign:'center',padding:8}}>
              <button className="btn-ghost" onClick={loadMore} style={{fontSize:13}}>Load older messages</button>
            </div>
          )}
          {loading && <div style={{textAlign:'center',padding:8,color:'var(--text-muted)',fontSize:13}}>Loading...</div>}
          {filtered.map((msg, i) => {
            const prev = i < filtered.length - 1 ? filtered[i + 1] : null;
            const sameAuthor = prev && prev.user_id === msg.user_id && !prev.deleted && !msg.deleted;
            return <MessageItem key={msg.id} msg={msg} sameAuthor={sameAuthor} chatId={chatId} />;
          })}
        </div>
      </div>
      <Composer chatId={chatId} />
    </div>
  );
}
