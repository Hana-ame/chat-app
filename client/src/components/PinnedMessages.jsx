// PinnedMessages.jsx — 置顶消息抽屉（【本地改动 2026-09-02】chatto FDR-037 多消息置顶）
//
// 与聊天公告（pinned_message）完全独立：本组件展示 chat_pins 表中的多条置顶。
// 行为：
//   - 打开时加载 /api/chats/{chatID}/pins（created_at DESC）。
//   - 每条显示：作者 + 时间 + 摘要；owner/admin 可见 Unpin 按钮。
//   - 点击条目跳转到对应消息（scrollIntoView，元素存在时）。
//   - 分页：has_more 时显示「加载更多」。
// 触发：ChatView header 新增 pin 图标按钮。

import { useEffect, useState, useCallback } from 'react';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { renderContent } from './renderContent';

function timeFormat(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = +now - +d;
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

export default function PinnedMessages({ chatId, open, onClose }) {
  const user = useAuthStore(s => s.user);
  const accessToken = useAuthStore(s => s.accessToken);
  const chats = useChatStore(s => s.chats);
  const [pins, setPins] = useState([]);
  const [loading, setLoading] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [cursor, setCursor] = useState('');

  const chat = chats.find(c => c.id === chatId);
  // 【本地改动 2026-09-02】权限：owner 或 admin（chat.members role）可 unpin；
  // DM 不展示置顶入口（后端也拒绝）。member 可读列表。
  const canPin = !!(chat && chat.type !== 'dm' && chat.type !== 'notify' &&
    (chat.owner_id === user.id || chat.members?.some(m => m.id === user.id && (m.role === 'admin' || m.role === 'owner'))));

  const load = useCallback(async (append) => {
    if (!chatId || !accessToken) return;
    setLoading(true);
    try {
      const resp = await api.listPinnedMessages(accessToken, chatId, cursor, 20);
      if (append) setPins(prev => [...prev, ...resp.pins]);
      else setPins(resp.pins);
      setHasMore(!!resp.has_more);
      if (resp.next) setCursor(resp.next);
    } catch (e) {
      console.error('Load pinned messages error:', e);
    } finally {
      setLoading(false);
    }
  }, [chatId, accessToken, cursor]);

  useEffect(() => {
    if (open && chatId) {
      setPins([]);
      setCursor('');
      setHasMore(false);
      load(false);
    }
  }, [open, chatId]);

  useEffect(() => {
    if (!open) return;
    const handler = (e) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [open, onClose]);

  const handleUnpin = async (messageId) => {
    if (!confirm('取消置顶这条消息？')) return;
    try {
      await api.unpinMessage(accessToken, chatId, messageId);
      setPins(prev => prev.filter(p => p.message_id !== messageId));
    } catch (e) {
      console.error('Unpin error:', e);
    }
  };

  const handleJump = (messageId) => {
    onClose();
    setTimeout(() => {
      const el = document.getElementById('msg-' + messageId);
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      // 【本地改动 2026-09-03】触发目标消息高亮闪烁（与 reply 引用跳转一致）。
      document.dispatchEvent(new CustomEvent('chat:jump-to-message', { detail: { messageId } }));
    }, 150);
  };

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={onClose} style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'center', padding: '40px 16px 32px' }}>
      <div className="modal-box" onClick={e => e.stopPropagation()}
        style={{ width: '100%', maxWidth: 640, maxHeight: '82vh', display: 'flex', flexDirection: 'column' }}>
        <div className="modal-header" style={{ alignItems: 'center', marginBottom: 12 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M11 5L6 9H2v6h4l5 4V5z"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14M15.54 8.46a5 5 0 0 1 0 7.07"/>
            </svg>
            <span style={{ fontWeight: 600 }}>置顶消息</span>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>({pins.length})</span>
          </div>
          <button className="btn-ghost" onClick={onClose} style={{ padding: '4px 8px', fontSize: 16, lineHeight: 1 }}>×</button>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '2px 0' }}>
          {!loading && pins.length === 0 && (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '32px 0', fontSize: 13 }}>
              暂无置顶消息
            </div>
          )}

          {pins.map((p, i) => {
            const m = p.message || {};
            const deleted = !!m.deleted_at;
            return (
              <div key={i}
                style={{
                  padding: '10px 12px',
                  borderBottom: '1px solid var(--border)',
                  cursor: deleted ? 'default' : 'pointer',
                  borderRadius: 6,
                }}
                onMouseEnter={e => !deleted && (e.currentTarget.style.background = 'var(--bg-hover)')}
                onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                onClick={() => !deleted && handleJump(m.id)}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, marginBottom: 4 }}>
                  <span style={{ fontWeight: 600 }}>{m.author?.username || '?'}</span>
                  <span style={{ color: 'var(--text-muted)' }}>· {timeFormat(m.created_at)}</span>
                  {deleted && <span style={{ color: 'var(--danger)', fontSize: 11 }}>(已删除)</span>}
                  {canPin && !deleted && (
                    <button
                      className="btn-ghost danger"
                      style={{ marginLeft: 'auto', fontSize: 11, padding: '2px 8px' }}
                      onClick={(e) => { e.stopPropagation(); handleUnpin(m.id); }}
                    >Unpin</button>
                  )}
                </div>
                <div style={{ fontSize: 13, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {deleted ? <span className="msg-deleted">(message deleted)</span> : renderContent(m.content || '', {})}
                </div>
              </div>
            );
          })}

          {hasMore && (
            <div style={{ textAlign: 'center', padding: '10px 0' }}>
              <button className="btn btn-ghost" style={{ fontSize: 12 }} onClick={() => load(true)} disabled={loading}>
                {loading ? '...' : '加载更多'}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
