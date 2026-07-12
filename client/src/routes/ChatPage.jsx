import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import ChatList from '../components/ChatList';
import ChatView from '../components/ChatView';
import MemberPanel from '../components/MemberPanel';
import WelcomeView from '../components/WelcomeView';

export default function ChatPage() {
  const { chatId: urlChatId } = useParams();
  const navigate = useNavigate();
  const { user, accessToken, logout } = useAuthStore();
  const { wsReady, mode, connectWS, connectSSE, connectPolling, disconnect, loadChats, loadMessages } = useChatStore();
  const [mobileView, setMobileView] = useState('list');
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

  // sync URL → store for internal unread/message logic
  useEffect(() => {
    useChatStore.setState({ activeChatId: urlChatId || null });
  }, [urlChatId]);

  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < 768);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  useEffect(() => {
    if (accessToken) {
      if (mode === 'ws') connectWS(accessToken);
      else if (mode === 'sse') connectSSE(accessToken);
      else if (mode === 'poll') connectPolling(accessToken);
      loadChats(accessToken);
    }
    return () => disconnect();
  }, [accessToken]);

  useEffect(() => {
    if (urlChatId && accessToken) {
      if (isMobile) setMobileView('chat');
      const { messages } = useChatStore.getState();
      if (messages.length === 0) loadMessages(accessToken, urlChatId);
      const msgs = messages.filter(m => m.chat_id === urlChatId && !m.deleted);
      if (msgs.length > 0) {
        api.markRead(accessToken, urlChatId, msgs[msgs.length - 1].id).catch(()=>{});
      }
    }
  }, [urlChatId, accessToken]);

  const handleSelectChat = (id) => {
    navigate('/g/' + id, { replace: true });
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
           onBack={isMobile ? handleBack : null}
         />
       ) : isMobile ? null : <WelcomeView />}
       {!isMobile && urlChatId && <MemberPanel chatId={urlChatId} />}
    </div>
  );
}
