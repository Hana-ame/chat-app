import { useState, useRef, useCallback, useEffect } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';

export default function Composer({ chatId }) {
  const { accessToken } = useAuthStore();
  const { sendMessage, sendTyping } = useChatStore();
  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [attachments, setAttachments] = useState([]);
  const fileInput = useRef(null);
  const typingTimer = useRef(null);
  const textRef = useRef(null);

  const autoResize = useCallback(() => {
    const el = textRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = el.scrollHeight + 'px';
  }, []);

  useEffect(() => { autoResize(); }, [text, autoResize]);

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
    } catch (e) { console.error('Send message error:', e); }
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
        <div style={{display:'flex',gap:6,alignItems:'stretch'}}>
          <textarea rows={1} placeholder={'Message #chat'} value={text}
            ref={textRef}
            onChange={e => { setText(e.target.value); handleTyping(); autoResize(); }}
            onKeyDown={handleKey}
            style={{flex:1,resize:'none',overflow:'hidden',minHeight:36}} />
          <button className="btn btn-primary" style={{whiteSpace:'nowrap',padding:'4px 14px'}}
            disabled={(!text.trim() && attachments.length === 0) || uploading}
            onClick={handleSend}>{uploading ? '...' : 'Send'}</button>
        </div>
        <div style={{display:'flex',gap:4,marginTop:6,alignItems:'center'}}>
          <input type="file" ref={fileInput} onChange={handleFile} style={{display:'none'}} multiple />
          <button className="btn-ghost" style={{fontSize:18}} onClick={() => fileInput.current?.click()} title="Attach file">📎</button>
          <button className="btn-ghost" style={{fontSize:18}} onClick={() => {
            if (!api.isMockEnabled()) { alert('Enable mock first (🧪 Generate test data)'); return; }
            const content = text.trim() || 'Tell me something interesting';
            sendMessage(accessToken, chatId, content, []).catch(console.error);
          }} title="Send to AI (mock only)">🤖</button>
        </div>
      </div>
    </div>
  );
}
