import { useEffect, useRef } from 'react';
import { useNotificationStore } from '../store/notification';

export default function Toast() {
  const notifications = useNotificationStore(s => s.notifications);
  const injected = useRef(false);
  useEffect(() => {
    if (injected.current) return;
    injected.current = true;
    const style = document.createElement('style');
    style.textContent = '@keyframes toastIn { from { opacity:0; transform:translateX(20px); } to { opacity:1; transform:translateX(0); } }';
    document.head.appendChild(style);
  }, []);
  if (!notifications.length) return null;
  return (
    <div style={{
      position: 'fixed', top: 12, right: 12, zIndex: 9999, display: 'flex',
      flexDirection: 'column', gap: 8, maxWidth: 360,
    }}>
      {notifications.map(n => (
        <div key={n.id} style={{
          padding: '10px 14px', borderRadius: 8, fontSize: 14, lineHeight: 1.4,
          boxShadow: '0 4px 12px rgba(0,0,0,0.25)',
          color: '#fff',
          background: n.type === 'error' ? 'var(--danger)' : n.type === 'success' ? 'var(--success)' : 'var(--accent)',
          animation: 'toastIn 0.2s ease-out',
        }}>
          {n.message}
        </div>
      ))}
    </div>
  );
}
