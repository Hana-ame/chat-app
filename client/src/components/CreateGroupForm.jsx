export default function CreateGroupForm({ name, visibility, onVisibilityChange, onNameChange, onNameKeyDown, onCreate, onCancel }) {
  return (
    <div>
      <input className="input-field" placeholder="Group name..." value={name} autoFocus
        onChange={onNameChange} onKeyDown={onNameKeyDown} style={{ fontSize: 14, padding: '8px 10px' }} />
      <div style={{ display: 'flex', gap: 6, marginTop: 6, fontSize: 12 }}>
        {['private', 'unlisted', 'public'].map(v => (
          <label key={v} style={{ display: 'flex', alignItems: 'center', gap: 3, cursor: 'pointer' }}>
            <input type="radio" name="visibility" value={v}
              checked={visibility === v}
              onChange={() => onVisibilityChange(v)} />
            {v}
          </label>
        ))}
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
        <button className="btn btn-primary" style={{ padding: '4px 12px', fontSize: 13 }} onClick={onCreate}>Create</button>
        <button className="btn-ghost" style={{ fontSize: 13 }} onClick={onCancel}>Cancel</button>
      </div>
    </div>
  );
}
