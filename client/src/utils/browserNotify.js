let permitted = Notification.permission === 'granted';

export function requestNotifyPermission() {
  if (permitted) return Promise.resolve(true);
  if (!('Notification' in window)) return Promise.resolve(false);
  return Notification.requestPermission().then(p => {
    permitted = p === 'granted';
    return permitted;
  });
}

export function sendBrowserNotification(title, body, onClick) {
  if (!permitted) return;
  try {
    const n = new Notification(title, { body, icon: '/favicon.ico' });
    if (onClick) n.onclick = () => { window.focus(); onClick(); };
    setTimeout(() => n.close(), 8000);
  } catch (e) {
    console.error('browser notification error:', e);
  }
}
