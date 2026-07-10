import { generateDummyData } from '../dev/dummy';

function randid() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

const CHAT_COLORS = [
  '#5865F2', '#23a559', '#f0b232', '#ed4245', '#9b59b6',
  '#1abc9c', '#e67e22', '#2ecc71', '#e74c3c', '#3498db',
  '#f39c12', '#1dd1a1', '#a29bfe', '#fd79a8', '#00cec9',
];

function currentUser() {
  try {
    const raw = localStorage.getItem('auth');
    if (raw) {
      const u = JSON.parse(raw).user;
      if (u) return u;
    }
  } catch {}
  return userById('dev-self');
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

let data = null;
let _store = null;

export function __setStoreRef(store) { _store = store; }
export function __getAuthUser() {
  try {
    const raw = localStorage.getItem('auth');
    if (raw) return JSON.parse(raw).user || null;
  } catch {}
  return null;
}

const MOCK_USERS = [
  { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', email: 'alice@test.com', status: 'online' },
  { id: 'dev-bob', username: 'Bob', avatar_color: '#23a559', email: 'bob@test.com', status: 'online' },
  { id: 'dev-carol', username: 'Carol', avatar_color: '#f0b232', email: 'carol@test.com', status: 'online' },
  { id: 'dev-dave', username: 'Dave', avatar_color: '#ed4245', email: 'dave@test.com', status: 'offline' },
  { id: 'dev-eve', username: 'Eve', avatar_color: '#9b59b6', email: 'eve@test.com', status: 'online' },
  { id: 'dev-frank', username: 'Frank', avatar_color: '#1abc9c', email: 'frank@test.com', status: 'offline' },
  { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f', email: '', status: 'online' },
];

function ensureData() {
  if (!data) {
    const gen = generateDummyData({ chatCount: 10, msgPerChat: 150 });
    data = { chats: gen.chats, messages: [...(gen.messages || [])] };
  }
  return data;
}

export function resetMockData() {
  data = null;
  ensureData();
}

function userById(id) {
  return MOCK_USERS.find(u => u.id === id) || { id, username: 'Unknown', avatar_color: '#5865F2', email: '' };
}

function messagesFor(chatId) {
  const d = ensureData();
  return d.messages.filter(m => m.chat_id === chatId).sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
}

export function mockListChats() {
  const d = ensureData();
  const enriched = d.chats.map((c, i) => {
    const msgs = messagesFor(c.id);
    const last = msgs.filter(m => m.content?.trim()).length > 0 ? msgs.filter(m => m.content?.trim()).pop() : msgs[msgs.length - 1];
    return {
      ...c,
      icon_color: c.icon_color || CHAT_COLORS[i % CHAT_COLORS.length],
      last_message: last ? { id: last.id, content: last.content, deleted: last.deleted, author: last.author, created_at: last.created_at } : c.last_message,
      last_message_at: last?.created_at || c.last_message_at,
      members: c.members?.map(m => ({ ...m, ...(MOCK_USERS.find(u => u.id === m.id) || {}) })) || [],
    };
  });
  return { chats: enriched };
}

export function mockListMessages(_token, chatId, before, limit) {
  const all = messagesFor(chatId);
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
  if (!chat) return { error: 'not_found' };
  return { ...chat, members: chat.members?.map(m => ({ ...m, ...userById(m.id) })) || [] };
}

export function mockCreateChat(_token, name, memberIds, visibility) {
  const d = ensureData();
  const cu = currentUser();
  const newChat = {
    id: randid(),
    type: 'group',
    name,
    icon_color: CHAT_COLORS[d.chats.length % CHAT_COLORS.length],
    owner_id: cu.id,
    visibility: visibility || 'private',
    created_at: new Date().toISOString(),
    last_message_at: null,
    members: [{ id: cu.id, ...userById(cu.id), role: 'owner' }, ...(memberIds || []).map(id => ({ id, ...userById(id), role: 'member' }))],
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
  const cu = currentUser();
  const existing = d.chats.find(c => c.type === 'dm' && c.members?.some(m => m.id === userId));
  if (existing) return existing;
  const newDM = {
    id: randid(),
    type: 'dm',
    name: '',
    icon_color: '#5865F2',
    owner_id: '',
    visibility: 'private',
    created_at: new Date().toISOString(),
    members: [
      { id: cu.id, ...userById(cu.id), role: 'member' },
      { id: userId, ...userById(userId), role: 'member' },
    ],
  };
  d.chats.unshift(newDM);
  return newDM;
}

export function mockAddMember(_token, chatId, userId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat && !chat.members?.some(m => m.id === userId)) {
    if (!chat.members) chat.members = [];
    chat.members.push({ id: userId, ...userById(userId), role: 'member' });
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
  const cu = currentUser();
  const results = MOCK_USERS
    .filter(u => u.id !== cu.id && u.username?.toLowerCase().includes(q.toLowerCase()))
    .slice(0, 20);
  return { users: results };
}

export function mockUpdateProfile(_token, data) {
  const cu = currentUser();
  const updated = { ...cu, username: data.username || cu.username, avatar_color: data.avatar_color || cu.avatar_color, avatar_url: data.avatar_url || cu.avatar_url || '' };
  const d = ensureData();
  d.chats.forEach(chat => {
    const mi = chat.members?.findIndex(m => m.id === updated.id);
    if (mi !== -1 && mi !== undefined) {
      chat.members[mi] = { ...chat.members[mi], ...updated };
      if (_store) _store.getState().onChatUpdate({ id: chat.id, members: [...chat.members] });
    }
  });
  if (_store) _store.setState({ user: updated });
  return updated;
}

export function mockSendMessage(_token, chatId, content, attachments) {
  const d = ensureData();
  const now = new Date().toISOString();

  const userMsg = {
    id: randid(),
    chat_id: chatId,
    content,
    user_id: currentUser().id,
    author: currentUser(),
    created_at: now,
    edited_at: null,
    deleted: false,
    attachments: attachments || [],
    reactions: [],
  };
  d.messages.push(userMsg);
  if (_store) _store.getState().onMessageCreate(userMsg);

  const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
  const aiId = randid();
  const aiCreatedAt = new Date(Date.now() + 1).toISOString();

  const aiStoreMsg = {
    id: aiId,
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: userById('ai'),
    created_at: aiCreatedAt,
    edited_at: null,
    deleted: false,
    attachments: [],
    reactions: [],
    streaming: true,
    source: async (emit) => {
      let acc = '';
      for (let i = 0; i < text.length; i++) {
        await new Promise(r => setTimeout(r, 25 + Math.random() * 30));
        emit(text[i]);
        acc += text[i];
        const m = d.messages.find(m => m.id === aiId);
        if (m) m.content = acc;
      }
      const m = d.messages.find(m => m.id === aiId);
      if (m) m.streaming = false;
    },
  };

  const aiDataMsg = {
    id: aiId,
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: userById('ai'),
    created_at: aiCreatedAt,
    edited_at: null,
    deleted: false,
    attachments: [],
    reactions: [],
    streaming: true,
  };

  setTimeout(() => {
    d.messages.push(aiDataMsg);
    if (_store) _store.getState().onMessageCreate(aiStoreMsg);
  }, 500 + Math.random() * 800);

  return userMsg;
}

export function mockEditMessage(_token, _chatId, msgId, content) {
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    msg.content = content;
    msg.edited_at = new Date().toISOString();
  }
  if (_store) _store.getState().onMessageUpdate({ id: msgId, content, edited_at: new Date().toISOString() });
  return { ok: true };
}

export function mockDeleteMessage(_token, _chatId, msgId) {
  const d = ensureData();
  d.messages = d.messages.filter(m => m.id !== msgId);
  if (_store) _store.getState().onMessageDelete({ message_id: msgId, chat_id: _chatId });
  return { ok: true };
}

export function mockAddReaction(_token, _chatId, msgId, emoji) {
  const d = ensureData();
  const cu = currentUser();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    const rxs = msg.reactions || [];
    const existing = rxs.find(r => r.emoji === emoji && r.user_ids?.includes(cu.id));
    if (!existing) {
      const byEmoji = rxs.find(r => r.emoji === emoji);
      if (byEmoji) {
        byEmoji.count += 1;
        byEmoji.user_ids = [...(byEmoji.user_ids || []), cu.id];
      } else {
        rxs.push({ emoji, count: 1, user_ids: [cu.id] });
      }
    }
  }
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: cu.id }, true);
  return { ok: true };
}

export function mockRemoveReaction(_token, _chatId, msgId, emoji) {
  const d = ensureData();
  const cu = currentUser();
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    const rxs = (msg.reactions || []).map(r => {
      if (r.emoji !== emoji) return r;
      const filtered = (r.user_ids || []).filter(id => id !== cu.id);
      return { ...r, count: filtered.length, user_ids: filtered };
    }).filter(r => r.count > 0);
    msg.reactions = rxs;
  }
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: cu.id }, false);
  return { ok: true };
}

export function mockSetPinnedMessage(_token, chatId, content) {
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: { content, pinned_at: new Date().toISOString() } });
  return { ok: true };
}

export function mockClearPinnedMessage(_token, chatId) {
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: null });
  return { ok: true };
}

export function mockMarkRead() {
  return { ok: true };
}

export function mockJoinChat(_token, chatId) {
  const d = ensureData();
  const cu = currentUser();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat && !chat.members?.some(m => m.id === cu.id)) {
    if (!chat.members) chat.members = [];
    chat.members.push({ id: cu.id, ...userById(cu.id), role: 'member' });
  }
  if (chat && chat.visibility !== 'public' && chat.visibility !== 'unlisted') {
    return { error: 'private' };
  }
  return { ok: true };
}

export function mockRegister(_token, email, username, password) {
  const existing = MOCK_USERS.find(u => u.email.toLowerCase() === email.toLowerCase());
  if (existing) {
    throw { status: 409, error: 'already_taken', message: 'email or username already taken' };
  }
  const idx = MOCK_USERS.length;
  const color = CHAT_COLORS[idx % CHAT_COLORS.length];
  return {
    user: { id: randid(), username: username.trim(), email: email.toLowerCase().trim(), avatar_color: color, avatar_url: '', status: 'online' },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockLogin(_token, email, password) {
  const user = MOCK_USERS.find(u => u.email.toLowerCase() === email.toLowerCase());
  if (!user) {
    throw { status: 401, error: 'invalid_credentials', message: 'invalid email or password' };
  }
  return {
    user: { ...user },
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockRefresh() {
  const cu = currentUser();
  return {
    user: userById(cu.id),
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

export function mockLogout() {
  return { ok: true };
}

export function mockMe() {
  const cu = currentUser();
  return userById(cu.id);
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
  const ext = file?.name?.split('.').pop() || 'bin';
  return {
    filename: file?.name || 'file.' + ext,
    mime_type: file?.type || 'application/octet-stream',
    size: file?.size || 0,
    url: URL.createObjectURL(file),
  };
}

export function mockUploadAvatar(_token, file) {
  return { url: URL.createObjectURL(file) };
}

export function mockTogglePin(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: false };
  chat.pinned = !chat.pinned;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned: chat.pinned });
  return { ok: true, pinned: chat.pinned };
}
