import React from 'react'
import useChat from './hooks/useChat'
import Login from './components/Login'
import Chat from './components/Chat'

export default function App() {
  const chat = useChat();
  if (!chat.user) return <Login onLogin={chat.login} />;
  return <Chat user={chat.user} messages={chat.messages} connStatus={chat.connStatus}
               onlineCount={chat.onlineCount} onSend={chat.sendMessage}
               onSendFile={chat.sendFile} onLogout={chat.logout} />;
}