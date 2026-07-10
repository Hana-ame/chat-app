import { generateDummyData } from '../dev/dummy';

const CHAT_COLORS = [
  '#5865F2', '#23a559', '#f0b232', '#ed4245', '#9b59b6',
  '#1abc9c', '#e67e22', '#2ecc71', '#e74c3c', '#3498db',
  '#f39c12', '#1dd1a1', '#a29bfe', '#fd79a8', '#00cec9',
];

function randid() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

function now() { return new Date().toISOString(); }

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

let _nextUserId = 1;
function allocUserId() { return 'u-' + (_nextUserId++); }
function registeredUsers() { return data?.users || []; }

function userById(id) {
  if (data?.users) {
    const found = data.users.find(u => u.id === id);
    if (found) return found;
  }
  return MOCK_USERS.find(u => u.id === id) || { id, username: 'Unknown', avatar_color: '#5865F2', email: '' };
}

function ensureData() {
  if (!data) {
    const gen = generateDummyData({ chatCount: 10, msgPerChat: 150 });
    const rawChats = gen.chats || [];
    const rawMessages = gen.messages || [];
    data = {
      users: [],
      chats: rawChats.map(c => ({
        ...c,
        last_message_at: c.last_message_at || null,
        pinned_message: c.pinned_message || null,
      })),
      messages: rawMessages.map(m => ({
        ...m,
        deleted_at: m.deleted_at || (m.deleted ? now() : null),
        edited_at: m.edited_at || null,
        reaction_count: m.reaction_count || (m.reactions?.length || 0),
        attachment_count: m.attachment_count || (m.attachments?.length || 0),
        mention_count: m.mention_count || 0,
        attachments: m.attachments || [],
        reactions: m.reactions || [],
        mentions: m.mentions || [],
      })),
      reactions: [],
    };
    data.messages.forEach(msg => {
      (msg.reactions || []).forEach(r => {
        (r.user_ids || []).forEach(uid => {
          data.reactions.push({ message_id: msg.id, user_id: uid, emoji: r.emoji, created_at: msg.created_at });
        });
      });
    });
  }
  return data;
}

export function resetMockData() {
  data = null;
  ensureData();
}

function messagesFor(chatId) {
  const d = ensureData();
  return d.messages
    .filter(m => m.chat_id === chatId)
    .sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
}

function syncReactionsColumn(msgId) {
  const d = ensureData();
  const grouped = {};
  const order = [];
  d.reactions.forEach(r => {
    if (r.message_id !== msgId) return;
    if (grouped[r.emoji]) {
      grouped[r.emoji].count++;
    } else {
      grouped[r.emoji] = { emoji: r.emoji, count: 1 };
      order.push(r.emoji);
    }
  });
  const rxs = order.map(e => ({ ...grouped[e], user_ids: d.reactions.filter(r => r.message_id === msgId && r.emoji === e).map(r => r.user_id) }));
  const msg = d.messages.find(m => m.id === msgId);
  if (msg) {
    msg.reactions = rxs;
    msg.reaction_count = rxs.reduce((s, r) => s + r.count, 0);
  }
}

function lastMessageFor(chatId) {
  const msgs = messagesFor(chatId).filter(m => !m.deleted_at);
  return msgs.length > 0 ? msgs[msgs.length - 1] : null;
}

function buildChatResponse(c) {
  const d = ensureData();
  const last = lastMessageFor(c.id);
  const members = (d.reactions ? [] : []); // placeholder
  const chatMembers = d.chatMembers || [];
  const memberList = chatMembers.filter(cm => cm.chat_id === c.id).map(cm => ({ ...cm, ...userById(cm.user_id) }));
  return {
    id: c.id,
    type: c.type || 'group',
    name: c.name || '',
    icon_color: c.icon_color || '#5865F2',
    visibility: c.visibility || 'private',
    owner_id: c.owner_id || '',
    created_at: c.created_at,
    last_message_at: (last ? last.created_at : null) || c.last_message_at || c.created_at,
    member_count: (c.members ? c.members.length : 0) || memberList.length,
    unread_count: c.unread_count || 0,
    pinned_message: c.pinned_message || null,
    last_message_id: last ? last.id : (c.last_message_id || ''),
    last_message: last ? {
      id: last.id,
      content: last.content,
      deleted: !!last.deleted_at,
      author: userById(last.user_id),
      created_at: last.created_at,
    } : (c.last_message || null),
  };
}

// ── Auth ──

export function mockRegister(_token, email, username, password) {
  const d = ensureData();
  const normalizedEmail = email.toLowerCase().trim();
  const existing = d.users.find(u => u.email === normalizedEmail) || MOCK_USERS.find(u => u.email.toLowerCase() === normalizedEmail);
  if (existing) {
    throw { status: 409, error: 'already_taken', message: 'email or username already taken' };
  }
  const id = allocUserId();
  const idx = d.users.length;
  const color = CHAT_COLORS[idx % CHAT_COLORS.length];
  const user = { id, username: username.trim(), email: normalizedEmail, avatar_color: color, avatar_url: '', status: 'online', last_seen: now(), created_at: now() };
  d.users.push(user);
  return {
    user,
    access_token: 'mock-token-' + id,
    expires_in: 3600,
  };
}

export function mockLogin(_token, email, password) {
  const d = ensureData();
  const normalizedEmail = email.toLowerCase().trim();
  let user = d.users.find(u => u.email === normalizedEmail);
  if (!user) {
    user = MOCK_USERS.find(u => u.email.toLowerCase() === normalizedEmail);
  }
  if (!user) {
    throw { status: 401, error: 'invalid_credentials', message: 'invalid email or password' };
  }
  return {
    user: { ...user },
    access_token: 'mock-token-' + user.id,
    expires_in: 3600,
  };
}

export function mockRefresh() {
  const cu = currentUser();
  return {
    user: userById(cu.id),
    access_token: 'mock-token-' + cu.id,
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

// ── Users ──

export function mockUpdateProfile(_token, data) {
  const cu = currentUser();
  const d = ensureData();
  const username = data.username ? data.username.trim() : cu.username;
  if (data.username && d.users.some(u => u.username === username && u.id !== cu.id)) {
    throw { status: 409, error: 'username_taken', message: 'username already taken' };
  }
  if (!username) {
    throw { status: 400, error: 'invalid_username', message: 'username required' };
  }
  const updated = {
    ...cu,
    username,
    avatar_color: data.avatar_color || cu.avatar_color,
    avatar_url: data.avatar_url || cu.avatar_url || '',
  };
  if (d.users) {
    const ui = d.users.findIndex(u => u.id === cu.id);
    if (ui >= 0) d.users[ui] = { ...d.users[ui], ...updated };
  }
  d.chats.forEach(chat => {
    const mi = chat.members?.findIndex(m => m.id === updated.id);
    if (mi !== -1 && mi !== undefined) {
      chat.members[mi] = { ...chat.members[mi], ...updated };
      if (_store) _store.getState().onChatUpdate({ id: chat.id, members: [...chat.members] });
    }
  });
  if (_store) _store.setState({ user: updated });
  // Broadcast user_update to all (via onChatUpdate for each chat's members)
  return updated;
}

export function mockSearchUsers(_token, q) {
  const cu = currentUser();
  const d = ensureData();
  const allUsers = [...MOCK_USERS, ...(d.users || [])];
  const lowerQ = q.toLowerCase();
  const results = allUsers
    .filter(u => u.id !== cu.id && (u.username?.toLowerCase().includes(lowerQ) || u.id === q))
    .slice(0, 20);
  return { users: results };
}

// ── Chats ──

export function mockListChats() {
  const cu = currentUser();
  const d = ensureData();
  const chatMembers = d.chatMembers || [];
  const myChatIds = new Set(
    chatMembers.filter(cm => cm.user_id === cu.id).map(cm => cm.chat_id)
  );
  const enriched = d.chats
    .filter(c => c.members?.some(m => m.id === cu.id) || myChatIds.has(c.id))
    .map(c => buildChatResponse(c));
  enriched.sort((a, b) => new Date(b.last_message_at || b.created_at) - new Date(a.last_message_at || a.created_at));
  return { chats: enriched };
}

export function mockListPublicChats() {
  const d = ensureData();
  return { chats: d.chats.filter(c => c.visibility === 'public').map(c => buildChatResponse(c)) };
}

export function mockGetChat(_token, id) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === id);
  if (!chat) return { error: 'not_found' };
  const isMember = chat.members?.some(m => m.id === cu.id);
  const chatMembers = d.chatMembers || [];
  const isMember2 = chatMembers.some(cm => cm.chat_id === id && cm.user_id === cu.id);
  if (!isMember && !isMember2) {
    throw { status: 403, error: 'forbidden', message: 'not a member' };
  }
  return buildChatResponse(chat);
}

export function mockCreateChat(_token, name, memberIds, visibility) {
  const cu = currentUser();
  const d = ensureData();
  if (!name || !name.trim()) {
    throw { status: 400, error: 'bad_request', message: 'name required' };
  }
  const members = [...(memberIds || [])];
  if (!members.some(m => m === cu.id)) {
    members.push(cu.id);
  }
  const id = randid();
  const color = CHAT_COLORS[d.chats.length % CHAT_COLORS.length];
  const newChat = {
    id,
    type: 'group',
    name: name.trim(),
    icon_color: color,
    owner_id: cu.id,
    visibility: visibility || 'private',
    created_at: now(),
    last_message_at: null,
    last_message_id: null,
    member_count: members.length,
    pinned_message: null,
    members: members.map(mid => ({ id: mid, ...userById(mid), role: mid === cu.id ? 'owner' : 'member' })),
  };
  d.chats.unshift(newChat);
  // Add to internal chatMembers
  if (!d.chatMembers) d.chatMembers = [];
  members.forEach(mid => {
    if (!d.chatMembers.some(cm => cm.chat_id === id && cm.user_id === mid)) {
      d.chatMembers.push({ chat_id: id, user_id: mid, role: mid === cu.id ? 'owner' : '', joined_at: now() });
    }
  });
  if (_store) _store.getState().onChatUpdate(newChat);
  return buildChatResponse(newChat);
}

export function mockCreateDM(_token, userId) {
  const cu = currentUser();
  const d = ensureData();
  if (!userId || userId === cu.id) {
    throw { status: 400, error: 'bad_request', message: 'invalid user_id' };
  }
  const target = userById(userId);
  if (!target || target.id === 'Unknown') {
    throw { status: 404, error: 'user_not_found', message: '' };
  }
  // Find existing DM
  const existing = d.chats.find(c =>
    c.type === 'dm' &&
    c.members?.some(m => m.id === cu.id) &&
    c.members?.some(m => m.id === userId)
  );
  if (existing) return buildChatResponse(existing);

  const id = randid();
  const newDM = {
    id,
    type: 'dm',
    name: '',
    icon_color: '#5865F2',
    owner_id: '',
    visibility: '',
    created_at: now(),
    last_message_at: null,
    last_message_id: null,
    member_count: 2,
    pinned_message: null,
    members: [
      { id: cu.id, ...userById(cu.id), role: 'member' },
      { id: userId, ...userById(userId), role: 'member' },
    ],
  };
  d.chats.unshift(newDM);
  if (!d.chatMembers) d.chatMembers = [];
  [cu.id, userId].forEach(mid => {
    if (!d.chatMembers.some(cm => cm.chat_id === id && cm.user_id === mid)) {
      d.chatMembers.push({ chat_id: id, user_id: mid, role: '', joined_at: now() });
    }
  });
  if (_store) _store.getState().onChatUpdate(newDM);
  return buildChatResponse(newDM);
}

export function mockRenameChat(_token, _id, name) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === _id);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  if (chat.type === 'dm') throw { status: 400, error: 'bad_request', message: 'cannot rename dm' };
  if (chat.owner_id !== cu.id) throw { status: 403, error: 'forbidden', message: 'only owner can rename' };
  if (!name || !name.trim()) throw { status: 400, error: 'bad_request', message: 'name required' };
  chat.name = name.trim();
  const updated = buildChatResponse(chat);
  if (_store) _store.getState().onChatUpdate(updated);
  return updated;
}

export function mockDeleteChat(_token, id) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === id);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  if (chat.type === 'dm') throw { status: 400, error: 'bad_request', message: 'cannot delete dm; leave instead' };
  if (chat.owner_id !== cu.id) throw { status: 403, error: 'forbidden', message: 'only owner can delete' };
  d.chats = d.chats.filter(c => c.id !== id);
  if (d.chatMembers) d.chatMembers = d.chatMembers.filter(cm => cm.chat_id !== id);
  if (_store) _store.getState().onChatUpdate({ id, deleted: true });
  return { ok: true };
}

export function mockJoinChat(_token, chatId) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  const vis = chat.visibility || 'private';
  if (vis !== 'public' && vis !== 'unlisted') {
    throw { status: 400, error: 'bad_request', message: 'chat is private, invitation required' };
  }
  if (!d.chatMembers) d.chatMembers = [];
  if (!d.chatMembers.some(cm => cm.chat_id === chatId && cm.user_id === cu.id)) {
    d.chatMembers.push({ chat_id: chatId, user_id: cu.id, role: '', joined_at: now() });
  }
  if (!chat.members) chat.members = [];
  if (!chat.members.some(m => m.id === cu.id)) {
    chat.members.push({ id: cu.id, ...userById(cu.id), role: 'member' });
  }
  const updated = buildChatResponse(chat);
  if (_store) _store.getState().onChatUpdate(updated);
  return { ok: true };
}

export function mockSetPinnedMessage(_token, chatId, content) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  if (chat.owner_id !== cu.id) throw { status: 403, error: 'forbidden', message: '' };
  const mc = chat.members?.length || 0;
  if (mc < 3) throw { status: 400, error: 'bad_request', message: 'need at least 3 members to pin' };
  chat.pinned_message = { content, pinned_at: now() };
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: chat.pinned_message });
  return { ok: true };
}

export function mockClearPinnedMessage(_token, chatId) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  const isOwner = chat.owner_id === cu.id;
  const cm = (d.chatMembers || []).find(cm => cm.chat_id === chatId && cm.user_id === cu.id);
  const isAdmin = cm?.role === 'admin';
  if (!isOwner && !isAdmin) throw { status: 403, error: 'forbidden', message: '' };
  chat.pinned_message = null;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: null });
  return { ok: true };
}

// ── Members ──

export function mockListMembers(_token, chatId) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { members: [] };
  const isMember = chat.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };
  const members = (chat.members || []).map(m => ({ ...m, ...userById(m.id) }));
  members.sort((a, b) => (a.username || '').localeCompare(b.username || ''));
  return { members };
}

export function mockAddMember(_token, chatId, userId) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  if (chat.type === 'dm') throw { status: 400, error: 'bad_request', message: 'cannot add to dm' };
  const isMember = chat.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };
  const target = userById(userId);
  if (!target || target.id === 'Unknown') throw { status: 404, error: 'user_not_found', message: '' };
  if (chat.members?.some(m => m.id === userId)) throw { status: 409, error: 'already_member', message: '' };
  if (!chat.members) chat.members = [];
  chat.members.push({ id: userId, ...target, role: 'member' });
  if (!d.chatMembers) d.chatMembers = [];
  if (!d.chatMembers.some(cm => cm.chat_id === chatId && cm.user_id === userId)) {
    d.chatMembers.push({ chat_id: chatId, user_id: userId, role: '', joined_at: now() });
  }
  const updated = buildChatResponse(chat);
  if (_store) _store.getState().onChatUpdate(updated);
  return updated;
}

export function mockRemoveMember(_token, chatId, userId) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) throw { status: 404, error: 'not_found', message: '' };
  if (chat.type === 'dm') throw { status: 400, error: 'bad_request', message: 'cannot remove from dm' };
  if (userId === chat.owner_id && userId !== cu.id) {
    throw { status: 403, error: 'forbidden', message: 'cannot kick owner' };
  }
  if (userId !== cu.id) {
    const isOwner = chat.owner_id === cu.id;
    const cm = (d.chatMembers || []).find(cm => cm.chat_id === chatId && cm.user_id === cu.id);
    const isAdmin = cm?.role === 'admin';
    if (!isOwner && !isAdmin) {
      throw { status: 403, error: 'forbidden', message: 'only owner or admin can kick others' };
    }
  }
  if (chat.members) chat.members = chat.members.filter(m => m.id !== userId);
  if (d.chatMembers) d.chatMembers = d.chatMembers.filter(cm => !(cm.chat_id === chatId && cm.user_id === userId));
  if (_store) _store.getState().onChatUpdate({ id: chatId, members: [...(chat.members || [])] });
  return { ok: true };
}

// ── Messages ──

export function mockListMessages(_token, chatId, before, limit) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  const isMember = chat?.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };

  let all = messagesFor(chatId).map(m => {
    const author = userById(m.user_id);
    const cuId = cu.id;
    const rxs = (m.reactions || []).map(r => ({
      ...r,
      me: (r.user_ids || []).includes(cuId),
    }));
    return {
      ...m,
      author,
      reactions: rxs,
      deleted: !!m.deleted_at,
      deleted_at: m.deleted_at || null,
      edited_at: m.edited_at || null,
      attachment_count: m.attachment_count || (m.attachments?.length || 0),
      mention_count: m.mention_count || 0,
      reaction_count: m.reaction_count || rxs.length,
    };
  });

  if (before) {
    const idx = all.findIndex(m => m.id === before);
    if (idx <= 0) return { messages: [] };
    const start = Math.max(0, idx - (limit || 50));
    all = all.slice(start, idx);
  } else {
    const total = all.length;
    const pageSize = limit || 50;
    const capped = Math.min(pageSize, 100);
    const start = Math.max(0, total - capped);
    all = all.slice(start, total);
  }

  return { messages: all };
}

export function mockSendMessage(_token, chatId, content, attachments) {
  const cu = currentUser();
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  const isMember = chat?.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };

  const trimmed = (content || '').trimRight();
  if (trimmed.length > 4000) throw { status: 403, error: 'content_too_long', message: 'content too long, use file upload instead' };
  if (trimmed === '' && (!attachments || attachments.length === 0)) {
    throw { status: 400, error: 'bad_request', message: 'empty message' };
  }

  const msgId = randid();
  const createdAt = now();

  const atts = (attachments || []).map(a => ({
    ...a,
    id: a.id || randid(),
    mime_type: a.mime_type || 'application/octet-stream',
  }));

  // Extract mentions: <@uuid>
  const mentionRe = /<@([a-f0-9-]{36})>/g;
  const mentions = [];
  let match;
  while ((match = mentionRe.exec(trimmed)) !== null) {
    if (!mentions.includes(match[1])) mentions.push(match[1]);
  }

  const userMsg = {
    id: msgId,
    chat_id: chatId,
    content: trimmed,
    user_id: cu.id,
    author: cu,
    created_at: createdAt,
    edited_at: null,
    deleted_at: null,
    deleted: false,
    attachments: atts,
    reactions: [],
    mentions,
    attachment_count: atts.length,
    mention_count: mentions.length,
    reaction_count: 0,
  };
  d.messages.push(userMsg);

  // Update chat's last_message_at and last_message_id
  chat.last_message_at = createdAt;
  chat.last_message_id = msgId;

  // Update member last_seen
  if (!d.chatMembers) d.chatMembers = [];
  const cm = d.chatMembers.find(cm => cm.chat_id === chatId && cm.user_id === cu.id);
  if (cm) cm.last_seen = createdAt;

  if (_store) _store.getState().onMessageCreate(userMsg);

  // AI response
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
    deleted_at: null,
    deleted: false,
    attachments: [],
    reactions: [],
    mentions: [],
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
    deleted_at: null,
    deleted: false,
    attachments: [],
    reactions: [],
    mentions: [],
    streaming: true,
  };

  setTimeout(() => {
    d.messages.push(aiDataMsg);
    if (_store) _store.getState().onMessageCreate(aiStoreMsg);
  }, 500 + Math.random() * 800);

  return userMsg;
}

export function mockEditMessage(_token, _chatId, msgId, content) {
  const cu = currentUser();
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (!msg) throw { status: 404, error: 'not_found', message: '' };
  if (msg.user_id !== cu.id) throw { status: 403, error: 'forbidden', message: '' };
  const trimmed = (content || '').trimRight();
  if (trimmed === '') throw { status: 400, error: 'bad_request', message: 'empty content' };
  if (trimmed.length > 4000) throw { status: 400, error: 'bad_request', message: 'content too long, use file upload instead' };
  if (msg.chat_id !== _chatId) throw { status: 400, error: 'bad_request', message: 'chat mismatch' };
  msg.content = trimmed;
  msg.edited_at = now();
  const updated = { ...msg, author: userById(msg.user_id) };
  if (_store) _store.getState().onMessageUpdate({ id: msgId, content: trimmed, edited_at: msg.edited_at });
  return updated;
}

export function mockDeleteMessage(_token, _chatId, msgId) {
  const cu = currentUser();
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (!msg) throw { status: 404, error: 'not_found', message: '' };
  if (msg.chat_id !== _chatId) throw { status: 400, error: 'bad_request', message: 'chat mismatch' };
  const chat = d.chats.find(c => c.id === _chatId);
  const canDeleteAny = chat && (chat.owner_id === cu.id || (d.chatMembers || []).some(cm => cm.chat_id === _chatId && cm.user_id === cu.id && cm.role === 'admin'));
  if (msg.user_id !== cu.id && !canDeleteAny) {
    throw { status: 403, error: 'forbidden', message: '' };
  }
  msg.deleted_at = now();
  msg.content = '';
  if (_store) _store.getState().onMessageDelete({ message_id: msgId, chat_id: _chatId });
  return { ok: true };
}

export function mockMarkRead() {
  return { ok: true };
}

// ── Reactions ──

export function mockAddReaction(_token, _chatId, msgId, emoji) {
  const cu = currentUser();
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (!msg) throw { status: 404, error: 'not_found', message: '' };
  if (msg.chat_id !== _chatId) throw { status: 400, error: 'bad_request', message: 'chat mismatch' };
  const chat = d.chats.find(c => c.id === _chatId);
  const isMember = chat?.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };

  const trimmed = (emoji || '').trim();
  if (!trimmed) throw { status: 400, error: 'bad_request', message: 'emoji required' };
  if (trimmed.length > 32) throw { status: 400, error: 'bad_request', message: 'emoji too long' };

  if (!d.reactions) d.reactions = [];
  if (!d.reactions.some(r => r.message_id === msgId && r.user_id === cu.id && r.emoji === trimmed)) {
    d.reactions.push({ message_id: msgId, user_id: cu.id, emoji: trimmed, created_at: now() });
  }
  syncReactionsColumn(msgId);
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji: trimmed, user_id: cu.id }, true);
  const updated = { ...msg, reactions: msg.reactions.map(r => ({ ...r, me: r.user_ids?.includes(cu.id) })) };
  return updated;
}

export function mockRemoveReaction(_token, _chatId, msgId, emoji) {
  const cu = currentUser();
  const d = ensureData();
  const msg = d.messages.find(m => m.id === msgId);
  if (!msg) throw { status: 404, error: 'not_found', message: '' };
  const chat = d.chats.find(c => c.id === _chatId);
  const isMember = chat?.members?.some(m => m.id === cu.id);
  if (!isMember) throw { status: 403, error: 'forbidden', message: '' };

  if (d.reactions) {
    d.reactions = d.reactions.filter(r => !(r.message_id === msgId && r.user_id === cu.id && r.emoji === emoji));
  }
  syncReactionsColumn(msgId);
  if (_store) _store.getState().onReaction({ message_id: msgId, emoji, user_id: cu.id }, false);
  const updated = { ...msg, reactions: msg.reactions.map(r => ({ ...r, me: r.user_ids?.includes(cu.id) })) };
  return updated;
}

// ── Uploads ──

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

// ── Toggle Pin (legacy, for frontend compatibility) ──

export function mockTogglePin(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: false };
  chat.pinned = !chat.pinned;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned: chat.pinned });
  return { ok: true, pinned: chat.pinned };
}
