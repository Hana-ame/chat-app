import { useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import ReactMarkdown from 'react-markdown';

const COMMON_EMOJI = ['👍','❤️','😂','🎉','😢','😡','👀','🔥','✅','❌'];

function timeFormat(t) {
  if (!t) return '';
  const d = new Date(t);
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export default function MessageItem({ msg, sameAuthor, chatId }) {
  const { user, accessToken } = useAuthStore();
  const isMe = msg.user_id === user.id;
  const [showEmoji, setShowEmoji] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState(msg.content);

  const author = msg.author || { username: 'Unknown', avatar_color: '#5865F2', id: msg.user_id };
  const initials = author.username ? author.username[0].toUpperCase() : '?';

  const handleReaction = async (emoji) => {
    const has = msg.reactions?.find(r => r.emoji === emoji && r.me);
    try {
      if (has) await api.removeReaction(accessToken, chatId, msg.id, emoji);
      else await api.addReaction(accessToken, chatId, msg.id, emoji);
    } catch {}
    setShowEmoji(false);
  };

  const handleEdit = async () => {
    if (!editText.trim()) return;
    try {
      await api.editMessage(accessToken, chatId, msg.id, editText);
      setEditing(false);
    } catch {}
  };

  const handleDelete = async () => {
    if (!confirm('Delete this message?')) return;
    try {
      await api.deleteMessage(accessToken, chatId, msg.id);
    } catch {}
  };

  return (
    <div className="msg-group">
      <div className={'msg-row' + (sameAuthor ? ' msg-continuation' : '')}>
        {!sameAuthor && (
          <div className="msg-avatar" style={{background:author.avatar_color}}>{initials}</div>
        )}
        <div style={{flex:1,minWidth:0}}>
          {!sameAuthor && (
            <div style={{display:'flex',alignItems:'baseline'}}>
              <span className="msg-author">{author.username}</span>
              <span className="msg-time">{timeFormat(msg.created_at)}</span>
              {msg.edited_at && <span className="msg-time">(edited)</span>}
            </div>
          )}
          {msg.deleted ? (
            <div className="msg-deleted">(message deleted)</div>
          ) : editing ? (
            <div style={{display:'flex',gap:8}}>
              <input className="input-field" value={editText} onChange={e=>setEditText(e.target.value)}
                style={{flex:1}} autoFocus onKeyDown={e=>e.key==='Enter'&&handleEdit()} />
              <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:12}} onClick={handleEdit}>Save</button>
              <button className="btn-ghost" style={{fontSize:12}} onClick={()=>setEditing(false)}>Cancel</button>
            </div>
          ) : (
            <div className="msg-content">
              <ReactMarkdown>{msg.content}</ReactMarkdown>
            </div>
          )}
          {msg.attachments?.map(a => (
            <div key={a.id} className="file-attach">
              {a.mime_type?.startsWith('image/')
                ? <img src={a.url} alt={a.filename} loading="lazy" />
                : <a href={a.url} target="_blank" rel="noreferrer" style={{fontSize:13}}>{a.filename}</a>
              }
            </div>
          ))}
          {msg.reactions?.length > 0 && (
            <div className="reaction-bar">
              {msg.reactions.map(r => (
                <div key={r.emoji} className={'reaction-chip' + (r.me ? ' me' : '')}
                  onClick={() => handleReaction(r.emoji)}>
                  <span>{r.emoji}</span>
                  <span style={{fontSize:12}}>{r.count}</span>
                </div>
              ))}
            </div>
          )}
          {!msg.deleted && (
            <div className="msg-actions">
              <button className="msg-btn" onClick={() => setShowEmoji(!showEmoji)}>😀</button>
              {isMe && <button className="msg-btn" onClick={() => { setEditing(true); setEditText(msg.content); }}>Edit</button>}
              {isMe && <button className="msg-btn" onClick={handleDelete}>Delete</button>}
            </div>
          )}
          {showEmoji && (
            <div className="emoji-picker">
              {COMMON_EMOJI.map(e => (
                <button key={e} className="emoji-btn" onClick={() => handleReaction(e)}>{e}</button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
