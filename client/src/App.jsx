import { Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store/auth'
import LoginPage from './routes/LoginPage'
import RegisterPage from './routes/RegisterPage'
import ChatPage from './routes/ChatPage'

export default function App() {
  const token = useAuthStore((s) => s.accessToken)
  return (
    <Routes>
      <Route path="/login" element={token ? <Navigate to="/" /> : <LoginPage />} />
      <Route path="/register" element={token ? <Navigate to="/" /> : <RegisterPage />} />
      <Route path="/*" element={token ? <ChatPage /> : <Navigate to="/login" />} />
      <Route path="/g/:chatId" element={token ? <ChatPage /> : <Navigate to="/login" />} />
    </Routes>
  )
}
