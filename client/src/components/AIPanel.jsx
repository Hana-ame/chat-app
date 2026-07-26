import { useState, useRef, useCallback, forwardRef, useImperativeHandle } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import { streamAI } from '../utils/ai';

const CONTEXT_LIMIT = 50;

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

const AIPanel = forwardRef(function AIPanel({ chatId, onActiveChange, onLoadingChange }, ref) {
  const { user, accessToken } = useAuthStore();
  const [aiMode, setAiMode] = useState(false);
  const [aiLoading, setAiLoading] = useState(false);

  const setMode = useCallback((v) => { setAiMode(v); onActiveChange?.(v); }, [onActiveChange]);
  const setLoading = useCallback((v) => { setAiLoading(v); onLoadingChange?.(v); }, [onLoadingChange]);
  const [aiEndpoint, setAiEndpoint] = useState(import.meta.env.VITE_AI_ENDPOINT || 'https://api.siliconflow.cn/v1/chat/completions');
  const [aiAuthKey, setAiAuthKey] = useState(import.meta.env.VITE_AI_AUTH_KEY || '');
  const [aiBodyMode, setAiBodyMode] = useState('simple');
  const [aiSendContext, setAiSendContext] = useState(true);
  const [aiModel, setAiModel] = useState(import.meta.env.VITE_AI_MODEL || 'deepseek-ai/Deepseek-V4-Flash');
  const [aiTemperature, setAiTemperature] = useState('0.7');
  const [aiMaxTokens, setAiMaxTokens] = useState('32768');
  const [aiTopP, setAiTopP] = useState('1');
  const [aiJsonBody, setAiJsonBody] = useState(JSON.stringify({
    model: 'deepseek-ai/Deepseek-V4-Flash-free',
    messages: [{ role: 'user', content: '' }],
    temperature: 0.7,
    max_tokens: 32768,
    top_p: 1,
  }, null, 2));
  const aiAbort = useRef(null);

  const cancelAI = useCallback(() => {
    if (aiAbort.current) {
      aiAbort.current.abort();
      aiAbort.current = null;
    }
  }, []);

  const handleAISend = useCallback(async (content) => {
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
    const context = content ? [...buildContext(messages), { role: 'user', content }] : [];

    const controller = new AbortController();
    aiAbort.current = controller;

    let body;
    if (aiBodyMode === 'json') {
      try { body = JSON.parse(aiJsonBody); } catch {
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
        model: aiModel.trim() || undefined,
        messages: aiSendContext ? context : [{ role: 'user', content }],
        temperature: parseFloat(aiTemperature) || 0.7,
        max_tokens: parseInt(aiMaxTokens) || 32768,
        top_p: parseFloat(aiTopP) || 1,
      };
    }

    const source = {
      endpoint: aiEndpoint,
      auth_key: aiAuthKey,
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
  }, [chatId, user, accessToken, aiBodyMode, aiEndpoint, aiAuthKey, aiModel, aiTemperature, aiMaxTokens, aiTopP, aiSendContext, aiJsonBody]);

  useImperativeHandle(ref, () => ({
    sendAI: handleAISend,
    cancelAI,
  }), [handleAISend, cancelAI]);

  return (
    <div style={{position:'relative',display:'inline-flex'}}>
      <button className={'btn-ghost' + (aiMode ? ' active' : '')} style={{fontSize:13,padding:'4px 6px',lineHeight:0,fontWeight:aiMode?600:400,color:aiMode?'var(--accent)':'var(--text-muted)'}}
        onClick={() => setMode(!aiMode)} title="Toggle AI mode"
        disabled={aiLoading}>
        AI{aiMode ? ' ▼' : ''} {aiLoading ? '...' : ''}
      </button>
      {aiMode && (
        <div style={{position:'absolute',bottom:'100%',left:0,marginBottom:4,display:'flex',flexDirection:'column',gap:4,background:'var(--bg-tertiary)',padding:8,borderRadius:8,border:'1px solid var(--border)',fontSize:12,whiteSpace:'nowrap',zIndex:100,maxHeight:'70vh',overflowY:'auto'}}>
          <label>endpoint:
            <input value={aiEndpoint} onChange={e => setAiEndpoint(e.target.value)}
              style={{marginLeft:6,width:320,fontSize:12,padding:'1px 4px'}} />
          </label>
          <label>auth_key:
            <input value={aiAuthKey} onChange={e => setAiAuthKey(e.target.value)}
              type="password" style={{marginLeft:6,width:320,fontSize:12,padding:'1px 4px'}} />
          </label>

          <div style={{display:'flex',gap:4,margin:'4px 0'}}>
            <label style={{display:'flex',alignItems:'center',gap:3,cursor:'pointer',fontWeight:aiBodyMode==='simple'?600:400}}>
              <input type="radio" name="aiBodyMode" value="simple" checked={aiBodyMode==='simple'}
                onChange={e => setAiBodyMode(e.target.value)} /> Simple
            </label>
            <label style={{display:'flex',alignItems:'center',gap:3,cursor:'pointer',fontWeight:aiBodyMode==='json'?600:400}}>
              <input type="radio" name="aiBodyMode" value="json" checked={aiBodyMode==='json'}
                onChange={e => setAiBodyMode(e.target.value)} /> JSON
            </label>
          </div>

          {aiBodyMode === 'simple' ? (
            <>
              <label>model:
                <input value={aiModel} onChange={e => setAiModel(e.target.value)}
                  style={{marginLeft:6,width:240,fontSize:12,padding:'1px 4px'}} />
              </label>
              <div style={{display:'flex',gap:8}}>
                <label>temperature:
                  <input value={aiTemperature} onChange={e => setAiTemperature(e.target.value)}
                    type="number" step="0.1" min="0" max="2"
                    style={{marginLeft:4,width:60,fontSize:12,padding:'1px 4px'}} />
                </label>
                <label>max_tokens:
                  <input value={aiMaxTokens} onChange={e => setAiMaxTokens(e.target.value)}
                    type="number" step="1" min="1"
                    style={{marginLeft:4,width:80,fontSize:12,padding:'1px 4px'}} />
                </label>
                <label>top_p:
                  <input value={aiTopP} onChange={e => setAiTopP(e.target.value)}
                    type="number" step="0.05" min="0" max="1"
                    style={{marginLeft:4,width:60,fontSize:12,padding:'1px 4px'}} />
                </label>
              </div>
              <label style={{display:'flex',alignItems:'center',gap:4,cursor:'pointer',marginTop:2}}>
                <input type="checkbox" checked={aiSendContext}
                  onChange={e => setAiSendContext(e.target.checked)} />
                发送 {CONTEXT_LIMIT} 条上下文
              </label>
            </>
          ) : (
            <label style={{display:'flex',flexDirection:'column',gap:2}}>
              body (JSON):
              <textarea value={aiJsonBody} onChange={e => setAiJsonBody(e.target.value)}
                rows={8} spellCheck={false}
                style={{width:360,fontSize:11,fontFamily:'monospace',padding:'4px 6px',resize:'vertical'}} />
            </label>
          )}
        </div>
      )}
    </div>
  );
});

export default AIPanel;
