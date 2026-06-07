import { useState } from 'react';
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

export default function ChatList({ onSelectChat, activeId, onLogout }) {
  const { user, accessToken } = useAuthStore();
  const { chats, onlineUserIds } = useChatStore();
  const [showCreate, setShowCreate] = useState(false);
  const [newChatName, setNewChatName] = useState('');
  const [dmUserId, setDmUserId] = useState('');
  const [dmSearch, setDmSearch] = useState('');
  const [dmResults, setDmResults] = useState([]);
  const [showProfile, setShowProfile] = useState(false);

  const handleCreate = async () => {
    if (!newChatName.trim()) return;
    try {
      const data = await api.createChat(accessToken, newChatName, []);
      setShowCreate(false);
      setNewChatName('');
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
          <div style={{display:'flex',gap:8,marginTop:8}}>
            <button className="btn btn-primary" style={{padding:'4px 12px',fontSize:13}} onClick={handleCreate}>Create</button>
            <button className="btn-ghost" style={{fontSize:13}} onClick={() => setShowCreate(false)}>Cancel</button>
          </div>
        </div>
      )}

      <div className="sidebar-body">
        {chats.map(c => {
          const name = c.type === 'dm' ? getDMName(c, user.id) : c.name;
          const avatar = c.type === 'dm' ? (c.members?.find(m => m.id !== user.id)?.avatar_color || c.icon_color) : c.icon_color;
          const unread = c.unread_count || 0;
          return (
            <div key={c.id} className={'chat-item' + (c.id === activeId ? ' active' : '')}
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
              <div style={{display:'flex',flexDirection:'column',alignItems:'flex-end',gap:2}}>
                <div className="chat-item-time">{timeAgo(c.last_message_at)}</div>
                {unread > 0 && <div className="unread-badge">{unread}</div>}
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
