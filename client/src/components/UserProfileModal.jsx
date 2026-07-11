import { useState } from 'react';
import ImagePreviewModal from './ImagePreviewModal';

export default function UserProfileModal({ user: profileUser, onClose }) {
  const [previewUrl, setPreviewUrl] = useState(null);
  const [avatarError, setAvatarError] = useState(false);
  if (!profileUser) return null;
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" onClick={e => e.stopPropagation()} style={{ maxWidth: 360, textAlign: 'center' }}>
        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>

        {profileUser.avatar_url && !avatarError
          ? <img src={profileUser.avatar_url} alt="" style={{ width: 64, height: 64, borderRadius: '50%', objectFit: 'cover', margin: '8px auto', cursor: 'pointer' }} onClick={() => setPreviewUrl(profileUser.avatar_url)} onError={() => setAvatarError(true)} />
          : <div className="msg-avatar" style={{ width: 64, height: 64, fontSize: 28, background: profileUser.avatar_color || '#5865F2', margin: '8px auto' }}>
              {profileUser.username ? profileUser.username[0].toUpperCase() : '?'}
            </div>
        }

        <h3 style={{ margin: '8px 0 4px' }}>{profileUser.username}</h3>

        {profileUser.status && (
          <span style={{ fontSize: 13, color: profileUser.status === 'online' ? '#23a559' : 'var(--text-muted)' }}>
            {profileUser.status === 'online' ? '● Online' : '○ Offline'}
          </span>
        )}

        <div style={{ marginTop: 16, fontSize: 14, textAlign: 'left' }}>
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
