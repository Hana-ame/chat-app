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

export function mockGetChat(_token, id) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === id);
  return chat || { error: 'not_found' };
}

export function mockCreateChat(_token, name) {
  const d = ensureData();
  const newChat = {
    id: 'mock-chat-' + Date.now(),
    type: 'group',
    name,
    owner_id: 'dev-self',
    visibility: 'private',
    created_at: new Date().toISOString(),
    last_message_at: null,
    members: [{ id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', role: 'owner' }],
  };
  d.chats.unshift(newChat);
  return newChat;
}

export function mockDeleteChat(_token, id) {
  const d = ensureData();
  d.chats = d.chats.filter(c => c.id !== id);
  return { ok: true };
}

export function mockCreateDM(_token, userId) {
  const d = ensureData();
  const other = d.chats.find(c => c.id === userId)?.members?.[0] || { id: userId, username: 'User', avatar_color: '#5865F2' };
  const existing = d.chats.find(c => c.type === 'dm' && c.members?.some(m => m.id === userId));
  if (existing) return existing;
  const newDM = {
    id: 'mock-dm-' + Date.now(),
    type: 'dm',
    name: '',
    owner_id: '',
    visibility: 'private',
    created_at: new Date().toISOString(),
    members: [
      { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', role: 'member' },
      other,
    ],
  };
  d.chats.unshift(newDM);
  return newDM;
}

export function mockAddMember(_token, chatId, userId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat && !chat.members?.some(m => m.id === userId)) {
    chat.members.push({ id: userId, username: 'User', avatar_color: '#5865F2', role: 'member' });
  }
  return chat || { ok: true };
}

export function mockRemoveMember(_token, chatId, userId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat) {
    chat.members = chat.members?.filter(m => m.id !== userId);
  }
  return { ok: true };
}

export function mockSearchUsers(_token, q) {
  const d = ensureData();
  const results = d.chats
    .flatMap(c => c.members || [])
    .filter((m, i, arr) => arr.findIndex(x => x.id === m.id) === i)
    .filter(m => m.username?.toLowerCase().includes(q.toLowerCase()) && m.id !== 'dev-self');
  return { users: results.slice(0, 20) };
}

export function mockUpdateProfile(_token, data) {
  return { id: 'dev-self', username: data.username || 'Alice', avatar_color: data.avatar_color || '#5865F2', avatar_url: data.avatar_url || '' };
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
  if (!store) { console.warn('mock: _store null'); return; }
  try {
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
      const cur = store.getState();
      const aiMsg = {
        id: 'mock-ai-' + Date.now(),
        chat_id: chatId,
        content: '',
        user_id: 'ai',
        author: { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f' },
        created_at: new Date().toISOString(),
        streaming: true,
        deleted: false,
        attachments: [],
        reactions: [],
        source: {
          type: 'mock',
          fn: async (emit) => {
            for (const char of text) {
              await new Promise(r => setTimeout(r, 25 + Math.random() * 20));
              emit(char);
            }
          },
        },
      };
      cur.onMessageCreate(aiMsg);
    }, delay);
  } catch (e) { console.error('mockSendMessage error:', e); }
}

export function mockEditMessage(_token, _chatId, msgId, content) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onMessageUpdate({ id: msgId, content, edited_at: new Date().toISOString() });
  return { ok: true };
}

export function mockDeleteMessage(_token, _chatId, msgId) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onMessageDelete({ message_id: msgId, chat_id: _chatId });
  return { ok: true };
}

export function mockAddReaction(_token, _chatId, msgId, emoji) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onReaction({ message_id: msgId, emoji, user_id: 'dev-self' }, true);
  return { ok: true };
}

export function mockRemoveReaction(_token, _chatId, msgId, emoji) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onReaction({ message_id: msgId, emoji, user_id: 'dev-self' }, false);
  return { ok: true };
}

export function mockSetPinnedMessage(_token, chatId, content) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onChatUpdate({ id: chatId, pinned_message: { content, pinned_at: new Date().toISOString() } });
  return { ok: true };
}

export function mockClearPinnedMessage(_token, chatId) {
  const store = _store;
  if (!store) return { ok: true };
  const s = store.getState();
  s.onChatUpdate({ id: chatId, pinned_message: null });
  return { ok: true };
}

export function mockMarkRead() {
  return { ok: true };
}

export function mockJoinChat(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat && !chat.members?.some(m => m.id === 'dev-self')) {
    if (!chat.members) chat.members = [];
    chat.members.push({ id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', role: 'member' });
  }
  if (chat && chat.visibility !== 'public' && chat.visibility !== 'unlisted') {
    return { error: 'private' };
  }
  return { ok: true };
}

export function mockRegister(_token, email, username, password) {
  return {
    user: { id: 'mock-' + Date.now(), username, email, avatar_color: '#5865F2' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockLogin(_token, email, password) {
  return {
    user: { id: 'mock-' + Date.now(), username: email.split('@')[0], email, avatar_color: '#5865F2' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockRefresh() {
  return {
    user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockLogout() {
  return { ok: true };
}

export function mockMe() {
  return { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' };
}

export function mockListPublicChats() {
  const d = ensureData();
  return { chats: d.chats.filter(c => c.visibility === 'public') };
}

export function mockRenameChat(_token, _id, name) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === _id);
  if (chat) chat.name = name;
  return { ok: true };
}

export function mockUpload(file) {
  return { id: 'mock-upload-' + Date.now() };
}

export function mockUploadAvatar(_token, file) {
  return { url: 'https://upload.moonchan.xyz/mock/avatar.png' };
}