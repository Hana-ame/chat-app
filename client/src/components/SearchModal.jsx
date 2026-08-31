// SearchModal.jsx — 消息全文搜索弹窗（【本地改动 2026-09-03】FTS5）
//
// 触发：ChatView header menu 中 "Search" 按钮。
// 行为：
//   - 输入框，Enter 触发搜索（300ms debounce）。
//   - 结果按 FTS5 MATCH 语义返回（空格 OR、"" 短语、* 前缀、AND 逻辑运算）。
//   - 每条结果显示：聊天名 + 作者 + 摘要 + 时间。
//   - 点击结果：导航到该消息所在聊天（/g/{chat_id}），并触发 store 事件
//     让 MessageList 滚动/高亮到该消息。
// 关闭：点遮罩 / 按 Escape / 点 ×。

import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';

// 【本地改动 2026-09-03】highlight 片段渲染：将 **...** 包围的 token 高亮显示。
function highlightText(text, term) {
  if (!text || !term) return <span>{text}</span>;
  const idx = text.toLowerCase().indexOf(term.toLowerCase());
  if (idx < 0) return <span>{text}</span>;
  return (
    <span>
      <span>{text.slice(0, idx)}</span>
      <span style={{ background: 'rgba(88,101,242,0.25)', color: 'var(--accent)', borderRadius: 2, padding: '0 2px' }}>
        {text.slice(idx, idx + term.length)}
      </span>
      <span>{text.slice(idx + term.length)}</span>
    </span>
  );
}

function timeFormat(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = +now - +d;
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

const debounce = (fn, ms) => {
  let t;
  return (...a) => { clearTimeout(t); t = setTimeout(() => fn(...a), ms); };
};

/** @param {{open:any, onClose:any, onOpenMessage?:any}} _ */
export default function SearchModal({ open, onClose, onOpenMessage }) {
  const user = useAuthStore(s => s.user);
  const accessToken = useAuthStore(s => s.accessToken);
  const chats = useChatStore(s => s.chats);
  const nav = useNavigate();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [cursor, setCursor] = useState('');
  const [filter, setFilter] = useState('all'); // 'all' | chat_id
  const [error, setError] = useState(null);
  const inputRef = useRef(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setResults([]);
      setLoading(false);
      setHasMore(false);
      setCursor('');
      setError(null);
      setFilter('all');
      setTimeout(() => inputRef.current?.focus(), 60);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const handler = (e) => {
      if (e.key === 'Escape') onClose();
      if (e.key === 'Enter' && !e.shiftKey && e.target === inputRef.current) {
        e.preventDefault();
        void doSearch(query, false);
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, query, onClose]);

  const doSearch = useCallback(async (q, append) => {
    if (!q.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const resp = await api.searchMessages(accessToken, q, filter === 'all' ? '' : filter, '', cursor, 30);
      if (append) {
        setResults(prev => [...prev, ...resp.messages]);
      } else {
        setResults(resp.messages);
        setCursor('');
      }
      setHasMore(!!resp.has_more);
      if (resp.next) setCursor(resp.next);
    } catch (e) {
      setError('搜索失败：' + (e.message || '未知错误'));
    } finally {
      setLoading(false);
    }
  }, [accessToken, filter, cursor]);

  const handleSearch = debounce((q) => doSearch(q, false), 300);
  const handleInputChange = (e) => {
    setQuery(e.target.value);
    handleSearch(e.target.value);
  };

  const handleLoadMore = () => {
    if (hasMore) doSearch(query, true);
  };

  const handleResultClick = (r) => {
    const msg = r.message;
    onClose();
    nav('/g/' + msg.chat_id);
    // 让 MessageList 滚动到高亮
    setTimeout(() => onOpenMessage && onOpenMessage(msg), 300);
  };

  const handleChatFilter = (chatId) => {
    setFilter(chatId);
    setResults([]);
    setCursor('');
    setHasMore(false);
  };

  if (!open) return null;

  const chatMap = {};
  for (const c of chats) chatMap[c.id] = c;

  return (
    <div
      className="modal-overlay"
      onClick={onClose}
      style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'center', padding: '40px 16px 32px' }}
    >
      <div
        className="modal-box"
        onClick={e => e.stopPropagation()}
        style={{ width: '100%', maxWidth: 720, maxHeight: '82vh', display: 'flex', flexDirection: 'column' }}
      >
        <div className="modal-header" style={{ alignItems: 'center', marginBottom: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/>
            </svg>
            <span style={{ fontWeight: 600 }}>搜索消息</span>
          </div>
          <button className="btn-ghost" onClick={onClose} style={{ padding: '4px 8px', fontSize: 16, lineHeight: 1 }}>×</button>
        </div>

        <div style={{ display: 'flex', gap: 8, marginBottom: 10, alignItems: 'center' }}>
          <input
            ref={inputRef}
            className="input-field"
            style={{ flex: 1, padding: '8px 12px', fontSize: 14 }}
            placeholder="搜索消息（空格 OR、&quot;&quot; 短语、* 前缀、AND）"
            value={query}
            onChange={handleInputChange}
          />
          <button
            className={'btn' + (loading ? ' btn-secondary' : ' btn-primary')}
            style={{ fontSize: 12, padding: '8px 12px', whiteSpace: 'nowrap' }}
            onClick={() => doSearch(query, false)}
            disabled={!query.trim() || loading}
          >
            {loading ? '...' : '搜索'}
          </button>
        </div>

        <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginBottom: 10 }}>
          <button
            className={'btn-ghost' + (filter === 'all' ? ' active' : '')}
            style={{ fontSize: 11, padding: '3px 8px' }}
            onClick={() => handleChatFilter('all')}
          >全部</button>
          {chats.filter(c => c.type !== 'notify').slice(0, 8).map(c => (
            <button
              key={c.id}
              className={'btn-ghost' + (filter === c.id ? ' active' : '')}
              style={{ fontSize: 11, padding: '3px 8px' }}
              onClick={() => handleChatFilter(c.id)}
            >
              {c.name || c.id.slice(0, 4)}
            </button>
          ))}
        </div>

        {error && (
          <div style={{ color: 'var(--danger)', fontSize: 12, padding: '4px 0' }}>
            {error}
          </div>
        )}

        <div style={{ flex: 1, overflowY: 'auto', padding: '2px 0' }}>
          {results.length === 0 && !loading && (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '32px 0', fontSize: 13 }}>
              {query.trim() ? '未找到匹配的消息' : '输入关键词搜索'}
            </div>
          )}

          {results.map((r, i) => {
            const msg = r.message;
            const chat = chatMap[msg.chat_id] || {};
            const chatName = chat.name || (chat.type === 'dm' ? 'DM' : msg.chat_id.slice(0, 8));
            const author = msg.author || {};
            const preview = (r.highlight || msg.content || '').slice(0, 140);
            const term = r.highlight ? preview.replace(/\*\*/g, '') : null;
            return (
              <div
                key={i}
                style={{
                  display: 'flex',
                  gap: 8,
                  padding: '8px 10px',
                  borderRadius: 6,
                  cursor: 'pointer',
                  borderBottom: '1px solid var(--border)',
                }}
                onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                onMouseLeave={e => e.currentTarget.style.background = 'transparent'}
                onClick={() => handleResultClick(r)}
              >
                <div style={{
                  width: 8, height: 8, borderRadius: '50%',
                  background: author.avatar_color || '#5865F2',
                  marginTop: 4, flexShrink: 0,
                }}/>
                <div style={{ flex: 1, overflow: 'hidden' }}>
                  <div style={{ display: 'flex', gap: 6, alignItems: 'center', fontSize: 12 }}>
                    <span style={{ fontWeight: 600 }}>{author.username || '?'}</span>
                    <span style={{ color: 'var(--text-muted)' }}>· {chatName}</span>
                    <span style={{ color: 'var(--text-muted)' }}>· {timeFormat(msg.created_at)}</span>
                  </div>
                  <div style={{ fontSize: 13, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {highlightText(preview, query.replace(/^"|"$/g, '').split(/\s+/)[0])}
                  </div>
                </div>
              </div>
            );
          })}

          {hasMore && (
            <div style={{ textAlign: 'center', padding: '10px 0' }}>
              <button className="btn btn-ghost" style={{ fontSize: 12 }} onClick={handleLoadMore}>
                加载更多
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
