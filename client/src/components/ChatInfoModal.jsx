import { useMemo } from 'react';
import { useChatStore } from '../store/chat';

export default function ChatInfoModal({ chatId, onClose }) {
  const { chats } = useChatStore();
  const chat = chats.find(c => c.id === chatId);

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

        <Section title={`Owner`}>
          {owner && <MemberRow member={owner} />}
        </Section>

        {admins.length > 0 && (
          <Section title={`Admin — ${admins.length}`}>
            {admins.map(m => <MemberRow key={m.id} member={m} />)}
          </Section>
        )}

        <Section title={`Member — ${members.length}`}>
          {members.map(m => <MemberRow key={m.id} member={m} />)}
        </Section>
      </div>
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

function MemberRow({ member }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '3px 0', fontSize: 14 }}>
      <div className="msg-avatar" style={{ width: 26, height: 26, fontSize: 11, background: member.avatar_color || '#5865F2' }}>
        {member.username ? member.username[0].toUpperCase() : '?'}
      </div>
      <span>{member.username}</span>
    </div>
  );
}
