import React from 'react'
import useChat from './hooks/useChat'
import Login from './components/Login'
import Chat from './components/Chat'

export default function App() {
  const chat = useChat();
  return chat.user ? <Chat {...chat} /> : <Login onLogin={chat.login} />;
}