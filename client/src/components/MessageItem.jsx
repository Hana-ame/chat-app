import React from 'react';

function formatSize(bytes) {
  if (!bytes) return '';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1048576).toFixed(1) + ' MB';
}

export default function MessageItem({ msg, currentUser, showAvatar }) {
  // System message
  if (msg.msg_type === 'system' || msg.type === 'system') {
    return (
      <div className="msg-row msg-system">
        <span className="msg-system-text">{msg.content}</span>
      </div>
    );
  }

  const isSelf = msg.user_id === currentUser?.user_id;
  const isBot = msg.is_bot;
  const nameColor = msg.avatar_color || '#5865f2';
  const initials = (msg.username || '?')[0].toUpperCase();
  const time = msg.created_at
    ? new Date(msg.created_at).toLocaleTimeString('zh', { hour: '2-digit', minute: '2-digit' })
    : '';

  // File message
  if (msg.msg_type === 'file') {
    let fd = null;
    try { fd = JSON.parse(msg.content); } catch (e) {}
    const fileContent = fd && fd.url ? (
      fd.mime && fd.mime.startsWith('image/')
        ? <a href={fd.url} target="_blank" rel="noopener noreferrer">
            <img src={fd.url} alt={fd.name} className="msg-image" loading="lazy" />
          </a>
        : <a href={fd.url} target="_blank" rel="noopener noreferrer" className="msg-file-link">
            📄 {fd.name} ({formatSize(fd.size)})
          </a>
    ) : msg.content;

    return (
      <div className="msg-row">
        {showAvatar ? <div className="msg-avatar" style={{ background: nameColor }}>{initials}</div> : <div style={{width:40,flexShrink:0}} />}
        <div className="msg-content">
          {showAvatar && (
            <div className="msg-header">
              <span className="msg-username" style={{ color: nameColor }}>{msg.username}</span>
              {isBot && <span className="msg-bot-tag">BOT</span>}
              <span className="msg-timestamp">{time}</span>
            </div>
          )}
          {fileContent}
        </div>
      </div>
    );
  }

  return (
    <div className="msg-row">
      {showAvatar
        ? <div className="msg-avatar" style={{ background: nameColor }}>{initials}</div>
        : <div style={{ width: 40, flexShrink: 0 }} />
      }
      <div className="msg-content">
        {showAvatar && (
          <div className="msg-header">
            <span className="msg-username" style={{ color: nameColor }}>{msg.username}</span>
            {isBot && <span className="msg-bot-tag">BOT</span>}
            <span className="msg-timestamp">{time}</span>
          </div>
        )}
        <div className="msg-text">{msg.content}</div>
      </div>
    </div>
  );
}
