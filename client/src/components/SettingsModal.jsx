import { useState } from 'react';
import ImagePreviewModal from './ImagePreviewModal';

export default function SettingsModal({ user, onClose, onSave }) {
  const [name, setName] = useState(user.username);
  const [saving, setSaving] = useState(false);
  const [previewUrl, setPreviewUrl] = useState(null);
  const [avatarError, setAvatarError] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(name);
    } catch (e) { console.error('Save settings error:', e); }
    setSaving(false);
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 style={{ fontSize: 18 }}>Settings</h3>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12, marginBottom: 16 }}>
          {user.avatar_url && !avatarError
            ? <img src={user.avatar_url} className="settings-avatar-img" alt="" onClick={() => setPreviewUrl(user.avatar_url)} style={{ cursor: 'pointer' }} onError={() => setAvatarError(true)} />
            : <div className="settings-avatar-placeholder" style={{ background: user.avatar_color }}>
              {user.username[0].toUpperCase()}
            </div>
          }
          {user.avatar_url ? (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', cursor: 'pointer' }} onClick={() => document.getElementById('avatar-file-input')?.click()}>Change avatar</div>
          ) : (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', cursor: 'pointer' }} onClick={() => document.getElementById('avatar-file-input')?.click()}>Click to upload</div>
          )}
          <input id="avatar-file-input" type="file" accept="image/*" style={{ display: 'none' }} />
        </div>
        <label className="form-label">Display Name</label>
        <input className="input-field" value={name} onChange={e => setName(e.target.value)}
          maxLength={32} autoFocus onKeyDown={e => e.key === 'Enter' && handleSave()} />
        <div style={{ display: 'flex', gap: 8, marginTop: 16, justifyContent: 'flex-end' }}>
          <button className="btn-ghost" onClick={onClose}>Cancel</button>
          <button className="btn btn-primary" style={{ padding: '8px 16px', fontSize: 13 }} onClick={handleSave} disabled={saving || !name.trim()}>
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}
