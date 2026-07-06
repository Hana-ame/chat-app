import { useState } from 'react';

export default function SettingsModal({ user, onClose, onSave }) {
  const [name, setName] = useState(user.username);
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave(name);
    } catch { }
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
          {user.avatar_url
            ? <img src={user.avatar_url} className="settings-avatar-img" alt="" onClick={() => document.getElementById('avatar-file-input')?.click()} style={{ cursor: 'pointer' }} />
            : <div className="settings-avatar-placeholder" style={{ background: user.avatar_color }}
              onClick={() => document.getElementById('avatar-file-input')?.click()}>
              {user.username[0].toUpperCase()}
            </div>
          }
          <div style={{ fontSize: 12, color: 'var(--text-muted)' }}>Click to upload</div>
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
    </div>
  );
}
