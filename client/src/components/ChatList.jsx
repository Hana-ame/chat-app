import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import pkg from '../../package.json';
import ChatListItem from './ChatListItem';
import PublicChannelList from './PublicChannelList';
import CreateGroupForm from './CreateGroupForm';
import { notify } from '../store/notification';
import ChatInfoModal from './ChatInfoModal';
import UserProfileModal from './UserProfileModal';
import ScrollArea from './ScrollArea';
import EmptyState from './EmptyState';
import SidebarFooter from './SidebarFooter';

const MODES = [
  { key: 'ws', label: 'WS' },
  { key: 'sse', label: 'SSE' },
  { key: 'poll', label: 'Poll' },
];

export default function ChatList({ onSelectChat, activeId, onLogout }) {
  const { user, accessToken } = useAuthStore();
  const { chats, mode, setMode } = useChatStore();
  const [showCreate, setShowCreate] = useState(false);
  const [newChatName, setNewChatName] = useState('');
  const [newChatVisibility, setNewChatVisibility] = useState('public');
  const [chatSearch, setChatSearch] = useState('');
  const [publicResults, setPublicResults] = useState(null);
  const [publicSearching, setPublicSearching] = useState(false);
  const [showPublicList, setShowPublicList] = useState(false);
  const allPublicChats = useRef(null);
  const searchInputRef = useRef(null);
  const searchTermRef = useRef('');
  const [contextMenu, setContextMenu] = useState(null);
  const [showChatInfo, setShowChatInfo] = useState(null);
  const [showProfile, setShowProfile] = useState(false);

  const joinAction = chatSearch.trim() && /^(join|create)\s/i.test(chatSearch.trim())
    ? chatSearch.trim().startsWith('join') ? 'join' : 'create' : null;

  const navigate = useNavigate();

  const closeContextMenu = useCallback(() => setContextMenu(null), []);

  useEffect(() => {
    if (!contextMenu) return;
    const close = (e) => { if (!e.target.closest('.context-menu')) setContextMenu(null); };
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [contextMenu]);

  const handleCreate = async () => {
    if (!newChatName.trim()) { notify('Please enter a group name', 'error'); return; }
    try {
      const data = await api.createChat(accessToken, newChatName, [], newChatVisibility);
      setShowCreate(false);
      setNewChatName('');
      onSelectChat(data.id);
    } catch (e) { alert(e.message); }
  };

  const handleLeaveChat = async (chatId) => {
    try {
      await api.removeMember(accessToken, chatId, user.id);
      if (chatId === activeId) {
        navigate('/', { replace: true });
        queueMicrotask(() => useChatStore.getState().onChatDelete({ chat_id: chatId }));
      } else {
        useChatStore.getState().onChatDelete({ chat_id: chatId });
      }
    } catch (e) { console.error('Leave chat error:', e); }
    setContextMenu(null);
    setShowChatInfo(null);
  };

  const handlePin = async (chatId) => {
    try {
      await api.pinChat(accessToken, chatId);
      useChatStore.getState().onChatUpdate({ id: chatId, pinned: true });
    } catch (e) { console.error('Pin error:', e); }
    setContextMenu(null);
  };

  const handleDelete = async (chatId) => {
    if (!confirm('Are you sure you want to delete this group chat? This cannot be undone.')) return;
    try {
      await api.deleteChat(accessToken, chatId);
      useChatStore.getState().onChatDelete({ chat_id: chatId });
      if (chatId === activeId) {
        navigate('/', { replace: true });
      }
    } catch (e) { console.error('Delete chat error:', e); }
    setContextMenu(null);
  };

  const loadAllPublicChats = async () => {
    if (allPublicChats.current) {
      const q = searchTermRef.current;
      setPublicResults(q.trim() ? filterPublicChats(allPublicChats.current, q) : allPublicChats.current);
      setShowPublicList(true);
      return;
    }
    setPublicSearching(true);
    try {
      const data = await api.listPublicChats(accessToken);
      allPublicChats.current = data.chats || [];
      const q = searchTermRef.current;
      setPublicResults(q.trim() ? filterPublicChats(allPublicChats.current, q) : allPublicChats.current);
      setShowPublicList(true);
    } catch (e) { console.error('Load public chats error:', e); }
    setPublicSearching(false);
  };

  const filterPublicChats = (list, q) => {
    const lower = q.toLowerCase();
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(q)) {
      return list.filter(c => c.id === q);
    }
    return list.filter(c => c.name?.toLowerCase().includes(lower));
  };

  const handleSearchFocus = () => {
    loadAllPublicChats();
  };

  const handleSearchChange = (q) => {
    searchTermRef.current = q;
    setChatSearch(q);
    if (allPublicChats.current) {
      setPublicResults(q.trim() ? filterPublicChats(allPublicChats.current, q) : allPublicChats.current);
    }
  };

  const handleSearchBlur = () => {
    setTimeout(() => setShowPublicList(false), 200);
  };

  const handleJoinPublic = async (chatId) => {
    try {
      await api.joinChat(accessToken, chatId);
      const data = await api.listChats(accessToken);
      useChatStore.getState().setChats(data.chats || []);
      onSelectChat(chatId);
    } catch (e) { alert(e.message); }
  };

  const joinChatByID = async (chatId) => {
    try {
      await api.joinChat(accessToken, chatId);
      setChatSearch('');
      searchTermRef.current = '';
      setShowPublicList(false);
      setPublicResults(null);
      const data = await api.listChats(accessToken);
      useChatStore.getState().setChats(data.chats || []);
    } catch (e) { alert(e.message); }
  };

  const isUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(chatSearch.trim());

  const filteredChats = useMemo(() => chats.filter(c => {
    if (c.type === 'dm') return false;
    if (!chatSearch.trim()) return true;
    if (isUUID) return c.id === chatSearch.trim();
    const q = chatSearch.toLowerCase();
    const name = c.name || '';
    return name.toLowerCase().includes(q);
  }), [chats, chatSearch, isUUID]);

  const uuidDM = isUUID && chatSearch.trim() ? chats.find(c => c.type === 'dm' && c.id === chatSearch.trim()) : null;

  return (
    <div className="sidebar">
      <div className="sidebar-header">
        <h3 style={{ fontSize: 13, fontWeight: 700 }}>{pkg.version}</h3>
        <div style={{ display: 'flex', gap: 4, alignItems: 'stretch' }}>
          <button className="btn-ghost" style={{ fontSize: 9, fontWeight: 700, minWidth: 34, textAlign: 'center', padding: '4px 2px', lineHeight: 1.2 }}
            onClick={() => setMode(MODES[(MODES.findIndex(m => m.key === mode) + 1) % MODES.length].key)} title={'Click to switch: ' + ['WS', 'SSE', 'Poll'][(['ws', 'sse', 'poll'].indexOf(mode) + 1) % 3]}>
            {mode.toUpperCase()}
          </button>
          <button className={'btn-ghost' + (showCreate ? ' active-mode' : '')} style={{ minWidth: 34 }} title="Create Group" onClick={() => setShowCreate(v => !v)}>+</button>
        </div>
      </div>

      <div className="sidebar-search-row">
        {showCreate ? (
          <CreateGroupForm name={newChatName} visibility={newChatVisibility}
            onVisibilityChange={setNewChatVisibility}
            onNameChange={e => setNewChatName(e.target.value)}
            onNameKeyDown={e => e.key === 'Enter' && handleCreate()}
            onCreate={handleCreate}
            onCancel={() => setShowCreate(false)} />
        ) : (
          <>
            <input className="input-field" placeholder="Search chats..." value={chatSearch}
              ref={searchInputRef}
              onFocus={handleSearchFocus}
              onBlur={handleSearchBlur}
              onChange={e => handleSearchChange(e.target.value)}
              style={{ fontSize: 14, padding: '8px 10px' }} />
            {joinAction && (
              <div style={{ display: 'flex', gap: 6, marginTop: 6 }}>
                {joinAction === 'join' && (() => {
                  const id = chatSearch.trim().replace(/^join\s+/i, '');
                  return (
                    <button className="btn" style={{ flex: 1, padding: '8px 12px', fontSize: 14, background: 'var(--accent)', color: '#fff', borderRadius: 'var(--radius)' }}
                      onClick={() => joinChatByID(id)}>
                      Join #{id}
                    </button>
                  );
                })()}
                {joinAction === 'create' && (
                  <button className="btn" style={{ flex: 1, padding: '8px 12px', fontSize: 14, background: 'var(--accent)', color: '#fff', borderRadius: 'var(--radius)' }}
                    onClick={() => { setNewChatName(chatSearch.trim()); setShowCreate(true); }}>
                    Create &ldquo;{chatSearch.trim()}&rdquo;
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </div>

      <ScrollArea className="sidebar-body">
        {showPublicList && <PublicChannelList results={publicResults} searching={publicSearching} onJoin={handleJoinPublic} />}

        {!showPublicList && (uuidDM ? (
          <ChatListItem key={uuidDM.id} chat={uuidDM} activeId={activeId} onSelectChat={onSelectChat}
            onContextMenu={setContextMenu} />
        ) : filteredChats.map(c => (
          <ChatListItem key={c.id} chat={c} activeId={activeId} onSelectChat={onSelectChat}
            onContextMenu={setContextMenu} />
        )))}

        {!showPublicList && chats.length === 0 && (
          <EmptyState message="No conversations yet. Create a new group!" />
        )}
      </ScrollArea>

      <SidebarFooter user={user} onLogout={onLogout} onSettings={() => setShowProfile(true)} />

      {contextMenu && (
        <div className="context-menu" style={{ position: 'fixed', left: contextMenu.x, top: contextMenu.y, zIndex: 1000, right: 'auto', width: 140 }}>
          {(() => { const c = chats.find(x => x.id === contextMenu.chatId); return c?.pinned
            ? <button className="context-menu-item" onClick={() => handleUnpin(contextMenu.chatId)}>Unpin</button>
            : <button className="context-menu-item" onClick={() => handlePin(contextMenu.chatId)}>Pin</button>;
          })()}
          <button className="context-menu-item" onClick={() => { setShowChatInfo(contextMenu.chatId); setContextMenu(null); }}>View Info</button>
          {(() => { const c = chats.find(x => x.id === contextMenu.chatId); return c?.type !== 'dm' && c?.owner_id === user.id
            ? <button className="context-menu-item danger" onClick={() => handleDelete(contextMenu.chatId)}>Delete</button>
            : null;
          })()}
          <button className="context-menu-item danger" onClick={() => handleLeaveChat(contextMenu.chatId)}>Leave</button>
        </div>
      )}

      {showChatInfo && (
        <ChatInfoModal chatId={showChatInfo} onClose={() => setShowChatInfo(null)} />
      )}
      {showProfile && (
        <UserProfileModal user={user} onClose={() => setShowProfile(false)} />
      )}
    </div>
  );
}