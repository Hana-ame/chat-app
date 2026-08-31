// QuickSwitcher.jsx — Cmd-K 快速切换器（【本地改动 2026-09-03】对齐 chatto FDR-015）
//
// 键盘驱动导航面板：Cmd+K / Ctrl+K 打开，Escape / 点外部关闭。
// 目录：notifications + 所有已加入聊天（group/dm）。模糊匹配（fuzzyMatch）。
// 空查询时显示 Recent（最近 15 个）+ 分组（Rooms / DMs）。
// 最近记忆用 localStorage，复用 lastRoom 的 key 模式（独立 key）。
// 选择 → onNavigate(id)。
//
// 与 chatto 的差异（chat-app 单服务器，已按 scope 收敛，不承认派生）：
//   - 无多服务器目录（chat-app 单实例）。
//   - 无 `?` 消息搜索（FTS 搜索已有独立入口 SearchModal）。
//   - 成员搜索需要后端 user 目录，本组件先只做聊天/通知目录（有成员搜索时再扩展）。

import { useEffect, useMemo, useRef, useState } from 'react';
import { fuzzyMatch, combinedScore } from '../utils/fuzzyMatch';

const RECENT_KEY = 'chatapp:quickSwitcherRecent';
const MAX_RECENT = 15;

function loadRecent() {
  try { return JSON.parse(localStorage.getItem(RECENT_KEY) || '[]'); } catch { return []; }
}
function saveRecent(list) {
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(list.slice(0, MAX_RECENT))); } catch {}
}

export default function QuickSwitcher({ open, onClose, chats, onNavigate, currentUserId }) {
  const [query, setQuery] = useState('');
  const [sel, setSel] = useState(0);
  const inputRef = useRef(null);
  const listRef = useRef(null);

  useEffect(() => {
    if (open) {
      setQuery('');
      setSel(0);
      setTimeout(() => inputRef.current?.focus(), 30);
    }
  }, [open]);

  // 目录构建：notifications + chats（真实聊天）
  const catalogue = useMemo(() => {
    const notify = chats.find(c => c.type === 'notify');
    const items = [];
    if (notify) items.push({ kind: 'notifications', id: 'notifications', label: '通知', detail: 'Notifications', chatId: notify.id });
    for (const c of chats) {
      if (c.type === 'notify') continue;
      if (c.type === 'dm') {
        const other = (c.members || []).find(m => m.id !== currentUserId);
        items.push({
          kind: 'dm',
          id: c.id,
          label: (other && other.username) || c.name || 'DM',
          detail: 'DM',
          chatId: c.id,
        });
      } else {
        items.push({ kind: 'room', id: c.id, label: c.name || 'Group', detail: c.member_count ? c.member_count + ' members' : '', chatId: c.id });
      }
    }
    return items;
  }, [chats]);

  const results = useMemo(() => {
    const q = query.trim();
    if (!q) {
      const recent = loadRecent();
      const recentMap = new Map(recent.map(id => [id, true]));
      const recents = recent.map(id => catalogue.find(x => x.chatId === id)).filter(Boolean);
      const rest = catalogue.filter(x => x.kind === 'notifications' || !recentMap.has(x.chatId));
      // 分组：rooms 和 dms 各按 label 排序
      const rooms = rest.filter(x => x.kind === 'room').sort((a, b) => a.label.localeCompare(b.label));
      const dms = rest.filter(x => x.kind === 'dm').sort((a, b) => a.label.localeCompare(b.label));
      const notifications = rest.filter(x => x.kind === 'notifications');
      const out = [
        ...(recents.length ? [{ group: '最近', items: recents }] : []),
        ...(rooms.length ? [{ group: 'Rooms', items: rooms }] : []),
        ...(dms.length ? [{ group: 'DMs', items: dms }] : []),
        ...(notifications.length ? [{ group: 'Notifications', items: notifications }] : []),
      ];
      return out;
    }

    const ranked = [];
    for (const item of catalogue) {
      const l = fuzzyMatch(q, item.label);
      const d = fuzzyMatch(q, item.detail);
      if (l || d) {
        ranked.push({ item, score: combinedScore(l ? l.score : 0, d ? d.score : 0) });
      }
    }
    ranked.sort((a, b) => b.score - a.score || a.item.label.localeCompare(b.item.label));
    return [{ group: '结果', items: ranked.map(r => r.item) }];
  }, [query, catalogue]);

  // 展平供键盘导航
  const flat = useMemo(() => results.flatMap(g => g.items), [results]);
  useEffect(() => { setSel(0); }, [query, open]);

  const pick = (item) => {
    const recent = loadRecent().filter(id => id !== item.chatId);
    recent.unshift(item.chatId);
    saveRecent(recent);
    onNavigate(item.chatId);
  };

  const onKeyDown = (e) => {
    if (e.key === 'ArrowDown') { e.preventDefault(); setSel(s => Math.min(s + 1, flat.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSel(s => Math.max(s - 1, 0)); }
    else if (e.key === 'Enter') { e.preventDefault(); const it = flat[sel]; if (it) pick(it); }
    else if (e.key === 'Escape') { onClose(); }
  };

  // 选中项滚动到可视
  useEffect(() => {
    const el = listRef.current?.querySelector('.qs-item.selected');
    el?.scrollIntoView({ block: 'nearest' });
  }, [sel, results]);

  if (!open) return null;

  return (
    <div className="modal-overlay" onClick={onClose}
      style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'center', paddingTop: '12vh' }}>
      <div className="modal-box" onClick={e => e.stopPropagation()} style={{ width: '100%', maxWidth: 560, display: 'flex', flexDirection: 'column' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '0 4px', marginBottom: 8 }}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
          </svg>
          <input
            ref={inputRef}
            className="input-field"
            style={{ flex: 1, padding: '8px 12px', fontSize: 14 }}
            placeholder="搜索聊天 / 通知…"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={onKeyDown}
          />
        </div>

        <div ref={listRef} style={{ maxHeight: '60vh', overflowY: 'auto', padding: '2px 0' }}>
          {flat.length === 0 && (
            <div style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '24px 0', fontSize: 13 }}>无匹配</div>
          )}
          {results.map((g, gi) => (
            <div key={gi}>
              {g.group && <div style={{ fontSize: 11, color: 'var(--text-muted)', padding: '6px 8px 2px', textTransform: 'uppercase', letterSpacing: 0.5 }}>{g.group}</div>}
              {g.items.map((item, ii) => {
                const idx = results.slice(0, gi).reduce((n, x) => n + x.items.length, 0) + ii;
                const selected = idx === sel;
                return (
                  <div
                    key={item.id}
                    className={'qs-item' + (selected ? ' selected' : '')}
                    onClick={() => pick(item)}
                    onMouseEnter={() => setSel(idx)}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 10,
                      padding: '8px 10px', borderRadius: 6, cursor: 'pointer',
                      background: selected ? 'var(--bg-active)' : 'transparent',
                    }}
                  >
                    <div style={{
                      width: 28, height: 28, borderRadius: '50%', flexShrink: 0,
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      fontSize: 13, fontWeight: 600, background: 'var(--bg-secondary)', color: 'var(--text-primary)',
                    }}>
                      {item.kind === 'notifications' ? '🔔' : item.kind === 'dm' ? '💬' : (item.label || '?')[0]}
                    </div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontWeight: 600, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.label}</div>
                      {item.detail && <div style={{ fontSize: 11, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.detail}</div>}
                    </div>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
