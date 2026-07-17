/**
 * @typedef {import('../types').User} User
 * @typedef {import('../types').Chat} Chat
 * @typedef {import('../types').Message} Message
 * @typedef {import('../types').Reaction} Reaction
 * @typedef {import('../types').Attachment} Attachment
 * @typedef {import('../types').StreamSource} StreamSource
 */

/**
 * @typedef {Object} ApiResponse
 * @property {number} [status]
 * @property {string} [error]
 * @property {string} [message]
 */

/**
 * @typedef {ApiResponse & { user: User, access_token: string, expires_in: number }} AuthResponse
 * @typedef {ApiResponse & { chats: Chat[] }} ListChatsResponse
 * @typedef {ApiResponse & { messages: Message[] }} ListMessagesResponse
 * @typedef {ApiResponse & { members: User[] }} ListMembersResponse
 * @typedef {ApiResponse & { reactions: Reaction[] }} ListReactionsResponse
 * @typedef {ApiResponse & { users: User[] }} SearchUsersResponse
 * @typedef {ApiResponse & { url: string, filename: string, mime_type: string, size: number }} UploadResponse
 * @typedef {ApiResponse & { pinned_message?: import('../types').PinnedContent, pinned_updated_at?: string }} AnnouncementResponse
 */

import {
  mockRegister, mockLogin, mockRefresh, mockLogout,
  mockListChats, mockListPublicChats, mockCreateChat, mockGetChat,
  mockDeleteChat, mockJoinChat,
  mockSetAnnouncement, mockClearAnnouncement,
  mockAddMember, mockRemoveMember, mockListMembers, mockSearchUsers, mockUpdateProfile,
  mockListMessages, mockSendMessage, mockEditMessage, mockDeleteMessage,
  mockMarkRead, mockAddReaction, mockRemoveReaction, mockGetReactions,
  mockUpload, mockUploadAvatar,
  mockPinChat, mockUnpinChat, mockMarkAnnouncementRead, mockUpdateChatAvatar, mockUpdateChatBanner, mockUpdateChatBackground,
  resetMockData,
} from './mock';
import { createStreamSource } from '../dev/stream-source';
import { useAuthStore } from '../store/auth';

const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = import.meta.env.VITE_API_BASE || (IS_PAGES ? 'https://chat.moonchan.xyz' : (() => { console.warn('[API] VITE_API_BASE not set — using empty string (same-origin)'); return ''; })());
const UPLOAD_BASE = import.meta.env.VITE_UPLOAD_BASE || (() => { console.warn('[API] VITE_UPLOAD_BASE not set — defaulting to https://upload.moonchan.xyz'); return 'https://upload.moonchan.xyz'; })();
/**
 * @param {import('../types').Attachment} data
 * @param {string} filename
 * @returns {string}
 */
function buildUploadUrl(data, filename) {
  return UPLOAD_BASE + '/api/' + data.id + '/' + encodeURIComponent(filename);
}

let _refreshing = false;

/**
 * @param {string} method
 * @param {string} path
 * @param {string|null} token
 * @param {object} [body]
 * @returns {Promise<any>}
 */
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
  if (res.status === 401 && path !== '/api/auth/refresh' && path !== '/api/auth/logout') {
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
        console.error('[API] refresh failed:', e);
      } finally {
        _refreshing = false;
      }
    }
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
  }
  if (res.status === 429) {
    throw { status: 429, error: 'too_many_requests', message: 'Too many requests, please try again later' };
  }
  if (!res.ok) {
    console.error(`[API Error] ${method} ${path} → ${res.status}`, data.error || '');
    throw { status: res.status, ...data };
  }
  return data;
}

// ── Auth ──
/** @type {{ [key: string]: (...args: any[]) => Promise<any>|any }} */
const _apiMethods = {
  /** @param {string} email @param {string} username @param {string} password @returns {Promise<AuthResponse>} */
  register: (email, username, password) =>
    request('POST', '/api/auth/register', null, { email, username, password }),
  /** @param {string} email @param {string} password @returns {Promise<AuthResponse>} */
  login: (email, password) =>
    request('POST', '/api/auth/login', null, { email, password }),
  /** @returns {Promise<AuthResponse>} */
  refresh: () =>
    fetch(API_BASE + '/api/auth/refresh', { method: 'POST', credentials: 'include' }).then(r => { if (!r.ok) throw r; return r.json(); }),
  /** @param {string} token @returns {Promise<ApiResponse>} */
  logout: (token) =>
    request('POST', '/api/auth/logout', token),
  /** @param {string} token @param {{ username?: string, avatar_color?: string, avatar_url?: string }} data @returns {Promise<{ user: User }>} */
  updateProfile: (token, data) => request('PATCH', '/api/users/me', token, data),
  /** @param {string} token @param {string} q @returns {Promise<SearchUsersResponse>} */
  searchUsers: (token, q) => request('GET', '/api/users?q=' + encodeURIComponent(q), token),

  // ── Chats ──
  /** @param {string} token @returns {Promise<ListChatsResponse>} */
  listChats: (token) => request('GET', '/api/chats/my', token),
  /** @param {string} token @param {number} [page] @param {number} [limit] @returns {Promise<ListChatsResponse>} */
  listPublicChats: (token, page, limit) => {
    let url = '/api/chats/public';
    const params = [];
    if (page) params.push('page=' + page);
    if (limit) params.push('limit=' + limit);
    if (params.length) url += '?' + params.join('&');
    return request('GET', url, token);
  },
  /** @param {string} token @param {string} name @param {string[]} memberIds @param {string} [visibility] @returns {Promise<Chat>} */
  createChat: (token, name, memberIds, visibility) =>
    request('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility: visibility || 'private' }),
  /** @param {string} token @param {string} id @returns {Promise<Chat>} */
  getChat: (token, id) => request('GET', '/api/chats/' + id, token),
  /** @param {string} token @param {string} id @returns {Promise<ApiResponse>} */
  deleteChat: (token, id) => request('DELETE', '/api/chats/' + id, token),
  /** @param {string} token @param {string} chatId @returns {Promise<ApiResponse>} */
  joinChat: (token, chatId) => request('POST', '/api/chats/' + chatId + '/join', token),
  /** @param {string} token @param {string} chatId @param {string} content @returns {Promise<AnnouncementResponse>} */
  setAnnouncement: (token, chatId, content) => request('POST', '/api/chats/' + chatId + '/announcement', token, { content }),
  /** @param {string} token @param {string} chatId @returns {Promise<ApiResponse>} */
  clearAnnouncement: (token, chatId) => request('DELETE', '/api/chats/' + chatId + '/announcement', token),
  /** @param {string} token @param {string} chatId @param {string} avatarUrl @returns {Promise<ApiResponse>} */
  updateChatAvatar: (token, chatId, avatarUrl) =>
    request('PUT', '/api/chats/' + chatId + '/avatar', token, { avatar_url: avatarUrl }),
  updateChatBanner: (token, chatId, bannerUrl, bannerOpacity) =>
    request('PUT', '/api/chats/' + chatId + '/banner', token, { avatar_url: bannerUrl, banner_opacity: bannerOpacity }),
  updateChatBackground: (token, chatId, backgroundUrl) =>
    request('PUT', '/api/chats/' + chatId + '/background', token, { avatar_url: backgroundUrl }),

  // ── Members ──
  /** @param {string} token @param {string} chatId @returns {Promise<ListMembersResponse>} */
  listMembers: (token, chatId) =>
    request('GET', '/api/chats/' + chatId + '/members', token),
  /** @param {string} token @param {string} chatId @param {string} userId @returns {Promise<ApiResponse>} */
  addMember: (token, chatId, userId) =>
    request('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
  /** @param {string} token @param {string} chatId @param {string} userId @returns {Promise<ApiResponse>} */
  removeMember: (token, chatId, userId) =>
    request('DELETE', '/api/chats/' + chatId + '/members/' + userId, token),

  // ── Messages ──
  /** @param {string} token @param {string} chatId @param {string} [before] @param {number} [limit] @returns {Promise<ListMessagesResponse>} */
  listMessages: (token, chatId, before, limit) => {
    let url = '/api/chats/' + chatId + '/messages?limit=' + (limit || 50);
    if (before) url += '&before=' + before;
    return request('GET', url, token);
  },
  /** @param {string} token @param {string} chatId @param {string} content @param {Attachment[]} [attachments] @returns {Promise<Message>} */
  sendMessage: (token, chatId, content, attachments) =>
    request('POST', '/api/chats/' + chatId + '/messages', token, { content, attachments: (attachments || []).map(({ _key, ...a }) => a) }),
  /** @param {string} token @param {string} chatId @param {string} msgId @param {string} content @returns {Promise<ApiResponse>} */
  editMessage: (token, chatId, msgId, content) =>
    request('PATCH', '/api/chats/' + chatId + '/messages/' + msgId, token, { content }),
  /** @param {string} token @param {string} chatId @param {string} msgId @returns {Promise<ApiResponse>} */
  deleteMessage: (token, chatId, msgId) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId, token),
  /** @param {string} token @param {string} chatId @returns {Promise<ApiResponse>} */
  markRead: (token, chatId) =>
    request('POST', '/api/chats/' + chatId + '/read', token, {}),
  // ── Reactions ──
  /** @param {string} token @param {string} chatId @param {string} msgId @param {string} emoji @returns {Promise<ApiResponse>} */
  addReaction: (token, chatId, msgId, emoji) =>
    request('PUT', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  /** @param {string} token @param {string} chatId @param {string} msgId @param {string} emoji @returns {Promise<ApiResponse>} */
  removeReaction: (token, chatId, msgId, emoji) =>
    request('DELETE', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  /** @param {string} token @param {string} chatId @param {string} msgId @returns {Promise<ListReactionsResponse>} */
  getReactions: (token, chatId, msgId) =>
    request('GET', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions', token),

  /** @param {string} _token @param {string} chatId @returns {Promise<ApiResponse & {pinned: boolean}>} */
  pinChat: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/pin', _token),
  /** @param {string} _token @param {string} chatId @returns {Promise<ApiResponse & {pinned: boolean}>} */
  unpinChat: (_token, chatId) => request('POST', '/api/chats/' + chatId + '/unpin', _token),
  /** @param {string} token @param {string} chatId @returns {Promise<ApiResponse>} */
  markAnnouncementRead: (token, chatId) => request('POST', '/api/chats/' + chatId + '/announcement/read', token, {}),

  // ── Uploads (external upload.moonchan.xyz) ──
  /** @param {File} file @returns {Promise<UploadResponse>} */
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
  // uploadAvatar defined after Proxy setup below

  // ── Misc ──
  /** @param {string} token @returns {string} */
  sseUrl: (token) => API_BASE + '/api/events?access_token=' + encodeURIComponent(token),

  /** @param {StreamSource|Function} source @returns {{ onChunk: Function, done: Promise<void> }} */
  startStreaming: (source) => {
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
  },
};

const _mockHandlers = {
  register: mockRegister, login: mockLogin, refresh: mockRefresh, logout: mockLogout,
  updateProfile: mockUpdateProfile, searchUsers: mockSearchUsers,
  listChats: mockListChats, listPublicChats: mockListPublicChats,
  createChat: mockCreateChat, getChat: mockGetChat, deleteChat: mockDeleteChat,
  joinChat: mockJoinChat,
  setAnnouncement: mockSetAnnouncement, clearAnnouncement: mockClearAnnouncement, updateChatAvatar: mockUpdateChatAvatar, updateChatBanner: mockUpdateChatBanner, updateChatBackground: mockUpdateChatBackground,
  addMember: mockAddMember, removeMember: mockRemoveMember,
  listMembers: mockListMembers, listMessages: mockListMessages,
  sendMessage: mockSendMessage, editMessage: mockEditMessage, deleteMessage: mockDeleteMessage,
  markRead: mockMarkRead, addReaction: mockAddReaction, removeReaction: mockRemoveReaction,
  getReactions: mockGetReactions, pinChat: mockPinChat, unpinChat: mockUnpinChat,
  markAnnouncementRead: mockMarkAnnouncementRead,
  upload: mockUpload, uploadAvatar: mockUploadAvatar,
};

let _mockEnabled = false;

/**
 * @param {string} key
 * @param {Function} fn
 * @param {any[]} args
 * @returns {Promise<any>}
 */
function mockCallLog(key, fn, args) {
  console.log(`[Mock API] ${key}(`, ...args, ')');
  const result = fn(...args);
  const p = result && typeof result.then === 'function' ? result : Promise.resolve(result);
  return p.then(v => { console.log(`[Mock API] ${key} =>`, v); return v; });
}

/**
 * Proxy that routes calls to mock handlers when mock mode is enabled.
 * @type {typeof _apiMethods & { enableMock: () => void, disableMock: () => void, isMockEnabled: () => boolean, uploadAvatar: (token: string, file: File) => Promise<{url: string}> }}
 */
export const api = new Proxy(_apiMethods, {
  get(target, prop) {
    if (_mockEnabled && prop in _mockHandlers) {
      return (...args) => mockCallLog(prop, _mockHandlers[prop], args);
    }
    return target[prop];
  },
});

/** @param {string} _token @param {File} file @returns {Promise<{url: string}>} */
api.uploadAvatar = async (_token, file) => {
  const data = await api.upload(file);
  return { url: data.url };
};

api.enableMock = () => {
  if (_mockEnabled) return;
  _mockEnabled = true;
  resetMockData();
};

api.disableMock = () => { _mockEnabled = false; };
api.isMockEnabled = () => _mockEnabled;
