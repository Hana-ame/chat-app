import { useState, useEffect, useRef } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';

function timeAgo(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = now - d;
  if (diff < 60e3) return 'now';
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}

function getDMName(chat, currentUserId) {
  const other = chat.members?.find(m => m.id !== currentUserId);
  return other ? other.username : 'Unknown';
}

const MODES = [
  { key: 'ws', label: 'WS' },
  { key: 'sse', label: 'SSE' },
  { key: 'poll', label: 'Poll' },
];

export default function ChatList({ onSelectChat, activeId, onLogout }) {
  const { user, accessToken } = useAuthStore();
  const { chats, onlineUserIds, mode, setMode, pinChat, unpinChat } = useChatStore();
  const [showCreate, setShowCreate] = useState(false);
  const [newChatName, setNewChatName] = useState('');
  const [newChatPublic, setNewChatPublic] = useState(false);
  const [dmUserId, setDmUserId] = useState('');
  const [dmSearch, setDmSearch] = useState('');
  const [dmResults, setDmResults] = useState([]);
  const [showProfile, setShowProfile] = useState(false);
  const [showPublic, setShowPublic] = useState(false);
  const [publicChats, setPublicChats] = useState([]);
  const [contextMenu, setContextMenu] = useState(null);

  useEffect(() => {
    if (!contextMenu) return;
    const close = () => setContextMenu(null);
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [contextMenu]);

  const handleCreate = async () => {
    if (!newChatName.trim()) return;
    try {
      const data = await api.createChat(accessToken, newChatName, [], newChatPublic ? 'public' : 'private');
      setShowCreate(false);
      setNewChatName('');
      setNewChatPublic(false);
      onSelectChat(data.id);
    } catch (e) { alert(e.message); }
  };

  const handleDM = async () => {
    if (!dmUserId) return;
    try {
      const data = await api.createDM(accessToken, dmUserId);
      setDmUserId('');
      setDmSearch('');
      setDmResults([]);
      onSelectChat(data.id);
    } catch (e) { alert(e.message); }
  };

  const searchUser = async (q) => {
    setDmSearch(q);
    if (q.length < 1) { setDmResults([]); return; }
    try {
      const data = await api.searchUsers(accessToken, q);
      setDmResults(data.users || []);
    } catch {}
  };

  const loadPublic = async () => {
    if (showPublic) { setShowPublic(false); return; }
    try {
      const data = await api.listPublicChats(accessToken);
      setPublicChats(data.chats || []);
      setShowPublic(true);
    } catch (e) { alert(e.message); }
  };

  const handleJoinPublic = async (chatId) => {
    try {
      await api.joinChat(accessToken, chatId);
      const data = await api.listChats(accessToken);
      useChatStore.getState().setChats(data.chats || []);
      onSelectChat(chatId);
      setShowPublic(false);
    } catch (e) { alert(e.message); }
  };

  const handlePin = async (e, chatId, pinned) => {
    e.stopPropagation();
    try {
      if (pinned) await unpinChat(accessToken, chatId);
      else await pinChat(accessToken, chatId);
    } catch {}
    setContextMenu(null);
  };

  const handleContextMenu = (e, chatId) => {
    e.stopPropagation();
    setContextMenu(chatId === contextMenu ? null : chatId);
  };

  const handleDeleteChat = async (e, chatId) => {
    e.stopPropagation();
    if (!confirm('Delete this chat?')) return;
    try {
      await api.deleteChat(accessToken, chatId);
    } catch {}
    setContextMenu(null);
  };

  const isOnline = (uid) => onlineUserIds.includes(uid);

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h3 style={{fontSize:15,fontWeight:700}}>WebChat</h3>
        <div style={{display:'flex',gap:4}}>
          <button className="btn-ghost" title="Create Group" onClick={() => setShowCreate(true)}>+</button>
          <button className="btn-ghost" title="New DM" onClick={() => searchUser('')} style={{fontWeight:700}}>@</button>
        </div>
      </div>

      <div style={{display:'flex',gap:2,padding:'4px 8px',borderBottom:'1px solid var(--border)'}}>
        {MODES.map(m => (
          <button key={m.key} className={'btn-ghost' + (mode === m.key ? ' active-mode' : '')}
            style={{flex:1,fontSize:11,padding:'2px 4px',borderRadius:4}}
            onClick={() => setMode(m.key)}>{m.label}</button>
        ))}
      </div>

      {dmSearch !== '' && (
        <div style={{padding:'8px 12px',borderBottom:'1px solid var(--border)'}}>
          <input className="input-field" placeholder="Search users..." value={dmSearch}
            onChange={e => searchUser(e.target.value)} autoFocus />
          {dmResults.map(u => (
            <div key={u.id} style={{display:'flex',alignItems:'center',gap:8,padding:'6px 0',cursor:'pointer'}}
              onClick={() => { setDmUserId(u.id); handleDM(); }}>
              <span className="msg-avatar" style={{width:32,height:32,fontSize:12,background:u.avatar_color}}>{u.username[0]}</span>
              <span>{u.username}</span>
            </div>
          ))}
          {dmResults.length === 0 && dmSearch.length > 1 && (
            <div style={{fontSize:13,color:'var(--text-muted)',padding:'8px 0'}}>No users found</div>
          )}
        </div>
      )}

      {showCreate && (
        <div style={{padding:'8px 12px',borderBottom:'1px solid var(--border)'}}>
          <input className="input-field" placeholder="Group name..." value={newChatName}
            onChange={e => setNewChatName(e.target.value)} autoFocus
            onKeyDown={e => e.key === 'Enter' && handleCreate()} />
          <label style={{display:'flex',alignItems:'center',gap:6,marginTop:6,fontSize:12,cursor:'pointer'}}>
            <input type="checkbox" checked={newChatPublic} onChange={e => setNewChatPublic(e.target.checked)} />
            Public group
          </label>
          <div style={{display:'flex',gap:8,marginTop:8}}>
            <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:13}} onClick={handleCreate}>Create</button>
            <button className="btn-ghost" style={{fontSize:13}} onClick={() => setShowCreate(false)}>Cancel</button>
          </div>
        </div>
      )}

      <div className="sidebar-body">
        <div style={{padding:'4px 12px',cursor:'pointer',display:'flex',alignItems:'center',gap:4}}
          onClick={loadPublic}>
          <span style={{fontSize:12,color:'var(--text-muted)',flex:1}}>PUBLIC GROUPS</span>
          <span style={{fontSize:14,color:'var(--text-muted)'}}>{showPublic ? '▾' : '▸'}</span>
        </div>

        {showPublic && publicChats.map(c => {
          const memberCount = c.members?.length || 0;
          return (
            <div key={c.id} className="chat-item" onClick={() => handleJoinPublic(c.id)}>
              <div className="chat-item-avatar" style={{background:c.icon_color}}>
                {c.name ? c.name[0].toUpperCase() : '?'}
              </div>
              <div className="chat-item-info">
                <div className="chat-item-name">{c.name}</div>
                <div className="chat-item-preview">{memberCount} member{memberCount !== 1 ? 's' : ''}</div>
              </div>
            </div>
          );
        })}

        {chats.map(c => {
          const name = c.type === 'dm' ? getDMName(c, user.id) : c.name;
          const avatar = c.type === 'dm' ? (c.members?.find(m => m.id !== user.id)?.avatar_color || c.icon_color) : c.icon_color;
          const unread = c.unread_count || 0;
          return (
            <div key={c.id} className={'chat-item' + (c.id === activeId ? ' active' : '') + (c.pinned ? ' pinned' : '') + (c.visibility === 'public' ? ' public' : '')}
              onClick={() => onSelectChat(c.id)}>
              <div className="chat-item-avatar" style={{background:avatar}}>
                {name ? name[0].toUpperCase() : '?'}
              </div>
              <div className="chat-item-info">
                <div className="chat-item-name">{name || getDMName(c, user.id)}</div>
                <div className="chat-item-preview">
                  {c.last_message ? (c.last_message.deleted ? '(message deleted)' : c.last_message.author?.username + ': ' + c.last_message.content) : ''}
                </div>
              </div>
              <div className="chat-item-meta">
                <div className="chat-item-time">{timeAgo(c.last_message_at)}</div>
                {unread > 0 && <div className="unread-badge">{unread}</div>}
                <div className="chat-item-menu-wrap">
                  <button className="btn-ghost chat-item-menu-btn" title="More"
                    onClick={(e) => handleContextMenu(e, c.id)}>⋮</button>
                  {contextMenu === c.id && (
                    <div className="context-menu">
                      {c.pinned
                        ? <button className="context-menu-item" onClick={(e) => handlePin(e, c.id, true)}>Unpin</button>
                        : <button className="context-menu-item" onClick={(e) => handlePin(e, c.id, false)}>Pin</button>}
                      {c.owner_id === user.id && (
                        <button className="context-menu-item danger" onClick={(e) => handleDeleteChat(e, c.id)}>Delete</button>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
        {chats.length === 0 && (
          <div style={{padding:24,textAlign:'center',color:'var(--text-muted)',fontSize:14}}>
            No conversations yet. Create a group or DM someone!
          </div>
        )}
      </div>

      <div className="sidebar-footer">
        <div style={{display:'flex',alignItems:'center',gap:8,cursor:'pointer'}}
          onClick={() => setShowProfile(!showProfile)}>
          <div className="chat-item-avatar" style={{width:32,height:32,fontSize:13,background:user.avatar_color}}>
            {user.username[0].toUpperCase()}
          </div>
          <div style={{flex:1,minWidth:0}}>
            <div style={{fontSize:14,fontWeight:600}}>{user.username}</div>
            <div style={{fontSize:11,color:'var(--text-muted)'}}>Online</div>
          </div>
          <button className="btn-ghost" onClick={(e)=>{e.stopPropagation();onLogout();}}>↪</button>
        </div>
      </div>
    </div>
  );
}
