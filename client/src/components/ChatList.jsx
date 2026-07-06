import { useState, useEffect, useRef } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { generateDummyData } from '../dev/dummy';

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
  const [newChatVisibility, setNewChatVisibility] = useState('private');
  const [dmUserId, setDmUserId] = useState('');
  const [dmSearch, setDmSearch] = useState('');
  const [dmResults, setDmResults] = useState([]);
  const [showProfile, setShowProfile] = useState(false);
  const [chatSearch, setChatSearch] = useState('');
  const [publicResults, setPublicResults] = useState(null);
  const [publicSearching, setPublicSearching] = useState(false);

  const joinAction = chatSearch.trim() && /^\d{1,2}-\d{1,2}$/.test(chatSearch.trim()) ? 'join'
    : chatSearch.trim() && /^\d+$/.test(chatSearch.trim()) ? 'join'
    : chatSearch.trim() && /^(join|create)\s/i.test(chatSearch.trim()) ? chatSearch.trim().startsWith('join') ? 'join' : 'create'
    : null;
  const [contextMenu, setContextMenu] = useState(null);
  const [showSettings, setShowSettings] = useState(false);
  const [settingsName, setSettingsName] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!contextMenu) return;
    const close = () => setContextMenu(null);
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [contextMenu]);

  const handleCreate = async () => {
    if (!newChatName.trim()) return;
    try {
      const data = await api.createChat(accessToken, newChatName, [], newChatVisibility);
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

  const searchPublic = async (q) => {
    if (!q.trim()) { setPublicResults(null); setPublicSearching(false); return; }
    setPublicSearching(true);
    try {
      const data = await api.listPublicChats(accessToken);
      const all = data.chats || [];
      const lower = q.toLowerCase();
      const matched = all.filter(c => c.name?.toLowerCase().includes(lower) || c.id.toLowerCase().includes(lower));
      setPublicResults(matched);
    } catch {}
    setPublicSearching(false);
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

  const handleSaveSettings = async () => {
    setSaving(true);
    try {
      const payload = { username: settingsName };
      const file = document.getElementById('avatar-file-input')?.files?.[0];
      if (file) {
        const data = await api.uploadAvatar(accessToken, file);
        payload.avatar_url = data.url;
      }
      const updated = await api.updateProfile(accessToken, payload);
      useAuthStore.getState().setUser(updated);
      setShowSettings(false);
    } catch (e) { alert(e.message); }
    setSaving(false);
  };

  const joinChatByID = async (chatId) => {
    await api.joinChat(accessToken, chatId);
    setJoinById('');
    const data = await api.listChats(accessToken);
    useChatStore.getState().setChats(data.chats || []);
  };

  const handleGenerateDummy = () => {
    const data = generateDummyData({ chatCount: 10, msgPerChat: 100 });
    useChatStore.setState(data);
    if (data.chats[0]) onSelectChat(data.chats[0].id);
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

      <div style={{padding:'4px 8px',borderBottom:'1px solid var(--border)'}}>
        <input className="input-field" placeholder="Search chats or public channels..." value={chatSearch}
          onChange={e => {
            const v = e.target.value;
            setChatSearch(v);
            if (v.trim()) searchPublic(v);
            else setPublicResults(null);
          }}
          style={{fontSize:14,padding:'8px 10px'}} />
        {joinAction && (
          <div style={{display:'flex',gap:6,marginTop:6}}>
            {joinAction === 'join' && (
              <button className="btn" style={{flex:1,padding:'8px 12px',fontSize:14,background:'var(--accent)',color:'#fff',borderRadius:'var(--radius)'}}
                onClick={async () => { try { await joinChatByID(chatSearch.trim()); setChatSearch(''); setPublicResults(null); } catch(e) { alert(e.message); } }}>
                Join #{chatSearch.trim()}
              </button>
            )}
            {joinAction === 'create' && (
              <button className="btn" style={{flex:1,padding:'8px 12px',fontSize:14,background:'var(--accent)',color:'#fff',borderRadius:'var(--radius)'}}
                onClick={() => { setNewChatName(chatSearch.trim()); setShowCreate(true); }}>
                Create &ldquo;{chatSearch.trim()}&rdquo;
              </button>
            )}
          </div>
        )}
      </div>

      {dmSearch !== '' && (
        <div style={{padding:'8px 12px',borderBottom:'1px solid var(--border)'}}>
          <input className="input-field" placeholder="Search users..." value={dmSearch}
            onChange={e => searchUser(e.target.value)} autoFocus />
          {dmResults.map(u => (
            <div key={u.id} style={{display:'flex',alignItems:'center',gap:8,padding:'6px 0',cursor:'pointer'}}
              onClick={() => { setDmUserId(u.id); handleDM(); }}>
              {u.avatar_url
                ? <img src={u.avatar_url} style={{width:32,height:32,borderRadius:'50%',objectFit:'cover',flexShrink:0}} alt={u.username} />
                : <span className="msg-avatar" style={{width:32,height:32,fontSize:12,background:u.avatar_color}}>{u.username[0]}</span>
              }
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
          <div style={{display:'flex',gap:6,marginTop:6,fontSize:12}}>
            {['private','unlisted','public'].map(v => (
              <label key={v} style={{display:'flex',alignItems:'center',gap:3,cursor:'pointer'}}>
                <input type="radio" name="visibility" value={v}
                  checked={newChatVisibility === v}
                  onChange={() => setNewChatVisibility(v)} />
                {v}
              </label>
            ))}
          </div>
          <div style={{display:'flex',gap:8,marginTop:8}}>
            <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:13}} onClick={handleCreate}>Create</button>
            <button className="btn-ghost" style={{fontSize:13}} onClick={() => setShowCreate(false)}>Cancel</button>
          </div>
        </div>
      )}

      <div className="sidebar-body">
        {publicSearching && (
          <div style={{padding:24,textAlign:'center',color:'var(--text-muted)',fontSize:13}}>
            搜索中...
          </div>
        )}
        {publicResults !== null && !publicSearching && publicResults.length > 0 && (
          <>
            <div style={{padding:'4px 12px',fontSize:11,color:'var(--text-muted)',textTransform:'uppercase'}}>
              Public Channels
            </div>
            {publicResults.map(c => (
              <div key={c.id} className="chat-item" onClick={() => handleJoinPublic(c.id)}>
                <div className="chat-item-avatar" style={{background:c.icon_color}}>
                  {c.name ? c.name[0].toUpperCase() : '?'}
                </div>
                <div className="chat-item-info">
                  <div className="chat-item-name">{c.name}</div>
                  <div className="chat-item-preview">{c.members?.length || 0} members</div>
                </div>
              </div>
            ))}
          </>
        )}
        {publicResults !== null && !publicSearching && publicResults.length === 0 && (
          <div style={{padding:24,textAlign:'center',color:'var(--text-muted)',fontSize:13}}>
            无结果
          </div>
        )}

        {chats.filter(c => {
          if (!chatSearch.trim()) return true;
          const q = chatSearch.toLowerCase();
          const name = c.type === 'dm' ? getDMName(c, user.id) : c.name || '';
          return name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q);
        }).map(c => {
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
                {unread > 0 ? <div className="unread-badge">{unread}</div> : <div style={{height:18}} />}
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
          {user.avatar_url
            ? <img src={user.avatar_url} className="user-avatar-img" alt="" />
            : <div className="chat-item-avatar" style={{width:32,height:32,fontSize:13,background:user.avatar_color}}>
                {user.username[0].toUpperCase()}
              </div>
          }
          <div style={{flex:1,minWidth:0}}>
            <div style={{fontSize:14,fontWeight:600}}>{user.username}</div>
            <div style={{fontSize:11,color:'var(--text-muted)'}}>Online</div>
          </div>
          <button className="btn-ghost" onClick={(e)=>{e.stopPropagation(); setShowSettings(true); setSettingsName(user.username);}} title="Settings">⚙</button>
          <button className="btn-ghost" onClick={(e)=>{e.stopPropagation();onLogout();}}>↪</button>
        </div>
        <div style={{marginTop:4,borderTop:'1px solid var(--border)',paddingTop:4}}>
          <button className="btn-ghost" style={{fontSize:11,width:'100%'}} onClick={handleGenerateDummy}>🧪 Generate test data</button>
        </div>
      </div>

      {showSettings && (
        <div className="modal-overlay" onClick={() => setShowSettings(false)}>
          <div className="modal-box" onClick={e => e.stopPropagation()}>
            <div className="modal-header">
              <h3 style={{fontSize:18}}>Settings</h3>
              <button className="btn-ghost" onClick={() => setShowSettings(false)}>✕</button>
            </div>
            <div style={{display:'flex',flexDirection:'column',alignItems:'center',gap:12,marginBottom:16}}>
              {user.avatar_url
                ? <img src={user.avatar_url} className="settings-avatar-img" alt="" onClick={() => document.getElementById('avatar-file-input')?.click()} style={{cursor:'pointer'}} />
                : <div className="settings-avatar-placeholder" style={{background:user.avatar_color}}
                    onClick={() => document.getElementById('avatar-file-input')?.click()}>
                    {user.username[0].toUpperCase()}
                  </div>
              }
              <div style={{fontSize:12,color:'var(--text-muted)'}}>Click to upload</div>
              <input id="avatar-file-input" type="file" accept="image/*" style={{display:'none'}} />
            </div>
            <label className="form-label">Display Name</label>
            <input className="input-field" value={settingsName} onChange={e => setSettingsName(e.target.value)}
              maxLength={32} autoFocus onKeyDown={e => e.key === 'Enter' && handleSaveSettings()} />
            <div style={{display:'flex',gap:8,marginTop:16,justifyContent:'flex-end'}}>
              <button className="btn-ghost" onClick={() => setShowSettings(false)}>Cancel</button>
              <button className="btn btn-primary" style={{padding:'8px 16px',fontSize:13}} onClick={handleSaveSettings} disabled={saving || !settingsName.trim()}>
                {saving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
