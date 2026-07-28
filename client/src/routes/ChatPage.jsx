import { useEffect, useState, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import ChatList from '../components/ChatList';
import ChatView from '../components/ChatView';
import MemberPanel from '../components/MemberPanel';
import WelcomeView from '../components/WelcomeView';

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
    useChatStore.setState({ activeChatId: urlChatId || null });
  }, [urlChatId, accessToken]);

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
    if (!urlChatId && notifyChatId && accessToken) {
      navigate('/g/notifications', { replace: true });
    }
  }, [urlChatId, notifyChatId, accessToken, navigate]);

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
      <ChatList onSelectChat={handleSelectChat} activeId={urlChatId} onLogout={logout} />
       {urlChatId ? (
         <ChatView
           chatId={urlChatId}
           isNotification={isNotification}
           onBack={isMobile ? handleBack : null}
         />
       ) : isMobile ? null : <WelcomeView />}
       {!isMobile && urlChatId && !isNotification && <MemberPanel chatId={urlChatId} />}
    </div>
  );
}
