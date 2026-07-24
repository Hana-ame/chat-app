import {
  mockRegister, mockLogin, mockRefresh, mockLogout,
  mockListChats, mockListPublicChats, mockCreateChat, mockGetChat,
  mockDeleteChat, mockJoinChat,
  mockSetAnnouncement, mockClearAnnouncement,
  mockAddMember, mockRemoveMember, mockListMembers, mockSearchUsers, mockUpdateProfile,
  mockListMessages, mockSendMessage, mockEditMessage, mockDeleteMessage,
  mockMarkRead, mockAddReaction, mockRemoveReaction,
  mockUpload, mockUploadAvatar,
  mockPinChat, mockUnpinChat, mockMarkAnnouncementRead, mockUpdateChatAvatar, mockUpdateChatBanner, mockUpdateChatBackground,
  resetMockData,
} from './mock';
import { createStreamSource } from '../dev/stream-source';
import { useAuthStore } from '../store/auth';
import { API_BASE, UPLOAD_BASE, validateEnv } from '../config';
import { AuthResponseSchema, validate } from '../schemas';
import type { User, Chat, Message, Attachment, StreamSource } from '../schemas';
validateEnv();

function buildUploadUrl(data: Record<string, unknown>): string {
  return (data.url as string) || (UPLOAD_BASE + '/api/local/' + (data.path as string));
}

interface ApiError {
  status: number;
  error?: string;
  message?: string;
  [key: string]: unknown;
}

let refreshPromise: Promise<boolean> | null = null;

async function refreshToken(): Promise<boolean> {
  try {
    const rr = await fetch(API_BASE + '/api/auth/refresh', {
      method: 'POST', credentials: 'include',
    });
    const rd = await rr.json().catch(() => ({})) as Record<string, unknown>;
    if (rr.ok) {
      const saved = JSON.parse(localStorage.getItem('auth') || '{}');
      saved.accessToken = rd.access_token;
      if (rd.user) saved.user = rd.user;
      localStorage.setItem('auth', JSON.stringify(saved));
      useAuthStore.setState({ accessToken: rd.access_token, user: rd.user || saved.user });
      return true;
    }
  } catch (e) {
    console.error('[API] refresh failed:', e);
  }
  return false;
}

async function request<T = unknown>(method: string, path: string, token: string | null, body?: unknown): Promise<T> {
  const opts: RequestInit & { headers: Record<string, string> } = {
    method,
    headers: {},
    credentials: 'include',
  };
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(API_BASE + path, opts);
  const data: Record<string, unknown> = await res.json().catch(() => ({}));
  if (res.status === 401 && path !== '/api/auth/refresh' && path !== '/api/auth/logout') {
    if (!refreshPromise) {
      refreshPromise = refreshToken();
    }
    const ok = await refreshPromise;
    refreshPromise = null;
    if (ok) {
      const retryRes = await fetch(API_BASE + path, opts);
      const retryData = await retryRes.json().catch(() => ({}));
      if (!retryRes.ok) throw { status: retryRes.status, ...retryData } as ApiError;
      return retryData as T;
    }
    window.dispatchEvent(new CustomEvent('auth:unauthorized'));
  }
  if (res.status === 429) {
    throw { status: 429, error: 'too_many_requests', message: 'Too many requests, please try again later' } as ApiError;
  }
  if (!res.ok) {
    console.error(`[API Error] ${method} ${path} → ${res.status}`, data.error || '');
    throw { status: res.status, ...data } as ApiError;
  }
  return data as T;
}

// ── Auth ──
const _apiMethods = {
  register: (email: string, username: string, password: string) =>
    request<{ user: User; access_token: string; expires_in: number }>('POST', '/api/auth/register', null, { email, username, password })
      .then(d => validate(AuthResponseSchema, d, 'register')),
  login: (email: string, password: string) =>
    request<{ user: User; access_token: string; expires_in: number }>('POST', '/api/auth/login', null, { email, password })
      .then(d => validate(AuthResponseSchema, d, 'login')),
  refresh: (): Promise<{ user: User; access_token: string; expires_in: number }> =>
    fetch(API_BASE + '/api/auth/refresh', { method: 'POST', credentials: 'include' })
      .then(r => { if (!r.ok) throw r; return r.json() as Promise<{ user: User; access_token: string; expires_in: number }>; })
      .then(d => validate(AuthResponseSchema, d, 'refresh')),
  logout: (token: string) =>
    request<ApiError>('POST', '/api/auth/logout', token),
  updateProfile: (token: string, data: { username?: string; avatar_color?: string; avatar_url?: string }) =>
    request<{ user: User }>('PATCH', '/api/users/me', token, data),
  searchUsers: (token: string, q: string) =>
    request<{ users: User[] }>('GET', '/api/users?q=' + encodeURIComponent(q), token),

  // ── Chats ──
  listChats: (token: string) =>
    request<{ chats: Chat[] }>('GET', '/api/chats/my', token),
  listPublicChats: (token: string, page?: number, limit?: number) => {
    let url = '/api/chats/public';
    const params: string[] = [];
    if (page) params.push('page=' + page);
    if (limit) params.push('limit=' + limit);
    if (params.length) url += '?' + params.join('&');
    return request<{ chats: Chat[] }>('GET', url, token);
  },
  createChat: (token: string, name: string, memberIds: string[], visibility?: string) =>
    request<Chat>('POST', '/api/chats', token, { type: 'group', name, member_ids: memberIds, visibility: visibility || 'private' }),
  getChat: (token: string, id: string) =>
    request<Chat>('GET', '/api/chats/' + id, token),
  deleteChat: (token: string, id: string) =>
    request<ApiError>('DELETE', '/api/chats/' + id, token),
  joinChat: (token: string, chatId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/join', token),
  setAnnouncement: (token: string, chatId: string, content: string) =>
    request<{ pinned_message?: import('../schemas').PinnedContent; pinned_updated_at?: string }>('POST', '/api/chats/' + chatId + '/announcement', token, { content }),
  clearAnnouncement: (token: string, chatId: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/announcement', token),
  updateChatAvatar: (token: string, chatId: string, avatarUrl: string) =>
    request<ApiError>('PUT', '/api/chats/' + chatId + '/avatar', token, { avatar_url: avatarUrl }),
  updateChatBanner: (token: string, chatId: string, bannerUrl: string, bannerOpacity?: number) =>
    request<ApiError>('PUT', '/api/chats/' + chatId + '/banner', token, { banner_url: bannerUrl, banner_opacity: bannerOpacity }),
  updateChatBackground: (token: string, chatId: string, backgroundUrl: string) =>
    request<ApiError>('PUT', '/api/chats/' + chatId + '/background', token, { background_url: backgroundUrl }),

  // ── Members ──
  listMembers: (token: string, chatId: string) =>
    request<{ members: User[] }>('GET', '/api/chats/' + chatId + '/members', token),
  addMember: (token: string, chatId: string, userId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
  removeMember: (token: string, chatId: string, userId: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/members/' + userId, token),

  // ── Messages ──
  listMessages: (token: string, chatId: string, before?: string, limit?: number) => {
    let url = '/api/chats/' + chatId + '/messages?limit=' + (limit || 50);
    if (before) url += '&before=' + before;
    return request<{ messages: Message[] }>('GET', url, token);
  },
  sendMessage: (token: string, chatId: string, content: string, attachments?: Attachment[]) =>
    request<Message>('POST', '/api/chats/' + chatId + '/messages', token, { content, attachments: (attachments || []).map(({ ...a }) => a) }),
  editMessage: (token: string, chatId: string, msgId: string, content: string) =>
    request<ApiError>('PATCH', '/api/chats/' + chatId + '/messages/' + msgId, token, { content }),
  deleteMessage: (token: string, chatId: string, msgId: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/messages/' + msgId, token),
  markRead: (token: string, chatId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/read', token, {}),

  // ── Reactions ──
  addReaction: (token: string, chatId: string, msgId: string, emoji: string) =>
    request<ApiError>('PUT', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  removeReaction: (token: string, chatId: string, msgId: string, emoji: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  pinChat: (_token: string, chatId: string) =>
    request<{ pinned: boolean }>('POST', '/api/chats/' + chatId + '/pin', _token),
  unpinChat: (_token: string, chatId: string) =>
    request<{ pinned: boolean }>('POST', '/api/chats/' + chatId + '/unpin', _token),
  setNotifyEnabled: (_token: string, chatId: string, enabled: boolean) =>
    request<{ enabled: boolean }>('PUT', '/api/chats/' + chatId + '/notify', _token, { enabled }),
  markAnnouncementRead: (token: string, chatId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/announcement/read', token, {}),

  // ── Uploads ──
  upload: async (file: File) => {
    const res = await fetch(UPLOAD_BASE + '/api/upload', {
      method: 'PUT',
      body: file,
    });
    if (!res.ok) throw { status: res.status, message: 'Upload failed' } as ApiError;
    const data = await res.json() as Record<string, unknown>;
    return {
      filename: file.name,
      mime_type: file.type || 'application/octet-stream',
      size: file.size,
      url: buildUploadUrl(data),
    };
  },

  // ── AI Chat ──
  aiChat: (token: string, body: { source?: string; messages: { role: string; content: string }[]; stream?: boolean; [key: string]: unknown }) =>
    fetch(API_BASE + '/api/ai/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify(body),
    }).then(r => { if (!r.ok) throw r; return r; }),

  // ── Misc ──
  sseUrl: (token: string) => API_BASE + '/api/events?access_token=' + encodeURIComponent(token),

  startStreaming: (source: StreamSource | (() => void)) => {
    if (typeof source === 'function') return createStreamSource(source);
    if (source.type === 'mock') return createStreamSource(source.fn!);
    if (source.type === 'sse') {
      return createStreamSource(async (emit: (data: string) => void) => {
        const res = await fetch(source.url!);
        const reader = res.body!.getReader();
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

export type ApiType = typeof _apiMethods & {
  enableMock: () => void;
  disableMock: () => void;
  isMockEnabled: () => boolean;
  uploadAvatar: (token: string, file: File) => Promise<{ url: string }>;
};

function buildMockProxy(target: typeof _apiMethods): ApiType {
  const mockHandlers: Record<string, (...args: unknown[]) => unknown> = {
    register: mockRegister, login: mockLogin, refresh: mockRefresh, logout: mockLogout,
    updateProfile: mockUpdateProfile, searchUsers: mockSearchUsers,
    listChats: mockListChats, listPublicChats: mockListPublicChats,
    createChat: mockCreateChat, getChat: mockGetChat, deleteChat: mockDeleteChat,
    joinChat: mockJoinChat,
    setAnnouncement: mockSetAnnouncement, clearAnnouncement: mockClearAnnouncement,
    updateChatAvatar: mockUpdateChatAvatar, updateChatBanner: mockUpdateChatBanner, updateChatBackground: mockUpdateChatBackground,
    addMember: mockAddMember, removeMember: mockRemoveMember,
    listMembers: mockListMembers, listMessages: mockListMessages,
    sendMessage: mockSendMessage, editMessage: mockEditMessage, deleteMessage: mockDeleteMessage,
    markRead: mockMarkRead, addReaction: mockAddReaction, removeReaction: mockRemoveReaction,
    pinChat: mockPinChat, unpinChat: mockUnpinChat,
    markAnnouncementRead: mockMarkAnnouncementRead,
    upload: mockUpload, uploadAvatar: mockUploadAvatar,
  };
  let mockEnabled = false;

  const p = new Proxy(target, {
    get(t, prop: string) {
      if (mockEnabled && prop in mockHandlers) {
        return (...args: unknown[]) => {
          const fn = mockHandlers[prop];
          console.log(`[Mock API] ${prop}(`, ...args, ')');
          const result = fn(...args);
          const promise = result && typeof (result as Promise<unknown>).then === 'function'
            ? (result as Promise<unknown>) : Promise.resolve(result);
          return promise.then(v => { console.log(`[Mock API] ${prop} =>`, v); return v; });
        };
      }
      return (t as Record<string, unknown>)[prop];
    },
  }) as ApiType;

  p.uploadAvatar = async (_token: string, file: File) => {
    const data = await p.upload(file);
    return { url: data.url };
  };
  p.enableMock = () => {
    if (mockEnabled) return;
    mockEnabled = true;
    resetMockData();
  };
  p.disableMock = () => { mockEnabled = false; };
  p.isMockEnabled = () => mockEnabled;
  return p;
}

export const api = buildMockProxy(_apiMethods);
