// fuzzyMatch.test.js — 模糊匹配评分测试（【本地改动 2026-09-03】Cmd-K 快速切换）
// 发现背景：ChatList 现有搜索是 includes 子串匹配，Cmd-K 需要子序列模糊匹配
// （非连续字符也能命中），单测保护评分正确性与越界索引。

import { describe, it, expect } from 'vitest';
import { fuzzyMatch, combinedScore } from './fuzzyMatch.js';

describe('fuzzyMatch — 子序列模糊匹配', () => {
  it('空 query 返回 null', () => {
    expect(fuzzyMatch('', 'hello')).toBeNull();
  });

  it('完全匹配返回正分', () => {
    const r = fuzzyMatch('hello', 'hello');
    expect(r).not.toBeNull();
    expect(r.score).toBeGreaterThan(0);
    expect(r.matches).toEqual([0, 1, 2, 3, 4]);
  });

  it('子串匹配', () => {
    const r = fuzzyMatch('chat', 'my-chat-room');
    expect(r).not.toBeNull();
    expect(r.matches).toEqual([3, 4, 5, 6]);
  });

  it('非连续子序列匹配', () => {
    // "gch" 命中 "group chat general"
    const r = fuzzyMatch('gch', 'group chat general');
    expect(r).not.toBeNull();
    expect(r.matches[0]).toBe(0);
  });

  it('大小写不敏感', () => {
    const r = fuzzyMatch('CHAT', 'Chat');
    expect(r).not.toBeNull();
  });

  it('字符不按序出现 → null', () => {
    expect(fuzzyMatch('hx', 'hello')).toBeNull();
    expect(fuzzyMatch('abc', '')).toBeNull();
  });

  it('连续匹配得分高于分散匹配', () => {
    const consecutive = fuzzyMatch('abc', 'abcXYZ').score;
    const scattered = fuzzyMatch('abc', 'axbycz').score;
    expect(consecutive).toBeGreaterThan(scattered);
  });

  it('空 target → null', () => {
    expect(fuzzyMatch('a', '')).toBeNull();
  });
});

describe('combinedScore — label 加权', () => {
  it('label 分数权重高于 detail', () => {
    // 相同原始分数时，label 侧（×3）应更高
    const labelHigh = combinedScore(10, 0);
    const detailHigh = combinedScore(0, 10);
    expect(labelHigh).toBe(30);
    expect(detailHigh).toBe(10);
    expect(labelHigh).toBeGreaterThan(detailHigh);
  });

  it('无匹配返回 0', () => {
    expect(combinedScore(0, 0)).toBe(0);
  });
});
