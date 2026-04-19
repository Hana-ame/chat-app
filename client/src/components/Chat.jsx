import React, { useEffect, useRef } from 'react'
import MessageItem from './MessageItem'

export default function Chat({ user, messages, connStatus, onlineCount, onSend, onLogout }) {
  const listRef = useRef(null);
  const inputRef = useRef(null);

  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSend = () => {
    if (inputRef.current?.value.trim()) {
      onSend(inputRef.current.value);
      inputRef.current.value = '';
    }
  };

  const statusMap = { ws: ['WS', 'status-ws'], poll: ['轮询', 'status-poll'], offline: ['离线', 'status-offline'] };
  const [statusText, statusClass] = statusMap[connStatus] || ['--', ''];

  return (
    <div className="chat-container">
      <div className="chat-header">
        <span>💬 大厅</span>
        <span style={{fontSize: '12px', color: '#aaa'}}>{onlineCount} 人在线</span>
        <span className={`status-badge ${statusClass}`}>{statusText}</span>
        <button onClick={onLogout} style={{background:'none', border:'none', color:'#e74c3c', cursor:'pointer'}}>退出</button>
      </div>
      
      <div className="message-list" ref={listRef}>
        {messages.map(msg => <MessageItem key={msg.id} msg={msg} currentUser={user} />)}
      </div>

      <div className="chat-input">
        <input ref={inputRef} placeholder="输入消息..." 
               onKeyUp={e => e.key === 'Enter' && handleSend()} />
        <button onClick={handleSend}>发送</button>
      </div>
    </div>
  );
}