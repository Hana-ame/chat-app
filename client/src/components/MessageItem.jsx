import { useState, useMemo, useRef, useEffect, memo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { renderContent } from './renderContent';
import UserAvatar from './UserAvatar';
import UserProfileModal from './UserProfileModal';

const COMMON_EMOJI = ['👍','❤️','😂','🎉','😢','😡','👀','🔥','✅','❌'];

function timeFormat(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = now - d;
  if (diff < 60e3) return 'now';
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

const MessageItem = memo(function MessageItem({ msg, sameAuthor, chatId }) {
  const { user, accessToken } = useAuthStore();
  const { pinnedMessage, chats } = useChatStore();
  const isMe = msg.type === 'stream' ? false : msg.user_id === user.id;
  const [showEmoji, setShowEmoji] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState(msg.content);
  const [opPending, setOpPending] = useState(false);
  const [profileUser, setProfileUser] = useState(null);

  const chat = useMemo(() => chats.find(c => c.id === chatId), [chats, chatId]);
  const author = useMemo(() => {
    if (msg.type === 'stream' && msg.author) return msg.author;
    if (msg.user_id === user.id) return user;
    return chat?.members?.find(m => m.id === msg.user_id) || msg.author || { username: 'Unknown', avatar_color: '#5865F2', id: msg.user_id };
  }, [chat, msg.user_id, msg.author, user, msg.type]);
  const pickerRef = useRef(null);
  const emojiBtnRef = useRef(null);
  const [pickerPos, setPickerPos] = useState(null);

  useEffect(() => {
    if (!showEmoji) { setPickerPos(null); return; }
    const btn = emojiBtnRef.current;
    if (!btn) return;
    const rect = btn.getBoundingClientRect();
    const spaceAbove = rect.top;
    const spaceRight = window.innerWidth - rect.right;
    setPickerPos({
      left: spaceRight < 240 ? Math.max(0, window.innerWidth - 240) : rect.left,
      top: spaceAbove > 100 ? rect.top - 8 : rect.bottom,
    });
    const handler = (e) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target)) {
        setShowEmoji(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showEmoji]);

  const userMap = useMemo(() => {
    const chat = chats.find(c => c.id === chatId);
    if (!chat?.members) return {};
    const map = {};
    for (const m of chat.members) {
      map[m.id] = m.username;
    }
    return map;
  }, [chats, chatId]);

  const displayReactions = msg.reactions || [];

  const handleReaction = async (emoji) => {
    const has = displayReactions.find(r => r.emoji === emoji && r.me);
    try {
      if (has) await api.removeReaction(accessToken, chatId, msg.id, emoji);
      else await api.addReaction(accessToken, chatId, msg.id, emoji);
    } catch (e) { console.error('Reaction error:', e); }
    setShowEmoji(false);
  };

  const handleEdit = async () => {
    if (!editText.trim()) return;
    setOpPending(true);
    try {
      await api.editMessage(accessToken, chatId, msg.id, editText);
      setEditing(false);
    } catch (e) { console.error('Edit message error:', e); } finally {
      setOpPending(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm('Delete this message?')) return;
    setOpPending(true);
    try {
      await api.deleteMessage(accessToken, chatId, msg.id);
    } catch (e) { console.error('Delete message error:', e); } finally {
      setOpPending(false);
    }
  };

  return (
    <div className="msg-group">
      <div className={'msg-row' + (sameAuthor ? ' msg-continuation' : '')}>
          {!sameAuthor && (
            <div onClick={() => setProfileUser(author)} style={{ cursor: 'pointer' }}>
              <UserAvatar user={author} size={40} />
            </div>
          )}
        <div style={{flex:1,minWidth:0}}>
          {!sameAuthor && (
            <div style={{display:'flex',alignItems:'center',gap:6,flexWrap:'wrap'}}>
              <span className="msg-author" onClick={() => setProfileUser(author)} style={{cursor:'pointer'}}>{author.username}</span>
              {(author.role === 'admin' || author.role === 'owner' || author.id === chat?.owner_id) && (
                <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'rgba(88,101,242,0.15)',color:'#5865F2',lineHeight:'18px'}}>ADMIN</span>
              )}
              <span className="msg-time">{timeFormat(msg.created_at)}</span>
              {msg.edited_at && <span className="msg-time">(edited)</span>}
            </div>
          )}
          <div style={{position:'relative'}}>
            {msg.deleted ? (
              <div className="msg-deleted">(message deleted)</div>
            ) : editing ? (
              <div style={{display:'flex',gap:8}}>
                <textarea className="input-field" value={editText} onChange={e=>setEditText(e.target.value)}
                  style={{flex:1,resize:'none',minHeight:36}} autoFocus onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();handleEdit();}}} />
                 <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:12}} onClick={handleEdit} disabled={opPending}>
                   {opPending ? '...' : 'Save'}
                 </button>
                <button className="btn-ghost" style={{fontSize:12}} onClick={()=>setEditing(false)}>Cancel</button>
              </div>
            ) : (
              <>
                {msg.thinking && (
                  <details style={{marginBottom:4}} open={msg.streaming && !msg.content}>
                    <summary style={{fontSize:12,color:'var(--text-muted)',cursor:'pointer',userSelect:'none',opacity:0.7}}>
                      💭 {msg.streaming && !msg.content ? <span className="stream-cursor" /> : 'Thought'}
                    </summary>
                    <div style={{fontSize:12,color:'var(--text-muted)',padding:'4px 8px',background:'var(--bg-tertiary)',borderRadius:4,whiteSpace:'pre-wrap',wordBreak:'break-word',marginTop:4}}>
                      {msg.thinking}
                    </div>
                  </details>
                )}
                {msg.streaming ? (
                  <div className="msg-content" style={{whiteSpace:'pre-wrap',wordBreak:'break-word'}}>
                    {msg.content}<span className="stream-cursor" />
                  </div>
                ) : (
                  <div className="msg-content">
                    {renderContent(msg.content, userMap)}
                  </div>
                )}
              </>
            )}
            {!msg.deleted && (
              <div className="msg-actions">
                <button ref={emojiBtnRef} className="msg-btn" onClick={() => setShowEmoji(!showEmoji)} disabled={opPending}>😀</button>
                {isMe && <button className="msg-btn" onClick={() => { setEditing(true); setEditText(msg.content); }} disabled={opPending}>Edit</button>}
                {isMe && <button className="msg-btn" onClick={handleDelete} disabled={opPending}>Delete</button>}
              </div>
            )}
          </div>
          {msg.attachments?.map(a => (
              <div key={a.id} className="file-attach">
                {a.mime_type?.startsWith('image/')
                  ? <a href={a.url} target="_blank" rel="noreferrer"><img src={a.url} alt={a.filename} loading="lazy" className="file-attach-img" /></a>
                  : <div className="file-pill"><a href={a.url} target="_blank" rel="noreferrer" style={{fontSize:13}}>{a.filename}</a></div>
                }
              </div>
            ))}
          {!msg.deleted && displayReactions.length > 0 && (
            <div className="reaction-bar">
              {displayReactions.map(r => (
                <div key={r.emoji} className={'reaction-chip' + (r.me ? ' me' : '')}
                  onClick={() => handleReaction(r.emoji)}>
                  <span>{r.emoji}</span>
                  <span style={{fontSize:12}}>{r.count}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      {showEmoji && pickerPos && (
        <div ref={pickerRef} style={{
          position:'fixed', left: pickerPos.left, top: pickerPos.top, zIndex:9999,
          display:'flex', flexWrap:'wrap', gap:4, padding:8,
          background:'var(--bg-tertiary)', borderRadius:'var(--radius)',
          maxWidth:240, boxShadow:'0 2px 12px rgba(0,0,0,0.3)',
        }}>
          {COMMON_EMOJI.map(e => (
            <button key={e} className="emoji-btn" onClick={() => handleReaction(e)}>{e}</button>
          ))}
        </div>
      )}
      {profileUser && (
        <UserProfileModal user={profileUser} chatId={chatId} onClose={() => setProfileUser(null)} />
      )}
    </div>
  );
});

export default MessageItem;
