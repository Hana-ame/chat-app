import { useEffect, useState } from 'react';
import UserAvatar from './UserAvatar';
import ImagePreviewModal from './ImagePreviewModal';

export default function SettingsModal({ user, onClose, onSave }) {
  const [name, setName] = useState(user.username);
  const [saving, setSaving] = useState(false);
  const [previewUrl, setPreviewUrl] = useState(null);

  useEffect(() => {
    const handler = (e) => { if (e.key === 'Escape') onClose?.(); };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [onClose]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(name);
    } catch (e) { console.error('Save settings error:', e); }
    setSaving(false);
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-box" role="dialog" aria-modal="true" aria-label="Settings" onClick={e => e.stopPropagation()}>
        <div className="modal-header">
          <h3 style={{ fontSize: 18 }}>Settings</h3>
          <button className="btn-ghost" onClick={onClose}>✕</button>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12, marginBottom: 16 }}>
          <UserAvatar user={user} size={80} onClick={() => setPreviewUrl(user.avatar_url)} />
          {user.avatar_url ? (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', cursor: 'pointer' }} onClick={() => document.getElementById('avatar-file-input')?.click()}>Change avatar</div>
          ) : (
            <div style={{ fontSize: 12, color: 'var(--text-muted)', cursor: 'pointer' }} onClick={() => document.getElementById('avatar-file-input')?.click()}>Click to upload</div>
          )}
          <input id="avatar-file-input" type="file" accept="image/*" style={{ display: 'none' }} />
        </div>
        <label className="form-label">Display Name</label>
        <input className="input-field" value={name} onChange={e => setName(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') handleSave(); }} />
        <button className="btn-primary" style={{ width: '100%', marginTop: 16 }} onClick={handleSave} disabled={saving || !name.trim()}>
          {saving ? 'Saving...' : 'Save'}
        </button>
      </div>
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}
