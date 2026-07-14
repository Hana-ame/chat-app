export default function CreateGroupForm({ name, visibility, onVisibilityChange, onNameChange, onNameKeyDown, onCreate, onCancel }) {
  return (
    <div>
      <input className="input-field" placeholder="Group name..." value={name} autoFocus
        onChange={onNameChange} onKeyDown={onNameKeyDown} style={{ fontSize: 14, padding: '8px 10px' }} />
      <div style={{ display: 'flex', gap: 6, marginTop: 8 }}>
        {['private', 'unlisted', 'public'].map(v => (
          <button key={v} type="button"
            onClick={() => onVisibilityChange(v)}
            style={{
              flex: 1, padding: '8px 6px', fontSize: 13, cursor: 'pointer', minHeight: 36,
              border: '1px solid var(--border)',
              borderRadius: 'var(--radius)',
              background: visibility === v ? 'var(--accent)' : 'var(--bg-secondary)',
              color: visibility === v ? '#fff' : 'var(--text-primary)',
              fontWeight: visibility === v ? 600 : 400,
            }}>
            {v === 'private' ? '🔒 Private' : v === 'unlisted' ? '🔗 Unlisted' : '🌍 Public'}
          </button>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
        <button className="btn btn-primary" style={{ flex: 1, padding: '8px 12px', fontSize: 14, minHeight: 36 }} onClick={onCreate}>Create</button>
        <button className="btn-ghost" style={{ flex: 1, padding: '8px 12px', fontSize: 14, minHeight: 36 }} onClick={onCancel}>Cancel</button>
      </div>
    </div>
  );
}
