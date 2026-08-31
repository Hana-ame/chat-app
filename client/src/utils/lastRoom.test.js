// lastRoom.test.js — 最近聊天记忆纯函数测试（【本地改动 2026-09-03】FDR-026）
// 发现背景：记忆逻辑是 localStorage 读写，node 环境下无 localStorage → try/catch
// 静默降级必须单测保护，防止未来改成直接抛错导致登录页崩溃。

import { describe, it, expect, beforeEach } from 'vitest';
import { getLastRoom, setLastRoom, clearLastRoom, LAST_ROOM_KEY } from './lastRoom.js';

describe('lastRoom — localStorage 读写', () => {
  beforeEach(() => {
    // node 环境无 localStorage，手动 stub 一个内存版
    const store = new Map();
    globalThis.localStorage = {
      getItem: (k) => store.has(k) ? store.get(k) : null,
      setItem: (k, v) => { store.set(k, String(v)); },
      removeItem: (k) => { store.delete(k); },
      clear: () => { store.clear(); },
    };
  });

  it('默认无记忆返回空字符串', () => {
    expect(getLastRoom()).toBe('');
  });

  it('set 后 get 返回该 id', () => {
    setLastRoom('c123');
    expect(getLastRoom()).toBe('c123');
  });

  it('set 空 id 不写入', () => {
    setLastRoom('');
    setLastRoom(null);
    expect(getLastRoom()).toBe('');
  });

  it('clear 后返回空', () => {
    setLastRoom('c123');
    clearLastRoom();
    expect(getLastRoom()).toBe('');
  });

  it('存储 key 是 chatapp:lastRoom', () => {
    setLastRoom('c456');
    expect(localStorage.getItem(LAST_ROOM_KEY)).toBe('c456');
  });

  it('localStorage 不可用时静默降级（不抛错）', () => {
    // 移除 localStorage 模拟隐私模式
    const orig = globalThis.localStorage;
    globalThis.localStorage = undefined;
    expect(() => setLastRoom('c1')).not.toThrow();
    expect(getLastRoom()).toBe('');
    expect(() => clearLastRoom()).not.toThrow();
    globalThis.localStorage = orig;
  });
});
