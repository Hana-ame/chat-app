import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import ChatList from '../components/ChatList';
import ChatView from '../components/ChatView';
import MemberPanel from '../components/MemberPanel';

export default function ChatPage() {
  const { chatId: urlChatId } = useParams();
  const navigate = useNavigate();
  const { user, accessToken, logout } = useAuthStore();
  const { setActiveChat, activeChatId, wsReady, mode, connectWS, connectSSE, connectPolling, disconnect, loadChats, loadMessages } = useChatStore();
  const [mobileView, setMobileView] = useState('list');
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);

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
      setActiveChat(urlChatId);
      if (isMobile) setMobileView('chat');
    }
  }, [urlChatId, accessToken]);

  useEffect(() => {
    if (activeChatId && accessToken) {
      const { messages } = useChatStore.getState();
      if (messages.length === 0) loadMessages(accessToken, activeChatId);
      const msgs = messages.filter(m => m.chat_id === activeChatId && !m.deleted);
      if (msgs.length > 0) {
        api.markRead(accessToken, activeChatId, msgs[msgs.length - 1].id).catch(()=>{});
      }
    }
  }, [activeChatId, accessToken]);

  const handleSelectChat = (id) => {
    setActiveChat(id);
    navigate('/g/' + id, { replace: true });
    if (isMobile) setMobileView('chat');
  };

  const handleBack = () => {
    setActiveChat(null);
    navigate('/', { replace: true });
    setMobileView('list');
  };

  const appClass = 'shell' + (isMobile ? (activeChatId && mobileView === 'chat' ? ' mobile-chat' : ' mobile-list') : '');

  return (
    <div className={appClass}>
      <ChatList onSelectChat={handleSelectChat} activeId={activeChatId} onLogout={logout} />
      {activeChatId ? (
        <ChatView
          chatId={activeChatId}
          onBack={isMobile ? handleBack : null}
        />
      ) : isMobile ? null : (
        <div className="main" style={{alignItems:'center',justifyContent:'center',color:'var(--text-muted)'}}>
          Select a conversation
        </div>
      )}
      {!isMobile && activeChatId && <MemberPanel chatId={activeChatId} />}
    </div>
  );
}
