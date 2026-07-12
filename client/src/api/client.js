import {
  mockRegister, mockLogin, mockRefresh, mockLogout, mockMe,
  mockListChats, mockListPublicChats, mockCreateChat, mockGetChat,
  mockDeleteChat, mockRenameChat, mockCreateDM, mockJoinChat,
  mockSetPinnedMessage, mockClearPinnedMessage,
  mockAddMember, mockRemoveMember, mockSearchUsers, mockUpdateProfile,
  mockListMessages, mockSendMessage, mockEditMessage, mockDeleteMessage,
  mockMarkRead, mockAddReaction, mockRemoveReaction, mockGetReactions,
  mockUpload, mockUploadAvatar,
  mockTogglePin, mockMarkPinnedRead,
  resetMockData,
} from './mock';
import { createStreamSource } from '../dev/stream-source';
import { useAuthStore } from '../store/auth';

const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = IS_PAGES ? 'https://wsl-8080.moonchan.xyz' : '';
const UPLOAD_BASE = 'https://upload.moonchan.xyz';

function buildUploadUrl(data, filename) {
  return UPLOAD_BASE + '/api/' + data.id + '/' + encodeURIComponent(filename);
}

let _refreshing = false;

async function request(method, path, token, body) {
  const opts = { 
    method, 
    headers: {}, 
    credentials: 'include' 
  };
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  const data = await res.json().catch(() => ({}));
  if (res.status === 401 && path !== '/api/auth/refresh') {
    if (!_refreshing) {
      _refreshing = true;
      try {
        const rr = await fetch(API_BASE + '/api/auth/refresh', {
          method: 'POST', credentials: 'include',
        });
        const rd = await rr.json().catch(() => ({}));
        if (rr.ok) {
          const saved = JSON.parse(localStorage.getItem('auth') || '{}');
          saved.accessToken = rd.access_token;
          if (rd.user) saved.user = rd.user;
          localStorage.setItem('auth', JSON.stringify(saved));
          useAuthStore.setState({ accessToken: rd.access_token, user: rd.user || saved.user });
          const retryRes = await fetch(API_BASE + path, opts);
          const retryData = await retryRes.json().catch(() => ({}));
          if (!retryRes.ok) throw { status: retryRes.status, ...retryData };
          return retryData;
        }
      } catch (e) {
        // refresh failed
      } finally {
        _refreshing = false;
      }
    }
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
  }
  if (res.status === 429) {
    throw { status: 429, error: 'too_many_requests', message: 'Too many requests, please try again later' };
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
  refresh: () =>
    fetch(API_BASE + '/api/auth/refresh', { method: 'POST', credentials: 'include' }).then(r => { if (!r.ok) throw r; return r.json(); }),
  logout: (token) =>
    request('POST', '/api/auth/logout', token),
  me: (token) => request('GET', '/api/users/me', token),
  updateProfile: (token, data) => request('PATCH', '/api/users/me', token, data),
  searchUsers: (token, q) => request('GET', '/api/users?q=' + encodeURIComponent(q), token),

  // ── Chats ──
  listChats: (token) => request('GET', '/api/chats/my', token),
  listPublicChats: (token, page, limit) => {
    let url = '/api/chats/public';
    const params = [];
    if (page) params.push('page=' + page);
    if (limit) params.push('limit=' + limit);
    if (params.length) url += '?' + params.join('&');
    return request('GET', url, token);
  },
  createChat: (token, name, memberIds, visibility) =>
    request('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility: visibility || 'private' }),
  getChat: (token, id) => request('GET', '/api/chats/' + id, token),
  deleteChat: (token, id) => request('DELETE', '/api/chats/' + id, token),
  renameChat: (token, id, name) =>
    request('PATCH', '/api/chats/' + id, token, { name }),
  createDM: (token, userId) =>
    request('POST', '/api/dms', token, { user_id: userId }), // @deprecated use createChat with type='dm'
  joinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/join', token),
  setPinnedMessage: (token, chatId, content) => request('POST', '/api/chats/' + chatId + '/pin', token, { content }),
  clearPinnedMessage: (token, chatId) => request('DELETE', '/api/chats/' + chatId + '/pin', token),

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
  getReactions: (token, chatId, msgId) =>
    request('GET', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions', token),

  togglePin: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/pin-toggle', _token),
  markPinnedRead: (token, chatId) => request('POST', '/api/chats/' + chatId + '/pin-read', token, {}),

  // ── Uploads (external upload.moonchan.xyz) ──
  upload: async (file) => {
    const res = await fetch(UPLOAD_BASE + '/api/upload', {
      method: 'PUT',
      body: file,
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
const _originals = {};

function save(key, fn) { _originals[key] = fn; }
function swap(key, mock) {
  api[key] = (...args) => {
    console.log(`[Mock API] ${key}(`, ...args, ')');
    const result = mock(...args);
    if (result && typeof result.then === 'function') {
      return result.then(v => { console.log(`[Mock API] ${key} =>`, v); return v; });
    }
    console.log(`[Mock API] ${key} =>`, result);
    return Promise.resolve(result);
  };
}

const MOCKABLE = [
  ['register', mockRegister],
  ['login', mockLogin],
  ['refresh', mockRefresh],
  ['logout', mockLogout],
  ['me', mockMe],
  ['updateProfile', mockUpdateProfile],
  ['searchUsers', mockSearchUsers],
  ['listChats', mockListChats],
  ['listPublicChats', mockListPublicChats],
  ['createChat', mockCreateChat],
  ['getChat', mockGetChat],
  ['deleteChat', mockDeleteChat],
  ['renameChat', mockRenameChat],
  ['createDM', mockCreateDM], // @deprecated
  ['joinChat', mockJoinChat],
  ['setPinnedMessage', mockSetPinnedMessage],
  ['clearPinnedMessage', mockClearPinnedMessage],
  ['addMember', mockAddMember],
  ['removeMember', mockRemoveMember],
  ['listMessages', mockListMessages],
  ['sendMessage', mockSendMessage],
  ['editMessage', mockEditMessage],
  ['deleteMessage', mockDeleteMessage],
  ['markRead', mockMarkRead],
  ['addReaction', mockAddReaction],
  ['removeReaction', mockRemoveReaction],
  ['getReactions', mockGetReactions],
  ['upload', mockUpload],
  ['uploadAvatar', mockUploadAvatar],
  ['togglePin', mockTogglePin],
  ['markPinnedRead', mockMarkPinnedRead],
];

api.enableMock = () => {
  if (_mockEnabled) return;
  _mockEnabled = true;
  resetMockData();
  for (const [key, mock] of MOCKABLE) {
    save(key, api[key]);
    swap(key, mock);
  }
};

api.disableMock = () => {
  if (!_mockEnabled) return;
  _mockEnabled = false;
  for (const [key] of MOCKABLE) {
    api[key] = _originals[key];
  }
};

api.isMockEnabled = () => _mockEnabled;
