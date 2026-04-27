import React, { useState } from 'react'

export default function Sidebar({ user, rooms, activeRoom, onRoomSelect, onLogout, onCreateRoom, onDeleteRoom }) {
  const [creating, setCreating] = useState(false);
  const [roomName, setRoomName] = useState('');

  const handleCreate = async (e) => {
    e.preventDefault();
    const name = roomName.trim();
    if (!name) return;
    if (onCreateRoom) await onCreateRoom(name);
    setRoomName('');
    setCreating(false);
  };

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
            {r.id !== 1 && onDeleteRoom && (
              <button className="room-delete" onClick={e => { e.stopPropagation(); onDeleteRoom(r.id); }}
                      title="删除房间">×</button>
            )}
          </div>
        ))}
        {creating ? (
          <form className="create-room-form" onSubmit={handleCreate}>
            <input
              autoFocus
              placeholder="room name"
              value={roomName}
              onChange={e => setRoomName(e.target.value)}
              maxLength={30}
            />
            <button type="submit">+</button>
            <button type="button" onClick={() => setCreating(false)}>x</button>
          </form>
        ) : (
          <div className="room-item create-room" onClick={() => setCreating(true)}>
            <span className="room-hash">+</span>
            Create Room
          </div>
        )}
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
