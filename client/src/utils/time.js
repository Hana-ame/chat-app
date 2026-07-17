export function formatRelativeTime(t) {
  if (!t) return '';
  const d = new Date(t);
  const now = new Date();
  const diff = now - d;
  if (diff < 60e3) return 'now';
  if (diff < 3600e3) return Math.floor(diff / 60e3) + 'm';
  if (diff < 86400e3) return Math.floor(diff / 3600e3) + 'h';
  return d.toLocaleDateString();
}
