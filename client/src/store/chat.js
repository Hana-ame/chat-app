import { create } from 'zustand';
import { api } from '../api/client';

export const useChatStore = create((set, get) => ({
  chats: [],
  activeChatId: null,
  messages: [],
  onlineUserIds: [],

  mode: 'ws',
  ws: null,
  wsReady: false,

  sse: null,
  sseReady: false,

  pollTimer: null,

  setMode(mode) {
    const s = get();
    s.disconnect();
    set({ mode });
    const token = s._lastToken;
    if (token) {
      if (mode === 'ws') get().connectWS(token);
      else if (mode === 'sse') get().connectSSE(token);
      else if (mode === 'poll') get().connectPolling(token);
    }
  },

  _lastToken: null,

  connectWS(token) {
    get().disconnect();
    set({ _lastToken: token });
    const isProd = location.hostname.endsWith('pages.dev');
    const host = isProd ? 'wsl-8080.moonchan.xyz' : location.host;
    const proto = isProd ? 'wss' : (location.protocol === 'https:' ? 'wss' : 'ws');
    const url = proto + '://' + host + '/ws?access_token=' + token;
    const ws = new WebSocket(url);
    ws.onopen = () => {};
    ws.onmessage = (e) => {
      try {
        const env = JSON.parse(e.data);
        switch (env.op) {
          case 'ready': {
            const p = env.payload || {};
            set({ onlineUserIds: p.online_user_ids || [], wsReady: true });
            get().setChats(p.chats || []);
            break;
          }
          case 'message_create':
            get().onMessageCreate(env.payload); break;
          case 'message_update':
            get().onMessageUpdate(env.payload); break;
          case 'message_delete':
            get().onMessageDelete(env.payload); break;
          case 'reaction_add':
          case 'reaction_remove':
            get().onReaction(env.payload, env.op === 'reaction_add'); break;
          case 'chat_create':
          case 'chat_update':
            get().onChatUpdate(env.payload); break;
          case 'chat_delete':
            get().onChatDelete(env.payload); break;
          case 'chat_remove':
            get().onChatRemove(env.payload); break;
          case 'user_update':
            set(s => ({ chats: s.chats.map(c => ({ ...c, members: c.members?.map(m => m.id === env.payload.id ? env.payload : m) })) }));
            break;
          case 'presence_update':
            set(s => {
              const ids = new Set(s.onlineUserIds);
              if (env.payload.status === 'online') ids.add(env.payload.user_id);
              else ids.delete(env.payload.user_id);
              return { onlineUserIds: [...ids] };
            });
            break;
          case 'typing':
            break;
        }
      } catch {}
    };
    ws.onclose = () => {
      set({ wsReady: false });
      setTimeout(() => {
        if (get().mode === 'ws') get().connectWS(token);
      }, 3000);
    };
    set({ ws, sse: null, sseReady: false, pollTimer: null });
  },

  connectSSE(token) {
    get().disconnect();
    set({ _lastToken: token });
    const url = api.sseUrl(token);
    const sse = new EventSource(url);
    sse.onopen = () => set({ sseReady: true });
    sse.addEventListener('ready', (e) => {
      try {
        const p = JSON.parse(e.data);
        set({ onlineUserIds: p.online_user_ids || [], sseReady: true });
        get().setChats(p.chats || []);
      } catch {}
    });
    sse.onmessage = (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.op === 'message_create') get().onMessageCreate(data.payload);
        else if (data.op === 'message_update') get().onMessageUpdate(data.payload);
        else if (data.op === 'message_delete') get().onMessageDelete(data.payload);
        else if (data.op === 'reaction_add' || data.op === 'reaction_remove') get().onReaction(data.payload, data.op === 'reaction_add');
        else if (data.op === 'chat_create' || data.op === 'chat_update') get().onChatUpdate(data.payload);
        else if (data.op === 'chat_delete') get().onChatDelete(data.payload);
        else if (data.op === 'chat_remove') get().onChatRemove(data.payload);
        else if (data.op === 'presence_update') {
          set(s => {
            const ids = new Set(s.onlineUserIds);
            if (data.payload.status === 'online') ids.add(data.payload.user_id);
            else ids.delete(data.payload.user_id);
            return { onlineUserIds: [...ids] };
          });
        }
      } catch {}
    };
    sse.onerror = () => {
      set({ sseReady: false });
      setTimeout(() => {
        if (get().mode === 'sse') get().connectSSE(token);
      }, 3000);
    };
    set({ sse, ws: null, wsReady: false, pollTimer: null });
  },

  connectPolling(token) {
    get().disconnect();
    set({ _lastToken: token, wsReady: false, sseReady: false });
    const poll = async () => {
      try {
        const data = await api.listChats(token);
        get().setChats(data.chats || []);
      } catch {}
      if (get().activeChatId) {
        try {
          const data = await api.listMessages(token, get().activeChatId);
          set({ messages: data.messages || [] });
        } catch {}
      }
      if (get().mode === 'poll') {
        const t = setTimeout(poll, 2000);
        set({ pollTimer: t });
      }
    };
    poll();
  },

  setChats(chats) {
    const sorted = (chats || []).sort((a, b) => {
      if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
      const da = a.last_message_at || a.created_at;
      const db = b.last_message_at || b.created_at;
      return new Date(db) - new Date(da);
    });
    set({ chats: sorted });
  },

  onChatUpdate(chat) {
    set(s => {
      const idx = s.chats.findIndex(c => c.id === chat.id);
      const updated = { ...chat };
      if (!updated.members) updated.members = idx >= 0 ? s.chats[idx].members : [];
      let n;
      if (idx >= 0) {
        n = [...s.chats];
        n[idx] = updated;
      } else {
        n = [updated, ...s.chats];
      }
      n.sort((a, b) => {
        if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
        const da = a.last_message_at || a.created_at;
        const db = b.last_message_at || b.created_at;
        return new Date(db) - new Date(da);
      });
      return { chats: n };
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
          if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
          return new Date(b.last_message_at||b.created_at) - new Date(a.last_message_at||a.created_at);
        }),
      };
    });
  },

  onMessageUpdate(msg) {
    set(s => ({ messages: s.messages.map(m => m.id === msg.id ? { ...m, ...msg } : m) }));
  },

  onMessageDelete(payload) {
    set(s => ({ messages: s.messages.map(m => m.id === payload.message_id ? { ...m, deleted: true, content: '' } : m) }));
  },

  onReaction(payload, added) {
    set(s => ({ messages: s.messages.map(m => {
      if (m.id !== payload.message_id) return m;
      const rxs = m.reactions || [];
      const idx = rxs.findIndex(r => r.emoji === payload.emoji);
      if (added) {
        if (idx >= 0) return m;
        return { ...m, reactions: [...rxs, { emoji: payload.emoji, count: 1, user_ids: [payload.user_id], me: payload.user_id === s.messages.find(x=>x.id===m.id)?.user_id }] };
      } else {
        return { ...m, reactions: rxs.map(r => r.emoji === payload.emoji ? { ...r, count: r.count - 1 } : r).filter(r => r.count > 0) };
      }
    }) }));
  },

  setActiveChat(chatId) {
    set(s => ({
      activeChatId: chatId,
      chats: s.chats.map(c => c.id === chatId ? { ...c, unread_count: 0 } : c),
    }));
  },

  async loadChats(token) {
    try {
      const data = await api.listChats(token);
      set({ chats: data.chats || [] });
    } catch {}
  },

  async loadMessages(token, chatId, before) {
    try {
      const data = await api.listMessages(token, chatId, before);
      const msgs = data.messages || [];
      set(s => ({
        messages: before ? [...msgs, ...s.messages] : msgs,
      }));
    } catch {}
  },

  async sendMessage(token, chatId, content, attachments) {
    await api.sendMessage(token, chatId, content, attachments);
  },

  finishStreaming(msgId) {
    set(s => ({
      messages: s.messages.map(m => m.id === msgId ? { ...m, streaming: false } : m),
    }));
  },

  startStreamingInChat(chatId, content) {
    const msg = {
      id: 'stream-' + Date.now(),
      chat_id: chatId,
      content,
      user_id: 'ai',
      author: { id: 'ai', username: 'AI Bot', avatar_color: '#10a37f' },
      created_at: new Date().toISOString(),
      streaming: true,
      deleted: false,
      attachments: [],
      reactions: [],
    };
    set(s => ({ messages: [...s.messages, msg] }));
    return msg.id;
  },

  async sendTyping(chatId) {
    const ws = get().ws;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ op: 'typing', chat_id: chatId }));
    }
  },

  async subscribe(chatId) {
    const ws = get().ws;
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ op: 'subscribe', chat_id: chatId }));
    }
  },

  async pinChat(token, chatId) {
    await api.pinChat(token, chatId);
    set(s => ({ chats: s.chats.map(c => c.id === chatId ? { ...c, pinned: true } : c) }));
  },

  async unpinChat(token, chatId) {
    await api.unpinChat(token, chatId);
    set(s => ({ chats: s.chats.map(c => c.id === chatId ? { ...c, pinned: false } : c) }));
  },

  disconnect() {
    const s = get();
    if (s.ws) { s.ws.onclose = null; s.ws.close(); }
    if (s.sse) { s.sse.close(); }
    if (s.pollTimer) { clearTimeout(s.pollTimer); }
    set({ ws: null, wsReady: false, sse: null, sseReady: false, pollTimer: null });
  },
}));
