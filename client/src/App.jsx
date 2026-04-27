import React, { useState, useEffect } from 'react'
import useChat from './hooks/useChat'
import Login from './components/Login'
import Sidebar from './components/Sidebar'
import Chat from './components/Chat'
import MemberList from './components/MemberList'

export default function App() {
  const chat = useChat();
  const [activeRoom, setActiveRoom] = useState(1);
  const [rooms, setRooms] = useState([]);

  useEffect(() => {
    if (chat.user) {
      const API = window.location.hostname.endsWith('pages.dev')
        ? 'https://wsl-8000.moonchan.xyz' : '';
      fetch(`${API}/api/rooms?token=${chat.user.token}`)
        .then(r => r.json())
        .then(d => { if (d.rooms) setRooms(d.rooms); })
        .catch(() => {});
    }
  }, [chat.user]);

  if (!chat.user) return <Login onLogin={chat.login} />;

  return (
    <div className="app-shell">
      <Sidebar
        user={chat.user}
        rooms={rooms}
        activeRoom={activeRoom}
        onRoomSelect={(id) => setActiveRoom(id)}
        onLogout={chat.logout}
        onCreateRoom={async (name) => {
          const room = await chat.createRoom(name);
          if (room) { setRooms(prev => [...prev, room]); setActiveRoom(room.id); }
          return room;
        }}
      />
      <Chat
        user={chat.user}
        messages={chat.messages.filter(m => m.room_id === activeRoom || m.type === 'system' || !m.room_id)}
        connStatus={chat.connStatus}
        onlineCount={chat.onlineCount}
        onSend={(text) => chat.sendMessage(text, activeRoom)}
        onSendFile={(file) => chat.sendFile(file, activeRoom)}
        onLoadMore={() => chat.loadMoreHistory(activeRoom)}
        roomName={rooms.find(r => r.id === activeRoom)?.name || '大厅'}
      />
      <MemberList onlineCount={chat.onlineCount} />
    </div>
  );
}
