import React from 'react';

function formatSize(bytes) {
  if (!bytes) return '';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1048576).toFixed(1) + ' MB';
}

export default function MessageItem({ msg, currentUser }) {
  if (msg.msg_type === 'system' || msg.type === 'system') {
    return <div className="msg-bubble system">{msg.content}</div>;
  }

  const isSelf = msg.user_id === currentUser?.user_id;
  const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString('zh', {hour:'2-digit', minute:'2-digit'}) : '';
  const isBot = msg.is_bot;

  if (msg.msg_type === 'file') {
    let fileData = null;
    try { fileData = JSON.parse(msg.content); } catch (e) {}
    if (!fileData || !fileData.url) {
      return <div className={`msg-bubble ${isSelf ? 'self' : 'other'}`}>{msg.content} <span className="msg-time">{time}</span></div>;
    }
    const isImage = fileData.mime && fileData.mime.startsWith('image/');
    return (
      <div className={`msg-bubble ${isSelf ? 'self' : 'other'} ${isBot ? 'bot' : ''}`}>
        {!isSelf && (
          <div className="msg-author" style={{color: msg.avatar_color}}>
            {isBot && <span className="bot-badge">{'\uD83E\uDD16'}</span>}
            {msg.username}
          </div>
        )}
        <div>
          {isImage ? (
            <a href={fileData.url} target="_blank" rel="noopener noreferrer">
              <img src={fileData.url} alt={fileData.name} className="msg-image" loading="lazy" />
            </a>
          ) : (
            <a href={fileData.url} target="_blank" rel="noopener noreferrer" className="msg-file-link">
              {'\uD83D\uDCCE'} {fileData.name} ({formatSize(fileData.size)})
            </a>
          )}
          <span className="msg-time">{time}</span>
        </div>
      </div>
    );
  }

  return (
    <div className={`msg-bubble ${isSelf ? 'self' : 'other'} ${isBot ? 'bot' : ''}`}>
      {!isSelf && (
        <div className="msg-author" style={{color: msg.avatar_color}}>
          {isBot && <span className="bot-badge">{'\uD83E\uDD16'}</span>}
          {msg.username}
        </div>
      )}
      <div>
        {msg.content}
        <span className="msg-time">{time}</span>
      </div>
    </div>
  );
}