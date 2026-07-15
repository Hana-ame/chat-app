import { create } from 'zustand';

export const useNotificationStore = create(() => ({
  notifications: [],
}));

const set = (fn) => useNotificationStore.setState(fn);

let _id = 0;

export function notify(message, type = 'error', duration = 4000) {
  const id = ++_id;
  set(s => ({ notifications: [...s.notifications, { id, message, type }] }));
  if (duration > 0) {
    setTimeout(() => set(s => ({ notifications: s.notifications.filter(n => n.id !== id) })), duration);
  }
  return id;
}
