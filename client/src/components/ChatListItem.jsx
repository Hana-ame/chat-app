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
  const bgColor = chat.icon_color || '#5865F2';
  const raw = chat.unread_count || 0;
  const unread = raw >= 99 ? '99+' : raw;

  const handleMenu = (e) => {
    e.stopPropagation();
    if (!onContextMenu) return;
    const rect = btnRef.current?.getBoundingClientRect();
    onContextMenu({ chatId: chat.id, x: rect?.right || 0, y: rect?.bottom || 0 });
  };

  const bannerStyle = chat.banner_url ? {
    backgroundImage: `linear-gradient(rgba(0,0,0,${chat.banner_opacity ?? 0.9}),rgba(0,0,0,${chat.banner_opacity ?? 0.9})), url(${chat.banner_url})`,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  } : {};

  return (
    <div key={chat.id} className={'chat-item' + (chat.id === activeId ? ' active' : '') + (chat.pinned ? ' pinned' : '') + (chat.visibility === 'public' ? ' public' : '') + (chat.owner_id === user.id ? ' owner' : '')}
      onClick={() => onSelectChat(chat.id)} style={bannerStyle}>
      <div className="chat-item-avatar" style={{ background: chat.avatar_url ? 'none' : bgColor }}>
        {chat.avatar_url ? <img src={chat.avatar_url} alt="" className="chat-item-avatar-img" /> : (name ? name[0].toUpperCase() : '?')}
      </div>
      <div className="chat-item-info">
        <div style={{display:'flex',alignItems:'center',gap:4}}>
          <div className="chat-item-name">{name}</div>
          <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background: chat.visibility === 'public' ? 'var(--success-bg)' : chat.visibility === 'unlisted' ? 'var(--accent-bg)' : 'rgba(128,132,142,0.15)', color: chat.visibility === 'public' ? 'var(--success)' : chat.visibility === 'unlisted' ? 'var(--accent)' : 'var(--text-muted)'}}>
            {chat.visibility || 'private'}
          </span>
        </div>
        <div className="chat-item-preview">
          {chat.last_message ? (chat.last_message.deleted ? '(message deleted)' : chat.last_message.author?.username + ': ' + chat.last_message.content) : ''}
        </div>
      </div>
        <div className="chat-item-meta">
          <div className="chat-item-time">{timeAgo(chat.last_message_at)}</div>
          <div className="chat-item-menu-wrap">
            {raw > 0 ? <div className="unread-badge">{unread}</div> : null}
            <button ref={btnRef} className="btn-ghost chat-item-menu-btn" title="More" onClick={handleMenu}>⋮</button>
          </div>
        </div>
    </div>
  );
}
