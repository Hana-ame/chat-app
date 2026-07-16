import ChatListItem from './ChatListItem';

export default function PublicChannelList({ results, searching, onJoin }) {
  if (searching) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
        searching...
      </div>
    );
  }

  if (!results) return null;

  if (results.length === 0) {
    return (
      <div style={{ padding: 24, textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
        no results
      </div>
    );
  }

  return (
    <>
      <div style={{ padding: '4px 12px', fontSize: 11, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
        Public Channels
      </div>
      {results.map(c => (
        <ChatListItem key={c.id} chat={c} onSelectChat={onJoin} />
      ))}
    </>
  );
}
