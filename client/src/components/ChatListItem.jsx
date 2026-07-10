import { useRef } from 'react';
import { useAuthStore } from '../store/auth';

function timeAgo(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = now - d;
  if (diff < 60e3) return 'now';
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

export default function ChatListItem({ chat, activeId, onSelectChat, onContextMenu }) {
  const { user } = useAuthStore();
  const btnRef = useRef(null);

  const name = chat.name || chat.id;
  const avatar = chat.icon_color || '#5865F2';
  const unread = chat.unread_count || 0;

  const handleMenu = (e) => {
    e.stopPropagation();
    const rect = btnRef.current?.getBoundingClientRect();
    onContextMenu({ chatId: chat.id, x: rect?.right || 0, y: rect?.bottom || 0 });
  };

  return (
    <div key={chat.id} className={'chat-item' + (chat.id === activeId ? ' active' : '') + (chat.pinned ? ' pinned' : '') + (chat.visibility === 'public' ? ' public' : '') + (chat.owner_id === user.id ? ' owner' : '')}
      onClick={() => onSelectChat(chat.id)}>
      <div className="chat-item-avatar" style={{ background: avatar }}>
        {name ? name[0].toUpperCase() : '?'}
      </div>
      <div className="chat-item-info">
        <div style={{display:'flex',alignItems:'center',gap:4}}>
          <div className="chat-item-name">{name}</div>
          <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background: chat.visibility === 'public' ? 'rgba(35,165,89,0.15)' : chat.visibility === 'unlisted' ? 'rgba(88,101,242,0.15)' : 'rgba(128,132,142,0.15)', color: chat.visibility === 'public' ? '#23a559' : chat.visibility === 'unlisted' ? '#5865F2' : 'var(--text-muted)'}}>
            {chat.visibility || 'private'}
          </span>
        </div>
        <div className="chat-item-preview">
          {chat.last_message ? (chat.last_message.deleted ? '(message deleted)' : chat.last_message.author?.username + ': ' + chat.last_message.content) : ''}
        </div>
      </div>
      <div className="chat-item-meta">
        <div className="chat-item-time">{timeAgo(chat.last_message_at)}</div>
        {unread > 0 ? <div className="unread-badge">{unread}</div> : <div style={{ height: 18 }} />}
        <div className="chat-item-menu-wrap">
          <button ref={btnRef} className="btn-ghost chat-item-menu-btn" title="More" onClick={handleMenu}>⋮</button>
        </div>
      </div>
    </div>
  );
}
