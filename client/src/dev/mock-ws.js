import { useChatStore } from '../store/chat';

let _interval = null;

function get() {
  return useChatStore.getState();
}

function fire(op, payload) {
  get().disconnect();
  window.dispatchEvent(new CustomEvent('mock:ws-event', { detail: { op, payload } }));
}

export function mockWebSocketConnect(token, mode) {
  const store = get();
  store.disconnect();

  if (mode === 'ws') {
    store.setMode('poll');
  }

  const chats = store.chats.length > 0 ? store.chats : [];

  window.setTimeout(() => {
    fire('ready', {
      user: { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2' },
      chats,
      online_user_ids: ['dev-self', 'dev-bob'],
    });
  }, 50);

  if (_interval) clearInterval(_interval);
  _interval = setInterval(() => {
    const s = useChatStore.getState();
    if (s.mode === 'poll') {
      s.loadChats(token);
      if (s.activeChatId) s.loadMessages(token, s.activeChatId);
    }
  }, 500);
}

export function mockWebSocketDisconnect() {
  if (_interval) {
    clearInterval(_interval);
    _interval = null;
  }
}

export function simulateEvent(op, payload) {
  const store = useChatStore.getState();

  switch (op) {
    case 'message_create':
      store.onMessageCreate(payload);
      break;
    case 'message_update':
      store.onMessageUpdate(payload);
      break;
    case 'message_delete':
      store.onMessageDelete(payload);
      break;
    case 'reaction_add':
      store.onReaction(payload, true);
      break;
    case 'reaction_remove':
      store.onReaction(payload, false);
      break;
    case 'chat_create':
    case 'chat_update':
      store.onChatUpdate(payload);
      break;
    case 'chat_delete':
      store.onChatDelete(payload);
      break;
    case 'chat_remove':
      store.onChatRemove(payload);
      break;
    case 'presence_update':
      store.onlineUserIds = payload;
      break;
    case 'typing':
      break;
    case 'user_update':
      break;
  }
}

export function resetMockWs() {
  mockWebSocketDisconnect();
}

const chatEvents = [
  'message_create', 'message_update', 'message_delete',
  'chat_create', 'chat_update', 'chat_delete', 'chat_remove',
  'reaction_add', 'reaction_remove',
  'presence_update', 'user_update',
];

export { chatEvents };