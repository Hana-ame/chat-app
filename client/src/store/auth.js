import { create } from 'zustand';
import { api } from '../api/client';

const MOCK_FLAG = 'chat:mock';

export const useAuthStore = create((set, get) => {
  if (localStorage.getItem(MOCK_FLAG) === 'true') {
    api.enableMock();
  }
  return {
    user: null,
    accessToken: null,
    booting: true,
    loading: false,
    error: null,

    boot: async () => {
      if (api.isMockEnabled()) {
        const { useChatStore } = await import('./chat');
        useChatStore.getState().setMode('poll');
        set({
          user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' },
          accessToken: 'mock-token',
          booting: false,
        });
        return;
      }
      try {
        const data = await api.refresh();
        set({ user: data.user, accessToken: data.access_token, booting: false });
      } catch {
        set({ booting: false });
      }
    },

    register: async (email, username, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.register(email, username, password);
        set({ user: data.user, accessToken: data.access_token, loading: false });
      } catch (e) {
        set({ user: null, accessToken: null, loading: false, error: e.message || 'Registration failed' });
        throw e;
      }
    },

    login: async (email, password) => {
      set({ loading: true, error: null });
      try {
        const data = await api.login(email, password);
        set({ user: data.user, accessToken: data.access_token, loading: false });
      } catch (e) {
        set({ user: null, accessToken: null, loading: false, error: e.message || 'Login failed' });
        throw e;
      }
    },

    refreshAuth: async () => {
      try {
        const data = await api.refresh();
        set({ user: data.user, accessToken: data.access_token });
      } catch {
        set({ user: null, accessToken: null });
      }
    },

    logout: async () => {
      api.disableMock();
      localStorage.removeItem(MOCK_FLAG);
      if (get().accessToken) {
        try { await api.logout(get().accessToken); } catch (e) { console.error('Logout error:', e); }
      }
      const { useChatStore } = await import('./chat');
      useChatStore.getState().disconnect();
      useChatStore.getState().reset();
      set({ user: null, accessToken: null });
    },

    setUser: (user) => {
      set({ user });
    },

    debugMode: false,
    setDebugMode: (v) => {
      set({ debugMode: v });
    },

    mockLogin: () => {
      localStorage.setItem(MOCK_FLAG, 'true');
      api.enableMock();
      import('./chat').then(m => m.useChatStore.getState().setMode('mock'));
      set({
        user: { id: 'dev-self', username: 'Alice', email: 'alice@test.com', avatar_color: '#5865F2' },
        accessToken: 'mock-token',
        loading: false, error: null,
      });
    },
  };
});