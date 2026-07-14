import { useState, useRef, useCallback, useEffect } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';

function compressImage(file) {
  return new Promise((resolve) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement('canvas');
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext('2d');
      ctx.drawImage(img, 0, 0);
      canvas.toBlob((blob) => {
        const name = file.name.replace(/\.[^.]+$/, '') + '.webp';
        resolve(new File([blob], name, { type: 'image/webp' }));
      }, 'image/webp', 0.75);
    };
    img.src = URL.createObjectURL(file);
  });
}

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

  const uploadFiles = async (files) => {
    setUploading(true);
    try {
      const results = [];
      for (const f of files) {
        const file = f.type?.startsWith('image/') ? await compressImage(f) : f;
        const data = await api.upload(file);
        results.push({ filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { alert(err.message || 'Upload failed'); }
    setUploading(false);
  };

  const handleFile = async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    await uploadFiles(files);
    fileInput.current.value = '';
  };

  const handlePaste = async (e) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    const imageFiles = [];
    for (const item of items) {
      if (item.type?.startsWith('image/')) {
        const file = item.getAsFile();
        if (file) imageFiles.push(file);
      }
    }
    if (imageFiles.length) {
      e.preventDefault();
      await uploadFiles(imageFiles);
    }
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
            onPaste={handlePaste}
            style={{flex:1,resize:'none',overflow:'hidden',minHeight:36}} />
          <button className="btn-ghost" style={{padding:'4px 10px',lineHeight:0}}
            disabled={(!text.trim() && attachments.length === 0) || uploading}
            onClick={handleSend} title="Send">
            {uploading ? <span style={{fontSize:14}}>...</span> : (
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="5" y1="12" x2="19" y2="12"/>
                <polyline points="12 5 19 12 12 19"/>
              </svg>
            )}
          </button>
        </div>
        <div style={{display:'flex',gap:4,marginTop:6,alignItems:'center'}}>
          <input type="file" ref={fileInput} onChange={handleFile} style={{display:'none'}} multiple />
          <button className="btn-ghost" style={{fontSize:18}} onClick={() => fileInput.current?.click()} title="Attach file">📎</button>
        </div>
      </div>
    </div>
  );
}
