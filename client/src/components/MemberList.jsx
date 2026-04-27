import React from 'react'

export default function MemberList({ onlineCount }) {
  return (
    <div className="member-list">
      <div className="member-list-header">在线 — {onlineCount || 0}</div>
    </div>
  );
}
