import { useRef } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';

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
  const storeChat = useChatStore(s => s.chats.find(c => c.id === chat.id));
  const merged = storeChat ? { ...chat, ...storeChat } : chat;
  const btnRef = useRef(null);

  const name = merged.name || merged.id;
  const bgColor = merged.icon_color || '#5865F2';
  const raw = merged.unread_count || 0;
  const unread = raw >= 99 ? '99+' : raw;

  const handleMenu = (e) => {
    e.stopPropagation();
    if (!onContextMenu) return;
    const rect = btnRef.current?.getBoundingClientRect();
    onContextMenu({ chatId: chat.id, x: rect?.right || 0, y: rect?.bottom || 0 });
  };

  const bannerStyle = merged.banner_url ? {
    backgroundImage: `linear-gradient(rgba(0,0,0,${merged.banner_opacity ?? 0.9}),rgba(0,0,0,${merged.banner_opacity ?? 0.9})), url(${merged.banner_url})`,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  } : {};

  return (
    <div key={merged.id} className={'chat-item' + (merged.id === activeId ? ' active' : '') + (merged.pinned ? ' pinned' : '') + (merged.visibility === 'public' ? ' public' : '') + (merged.owner_id === user.id ? ' owner' : '')}
      onClick={() => onSelectChat(merged.id)} style={bannerStyle}>
      <div className="chat-item-avatar" style={{ background: merged.avatar_url ? 'none' : (merged.type === 'notify' ? 'var(--accent)' : bgColor) }}>
        {merged.avatar_url ? <img src={merged.avatar_url} alt="" className="chat-item-avatar-img" /> : merged.type === 'notify' ? <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg> : (name ? name[0].toUpperCase() : '?')}
      </div>
      <div className="chat-item-info">
        <div style={{display:'flex',alignItems:'center',gap:4}}>
          <div className="chat-item-name">{name}</div>
          {merged.type !== 'notify' && <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background: merged.visibility === 'public' ? 'var(--success-bg)' : merged.visibility === 'unlisted' ? 'var(--accent-bg)' : 'rgba(128,132,142,0.15)', color: merged.visibility === 'public' ? 'var(--success)' : merged.visibility === 'unlisted' ? 'var(--accent)' : 'var(--text-muted)'}}>
            {merged.visibility || 'private'}
          </span>}
        </div>
        <div className="chat-item-preview">
          {merged.last_message ? (merged.last_message.deleted ? '(message deleted)' : merged.last_message.author?.username + ': ' + merged.last_message.content) : ''}
        </div>
      </div>
        <div className="chat-item-meta">
          <div className="chat-item-time">{timeAgo(merged.last_message_at)}</div>
          <div className="chat-item-menu-wrap">
            {raw > 0 ? <div className="unread-badge">{unread}</div> : null}
            <button ref={btnRef} className="btn-ghost chat-item-menu-btn" title="More" onClick={handleMenu}>⋮</button>
          </div>
        </div>
    </div>
  );
}
