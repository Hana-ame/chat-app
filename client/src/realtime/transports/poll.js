import { api } from '../../api/client';

export function createPollTransport({ token, onChats, onMessages, getActiveChatId, onClose }) {
  let timer = null;

  const poll = async () => {
    try {
      const data = await api.listChats(token);
      onChats(data.chats || []);
    } catch (e) { console.error('Poll chats error:', e); }

    const chatId = getActiveChatId?.();
    if (chatId) {
      try {
        const data = await api.listMessages(token, chatId);
        onMessages(data.messages || []);
      } catch (e) { console.error('Poll messages error:', e); }
    }

    timer = setTimeout(poll, 2000);
  };

  poll();

  return { disconnect() { if (timer) { clearTimeout(timer); timer = null; } } };
}
