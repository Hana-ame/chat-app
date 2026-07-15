import { create } from 'zustand';
import { api } from '../api/client';
import { useAuthStore } from './auth';
import { getCoordinator } from '../realtime/coordinator';

const coord = getCoordinator();

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
      case 'poll:chats': s.setChats(payload); break;
      case 'poll:messages': set({ messages: payload }); break;
    }
  },
  onClose: () => {
    set({ wsReady: false, sseReady: false });
  },
  getActiveChatId: () => get().activeChatId,
});

const set = (fn) => useChatStore.setState(fn);
const get = () => useChatStore.getState();

export const useChatStore = create((set, get) => ({
  chats: [],
  activeChatId: null,
  messages: [],
  pinnedMessage: {},
  onlineUserIds: [],

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
      return { ...c, last_message_at: lma, last_message: lm, unread_count: c.unread_count ?? 0 };
    });
      const sorted = merged.sort((a, b) => {
        const pa = !!a.pinned, pb = !!b.pinned;
        if (pa !== pb) return pa ? -1 : 1;
        const da = a.last_message_at || a.created_at;
        const db = b.last_message_at || b.created_at;
        return new Date(db) - new Date(da);
      });
      const pinned = {};
      for (const c of sorted) {
        if (c.pinned_message?.content) pinned[c.id] = c.pinned_message;
      }
      return { chats: sorted, pinnedMessage: { ...s.pinnedMessage, ...pinned } };
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
      n.sort((a, b) => {
        const pa = !!a.pinned, pb = !!b.pinned;
        if (pa !== pb) return pa ? -1 : 1;
        const da = a.last_message_at || a.created_at;
        const db = b.last_message_at || b.created_at;
        return new Date(db) - new Date(da);
      });
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

  onMessageCreate(msg) {
    set(s => {
      const chat = s.chats.find(c => c.id === msg.chat_id);
      if (!chat) return { messages: s.activeChatId === msg.chat_id ? [...s.messages, msg] : s.messages };
      return {
        messages: s.activeChatId === msg.chat_id ? [...s.messages, msg] : s.messages,
        chats: s.chats.map(c => c.id === msg.chat_id ? { ...c, last_message: msg, last_message_at: msg.created_at, unread_count: s.activeChatId === msg.chat_id ? 0 : (c.unread_count || 0) + 1 } : c).sort((a,b) => {
          const pa = !!a.pinned, pb = !!b.pinned;
          if (pa !== pb) return pa ? -1 : 1;
          return new Date(b.last_message_at || b.created_at) - new Date(a.last_message_at || a.created_at);
        }),
      };
    });
    if (msg.streaming && msg.source) {
      get().startConsumingStream(msg);
    }
  },

  startConsumingStream(msg) {
    api.startStreaming(msg.source)
      .onChunk(chunk => {
        set(s => ({
          messages: s.messages.map(m =>
            m.id === msg.id ? { ...m, content: m.content + chunk } : m
          ),
        }));
      })
      .done.then(() => {
        get().finishStreaming(msg.id);
      });
  },

  onMessageUpdate(msg) {
    set(s => ({ messages: s.messages.map(m => m.id === msg.id ? { ...m, ...msg } : m) }));
  },

  onMessageDelete(payload) {
    set(s => ({ messages: s.messages.map(m => m.id === payload.message_id ? { ...m, deleted: true, content: '' } : m) }));
  },

  onReaction(payload, added) {
    const myId = useAuthStore.getState().user?.id;
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

  _msgLoadId: 0,
  async loadMessages(token, chatId, before) {
    const loadId = ++get()._msgLoadId;
    try {
      const data = await api.listMessages(token, chatId, before);
      set(s => {
        if (s._msgLoadId !== loadId) return {};
        return {
          messages: before ? [...(data.messages || []), ...s.messages] : (data.messages || []),
        };
      });
    } catch (e) { console.error('loadMessages error:', e); }
  },

  async sendMessage(token, chatId, content, attachments) {
    await api.sendMessage(token, chatId, content, attachments);
  },

  finishStreaming(msgId) {
    set(s => ({
      messages: s.messages.map(m => m.id === msgId ? { ...m, streaming: false } : m),
    }));
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
    const { accessToken } = useAuthStore.getState();
    try { await api.markAnnouncementRead(accessToken, chatId); } catch (e) { console.error('markAnnouncementRead error:', e); }
    set(s => ({
      chats: s.chats.map(c => c.id === chatId ? { ...c, pinned_last_read_at: new Date().toISOString() } : c),
    }));
  },

  reset() {
    set({ chats: [], activeChatId: null, messages: [], pinnedMessage: {} });
  },
}));
