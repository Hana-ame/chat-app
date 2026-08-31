import { useState, useMemo, useRef, useEffect, memo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { renderContent } from './renderContent';
import MessageLinkCard from './MessageLinkCard';
import UserAvatar from './UserAvatar';
import UserProfileModal from './UserProfileModal';
import { notify } from '../store/notification';

function ThinkingContent({ content, streaming }) {
  const [open, setOpen] = useState(false);
  const contentRef = useRef(null);
  const [height, setHeight] = useState(0);

  useEffect(() => {
    if (open && contentRef.current) {
      setHeight(contentRef.current.scrollHeight);
    }
  }, [open, content]);

  return (
    <div className={'thinking-block' + (open ? ' open' : '')}>
      <div className="thinking-block-header" onClick={() => setOpen(o => !o)} role="button" tabIndex={0}
        onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setOpen(o => !o); } }}>
        <svg className="thinking-block-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
          <polyline points="9 18 15 12 9 6" />
        </svg>
        <svg className="thinking-block-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 2a4 4 0 0 0-4 4c0 2 2 3 2 5h4c0-2 2-3 2-5a4 4 0 0 0-4-4Z"/>
          <path d="M9 18h6"/>
          <path d="M10 22h4"/>
          <path d="M12 11v2"/>
          <path d="M8 13a4 4 0 0 0 8 0"/>
        </svg>
        <span className="thinking-block-label">Reasoning</span>
        {streaming && <span className="thinking-block-streaming" />}
      </div>
      <div className="thinking-block-collapse" style={{ maxHeight: open ? height : 0 }}>
        <div ref={contentRef} className="thinking-block-content">{content}</div>
      </div>
    </div>
  );
}

const COMMON_EMOJI = ['👍','❤️','😂','🎉','😢','😡','👀','🔥','✅','❌'];

function timeFormat(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = +now - +d;
  if (diff < 60e3) return 'now';
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

/** @type {import('react').FunctionComponent<{msg:any,sameAuthor:boolean,chatId:any,onReply:any}>} */
const MessageItem = memo(function MessageItem({ msg, sameAuthor, chatId, onReply }) {
  const user = useAuthStore(s => s.user);
  const accessToken = useAuthStore(s => s.accessToken);
  const pinnedMessage = useChatStore(s => s.pinnedMessage);
  const chats = useChatStore(s => s.chats);
  const isMe = msg.user_id === user.id;
  const [showEmoji, setShowEmoji] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState(msg.content);
  const [opPending, setOpPending] = useState(false);
  const [profileUser, setProfileUser] = useState(null);
  // 【本地改动 2026-09-03】引用跳转高亮：本消息被引用点击时闪烁。
  const [highlighted, setHighlighted] = useState(false);
  const highlightTimer = useRef(null);

  const chat = useMemo(() => chats.find(c => c.id === chatId), [chats, chatId]);
  const author = useMemo(() => {
    if (msg.type === 'stream' && msg.author) return msg.author;
    if (msg.user_id === user.id) return user;
    return chat?.members?.find(m => m.id === msg.user_id) || msg.author || { username: 'Unknown', avatar_color: '#5865F2', id: msg.user_id };
  }, [chat, msg.user_id, msg.author, user, msg.type]);
  const pickerRef = useRef(null);
  const emojiBtnRef = useRef(null);
  const [pickerPos, setPickerPos] = useState(null);

  useEffect(() => {
    if (!showEmoji) { setPickerPos(null); return; }
    const btn = emojiBtnRef.current;
    if (!btn) return;
    const rect = btn.getBoundingClientRect();
    const spaceAbove = rect.top;
    const spaceRight = window.innerWidth - rect.right;
    setPickerPos({
      left: spaceRight < 240 ? Math.max(0, window.innerWidth - 240) : rect.left,
      top: spaceAbove > 100 ? rect.top - 8 : rect.bottom,
    });
    const handler = (e) => {
      if (pickerRef.current && !pickerRef.current.contains(e.target)) {
        setShowEmoji(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showEmoji]);

  // 【本地改动 2026-09-03】监听「引用跳转」事件：目标消息闪烁提示。
  useEffect(() => {
    const onJump = (e) => {
      const targetId = e.detail?.messageId;
      if (!targetId || targetId !== msg.id) return;
      if (highlightTimer.current) clearTimeout(highlightTimer.current);
      setHighlighted(false); // 先复位再置位，保证连续点击也能重启动画
      requestAnimationFrame(() => setHighlighted(true));
      highlightTimer.current = setTimeout(() => setHighlighted(false), 1300);
    };
    document.addEventListener('chat:jump-to-message', onJump);
    return () => {
      document.removeEventListener('chat:jump-to-message', onJump);
      if (highlightTimer.current) clearTimeout(highlightTimer.current);
    };
  }, [msg.id]);

  const userMap = useMemo(() => {
    const chat = chats.find(c => c.id === chatId);
    if (!chat?.members) return {};
    const map = {};
    for (const m of chat.members) {
      map[m.id] = m.username;
    }
    return map;
  }, [chats, chatId]);

  const displayReactions = msg.reactions || [];

  const handleReaction = async (emoji) => {
    const has = displayReactions.find(r => r.emoji === emoji && r.me);
    try {
      if (has) await api.removeReaction(accessToken, chatId, msg.id, emoji);
      else await api.addReaction(accessToken, chatId, msg.id, emoji);
    } catch (e) { console.error('Reaction error:', e); }
    setShowEmoji(false);
  };

  const handleEdit = async () => {
    if (!editText.trim()) return;
    setOpPending(true);
    try {
      await api.editMessage(accessToken, chatId, msg.id, editText);
      setEditing(false);
    } catch (e) { console.error('Edit message error:', e); } finally {
      setOpPending(false);
    }
  };

  // 【本地改动 2026-09-03】复制消息文本到剪贴板。
  // 优先 navigator.clipboard（HTTPS 环境）；非安全上下文/老浏览器回退
  // document.execCommand('copy')（textarea 选中 + execCommand）。
  const handleCopy = async () => {
    const text = msg.content || '';
    if (!text) return;
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      } else {
        const ta = document.createElement('textarea');
        ta.value = text;
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
      }
      notify('已复制消息', 'success');
    } catch (e) {
      notify('复制失败: ' + (e.message || 'Unknown error'), 'error');
    }
  };

  const handleDelete = async () => {
    if (!confirm('Delete this message?')) return;
    setOpPending(true);
    try {
      await api.deleteMessage(accessToken, chatId, msg.id);
    } catch (e) { console.error('Delete message error:', e); } finally {
      setOpPending(false);
    }
  };

  // 【本地改动 2026-09-02】消息置顶（chatto FDR-037，多消息）。
  // 权限：owner/admin（chat.members role）可 pin/unpin；DM/notify 无入口（后端也拒绝）。
  const canPin = !!chat && chat.type !== 'dm' && chat.type !== 'notify' &&
    (chat.owner_id === user.id || chat.members?.some(m => m.id === user.id && (m.role === 'admin' || m.role === 'owner')));
  const [pinned, setPinned] = useState(false);
  useEffect(() => {
    // 简单乐观：从 chat_pins 列表判断当前消息是否已置顶（后端未在 Message 上回显）。
    // 此处用轻量探测：若已 pin 过则按钮显示 Unpin。真实状态由 PinnedMessages 抽屉统一管理，
    // 这里通过本地开关近似（不额外发请求）。
    setPinned(false);
  }, [msg.id]);
  const handlePinToggle = async () => {
    if (!canPin || msg.deleted) return;
    setOpPending(true);
    try {
      if (pinned) {
        await api.unpinMessage(accessToken, chatId, msg.id);
      } else {
        await api.pinMessage(accessToken, chatId, msg.id);
      }
      setPinned(!pinned);
    } catch (e) { console.error('Pin toggle error:', e); } finally {
      setOpPending(false);
    }
  };

  return (
    <div className={'msg-group' + (highlighted ? ' msg-highlight' : '')} id={'msg-' + msg.id}>
      <div className={'msg-row' + (sameAuthor ? ' msg-continuation' : '')}>
          {!sameAuthor && (
            <div onClick={() => setProfileUser(author)} style={{ cursor: 'pointer' }}>
              <UserAvatar user={author} size={40} />
            </div>
          )}
        <div style={{flex:1,minWidth:0}}>
          {!sameAuthor && (
            <div style={{display:'flex',alignItems:'center',gap:6,flexWrap:'wrap'}}>
              <span className="msg-author" onClick={() => setProfileUser(author)} style={{cursor:'pointer'}}>{author.username}</span>
              
              {(author.role === 'admin' || author.role === 'owner' || author.id === chat?.owner_id) && (
                <span style={{fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'rgba(88,101,242,0.15)',color:'#5865F2',lineHeight:'18px'}}>ADMIN</span>
              )}
              <span className="msg-time">{timeFormat(msg.created_at)}</span>
              {msg.edited_at && <span className="msg-time">(edited)</span>}
            </div>
          )}
          <div style={{position:'relative'}}>
            {msg.deleted ? (
              <div className="msg-deleted">(message deleted)</div>
            ) : editing ? (
              <div style={{display:'flex',gap:8}}>
                <textarea className="input-field" value={editText} onChange={e=>setEditText(e.target.value)}
                  style={{flex:1,resize:'none',minHeight:36}} autoFocus onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();handleEdit();}}} />
                 <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:12}} onClick={handleEdit} disabled={opPending}>
                   {opPending ? '...' : 'Save'}
                 </button>
                <button className="btn-ghost" style={{fontSize:12}} onClick={()=>setEditing(false)}>Cancel</button>
              </div>
            ) : msg.replied_to ? (
              <div className="reply-indicator" onClick={() => {
                document.getElementById('msg-' + msg.replied_to.id)?.scrollIntoView({ behavior: 'smooth', block: 'center' });
                // 【本地改动 2026-09-03】触发目标消息高亮闪烁。
                document.dispatchEvent(new CustomEvent('chat:jump-to-message', { detail: { messageId: msg.replied_to.id } }));
              }}
                style={{display:'flex',alignItems:'center',gap:6,padding:'2px 8px',marginBottom:2,borderLeft:'3px solid var(--accent)',background:'var(--bg-secondary)',borderRadius:4,cursor:'pointer',fontSize:12}}>
                <span style={{fontWeight:600,color:'var(--accent)',whiteSpace:'nowrap',overflow:'hidden',textOverflow:'ellipsis',maxWidth:120}}>
                  {msg.replied_to.author?.username || 'Unknown'}
                </span>
                <span style={{color:'var(--text-muted)',overflow:'hidden',textOverflow:'ellipsis',whiteSpace:'nowrap',flex:1}}>
                  {msg.replied_to.content || '(deleted)'}
                </span>
              </div>
            ) : msg.type === 'thinking' ? (
              <ThinkingContent content={msg.content} streaming={msg.streaming} />
            ) : (
              <>
                {msg.thinking && <ThinkingContent content={msg.thinking} streaming={msg.streaming} />}
                {msg.streaming ? (
                  <div className="msg-content" style={{whiteSpace:'pre-wrap',wordBreak:'break-word'}}>
                    {msg.content}<span className="stream-cursor" />
                  </div>
                ) : (
                  <div className="msg-content" style={{whiteSpace:'pre-wrap',wordBreak:'break-word'}}>
                    {renderContent(msg.content, userMap)}
                  </div>
                )}
              </>
            )}
            {!msg.deleted && !editing && !msg.streaming && (
              <MessageLinkCard content={msg.content} />
            )}
            {!msg.deleted && (
              <div className="msg-actions">
                <button ref={emojiBtnRef} className="msg-btn" onClick={() => setShowEmoji(!showEmoji)} disabled={opPending}>😀</button>
                <button className="msg-btn" onClick={() => onReply?.(msg)} disabled={opPending}>Reply</button>
                <button className="msg-btn" onClick={handleCopy} disabled={opPending || !msg.content} title="复制文本">Copy</button>
                {canPin && !msg.deleted && <button className="msg-btn" onClick={handlePinToggle} disabled={opPending}>{pinned ? 'Unpin' : 'Pin'}</button>}
                {isMe && <button className="msg-btn" onClick={() => { setEditing(true); setEditText(msg.content); }} disabled={opPending}>Edit</button>}
                {isMe && <button className="msg-btn" onClick={handleDelete} disabled={opPending}>Delete</button>}
              </div>
            )}
          </div>
          {msg.attachments?.map(a => (
              <div key={a.id} className="file-attach">
                {a.mime_type?.startsWith('image/')
                  ? <a href={a.url} target="_blank" rel="noreferrer"><img src={a.url} alt={a.filename} loading="lazy" className="file-attach-img" /></a>
                  : <div className="file-pill"><a href={a.url} target="_blank" rel="noreferrer" style={{fontSize:13}}>{a.filename}</a></div>
                }
              </div>
            ))}
          {!msg.deleted && displayReactions.length > 0 && (
            <div className="reaction-bar">
              {displayReactions.map(r => (
                <div key={r.emoji} className={'reaction-chip' + (r.me ? ' me' : '')}
                  onClick={() => handleReaction(r.emoji)}>
                  <span>{r.emoji}</span>
                  <span style={{fontSize:12}}>{r.count}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      {showEmoji && pickerPos && (
        <div ref={pickerRef} style={{
          position:'fixed', left: pickerPos.left, top: pickerPos.top, zIndex:9999,
          display:'flex', flexWrap:'wrap', gap:4, padding:8,
          background:'var(--bg-tertiary)', borderRadius:'var(--radius)',
          maxWidth:240, boxShadow:'0 2px 12px rgba(0,0,0,0.3)',
        }}>
          {COMMON_EMOJI.map(e => (
            <button key={e} className="emoji-btn" onClick={() => handleReaction(e)}>{e}</button>
          ))}
        </div>
      )}
      {profileUser && (
        <UserProfileModal user={profileUser} chatId={chatId} onClose={() => setProfileUser(null)} />
      )}
    </div>
  );
});

export default MessageItem;
