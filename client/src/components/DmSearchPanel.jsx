import UserAvatar from './UserAvatar';

export default function DmSearchPanel({ query, results, onSearch, onSelect }) {
  return (
    <div style={{ padding: '8px 12px', borderBottom: '1px solid var(--border)' }}>
      <input className="input-field" placeholder="Search users..." value={query}
        onChange={e => onSearch(e.target.value)} autoFocus />
      {results.map(u => (
        <div key={u.id} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '6px 0', cursor: 'pointer' }}
          onClick={() => onSelect(u)}>
          <UserAvatar user={u} size={32} />
          <span>{u.username}</span>
        </div>
      ))}
      {results.length === 0 && query.length > 1 && (
        <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '8px 0' }}>No users found</div>
      )}
    </div>
  );
}
