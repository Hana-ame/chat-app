// unreadBoundary.js — 未读边界计算（【本地改动 2026-09-03】未读分隔线）
//
// 计算「未读消息」分隔线应插入的位置：第一条 created_at > last_active_at
// 且未删除的消息索引。语义与后端 UnreadCount（created_at > last_active_at）
// 完全一致。纯函数可单测。

/**
 * @param {Array<{created_at?:string, deleted?:boolean}>} messages 时间升序的消息
 * @param {string|undefined|null} unreadSince 用户最后活跃时间（chat.last_active_at）
 * @returns {number} 未读边界索引；无未读 / 无依据时返回 -1
 */
export function computeUnreadIndex(messages, unreadSince) {
  if (!unreadSince || !Array.isArray(messages)) return -1;
  const since = new Date(unreadSince).getTime();
  if (Number.isNaN(since)) return -1;
  for (let i = 0; i < messages.length; i++) {
    const t = new Date(messages[i].created_at).getTime();
    if (t > since && !messages[i].deleted) return i;
  }
  return -1;
}
