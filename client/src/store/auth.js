import { create } from 'zustand';
import { api } from '../api/client';

const storage = {
  get: () => { try { return JSON.parse(localStorage.getItem('auth') || '{}'); } catch { return {}; } },
  set: (v) => localStorage.setItem('auth', JSON.stringify(v)),
  clear: () => localStorage.removeItem('auth'),
};

export const useAuthStore = create((set, get) => {
  const saved = storage.get();
  if (saved.accessToken === 'mock-token') {
    api.enableMock();
  }
  return {
    user: saved.user || null,
    accessToken: saved.accessToken || null,
    loading: false,
    error: null,

    register: async (email, username, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.register(email, username, password);
        const payload = { user: data.user, accessToken: data.access_token };
        storage.set(payload);
        set({ ...payload, loading: false });
      } catch (e) {
        storage.clear();
        set({ user: null, accessToken: null, loading: false, error: e.message || 'Registration failed' });
        throw e;
      }
    },

    login: async (email, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.login(email, password);
        const payload = { user: data.user, accessToken: data.access_token };
        storage.set(payload);
        set({ ...payload, loading: false });
      } catch (e) {
        storage.clear();
        set({ user: null, accessToken: null, loading: false, error: e.message || 'Login failed' });
        throw e;
      }
    },

    refreshAuth: async () => {
      try {
        const data = await api.refresh();
        const payload = { user: data.user, accessToken: data.access_token };
        storage.set(payload);
        set(payload);
      } catch {
        storage.clear();
        set({ user: null, accessToken: null });
      }
    },

    logout: async () => {
      api.disableMock();
      if (get().accessToken) {
        try { await api.logout(get().accessToken); } catch (e) { console.error('Logout error:', e); }
      }
      const { useChatStore } = await import('./chat');
      useChatStore.getState().reset();
      storage.clear();
      set({ user: null, accessToken: null });
    },

    setUser: (user) => {
      set({ user });
      const s = storage.get();
      s.user = user;
      storage.set(s);
    },

    debugMode: saved.debugMode || false,
    setDebugMode: (v) => {
      set({ debugMode: v });
      const s = storage.get();
      s.debugMode = v;
      storage.set(s);
    },

    mockLogin: () => {
      api.enableMock();
      import('./chat').then(m => m.useChatStore.getState().setMode('poll'));
      const payload = {
        user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' },
        accessToken: 'mock-token',
      };
      storage.set(payload);
      set({ ...payload, loading: false, error: null });
    },
  };
});