import { create } from 'zustand';
import { api } from '../api/client';
import { getCoordinator } from '../realtime/coordinator';
import { useAuthStore } from './auth';
import { fetchStream } from '../realtime/fetchStream';
import { maybeNotifyMessage } from '../utils/notifyMessage';

const coord = getCoordinator();

function getAuth() {
  return useAuthStore.getState();
}

function sortChats(a, b) {
  const pa = !!a.pinned, pb = !!b.pinned;
  if (pa !== pb) return pa ? -1 : 1;
  const da = a.last_message_at || a.created_at;
  const db = b.last_message_at || b.created_at;
  return +new Date(db) - +new Date(da);
}

coord.setHandlers({
  onReady: ({ onlineUserIds, chats }) => {
    set({ onlineUserIds, wsReady: true, sseReady: true });
    get().setChats(chats || []);
  },
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

const set = (fn) => useChatStore.setState(fn);
const get = () => useChatStore.getState();

function getAccessToken() {
  try { const a = JSON.parse(localStorage.getItem('auth') || '{}'); return a.accessToken; } catch { return null; }
}

function onStreamChunk(msgId, contentAcc, thinkingAcc, done) {
  set(s => ({
    messages: s.messages.map(m =>
      m.id === msgId ? { ...m, content: contentAcc, thinking: thinkingAcc, streaming: !done } : m
    ),
  }));
}

export const useChatStore = create((set, get) => ({
  chats: [],
  activeChatId: null,
  messages: [],
  pinnedMessage: {},
  onlineUserIds: [],
  notifyEnabled: {},
  _localStreaming: {},
  _optimisticIds: new Set(),
  pendingReply: null,

  mode: 'ws',
  wsReady: false,
  sseReady: false,

  setMode(mode) {
    coord.disconnect();
    set({ mode, wsReady: false, sseReady: false });
    const token = coord.token;
    if (token) coord.connect(mode, token);
  },

  connect(token) {
    if (!token) { coord.disconnect(); set({ wsReady: false, sseReady: false }); return; }
    const mode = get().mode;
    coord.connect(mode, token);
  },

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

  onChatDelete(payload) {
    const id = payload.chat_id || payload.id;
    set(s => ({
      chats: s.chats.filter(c => c.id !== id),
      messages: s.activeChatId === id ? [] : s.messages,
      activeChatId: s.activeChatId === id ? null : s.activeChatId,
    }));
  },

  onChatRemove(payload) {
    set(s => ({
      chats: s.chats.filter(c => c.id !== payload.chat_id),
      messages: s.activeChatId === payload.chat_id ? [] : s.messages,
      activeChatId: s.activeChatId === payload.chat_id ? null : s.activeChatId,
    }));
  },

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

  onMessageCreate(msg) {
    if (get()._optimisticIds.has(msg.id)) {
      get()._optimisticIds.delete(msg.id);
      return;
    }
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
        ? { ...msg, content: '', streaming: true }
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
          fetchStream(streamUrl, msg.id, onStreamChunk);
        }
      }
    }
    if (msg.chat_id !== get().activeChatId) {
      maybeNotifyMessage(msg, get().chats);
    }
  },

  onMessageUpdate(msg) {
    set(s => {
      if (s._localStreaming[msg.id]) return {};
      const { type, user_id, author, created_at, stream_url, ...rest } = msg;
      return { messages: s.messages.map(m => m.id === msg.id ? { ...m, ...rest, streaming: m.streaming || msg.streaming } : m) };
    });
  },

  onMessageDelete(payload) {
    set(s => ({ messages: s.messages.map(m => m.id === payload.message_id ? { ...m, deleted: true, content: '' } : m) }));
  },

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

  async loadChats(token) {
    try {
      const data = await api.listChats(token);
      get().setChats(data.chats || []);
    } catch (e) { console.error('loadChats error:', e); }
  },

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

  async sendMessage(token, chatId, content, attachments, replyTo) {
    const optimisticId = crypto.randomUUID();
    const optimisticMsg = {
      id: optimisticId,
      chat_id: chatId,
      content,
      user_id: useAuthStore.getState().user?.id,
      author: useAuthStore.getState().user,
      created_at: new Date().toISOString(),
      attachments: attachments || [],
      reactions: [],
      reply_to: replyTo || '',
      optimistic: true,
    };
    get()._optimisticIds.add(optimisticId);
    set(s => {
      if (s.activeChatId !== chatId) return {};
      return { messages: [...s.messages, optimisticMsg] };
    });
    try {
      const created = await api.sendMessage(token, chatId, content, attachments, replyTo);
      set(s => ({
        messages: s.messages.map(m => m.id === optimisticId ? created : m),
      }));
    } catch (e) {
      set(s => ({
        messages: s.messages.filter(m => m.id !== optimisticId),
      }));
      get()._optimisticIds.delete(optimisticId);
      throw e;
    }
  },

  sendTyping(chatId) { coord.sendTyping(chatId); },
  subscribe(chatId) { coord.subscribe(chatId); },
  wsRequest(op, payload) { return coord.wsRequest(op, payload); },

  async setAnnouncement(token, chatId, content) {
    const res = await api.setAnnouncement(token, chatId, content);
    const p = res.pinned_message || { id: '', content, pinned_at: new Date().toISOString() };
    set(s => ({
      pinnedMessage: { ...s.pinnedMessage, [chatId]: p }
    }));
  },

  async clearAnnouncement(token, chatId) {
    await api.clearAnnouncement(token, chatId);
    set(s => {
      const next = { ...s.pinnedMessage };
      delete next[chatId];
      return { pinnedMessage: next };
    });
  },

  async markAnnouncementRead(chatId) {
    const { accessToken } = getAuth();
    try { await api.markAnnouncementRead(accessToken, chatId); } catch (e) { console.error('markAnnouncementRead error:', e); }
    set(s => ({
      chats: s.chats.map(c => c.id === chatId ? { ...c, pinned_last_read_at: new Date().toISOString() } : c),
    }));
  },

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
