import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import MessageItem from './MessageItem';
import Composer from './Composer';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

function getDMName(chat, currentUserId) {
  if (chat.type !== 'dm') return chat.name;
  const other = chat.members?.find(m => m.id !== currentUserId);
  return other ? other.username : 'Unknown';
}

export default function ChatView({ chatId, onBack }) {
  const { user, accessToken } = useAuthStore();
  const { chats, messages, loadMessages, subscribe, markRead, pinnedMessages, pinMessage, unpinMessage } = useChatStore();
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [pinnedCollapsed, setPinnedCollapsed] = useState(false);
  const bodyRef = useRef(null);
  const loadingMoreRef = useRef(false);
  const prevChatIdRef = useRef(null);

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
    loadingMoreRef.current = true;
    const prevScrollHeight = bodyRef.current?.scrollHeight || 0;
    const prevScrollTop = bodyRef.current?.scrollTop || 0;

    const msgs = await api.listMessages(accessToken, chatId, messages[0]?.id, 50);
    const list = msgs.messages || [];
    if (list.length) {
      useChatStore.setState(s => ({ messages: [...list, ...s.messages] }));
      requestAnimationFrame(() => {
        if (bodyRef.current) {
          bodyRef.current.scrollTop = bodyRef.current.scrollHeight - prevScrollHeight + prevScrollTop;
        }
        loadingMoreRef.current = false;
      });
    } else {
      loadingMoreRef.current = false;
    }
    if (list.length < 50) setHasMore(false);
    setLoading(false);
  }, [loading, hasMore, chatId, accessToken, messages]);

  const filtered = messages.filter(m => m.chat_id === chatId);

  useEffect(() => {
    if (loadingMoreRef.current) return;
    if (filtered.length > 0 && bodyRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = bodyRef.current;
      const isNewChat = prevChatIdRef.current !== chatId;
      
      if (isNewChat || (scrollHeight - scrollTop - clientHeight < 100)) {
        requestAnimationFrame(() => {
          if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
          }
        });
        prevChatIdRef.current = chatId;
      }
    }
  }, [chatId, filtered.length]);

  const name = chat ? getDMName(chat, user.id) : 'Loading...';
  const memberCount = chat?.members?.length || 0;
  const onlineCount = chat?.members?.filter(m => m.status === 'online')?.length || 0;

  const addCustomPin = () => {
    const text = prompt('Enter text to pin:');
    if (text?.trim()) {
      pinMessage(chatId, { id: 'custom-' + Date.now(), content: text, type: 'custom' });
    }
  };

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

       {pinnedMessages[chatId] && pinnedMessages[chatId].length > 0 && (
         <div style={{
           background: 'var(--bg-tertiary)',
           borderBottom: '1px solid var(--border)',
           fontSize: 13,
           transition: 'max-height .2s ease-out',
           overflow: 'hidden',
           maxHeight: pinnedCollapsed ? '30px' : '200px'
         }}>
           <div style={{
             display: 'flex',
             alignItems: 'center',
             justifyContent: 'space-between',
             padding: '4px 16px',
             cursor: 'pointer',
             background: 'rgba(0,0,0,0.1)'
           }} onClick={() => setPinnedCollapsed(!pinnedCollapsed)}>
             <span style={{fontWeight: 600, fontSize: 12, color: 'var(--text-muted)'}}>
               📌 Pinned Messages ({pinnedMessages[chatId].length})
             </span>
             <span>{pinnedCollapsed ? '展开 ▽' : '收起 △'}</span>
           </div>
           {!pinnedCollapsed && (
             <div style={{padding: '8px 16px', display: 'flex', flexDirection: 'column', gap: 4}}>
               {pinnedMessages[chatId].map(p => (
                 <div key={p.id} style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8}}>
                   <div style={{flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                     <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ p: 'span' }}>{p.content}</ReactMarkdown>
                   </div>
                   <button className="btn-ghost" style={{fontSize: 11, padding: '2px 6px'}} onClick={() => unpinMessage(chatId, p.id)}>Remove</button>
                 </div>
               ))}
               <button className="btn-ghost" style={{fontSize: 11, textAlign: 'left', padding: '4px 0', color: 'var(--accent)'}} onClick={addCustomPin}>
                 + Add custom pin
               </button>
             </div>
           )}
         </div>
       )}

       <div className="chat-body" ref={bodyRef}>

        <div>
          {hasMore && !loading && filtered.length > 0 && (
            <div style={{textAlign:'center',padding:8}}>
              <button className="btn-ghost" onClick={loadMore} style={{fontSize:13}}>Load older messages</button>
            </div>
          )}
          {loading && <div style={{textAlign:'center',padding:8,color:'var(--text-muted)',fontSize:13}}>Loading...</div>}
          {!loading && filtered.length === 0 && (
            <div style={{textAlign:'center',padding: '40px 20px',color:'var(--text-muted)',fontSize:14,lineHeight:1.6}}>
              <div style={{fontSize:24,marginBottom:8}}>💬</div>
              <div>No messages yet. Start the conversation!</div>
            </div>
          )}
           {filtered.map((msg, i) => {
             const prev = i > 0 ? filtered[i - 1] : null;
             const sameAuthor = prev && prev.user_id === msg.user_id && !prev.deleted && !msg.deleted;
             return <MessageItem key={msg.id || `msg-${i}`} msg={msg} sameAuthor={sameAuthor} chatId={chatId} />;
           })}
        </div>
      </div>
      <Composer chatId={chatId} />
    </div>
  );
}
