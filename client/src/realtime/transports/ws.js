const _wsReqs = {};
let _reqId = 0;

export function createWsTransport({ token, onReady, onEvent, onClose }) {
  const wsUrl = import.meta.env.VITE_WS_URL;
  const url = wsUrl || (() => {
    const isProd = location.hostname.endsWith('pages.dev');
    const host = isProd ? 'chat.moonchan.xyz' : location.host;
    const proto = isProd ? 'wss' : (location.protocol === 'https:' ? 'wss' : 'ws');
    return `${proto}://${host}/ws`;
  })();
  const ws = new WebSocket(url);

  ws.onmessage = (e) => {
    try {
      const env = JSON.parse(e.data);
      if (env.op === 'ready') {
        onReady({ onlineUserIds: env.payload?.online_user_ids || [], chats: env.payload?.chats || [] });
        return;
      }
      if (env.op) onEvent(env.op, env.payload);
      if (env.req_id && _wsReqs[env.req_id]) {
        clearTimeout(_wsReqs[env.req_id].timer);
        _wsReqs[env.req_id].resolve(env.payload);
        delete _wsReqs[env.req_id];
      }
    } catch (err) { console.error('WS parse error:', err); }
  };

  ws.onclose = () => onClose();

  return {
    disconnect() { ws.onclose = null; ws.close(); },
    sendTyping(chatId) {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ op: 'typing', chat_id: chatId }));
    },
    subscribe(chatId) {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ op: 'subscribe', chat_id: chatId }));
    },
    wsRequest(op, payload) {
      return new Promise((resolve, reject) => {
        if (ws.readyState !== WebSocket.OPEN) { reject(new Error('WS not connected')); return; }
        const reqId = ++_reqId;
        ws.send(JSON.stringify({ op, req_id: reqId, payload }));
        _wsReqs[reqId] = {
          resolve, reject,
          timer: setTimeout(() => { delete _wsReqs[reqId]; reject(new Error('WS request timeout')); }, 10000),
        };
      });
    },
  };
}
