import { useMemo, useState } from 'react';
import { useChatStore } from '../store/chat';
import UserProfileModal from './UserProfileModal';

function fmtTime(t) {
  if (!t) return '-';
  return new Date(t).toLocaleString();
}

export default function ChatInfoModal({ chatId, onClose }) {
  const { chats } = useChatStore();
  const chat = chats.find(c => c.id === chatId);

  const [profileUser, setProfileUser] = useState(null);

  const { owner, admins, members } = useMemo(() => {
    if (!chat?.members) return { owner: null, admins: [], members: [] };
    const o = chat.members.find(m => m.id === chat.owner_id) || null;
    const a = chat.members.filter(m => m.role === 'admin' && m.id !== chat.owner_id);
    const m = chat.members.filter(m => m.id !== chat.owner_id && m.role !== 'admin');
    return { owner: o, admins: a, members: m };
  }, [chat]);

  if (!chat) return null;

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

        <Section title={`Owner`}>
          {owner && <MemberRow member={owner} onProfile={setProfileUser} />}
        </Section>

        {admins.length > 0 && (
          <Section title={`Admin — ${admins.length}`}>
            {admins.map(m => <MemberRow key={m.id} member={m} onProfile={setProfileUser} />)}
          </Section>
        )}

        <Section title={`Member — ${members.length}`}>
          {members.map(m => <MemberRow key={m.id} member={m} onProfile={setProfileUser} />)}
        </Section>
      </div>
      {profileUser && (
        <UserProfileModal user={profileUser} onClose={() => setProfileUser(null)} />
      )}
    </div>
  );
}

function Section({ title, children }) {
  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ fontSize: 11, textTransform: 'uppercase', color: 'var(--text-muted)', marginBottom: 6, letterSpacing: 0.5 }}>
        {title}
      </div>
      {children}
    </div>
  );
}

function MemberRow({ member, onProfile }) {
  return (
    <div onClick={() => onProfile?.(member)} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 0', fontSize: 14, cursor: 'pointer' }}>
      <div className="msg-avatar" style={{ width: 26, height: 26, fontSize: 11, background: member.avatar_color || '#5865F2' }}>
        {member.username ? member.username[0].toUpperCase() : '?'}
      </div>
      <span>{member.username}</span>
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
