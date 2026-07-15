import { useEffect, useState } from 'react';
import UserAvatar from './UserAvatar';
import ImagePreviewModal from './ImagePreviewModal';

export default function UserProfileModal({ user: profileUser, onClose, chatId }) {
  const [previewUrl, setPreviewUrl] = useState(null);

  useEffect(() => {
    const handler = (e) => { if (e.key === 'Escape') onClose?.(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  if (!profileUser) return null;
  const role = profileUser.role;
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" role="dialog" aria-modal="true" aria-label="User profile" onClick={e => e.stopPropagation()} style={{ maxWidth: 360, textAlign: 'center' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>

        <div style={{ display: 'flex', justifyContent: 'center', margin: '8px 0' }}>
          <UserAvatar user={profileUser} size={64} onClick={() => setPreviewUrl(profileUser.avatar_url)} />
        </div>

        <h3 style={{ margin: '8px 0 4px' }}>{profileUser.username}</h3>

        {profileUser.status && (
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
          {profileUser.email && (
            <div style={{ padding: '4px 0', display: 'flex', justifyContent: 'space-between' }}>
              <span style={{ color: 'var(--text-muted)' }}>Email</span>
              <span>{profileUser.email}</span>
            </div>
          )}
          <div style={{ padding: '4px 0', display: 'flex', justifyContent: 'space-between' }}>
            <span style={{ color: 'var(--text-muted)' }}>ID</span>
            <span style={{ fontSize: 12 }}>{profileUser.id}</span>
          </div>
        </div>
      </div>
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}
