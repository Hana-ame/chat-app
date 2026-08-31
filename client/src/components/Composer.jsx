import { useState, useRef, useCallback, useEffect } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import { streamAI } from '../utils/ai';
import { extractFirstUrl, fetchOgp } from '../utils/linkPreview';
import LinkPreviewCard from './LinkPreviewCard';

const CONTEXT_LIMIT = 50;
const STORAGE_KEY = 'ai_settings';

function buildContext(msgs, limit) {
  const context = [];
  for (let i = msgs.length - 1; i >= 0; i--) {
    const m = msgs[i];
    if (context.length >= limit) break;
    if (m.type === 'stream' || m.user_id === 'ai') {
      context.unshift({ role: 'assistant', content: m.content });
    } else if (m.user_id && m.content) {
      context.unshift({ role: 'user', content: m.content });
    }
  }
  return context;
}

function loadSettings() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return null;
}

function saveSettings(s) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(s)); } catch {}
}

function toJsonBody(s) {
  return JSON.stringify({
    model: s.model || 'deepseek-ai/Deepseek-V4-Flash-free',
    messages: [{ role: 'user', content: '' }],
    temperature: parseFloat(s.temperature) || 0.7,
    max_tokens: parseInt(s.maxTokens) || 32768,
    top_p: parseFloat(s.topP) || 1,
  }, null, 2);
}

const defaultSettings = {
  endpoint: import.meta.env.VITE_AI_ENDPOINT || 'https://api.siliconflow.cn/v1/chat/completions',
  authKey: import.meta.env.VITE_AI_AUTH_KEY || '',
  model: import.meta.env.VITE_AI_MODEL || 'deepseek-ai/Deepseek-V4-Flash',
  temperature: '0.7',
  maxTokens: '32768',
  topP: '1',
  contextLimit: 50,
  mode: 'basic',
  jsonBody: '',
};

defaultSettings.jsonBody = toJsonBody(defaultSettings);

function compressImage(file) {
  return new Promise((resolve, reject) => {
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
    img.onerror = () => reject(new Error('Failed to load image'));
    img.src = URL.createObjectURL(file);
  });
}

export default function Composer({ chatId, isNotification, replyTo, onCancelReply }) {
  const { user, accessToken } = useAuthStore();
  const { sendMessage, sendTyping } = useChatStore();
  const [text, setText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [attachments, setAttachments] = useState([]);
  const [aiActive, setAiActive] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);
  const [sending, setSending] = useState(false);
  const aiAbort = useRef(null);
  const fileInput = useRef(null);
  const typingTimer = useRef(null);
  const textRef = useRef(null);
  const [mentionQuery, setMentionQuery] = useState(null);
  const [mentionIdx, setMentionIdx] = useState(0);

  // 【本地改动 2026-09-03】链接预览（轻量版，纯前端 OGP）。
  // linkPreview = {status:'loading'|'ok'|'fail', url, meta}; dismissedUrls 记忆本会话关闭的 URL。
  const [linkPreview, setLinkPreview] = useState(null);
  const dismissedUrls = useRef(new Set());
  const previewAbort = useRef(null);

  // 检测文本中的首个 URL 并抓 OGP；URL 变化 / 已关闭时不重复抓。
  useEffect(() => {
    const url = extractFirstUrl(text);
    if (!url || dismissedUrls.current.has(url)) {
      setLinkPreview(null);
      return;
    }
    if (previewAbort.current) previewAbort.current.abort();
    const ac = new AbortController();
    previewAbort.current = ac;
    setLinkPreview({ status: 'loading', url, meta: null });
    fetchOgp(url, { signal: ac.signal }).then(r => {
      if (ac.signal.aborted) return;
      setLinkPreview(r.ok ? { status: 'ok', url, meta: r } : { status: 'fail', url, meta: null });
    });
    return () => ac.abort();
  }, [text]);

  const handleDismissPreview = useCallback(() => {
    if (linkPreview) dismissedUrls.current.add(linkPreview.url);
    setLinkPreview(null);
  }, [linkPreview]);

  const saved = useRef((() => {
    const s = loadSettings() || { ...defaultSettings };
    if (s.sendContext !== undefined && s.contextLimit === undefined) {
      s.contextLimit = s.sendContext ? 50 : 0;
    }
    delete s.sendContext;
    return s;
  })());
  const [settings, setSettings] = useState({ ...saved.current });

  const setField = useCallback((k, v) => {
    setSettings(prev => {
      let next = { ...prev, [k]: v };
      if (['model', 'temperature', 'maxTokens', 'topP'].includes(k)) {
        next.jsonBody = toJsonBody(next);
      }
      if (k === 'jsonBody') {
        try {
          const obj = JSON.parse(v);
          if (obj.model && typeof obj.model === 'string') next.model = obj.model;
          if (obj.temperature != null) next.temperature = String(obj.temperature);
          if (obj.max_tokens != null) next.maxTokens = String(obj.max_tokens);
          if (obj.top_p != null) next.topP = String(obj.top_p);
        } catch {}
      }
      saved.current = { ...saved.current, ...next };
      saveSettings(saved.current);
      return next;
    });
  }, []);

  const setMode = useCallback((mode) => {
    setSettings(prev => {
      let next = { ...prev, mode };
      if (mode === 'json') {
        next.jsonBody = toJsonBody(next);
      } else {
        try {
          const obj = JSON.parse(prev.jsonBody);
          if (obj.model && typeof obj.model === 'string') next.model = obj.model;
          if (obj.temperature != null) next.temperature = String(obj.temperature);
          if (obj.max_tokens != null) next.maxTokens = String(obj.max_tokens);
          if (obj.top_p != null) next.topP = String(obj.top_p);
        } catch {}
      }
      saved.current = { ...saved.current, ...next };
      saveSettings(saved.current);
      return next;
    });
  }, []);

  const autoResize = useCallback(() => {
    const el = textRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = el.scrollHeight + 'px';
  }, []);

  useEffect(() => { autoResize(); }, [text, autoResize]);

  const handleTyping = () => {
    if (typingTimer.current) return;
    sendTyping(chatId);
    typingTimer.current = setTimeout(() => { typingTimer.current = null; }, 2000);
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

  const cancelAI = useCallback(() => {
    if (aiAbort.current) {
      aiAbort.current.abort();
      aiAbort.current = null;
    }
  }, []);

  const doSendAI = useCallback(async (content) => {
    if (!content || isNotification) return;
    const msgId = crypto.randomUUID();
    setAiLoading(true);

    const f = saved.current;
    const store = useChatStore.getState();
    const messages = store.messages;
    const context = [...buildContext(messages, f.contextLimit || 0), { role: 'user', content }];

    const controller = new AbortController();
    aiAbort.current = controller;
    let body;
    if (f.mode === 'json') {
      try { body = JSON.parse(f.jsonBody); } catch (e) {
        notify('Invalid JSON body: ' + (e.message || 'Unknown parse error'), 'error');
        useChatStore.setState(s => ({
          messages: s.messages.map(m => m.id === msgId ? { ...m, streaming: false } : m),
        }));
        setAiLoading(false);
        return;
      }
    } else {
      body = {
        model: (f.model || '').trim() || undefined,
        messages: f.contextLimit ? context : [{ role: 'user', content }],
        temperature: parseFloat(f.temperature) || 0.7,
        max_tokens: parseInt(f.maxTokens) || 32768,
        top_p: parseFloat(f.topP) || 1,
      };
    }

    const source = { endpoint: f.endpoint, auth_key: f.authKey, body };
    useChatStore.setState(s => ({ _localStreaming: { ...s._localStreaming, [msgId]: true } }));

    const clearLocalStreaming = () => {
      useChatStore.setState(s => {
        const next = { ...s._localStreaming };
        delete next[msgId];
        return { _localStreaming: next };
      });
    };
    const done = () => {
      useChatStore.setState(s => ({
        messages: s.messages.map(m => m.id === msgId ? { ...m, streaming: false } : m),
      }));
      clearLocalStreaming();
      setAiLoading(false);
      aiAbort.current = null;
    };

    try {
      const res = await api.sendStreamMessage(accessToken, chatId, '', source, msgId);
      const { cancel } = streamAI(res,
        (chunk) => {
          if (chunk.type === 'content') {
            useChatStore.setState(s => ({
              messages: s.messages.map(m => m.id === msgId ? { ...m, content: m.content + chunk.content } : m),
            }));
          } else if (chunk.type === 'thinking') {
            useChatStore.setState(s => ({
              messages: s.messages.map(m => m.id === msgId ? { ...m, thinking: (m.thinking || '') + chunk.content } : m),
            }));
          }
        },
        done,
        (err) => {
          notify(err?.message ? 'AI response failed: ' + err.message : 'AI response failed', 'error');
          done();
        },
      );
      controller.signal.addEventListener('abort', () => {
        cancel();
        done();
      });
    } catch (e) {
      if (e.name === 'AbortError') return;
      notify('AI request failed: ' + (e.message || e.statusText || e.error || 'Unknown error'), 'error');
      done();
    }
  }, [chatId, user, accessToken]);

  const handleSend = async () => {
    if (sending) return;
    setSending(true);
    const content = text.trim();
    const savedText = text;
    const savedAttachments = attachments;
    if (!content && savedAttachments.length === 0) { setSending(false); return; }
    try {
      if (isNotification) {
        await api.notifications.sendMessage(accessToken, content, savedAttachments);
      } else {
        await sendMessage(accessToken, chatId, content, savedAttachments, replyTo?.id || '');
      }
      setText('');
      if (onCancelReply) onCancelReply();
      setAttachments([]);
      // 【本地改动 2026-09-03】发送后清空链接预览（新合成会话）。
      setLinkPreview(null);
      dismissedUrls.current.clear();
      if (aiActive && content) {
        await doSendAI(content);
      }
    } catch (e) {
      setText(prev => prev || savedText);
      setAttachments(prev => prev.length > 0 ? prev : savedAttachments);
      notify('Failed to send message: ' + (e.message || e.statusText || e.error || 'Unknown error'), 'error');
    } finally {
      setSending(false);
    }
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
        const data = await api.upload(file, accessToken);
        results.push({ _key: crypto.randomUUID(), filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { notify('Upload failed: ' + (err.message || err.statusText || 'Unknown error'), 'error'); }
    setUploading(false);
  };

  const handleFile = async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    setUploading(true);
    try {
      const results = [];
      for (const f of files) {
        const data = await api.upload(f, accessToken);
        results.push({ _key: crypto.randomUUID(), filename: data.filename, mime_type: data.mime_type, size: data.size, url: data.url });
      }
      setAttachments(prev => [...prev, ...results]);
    } catch (err) { notify('Upload failed: ' + (err.message || err.statusText || 'Unknown error'), 'error'); }
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

  /** @type {import('react').CSSProperties} */
  const inputStyle = {
    width: '100%', padding: '4px 6px', fontSize: 13, fontFamily: 'monospace',
    background: 'var(--bg-secondary)', border: '1px solid var(--border)',
    borderRadius: 4, color: 'var(--text-primary)',
    outline: 'none', boxSizing: 'border-box',
  };
  const labelStyle = { fontSize: 13, color: 'var(--text-muted)', whiteSpace: 'nowrap' };

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
      {replyTo && (
        <div className="reply-preview" style={{display:'flex',alignItems:'center',gap:8,padding:'4px 8px',marginBottom:4,background:'var(--bg-secondary)',borderRadius:4,borderLeft:'3px solid var(--accent)',fontSize:12}}>
          <span style={{fontWeight:600,color:'var(--accent)',whiteSpace:'nowrap'}}>Replying to {replyTo.author?.username || 'Unknown'}</span>
          <span style={{color:'var(--text-muted)',flex:1,overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap'}}>{replyTo.content}</span>
          <button className="btn-ghost" style={{fontSize:14,lineHeight:1,padding:'2px 6px'}} onClick={onCancelReply}>×</button>
        </div>
      )}
      <div className="chat-input">
        <div style={{display:'flex',flexDirection:'column',gap:4,position:'relative'}}>
          {linkPreview && (
            <LinkPreviewCard url={linkPreview.url} status={linkPreview.status} meta={linkPreview.meta} onDismiss={handleDismissPreview} />
          )}
          {aiActive && (
            <div style={{display:'grid',gridTemplateColumns:'auto 1fr',gap:'4px 8px',alignItems:'center'}}>
              <span style={labelStyle}>Endpoint</span>
              <input data-testid="ai-endpoint" style={inputStyle} value={settings.endpoint} onChange={e => setField('endpoint', e.target.value)} spellCheck={false} />
              <span style={labelStyle}>Key</span>
              <input data-testid="ai-key" style={inputStyle} value={settings.authKey} onChange={e => setField('authKey', e.target.value)} type="password" spellCheck={false} />
              <div style={{gridColumn:'1/-1',display:'flex',gap:6}}>
                <button data-testid="ai-mode-basic" className="btn-ghost" style={{fontSize:13,padding:'2px 6px',fontWeight:settings.mode==='basic'?600:400,color:settings.mode==='basic'?'var(--accent)':'var(--text-muted)'}}
                  onClick={() => setMode('basic')}>Basic</button>
                <button data-testid="ai-mode-json" className="btn-ghost" style={{fontSize:13,padding:'2px 6px',fontWeight:settings.mode==='json'?600:400,color:settings.mode==='json'?'var(--accent)':'var(--text-muted)'}}
                  onClick={() => setMode('json')}>JSON</button>
              </div>
              {settings.mode==='basic' ? (
                <>
                  <span style={labelStyle}>Model</span>
                  <input data-testid="ai-model" style={inputStyle} value={settings.model} onChange={e => setField('model', e.target.value)} spellCheck={false} />
                  <div style={{gridColumn:'1/-1',display:'flex',gap:6,flexWrap:'wrap'}}>
                    <label style={{...labelStyle,display:'flex',alignItems:'center',gap:3}}>
                      Temperature
                      <input data-testid="ai-temperature" style={{width:60,padding:'3px 5px',fontSize:13,background:'var(--bg-primary)',border:'1px solid var(--border)',borderRadius:4,color:'var(--text-primary)'}}
                        value={settings.temperature} onChange={e => setField('temperature',e.target.value)} type="number" step="0.1" min="0" max="2" />
                    </label>
                    <label style={{...labelStyle,display:'flex',alignItems:'center',gap:3}}>
                      Top P
                      <input data-testid="ai-top-p" style={{width:60,padding:'3px 5px',fontSize:13,background:'var(--bg-primary)',border:'1px solid var(--border)',borderRadius:4,color:'var(--text-primary)'}}
                        value={settings.topP} onChange={e => setField('topP',e.target.value)} type="number" step="0.05" min="0" max="1" />
                    </label>
                    <label style={{...labelStyle,display:'flex',alignItems:'center',gap:3}}>
                      Max Tokens
                      <input data-testid="ai-max-tokens" style={{width:70,padding:'3px 5px',fontSize:13,background:'var(--bg-primary)',border:'1px solid var(--border)',borderRadius:4,color:'var(--text-primary)'}}
                        value={settings.maxTokens} onChange={e => setField('maxTokens',e.target.value)} type="number" step="1" min="1" />
                    </label>
                    <label style={{...labelStyle,display:'flex',alignItems:'center',gap:6}}>
                      <input data-testid="ai-context-limit" type="range" min="0" max="100" step="1"
                        value={settings.contextLimit}
                        onChange={e => setField('contextLimit', Number(e.target.value))}
                        style={{width:80,height:4,accentColor:'var(--accent)',cursor:'pointer'}} />
                      <span style={{fontSize:12,whiteSpace:'nowrap',color:'var(--text-muted)'}}>
                        {settings.contextLimit ? `最近${settings.contextLimit}条` : '不发送'}
                      </span>
                    </label>
                  </div>
                </>
              ) : (
                <textarea data-testid="ai-json-body" style={{gridColumn:'1/-1',width:'100%',padding:'4px 6px',fontSize:13,fontFamily:'monospace',background:'var(--bg-primary)',border:'1px solid var(--border)',borderRadius:4,color:'var(--text-primary)',resize:'vertical',boxSizing:'border-box'}}
                  value={settings.jsonBody} onChange={e => setField('jsonBody',e.target.value)} rows={3} spellCheck={false} />
              )}
            </div>
          )}
          {mentionQuery !== null && mentionMembers.length > 0 && (
            <div className="mention-dropdown" style={{bottom:'100%',top:'auto',marginBottom:0}}>
              {mentionMembers.map((m, i) => (
                <div key={m.id} className={'mention-item' + (i === mentionIdx ? ' active' : '')}
                  onMouseDown={e => { e.preventDefault(); handleMentionSelect(m); }}
                  onMouseEnter={() => setMentionIdx(i)}>
                  {m.username}
                </div>
              ))}
            </div>
          )}
          <div style={{display:'flex',gap:6,alignItems:'stretch'}}>
            <textarea data-testid="chat-input" rows={1} placeholder={aiActive ? 'Ask AI...' : 'Message #chat'} value={text}
              ref={textRef}
              onChange={handleTextChange}
              onKeyDown={handleKey}
              onPaste={handlePaste}
              style={{flex:1,resize:'none',minHeight:36}} />
            <input type="file" ref={fileInput} onChange={handleFile} style={{display:'none'}} multiple />
            <button className="btn-ghost" style={{display:'inline-flex',alignItems:'center',justifyContent:'center',fontSize:18,padding:'4px 6px'}} onClick={() => fileInput.current?.click()} title="Attach file">📎</button>
            <button data-testid="ai-toggle" className={'btn-ghost' + (aiActive ? ' active' : '')}
              style={{display:'inline-flex',alignItems:'center',justifyContent:'center',fontSize:13,padding:'4px 8px',fontWeight:aiActive?600:400,color:aiActive?'var(--accent)':'var(--text-muted)'}}
              onClick={() => setAiActive(!aiActive)} disabled={aiLoading} title={aiActive?'Disable AI':'Enable AI'}>
              🤖 AI
            </button>
            {aiLoading && (
              <button className="btn-ghost" style={{display:'inline-flex',alignItems:'center',justifyContent:'center',padding:'4px 10px',color:'var(--danger)'}}
                onClick={cancelAI} title="Cancel">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </button>
            )}
            <button className="btn-ghost" style={{display:'inline-flex',alignItems:'center',justifyContent:'center',padding:'4px 10px'}}
              disabled={(!text.trim() && attachments.length === 0) || uploading || sending}
              onClick={handleSend} title={aiActive ? 'Send + AI reply' : 'Send'}>
              {uploading ? <span style={{fontSize:14}}>...</span> : (
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="5" y1="12" x2="19" y2="12"/>
                  <polyline points="12 5 19 12 12 19"/>
                </svg>
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
