// mentionRank.js — @提及自动补全排序（【本地改动 2026-09-03】）
//
// 背景：chatto 的提及候选按最近互动排序，让"最近聊过的人"排前面，比纯字母序
// 更好用。chat-app 原本只是 includes 过滤 + 字母序截断。本模块把排序逻辑抽成
// 纯函数：按「最近 n 条消息里每位成员的出现次数/新鲜度」打分，高者优先，
// 分数相同按用户名字母序（稳定兜底）。
//
// 只读 messages，不改状态。权重：越新的消息权重越高（指数衰减），避免一条
// 很久以前的刷屏消息长期霸占候选。

/** 每个候选成员的排序分：freshness 权重 + 出现次数。 */
export function mentionScore(memberId, messages, nowMs) {
  let score = 0;
  if (!messages || !Array.isArray(messages)) return 0;
  for (let i = messages.length - 1; i >= 0; i--) {
    const m = messages[i];
    if (!m || m.user_id !== memberId || m.deleted || m.type === 'stream') continue;
    const t = m.created_at ? new Date(m.created_at).getTime() : 0;
    if (!t) continue;
    const ageMs = (nowMs || Date.now()) - t;
    // 指数衰减：1 小时内权重最高，1 天前几乎为 0
    const freshness = Math.exp(-ageMs / (24 * 3600 * 1000));
    score += 0.5 + freshness;
  }
  return score;
}

/**
 * 对候选成员排序：按 mentionScore 降序，分数相同按用户名字母序。
 * 返回原数组的排序副本（不修改入参）。
 * @param {Array<{id:string, username:string}>} members
 * @param {Array<any>} messages
 * @param {number} [nowMs]
 */
export function sortMentionCandidates(members, messages, nowMs) {
  if (!members || members.length === 0) return [];
  const now = nowMs || Date.now();
  return [...members].sort((a, b) => {
    const diff = mentionScore(b.id, messages, now) - mentionScore(a.id, messages, now);
    if (diff !== 0) return diff;
    return (a.username || '').localeCompare(b.username || '');
  });
}
