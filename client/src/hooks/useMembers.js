import { useEffect, useCallback } from 'react';
import { useChatStore } from '../store/chat';
import { useAuthStore } from '../store/auth';
import { api } from '../api/client';
import { notify } from '../store/notification';

export function useMembers(chatId) {
  const { accessToken } = useAuthStore();
  const { chats } = useChatStore();

  const members = chats.find(c => c.id === chatId)?.members;
  const loading = members == null;

  useEffect(() => {
    if (members != null || !chatId || !accessToken) return;
    let cancelled = false;
    (async () => {
      try {
        const { mode, wsReady, wsRequest } = useChatStore.getState();
        const data = mode === 'ws' && wsReady
          ? await wsRequest('list_members', { chat_id: chatId }).then(d => d.members || [])
          : await api.listMembers(accessToken, chatId).then(d => d.members || []);
        if (cancelled) return;
        useChatStore.setState(s => ({
          chats: s.chats.map(c => c.id === chatId ? { ...c, members: data } : c),
        }));
      } catch {
        notify('Failed to load members');
      }
    })();
    return () => { cancelled = true; };
  }, [chatId, accessToken, members != null]);

  const refetch = useCallback(async () => {
    if (!chatId || !accessToken) return;
    try {
      const { mode, wsReady, wsRequest } = useChatStore.getState();
      const data = mode === 'ws' && wsReady
        ? await wsRequest('list_members', { chat_id: chatId }).then(d => d.members || [])
        : await api.listMembers(accessToken, chatId).then(d => d.members || []);
      useChatStore.setState(s => ({
        chats: s.chats.map(c => c.id === chatId ? { ...c, members: data } : c),
      }));
    } catch {
      notify('Failed to load members');
    }
  }, [chatId, accessToken]);

  const setLocalMembers = useCallback((fn) => {
    const s = useChatStore.getState();
    const chat = s.chats.find(c => c.id === chatId);
    if (!chat) return;
    const updated = fn(chat.members || []);
    useChatStore.setState(s => ({
      chats: s.chats.map(c => c.id === chatId ? { ...c, members: updated } : c),
    }));
  }, [chatId]);

  const FIVE_MIN = 300_000;
  const merged = (members || []).map(m => ({
    ...m,
    isOnline: m.last_seen ? (Date.now() - new Date(m.last_seen).getTime()) < FIVE_MIN : false,
  }));

  return { members: merged, loading, refetch, setLocalMembers };
}
