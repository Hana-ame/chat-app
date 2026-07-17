import { useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import { notify } from '../store/notification';
import { useMembers } from '../hooks/useMembers';
import MemberList from './MemberList';
import UserProfileModal from './UserProfileModal';

export default function MemberPanel({ chatId }) {
  const { user, accessToken } = useAuthStore();
  const { chats } = useChatStore();
  const { members, refetch, setLocalMembers } = useMembers(chatId);
  const [profileUser, setProfileUser] = useState(null);

  const chat = chats.find(c => c.id === chatId);
  if (!chat) return null;

  const removeUser = async (userId) => {
    if (!confirm('Kick this member?')) return;
    const removed = members.find(m => m.id === userId);
    setLocalMembers(prev => prev.filter(m => m.id !== userId));
    try {
      await api.removeMember(accessToken, chatId, userId);
    } catch {
      notify('Failed to remove member');
      if (removed) setLocalMembers(prev => [...prev, removed]);
    }
  };

  return (
    <div className="members-panel">
      <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:12}}>
        Members — {chat?.member_count || members.length}
      </h4>

      <MemberList
        members={members}
        chat={chat}
        currentUserId={user.id}
        onProfile={setProfileUser}
        onKick={removeUser}
      />

      {profileUser && (
        <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} chatId={chatId} />
      )}
    </div>
  );
}
