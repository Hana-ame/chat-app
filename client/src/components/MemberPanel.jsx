import { useEffect, useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import UserProfileModal from './UserProfileModal';
import ImagePreviewModal from './ImagePreviewModal';

export default function MemberPanel({ chatId }) {
  const { user, accessToken } = useAuthStore();
  const { chats } = useChatStore();
  const ONLINE_THRESHOLD = 300000; // 5 min

  const isOnline = (m) => {
    if (!m.last_seen) return false;
    return Date.now() - new Date(m.last_seen).getTime() < ONLINE_THRESHOLD;
  };
  const [members, setMembers] = useState([]);
  const [profileUser, setProfileUser] = useState(null);
  const [previewUrl, setPreviewUrl] = useState(null);

  const chat = chats.find(c => c.id === chatId);

  useEffect(() => {
    if (chat?.members) setMembers(chat.members);
  }, [chat]);

  const removeUser = async (userId) => {
    if (!confirm('Kick this member?')) return;
    await api.removeMember(accessToken, chatId, userId);
    setMembers(prev => prev.filter(m => m.id !== userId));
  };

  return (
    <div className="members-panel">
      <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:12}}>
        Members — {chat?.member_count || members.length}
      </h4>

      {(() => {
        const isAdmin = m => m.role === 'admin' || m.id === chat.owner_id;
        return members.map(m => (
          <div key={m.id} style={{display:'flex',alignItems:'center',gap:8,padding:'4px 0',fontSize:14,cursor:'pointer'}}
            onClick={() => setProfileUser(m)}>
            <span className={'status-dot ' + (isOnline(m) ? 'online' : 'offline')} />
            {m.avatar_url
              ? <img src={m.avatar_url} style={{width:28,height:28,borderRadius:'50%',objectFit:'cover',flexShrink:0}} alt={m.username} onClick={e => { e.stopPropagation(); setPreviewUrl(m.avatar_url); }} />
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
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}
