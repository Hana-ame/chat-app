import { useState } from 'react';

export default function UserAvatar({ user, size = 32, onClick, onFallbackClick }) {
  const [imgError, setImgError] = useState(false);
  const showImg = user?.avatar_url && !imgError;

  if (showImg) {
    return (
      <img
        src={user.avatar_url}
        alt={user?.username || ''}
        style={{ width: size, height: size, borderRadius: '50%', objectFit: 'cover', flexShrink: 0, display: 'block' }}
        onClick={onClick}
        onError={() => setImgError(true)}
      />
    );
  }

  const initial = (user?.username?.[0] || '?').toUpperCase();
  const bg = user?.avatar_color || '#5865F2';

  return (
    <div
      style={{
        width: size,
        height: size,
        borderRadius: '50%',
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontWeight: 700,
        color: '#fff',
        fontSize: Math.max(10, Math.round(size * 0.4)),
        background: bg,
      }}
      onClick={onFallbackClick}
    >
      {initial}
    </div>
  );
}
