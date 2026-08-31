// createOpHandlers 单元测试:realtime 事件 → store 动作的分发映射。
//
// 覆盖:
//   - 每个 op 分发到正确的 store 动作(参数透传)
//   - presence_update / user_update / poll:messages 走 set/get 内联逻辑
//   - onReady 设置连接状态 + 载入 chats
//   - 未知 op 静默不抛错
//
// 运行: cd client && npx vitest run src/realtime/opHandlers.test.js
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createOpHandlers } from './opHandlers';

function makeBridge() {
  const set = vi.fn();
  const normalize = vi.fn(m => m);
  const get = vi.fn(() => ({ activeChatId: 'c1', _normalize: normalize }));
  const actions = {
    setChats: vi.fn(),
    onMessageCreate: vi.fn(),
    onMessageUpdate: vi.fn(),
    onMessageDelete: vi.fn(),
    onReaction: vi.fn(),
    onChatUpdate: vi.fn(),
    onChatDelete: vi.fn(),
    onChatRemove: vi.fn(),
    // 【本地改动 2026-09-03】打字指示器分发目标
    onTyping: vi.fn(),
    _normalize: normalize,
  };
  const handlers = createOpHandlers({ set, get, actions: () => actions });
  return { set, get, actions, handlers };
}

describe('createOpHandlers', () => {
  let b;
  beforeEach(() => { b = makeBridge(); });

  it('onReady 设置连接状态并载入 chats', () => {
    b.handlers.onReady({ onlineUserIds: ['u1'], chats: [{ id: 'c1' }] });
    expect(b.set).toHaveBeenCalledWith({ onlineUserIds: ['u1'], wsReady: true, sseReady: true });
    expect(b.actions.setChats).toHaveBeenCalledWith([{ id: 'c1' }]);
  });

  it('onReady 空 chats 兜底为空数组', () => {
    b.handlers.onReady({ onlineUserIds: [], chats: undefined });
    expect(b.actions.setChats).toHaveBeenCalledWith([]);
  });

  it('消息类 op 分发到对应动作', () => {
    const msg = { id: 'm1', content: 'hi' };
    b.handlers.onEvent('message_create', msg);
    expect(b.actions.onMessageCreate).toHaveBeenCalledWith(msg);

    b.handlers.onEvent('message_update', msg);
    expect(b.actions.onMessageUpdate).toHaveBeenCalledWith(msg);

    const del = { chat_id: 'c1', message_id: 'm1' };
    b.handlers.onEvent('message_delete', del);
    expect(b.actions.onMessageDelete).toHaveBeenCalledWith(del);
  });

  it('reaction op 带 added 标志', () => {
    const p = { chat_id: 'c1', message_id: 'm1', emoji: '👍', user_id: 'u1' };
    b.handlers.onEvent('reaction_add', p);
    expect(b.actions.onReaction).toHaveBeenCalledWith(p, true);
    b.handlers.onEvent('reaction_remove', p);
    expect(b.actions.onReaction).toHaveBeenCalledWith(p, false);
  });

  it('chat op 分发到对应动作', () => {
    const c = { id: 'c1' };
    b.handlers.onEvent('chat_create', c);
    expect(b.actions.onChatUpdate).toHaveBeenCalledWith(c);
    b.handlers.onEvent('chat_update', c);
    expect(b.actions.onChatUpdate).toHaveBeenCalledWith(c);
    b.handlers.onEvent('chat_delete', c);
    expect(b.actions.onChatDelete).toHaveBeenCalledWith(c);
    b.handlers.onEvent('chat_remove', c);
    expect(b.actions.onChatRemove).toHaveBeenCalledWith(c);
  });

  it('presence_update 维护在线集合', () => {
    const setFn = vi.fn(fn => fn({ onlineUserIds: ['u1'] }));
    b.set.mockImplementation(setFn);

    b.handlers.onEvent('presence_update', { user_id: 'u2', status: 'online' });
    const r1 = setFn.mock.calls[0][0]({ onlineUserIds: ['u1'] });
    expect(r1).toEqual({ onlineUserIds: ['u1', 'u2'] });

    b.handlers.onEvent('presence_update', { user_id: 'u1', status: 'offline' });
    const r2 = setFn.mock.calls[1][0]({ onlineUserIds: ['u1', 'u2'] });
    expect(r2).toEqual({ onlineUserIds: ['u2'] });
  });

  it('poll:messages 归一化后写入', () => {
    b.handlers.onEvent('poll:messages', [{ id: 'm1' }]);
    expect(b.set).toHaveBeenCalledWith({ messages: [{ id: 'm1' }] });
  });

  it('onClose 清除连接状态', () => {
    b.handlers.onClose();
    expect(b.set).toHaveBeenCalledWith({ wsReady: false, sseReady: false });
  });

  it('getActiveChatId 返回当前聊天', () => {
    expect(b.handlers.getActiveChatId()).toBe('c1');
  });

  it('typing 事件转发到 onTyping', () => {
    b.handlers.onEvent('typing', { chat_id: 'c1', user_id: 'u2' });
    expect(b.actions.onTyping).toHaveBeenCalledWith('c1', 'u2');
  });

  it('未知 op 静默不抛错', () => {
    expect(() => b.handlers.onEvent('typing', {})).not.toThrow();
    expect(() => b.handlers.onEvent('bogus_op', {})).not.toThrow();
  });
});
