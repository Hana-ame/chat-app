import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const { login, debugMode, setDebugMode, mockLogin, loading, error } = useAuthStore();
  const nav = useNavigate();

  const handle = async (e) => {
    e.preventDefault();
    try { await login(email, password); nav('/'); } catch (e) { console.error('Login page error:', e); }
  };

  const quickEnter = () => {
    mockLogin();
    nav('/');
  };

  return (
    <div style={{ minHeight:'100vh',display:'flex',alignItems:'center',justifyContent:'center', background:'var(--bg-tertiary)' }}>
      <form className="form-box" onSubmit={handle}>
        <h1>Welcome back!</h1>
        <p>We're so excited to see you again!</p>
        <label className="form-label">EMAIL</label>
        <input className="input-field" type="email" value={email} onChange={e=>setEmail(e.target.value)} required autoFocus />
        <label className="form-label">PASSWORD</label>
        <input className="input-field" type="password" value={password} onChange={e=>setPassword(e.target.value)} required />
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn-primary" style={{width:'100%',marginTop:16}} disabled={loading}>
          {loading ? 'Logging in...' : 'Log In'}
        </button>
        <div style={{display:'flex',alignItems:'center',gap:8,marginTop:12,fontSize:12,color:'var(--text-muted)'}}>
          <label style={{display:'flex',alignItems:'center',gap:4,cursor:'pointer'}}>
            <input type="checkbox" checked={debugMode} onChange={e => setDebugMode(e.target.checked)} />
            Debug mode
          </label>
          {debugMode && <button type="button" className="btn-ghost" style={{fontSize:12}} onClick={quickEnter}>
            ⚡ Quick Enter (mock)
          </button>}
        </div>
        <p style={{marginTop:12,fontSize:13,color:'var(--text-muted)'}}>
          Need an account? <Link to="/register">Register</Link>
        </p>
      </form>
    </div>
  );
}
