import React from 'react'

const COLORS = ['#e74c3c','#3498db','#2ecc71','#f39c12','#9b59b6','#1abc9c','#e67e22','#1f8b4c'];

export default function MemberList({ onlineCount, onlineUsers }) {
  const users = onlineUsers || [];
  const count = onlineCount || users.length || 0;

  return (
    <div className="member-list">
      <div className="member-list-header">在线 — {count}</div>
      <div className="member-list-scroll">
        {users.map((name, i) => (
          <div key={name} className="member-item">
            <div className="member-avatar" style={{ background: COLORS[i % COLORS.length] }}>
              {name[0].toUpperCase()}
              <div className="member-online" />
            </div>
            {name}
          </div>
        ))}
        {users.length === 0 && count > 0 && (
          <div className="member-item" style={{color:'#949ba4',fontSize:'13px',padding:'8px 16px'}}>
            {count} 人在线
          </div>
        )}
      </div>
    </div>
  );
}
