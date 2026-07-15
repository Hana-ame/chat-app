import { api } from '../../api/client';

export function createSseTransport({ token, onReady, onEvent, onClose }) {
  const sse = new EventSource(api.sseUrl(token));

  sse.addEventListener('ready', (e) => {
    try {
      const p = JSON.parse(e.data);
      onReady({ onlineUserIds: p.online_user_ids || [], chats: p.chats || [] });
    } catch (err) { console.error('SSE ready parse error:', err); }
  });

  sse.onmessage = (e) => {
    try {
      const data = JSON.parse(e.data);
      if (data.op) onEvent(data.op, data.payload);
    } catch (err) { console.error('SSE message parse error:', err); }
  };

  sse.onerror = () => onClose();

  return { disconnect() { sse.close(); } };
}
