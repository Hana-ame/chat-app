import { useState, useEffect, useCallback } from 'react';
import { useChatStore } from '../store/chat';
import { useAuthStore } from '../store/auth';
import { api } from '../api/client';
import { notify } from '../store/notification';

export function useMembers(chatId) {
  const { accessToken } = useAuthStore();
  const { onlineUserIds } = useChatStore();
  const [members, setMembers] = useState([]);
  const [loading, setLoading] = useState(false);

  const fetchMembers = useCallback(async () => {
    if (!chatId || !accessToken) return;
    setLoading(true);
    try {
      const { mode, wsReady, wsRequest } = useChatStore.getState();
      const data = mode === 'ws' && wsReady
        ? await wsRequest('list_members', { chat_id: chatId }).then(d => d.members || [])
        : await api.listMembers(accessToken, chatId).then(d => d.members || []);
      setMembers(data);
    } catch {
      notify('Failed to load members');
    } finally {
      setLoading(false);
    }
  }, [chatId, accessToken]);

  useEffect(() => {
    fetchMembers();
    const id = setInterval(fetchMembers, 60000);
    return () => clearInterval(id);
  }, [fetchMembers]);

  const merged = members.map(m => ({
    ...m,
    isOnline: onlineUserIds.includes(m.id),
  }));

  const setLocalMembers = useCallback((fn) => {
    setMembers(fn);
  }, []);

  return { members: merged, loading, refetch: fetchMembers, setLocalMembers };
}
