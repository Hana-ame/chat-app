import UserAvatar from './UserAvatar';
import { api } from '../api/client';

export default function SidebarFooter({ user, onLogout, onSettings }) {
  return (
    <div className="sidebar-footer">
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }} onClick={onSettings}>
        <UserAvatar user={user} size={32} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 14, fontWeight: 600 }}>{user?.username || 'Unknown'}</div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>Online</div>
        </div>
        <button className="btn-ghost" onClick={(e) => { e.stopPropagation(); onSettings(); }} title="Settings">⚙</button>
        <button className="btn-ghost" onClick={onLogout}>↪</button>
      </div>
      {api.isMockEnabled() && (
        <div style={{ marginTop: 4, borderTop: '1px solid var(--border)', paddingTop: 4 }}>
          <div style={{ fontSize: 11, color: 'var(--text-muted)', textAlign: 'center', padding: '4px 0' }}>
            ⚡ Using Mock API
          </div>
        </div>
      )}
    </div>
  );
}
