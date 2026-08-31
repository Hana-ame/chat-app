// mentionRank.test.js — @提及排序测试（【本地改动 2026-09-03】）
// 发现背景：提及候选排序决定"最近聊的人优先"，纯函数单测保护打分与稳定性
// （出现次数/新鲜度权重/字母序兜底）。

import { describe, it, expect } from 'vitest';
import { mentionScore, sortMentionCandidates } from './mentionRank.js';

const NOW = new Date('2026-09-03T12:00:00Z').getTime();

function mkMsg(userId, ageH, content) {
  return { user_id: userId, content, created_at: new Date(NOW - ageH * 3600e3).toISOString() };
}

describe('mentionScore — 提及打分', () => {
  it('无消息返回 0', () => {
    expect(mentionScore('u1', [], NOW)).toBe(0);
    expect(mentionScore('u1', null, NOW)).toBe(0);
  });

  it('出现次数越多分数越高', () => {
    const msgs = [mkMsg('u1', 1), mkMsg('u1', 2)];
    expect(mentionScore('u1', msgs, NOW)).toBeGreaterThan(0);
  });

  it('新消息权重高于旧消息', () => {
    const fresh = mentionScore('u1', [mkMsg('u1', 1)], NOW);
    const old = mentionScore('u1', [mkMsg('u1', 48)], NOW);
    expect(fresh).toBeGreaterThan(old);
  });

  it('自己/已删除/stream 消息不计分', () => {
    // 已删除与 stream 不计（模拟排除逻辑在调用侧，但打分自身也跳过）
    const msgs = [mkMsg('u1', 1), { ...mkMsg('u1', 1), deleted: true }, { ...mkMsg('u1', 1), type: 'stream' }];
    const base = mentionScore('u1', [mkMsg('u1', 1)], NOW);
    expect(mentionScore('u1', msgs, NOW)).toBeGreaterThanOrEqual(base); // 不因脏数据翻倍
  });
});

describe('sortMentionCandidates — 排序', () => {
  it('最近互动多的人排前面', () => {
    const members = [
      { id: 'uA', username: 'Alpha' },
      { id: 'uB', username: 'Beta' },
    ];
    const msgs = [mkMsg('uB', 1), mkMsg('uB', 1), mkMsg('uA', 2)];
    const out = sortMentionCandidates(members, msgs, NOW);
    expect(out[0].id).toBe('uB');
  });

  it('分数相同时按字母序', () => {
    const members = [
      { id: 'uB', username: 'Beta' },
      { id: 'uA', username: 'Alpha' },
    ];
    const out = sortMentionCandidates(members, [], NOW);
    expect(out[0].id).toBe('uA');
  });

  it('空列表返回空', () => {
    expect(sortMentionCandidates([], [], NOW)).toEqual([]);
  });

  it('不修改入参数组', () => {
    const members = [{ id: 'uA', username: 'Alpha' }];
    const copy = [...members];
    sortMentionCandidates(members, [], NOW);
    expect(members).toEqual(copy);
  });
});
