import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';

export default function RegisterPage() {
  const [email, setEmail] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const { register, loading, error } = useAuthStore();
  const nav = useNavigate();

  const handle = async (e) => {
    e.preventDefault();
    try { await register(email, username, password); nav('/'); } catch (e) { console.error('Register page error:', e); }
  };

  return (
    <div style={{ minHeight:'100vh',display:'flex',alignItems:'center',justifyContent:'center', background:'var(--bg-tertiary)' }}>
      <form className="form-box" onSubmit={handle}>
        <h1>Create an account</h1>
        <label className="form-label">EMAIL</label>
        <input className="input-field" type="email" value={email} onChange={e=>setEmail(e.target.value)} required autoFocus />
        <label className="form-label">USERNAME</label>
        <input className="input-field" type="text" value={username} onChange={e=>setUsername(e.target.value)} required minLength={2} maxLength={32} />
        <label className="form-label">PASSWORD</label>
        <input className="input-field" type="password" value={password} onChange={e=>setPassword(e.target.value)} required minLength={8} />
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn-primary" style={{width:'100%',marginTop:16}} disabled={loading}>
          {loading ? 'Registering...' : 'Continue'}
        </button>
        <p style={{marginTop:12,fontSize:13,color:'var(--text-muted)'}}>
          <Link to="/login">Already have an account?</Link>
        </p>
      </form>
    </div>
  );
}