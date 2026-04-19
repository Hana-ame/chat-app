import React, { useState } from 'react'

export default function Login({ onLogin }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    try {
      await onLogin(username, password);
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <div className="login-container">
      <h1>💬 极简聊天</h1>
      <form onSubmit={handleSubmit}>
        <input value={username} onChange={e => setUsername(e.target.value)} placeholder="用户名" required />
        <input value={password} onChange={e => setPassword(e.target.value)} type="password" placeholder="密码 (首次即注册)" required />
        {error && <div style={{color: '#e74c3c', fontSize: '14px'}}>{error}</div>}
        <button type="submit">进入聊天室</button>
      </form>
    </div>
  );
}