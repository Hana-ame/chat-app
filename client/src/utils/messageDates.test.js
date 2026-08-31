// messageDates.test.js — 日期分隔工具测试（【本地改动 2026-09-03】日期分组）
// 发现背景：dateKey/formatDateDivider 决定消息列表何时插入 Today/Yesterday/日期分隔线，
// 跨天边界与本地时区都易错，纯函数单测保护。
// 踩坑：CI 与本地时区可能非 UTC，用 UTC 字符串断言会因时区漂移失败 —— 本测试
// 全部用「本地时间 Date 组件」构造（new Date(y,m,d)），与时区无关。

import { describe, it, expect } from 'vitest';
import { dateKey, formatDateDivider } from './messageDates.js';

describe('dateKey — 本地日粒度 key', () => {
  it('返回 yyyy-MM-dd', () => {
    // 本地时间 2026-09-01 10:00
    expect(dateKey(new Date(2026, 8, 1, 10, 0, 0).toISOString())).toBe('2026-09-01');
  });

  it('同一天不同时刻 key 相同', () => {
    const a = new Date(2026, 8, 1, 0, 0, 0).toISOString();
    const b = new Date(2026, 8, 1, 23, 59, 59).toISOString();
    expect(dateKey(a)).toBe(dateKey(b));
  });

  it('跨天 key 不同', () => {
    const a = new Date(2026, 8, 1, 23, 59, 59).toISOString();
    const b = new Date(2026, 8, 2, 0, 0, 0).toISOString();
    expect(dateKey(a)).not.toBe(dateKey(b));
  });

  it('空 / 非法输入返回空串', () => {
    expect(dateKey('')).toBe('');
    expect(dateKey(null)).toBe('');
    expect(dateKey('not-a-date')).toBe('');
  });
});

describe('formatDateDivider — 分隔标签', () => {
  // 用本地时间构造 now，保证相对日判断与时区无关
  const NOW = new Date(2026, 8, 3, 12, 0, 0); // 本地 2026-09-03

  it('今天 → Today', () => {
    expect(formatDateDivider(new Date(2026, 8, 3, 8, 0, 0).toISOString(), NOW)).toBe('Today');
  });

  it('昨天 → Yesterday', () => {
    expect(formatDateDivider(new Date(2026, 8, 2, 20, 0, 0).toISOString(), NOW)).toBe('Yesterday');
  });

  it('更早 → 非 Today/Yesterday 的日期', () => {
    const out = formatDateDivider(new Date(2026, 7, 15, 0, 0, 0).toISOString(), NOW);
    expect(out).not.toBe('Today');
    expect(out).not.toBe('Yesterday');
    expect(out).toMatch(/\d{1,4}/);
  });

  it('空输入返回空串', () => {
    expect(formatDateDivider('', NOW)).toBe('');
    expect(formatDateDivider(null, NOW)).toBe('');
  });
});
