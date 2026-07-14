import { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/auth';
import { useChatStore } from '../store/chat';
import { api } from '../api/client';
import pkg from '../../package.json';
import ChatListItem from './ChatListItem';
import PublicChannelList from './PublicChannelList';
import CreateGroupForm from './CreateGroupForm';
import SettingsModal from './SettingsModal';
import ChatInfoModal from './ChatInfoModal';
import ScrollArea from './ScrollArea';
import EmptyState from './EmptyState';
import ImagePreviewModal from './ImagePreviewModal';

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
  const [newChatVisibility, setNewChatVisibility] = useState('private');
  const [chatSearch, setChatSearch] = useState('');
  const [publicResults, setPublicResults] = useState(null);
  const [publicSearching, setPublicSearching] = useState(false);
  const [showPublicList, setShowPublicList] = useState(false);
  const allPublicChats = useRef(null);
  const searchInputRef = useRef(null);
  const [contextMenu, setContextMenu] = useState(null); // { chatId, x, y }
  const [showSettings, setShowSettings] = useState(false);
  const [showChatInfo, setShowChatInfo] = useState(null); // chatId
  const [previewUrl, setPreviewUrl] = useState(null);
  const [avatarError, setAvatarError] = useState(false);

  const joinAction = chatSearch.trim() && /^\d{1,2}-\d{1,2}$/.test(chatSearch.trim()) ? 'join'
    : chatSearch.trim() && /^\d+$/.test(chatSearch.trim()) ? 'join'
      : chatSearch.trim() && /^(join|create)\s/i.test(chatSearch.trim()) ? chatSearch.trim().startsWith('join') ? 'join' : 'create'
        : null;

  const navigate = useNavigate();

  const closeContextMenu = useCallback(() => setContextMenu(null), []);

  useEffect(() => {
    if (!contextMenu) return;
    const close = (e) => { if (!e.target.closest('.context-menu')) setContextMenu(null); };
    document.addEventListener('click', close);
    return () => document.removeEventListener('click', close);
  }, [contextMenu]);

  const handleCreate = async () => {
    if (!newChatName.trim()) return;
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

  const handleTogglePin = async (chatId) => {
    try {
      await api.togglePin(accessToken, chatId);
      useChatStore.getState().onChatUpdate({ id: chatId, pinned: !useChatStore.getState().chats.find(c => c.id === chatId)?.pinned });
    } catch (e) { console.error('Toggle pin error:', e); }
    setContextMenu(null);
  };

  const loadAllPublicChats = async () => {
    if (allPublicChats.current) {
      setPublicResults(
        chatSearch.trim()
          ? filterPublicChats(allPublicChats.current, chatSearch)
          : allPublicChats.current
      );
      setShowPublicList(true);
      return;
    }
    setPublicSearching(true);
    try {
      const data = await api.listPublicChats(accessToken);
      allPublicChats.current = data.chats || [];
      setPublicResults(
        chatSearch.trim()
          ? filterPublicChats(allPublicChats.current, chatSearch)
          : allPublicChats.current
      );
      setShowPublicList(true);
    } catch (e) { console.error('Load public chats error:', e); }
    setPublicSearching(false);
  };

  const filterPublicChats = (list, q) => {
    const lower = q.toLowerCase();
    return list.filter(c => c.name?.toLowerCase().includes(lower) || c.id.toLowerCase().includes(lower));
  };

  const handleSearchFocus = () => {
    loadAllPublicChats();
  };

  const handleSearchChange = (q) => {
    setChatSearch(q);
    if (!showPublicList) {
      setShowPublicList(true);
    }
    if (allPublicChats.current) {
      setPublicResults(q.trim() ? filterPublicChats(allPublicChats.current, q) : allPublicChats.current);
    }
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
      setShowPublicList(false);
      setPublicResults(null);
      const data = await api.listChats(accessToken);
      useChatStore.getState().setChats(data.chats || []);
    } catch (e) { alert(e.message); }
  };

  const handleSaveSettings = async (name) => {
    const payload = { username: name };
    const file = document.getElementById('avatar-file-input')?.files?.[0];
    if (file) {
      const data = await api.uploadAvatar(accessToken, file);
      payload.avatar_url = data.url;
    }
    const updated = await api.updateProfile(accessToken, payload);
    useAuthStore.getState().setUser(updated);
    setShowSettings(false);
  };

  const filteredChats = chats.filter(c => {
    if (c.type === 'dm') return false;
    if (!chatSearch.trim()) return true;
    const q = chatSearch.toLowerCase();
    const name = c.name || '';
    return name.toLowerCase().includes(q) || c.id.toLowerCase().includes(q);
  });

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

      {!showCreate && (
        <div className="sidebar-search-row">
          <input className="input-field" placeholder="Search chats..." value={chatSearch}
            ref={searchInputRef}
            onFocus={handleSearchFocus}
            onChange={e => handleSearchChange(e.target.value)}
            style={{ fontSize: 14, padding: '8px 10px' }} />
          {joinAction && (
            <div style={{ display: 'flex', gap: 6, marginTop: 6 }}>
              {joinAction === 'join' && (
                <button className="btn" style={{ flex: 1, padding: '8px 12px', fontSize: 14, background: 'var(--accent)', color: '#fff', borderRadius: 'var(--radius)' }}
                  onClick={() => joinChatByID(chatSearch.trim())}>
                  Join #{chatSearch.trim()}
                </button>
              )}
              {joinAction === 'create' && (
                <button className="btn" style={{ flex: 1, padding: '8px 12px', fontSize: 14, background: 'var(--accent)', color: '#fff', borderRadius: 'var(--radius)' }}
                  onClick={() => { setNewChatName(chatSearch.trim()); setShowCreate(true); }}>
                  Create &ldquo;{chatSearch.trim()}&rdquo;
                </button>
              )}
            </div>
          )}
        </div>
      )}

      <ScrollArea className="sidebar-body">
        {showCreate && (
          <CreateGroupForm name={newChatName} visibility={newChatVisibility}
            onVisibilityChange={setNewChatVisibility}
            onNameChange={e => setNewChatName(e.target.value)}
            onNameKeyDown={e => e.key === 'Enter' && handleCreate()}
            onCreate={handleCreate}
            onCancel={() => setShowCreate(false)} />
        )}
        {showPublicList && <PublicChannelList results={publicResults} searching={publicSearching} onJoin={handleJoinPublic} />}

        {filteredChats.map(c => (
          <ChatListItem key={c.id} chat={c} activeId={activeId} onSelectChat={onSelectChat}
            onContextMenu={setContextMenu} />
        ))}

        {chats.length === 0 && (
          <EmptyState message="No conversations yet. Create a new group!" />
        )}
      </ScrollArea>

      <div className="sidebar-footer">
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }} onClick={(e) => { if (!e.target.closest('img')) setShowSettings(true); }}>
          {user?.avatar_url && !avatarError
            ? <img src={user.avatar_url} className="user-avatar-img" alt="" onClick={e => { e.stopPropagation(); setPreviewUrl(user.avatar_url); }} onError={() => setAvatarError(true)} />
            : <div className="chat-item-avatar" style={{ width: 32, height: 32, fontSize: 13, background: user?.avatar_color || '#5865F2' }} onClick={() => setShowSettings(true)}>
              {(user?.username?.[0] || '?').toUpperCase()}
            </div>
          }
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 14, fontWeight: 600 }}>{user?.username || 'Unknown'}</div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>Online</div>
          </div>
          <button className="btn-ghost" onClick={(e) => { e.stopPropagation(); setShowSettings(true); }} title="Settings">⚙</button>
          <button className="btn-ghost" onClick={onLogout}>↪</button>
        </div>
        {api.isMockEnabled() && (
          <div style={{ marginTop: 4, borderTop: '1px solid var(--border)', paddingTop: 4 }}>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', textAlign: 'center', padding: '4px 0' }}>
              ⚡ Using Mock API
            </div>
          </div>
        )}
      </div>

      {contextMenu && (
        <div className="context-menu" style={{ position: 'fixed', left: contextMenu.x, top: contextMenu.y, zIndex: 1000, right: 'auto', width: 140 }}>
          <button className="context-menu-item" onClick={() => handleTogglePin(contextMenu.chatId)}>{chats.find(c => c.id === contextMenu.chatId)?.pinned ? 'Unpin' : 'Pin'}</button>
          <button className="context-menu-item" onClick={() => { setShowChatInfo(contextMenu.chatId); setContextMenu(null); }}>View Info</button>
          <button className="context-menu-item danger" onClick={() => handleLeaveChat(contextMenu.chatId)}>Leave</button>
        </div>
      )}

      {showChatInfo && (
        <ChatInfoModal chatId={showChatInfo} onClose={() => setShowChatInfo(null)} />
      )}
      {showSettings && (
        <SettingsModal user={user} onClose={() => setShowSettings(false)} onSave={handleSaveSettings} />
      )}
      {previewUrl && (
        <ImagePreviewModal url={previewUrl} onClose={() => setPreviewUrl(null)} />
      )}
    </div>
  );
}