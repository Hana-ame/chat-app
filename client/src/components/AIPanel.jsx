import { useState, useRef, useCallback, useEffect, forwardRef, useImperativeHandle } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import { streamAI } from '../utils/ai';

const CONTEXT_LIMIT = 50;
const STORAGE_KEY = 'ai_panel_settings';

function buildContext(msgs) {
  const context = [];
  for (const m of msgs) {
    if (context.length >= CONTEXT_LIMIT) break;
    if (m.type === 'stream' || m.user_id === 'ai') {
      context.push({ role: 'assistant', content: m.content });
    } else if (m.user_id && m.content) {
      context.push({ role: 'user', content: m.content });
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
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s));
  } catch {}
}

const defaults = {
  endpoint: import.meta.env.VITE_AI_ENDPOINT || 'https://api.siliconflow.cn/v1/chat/completions',
  authKey: import.meta.env.VITE_AI_AUTH_KEY || '',
  model: import.meta.env.VITE_AI_MODEL || 'deepseek-ai/Deepseek-V4-Flash',
  temperature: '0.7',
  maxTokens: '32768',
  topP: '1',
  sendContext: true,
  jsonBody: JSON.stringify({
    model: 'deepseek-ai/Deepseek-V4-Flash-free',
    messages: [{ role: 'user', content: '' }],
    temperature: 0.7,
    max_tokens: 32768,
    top_p: 1,
  }, null, 2),
};

const AIPanel = forwardRef(function AIPanel({ chatId, onActiveChange, onLoadingChange }, ref) {
  const { user, accessToken } = useAuthStore();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [prompt, setPrompt] = useState('');
  const [tab, setTab] = useState('basic');
  const tabRef = useRef(tab);
  tabRef.current = tab;
  const saved = useRef(loadSettings() || defaults);
  const [fields, setFields] = useState({ ...saved.current });
  const aiAbort = useRef(null);
  const panelRef = useRef(null);

  useEffect(() => {
    const handler = (e) => {
      if (panelRef.current && !panelRef.current.contains(e.target)) {
        setOpen(false);
      }
    };
    if (open) document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  useEffect(() => {
    onActiveChange(open);
  }, [open, onActiveChange]);

  useEffect(() => {
    onLoadingChange(loading);
  }, [loading, onLoadingChange]);

  const setField = useCallback((k, v) => {
    setFields(prev => ({ ...prev, [k]: v }));
    saved.current = { ...saved.current, [k]: v };
    saveSettings(saved.current);
  }, []);

  const cancelAI = useCallback(() => {
    if (aiAbort.current) {
      aiAbort.current.abort();
      aiAbort.current = null;
    }
  }, []);

  const doSend = useCallback(async (content) => {
    if (!content) return;
    const msgId = crypto.randomUUID();
    const botMsg = {
      id: msgId,
      chat_id: chatId,
      user_id: user.id,
      content: '',
      created_at: new Date().toISOString(),
      streaming: true,
      author: { id: 'ai', username: 'AI', avatar_color: '#10a37f' },
    };
    useChatStore.getState().onMessageCreate(botMsg);
    setLoading(true);

    const store = useChatStore.getState();
    const messages = store.messages;
    const context = [...buildContext(messages), { role: 'user', content }];

    const controller = new AbortController();
    aiAbort.current = controller;

    const f = saved.current;
    const t = tabRef.current;
    let body;
    if (t === 'json') {
      try { body = JSON.parse(f.jsonBody); } catch {
        notify('Invalid JSON body', 'error');
        useChatStore.setState(s => ({
          messages: s.messages.map(m =>
            m.id === msgId ? { ...m, streaming: false } : m
          ),
        }));
        setLoading(false);
        return;
      }
    } else {
      body = {
        model: (f.model || '').trim() || undefined,
        messages: f.sendContext ? context : [{ role: 'user', content }],
        temperature: parseFloat(f.temperature) || 0.7,
        max_tokens: parseInt(f.maxTokens) || 32768,
        top_p: parseFloat(f.topP) || 1,
      };
    }

    const source = {
      endpoint: f.endpoint,
      auth_key: f.authKey,
      body,
    };

    try {
      const res = await api.sendStreamMessage(accessToken, chatId, '', source, msgId);
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
          setLoading(false);
          setPrompt('');
          aiAbort.current = null;
        },
        () => {
          notify('AI response failed', 'error');
          useChatStore.setState(s => ({
            messages: s.messages.map(m =>
              m.id === msgId ? { ...m, streaming: false } : m
            ),
          }));
          setLoading(false);
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
        setLoading(false);
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
      setLoading(false);
      aiAbort.current = null;
    }
  }, [chatId, user, accessToken]);

  const handleSend = useCallback(async () => {
    const content = prompt.trim();
    if (!content) return;
    await doSend(content);
  }, [prompt, doSend]);

  const handleKey = useCallback((e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [handleSend]);

  useImperativeHandle(ref, () => ({
    sendAI: doSend,
    cancelAI,
  }), [doSend, cancelAI]);

  return (
    <div ref={panelRef} style={{ position: 'relative', display: 'inline-flex' }}>
      <button className={'btn-ghost' + (open ? ' active' : '')}
        style={{ fontSize: 13, padding: '4px 8px', lineHeight: 0, fontWeight: open ? 600 : 400, color: open ? 'var(--accent)' : 'var(--text-muted)' }}
        onClick={() => setOpen(!open)} title="AI settings"
        disabled={loading}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" style={{ marginRight: 3, verticalAlign: 'middle' }}>
          <path d="M12 2a4 4 0 0 1 4 4v1a4 4 0 0 1-4 4 4 4 0 0 1-4-4V6a4 4 0 0 1 4-4z"/>
          <path d="M16 14H8a4 4 0 0 0-4 4v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2a4 4 0 0 0-4-4z"/>
          <path d="M12 14v6"/>
        </svg>
        AI
      </button>

      {open && (
        <div className="ai-panel">
          <div className="ai-panel-header">
            <div className="ai-panel-tabs">
              <button className={'ai-panel-tab' + (tab === 'basic' ? ' active' : '')}
                onClick={() => setTab('basic')}>Basic</button>
              <button className={'ai-panel-tab' + (tab === 'json' ? ' active' : '')}
                onClick={() => setTab('json')}>JSON</button>
            </div>
          </div>

          <div className="ai-panel-body">
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              {tab === 'basic' ? (
                <>
                  <label className="ai-panel-label">Endpoint</label>
                  <input className="ai-panel-input ai-panel-input-mono" value={fields.endpoint}
                    onChange={e => setField('endpoint', e.target.value)}
                    spellCheck={false} />

                  <label className="ai-panel-label">Auth Key</label>
                  <input className="ai-panel-input ai-panel-input-mono" value={fields.authKey}
                    onChange={e => setField('authKey', e.target.value)}
                    type="password" spellCheck={false} />

                  <hr className="ai-panel-divider" />

                  <label className="ai-panel-label">Model</label>
                  <input className="ai-panel-input ai-panel-input-mono" value={fields.model}
                    onChange={e => setField('model', e.target.value)}
                    spellCheck={false} />

                  <div className="ai-panel-grid">
                    <label className="ai-panel-label">
                      Temperature
                      <input className="ai-panel-input" value={fields.temperature}
                        onChange={e => setField('temperature', e.target.value)}
                        type="number" step="0.1" min="0" max="2"
                        style={{ marginTop: 2 }} />
                    </label>
                    <label className="ai-panel-label">
                      Max Tokens
                      <input className="ai-panel-input" value={fields.maxTokens}
                        onChange={e => setField('maxTokens', e.target.value)}
                        type="number" step="1" min="1"
                        style={{ marginTop: 2 }} />
                    </label>
                    <label className="ai-panel-label">
                      Top P
                      <input className="ai-panel-input" value={fields.topP}
                        onChange={e => setField('topP', e.target.value)}
                        type="number" step="0.05" min="0" max="1"
                        style={{ marginTop: 2 }} />
                    </label>
                  </div>

                  <label className="ai-panel-checkbox">
                    <input type="checkbox" checked={fields.sendContext}
                      onChange={e => setField('sendContext', e.target.checked)} />
                    Send {CONTEXT_LIMIT} context messages
                  </label>
                </>
              ) : (
                <>
                  <label className="ai-panel-label">Request Body (JSON)</label>
                  <textarea className="ai-panel-input" value={fields.jsonBody}
                    onChange={e => setField('jsonBody', e.target.value)}
                    rows={8} spellCheck={false}
                    style={{ fontFamily: 'monospace', fontSize: 11, resize: 'vertical' }} />
                </>
              )}
            </div>
          </div>

          <div className="ai-panel-footer">
            <textarea className="ai-panel-textarea"
              placeholder="Ask AI something..."
              value={prompt}
              onChange={e => setPrompt(e.target.value)}
              onKeyDown={handleKey}
              rows={2}
            />
            <button className="btn-primary ai-panel-btn"
              disabled={!prompt.trim() || loading}
              onClick={handleSend}
              style={{ marginTop: 8 }}>
              {loading ? (
                <>
                  <span className="spinner" style={{ width: 14, height: 14, borderWidth: 2, display: 'inline-block', flexShrink: 0 }} />
                  Generating...
                </>
              ) : (
                <>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <line x1="5" y1="12" x2="19" y2="12"/>
                    <polyline points="12 5 19 12 12 19"/>
                  </svg>
                  Send to AI
                </>
              )}
            </button>
          </div>
        </div>
      )}
    </div>
  );
});

export default AIPanel;
