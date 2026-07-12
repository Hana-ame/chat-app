export default function PublicChannelList({ results, searching, onJoin }) {
  if (searching) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
        搜索中...
      </div>
    );
  }

  if (!results) return null;

  if (results.length === 0) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
        无结果
      </div>
    );
  }

  return (
    <>
      <div style={{ padding: '4px 12px', fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
        Public Channels
      </div>
      {results.map(c => (
        <div key={c.id} className="chat-item" onClick={() => onJoin(c.id)}>
          <div className="chat-item-avatar" style={{ background: c.icon_color }}>
            {c.name ? c.name[0].toUpperCase() : '?'}
          </div>
          <div className="chat-item-info">
            <div className="chat-item-name">{c.name}</div>
            <div className="chat-item-preview">{c.member_count || c.members?.length || 0} members</div>
          </div>
        </div>
      ))}
    </>
  );
}
