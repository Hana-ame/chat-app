/**
 * @typedef {import('../schemas').User} User
 * @typedef {import('../schemas').Chat} Chat
 * @typedef {import('../schemas').Message} Message
 * @typedef {import('../schemas').Reaction} Reaction
 * @typedef {import('../schemas').Attachment} Attachment
 * @typedef {import('../schemas').PinnedContent} PinnedContent
 * @typedef {import('../schemas').NotificationOccurrence} NotificationOccurrence
 * @typedef {import('../schemas').PushSubscription} PushSubscription
 */

import { generateDummyData } from '../dev/dummy';

/** @returns {string} */
function randid() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

/** @type {string[]} */
const CHAT_COLORS = [
  '#5865F2', '#23a559', '#f0b232', '#ed4245', '#9b59b6',
  '#1abc9c', '#e67e22', '#2ecc71', '#e74c3c', '#3498db',
  '#f39c12', '#1dd1a1', '#a29bfe', '#fd79a8', '#00cec9',
];

/** @returns {User} */
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

/** @type {string[]} */
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

/**
 * @type {{ chats: Chat[], messages: Message[], onlineUserIds: string[], notificationOccurrences: NotificationOccurrence[], pushSubscriptions: PushSubscription[] }|null}
 */
let data = null;
/**
 * 【本地改动 2026-08-31】线程 mock 状态。
 * @type {Map<string, Set<string>>}  thread_root_message_id -> Set(user_id)
 */
let threadFollows = new Map();
/**
 * @type {Map<string, string>}  `${user_id}:${thread_root}` -> last-seen message id
 */
let threadReadState = new Map();
/** @type {import('zustand').UseBoundStore<import('zustand').StoreApi<any>>|null} */
let _store = null;
import('../store/chat').then(m => { _store = m.useChatStore; }).catch(() => {});

/** @type {User[]} */
const MOCK_USERS = [
  { id: 'dev-self', username: 'Alice', avatar_color: '#5865F2', email: 'alice@test.com', role: 'owner', last_seen: new Date().toISOString() },
  { id: 'dev-bob', username: 'Bob', avatar_color: '#23a559', email: 'bob@test.com', role: 'admin', last_seen: new Date().toISOString() },
  { id: 'dev-carol', username: 'Carol', avatar_color: '#f0b232', email: 'carol@test.com', role: 'admin', last_seen: new Date().toISOString() },
  { id: 'dev-dave', username: 'Dave', avatar_color: '#ed4245', email: 'dave@test.com', role: '', last_seen: new Date(Date.now() - 60000).toISOString() },
  { id: 'dev-eve', username: 'Eve', avatar_color: '#9b59b6', email: 'eve@test.com', role: '', last_seen: new Date().toISOString() },
  { id: 'dev-frank', username: 'Frank', avatar_color: '#1abc9c', email: 'frank@test.com', role: '', last_seen: new Date(Date.now() - 86400000).toISOString() },
  { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f', email: '', role: '', last_seen: new Date().toISOString() },
];

/** @returns {{ chats: Chat[], messages: Message[], notificationOccurrences: NotificationOccurrence[], pushSubscriptions: PushSubscription[] }} */
function ensureData() {
  if (!data) {
    const gen = generateDummyData({ chatCount: 10, msgPerChat: 150 });
    data = /** @type {any} */ ({ chats: gen.chats, messages: [...(gen.messages || [])], onlineUserIds: gen.onlineUserIds || [], notificationOccurrences: [], pushSubscriptions: [] });
    if (_store) _store.setState({ onlineUserIds: data.onlineUserIds });
  }
  return data;
}

export function resetMockData() {
  data = null;
  threadFollows.clear();
  threadReadState.clear();
  ensureData();
}

/**
 * @param {string} id
 * @returns {User}
 */
function userById(id) {
  return MOCK_USERS.find(u => u.id === id) || { id, username: 'Unknown', avatar_color: '#5865F2', email: '' };
}

/**
 * @param {string} chatId
 * @returns {Message[]}
 */
function messagesFor(chatId) {
  const d = ensureData();
  return d.messages.filter(m => m.chat_id === chatId).sort((a, b) => +new Date(a.created_at) - +new Date(b.created_at));
}

/** @returns {{ chats: Chat[] }} */
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
      members: c.members || [],
    };
  });
  return /** @type {any} */ ({ chats: enriched });
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} [before]
 * @param {number} [limit]
 * @returns {{ messages: Message[] }}
 */
export function mockListMessages(_token, chatId, before = undefined, limit = 50, inThread = undefined) {
  const all = inThread ? messagesFor(chatId).filter(m => m.thread_root_message_id === inThread) : messagesFor(chatId);
  const cu = currentUser();
  const mapped = all.map(msg => ({
    ...msg,
    reactions: (msg.reactions || []).map(r => ({
      ...r,
      me: r.user_ids?.includes(cu.id) || false,
    })),
  }));
  if (before) {
    const idx = mapped.findIndex(m => m.id === before);
    if (idx <= 0) return { messages: [] };
    const start = Math.max(0, idx - (limit || 50));
    return { messages: mapped.slice(start, idx) };
  }
  const total = mapped.length;
  const pageSize = limit || 50;
  const start = Math.max(0, total - pageSize);
  return { messages: mapped.slice(start, total) };
}

/**
 * @param {string} _token
 * @param {string} id
 * @returns {Chat & { members: User[] }}
 */
export function mockGetChat(_token, id) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === id);
  if (!chat) return /** @type {any} */ ({ error: 'not_found' });
  return { ...chat, members: chat.members || [] };
}

/**
 * @param {string} _token
 * @param {string} name
 * @param {string[]} memberIds
 * @param {string} [visibility]
 * @returns {Chat}
 */
export function mockCreateChat(_token, name, memberIds, visibility) {
  const d = ensureData();
  const cu = currentUser();
  const members = [{ id: cu.id, ...userById(cu.id), role: 'owner' }, ...(memberIds || []).map(id => ({ id, ...userById(id), role: '' }))];
  const newChat = {
    id: randid(),
    type: 'group',
    name,
    icon_color: CHAT_COLORS[d.chats.length % CHAT_COLORS.length],
    owner_id: cu.id,
    visibility: visibility || 'private',
    pinned: false,
    created_at: new Date().toISOString(),
    last_message_at: null,
    member_count: members.length,
    members,
  };
  d.chats.unshift(newChat);
  if (_store) _store.getState().onChatUpdate(newChat);
  return newChat;
}

/**
 * @param {string} _token
 * @param {string} id
 * @returns {{ ok: boolean }}
 */
export function mockDeleteChat(_token, id) {
  const d = ensureData();
  d.chats = d.chats.filter(c => c.id !== id);
  if (_store) _store.getState().onChatDelete({ chat_id: id });
  return { ok: true };
}

// @deprecated DM 已下线。保留函数体供日后恢复。
export function mockCreateDM(_token, userId) {
  const d = ensureData();
  const cu = currentUser();
  const existing = d.chats.find(c => c.type === 'dm' && c.members?.some(m => m.id === userId));
  if (existing) return existing;
  const dmMembers = [
    { id: cu.id, ...userById(cu.id), role: '' },
    { id: userId, ...userById(userId), role: '' },
  ];
  const newDM = {
    id: randid(),
    type: 'dm',
    name: '',
    icon_color: '#5865F2',
    owner_id: '',
    visibility: 'private',
    created_at: new Date().toISOString(),
    member_count: dmMembers.length,
    members: dmMembers,
  };
  d.chats.unshift(newDM);
  if (_store) _store.getState().onChatUpdate(newDM);
  return newDM;
}

/** @param {Partial<Chat>} chat */
function syncChatToStore(chat) {
  if (!_store) return;
  _store.getState().onChatUpdate({ ...chat });
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} userId
 * @returns {Chat|{ ok: boolean }}
 */
export function mockAddMember(_token, chatId, userId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (chat && !chat.members?.some(m => m.id === userId)) {
    if (!chat.members) chat.members = [];
    chat.members.push({ id: userId, ...userById(userId), role: '' });
    chat.member_count = (chat.member_count || 0) + 1;
    syncChatToStore(chat);
  }
  return chat || { ok: true };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} userId
 * @returns {{ ok: boolean }}
 */
export function mockRemoveMember(_token, chatId, userId) {
  const d = ensureData();
  const cu = currentUser();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: true };
  chat.members = chat.members?.filter(m => m.id !== userId);
  chat.member_count = Math.max(0, (chat.member_count || 0) - 1);
  if (userId === cu.id) {
    if (_store) _store.getState().onChatDelete({ chat_id: chatId });
  } else {
    syncChatToStore(chat);
  }
  return { ok: true };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ members: User[] }}
 */
export function mockListMembers(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  const members = chat?.members || [];
  return { members };
}

/**
 * @param {string} _token
 * @param {string} q
 * @returns {{ users: User[] }}
 */
export function mockSearchUsers(_token, q) {
  const cu = currentUser();
  const results = MOCK_USERS
    .filter(u => u.id !== cu.id && u.username?.toLowerCase().includes(q.toLowerCase()))
    .slice(0, 20);
  return { users: results };
}

/**
 * @param {string} _token
 * @param {{ username?: string, avatar_color?: string, avatar_url?: string }} data
 * @returns {User}
 */
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

/**
 * AI 流式消息的占位事件(mock 模式)。
 *
 * 生产链路:客户端 POST /api/chats/:id/messages(type=stream)后,后端先通过
 * WS 广播一条 id 为 msg_id、streaming=true 的 message_create 占位消息,再以
 * SSE 流式返回正文;Composer 的 streamAI 把分片内容累积进同一条占位消息。
 *
 * mock 模式没有 WS,因此由这里补发等价的 onMessageCreate 事件,让 UI 先出现
 * 占位气泡;真正的 SSE 请求体与流式正文仍由真实 fetch 完成(测试用
 * page.route 拦截断言)。注意:占位消息只进 store,不写 d.messages ——
 * 轮询 reload 时 _mergeMessages 会保留 store 已有消息,不会被清掉。
 *
 * @param {string} chatId
 * @param {string} msgId
 */
export function mockEmitStreamPlaceholder(chatId, msgId) {
  /** @type {Message} */
  const placeholder = {
    id: msgId,
    chat_id: chatId,
    content: '',
    user_id: 'ai',
    author: userById('ai'),
    created_at: new Date().toISOString(),
    edited_at: null,
    deleted: false,
    attachments: [],
    reactions: [],
    streaming: true,
  };
  if (_store) _store.getState().onMessageCreate(placeholder);
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} content
 * @param {Attachment[]} [attachments]
 * @returns {Message}
 */
export function mockSendMessage(_token, chatId, content, attachments = undefined, _replyTo = undefined, _threadRoot = undefined, _startThread = false) {
  const d = ensureData();
  const now = new Date().toISOString();

  // 【本地改动 2026-08-31】线程字段：startThread=true → 自引用为根；
  // 显式 threadRoot → 加入既有线程；给 replyTo 但没 threadRoot → 尝试继承父消息的根。
  let threadRootMessageId = '';
  if (_startThread) {
    // 先写消息占位，回填自引用在下方。
  } else if (_threadRoot) {
    threadRootMessageId = _threadRoot;
  } else if (_replyTo) {
    const parent = d.messages.find(m => m.id === _replyTo);
    if (parent) threadRootMessageId = parent.thread_root_message_id || '';
  }

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
    reply_to: _replyTo || '',
    thread_root_message_id: threadRootMessageId || '',
  };
  if (_startThread) userMsg.thread_root_message_id = userMsg.id;
  d.messages.push(userMsg);
  if (_store) _store.getState().onMessageCreate(userMsg);

  if (Math.random() < 0.5) {
    const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
    const aiId = randid();
    const aiCreatedAt = new Date(Date.now() + 1).toISOString();

    /** @type {Message} */
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
      source: /** @type {any} */ (async (emit) => {
        let acc = '';
        for (let i = 0; i < text.length; i++) {
          await new Promise(r => setTimeout(r, 25 + 10 * i));
          emit(text[i]);
          acc += text[i];
          const m = d.messages.find(m => m.id === aiId);
          if (m) m.content = acc;
        }
        const m = d.messages.find(m => m.id === aiId);
        if (m) m.streaming = false;
      }),
    };

    /** @type {Message} */
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
    }, 500);
  }

  return userMsg;
}

/**
 * @param {string} _token
 * @param {string} _chatId
 * @param {string} msgId
 * @param {string} content
 * @returns {{ ok: boolean }}
 */
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

/**
 * @param {string} _token
 * @param {string} _chatId
 * @param {string} msgId
 * @returns {{ ok: boolean }}
 */
export function mockDeleteMessage(_token, _chatId, msgId) {
  const d = ensureData();
  d.messages = d.messages.filter(m => m.id !== msgId);
  if (_store) _store.getState().onMessageDelete({ message_id: msgId, chat_id: _chatId });
  return { ok: true };
}

/**
 * @param {string} _token
 * @param {string} _chatId
 * @param {string} msgId
 * @param {string} emoji
 * @returns {{ ok: boolean }}
 */
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

/**
 * @param {string} _token
 * @param {string} _chatId
 * @param {string} msgId
 * @param {string} emoji
 * @returns {{ ok: boolean }}
 */
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

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} content
 * @returns {{ ok: boolean, pinned_message?: PinnedContent, pinned_updated_at?: string }}
 */
export function mockSetAnnouncement(_token, chatId, content) {
  const now = new Date().toISOString();
  const pinned = { id: 'pm-' + randid(), content, pinned_at: now };
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: pinned, pinned_updated_at: now });
  return { ok: true, pinned_message: pinned, pinned_updated_at: now };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ ok: boolean }}
 */
export function mockClearAnnouncement(_token, chatId) {
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_message: null, pinned_updated_at: null });
  return { ok: true };
}

/** @returns {{ ok: boolean }} */
export function mockMarkRead() {
  return { ok: true };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ ok: boolean, error?: string }}
 */
export function mockJoinChat(_token, chatId) {
  const d = ensureData();
  const cu = currentUser();
  const chat = d.chats.find(c => c.id === chatId);
      if (!chat) return /** @type {any} */ ({ error: 'not_found', ok: false });
  if (chat.visibility !== 'public' && chat.visibility !== 'unlisted') {
    return /** @type {any} */ ({ error: 'private', ok: false });
  }
  if (!chat.members?.some(m => m.id === cu.id)) {
    if (!chat.members) chat.members = [];
    chat.members.push({ id: cu.id, ...userById(cu.id), role: 'member' });
    chat.member_count = (chat.member_count || 0) + 1;
    syncChatToStore(chat);
  }
  return { ok: true };
}

/**
 * @param {string} _token
 * @param {string} email
 * @param {string} username
 * @param {string} password
 * @returns {{ user: User, access_token: string, expires_in: number }}
 */
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

/**
 * @param {string} _token
 * @param {string} email
 * @param {string} password
 * @returns {{ user: User, access_token: string, expires_in: number }}
 */
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

/** @returns {{ user: User, access_token: string, expires_in: number }} */
export function mockRefresh() {
  const cu = currentUser();
  return {
    user: userById(cu.id),
    access_token: 'mock-token',
    expires_in: 3600,
  };
}

/** @returns {{ ok: boolean }} */
export function mockLogout() {
  return { ok: true };
}

/** @returns {User} */
export function mockMe() {
  const cu = currentUser();
  return userById(cu.id);
}

/**
 * @param {string} _token
 * @param {number} [page]
 * @param {number} [limit]
 * @returns {{ chats: Chat[] }}
 */
export function mockListPublicChats(_token, page = 1, limit = 20) {
  const d = ensureData();
  const all = d.chats.filter(c => c.visibility === 'public')
    .map(c => {
      const msgs = messagesFor(c.id).filter(m => m.content?.trim());
      const last = msgs[msgs.length - 1];
      let lastMsgContent = '';
      if (last?.content) {
        lastMsgContent = last.content.length > 100 ? last.content.slice(0, 100) + '...' : last.content;
      }
      return { ...c, last_message: lastMsgContent ? /** @type {any} */ ({ content: lastMsgContent }) : undefined };
    })
    .sort((a, b) => {
      const da = a.last_message_at || a.created_at;
      const db = b.last_message_at || b.created_at;
      return +new Date(db) - +new Date(da);
    });
  const start = (page - 1) * limit;
  const chats = all.slice(start, start + limit);
  return { chats };
}

/**
 * @param {string} _token
 * @param {string} _id
 * @param {string} name
 * @returns {{ ok: boolean }}
 */
export function mockRenameChat(_token, _id, name) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === _id);
  if (chat) chat.name = name;
  if (_store) _store.getState().onChatUpdate({ id: _id, name });
  return { ok: true };
}

/**
 * @param {File} file
 * @returns {{ id: string, filename: string, mime_type: string, size: number, url: string, delete_url: string }}
 * 注：mock 模式用浏览器 ObjectURL（blob://），不模拟 /assets/files/ 磁盘路径；
 * 生产环境上传响应为 {id: uuid, url: /assets/files/{uuid}/{fn.ext}, delete_url: /api/files/{uuid}}。
 */
export function mockUpload(file) {
  const ext = file?.name?.split('.').pop() || 'bin';
  return {
    id: crypto.randomUUID(),
    filename: file?.name || 'file.' + ext,
    mime_type: file?.type || 'application/octet-stream',
    size: file?.size || 0,
    url: URL.createObjectURL(file),
    delete_url: '',
  };
}

/**
 * @param {string} _token
 * @param {File} file
 * @returns {{ url: string }}
 */
export function mockUploadAvatar(_token, file) {
  return { url: URL.createObjectURL(file) };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ ok: boolean, pinned: boolean }}
 */
export function mockPinChat(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: false, pinned: false };
  chat.pinned = true;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned: true });
  return { ok: true, pinned: true };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ ok: boolean, pinned: boolean }}
 */
export function mockUnpinChat(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: false, pinned: false };
  chat.pinned = false;
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned: false });
  return { ok: true, pinned: false };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @returns {{ ok: boolean }}
 */
export function mockMarkAnnouncementRead(_token, chatId) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { ok: true };
  chat.pinned_last_read_at = new Date().toISOString();
  if (_store) _store.getState().onChatUpdate({ id: chatId, pinned_last_read_at: chat.pinned_last_read_at });
  return { ok: true };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} avatarUrl
 * @returns {{ id: string, avatar_url: string }}
 */
export function mockUpdateChatAvatar(_token, chatId, avatarUrl) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { id: chatId, avatar_url: avatarUrl };
  chat.avatar_url = avatarUrl;
  if (_store) _store.getState().onChatUpdate({ id: chatId, avatar_url: avatarUrl });
  return { id: chatId, avatar_url: avatarUrl };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} bannerUrl
 * @returns {{ id: string, banner_url: string }}
 */
export function mockUpdateChatBanner(_token, chatId, bannerUrl) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { id: chatId, banner_url: bannerUrl };
  chat.banner_url = bannerUrl;
  if (_store) _store.getState().onChatUpdate({ id: chatId, banner_url: bannerUrl });
  return { id: chatId, banner_url: bannerUrl };
}

/**
 * @param {string} _token
 * @param {string} chatId
 * @param {string} backgroundUrl
 * @returns {{ id: string, background_url: string }}
 */
export function mockUpdateChatBackground(_token, chatId, backgroundUrl) {
  const d = ensureData();
  const chat = d.chats.find(c => c.id === chatId);
  if (!chat) return { id: chatId, background_url: backgroundUrl };
  chat.background_url = backgroundUrl;
  if (_store) _store.getState().onChatUpdate({ id: chatId, background_url: backgroundUrl });
  return { id: chatId, background_url: backgroundUrl };
}

/**
 * @param {string} _token
 * @returns {Chat}
 */
export function mockGetNotifyChat(_token) {
  const d = ensureData();
  const cu = currentUser();
  let chat = d.chats.find(c => c.type === 'notify');
  if (!chat) {
    const members = [{ id: cu.id, ...userById(cu.id), role: 'owner' }];
    chat = {
      id: randid(),
      type: 'notify',
      name: 'Notifications',
      icon_color: '#E8590C',
      owner_id: cu.id,
      created_at: new Date(0).toISOString(),
      last_message_at: new Date(0).toISOString(),
      member_count: 1,
      visibility: '',
      pinned_message: null,
      pinned_updated_at: null,
      pinned_last_read_at: null,
      pinned: false,
      notify_enabled: true,
      last_active_at: new Date().toISOString(),
      last_message_id: '',
      unread_count: 0,
      members,
    };
    d.chats.push(chat);
    if (_store) _store.getState().onChatUpdate(chat);
  }
  return { ...chat, members: chat.members || [] };
}



/**
 * Mock notifications list (empty by default; messages sent to the
 * Notifications chat are visible here).
 * @returns {{ messages: Message[] }}
 */
export function mockNotificationsList() {
  const d = ensureData();
  const notify = d.chats.find(c => c.type === 'notify');
  return { messages: notify ? messagesFor(notify.id) : [] };
}

/**
 * Mock sending to the Notifications chat.
 * @returns {Message}
 */
export function mockNotifySend(_token, content, attachments = undefined) {
  const d = ensureData();
  const notify = d.chats.find(c => c.type === 'notify');
  const chatId = notify ? notify.id : 'notify';
  const msg = {
    id: randid(),
    chat_id: chatId,
    content,
    user_id: currentUser().id,
    author: currentUser(),
    created_at: new Date().toISOString(),
    edited_at: null,
    deleted: false,
    attachments: attachments || [],
    reactions: [],
  };
  d.messages.push(msg);
  return msg;
}

/** @returns {{ ok: boolean }} */
export function mockNotifyMarkRead() {
  return { ok: true };
}

/** @returns {{ ok: boolean }} */
export function mockNotifyDelete(_token, _msgId) {
  return { ok: true };
}

// ── 持久化通知 occurrence mock（【本地改动 2026-08-31】持久化通知机制）──
// 与后端 handlers/notification_occurrences.go 新端点一一对应。mock 数据保存在
// mock 全局 state 的 notificationOccurrences 数组里（见 mockOccurrenceAdd 的
// 写入），列表/未读/已读/删除都在内存里生效，方便前端单测与 mock E2E 断言。

/**
 * 追加一条 occurrence 到 mock 全局 state（供 mock 发送路径调用）。
 * @param {NotificationOccurrence} occ
 */
export function mockOccurrenceAdd(occ) {
  const d = ensureData();
  d.notificationOccurrences = d.notificationOccurrences || [];
  d.notificationOccurrences.unshift(occ);
}

/** @returns {{ occurrences: NotificationOccurrence[] }} */
export function mockOccurrenceList(_token, _before, limit) {
  const d = ensureData();
  const all = d.notificationOccurrences || [];
  return { occurrences: all.slice(0, limit || 50) };
}

/** @returns {{ count: number }} */
export function mockOccurrenceUnreadCount(_token) {
  const d = ensureData();
  return { count: (d.notificationOccurrences || []).filter(o => !o.read).length };
}

/** @returns {{ ok: boolean }} */
export function mockOccurrenceMarkRead(_token, id) {
  const d = ensureData();
  const occ = (d.notificationOccurrences || []).find(o => o.id === id);
  if (occ) occ.read = true;
  return { ok: !!occ };
}

/** @returns {{ ok: boolean }} */
export function mockOccurrenceMarkAllRead(_token) {
  const d = ensureData();
  for (const o of (d.notificationOccurrences || [])) o.read = true;
  return { ok: true };
}

/** @returns {{ ok: boolean }} */
export function mockOccurrenceDelete(_token, id) {
  const d = ensureData();
  d.notificationOccurrences = (d.notificationOccurrences || []).filter(o => o.id !== id);
  return { ok: true };
}

// ── Web Push mock（【本地改动 2026-08-31】Web Push 机制）──
// 与后端 handlers/push.go 三端点一一对应。mock 状态下：
//   - VAPID 公钥固定返回（未配置语义无法在纯前端 mock，固定给一把测试公钥，
//     与后端 503 语义由调用方在真实端区分；mock 里都返回 200 便于单测）；
//   - 订阅写入 pushSubscriptions 数组（endpoint 唯一，重复注册覆盖）。

/**
 * 返回 VAPID 公钥。mock 固定给一把合法的测试公钥（base64url），
 * 前端注册流程用它与真实端一致。
 * @returns {{ vapid_public_key: string }}
 */
export function mockPushGetVAPIDPublicKey(_token) {
  return { vapid_public_key: 'BMockVAPIDPublicKey0000000000000000000000000000000000000000' };
}

/**
 * 保存一条推送订阅（endpoint 唯一→覆盖；返回 created 表示新插入）。
 * @param {string} _token
 * @param {{ endpoint: string, p256dh: string, auth: string }} sub
 * @returns {{ subscribed: boolean, created: boolean }}
 */
export function mockPushSubscribe(_token, sub) {
  const d = ensureData();
  d.pushSubscriptions = d.pushSubscriptions || [];
  const existing = d.pushSubscriptions.find(s => s.endpoint === sub.endpoint);
  if (existing) {
    Object.assign(existing, sub);
    return { subscribed: true, created: false };
  }
  d.pushSubscriptions.push({ id: randid(), user_id: '', created_at: new Date().toISOString(), ...sub });
  return { subscribed: true, created: true };
}

/**
 * 删除一条推送订阅（幂等：不存在也返回成功）。
 * @param {string} _token
 * @param {string} endpoint
 * @returns {{ unsubscribed: boolean }}
 */
export function mockPushUnsubscribe(_token, endpoint) {
  const d = ensureData();
  d.pushSubscriptions = (d.pushSubscriptions || []).filter(s => s.endpoint !== endpoint);
  return { unsubscribed: true };
}

// ──────────────────────────────────────────────────────────────────────
// 【本地改动 2026-08-31】线程 mock。
// 状态：threadFollows  (threadRootID -> Set<userID>)、threadReadState (userID:rootID -> last-seen id)
// ──────────────────────────────────────────────────────────────────────

/**
 * @param {string} threadRootID
 * @returns {Set<string>}
 */
function followSetFor(threadRootID) {
  if (!threadFollows.has(threadRootID)) threadFollows.set(threadRootID, new Set());
  return threadFollows.get(threadRootID);
}

/**
 * 获取线程摘要：root_message + meta。
 * @param {string} _token
 * @param {string} chatId
 * @param {string} threadRootID
 * @returns {{ root_message: Message, thread_root_message_id: string, chat_id: string, reply_count: number, last_reply_at: string, latest_reply_id: string, is_following: boolean, has_unread: boolean }}
 */
export function mockThreadGetSummary(_token, chatId, threadRootID) {
  const d = ensureData();
  const msgs = d.messages.filter(m => m.chat_id === chatId && (m.thread_root_message_id === threadRootID || m.id === threadRootID));
  const root = msgs.find(m => m.id === threadRootID);
  if (!root) return /** @type {any} */ ({ error: 'not_found' });
  const replies = msgs.filter(m => m.id !== threadRootID);
  const latest = replies.length ? replies[replies.length - 1] : null;
  const user = currentUser();
  const isFollowing = followSetFor(threadRootID).has(user.id);
  const lastSeen = threadReadState.get(`${user.id}:${threadRootID}`);
  const hasUnread = lastSeen
    ? latest && latest.id !== root.id && latest.created_at > lastSeen
    : replies.length > 0;
  return {
    root_message: root,
    thread_root_message_id: threadRootID,
    chat_id: chatId,
    reply_count: replies.length,
    last_reply_at: latest?.created_at || '',
    latest_reply_id: latest?.id || '',
    is_following: isFollowing,
    has_unread: hasUnread,
  };
}

/**
 * @param {string} _token
 * @param {string} [before]
 * @param {number} [limit]
 * @returns {{ threads: Array<{ root_message: Message, meta: any }> }}
 */
export function mockThreadsListFollowed(_token, before, limit) {
  const user = currentUser();
  const threads = [];
  for (const [rootID, followers] of threadFollows.entries()) {
    if (!followers.has(user.id)) continue;
    const d = ensureData();
    const msgs = d.messages.filter(m => m.thread_root_message_id === rootID || m.id === rootID);
    const root = msgs.find(m => m.id === rootID);
    if (!root) continue;
    const s = mockThreadGetSummary(_token, root.chat_id, rootID);
    threads.push(s);
  }
  threads.sort((a, b) => (b.last_reply_at || '').localeCompare(a.last_reply_at || ''));
  if (before) {
    const idx = threads.findIndex(t => t.thread_root_message_id === before);
    if (idx > 0) threads.splice(0, idx);
  }
  const limitNum = limit || 50;
  return { threads: threads.slice(0, limitNum) };
}

/**
 * @param {string} _token
 * @param {{ thread_root_message_id: string }} body
 * @returns {{ following: boolean }}
 */
export function mockThreadWatch(_token, body) {
  followSetFor(body.thread_root_message_id).add(currentUser().id);
  return { following: true };
}

/**
 * @param {string} _token
 * @param {{ thread_root_message_id: string }} body
 * @returns {{ following: boolean }}
 */
export function mockThreadUnfollow(_token, body) {
  followSetFor(body.thread_root_message_id).delete(currentUser().id);
  return { following: false };
}

/**
 * @param {string} _token
 * @param {{ thread_root_message_id: string }} body
 * @returns {{ thread_root_message_id: string }}
 */
export function mockThreadMarkRead(_token, body) {
  const d = ensureData();
  const msgs = d.messages.filter(m => m.thread_root_message_id === body.thread_root_message_id || m.id === body.thread_root_message_id);
  const latest = msgs[msgs.length - 1];
  if (latest) threadReadState.set(`${currentUser().id}:${body.thread_root_message_id}`, latest.created_at);
  return { thread_root_message_id: body.thread_root_message_id };
}
