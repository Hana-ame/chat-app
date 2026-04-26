import React, { useEffect, useRef, useState, useCallback } from 'react'
import MessageItem from './MessageItem'

export default function Chat({ user, messages, connStatus, onlineCount, onSend, onSendFile, onLogout }) {
  const listRef = useRef(null);
  const inputRef = useRef(null);
  const fileRef = useRef(null);
  const dropRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);

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

  const handleFiles = useCallback(async (files) => {
    if (!onSendFile || files.length === 0) return;
    setUploading(true);
    for (const file of files) {
      try { await onSendFile(file); } catch (e) { console.error('Upload failed:', e); }
    }
    setUploading(false);
  }, [onSendFile]);

  useEffect(() => {
    const onPaste = (e) => {
      const items = e.clipboardData?.items;
      if (!items) return;
      const files = [];
      for (const item of items) {
        if (item.kind === 'file') files.push(item.getAsFile());
      }
      if (files.length > 0) handleFiles(files);
    };
    document.addEventListener('paste', onPaste);
    return () => document.removeEventListener('paste', onPaste);
  }, [handleFiles]);

  useEffect(() => {
    const el = dropRef.current;
    if (!el) return;
    const onDragOver = (e) => { e.preventDefault(); e.stopPropagation(); setDragOver(true); };
    const onDragLeave = (e) => { e.preventDefault(); e.stopPropagation(); setDragOver(false); };
    const onDrop = (e) => { e.preventDefault(); e.stopPropagation(); setDragOver(false); handleFiles(e.dataTransfer.files); };
    el.addEventListener('dragover', onDragOver);
    el.addEventListener('dragleave', onDragLeave);
    el.addEventListener('drop', onDrop);
    return () => {
      el.removeEventListener('dragover', onDragOver);
      el.removeEventListener('dragleave', onDragLeave);
      el.removeEventListener('drop', onDrop);
    };
  }, [handleFiles]);

  const statusMap = { ws: ['WS', 'status-ws'], poll: ['轮询', 'status-poll'], offline: ['离线', 'status-offline'] };
  const [statusText, statusClass] = statusMap[connStatus] || ['--', ''];

  return (
    <div className="chat-container" ref={dropRef}>
      {dragOver && <div className="drop-overlay">释放以发送文件</div>}
      {uploading && <div className="upload-toast">上传中...</div>}
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
        <input type="file" ref={fileRef} style={{display:'none'}} multiple onChange={e => { handleFiles(e.target.files); e.target.value = ''; }} />
        <button onClick={() => fileRef.current?.click()} title="发送文件" style={{padding:'10px 12px', background:'none', border:'none', color:'#aaa', cursor:'pointer', fontSize:'18px'}}>📎</button>
        <input ref={inputRef} placeholder="输入消息..."
               onKeyUp={e => e.key === 'Enter' && handleSend()} />
        <button onClick={handleSend}>发送</button>
      </div>
    </div>
  );
}