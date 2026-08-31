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
	mockGetNotifyChat, mockNotificationsList, mockNotifySend, mockNotifyMarkRead, mockNotifyDelete,
	mockOccurrenceList, mockOccurrenceUnreadCount, mockOccurrenceMarkRead, mockOccurrenceMarkAllRead, mockOccurrenceDelete,
	mockPushGetVAPIDPublicKey, mockPushSubscribe, mockPushUnsubscribe,
	mockThreadGetSummary, mockThreadsListFollowed, mockThreadWatch, mockThreadUnfollow, mockThreadMarkRead,
	mockEmitStreamPlaceholder,
	resetMockData,
} from './mock';
import { useAuthStore } from '../store/auth';
import { API_BASE, UPLOAD_BASE, validateEnv } from '../config';
import { AuthResponseSchema, validate } from '../schemas';
import type { User, Chat, Message, Attachment, NotificationOccurrence, PushSubscription, ThreadMeta, ThreadSummary } from '../schemas';
validateEnv();

// buildUploadUrl 构建上传文件的可访问 URL:优先使用服务端返回的绝对 url;
// 兜底路径从 path 拼接,去掉前导斜杠避免双斜杠(UPLOAD_BASE + '/api/local/'
// 已自带斜杠),path 缺失时不再拼出 "undefined" 字符串。
function buildUploadUrl(data: Record<string, unknown>): string {
  if (data.url) return data.url as string;
  const p = String(data.path || '').replace(/^\/+/, '');
  return UPLOAD_BASE + '/api/local/' + p;
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
    request<User>('PATCH', '/api/users/me', token, data),
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
    request<Chat>('PUT', '/api/chats/' + chatId + '/avatar', token, { avatar_url: avatarUrl }),
  updateChatBanner: (token: string, chatId: string, bannerUrl: string, bannerOpacity?: number) =>
    request<Chat>('PUT', '/api/chats/' + chatId + '/banner', token, { banner_url: bannerUrl, banner_opacity: bannerOpacity }),
  updateChatBackground: (token: string, chatId: string, backgroundUrl: string) =>
    request<Chat>('PUT', '/api/chats/' + chatId + '/background', token, { background_url: backgroundUrl }),

  // ── Members ──
  listMembers: (token: string, chatId: string) =>
    request<{ members: User[] }>('GET', '/api/chats/' + chatId + '/members', token),
  addMember: (token: string, chatId: string, userId: string) =>
    request<Chat>('POST', '/api/chats/' + chatId + '/members', token, { user_id: userId }),
  removeMember: (token: string, chatId: string, userId: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/members/' + userId, token),

  // ── Messages ──
  // 【本地改动 2026-08-31】inThread 参数：过滤返回
  // 属于该线程的消息（含根）。
  listMessages: (token: string, chatId: string, before?: string, limit?: number, inThread?: string) => {
    let url = '/api/chats/' + chatId + '/messages?limit=' + (limit || 50);
    if (before) url += '&before=' + before;
    if (inThread) url += '&in_thread=' + encodeURIComponent(inThread);
    return request<{ messages: Message[] }>('GET', url, token);
  },
  // 【本地改动 2026-08-31】startThread / threadRoot 参数：startThread=true 让
  // 本消息成为线程根（自引用）；显式 threadRoot 让本消息加入既有线程。
  sendMessage: (token: string, chatId: string, content: string, attachments?: Attachment[], replyTo?: string, threadRoot?: string, startThread?: boolean) =>
    request<Message>('POST', '/api/chats/' + chatId + '/messages', token, {
      content,
      attachments: (attachments || []).map(({ ...a }) => a),
      reply_to: replyTo || '',
      thread_root: threadRoot || '',
      start_thread: !!startThread,
    }),
  editMessage: (token: string, chatId: string, msgId: string, content: string) =>
    request<Message>('PATCH', '/api/chats/' + chatId + '/messages/' + msgId, token, { content }),
  deleteMessage: (token: string, chatId: string, msgId: string) =>
    request<ApiError>('DELETE', '/api/chats/' + chatId + '/messages/' + msgId, token),
  markRead: (token: string, chatId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/read', token, {}),

  // ── Reactions ──
  addReaction: (token: string, chatId: string, msgId: string, emoji: string) =>
    request<Message>('PUT', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  removeReaction: (token: string, chatId: string, msgId: string, emoji: string) =>
    request<Message>('DELETE', '/api/chats/' + chatId + '/messages/' + msgId + '/reactions/' + encodeURIComponent(emoji), token),
  pinChat: (_token: string, chatId: string) =>
    request<{ pinned: boolean }>('POST', '/api/chats/' + chatId + '/pin', _token),
  unpinChat: (_token: string, chatId: string) =>
    request<{ pinned: boolean }>('POST', '/api/chats/' + chatId + '/unpin', _token),
  setNotifyEnabled: (_token: string, chatId: string, enabled: boolean) =>
    request<{ enabled: boolean }>('PUT', '/api/chats/' + chatId + '/notify', _token, { enabled }),
  markAnnouncementRead: (token: string, chatId: string) =>
    request<ApiError>('POST', '/api/chats/' + chatId + '/announcement/read', token, {}),

  // ── Notifications ──
  getNotificationsChat: (token: string) =>
    request<Chat>('GET', '/api/chats/notify', token),
  notifications: {
    listMessages: (token: string, before?: string, limit?: number) => {
      let url = '/api/notifications/messages?limit=' + (limit || 50);
      if (before) url += '&before=' + before;
      return request<{ messages: Message[] }>('GET', url, token);
    },
    sendMessage: (token: string, content: string, attachments?: Attachment[]) =>
      request<Message>('POST', '/api/notifications/messages', token, { content, attachments: (attachments || []).map(({ ...a }) => a) }),
    deleteMessage: (token: string, msgId: string) =>
      request<ApiError>('DELETE', '/api/notifications/messages/' + msgId, token),
    markRead: (token: string) =>
      request<ApiError>('POST', '/api/notifications/read', token, {}),
    // 【本地改动 2026-08-31】持久化通知 occurrence 端点
    // 后端 handlers/notification_occurrences.go）。
    listOccurrences: (token: string, before?: string, limit?: number) => {
      let url = '/api/notifications?limit=' + (limit || 50);
      if (before) url += '&before=' + before;
      return request<{ occurrences: NotificationOccurrence[] }>('GET', url, token);
    },
    unreadCount: (token: string) =>
      request<{ count: number }>('GET', '/api/notifications/unread-count', token),
    markReadOccurrence: (token: string, id: string) =>
      request<ApiError>('POST', '/api/notifications/' + id + '/read', token, {}),
    markAllReadOccurrences: (token: string) =>
      request<ApiError>('POST', '/api/notifications/read-all', token, {}),
    deleteOccurrence: (token: string, id: string) =>
      request<ApiError>('DELETE', '/api/notifications/' + id, token),
  },

  // ── Threads ──
  // 【本地改动 2026-08-31】线程 API（关注列表 / 关注 / 取关 / 已读）：
  // 关注列表、关注/取关、单线程详情、标记已读。
  threads: {
    listFollowed: (token: string, before?: string, limit?: number) => {
      let url = '/api/threads?limit=' + (limit || 50);
      if (before) url += '&before=' + before;
      return request<{ threads: ThreadSummary[] }>('GET', url, token);
    },
    follow: (token: string, threadRootMessageId: string) =>
      request<{ following: boolean }>('POST', '/api/threads/follow', token, { thread_root_message_id: threadRootMessageId }),
    unfollow: (token: string, threadRootMessageId: string) =>
      request<{ following: boolean }>('DELETE', '/api/threads/follow', token, { thread_root_message_id: threadRootMessageId }),
    getSummary: (token: string, chatId: string, threadRootMessageId: string) =>
      request<ThreadSummary>('GET', '/api/chats/' + chatId + '/threads/' + threadRootMessageId, token),
    markRead: (token: string, threadRootMessageId: string) =>
      request<{ thread_root_message_id: string }>('POST', '/api/threads/read', token, { thread_root_message_id: threadRootMessageId }),
  },

  // ── Web Push ──
  push: {
    // 【本地改动 2026-08-31】Web Push VAPID
    // handlers/push.go）。getVAPIDPublicKey 在未配置时返回 503 → 调用方
    // （SW 注册流程）捕获后静默跳过推送注册。
    getVAPIDPublicKey: (token: string) =>
      request<{ vapid_public_key: string }>('GET', '/api/push/vapid-public-key', token),
    subscribe: (token: string, sub: { endpoint: string; p256dh: string; auth: string }) =>
      request<{ subscribed: boolean; created: boolean }>('POST', '/api/push/subscribe', token, sub),
    unsubscribe: (token: string, endpoint: string) =>
      request<{ unsubscribed: boolean }>('DELETE', '/api/push/subscribe', token, { endpoint }),
  },

  // ── Uploads ──
  upload: async (file: File, token?: string) => {
    const headers: Record<string, string> = {};
    if (token) headers.Authorization = 'Bearer ' + token;
    const res = await fetch(UPLOAD_BASE + '/api/upload', {
      method: 'PUT',
      headers,
      credentials: 'include',
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

  // ── Stream (AI) Messages ──
  sendStreamMessage: (token: string, chatId: string, content: string, source: Record<string, unknown>, msgId: string) =>
    fetch(API_BASE + '/api/chats/' + chatId + '/messages', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
      body: JSON.stringify({ content, type: 'stream', source, msg_id: msgId }),
    }).then(r => { if (!r.ok) throw r; return r; }),

  // ── Misc ──
  sseUrl: () => API_BASE + '/api/events',
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
    getNotificationsChat: mockGetNotifyChat,
    upload: mockUpload, uploadAvatar: mockUploadAvatar,
  };
  let mockEnabled = false;

  const p = new Proxy(target, {
    get(t, prop: string) {
      if (mockEnabled && prop === 'notifications') {
        return {
          listMessages: () => Promise.resolve(mockNotificationsList()),
          sendMessage: (token: string, content: string, attachments?: Attachment[]) => Promise.resolve(mockNotifySend(token, content, attachments)),
          deleteMessage: (token: string, msgId: string) => Promise.resolve(mockNotifyDelete(token, msgId)),
          markRead: () => Promise.resolve(mockNotifyMarkRead()),
          // 【本地改动 2026-08-31】持久化通知 mock（与后端新端点对应）。
          listOccurrences: (token: string, before?: string, limit?: number) => Promise.resolve(mockOccurrenceList(token, before, limit)),
          unreadCount: (token: string) => Promise.resolve(mockOccurrenceUnreadCount(token)),
          markReadOccurrence: (token: string, id: string) => Promise.resolve(mockOccurrenceMarkRead(token, id)),
          markAllReadOccurrences: (token: string) => Promise.resolve(mockOccurrenceMarkAllRead(token)),
          deleteOccurrence: (token: string, id: string) => Promise.resolve(mockOccurrenceDelete(token, id)),
        };
      }
      if (mockEnabled && prop === 'push') {
        // 【本地改动 2026-08-31】Web Push mock（与后端 handlers/push.go 对应）。
        return {
          getVAPIDPublicKey: (token: string) => Promise.resolve(mockPushGetVAPIDPublicKey(token)),
          subscribe: (token: string, sub: { endpoint: string; p256dh: string; auth: string }) =>
            Promise.resolve(mockPushSubscribe(token, sub)),
          unsubscribe: (token: string, endpoint: string) => Promise.resolve(mockPushUnsubscribe(token, endpoint)),
        };
      }
      if (mockEnabled && prop === 'threads') {
        // 【本地改动 2026-08-31】线程 mock。
        return {
          listFollowed: (token: string, before?: string, limit?: number) =>
            Promise.resolve(mockThreadsListFollowed(token, before, limit)),
          follow: (token: string, threadRootMessageId: string) =>
            Promise.resolve(mockThreadWatch(token, { thread_root_message_id: threadRootMessageId })),
          unfollow: (token: string, threadRootMessageId: string) =>
            Promise.resolve(mockThreadUnfollow(token, { thread_root_message_id: threadRootMessageId })),
          getSummary: (token: string, chatId: string, threadRootMessageId: string) =>
            Promise.resolve(mockThreadGetSummary(token, chatId, threadRootMessageId)),
          markRead: (token: string, threadRootMessageId: string) =>
            Promise.resolve(mockThreadMarkRead(token, { thread_root_message_id: threadRootMessageId })),
        };
      }
      if (mockEnabled && prop === 'sendStreamMessage') {
        // AI 流式发送:mock 模式补发占位消息事件(等价后端 WS message_create),
        // 但 SSE 请求本身仍走真实 fetch,测试用 page.route 拦截断言请求体。
        return (token: string, chatId: string, content: string, source: Record<string, unknown>, msgId: string) => {
          mockEmitStreamPlaceholder(chatId, msgId);
          return _apiMethods.sendStreamMessage(token, chatId, content, source, msgId);
        };
      }
      if (mockEnabled && prop in mockHandlers) {
        return (...args: unknown[]) => {
          const fn = mockHandlers[prop];
          console.debug(`[Mock API] ${prop}(`, ...args, ')');
          const result = fn(...args);
          const promise = result && typeof (result as Promise<unknown>).then === 'function'
            ? (result as Promise<unknown>) : Promise.resolve(result);
          return promise.then(v => { console.debug(`[Mock API] ${prop} =>`, v); return v; });
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
