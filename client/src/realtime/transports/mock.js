export function createMockTransport({ token, onReady }) {
  let timer = null;

  onReady({ onlineUserIds: [], chats: [] });

  timer = setInterval(async () => {
    const { useChatStore } = await import('../../store/chat');
    const s = useChatStore.getState();
    s.loadChats(token);
    if (s.activeChatId) s.loadMessages(token, s.activeChatId);
  }, 500);

  return { disconnect() { if (timer) { clearInterval(timer); timer = null; } } };
}
