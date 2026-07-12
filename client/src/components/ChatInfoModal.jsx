import { useEffect, useState } from 'react';
import { useChatStore } from '../store/chat';
import { useAuthStore } from '../store/auth';
import { api } from '../api/client';
import UserProfileModal from './UserProfileModal';

function fmtTime(t) {
  if (!t) return '-';
  return new Date(t).toLocaleString();
}

export default function ChatInfoModal({ chatId, onClose }) {
  const { accessToken } = useAuthStore();
  const { chats } = useChatStore();
  const chat = chats.find(c => c.id === chatId);
  const [members, setMembers] = useState([]);
  const [profileUser, setProfileUser] = useState(null);
  if (!chat) return null;

  const isAdmin = m => m.role === 'admin' || m.id === chat.owner_id;

  useEffect(() => {
    if (!chatId || !accessToken) return;
    api.listMembers(accessToken, chatId).then(d => setMembers(d.members || [])).catch(() => {});
  }, [chatId, accessToken]);

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" onClick={e => e.stopPropagation()} style={{ maxWidth: 400 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <h3 style={{ margin: 0 }}>Chat Info</h3>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>

        <div style={{ fontSize: 14, marginBottom: 16 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: 12 }}>Name</div>
          <div>{chat.name || chat.id}</div>
        </div>

        <InfoRow label="Created at" value={fmtTime(chat.created_at)} />
        <InfoRow label="Last message" value={fmtTime(chat.last_message_at)} />

        <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:8}}>Members — {chat.member_count || 0}</h4>
        {members.map(m => (
          <div key={m.id} onClick={() => setProfileUser(m)} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 0', fontSize: 14, cursor: 'pointer' }}>
            <div className="msg-avatar" style={{ width: 26, height: 26, fontSize: 11, background: m.avatar_color || '#5865F2' }}>
              {m.username ? m.username[0].toUpperCase() : '?'}
            </div>
            <span>{m.username}</span>
            <div style={{flex:1}} />
            <div style={{width:44,height:26,position:'relative',flexShrink:0}}>
              {isAdmin(m) && <span style={{position:'absolute',right:0,top:'50%',transform:'translateY(-50%)',fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'rgba(88,101,242,0.15)',color:'#5865F2'}}>ADMIN</span>}
            </div>
          </div>
        ))}
      </div>
      {profileUser && (
        <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} />
      )}
    </div>
  );
}

function InfoRow({ label, value }) {
  return (
    <div style={{ fontSize: 13, marginBottom: 8, display: 'flex', justifyContent: 'space-between' }}>
      <span style={{ color: 'var(--text-muted)' }}>{label}</span>
      <span>{value}</span>
    </div>
  );
}
