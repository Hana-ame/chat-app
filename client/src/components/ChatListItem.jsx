import { useChatStore } from '../store/chat';
import { api } from '../api/client';
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

function getDMName(chat, currentUserId) {
  const other = chat.members?.find(m => m.id !== currentUserId);
  return other ? other.username : 'Unknown';
}

export default function ChatListItem({ chat, activeId, onSelectChat, contextMenu, onContextMenu }) {
  const { user, accessToken } = useAuthStore();
  const { pinChat, unpinChat } = useChatStore();

  const name = chat.type === 'dm' ? getDMName(chat, user.id) : chat.name;
  const avatar = chat.type === 'dm' ? (chat.members?.find(m => m.id !== user.id)?.avatar_color || chat.icon_color) : chat.icon_color;
  const unread = chat.unread_count || 0;

  const handlePin = async (e, chatId, pinned) => {
    e.stopPropagation();
    try { if (pinned) await unpinChat(accessToken, chatId); else await pinChat(accessToken, chatId); } catch (e) { console.error('Pin chat error:', e); }
    onContextMenu(null);
  };

  const handleDelete = async (e, chatId) => {
    e.stopPropagation();
    if (!confirm('Delete this chat?')) return;
    try { await api.deleteChat(accessToken, chatId); } catch (e) { console.error('Delete chat error:', e); }
    onContextMenu(null);
  };

  return (
    <div key={chat.id} className={'chat-item' + (chat.id === activeId ? ' active' : '') + (chat.pinned ? ' pinned' : '') + (chat.visibility === 'public' ? ' public' : '')}
      onClick={() => onSelectChat(chat.id)}>
      <div className="chat-item-avatar" style={{ background: avatar }}>
        {name ? name[0].toUpperCase() : '?'}
      </div>
      <div className="chat-item-info">
        <div className="chat-item-name">{name || getDMName(chat, user.id)}</div>
        <div className="chat-item-preview">
          {chat.last_message ? (chat.last_message.deleted ? '(message deleted)' : chat.last_message.author?.username + ': ' + chat.last_message.content) : ''}
        </div>
      </div>
      <div className="chat-item-meta">
        <div className="chat-item-time">{timeAgo(chat.last_message_at)}</div>
        {unread > 0 ? <div className="unread-badge">{unread}</div> : <div style={{ height: 18 }} />}
        <div className="chat-item-menu-wrap">
          <button className="btn-ghost chat-item-menu-btn" title="More"
            onClick={(e) => { e.stopPropagation(); onContextMenu(chat.id); }}>⋮</button>
          {contextMenu === chat.id && (
            <div className="context-menu">
              {chat.pinned
                ? <button className="context-menu-item" onClick={(e) => handlePin(e, chat.id, true)}>Unpin</button>
                : <button className="context-menu-item" onClick={(e) => handlePin(e, chat.id, false)}>Pin</button>}
              {chat.owner_id === user.id && (
                <button className="context-menu-item danger" onClick={(e) => handleDelete(e, chat.id)}>Delete</button>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
