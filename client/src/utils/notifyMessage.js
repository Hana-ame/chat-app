import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { requestNotifyPermission, sendBrowserNotification } from './browserNotify';

export function maybeNotifyMessage(msg, chats) {
  const st = useAuthStore.getState();
  const uid = st.user?.id;
  if (!uid || msg.chat_id === undefined) return;
  if (msg.type === 'stream') return;
  const chat = chats.find(c => c.id === msg.chat_id);
  // Per-chat notification toggle: off mutes everything (including mentions).
  if (chat && chat.notify_enabled === false) return;
  const blocked = st.user?.notify_blocked || [];
  if (blocked.includes(msg.user_id)) return;
  const mentioned = msg.content?.includes(`<@${uid}>`);
  const chatName = chat?.name || 'Chat';
  const authorName = msg.author?.username || 'Someone';
  if (mentioned) {
    requestNotifyPermission().then(granted => {
      if (!granted) return;
      sendBrowserNotification(`@mentioned in ${chatName}`, msg.content?.replace(/<@[^>]+>/g, '').slice(0, 120) || '', () => {
        useChatStore.getState().setActiveChatId(msg.chat_id);
      });
    });
  }
}
