// messageDates.js — 消息日期分隔工具（【本地改动 2026-09-03】日期分组）
//
// 背景：翻历史时没有日期分隔线，跨天连续阅读体验差。聊天标准做法是在
// 日期变化处插入「Today / Yesterday / 具体日期」的居中分隔。
// 纯函数可单测；日期用本地时区的「日」粒度比较（同一本地日历日算一组）。

/** 返回本地时区当日的 key（yyyy-MM-dd），用于判断相邻消息是否同一天。 */
export function dateKey(ts) {
  if (!ts) return '';
  const d = new Date(ts);
  if (Number.isNaN(+d)) return '';
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

// 返回「相对日」：0=今天，-1=昨天，其余返回 null（用完整日期）。
function relativeDay(ts, now) {
  const t = new Date(ts);
  const n = now || new Date();
  const startToday = new Date(n.getFullYear(), n.getMonth(), n.getDate());
  const startOfDay = new Date(t.getFullYear(), t.getMonth(), t.getDate());
  return Math.round((startOfDay.getTime() - startToday.getTime()) / 86400000);
}

/**
 * 把时间戳格式化为分隔标签：
 *   - 今天 → "Today"
 *   - 昨天 → "Yesterday"
 *   - 更早 → 本地化日期（如 "2026-09-01" 或浏览器 locale 格式）
 * now 参数便于测试注入。
 */
export function formatDateDivider(ts, now) {
  if (!ts) return '';
  const d = new Date(ts);
  if (Number.isNaN(+d)) return '';
  const rd = relativeDay(ts, now);
  if (rd === 0) return 'Today';
  if (rd === -1) return 'Yesterday';
  return d.toLocaleDateString();
}
