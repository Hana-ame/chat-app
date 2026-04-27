import { useState, useEffect, useRef, useCallback } from 'react';

const WS_RECONNECT_DELAYS = [2000, 5000, 10000, 30000, 60000];

const isPages = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');
const API_BASE = isPages ? 'https://wsl-8000.moonchan.xyz' : '';
const WS_BASE = isPages ? 'wss://wsl-8000.moonchan.xyz/ws' : null;

export default function useChat() {
  const [user, setUser] = useState(null);
  const [messages, setMessages] = useState([]);
  const [connStatus, setConnStatus] = useState('offline');
  const [onlineCount, setOnlineCount] = useState(0);
  const [onlineUsers, setOnlineUsers] = useState([]);
  const [activeRoom, setActiveRoom] = useState(1);

  const wsRef = useRef(null);
  const pollRef = useRef(false);
  const lastMsgIdRef = useRef(0);
  const minMsgIdRef = useRef(Infinity);
  const retryIdxRef = useRef(0);
  const heartbeatRef = useRef(null);
  const roomRef = useRef(1);

  const addMessages = useCallback((msgs) => {
    if (!msgs || msgs.length === 0) return;
    setMessages(prev => {
      const existingIds = new Set(prev.map(m => m.id));
      const newMsgs = msgs.filter(m => !existingIds.has(m.id));
      if (newMsgs.length === 0) return prev;
      const combined = [...prev, ...newMsgs].sort((a, b) => a.id - b.id);
      lastMsgIdRef.current = combined[combined.length - 1].id;
      minMsgIdRef.current = combined[0].id;
      return combined;
    });
  }, []);

  // 处理服务端消息
  const handleMessage = useCallback((msg) => {
    if (msg.type === 'pong') return;
    if (msg.type === 'system') {
      if (msg.online_count != null) setOnlineCount(msg.online_count);
      // Track online users from join/leave messages, but don't show in chat
      const content = msg.content || '';
      const joinMatch = content.match(/^\[Bot\]\s*(.+)\s+joined|^(.+)\s+joined/);
      const leaveMatch = content.match(/^\[Bot\]\s*(.+)\s+left|^(.+)\s+left/);
      if (joinMatch) {
        const name = joinMatch[1] || joinMatch[2];
        if (name && name !== '系统通知') setOnlineUsers(prev => { const s = new Set(prev); s.add(name); return [...s]; });
      } else if (leaveMatch) {
        const name = leaveMatch[1] || leaveMatch[2];
        if (name) setOnlineUsers(prev => prev.filter(u => u !== name));
      }
      return;
    }
    addMessages([msg]);
  }, [addMessages]);

  // ---- 长轮询逻辑 ----
  const startPolling = useCallback(() => {
    if (pollRef.current) return;
    pollRef.current = true;
    setConnStatus('poll');

    const pollLoop = async () => {
      while (pollRef.current) {
        try {
          const res = await fetch(`${API_BASE}/api/poll?room_id=${roomRef.current}&token=${user.token}&after_id=${lastMsgIdRef.current}&timeout=30`);
          if (res.status === 401) { pollRef.current = false; logout(); return; }
          const data = await res.json();
          if (data.messages && data.messages.length > 0) addMessages(data.messages);
        } catch (e) {
          await new Promise(r => setTimeout(r, 3000)); // 网络错误等3秒
        }
      }
    };
    pollLoop();
  }, [user, addMessages]);

  const stopPolling = useCallback(() => {
    pollRef.current = false;
  }, []);

  // ---- WS 逻辑 ----
  const connectWS = useCallback(() => {
    if (!user || wsRef.current) return;
    
    const wsUrl = WS_BASE
      ? `${WS_BASE}/${roomRef.current}?token=${user.token}`
      : `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/${roomRef.current}?token=${user.token}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      retryIdxRef.current = 0;
      setConnStatus('ws');
      stopPolling(); // WS 连上，停轮询
      startHeartbeat();
    };

    ws.onmessage = (e) => {
      try { handleMessage(JSON.parse(e.data)); } catch (err) {}
    };

    ws.onclose = (e) => {
      wsRef.current = null;
      stopHeartbeat();
      if (e.code === 4001) { logout(); return; }
      
      setConnStatus('offline');
      startPolling(); // 降级轮询

      // 后台尝试 WS 重连
      const delay = WS_RECONNECT_DELAYS[Math.min(retryIdxRef.current, WS_RECONNECT_DELAYS.length - 1)];
      retryIdxRef.current++;
      setTimeout(() => { if (!wsRef.current) connectWS(); }, delay);
    };

    ws.onerror = () => {}; // onclose 会处理
  }, [user, startPolling, stopPolling, handleMessage]);

  // ---- 心跳 (Doze 适配：屏幕暗时暂停) ----
  const startHeartbeat = () => {
    stopHeartbeat();
    heartbeatRef.current = setInterval(() => {
      if (wsRef.current?.readyState === WebSocket.OPEN && !document.hidden) {
        wsRef.current.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000);
  };

  const stopHeartbeat = () => {
    if (heartbeatRef.current) clearInterval(heartbeatRef.current);
    heartbeatRef.current = null;
  };

  // 页面可见性变化
  useEffect(() => {
    const onVisChange = () => {
      if (!document.hidden && wsRef.current?.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: 'ping' })); // 恢复前台立刻检测
      }
    };
    document.addEventListener('visibilitychange', onVisChange);
    return () => document.removeEventListener('visibilitychange', onVisChange);
  }, []);

  // ---- API 动作 ----
  const login = async (username, password) => {
    const res = await fetch(`${API_BASE}/api/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await res.json();
    if (data.error) throw new Error(data.error);
    setUser(data);
    return data;
  };

  const logout = () => {
    wsRef.current?.close();
    stopHeartbeat();
    stopPolling();
    setUser(null);
    setMessages([]);
    setConnStatus('offline');
  };

  const loadHistory = async (roomId = 1) => {
    if (!user) return;
    const res = await fetch(`${API_BASE}/api/history/${roomId}?token=${user.token}&after_id=0&limit=50`);
    const data = await res.json();
    if (data.messages) addMessages(data.messages);
  };

  const loadMoreHistory = useCallback(async (roomId = 1) => {
    if (!user) return;
    const afterId = Math.max(0, minMsgIdRef.current - 51);
    const res = await fetch(`${API_BASE}/api/history/${roomId}?token=${user.token}&after_id=${afterId}&limit=50`);
    const data = await res.json();
    if (data.messages && data.messages.length) addMessages(data.messages);
  }, [user, addMessages]);

  const sendPayload = useCallback(async (payload, roomId = 1) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'message', room_id: roomId, ...payload }));
    } else {
      await fetch(`${API_BASE}/api/msg`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: user.token, room_id: roomId, ...payload })
      });
    }
  }, [user]);

  const sendMessage = useCallback(async (content, roomId = 1) => {
    if (!content.trim()) return;
    await sendPayload({ content, msg_type: 'text' }, roomId);
  }, [sendPayload]);

  const uploadFile = useCallback(async (file) => {
    const resp = await fetch('https://upload.moonchan.xyz/api/upload', {
      method: 'PUT',
      body: file,
    });
    if (!resp.ok) throw new Error(`Upload failed: ${resp.statusText}`);
    const data = await resp.json();
    return {
      url: `https://upload.moonchan.xyz/api/${data.id}/${encodeURIComponent(file.name)}`,
      name: file.name,
      size: file.size,
      mime: file.type,
    };
  }, []);

  const sendFile = useCallback(async (file, roomId = 1) => {
    const fileData = await uploadFile(file);
    await sendPayload({
      content: JSON.stringify(fileData),
      msg_type: 'file'
    }, roomId);
    return fileData;
  }, [uploadFile, sendPayload]);

  // 初始化：登录后加载历史并连接 WS
  useEffect(() => {
    if (user) {
      loadHistory(activeRoom);
      roomRef.current = activeRoom;
      connectWS();
    }
    return () => { wsRef.current?.close(); stopHeartbeat(); stopPolling(); };
  }, [user]);

  // 切换房间：加载新房间历史
  const switchRoom = useCallback((roomId) => {
    setActiveRoom(roomId);
    roomRef.current = roomId;
    loadHistory(roomId);
    // Reconnect WS to new room
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
      setTimeout(() => connectWS(), 300);
    }
  }, [user, connectWS]);

  const createRoom = useCallback(async (name) => {
    if (!user) return;
    const res = await fetch(`${API_BASE}/api/rooms`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: user.token, name })
    });
    const data = await res.json();
    if (data.id) return data;
    throw new Error(data.error || '创建失败');
  }, [user]);

  return { user, messages, connStatus, onlineCount, onlineUsers, activeRoom, switchRoom, login, logout, sendMessage, sendFile, loadMoreHistory, createRoom };
}