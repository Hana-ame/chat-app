import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { useAuthStore } from './store/auth'
import Toast from './components/Toast'
import './styles/global.css'

if (typeof window !== 'undefined') {
  window.__mockLogin = () => useAuthStore.getState().mockLogin();
}

ReactDOM.createRoot(document.getElementById('root')).render(
  <BrowserRouter>
    <App />
    <Toast />
  </BrowserRouter>
)
