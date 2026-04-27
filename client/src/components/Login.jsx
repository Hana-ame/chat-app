import React, { useState } from 'react'

export default function Login({ onLogin }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    try { await onLogin(username, password); }
    catch (err) { setError(err.message); }
  };

  return (
    <div className="login-container">
      <form className="login-box" onSubmit={handleSubmit}>
        <h1>Chat Room</h1>
        <label>用户名</label>
        <input value={username} onChange={e => setUsername(e.target.value)}
               placeholder="输入用户名" autoFocus />
        <label>密码（首次自动注册）</label>
        <input value={password} onChange={e => setPassword(e.target.value)}
               type="password" placeholder="输入密码" />
        {error && <div className="login-err">{error}</div>}
        <button type="submit">登录</button>
      </form>
    </div>
  );
}
