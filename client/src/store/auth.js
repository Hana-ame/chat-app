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
    accessToken: saved.accessToken || null,
    refreshToken: saved.refreshToken || null,
    loading: false,
    error: null,

    register: async (email, username, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.register(email, username, password);
        const payload = { user: data.user || data, accessToken: data.access_token, refreshToken: data.refresh_token };
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
        const payload = { user: data.user || data, accessToken: data.access_token, refreshToken: data.refresh_token };
        storage.set(payload);
        set({ ...payload, loading: false });
      } catch (e) {
        set({ loading: false, error: e.message || 'Login failed' });
        throw e;
      }
    },

    refreshAuth: async () => {
      const rt = get().refreshToken;
      if (!rt) { set({ accessToken: null, refreshToken: null, user: null }); return; }
      try {
        const data = await api.refresh(rt);
        const payload = { user: data.user, accessToken: data.access_token, refreshToken: data.refresh_token };
        storage.set(payload);
        set(payload);
      } catch {
        storage.clear();
        set({ user: null, accessToken: null, refreshToken: null });
      }
    },

    logout: async () => {
      try { await api.logout(get().accessToken, get().refreshToken); } catch {}
      storage.clear();
      set({ user: null, accessToken: null, refreshToken: null });
    },

    setUser: (user) => {
      set({ user });
      const s = storage.get();
      s.user = user;
      storage.set(s);
    },
  };
});
