export default function DmSearchPanel({ query, results, onSearch, onSelect }) {
  return (
    <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)' }}>
      <input className="input-field" placeholder="Search users..." value={query}
        onChange={e => onSearch(e.target.value)} autoFocus />
      {results.map(u => (
        <div key={u.id} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', cursor: 'pointer' }}
          onClick={() => onSelect(u)}>
          {u.avatar_url
            ? <img src={u.avatar_url} style={{ width: 32, height: 32, borderRadius: '50%', objectFit: 'cover', flexShrink: 0 }} alt={u.username} />
            : <span className="msg-avatar" style={{ width: 32, height: 32, fontSize: 12, background: u.avatar_color }}>{u.username[0]}</span>
          }
          <span>{u.username}</span>
        </div>
      ))}
      {results.length === 0 && query.length > 1 && (
        <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '8px 0' }}>No users found</div>
      )}
    </div>
  );
}
