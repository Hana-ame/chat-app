import { mockListChats, mockListMessages, mockSendMessage, resetMockData } from './mock';
import { createStreamSource } from '../dev/stream-source';

const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = IS_PAGES ? 'https://wsl-8080.moonchan.xyz' : '';
const UPLOAD_BASE = 'https://upload.moonchan.xyz';

function buildUploadUrl(data, filename) {
  return UPLOAD_BASE + '/api/' + data.id + '/' + encodeURIComponent(filename);
}

async function request(method, path, token, body) {
  const opts = { method, headers: {} };
  if (token) opts.headers['Authorization'] = 'Bearer ' + token;
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  const data = await res.json().catch(() => ({}));
  if (res.status === 401) {
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
  }
  if (!res.ok) throw { status: res.status, ...data };
  return data;
}

// ── Auth ──
export const api = {
  register: (email, username, password) =>
    request('POST', '/api/auth/register', null, { email, username, password }),
  login: (email, password) =>
    request('POST', '/api/auth/login', null, { email, password }),
  refresh: (refreshToken) =>
    request('POST', '/api/auth/refresh', null, { refresh_token: refreshToken }),
  logout: (token, refreshToken) =>
    request('POST', '/api/auth/logout', token, { refresh_token: refreshToken }),
  me: (token) => request('GET', '/api/users/me', token),
  updateProfile: (token, data) => request('PATCH', '/api/users/me', token, data),
  searchUsers: (token, q) => request('GET', '/api/users?q=' + encodeURIComponent(q), token),

  // ── Chats ──
  listChats: (token) => request('GET', '/api/chats', token),
  listPublicChats: (token) => request('GET', '/api/chats/public', token),
  createChat: (token, name, memberIds, visibility) =>
    request('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility: visibility || 'private' }),
  getChat: (token, id) => request('GET', '/api/chats/' + id, token),
  deleteChat: (token, id) => request('DELETE', '/api/chats/' + id, token),
  renameChat: (token, id, name) =>
    request('PATCH', '/api/chats/' + id, token, { name }),
  createDM: (token, userId) =>
    request('POST', '/api/dms', token, { user_id: userId }),
  joinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/join', token),
  pinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/pin', token),
  unpinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/unpin', token),

  // ── Members ──
  addMember: (token, chatId, userId) =>
    request('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
  removeMember: (token, chatId, userId) =>
    request('DELETE', '/api/chats/' + chatId + '/members/' + userId, token),

  // ── Messages ──
  listMessages: (token, chatId, before, limit) => {
    let url = '/api/chats/' + chatId + '/messages?limit=' + (limit || 50);
    if (before) url += '&before=' + before;
    return request('GET', url, token);
  },
  sendMessage: (token, chatId, content, attachments) =>
    request('POST', '/api/chats/' + chatId + '/messages', token, { content, attachments: attachments || [] }),
  editMessage: (token, chatId, msgId, content) =>
    request('PATCH', '/api/chats/' + chatId + '/messages/' + msgId, token, { content }),
  deleteMessage: (token, chatId, msgId) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId, token),
  markRead: (token, chatId, messageId) =>
    request('POST', '/api/chats/' + chatId + '/read', token, { message_id: messageId }),

  // ── Reactions ──
  addReaction: (token, chatId, msgId, emoji) =>
    request('PUT', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  removeReaction: (token, chatId, msgId, emoji) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),

  // ── Uploads (external upload.moonchan.xyz) ──
  upload: async (file) => {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(UPLOAD_BASE + '/api/upload', {
      method: 'PUT',
      body: form,
    });
    if (!res.ok) throw { status: res.status, message: 'Upload failed' };
    const data = await res.json();
    return {
      filename: file.name,
      mime_type: file.type || 'application/octet-stream',
      size: file.size,
      url: buildUploadUrl(data, file.name),
    };
  },
  uploadAvatar: async (_token, file) => {
    const data = await api.upload(file);
    return { url: data.url };
  },

  // ── Misc ──
  sseUrl: (token) => API_BASE + '/api/events?access_token=' + encodeURIComponent(token),
};

api.startStreaming = (source) => {
  if (typeof source === 'function') return createStreamSource(source);
  if (source.type === 'mock') return createStreamSource(source.fn);
  if (source.type === 'sse') {
    return createStreamSource(async (emit) => {
      const res = await fetch(source.url);
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        for (const line of decoder.decode(value).split('\n')) {
          if (line.startsWith('data: ')) emit(line.slice(6));
        }
      }
    });
  }
  return createStreamSource(source);
};

let _mockEnabled = false;
const _origListChats = api.listChats;
const _origListMessages = api.listMessages;
const _origSendMessage = api.sendMessage;

api.enableMock = () => {
  if (_mockEnabled) return;
  _mockEnabled = true;
  resetMockData();
  api.listChats = mockListChats;
  api.listMessages = mockListMessages;
  api.sendMessage = mockSendMessage;
};

api.disableMock = () => {
  if (!_mockEnabled) return;
  _mockEnabled = false;
  api.listChats = _origListChats;
  api.listMessages = _origListMessages;
  api.sendMessage = _origSendMessage;
};

api.isMockEnabled = () => _mockEnabled;
