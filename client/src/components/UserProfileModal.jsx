import { useEffect, useState, useMemo } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import UserAvatar from './UserAvatar';

export default function UserProfileModal({ user: profileUser, onClose, chatId = undefined }) {
  const { user: me, accessToken } = useAuthStore();
  const { chats } = useChatStore();
  const isMe = profileUser.id === me.id;
  const chat = useMemo(() => chats.find(c => c.id === chatId), [chats, chatId]);

  const [name, setName] = useState(profileUser.username);
  const [color, setColor] = useState(profileUser.avatar_color || '');
  const [saving, setSaving] = useState(false);

  const COLORS = ['#5865F2','#ed4245','#faa61a','#23a559','#eb459f','#57F287','#00b0f4','#949cf7','#FF73FA','#fee75c'];

  useEffect(() => {
    const handler = (e) => { if (e.key === 'Escape') onClose?.(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const payload = { username: name, avatar_color: color || undefined };
      const avatarInput = document.getElementById('profile-avatar-input');
      const file = avatarInput instanceof HTMLInputElement ? avatarInput.files?.[0] : undefined;
      if (file) {
        const data = await api.uploadAvatar(accessToken, file);
        payload.avatar_url = data.url + '?v=' + Date.now();
      } else if (me.avatar_url) {
        payload.avatar_url = me.avatar_url;
      }
      const updated = await api.updateProfile(accessToken, payload);
      useAuthStore.getState().setUser(updated);
      onClose();
    } catch (e) { console.error('Save profile error:', e); }
    setSaving(false);
  };

  if (!profileUser) return null;
  const role = profileUser.role || (chat && profileUser.id === chat.owner_id ? 'owner' : 'member');

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" role="dialog" aria-modal="true" aria-label={isMe ? 'Settings' : 'User profile'} onClick={e => e.stopPropagation()} style={{ maxWidth: 360, textAlign: 'center' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8, margin: '8px 0' }}>
          <UserAvatar user={profileUser} size={isMe ? 80 : 64} />
          {isMe && (
            <>
              <div style={{ fontSize: 12, color: 'var(--text-muted)', cursor: 'pointer' }} onClick={() => document.getElementById('profile-avatar-input')?.click()}>
                {profileUser.avatar_url ? 'Change avatar' : 'Click to upload'}
              </div>
              <input id="profile-avatar-input" type="file" accept="image/*" style={{ display: 'none' }} />
            </>
          )}
        </div>

        {isMe && (
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'center', margin: '4px 0 0' }}>
            {COLORS.map(c => (
              <div key={c} onClick={() => setColor(c === color ? '' : c)}
                style={{ width: 22, height: 22, borderRadius: '50%', background: c, cursor: 'pointer', border: c === color ? '2px solid var(--text-primary)' : '2px solid transparent', boxSizing: 'border-box' }} />
            ))}
          </div>
        )}

        {isMe ? (
          <input className="input-field" value={name} onChange={e => setName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') handleSave(); }} style={{width:'100%'}} />
        ) : (
          <h3 style={{ margin: '8px 0 4px' }}>{profileUser.username}</h3>
        )}

        {!isMe && profileUser.status && (
          <span style={{ fontSize: 13, color: profileUser.status === 'online' ? 'var(--success)' : 'var(--text-muted)' }}>
            {profileUser.status === 'online' ? '● Online' : '○ Offline'}
          </span>
        )}

        <div style={{ marginTop: 16, fontSize: 14, textAlign: 'left' }}>
          {chatId && (
            <div style={{ padding: '4px 0', display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ color: 'var(--text-muted)' }}>Role</span>
              <span style={{ textTransform: 'capitalize' }}>{role || 'member'}</span>
            </div>
          )}
          {chatId && profileUser.last_seen && (
            <div style={{ padding: '4px 0', display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ color: 'var(--text-muted)' }}>Last seen</span>
              <span style={{ fontSize: 12 }}>{new Date(profileUser.last_seen).toLocaleString()}</span>
            </div>
          )}
          <div style={{ padding: '4px 0', display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ color: 'var(--text-muted)' }}>ID</span>
            <span style={{ fontSize: 12 }}>{profileUser.id}</span>
          </div>
        </div>

        {isMe && (
          <button className="btn-primary" style={{ width: '100%', marginTop: 16 }} onClick={handleSave} disabled={saving || !name.trim()}>
            {saving ? 'Saving...' : 'Save'}
          </button>
        )}
      </div>
    </div>
  );
}
