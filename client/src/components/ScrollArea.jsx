export default function ScrollArea({ children, style, className }) {
  return (
    <div className={className} style={{
      flex: 1, overflowY: 'auto', minHeight: 0,
      display: 'flex', flexDirection: 'column',
      padding: '8px 0',
      ...style,
    }}>
      {children}
    </div>
  );
}
