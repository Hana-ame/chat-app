import { useEffect, useRef, useCallback, useState, useMemo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import MessageItem from './MessageItem';
import Composer from './Composer';
import { renderContent } from './renderContent';

function getDMName(chat) {
  if (chat.type !== 'dm') return chat.name;
  return chat.name || 'DM';
}

export default function ChatView({ chatId, onBack }) {
  const { user, accessToken } = useAuthStore();
  const { chats, messages, loadMessages, subscribe, markRead, pinnedMessage, setAnnouncement, clearAnnouncement, markAnnouncementRead } = useChatStore();
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(true);
  const [noticeInput, setNoticeInput] = useState('');
  const [isEditingNotice, setIsEditingNotice] = useState(false);
  const [showNotice, setShowNotice] = useState(true);
  const [memberCount, setMemberCount] = useState(0);
  const bodyRef = useRef(null);
  const loadingMoreRef = useRef(false);
  const prevChatIdRef = useRef(null);

  const chat = useMemo(() => chats.find(c => c.id === chatId), [chats, chatId]);

  useEffect(() => {
    if (!chatId || !accessToken) return;
    api.listMembers(accessToken, chatId).then(d => setMemberCount(d.members?.length || 0)).catch(() => notify('Failed to load members', 'error'));
  }, [chatId, accessToken]);

  useEffect(() => {
    if (chatId && accessToken) {
      subscribe(chatId);
      loadMessages(accessToken, chatId);
      setHasMore(true);
    }
  }, [chatId, accessToken]);

  useEffect(() => {
    if (showNotice && pinnedMessage[chatId]) {
      markAnnouncementRead(chatId);
    }
  }, [showNotice, chatId, pinnedMessage[chatId]]);

  // Re-fetch chat if deleted from store by onChatDelete (e.g. after leave)
  useEffect(() => {
    if (!chat && chatId && accessToken) {
      api.getChat(accessToken, chatId).then(data => {
        if (data && data.id) useChatStore.getState().onChatUpdate(data);
      }).catch(() => notify('Failed to load chat', 'error'));
    }
  }, [chatId, accessToken, chat]);

  const loadMore = useCallback(async () => {
    if (loading || !hasMore) return;
    setLoading(true);
    loadingMoreRef.current = true;
    const prevScrollHeight = bodyRef.current?.scrollHeight || 0;
    const prevScrollTop = bodyRef.current?.scrollTop || 0;

    try {
      const msgs = await api.listMessages(accessToken, chatId, messages[0]?.id, 100);
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
    } catch (e) {
      if (e.status === 429) {
        alert(e.message || 'Too many requests, please try again later');
      } else {
        console.error('Load more error:', e);
      }
      loadingMoreRef.current = false;
    }
    setLoading(false);
  }, [loading, hasMore, chatId, accessToken, messages]);

  const filtered = messages.filter(m => m.chat_id === chatId);

  useEffect(() => {
    if (loadingMoreRef.current) return;
    if (filtered.length > 0 && bodyRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = bodyRef.current;
      const isNewChat = prevChatIdRef.current !== chatId;

      if (isNewChat || (scrollHeight - scrollTop - clientHeight < 300)) {
        requestAnimationFrame(() => {
          if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
          }
        });
        prevChatIdRef.current = chatId;
      }
    }
  }, [chatId, messages]);

  const name = chat ? getDMName(chat) : 'Loading...';
  const userMap = useMemo(() => ({}), []);

  const handleSaveNotice = async () => {
    if (!noticeInput.trim()) return;
    try {
      await setAnnouncement(accessToken, chatId, noticeInput);
      setIsEditingNotice(false);
    } catch (e) {
      if (e.status === 429) alert(e.message);
      else console.error('Save notice error:', e);
    }
  };

  const handleClearNotice = async () => {
    try {
      await clearAnnouncement(accessToken, chatId);
      setNoticeInput('');
      setIsEditingNotice(false);
    } catch (e) {
      if (e.status === 429) alert(e.message);
      else console.error('Clear notice error:', e);
    }
  };

  if (!chat) return null;

  return (
    <div className="main">
      <div className="chat-header">
        {onBack && <button className="btn-ghost" onClick={onBack} style={{fontSize:18}}>←</button>}
        <div style={{flex:1}}>
            <div style={{fontWeight:600}}>{name}</div>
            <div style={{fontSize:12,color:'var(--text-muted)'}}>
              {memberCount} member{memberCount !== 1 ? 's' : ''}
            </div>
          </div>
          <button className="btn-ghost" style={{
            position: 'relative',
            padding: '6px 8px',
            background: (showNotice && pinnedMessage[chatId]) ? 'var(--bg-tertiary)' : 'transparent',
            borderRadius: 4,
            lineHeight: 0,
            opacity: pinnedMessage[chatId] ? 1 : 0.4,
          }} onClick={() => {
            if (!pinnedMessage[chatId]) return;
            setShowNotice(!showNotice);
          }}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M11 5L6 9H2v6h4l5 4V5z"/>
              <path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"/>
            </svg>
            {pinnedMessage[chatId] && chat?.pinned_updated_at && (!chat?.pinned_last_read_at || new Date(chat.pinned_updated_at) > new Date(chat.pinned_last_read_at)) && (
              <span style={{
                position: 'absolute', top: 2, right: 2,
                width: 8, height: 8, borderRadius: '50%',
                background: '#ed4245',
              }}/>
            )}
          </button>
        </div>

        {((showNotice && pinnedMessage[chatId]) || isEditingNotice) && (
          <div style={{
            background: 'var(--bg-tertiary)',
            borderBottom: '1px solid var(--border)',
            fontSize: 13,
            padding: '8px 16px',
            display: 'flex',
            flexDirection: 'column',
            gap: 8
          }}>
            <div style={{display: 'flex', alignItems: 'center', gap: 8}}>
              {isEditingNotice ? (
                <div style={{flex: 1, display: 'flex', gap: 8}}>
                  <input
                    className="input-field"
                    style={{flex: 1, fontSize: 13, padding: '4px 8px'}}
                    value={noticeInput}
                    onChange={e => setNoticeInput(e.target.value)}
                    autoFocus
                  />
                  <button className="btn btn-primary" style={{fontSize: 11, padding: '4px 8px'}} onClick={handleSaveNotice}>Save</button>
                  <button className="btn-ghost" style={{fontSize: 11, padding: '4px 8px'}} onClick={() => setIsEditingNotice(false)}>Cancel</button>
                </div>
              ) : showNotice ? (
                <div style={{flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                  <strong style={{color:'var(--accent)',marginRight:6}}>📢 公告</strong>
                  {renderContent(pinnedMessage[chatId]?.content, userMap)}
                </div>
              ) : null}
            </div>
            {showNotice && pinnedMessage[chatId] && chat?.owner_id === user.id && !isEditingNotice && (
              <div style={{display: 'flex', justifyContent: 'flex-end', gap: 8}}>
                <button className="btn-ghost" style={{fontSize: 11, padding: '2px 6px'}} onClick={() => { setNoticeInput(pinnedMessage[chatId]?.content); setIsEditingNotice(true); }}>Edit</button>
                <button className="btn-ghost danger" style={{fontSize: 11, padding: '2px 6px'}} onClick={handleClearNotice}>Clear</button>
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
