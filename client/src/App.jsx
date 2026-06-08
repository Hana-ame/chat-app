import { useEffect, useRef } from 'react'
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { useAuthStore } from './store/auth'
import LoginPage from './routes/LoginPage'
import RegisterPage from './routes/RegisterPage'
import ChatPage from './routes/ChatPage'

export default function App() {
  const token = useAuthStore((s) => s.accessToken)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const loggingOut = useRef(false)

  useEffect(() => {
    const onUnauth = () => {
      if (loggingOut.current) return
      loggingOut.current = true
      logout()
      navigate('/login', { replace: true })
      setTimeout(() => { loggingOut.current = false }, 500)
    }
    window.addEventListener('auth:unauthorized', onUnauth)
    return () => window.removeEventListener('auth:unauthorized', onUnauth)
  }, [logout, navigate])

  return (
    <Routes>
      <Route path="/login" element={token ? <Navigate to="/" /> : <LoginPage />} />
      <Route path="/register" element={token ? <Navigate to="/" /> : <RegisterPage />} />
      <Route path="/*" element={token ? <ChatPage /> : <Navigate to="/login" />} />
      <Route path="/g/:chatId" element={token ? <ChatPage /> : <Navigate to="/login" />} />
    </Routes>
  )
}
