import { useState, useRef, useCallback } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';

export default function Composer({ chatId }) {
  const { accessToken } = useAuthStore();
  const { sendMessage, sendTyping, startStreamingInChat } = useChatStore();
  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [attachments, setAttachments] = useState([]);
  const fileInput = useRef(null);
  const typingTimer = useRef(null);

  const handleTyping = () => {
    sendTyping(chatId);
    if (typingTimer.current) clearTimeout(typingTimer.current);
    typingTimer.current = setTimeout(() => {}, 2000);
  };

  const handleSend = async () => {
    const content = text.trim();
    if (!content && attachments.length === 0) return;
    try {
      await sendMessage(accessToken, chatId, content, attachments);
      setText('');
      setAttachments([]);
    } catch {}
  };

  const handleKey = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
    handleTyping();
  };

  const handleFile = async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    setUploading(true);
    try {
      const results = [];
      for (const f of files) {
        const data = await api.upload(f);
        results.push({ filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { alert(err.message || 'Upload failed'); }
    setUploading(false);
    fileInput.current.value = '';
  };

  return (
    <div className="chat-footer">
      {attachments.length > 0 && (
        <div style={{display:'flex',flexWrap:'wrap',gap:6,marginBottom:8}}>
          {attachments.map((a, i) => (
            <div key={i} className="file-attach" style={{fontSize:12}}>
              {a.filename}
              <button className="btn-ghost" style={{fontSize:14}} onClick={() => setAttachments(a => a.filter((_,j) => j!==i))}>×</button>
            </div>
          ))}
        </div>
      )}
      <div className="chat-input">
        <textarea rows={1} placeholder={'Message #chat'} value={text}
          onChange={e => { setText(e.target.value); handleTyping(); }}
          onKeyDown={handleKey} />
        <div style={{display:'flex',gap:4,marginTop:6,alignItems:'center'}}>
          <input type="file" ref={fileInput} onChange={handleFile} style={{display:'none'}} multiple />
          <button className="btn-ghost" style={{fontSize:18}} onClick={() => fileInput.current?.click()} title="Attach file">📎</button>
          <button className="btn-ghost" style={{fontSize:18}} onClick={() => {
            const input = text.trim() || 'Hello! This is a streaming message that types out slowly. You can see each character appearing one by one. ✨';
            startStreamingInChat(chatId, async (emit) => {
              for (const char of input) {
                await new Promise(r => setTimeout(r, 40));
                emit(char);
              }
            });
          }} title="AI stream test">🤖</button>
          <button className="btn-ghost" style={{fontSize:13}} disabled={(!text.trim() && attachments.length === 0) || uploading}
            onClick={handleSend}>{uploading ? 'Uploading...' : 'Send'}</button>
        </div>
      </div>
    </div>
  );
}
