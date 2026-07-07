import { generateDummyData } from '../dev/dummy';

let data = null;
let _store = null;

export function __setStoreRef(store) { _store = store; }

function ensureData() {
  if (!data) {
    data = generateDummyData({ chatCount: 10, msgPerChat: 150 });
  }
  return data;
}

export function resetMockData() {
  data = null;
  ensureData();
}

export function mockListChats() {
  return { chats: ensureData().chats };
}

export function mockListMessages(_token, chatId, before, limit) {
  const d = ensureData();
  const all = d.messages
    .filter(m => m.chat_id === chatId)
    .sort((a, b) => new Date(a.created_at) - new Date(b.created_at));

  if (before) {
    const idx = all.findIndex(m => m.id === before);
    if (idx <= 0) return { messages: [] };
    const start = Math.max(0, idx - (limit || 50));
    return { messages: all.slice(start, idx) };
  }

  const total = all.length;
  const pageSize = limit || 50;
  const start = Math.max(0, total - pageSize);
  return { messages: all.slice(start, total) };
}

const AI_RESPONSES = [
  "That's an interesting thought! Let me add my perspective here.",
  "I agree with you. Building on what you said, there's more to consider.",
  "Great question! Here's my take on this complex topic.",
  "I've been thinking about this too. It really depends on the context.",
  "Excellent point. Let me expand with some additional insights.",
  "You raise a valid concern. Here's how I see it working out.",
  "That reminds me of a similar situation we handled before.",
  "I'd approach it differently. Let me explain my reasoning.",
  "Good observation! The key factor here is timing and coordination.",
  "Let me share what I've learned from past experience with this.",
];

export function mockSendMessage(_token, chatId, content, attachments) {
  const store = _store;
  if (!store) return;

  const s = store.getState();
  const now = new Date().toISOString();

  const userMsg = {
    id: 'mock-msg-' + Date.now(),
    chat_id: chatId,
    content,
    user_id: 'dev-self',
    author: { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2' },
    created_at: now,
    edited_at: null,
    deleted: false,
    attachments: attachments || [],
    reactions: [],
  };
  s.onMessageCreate(userMsg);

  const delay = 500 + Math.random() * 1200;
  setTimeout(() => {
    const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
    s.startStreamingInChat(chatId, async (emit) => {
      for (const char of text) {
        await new Promise(r => setTimeout(r, 25 + Math.random() * 20));
        emit(char);
      }
    });
  }, delay);
}
