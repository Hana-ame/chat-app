import React, { useEffect, useRef, useState, useCallback } from 'react'
import MessageItem from './MessageItem'

export default function Chat({ user, messages, connStatus, onlineCount, onSend, onSendFile, onLoadMore, roomName }) {
  const listRef = useRef(null);
  const inputRef = useRef(null);
  const fileRef = useRef(null);
  const dropRef = useRef(null);
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const scrolledUp = useRef(false);

  useEffect(() => {
    const el = listRef.current;
    if (!el) return;
    if (!scrolledUp.current) el.scrollTop = el.scrollHeight;
  }, [messages]);

  const handleScroll = useCallback(() => {
    const el = listRef.current;
    if (!el) return;
    scrolledUp.current = el.scrollHeight - el.scrollTop - el.clientHeight > 60;
    if (el.scrollTop < 200) onLoadMore && onLoadMore();
  }, [onLoadMore]);

  const handleSend = () => {
    const text = inputRef.current?.value.trim();
    if (text) { onSend(text); inputRef.current.value = ''; inputRef.current.style.height = 'auto'; }
  };

  const handleKey = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleSend(); }
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
    const ops = [
      ['dragover', (e) => { e.preventDefault(); setDragOver(true); }],
      ['dragleave', (e) => { e.preventDefault(); setDragOver(false); }],
      ['drop', (e) => { e.preventDefault(); setDragOver(false); handleFiles(e.dataTransfer.files); }],
    ];
    ops.forEach(([ev, fn]) => el.addEventListener(ev, fn));
    return () => ops.forEach(([ev, fn]) => el.removeEventListener(ev, fn));
  }, [handleFiles]);

  const statusMap = { ws: ['在线', 'ch-badge-ws'], poll: ['轮询', 'ch-badge-poll'], offline: ['离线', 'ch-badge-offline'] };
  const [st, sc] = statusMap[connStatus] || ['--', ''];

  return (
    <div className="chat-area" ref={dropRef}>
      {dragOver && <div className="drop-overlay">释放以发送文件</div>}
      {uploading && <div className="upload-toast">上传中…</div>}

      <div className="chat-header">
        <span className="ch-hash">#</span>
        {roomName || '大厅'}
        <span className="ch-count">{onlineCount} 人在线</span>
        <span className={`ch-badge ${sc}`}>{st}</span>
      </div>

      <div className="message-list" ref={listRef} onScroll={handleScroll}>
        {messages.map((msg, i) => (
          <MessageItem
            key={msg.id}
            msg={msg}
            currentUser={user}
            showAvatar={i === 0 || messages[i-1]?.user_id !== msg.user_id || messages[i-1]?.type === 'system'}
          />
        ))}
      </div>

      <div className="chat-input">
        <div className="chat-input-box">
          <input type="file" ref={fileRef} style={{display:'none'}} multiple
                 onChange={e => { handleFiles(e.target.files); e.target.value = ''; }} />
          <button title="上传文件" onClick={() => fileRef.current?.click()}>📎</button>
          <textarea
            ref={inputRef}
            placeholder={`发送消息到 #${roomName || '大厅'}`}
            onKeyDown={handleKey}
            rows={1}
          />
          <button className="send-btn" onClick={handleSend}>➤</button>
        </div>
      </div>
    </div>
  );
}
