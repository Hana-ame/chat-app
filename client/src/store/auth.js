import { create } from 'zustand';
import { api } from '../api/client';

const storage = {
  get: () => { try { return JSON.parse(localStorage.getItem('auth') || '{}'); } catch { return {}; } },
  set: (v) => localStorage.setItem('auth', JSON.stringify(v)),
  clear: () => localStorage.removeItem('auth'),
};

export const useAuthStore = create((set, get) => {
  const saved = storage.get();
  return {
    user: saved.user || null,
    loading: false,
    error: null,

    register: async (email, username, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.register(email, username, password);
        const payload = { user: data.user || data };
        storage.set(payload);
        set({ ...payload, loading: false });
      } catch (e) {
        set({ loading: false, error: e.message || 'Registration failed' });
        throw e;
      }
    },

    login: async (email, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.login(email, password);
        const payload = { user: data.user || data };
        storage.set(payload);
        set({ ...payload, loading: false });
      } catch (e) {
        set({ loading: false, error: e.message || 'Login failed' });
        throw e;
      }
    },

    refreshAuth: async () => {
      try {
        const data = await api.refresh();
        const payload = { user: data.user };
        storage.set(payload);
        set(payload);
      } catch {
        storage.clear();
        set({ user: null });
      }
    },

    logout: async () => {
      try { await api.logout(); } catch (e) { console.error('Logout error:', e); }
      storage.clear();
      set({ user: null });
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
      const payload = {
        user: { id: 'mock-' + Date.now(), username: 'DebugUser', email: 'debug@test.com', avatar_color: '#5865F2' },
      };
      storage.set(payload);
      set({ ...payload, loading: false, error: null });
    },
  };
});
