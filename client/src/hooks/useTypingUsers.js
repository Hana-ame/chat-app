// useTypingUsers.js — 打字指示器 hook（【本地改动 2026-09-03】对齐 chatto FDR-010）
//
// 订阅 store.typingByChat[chatId]，返回"正在输入"的成员名列表。
// 过期条目用 store.pruneTyping 清理（每 1s 由本 hook 的 interval 触发），
// 避免过期条目永久残留导致"一直显示在输入"。composer 每 2s 发心跳，
// 3s TTL 由 store.onTyping 写入，这里只负责展示与清理。

import { useEffect, useMemo, useState } from 'react';
import { useChatStore } from '../store/chat';
import { useAuthStore } from '../store/auth';

export function useTypingUsers(chatId) {
  // 【本地改动 2026-09-03】防御性过滤：服务端 sendToChat 已排除发送者本人，
  // 但 SSE/poll 兜底路径可能回显自己的 typing → 显示时再滤掉当前用户。
  const me = useAuthStore(s => s.user?.id);
  const [now, setNow] = useState(() => Date.now());

  // 每秒刷新，驱动过期判定 + 清理
  useEffect(() => {
    const t = setInterval(() => {
      setNow(Date.now());
      useChatStore.getState().pruneTyping();
    }, 1000);
    return () => clearInterval(t);
  }, []);

  const typingMap = useChatStore(s => (chatId ? s.typingByChat[chatId] : undefined));

  const userIds = useMemo(() => {
    if (!typingMap) return [];
    const out = [];
    for (const [uid, exp] of Object.entries(typingMap)) {
      if (exp > now && uid !== me) out.push(uid);
    }
    return out;
  }, [typingMap, now, me]);

  return userIds;
}
