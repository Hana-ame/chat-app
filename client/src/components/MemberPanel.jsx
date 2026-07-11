import { useEffect, useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import UserProfileModal from './UserProfileModal';

export default function MemberPanel({ chatId }) {
  const { user, accessToken } = useAuthStore();
  const { chats } = useChatStore();
  const ONLINE_THRESHOLD = 300000; // 5 min

  const isOnline = (m) => {
    if (!m.last_seen) return false;
    return Date.now() - new Date(m.last_seen).getTime() < ONLINE_THRESHOLD;
  };
  const [members, setMembers] = useState([]);
  const [adding, setAdding] = useState(false);
  const [search, setSearch] = useState('');
  const [results, setResults] = useState([]);
  const [profileUser, setProfileUser] = useState(null);

  const chat = chats.find(c => c.id === chatId);

  useEffect(() => {
    if (chat?.members) setMembers(chat.members);
  }, [chat]);

  const searchUsers = async (q) => {
    setSearch(q);
    if (q.length < 1) { setResults([]); return; }
    const data = await api.searchUsers(accessToken, q);
    setResults((data.users || []).filter(u => !members.find(m => m.id === u.id)));
  };

  const addUser = async (userId) => {
    await api.addMember(accessToken, chatId, userId);
    setAdding(false);
    setSearch('');
    setResults([]);
  };

  const removeUser = async (userId) => {
    if (!confirm('Kick this member?')) return;
    await api.removeMember(accessToken, chatId, userId);
  };

  return (
    <div className="members-panel">
      <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:12}}>
        Members — {members.length}
      </h4>
      {chat?.type !== 'dm' && (
        <button className="btn-ghost" style={{fontSize:13,marginBottom:8,width:'100%',textAlign:'left'}}
          onClick={() => setAdding(!adding)}>
          {adding ? '− Cancel' : '+ Add member'}
        </button>
      )}
      {adding && (
        <div style={{marginBottom:8}}>
          <input className="input-field" style={{fontSize:13,padding:'4px 8px'}} placeholder="Search users..."
            value={search} onChange={e=>searchUsers(e.target.value)} autoFocus />
          {results.map(u => (
            <div key={u.id} style={{display:'flex',alignItems:'center',gap:6,padding:'4px 0',cursor:'pointer',fontSize:13}}
              onClick={() => addUser(u.id)}>
              <span className="status-dot offline" />
              {u.username}
            </div>
          ))}
        </div>
      )}
      {(() => {
        const isAdmin = m => m.role === 'admin' || m.id === chat.owner_id;
        return members.map(m => (
          <div key={m.id} style={{display:'flex',alignItems:'center',gap:8,padding:'4px 0',fontSize:14,cursor:'pointer'}}
            onClick={() => setProfileUser(m)}>
            <span className={'status-dot ' + (isOnline(m) ? 'online' : 'offline')} />
            {m.avatar_url
              ? <img src={m.avatar_url} style={{width:28,height:28,borderRadius:'50%',objectFit:'cover',flexShrink:0}} alt={m.username} />
              : <div className="msg-avatar" style={{width:28,height:28,fontSize:11,background:m.avatar_color}}>{m.username[0]}</div>
            }
            <span>{m.username}</span>
            <div style={{flex:1}} />
            <div style={{width:66,height:28,position:'relative',flexShrink:0}}>
              {isAdmin(m) && <span style={{position:'absolute',right:22,top:'50%',transform:'translateY(-50%)',fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'rgba(88,101,242,0.15)',color:'#5865F2'}}>ADMIN</span>}
              {chat?.owner_id === user.id && m.id !== user.id && chat?.type !== 'dm' && (
                <button className="btn-ghost" style={{position:'absolute',right:0,top:'50%',transform:'translateY(-50%)',fontSize:12}} onClick={(e) => { e.stopPropagation(); removeUser(m.id); }}>×</button>
              )}
            </div>
          </div>
        ));
      })()}
      {profileUser && (
        <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} />
      )}
    </div>
  );
}
