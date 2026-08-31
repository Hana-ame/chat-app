import { useEffect, useState, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import ChatList from '../components/ChatList';
import ChatView from '../components/ChatView';
import MemberPanel from '../components/MemberPanel';
import WelcomeView from '../components/WelcomeView';
import { getLastRoom, setLastRoom, clearLastRoom } from '../utils/lastRoom';
import QuickSwitcher from '../components/QuickSwitcher';

export default function ChatPage() {
  const loc = useLocation();
  const navigate = useNavigate();
  const { user, accessToken, logout } = useAuthStore();
  const { wsReady, mode, connect, loadChats, chats } = useChatStore();
  const [mobileView, setMobileView] = useState('list');
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

  const notifyChat = useMemo(() => chats.find(c => c.type === 'notify'), [chats]);
  const notifyChatId = notifyChat?.id;
  const isNotification = loc.pathname === '/g/notifications';

  const urlChatId = isNotification
    ? notifyChatId
    : loc.pathname.startsWith('/g/') ? loc.pathname.slice(3) : null;

  useEffect(() => {
    useChatStore.getState().setActiveChatId(urlChatId || null);
  }, [urlChatId, accessToken]);

  // 【本地改动 2026-09-03】最近聊天记忆（FDR-026）：进入真实聊天时记录。
  // notifications 是特殊视图，不覆盖记忆。
  useEffect(() => {
    if (urlChatId && !isNotification && urlChatId !== notifyChatId) {
      setLastRoom(urlChatId);
    }
  }, [urlChatId, isNotification, notifyChatId]);

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < 768);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  useEffect(() => {
    if (accessToken) {
      connect(accessToken);
      loadChats(accessToken);
    }
    return () => useChatStore.getState().connect(null);
  }, [accessToken]);

  useEffect(() => {
    if (!accessToken) return;
    if (!notifyChat) {
      api.getNotificationsChat(accessToken).then(chat => {
        if (chat && chat.id) useChatStore.getState().onChatUpdate(chat);
      }).catch(() => {});
    }
  }, [accessToken, notifyChat]);

  useEffect(() => {
    if (urlChatId && accessToken) {
      if (isMobile) setMobileView('chat');
      if (isNotification) {
        api.notifications.markRead(accessToken).catch(() => {});
      } else {
        api.markRead(accessToken, urlChatId).catch(() => {});
      }
    }
  }, [urlChatId, accessToken]);

  useEffect(() => {
    if (!urlChatId && notifyChatId && accessToken && !isMobile) {
      const last = getLastRoom();
      const stillAccessible = last && chats.some(c => c.id === last && c.id !== notifyChatId);
      if (stillAccessible) {
        navigate('/g/' + last, { replace: true });
      } else {
        if (last) clearLastRoom();
        navigate('/g/notifications', { replace: true });
      }
    }
  }, [urlChatId, notifyChatId, chats, accessToken, isMobile, navigate]);

  // 【本地改动 2026-09-03】聊天不可达（被删/失去访问）时清除记忆并回根路径，
  // 避免 FDR-026 描述的「根 → 记忆聊天 → 403 → 根」死循环。
  useEffect(() => {
    if (!accessToken) return;
    if (urlChatId && !isNotification && urlChatId !== notifyChatId
        && chats.length > 0 && !chats.some(c => c.id === urlChatId)) {
      clearLastRoom();
      navigate('/', { replace: true });
    }
  }, [urlChatId, chats, notifyChatId, isNotification, accessToken, navigate]);

  // 【本地改动 2026-09-03】Cmd-K 快速切换（FDR-015）。
  const [qsOpen, setQsOpen] = useState(false);
  useEffect(() => {
    const onKey = (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setQsOpen(v => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const handleQuickNavigate = (chatId) => {
    setQsOpen(false);
    if (chatId === 'notifications') {
      navigate('/g/notifications', { replace: true });
    } else {
      navigate('/g/' + chatId, { replace: true });
      useChatStore.setState(s => ({
        activeChatId: chatId,
        chats: s.chats.map(c => c.id === chatId ? { ...c, unread_count: 0 } : c),
      }));
      if (isMobile) setMobileView('chat');
    }
  };

  const handleSelectChat = (id) => {
    useChatStore.setState(s => ({
      activeChatId: id,
      chats: s.chats.map(c => c.id === id ? { ...c, unread_count: 0 } : c),
    }));
    if (id === notifyChatId) {
      navigate('/g/notifications', { replace: true });
    } else {
      navigate('/g/' + id, { replace: true });
    }
    if (isMobile) setMobileView('chat');
  };

  const handleBack = () => {
    navigate('/', { replace: true });
    setMobileView('list');
  };

  const appClass = 'shell' + (isMobile ? (urlChatId && mobileView === 'chat' ? ' mobile-chat' : ' mobile-list') : '');

  return (
    <div className={appClass}>
      <ChatList onSelectChat={handleSelectChat} activeId={urlChatId}
        onLogout={() => { clearLastRoom(); logout(); }} />
       {urlChatId ? (
         <ChatView
           chatId={urlChatId}
           isNotification={isNotification}
           onBack={isMobile ? handleBack : null}
         />
       ) : isMobile ? null : <WelcomeView />}
       {!isMobile && urlChatId && !isNotification && <MemberPanel chatId={urlChatId} />}
      <QuickSwitcher
        open={qsOpen}
        onClose={() => setQsOpen(false)}
        chats={chats}
        currentUserId={user?.id}
        onNavigate={handleQuickNavigate}
      />
    </div>
  );
}
