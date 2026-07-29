/**
 * @typedef {import('../schemas').Chat} Chat
 * @typedef {import('../schemas').Message} Message
 * @typedef {import('../schemas').User} User
 * @typedef {import('../schemas').PinnedContent} PinnedContent
 */

/**
 * @typedef {Object} ChatStore
 * @property {Chat[]} chats
 * @property {string|null} activeChatId
 * @property {Message[]} messages
 * @property {Object<string, PinnedContent|null>} pinnedMessage
 * @property {string[]} onlineUserIds
 * @property {'ws'|'sse'|'poll'} mode
 * @property {boolean} wsReady
 * @property {boolean} sseReady
 * @property {(mode: string) => void} setMode
 * @property {(token: string|null) => void} connect
 * @property {(chats: Chat[]) => void} setChats
 * @property {(chat: Partial<Chat>) => void} onChatUpdate
 * @property {(payload: {chat_id?:string, id?:string}) => void} onChatDelete
 * @property {(payload: {chat_id:string}) => void} onChatRemove
 * @property {(msg: Message) => void} onMessageCreate
 * @property {(msg: Partial<Message>) => void} onMessageUpdate
 * @property {(payload: {message_id:string}) => void} onMessageDelete
 * @property {(payload: {message_id:string, emoji:string, user_id:string}, added: boolean) => void} onReaction
 * @property {(token: string) => Promise<void>} loadChats
 * @property {(token: string, chatId: string, before?: string) => Promise<void>} loadMessages
 * @property {(token: string, chatId: string, content: string, attachments?: import('../schemas').Attachment[]) => Promise<void>} sendMessage
 * @property {(chatId: string) => void} sendTyping
 * @property {(chatId: string) => void} subscribe
 * @property {(op: string, payload?: any) => Promise<any>} wsRequest
 * @property {(token: string, chatId: string, content: string) => Promise<void>} setAnnouncement
 * @property {(token: string, chatId: string) => Promise<void>} clearAnnouncement
 * @property {(chatId: string) => Promise<void>} markAnnouncementRead
 * @property {() => void} reset
 */

import { create } from 'zustand';
import { api } from '../api/client';
import { getCoordinator } from '../realtime/coordinator';
import { useAuthStore } from '../store/auth';
import { requestNotifyPermission, sendBrowserNotification } from '../utils/browserNotify';

const coord = getCoordinator();

function getAuth() {
  return useAuthStore.getState();
}

function sortChats(a, b) {
  const pa = !!a.pinned, pb = !!b.pinned;
  if (pa !== pb) return pa ? -1 : 1;
  const da = a.last_message_at || a.created_at;
  const db = b.last_message_at || b.created_at;
  return new Date(db) - new Date(da);
}

async function fetchStream(url, msgId) {
  const token = useAuthStore.getState().accessToken;
  try {
    const res = await fetch(url, { headers: token ? { Authorization: 'Bearer ' + token } : {} });
    if (!res.ok) { console.error('fetchStream: HTTP', res.status); return; }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    let streamDone = false;
    while (!streamDone) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split('\n');
      buf = lines.pop() || '';
      for (const line of lines) {
        const t = line.trim();
        if (!t.startsWith('data: ')) continue;
        const p = t.slice(6);
        if (p === '[DONE]') { streamDone = true; break; }
        try {
          const json = JSON.parse(p);
          if (json.type === 'reasoning' && json.content) {
            useChatStore.setState(s => ({ messages: s.messages.map(m =>
              m.id === msgId ? { ...m, thinking: (m.thinking || '') + json.content } : m
            ) }));
          } else if (json.content) {
            useChatStore.setState(s => ({ messages: s.messages.map(m =>
              m.id === msgId ? { ...m, content: m.content + json.content } : m
            ) }));
          }
        } catch {}
      }
    }
    useChatStore.setState(s => ({ messages: s.messages.map(m =>
      m.id === msgId ? { ...m, streaming: false } : m
    ) }));
  } catch (e) { console.error('fetchStream error:', e); }
}

coord.setHandlers({
  /** @param {{ onlineUserIds?: string[], chats?: import('../schemas').Chat[] }} payload */
  onReady: ({ onlineUserIds, chats }) => {
    set({ onlineUserIds, wsReady: true, sseReady: true });
    get().setChats(chats || []);
  },
  /** @param {string} op @param {any} payload */
  onEvent: (op, payload) => {
    const s = get();
    switch (op) {
      case 'message_create': s.onMessageCreate(payload); break;
      case 'message_update': s.onMessageUpdate(payload); break;
      case 'message_delete': s.onMessageDelete(payload); break;
      case 'reaction_add': s.onReaction(payload, true); break;
      case 'reaction_remove': s.onReaction(payload, false); break;
      case 'chat_create': case 'chat_update': s.onChatUpdate(payload); break;
      case 'chat_delete': s.onChatDelete(payload); break;
      case 'chat_remove': s.onChatRemove(payload); break;
      case 'presence_update': {
        set(s => {
          const ids = new Set(s.onlineUserIds);
          if (payload.status === 'online') ids.add(payload.user_id);
          else ids.delete(payload.user_id);
          return { onlineUserIds: [...ids] };
        });
        break;
      }
      case 'user_update': {
        set(s => ({
          chats: s.chats.map(c => ({
            ...c,
            members: c.members?.map(m => m.id === payload.id ? { ...m, ...payload } : m),
          })),
          userUpdateVer: (s.userUpdateVer || 0) + 1,
        }));
        break;
      }
      case 'poll:chats': s.setChats(payload); break;
      case 'poll:messages': set({ messages: (payload || []).map(m => get()._normalize(m)) }); break;
    }
  },
  onClose: () => {
    set({ wsReady: false, sseReady: false });
  },
  getActiveChatId: () => get().activeChatId,
});

/** @param {Partial<ChatStore>|((s: ChatStore) => Partial<ChatStore>)} fn */
const set = (fn) => useChatStore.setState(fn);
const get = () => useChatStore.getState();

function getAccessToken() {
  try { const a = JSON.parse(localStorage.getItem('auth') || '{}'); return a.accessToken; } catch { return null; }
}

/** @type {import('zustand').StateCreator<ChatStore>} */
export const useChatStore = create((set, get) => ({
  chats: [],
  activeChatId: null,
  messages: [],
  pinnedMessage: {},
  onlineUserIds: [],
  notifyEnabled: {},
  _localStreaming: {}, // msgId → true: Composer 正在本地处理 AI 流, fetchStream 应跳过

  mode: 'ws',
  wsReady: false,
  sseReady: false,

  /** @param {string} mode */
  setMode(mode) {
    coord.disconnect();
    set({ mode, wsReady: false, sseReady: false });
    const token = coord.token;
    if (token) coord.connect(mode, token);
  },

  /** @param {string|null} token */
  connect(token) {
    if (!token) { coord.disconnect(); set({ wsReady: false, sseReady: false }); return; }
    const mode = get().mode;
    coord.connect(mode, token);
  },

  /** @param {import('../schemas').Chat[]} chats */
  setChats(chats) {
    set(s => {
      const existing = new Map(s.chats.map(c => [c.id, c]));
      const merged = (chats || []).map(c => {
      const old = existing.get(c.id);
      if (!old) {
        const lma = c.last_message_at || c.created_at;
        return { ...c, last_message_at: lma };
      }
      const lm = (c.last_message?.content?.trim() ? c.last_message : null) || (old.last_message?.content?.trim() ? old.last_message : null);
      const lma = c.last_message_at || old.last_message_at || c.created_at;
      return { ...c, last_message_at: lma, last_message: lm, unread_count: c.unread_count ?? 0, members: old.members };
    });
      const sorted = merged.sort(sortChats);
      const pinned = {};
      const notify = {};
      for (const c of sorted) {
        if (c.pinned_message?.content) pinned[c.id] = c.pinned_message;
        if (c.notify_enabled !== undefined) notify[c.id] = c.notify_enabled;
      }
      return { chats: sorted, pinnedMessage: { ...s.pinnedMessage, ...pinned }, notifyEnabled: { ...s.notifyEnabled, ...notify } };
    });
  },

  /** @param {Partial<import('../schemas').Chat>} chat */
  onChatUpdate(chat) {
    set(s => {
      const idx = s.chats.findIndex(c => c.id === chat.id);
      let n;
      if (idx >= 0) {
        n = [...s.chats];
        n[idx] = { ...n[idx], ...chat };
      } else {
        n = [chat, ...s.chats];
      }
      n.sort(sortChats);
      const next = chat.pinned_message !== undefined
        ? { ...s.pinnedMessage, [chat.id]: chat.pinned_message || null }
        : s.pinnedMessage;
      return { chats: n, pinnedMessage: next };
    });
  },

  /** @param {{ chat_id?: string, id?: string }} payload */
  onChatDelete(payload) {
    const id = payload.chat_id || payload.id;
    set(s => ({
      chats: s.chats.filter(c => c.id !== id),
      messages: s.activeChatId === id ? [] : s.messages,
      activeChatId: s.activeChatId === id ? null : s.activeChatId,
    }));
  },

  /** @param {{ chat_id: string }} payload */
  onChatRemove(payload) {
    set(s => ({
      chats: s.chats.filter(c => c.id !== payload.chat_id),
      messages: s.activeChatId === payload.chat_id ? [] : s.messages,
      activeChatId: s.activeChatId === payload.chat_id ? null : s.activeChatId,
    }));
  },

  /** @param {string} chatId */
  setNotifyEnabled(chatId, enabled) {
    const token = getAccessToken();
    if (token) {
      api.setNotifyEnabled(token, chatId, enabled).catch(e => console.error('setNotifyEnabled error:', e));
    }
    set(s => {
      const next = { ...s.notifyEnabled, [chatId]: enabled };
      return { notifyEnabled: next };
    });
  },

  /** @param {import('../schemas').Message} msg */
  onMessageCreate(msg) {
    let wasNew = false;
    set(s => {
      const exists = s.messages.find(m => m.id === msg.id);
      wasNew = !exists;
      if (exists) {
        const keepContent = msg.type === 'stream' && exists.streaming;
        return {
          messages: s.activeChatId === msg.chat_id
            ? s.messages.map(m => m.id === msg.id ? { ...m, ...msg, content: keepContent ? m.content : msg.content, streaming: m.streaming || msg.streaming } : m)
            : s.messages,
          chats: s.chats.map(c => c.id === msg.chat_id ? { ...c, last_message: msg, last_message_at: msg.created_at, unread_count: s.activeChatId === msg.chat_id ? 0 : (c.unread_count || 0) + 1 } : c).sort(sortChats),
        };
      }
      const sanitized = msg.type === 'stream' && (msg.stream_url || (typeof msg.content === 'string' && msg.content.startsWith('/api/chats/')))
        ? { ...msg, content: '' }
        : msg;
      const chat = s.chats.find(c => c.id === msg.chat_id);
      if (!chat) return { messages: s.activeChatId === msg.chat_id ? [...s.messages, sanitized] : s.messages };
      return {
        messages: s.activeChatId === msg.chat_id ? [...s.messages, sanitized] : s.messages,
        chats: s.chats.map(c => c.id === msg.chat_id ? { ...c, last_message: msg, last_message_at: msg.created_at, unread_count: s.activeChatId === msg.chat_id ? 0 : (c.unread_count || 0) + 1 } : c).sort(sortChats),
      };
    });
    if (wasNew && msg.type === 'stream') {
      const streamUrl = msg.stream_url || (typeof msg.content === 'string' && msg.content.startsWith('/api/chats/') ? msg.content : null);
      if (streamUrl) {
        const uid = getAuth().user?.id;
        if (msg.user_id !== uid && !get()._localStreaming[msg.id]) {
          fetchStream(streamUrl, msg.id);
        }
      }
    }
    const st = get();
    if (msg.chat_id !== st.activeChatId && msg.type !== 'stream') {
      const uid = getAuth().user?.id;
      const token = getAuth().accessToken;
      const blocked = getAuth().user?.notify_blocked || [];
      if (blocked.includes(msg.user_id)) return;
      const mentioned = uid && msg.content?.includes(`<@${uid}>`);
      const chatName = st.chats.find(c => c.id === msg.chat_id)?.name || 'Chat';
      const authorName = msg.author?.username || 'Someone';
      if (mentioned) {
        requestNotifyPermission().then(() => {
          sendBrowserNotification(`@mentioned in ${chatName}`, msg.content?.replace(/<@[^>]+>/g, '').slice(0, 120) || '', () => {
            const s = get();
            s.setActiveChatId(msg.chat_id);
            s.loadMessages(token, msg.chat_id);
          });
        });
      } else if (st.notifyEnabled?.[msg.chat_id]) {
        requestNotifyPermission().then(() => {
          sendBrowserNotification(authorName + ' — ' + chatName, msg.content?.slice(0, 120) || '(attachment)', () => {
            const s = get();
            s.setActiveChatId(msg.chat_id);
            s.loadMessages(token, msg.chat_id);
          });
        });
      }
    }
  },

  /** @param {Partial<import('../schemas').Message>} msg */
  onMessageUpdate(msg) {
    set(s => ({ messages: s.messages.map(m => m.id === msg.id ? { ...m, ...msg } : m) }));
  },

  /** @param {{ message_id: string }} payload */
  onMessageDelete(payload) {
    set(s => ({ messages: s.messages.map(m => m.id === payload.message_id ? { ...m, deleted: true, content: '' } : m) }));
  },

  /** @param {{ message_id: string, emoji: string, user_id: string }} payload @param {boolean} added */
  onReaction(payload, added) {
    const myId = getAuth().user?.id;
    set(s => ({ messages: s.messages.map(m => {
      if (m.id !== payload.message_id) return m;
      const rxs = m.reactions || [];
      const idx = rxs.findIndex(r => r.emoji === payload.emoji);
      if (added) {
        if (idx >= 0) {
          const existing = rxs[idx];
          if (existing.user_ids?.includes(payload.user_id)) return m;
          return {
            ...m,
            reactions: rxs.map((r, i) => i === idx ? { ...r, count: r.count + 1, user_ids: [...(r.user_ids || []), payload.user_id], me: payload.user_id === myId } : r),
          };
        }
        return { ...m, reactions: [...rxs, { emoji: payload.emoji, count: 1, user_ids: [payload.user_id], me: payload.user_id === myId }] };
      } else {
        return { ...m, reactions: rxs.map(r => r.emoji === payload.emoji ? { ...r, count: r.count - 1, user_ids: (r.user_ids || []).filter(id => id !== payload.user_id), me: false } : r).filter(r => r.count > 0) };
      }
    }) }));
  },

  /** @param {string} token */
  async loadChats(token) {
    try {
      const data = await api.listChats(token);
      get().setChats(data.chats || []);
    } catch (e) { console.error('loadChats error:', e); }
  },

  /** @param {import('../schemas').Message} m */
  _normalize(m) {
    if (m.deleted_at) {
      return { ...m, deleted: true, content: '' };
    }
    if (m.deleted) {
      return { ...m, content: '' };
    }
    return m;
  },

  _msgLoadId: 0,
  /** @param {string} token @param {string} chatId @param {string} [before] */
  async loadMessages(token, chatId, before) {
    const loadId = ++get()._msgLoadId;
    try {
      const data = await api.listMessages(token, chatId, before);
      const norm = (data.messages || []).map(m => get()._normalize(m));
      set(s => {
        if (s._msgLoadId !== loadId) return {};
        return {
          messages: before ? [...norm, ...s.messages] : norm,
        };
      });
    } catch (e) { console.error('loadMessages error:', e); }
  },

  /** @param {string} token @param {string} chatId @param {string} content @param {import('../schemas').Attachment[]} [attachments] */
  async sendMessage(token, chatId, content, attachments) {
    await api.sendMessage(token, chatId, content, attachments);
  },

  /** @param {string} chatId */
  sendTyping(chatId) { coord.sendTyping(chatId); },
  /** @param {string} chatId */
  subscribe(chatId) { coord.subscribe(chatId); },
  /** @param {string} op @param {any} [payload] @returns {Promise<any>} */
  wsRequest(op, payload) { return coord.wsRequest(op, payload); },

  /** @param {string} token @param {string} chatId @param {string} content */
  async setAnnouncement(token, chatId, content) {
    const res = await api.setAnnouncement(token, chatId, content);
    const p = res.pinned_message || { id: '', content, pinned_at: new Date().toISOString() };
    set(s => ({
      pinnedMessage: { ...s.pinnedMessage, [chatId]: p }
    }));
  },

  /** @param {string} token @param {string} chatId */
  async clearAnnouncement(token, chatId) {
    await api.clearAnnouncement(token, chatId);
    set(s => {
      const next = { ...s.pinnedMessage };
      delete next[chatId];
      return { pinnedMessage: next };
    });
  },

  /** @param {string} chatId */
  async markAnnouncementRead(chatId) {
    const { accessToken } = getAuth();
    try { await api.markAnnouncementRead(accessToken, chatId); } catch (e) { console.error('markAnnouncementRead error:', e); }
    set(s => ({
      chats: s.chats.map(c => c.id === chatId ? { ...c, pinned_last_read_at: new Date().toISOString() } : c),
    }));
  },

  /** @param {string|null} id */
  setActiveChatId(id) {
    set(s => {
      if (id && s.activeChatId !== id) {
        return {
          activeChatId: id,
          messages: [],
          chats: s.chats.map(c => c.id === id ? { ...c, unread_count: 0 } : c),
        };
      }
      return { activeChatId: id };
    });
  },

  reset() {
    set({ chats: [], activeChatId: null, messages: [], pinnedMessage: {} });
  },

}));
