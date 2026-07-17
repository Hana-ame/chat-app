import { useState } from 'react';
import { useChatStore } from '../store/chat';
import { useAuthStore } from '../store/auth';
import { useEscapeKey } from '../hooks/useEscapeKey';
import { useMembers } from '../hooks/useMembers';
import MemberList from './MemberList';
import UserProfileModal from './UserProfileModal';

function fmtTime(t) {
  if (!t) return '-';
  return new Date(t).toLocaleString();
}

export default function ChatInfoModal({ chatId, onClose }) {
  const { user } = useAuthStore();
  const { chats } = useChatStore();
  const { members } = useMembers(chatId);
  const [profileUser, setProfileUser] = useState(null);

  useEscapeKey(onClose);

  const chat = chats.find(c => c.id === chatId);
  if (!chat) return null;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" role="dialog" aria-modal="true" aria-label="Chat info" onClick={e => e.stopPropagation()} style={{ maxWidth: 400 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <h3 style={{ margin: 0 }}>Chat Info</h3>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>

        <div style={{ fontSize: 14, marginBottom: 16 }}>
          <div style={{ color: 'var(--text-muted)', fontSize: 12 }}>Name</div>
          <div>{chat.name || chat.id}</div>
        </div>

        <InfoRow label="ID"
          value={<span style={{cursor:'pointer',textDecoration:'underline dotted'}} onClick={() => { navigator.clipboard.writeText(chatId); }} title="Click to copy">{chatId}</span>} />

        <InfoRow label="Created at" value={fmtTime(chat.created_at)} />
        <InfoRow label="Last message" value={fmtTime(chat.last_message_at)} />

        <h4 style={{fontSize:12,textTransform:'uppercase',color:'var(--text-muted)',marginBottom:8}}>Members — {chat.member_count || 0}</h4>
        <MemberList
          members={members}
          chat={chat}
          currentUserId={user.id}
          onProfile={setProfileUser}
        />
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
