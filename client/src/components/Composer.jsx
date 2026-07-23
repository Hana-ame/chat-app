import { useState, useRef, useCallback, useEffect } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import { streamAI } from '../utils/ai';

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

const CONTEXT_LIMIT = 40;

export default function Composer({ chatId }) {
  const { user, accessToken } = useAuthStore();
  const { sendMessage, sendTyping } = useChatStore();
  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [attachments, setAttachments] = useState([]);
  const [aiMode, setAiMode] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiSource, setAiSource] = useState('default');
  const [aiModel, setAiModel] = useState('');
  const aiAbort = useRef(null);
  const fileInput = useRef(null);
  const typingTimer = useRef(null);
  const textRef = useRef(null);
  const [mentionQuery, setMentionQuery] = useState(null);
  const [mentionIdx, setMentionIdx] = useState(0);

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

  const mentionMembers = (() => {
    if (!mentionQuery) return [];
    const chat = useChatStore.getState().chats.find(c => c.id === chatId);
    if (!chat?.members) return [];
    const q = mentionQuery.toLowerCase();
    return chat.members.filter(m => m.id !== user.id && m.username.toLowerCase().includes(q)).slice(0, 10);
  })();

  const handleMentionSelect = (m) => {
    const el = textRef.current;
    if (!el) return;
    const pos = el.selectionStart;
    const before = text.slice(0, pos);
    const after = text.slice(pos);
    const atIdx = before.lastIndexOf('@');
    if (atIdx === -1) return;
    const newText = before.slice(0, atIdx) + `<@${m.id}> ` + after;
    setText(newText);
    setMentionQuery(null);
    requestAnimationFrame(() => {
      el.focus();
      el.selectionStart = el.selectionEnd = atIdx + `<@${m.id}> `.length;
    });
  };

  const handleTextChange = (e) => {
    const val = e.target.value;
    setText(val);
    handleTyping();
    autoResize();
    const el = textRef.current;
    if (!el) return;
    const pos = el.selectionStart;
    const before = val.slice(0, pos);
    const atIdx = before.lastIndexOf('@');
    if (atIdx >= 0 && (atIdx === 0 || before[atIdx - 1] === ' ' || before[atIdx - 1] === '\n')) {
      const q = before.slice(atIdx + 1);
      if (q && !q.includes(' ') && !q.includes('\n') && !q.includes('<')) {
        setMentionQuery(q);
        setMentionIdx(0);
        return;
      }
    }
    setMentionQuery(null);
  };

  const cancelAI = () => {
    if (aiAbort.current) {
      aiAbort.current.abort();
      aiAbort.current = null;
    }
  };

  const buildContext = (msgs) => {
    const context = [];
    for (const m of msgs) {
      if (context.length >= CONTEXT_LIMIT) break;
      if (m.user_id === 'ai') {
        context.push({ role: 'assistant', content: m.content });
      } else if (m.user_id && m.content) {
        context.push({ role: 'user', content: m.content });
      }
    }
    return context;
  };

  const handleAISend = async (content) => {
    const msgId = crypto.randomUUID();
    const botMsg = {
      id: msgId,
      chat_id: chatId,
      user_id: 'ai',
      content: '',
      created_at: new Date().toISOString(),
      streaming: true,
      author: { id: 'ai', username: 'AI', avatar_color: '#10a37f' },
    };
    useChatStore.getState().onMessageCreate(botMsg);
    setAiLoading(true);

    const store = useChatStore.getState();
    const messages = store.messages;
    const context = content ? [...buildContext(messages), { role: 'user', content }] : [];

    const controller = new AbortController();
    aiAbort.current = controller;

    const body = {
      source: aiSource,
      chat_id: chatId,
      msg_id: msgId,
      messages: context,
      stream: true,
      temperature: 0.7,
      max_tokens: 32768,
      top_p: 1,
    };
    if (aiModel.trim()) body.model = aiModel.trim();

    try {
      const res = await api.aiChat(accessToken, body);
      const { cancel } = streamAI(res,
        (chunk) => {
          if (chunk.type === 'content') {
            useChatStore.setState(s => ({
              messages: s.messages.map(m =>
                m.id === msgId ? { ...m, content: m.content + chunk.content } : m
              ),
            }));
          }
        },
        () => {
          useChatStore.setState(s => ({
            messages: s.messages.map(m =>
              m.id === msgId ? { ...m, streaming: false } : m
            ),
          }));
          setAiLoading(false);
          aiAbort.current = null;
        },
        () => {
          notify('AI response failed', 'error');
          useChatStore.setState(s => ({
            messages: s.messages.map(m =>
              m.id === msgId ? { ...m, streaming: false } : m
            ),
          }));
          setAiLoading(false);
          aiAbort.current = null;
        },
      );
      controller.signal.addEventListener('abort', () => {
        cancel();
        useChatStore.setState(s => ({
          messages: s.messages.map(m =>
            m.id === msgId ? { ...m, streaming: false } : m
          ),
        }));
        setAiLoading(false);
        aiAbort.current = null;
      });
    } catch (e) {
      if (e.name === 'AbortError') return;
      notify('AI request failed', 'error');
      useChatStore.setState(s => ({
        messages: s.messages.map(m =>
          m.id === msgId ? { ...m, streaming: false } : m
        ),
      }));
      setAiLoading(false);
      aiAbort.current = null;
    }
  };

  const handleSend = async () => {
    const content = text.trim();
    if (!content && attachments.length === 0) return;
    try {
      await sendMessage(accessToken, chatId, content, attachments);
      setText('');
      setAttachments([]);
      if (aiMode && content) {
        await handleAISend(content);
      }
    } catch (e) { notify('Failed to send message', 'error'); }
  };

  const handleKey = (e) => {
    const members = mentionMembers;
    if (members.length > 0) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setMentionIdx(i => Math.min(i + 1, members.length - 1)); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); setMentionIdx(i => Math.max(i - 1, 0)); return; }
      if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); handleMentionSelect(members[mentionIdx]); return; }
      if (e.key === 'Escape') { setMentionQuery(null); return; }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
    handleTyping();
  };

  const uploadPastedImages = async (files) => {
    setUploading(true);
    try {
      const results = [];
      for (const f of files) {
        const file = await compressImage(f);
        const data = await api.upload(file);
        results.push({ _key: crypto.randomUUID(), filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { notify('Upload failed', 'error'); }
    setUploading(false);
  };

  const handleFile = async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    setUploading(true);
    try {
      const results = [];
      for (const f of files) {
        const data = await api.upload(f);
        results.push({ _key: crypto.randomUUID(), filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { notify('Upload failed', 'error'); }
    setUploading(false);
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
      await uploadPastedImages(imageFiles);
    }
  };

  return (
    <div className="chat-footer">
      {attachments.length > 0 && (
        <div style={{display:'flex',flexWrap:'wrap',gap:6,marginBottom:8}}>
          {attachments.map((a, i) => (
            <div key={a._key} className="file-attach" style={{position:'relative',fontSize:12,display:'flex',alignItems:'center',gap:4}}>
              {a.mime_type?.startsWith('image/')
                ? <img src={a.url} alt="" style={{maxWidth:120,maxHeight:80,borderRadius:4}} />
                : <span style={{overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap',maxWidth:120}}>{a.filename}</span>}
              <button className="btn-ghost" style={{fontSize:14,lineHeight:1,width:18,height:18,borderRadius:'50%',padding:0,flexShrink:0}} onClick={() => setAttachments(a => a.filter((_,j) => j!==i))}>×</button>
            </div>
          ))}
        </div>
      )}
      <div className="chat-input">
        <div style={{display:'flex',gap:6,alignItems:'stretch',position:'relative'}}>
          {mentionQuery !== null && mentionMembers.length > 0 && (
            <div className="mention-dropdown">
              {mentionMembers.map((m, i) => (
                <div key={m.id} className={'mention-item' + (i === mentionIdx ? ' active' : '')}
                  onMouseDown={e => { e.preventDefault(); handleMentionSelect(m); }}
                  onMouseEnter={() => setMentionIdx(i)}>
                  {m.username}
                </div>
              ))}
            </div>
          )}
          <textarea rows={1} placeholder={aiMode ? 'Ask AI...' : 'Message #chat'} value={text}
            ref={textRef}
            onChange={handleTextChange}
            onKeyDown={handleKey}
            onPaste={handlePaste}
            style={{flex:1,resize:'none',minHeight:36}} />
          <input type="file" ref={fileInput} onChange={handleFile} style={{display:'none'}} multiple />
          <button className="btn-ghost" style={{fontSize:18,padding:'4px 6px',lineHeight:0}} onClick={() => fileInput.current?.click()} title="Attach file">📎</button>
          <div style={{position:'relative'}}>
            <button className={'btn-ghost' + (aiMode ? ' active' : '')} style={{fontSize:13,padding:'4px 6px',lineHeight:0,fontWeight:aiMode?600:400,color:aiMode?'var(--accent)':'var(--text-muted)'}}
              onClick={() => setAiMode(!aiMode)} title="Toggle AI mode"
              disabled={aiLoading}>
              AI{aiMode ? ' ▼' : ''}
            </button>
            {aiMode && (
              <div style={{position:'absolute',bottom:'100%',left:0,marginBottom:4,display:'flex',flexDirection:'column',gap:4,background:'var(--bg-tertiary)',padding:6,borderRadius:6,border:'1px solid var(--border)',fontSize:12,whiteSpace:'nowrap'}}>
                <label>source:
                  <input value={aiSource} onChange={e => setAiSource(e.target.value)}
                    style={{marginLeft:6,width:120,fontSize:12,padding:'1px 4px'}} />
                </label>
                <label>model:
                  <input value={aiModel} onChange={e => setAiModel(e.target.value)}
                    placeholder="(default)" style={{marginLeft:6,width:200,fontSize:12,padding:'1px 4px'}} />
                </label>
              </div>
            )}
          </div>
          {aiLoading ? (
            <button className="btn-ghost" style={{padding:'4px 10px',lineHeight:0,color:'var(--danger)'}}
              onClick={cancelAI} title="Cancel">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          ) : (
            <button className="btn-ghost" style={{padding:'4px 10px',lineHeight:0}}
              disabled={(!text.trim() && attachments.length === 0) || uploading}
              onClick={handleSend} title={aiMode ? 'Send + AI reply' : 'Send'}>
              {uploading ? <span style={{fontSize:14}}>...</span> : (
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="5" y1="12" x2="19" y2="12"/>
                  <polyline points="12 5 19 12 12 19"/>
                </svg>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
