import { useState, useEffect, useRef, useCallback } from 'react';

const WS_RECONNECT_DELAYS = [2000, 5000, 10000, 30000, 60000];

export default function useChat() {
  const [user, setUser] = useState(null);
  const [messages, setMessages] = useState([]);
  const [connStatus, setConnStatus] = useState('offline'); // ws | poll | offline
  const [onlineCount, setOnlineCount] = useState(0);
  
  const wsRef = useRef(null);
  const pollRef = useRef(false);
  const lastMsgIdRef = useRef(0);
  const retryIdxRef = useRef(0);
  const heartbeatRef = useRef(null);

  const addMessages = useCallback((msgs) => {
    if (!msgs || msgs.length === 0) return;
    setMessages(prev => {
      const existingIds = new Set(prev.map(m => m.id));
      const newMsgs = msgs.filter(m => !existingIds.has(m.id));
      if (newMsgs.length === 0) return prev;
      const combined = [...prev, ...newMsgs].sort((a, b) => a.id - b.id);
      lastMsgIdRef.current = combined[combined.length - 1].id;
      return combined;
    });
  }, []);

  // 处理服务端消息
  const handleMessage = useCallback((msg) => {
    if (msg.type === 'pong') return;
    if (msg.type === 'online_count') { setOnlineCount(msg.count); return; }
    if (msg.type === 'system') { addMessages([{ ...msg, id: Date.now() }]); return; }
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
          const res = await fetch(`/api/poll?room_id=1&token=${user.token}&after_id=${lastMsgIdRef.current}&timeout=30`);
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
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/1?token=${user.token}`;
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
    const res = await fetch('/api/login', {
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

  const loadHistory = async () => {
    if (!user) return;
    const res = await fetch(`/api/history/1?token=${user.token}&after_id=0&limit=50`);
    const data = await res.json();
    if (data.messages) addMessages(data.messages);
  };

  const sendMessage = async (content) => {
    if (!content.trim()) return;
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'message', content }));
    } else {
      await fetch('/api/msg', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: user.token, room_id: 1, content })
      });
    }
  };

  // 初始化：登录后加载历史并连接 WS
  useEffect(() => {
    if (user) {
      loadHistory();
      connectWS();
    }
    return () => { wsRef.current?.close(); stopHeartbeat(); stopPolling(); };
  }, [user]);

  return { user, messages, connStatus, onlineCount, login, logout, sendMessage };
}