export default function EmptyState({ message, icon }) {
  return (
    <div style={{
      flex: 1, display: 'flex', flexDirection: 'column',
      alignItems: 'center', justifyContent: 'center',
      color: 'var(--text-muted)', padding: 24, textAlign: 'center',
    }}>
      {icon && <div style={{ fontSize: 32, marginBottom: 12 }}>{icon}</div>}
      <div style={{ fontSize: 14 }}>{message}</div>
    </div>
  );
}
