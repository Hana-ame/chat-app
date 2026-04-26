import React from 'react';

export default function MessageItem({ msg, currentUser }) {
  if (msg.msg_type === 'system' || msg.type === 'system') {
    return <div className="msg-bubble system">{msg.content}</div>;
  }

  const isSelf = msg.user_id === currentUser?.user_id;
  const time = msg.created_at ? new Date(msg.created_at).toLocaleTimeString('zh', {hour:'2-digit', minute:'2-digit'}) : '';
  const isBot = msg.is_bot;

  return (
    <div className={`msg-bubble ${isSelf ? 'self' : 'other'} ${isBot ? 'bot' : ''}`}>
      {!isSelf && (
        <div className="msg-author" style={{color: msg.avatar_color}}>
          {isBot && <span className="bot-badge">&#x1f916;</span>}
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