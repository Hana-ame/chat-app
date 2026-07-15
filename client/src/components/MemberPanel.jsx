import { useEffect, useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import UserAvatar from './UserAvatar';
import UserProfileModal from './UserProfileModal';
import ImagePreviewModal from './ImagePreviewModal';

export default function MemberPanel({ chatId }) {
  const { user, accessToken } = useAuthStore();
  const { chats, wsReady, mode, wsRequest } = useChatStore();
  const ONLINE_THRESHOLD = 300000; // 5 min

  const isOnline = (m) => {
    if (!m.last_seen) return false;
    return Date.now() - new Date(m.last_seen).getTime() < ONLINE_THRESHOLD;
  };
  const [members, setMembers] = useState([]);
  const [profileUser, setProfileUser] = useState(null);
  const [previewUrl, setPreviewUrl] = useState(null);

  useEffect(() => {
    if (!chatId || !accessToken) return;
    const fetch = () => {
      if (mode === 'ws' && wsReady) {
        wsRequest('list_members', { chat_id: chatId }).then(d => setMembers(d.members || [])).catch(() => notify('Failed to load members', 'error'));
      } else {
        api.listMembers(accessToken, chatId).then(d => setMembers(d.members || [])).catch(() => notify('Failed to load members', 'error'));
      }
    };
    fetch();
    const id = setInterval(fetch, 60000);
    return () => clearInterval(id);
  }, [chatId, accessToken, mode, wsReady]);

  const chat = chats.find(c => c.id === chatId);
  if (!chat) return null;

  const removeUser = async (userId) => {
    if (!confirm('Kick this member?')) return;
    setMembers(prev => prev.filter(m => m.id !== userId));
    try {
      await api.removeMember(accessToken, chatId, userId);
    } catch (e) {
      notify('Failed to remove member', 'error');
      setMembers(prev => [...prev, members.find(m => m.id === userId)].filter(Boolean));
    }
  };

  return (
    <div className="members-panel">
      <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:12}}>
        Members — {chat?.member_count || members.length}
      </h4>

      {(() => {
        const isAdmin = m => m.role === 'admin' || m.id === chat?.owner_id;
        return members.map(m => (
          <div key={m.id} style={{display:'flex',alignItems:'center',gap:8,padding:'4px 0',fontSize:14,cursor:'pointer'}}
            onClick={() => setProfileUser(m)}>
            <span className={'status-dot ' + (isOnline(m) ? 'online' : 'offline')} />
            <UserAvatar user={m} size={28} onClick={(e) => { e.stopPropagation(); setPreviewUrl(m.avatar_url); }} />
            <span>{m.username}</span>
            <div style={{flex:1}} />
            <div style={{width:66,height:28,position:'relative',flexShrink:0}}>
              {isAdmin(m) && <span style={{position:'absolute',right:22,top:'50%',transform:'translateY(-50%)',fontSize:10,padding:'0 5px',borderRadius:3,fontWeight:500,background:'var(--accent-bg)',color:'var(--accent)'}}>ADMIN</span>}
              {chat?.owner_id === user.id && m.id !== user.id && chat?.type !== 'dm' && (
                <button className="btn-ghost" style={{position:'absolute',right:0,top:'50%',transform:'translateY(-50%)',fontSize:12}} onClick={(e) => { e.stopPropagation(); removeUser(m.id); }}>×</button>
              )}
            </div>
          </div>
        ));
      })()}
      {profileUser && (
        <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} chatId={chatId} />
      )}
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}
