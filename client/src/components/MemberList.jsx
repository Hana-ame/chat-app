import React from 'react'

const COLORS = ['#e74c3c','#3498db','#2ecc71','#f39c12','#9b59b6','#1abc9c','#e67e22','#1f8b4c'];

export default function MemberList({ onlineCount }) {
  // Simple: show online count, could be expanded with actual user list
  const count = onlineCount || 0;
  const fake = Array.from({ length: Math.max(count, 1) }, (_, i) => ({
    name: `在线用户 ${i+1}`,
    color: COLORS[i % COLORS.length],
  }));

  return (
    <div className="member-list">
      <div className="member-list-header">在线 — {count}</div>
      <div className="member-list-scroll">
        {fake.map((u, i) => (
          <div key={i} className="member-item">
            <div className="member-avatar" style={{ background: u.color }}>
              {u.name[0]}
              <div className="member-online" />
            </div>
            {u.name}
          </div>
        ))}
      </div>
    </div>
  );
}
