import { useEffect, useState } from 'react';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';

export default function WelcomeView() {
  const { accessToken } = useAuthStore();
  const { chats, setActiveChat } = useChatStore();
  const [publicChats, setPublicChats] = useState([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const data = await api.listPublicChats(accessToken, page, 20);
        setPublicChats(data.chats || []);
      } catch (e) {
        console.error('Failed to load public chats:', e);
      }
      setLoading(false);
    })();
  }, [accessToken, page]);

  const handleEnter = async (chatId) => {
    const isMember = chats.some(c => c.id === chatId);
    if (!isMember) {
      try {
        await api.joinChat(accessToken, chatId);
      } catch (e) {
        console.error('Failed to join chat:', e);
        return;
      }
    }
    setActiveChat(chatId);
    window.history.pushState(null, '', '/g/' + chatId);
  };

  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
      <div style={{ padding: '24px 24px 8px', fontSize: 14, fontWeight: 600, color: 'var(--text-primary)' }}>
        Public Chats
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: '0 24px' }}>
        {loading && publicChats.length === 0 ? (
          <div style={{ textAlign: 'center', color: 'var(--text-muted)', paddingTop: 40, fontSize: 13 }}>Loading...</div>
        ) : publicChats.length === 0 ? (
          <div style={{ textAlign: 'center', color: 'var(--text-muted)', paddingTop: 40, fontSize: 13 }}>
            No public chats yet. Create a group and set it to public!
          </div>
        ) : (
          publicChats.map(c => (
            <div key={c.id} className="public-chat-card" onClick={() => handleEnter(c.id)}>
              <div className="chat-item-avatar" style={{ background: c.icon_color || '#5865F2' }}>
                {c.name ? c.name[0].toUpperCase() : '?'}
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                  <span style={{ fontWeight: 600, fontSize: 14, color: 'var(--text-primary)' }}>{c.name}</span>
                  <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{c.member_count || c.members?.length || 0} members</span>
                </div>
                <div style={{ fontSize: 13, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginTop: 2 }}>
                  {c.last_message?.content || 'No messages yet'}
                </div>
              </div>
              <button className="btn-ghost" style={{ fontSize: 12, whiteSpace: 'nowrap', marginLeft: 8 }}
                onClick={e => { e.stopPropagation(); handleEnter(c.id); }}>
                {chats.some(x => x.id === c.id) ? 'Open' : 'Join'}
              </button>
            </div>
          ))
        )}
        {publicChats.length > 0 && (
          <div style={{ display: 'flex', justifyContent: 'center', gap: 12, padding: '16px 0' }}>
            <button className="btn-ghost" style={{ fontSize: 13 }} disabled={page <= 1 || loading} onClick={() => setPage(p => p - 1)}>
              ← Prev
            </button>
            <span style={{ fontSize: 13, color: 'var(--text-muted)', lineHeight: '28px' }}>{page}</span>
            <button className="btn-ghost" style={{ fontSize: 13 }} disabled={publicChats.length < 20 || loading} onClick={() => setPage(p => p + 1)}>
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
