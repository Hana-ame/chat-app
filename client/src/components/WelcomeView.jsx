import React from 'react';

export default function WelcomeView() {
  return (
    <div style={{
      flex: 1,
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      color: 'var(--text-muted)',
      textAlign: 'center',
      padding: '0 20px'
    }}>
      <div style={{
        fontSize: 64,
        marginBottom: 16,
        opacity: 0.5
      }}>💬</div>
      <h2 style={{
        color: 'var(--text-primary)',
        marginBottom: 8,
        fontSize: 20,
        fontWeight: 600
      }}>
        Welcome to ChatApp
      </h2>
      <p style={{
        fontSize: 14,
        lineHeight: 1.6,
        maxWidth: 300
      }}>
        Select a conversation from the sidebar to start chatting, or create a new group to bring your friends together.
      </p>
    </div>
  );
}
