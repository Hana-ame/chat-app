import React from 'react'

export default function Sidebar({ user, rooms, activeRoom, onRoomSelect, onLogout }) {
  return (
    <div className="sidebar">
      <div className="sidebar-header">Chat Rooms</div>
      <div className="sidebar-scroll">
        <div className="sidebar-label">ROOMS</div>
        {rooms.map(r => (
          <div
            key={r.id}
            className={`room-item ${r.id === activeRoom ? 'active' : ''}`}
            onClick={() => onRoomSelect(r.id)}
          >
            <span className="room-hash">#</span>
            {r.name}
          </div>
        ))}
      </div>
      <div className="sidebar-footer">
        <div className="user-avatar" style={{ background: user.avatar_color }}>
          {user.username[0].toUpperCase()}
        </div>
        <div>
          <div className="user-name">{user.username}</div>
          <div className="user-tag">online</div>
        </div>
        <button className="logout-btn" onClick={onLogout}>退出</button>
      </div>
    </div>
  );
}
