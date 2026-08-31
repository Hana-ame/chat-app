// unreadBoundary.test.js — 未读边界计算测试（【本地改动 2026-09-03】）
// 发现背景：未读分隔线插入位置依赖「created_at > last_active_at」语义，
// 与后端 UnreadCount 一致；边界（严格大于、删除消息跳过、无依据）易错，
// 纯函数单测保护。

import { describe, it, expect } from 'vitest';
import { computeUnreadIndex } from './unreadBoundary.js';

const S = '2026-09-03T12:00:00Z';
function msg(id, created, overrides = {}) {
  return { id, created_at: created, ...overrides };
}

describe('computeUnreadIndex — 未读边界', () => {
  it('无 last_active_at 返回 -1', () => {
    expect(computeUnreadIndex([msg('a', S)], undefined)).toBe(-1);
    expect(computeUnreadIndex([msg('a', S)], null)).toBe(-1);
  });

  it('无消息返回 -1', () => {
    expect(computeUnreadIndex([], S)).toBe(-1);
  });

  it('返回第一条严格晚于 last_active_at 的消息', () => {
    const msgs = [
      msg('a', '2026-09-03T10:00:00Z'),
      msg('b', '2026-09-03T11:00:00Z'),
      msg('c', '2026-09-03T13:00:00Z'), // 第一条未读
      msg('d', '2026-09-03T14:00:00Z'),
    ];
    expect(computeUnreadIndex(msgs, S)).toBe(2);
  });

  it('全部已读返回 -1', () => {
    const msgs = [msg('a', '2026-09-03T10:00:00Z'), msg('b', '2026-09-03T11:00:00Z')];
    expect(computeUnreadIndex(msgs, S)).toBe(-1);
  });

  it('边界严格大于（等于不算未读）', () => {
    const msgs = [msg('a', '2026-09-03T12:00:00Z')]; // 恰等于 last_active_at
    expect(computeUnreadIndex(msgs, S)).toBe(-1);
  });

  it('跳过已删除消息', () => {
    const msgs = [
      msg('a', '2026-09-03T13:00:00Z', { deleted: true }), // 已删除，不算未读起点
      msg('b', '2026-09-03T14:00:00Z'),                     // 未读起点
    ];
    expect(computeUnreadIndex(msgs, S)).toBe(1);
  });

  it('非法 last_active_at 返回 -1', () => {
    expect(computeUnreadIndex([msg('a', S)], 'not-a-date')).toBe(-1);
  });
});
