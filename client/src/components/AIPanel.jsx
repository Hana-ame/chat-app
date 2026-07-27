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

const AIPanel = forwardRef(function AIPanel({ chatId, active, onActiveChange, onLoadingChange }, ref) {
  const { user, accessToken } = useAuthStore();
  const [loading, setLoading] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [tab, setTab] = useState('basic');
  const tabRef = useRef(tab);
  tabRef.current = tab;
  const saved = useRef(loadSettings() || defaults);
  const [fields, setFields] = useState({ ...saved.current });
  const aiAbort = useRef(null);
  const settingsRef = useRef(null);

  useEffect(() => {
    const handler = (e) => {
      if (settingsRef.current && !settingsRef.current.contains(e.target)) {
        setShowSettings(false);
      }
    };
    if (showSettings) document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showSettings]);

  useEffect(() => {
    onActiveChange(active);
  }, [active, onActiveChange]);

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
      author: { id: user.id, username: user.username, avatar_color: user.avatar_color },
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

  useImperativeHandle(ref, () => ({
    sendAI: doSend,
    cancelAI,
  }), [doSend, cancelAI]);

  return (
    <div style={{ position: 'relative', display: 'inline-flex' }}>
      <button className={'btn-ghost' + (active ? ' active' : '')}
        style={{ fontSize: 13, padding: '4px 8px', lineHeight: 0, fontWeight: active ? 600 : 400, color: active ? 'var(--accent)' : 'var(--text-muted)' }}
        onClick={() => onActiveChange(!active)} title={active ? 'Disable AI' : 'Enable AI'}
        disabled={loading}>
        🤖 AI
      </button>
      {active && (
        <button className="btn-ghost"
          style={{ fontSize: 11, padding: '2px 4px', lineHeight: 0, marginLeft: 2 }}
          onClick={(e) => { e.stopPropagation(); setShowSettings(!showSettings); }}
          title="AI settings">
          ⚙
        </button>
      )}
      {showSettings && (
        <div ref={settingsRef} style={{
          position: 'absolute', bottom: '100%', right: 0, marginBottom: 6, zIndex: 50,
          background: 'var(--bg-primary)', border: '1px solid var(--border)',
          borderRadius: 'var(--radius)', boxShadow: '0 4px 16px rgba(0,0,0,0.5)',
          padding: 10, minWidth: 360, maxWidth: 400,
        }}>
          <div style={{ display: 'flex', gap: 2, marginBottom: 8 }}>
            <button style={{ fontSize: 12, padding: '4px 10px', background: 'none', cursor: 'pointer', color: tab === 'basic' ? 'var(--accent)' : 'var(--text-muted)', borderBottom: tab === 'basic' ? '2px solid var(--accent)' : '2px solid transparent', fontWeight: tab === 'basic' ? 600 : 400 }}
              onClick={() => setTab('basic')}>Basic</button>
            <button style={{ fontSize: 12, padding: '4px 10px', background: 'none', cursor: 'pointer', color: tab === 'json' ? 'var(--accent)' : 'var(--text-muted)', borderBottom: tab === 'json' ? '2px solid var(--accent)' : '2px solid transparent', fontWeight: tab === 'json' ? 600 : 400 }}
              onClick={() => setTab('json')}>JSON</button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            {tab === 'basic' ? (
              <>
                <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Endpoint</label>
                <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, fontFamily: 'monospace', background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)' }} value={fields.endpoint}
                  onChange={e => setField('endpoint', e.target.value)} spellCheck={false} />
                <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Auth Key</label>
                <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, fontFamily: 'monospace', background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)' }} value={fields.authKey}
                  onChange={e => setField('authKey', e.target.value)} type="password" spellCheck={false} />
                <hr style={{ border: 'none', borderTop: '1px solid var(--border)', margin: '4px 0' }} />
                <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Model</label>
                <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, fontFamily: 'monospace', background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)' }} value={fields.model}
                  onChange={e => setField('model', e.target.value)} spellCheck={false} />
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 6 }}>
                  <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Temperature
                    <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)', marginTop: 2 }} value={fields.temperature}
                      onChange={e => setField('temperature', e.target.value)} type="number" step="0.1" min="0" max="2" />
                  </label>
                  <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Max Tokens
                    <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)', marginTop: 2 }} value={fields.maxTokens}
                      onChange={e => setField('maxTokens', e.target.value)} type="number" step="1" min="1" />
                  </label>
                  <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Top P
                    <input style={{ width: '100%', padding: '6px 8px', fontSize: 12, background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)', marginTop: 2 }} value={fields.topP}
                      onChange={e => setField('topP', e.target.value)} type="number" step="0.05" min="0" max="1" />
                  </label>
                </div>
                <label style={{ fontSize: 11, color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: 4 }}>
                  <input type="checkbox" checked={fields.sendContext}
                    onChange={e => setField('sendContext', e.target.checked)} />
                  Send {CONTEXT_LIMIT} context messages
                </label>
              </>
            ) : (
              <>
                <label style={{ fontSize: 11, color: 'var(--text-muted)' }}>Request Body (JSON)</label>
                <textarea style={{ width: '100%', padding: '6px 8px', fontSize: 11, fontFamily: 'monospace', background: 'var(--bg-secondary)', border: '1px solid var(--border)', borderRadius: 4, color: 'var(--text-primary)', resize: 'vertical' }} value={fields.jsonBody}
                  onChange={e => setField('jsonBody', e.target.value)} rows={8} spellCheck={false} />
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
});

export default AIPanel;
