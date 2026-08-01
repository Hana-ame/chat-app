// useChatStore 单元测试:聊天状态管理核心逻辑。
//
// 覆盖:
//   - setChats: 排序(置顶优先、last_message_at 降序)、last_message 合并
//   - onMessageCreate: 乐观消息替换、新消息追加、last_message/未读数更新、
//     stream 消息占位、跨聊天通知
//   - onMessageUpdate / onMessageDelete / onReaction(增删与去重)
//   - onChatUpdate(合并+排序)/ onChatDelete(清理 activeChat)
//   - _mergeMessages 按 id 去重合并、_normalize 删除态
//   - sendMessage: 乐观追加 → 成功替换 → 失败回滚
//
// Mock 说明(见 vitest.config.js,environment=node):
//   - ../api/client 与 ../realtime/coordinator 全部替换为 vi.fn
//   - ./auth 换成假 store(localStorage/DOM 不可用)
//   - fetchStream/notifyMessage 换成 vi.fn 以隔离副作用
//
// 运行: cd client && npx vitest run src/store/chat.test.js
import { describe, it, expect, vi, beforeEach } from 'vitest';

// vi.hoisted:vi.mock 工厂被提升到模块顶部执行,外部 const 不可引用,
// 需要在 hoisted 里创建共享状态(coordHandlers 用于注入事件分发)。
const { coordHandlers, fakeCoord } = vi.hoisted(() => {
  const coordHandlers = {};
  const fakeCoord = {
    token: 'tok',
    connect: vi.fn(),
    disconnect: vi.fn(),
    sendTyping: vi.fn(),
    subscribe: vi.fn(),
    wsRequest: vi.fn(),
    setHandlers: vi.fn(h => Object.assign(coordHandlers, h)),
  };
  return { coordHandlers, fakeCoord };
});
vi.mock('../api/client', () => ({
  api: {
    listChats: vi.fn(),
    listMessages: vi.fn(),
    sendMessage: vi.fn(),
    setNotifyEnabled: vi.fn(),
    setAnnouncement: vi.fn(),
    clearAnnouncement: vi.fn(),
    markAnnouncementRead: vi.fn(),
  },
}));
vi.mock('../realtime/coordinator', () => ({ getCoordinator: () => fakeCoord }));
vi.mock('./auth', () => ({
  useAuthStore: {
    getState: () => ({ user: { id: 'u1', username: 'alice' }, accessToken: 'tok' }),
  },
}));
vi.mock('../realtime/fetchStream', () => ({ fetchStream: vi.fn() }));
vi.mock('../utils/notifyMessage', () => ({ maybeNotifyMessage: vi.fn() }));

import { useChatStore } from './chat';
import { api } from '../api/client';
import { maybeNotifyMessage } from '../utils/notifyMessage';
import { fetchStream } from '../realtime/fetchStream';

/** 构造一条最小消息。 */
function msg(id, chatId, overrides = {}) {
  return {
    id, chat_id: chatId, user_id: 'u1', content: 'hello', created_at: '2026-01-01T00:00:00Z',
    reactions: [], ...overrides,
  };
}

function chat(id, overrides = {}) {
  return { id, type: 'group', name: 'Chat ' + id, created_at: '2026-01-01T00:00:00Z', ...overrides };
}

beforeEach(() => {
  useChatStore.getState().reset();
  vi.clearAllMocks();
});

describe('setChats', () => {
  it('按 last_message_at 降序排列', () => {
    const s = useChatStore.getState();
    s.setChats([
      chat('c1', { last_message_at: '2026-01-01T10:00:00Z' }),
      chat('c2', { last_message_at: '2026-01-01T12:00:00Z' }),
    ]);
    expect(useChatStore.getState().chats.map(c => c.id)).toEqual(['c2', 'c1']);
  });

  it('置顶聊天排在前面', () => {
    const s = useChatStore.getState();
    s.setChats([
      chat('c1', { pinned: false, last_message_at: '2026-01-01T12:00:00Z' }),
      chat('c2', { pinned: true, last_message_at: '2026-01-01T10:00:00Z' }),
    ]);
    expect(useChatStore.getState().chats.map(c => c.id)).toEqual(['c2', 'c1']);
  });

  it('无 last_message_at 时回退 created_at', () => {
    const s = useChatStore.getState();
    s.setChats([
      chat('c1', { created_at: '2026-01-01T10:00:00Z' }),
      chat('c2', { created_at: '2026-01-01T11:00:00Z' }),
    ]);
    expect(useChatStore.getState().chats.map(c => c.id)).toEqual(['c2', 'c1']);
  });

  it('新数据保留旧 members(服务端列表不含 members 时不清空)', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1', { members: [{ id: 'u9' }] })]);
    s.setChats([chat('c1')]);
    expect(useChatStore.getState().chats[0].members).toEqual([{ id: 'u9' }]);
  });
});

describe('onMessageCreate', () => {
  it('新消息追加到 activeChat 并更新 last_message', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));

    const st = useChatStore.getState();
    expect(st.messages.map(m => m.id)).toEqual(['m1']);
    expect(st.chats[0].last_message.id).toBe('m1');
    expect(st.chats[0].last_message_at).toBe('2026-01-01T00:00:00Z');
  });

  it('非 activeChat 的新消息累加未读数', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1', { unread_count: 0 }), chat('c2')]);
    s.setActiveChatId('c2');
    s.onMessageCreate(msg('m1', 'c1', { created_at: '2026-01-02T00:00:00Z' }));

    const c1 = useChatStore.getState().chats.find(c => c.id === 'c1');
    expect(c1.unread_count).toBe(1);
  });

  it('activeChat 内的消息不累加未读数', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1', { unread_count: 3 })]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    expect(useChatStore.getState().chats[0].unread_count).toBe(0);
  });

  it('乐观消息(optimisticIds)到达时原位替换并清除标记', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');

    // 模拟 sendMessage 的乐观消息已经在列表里,随后服务端 confirm 事件到达。
    const optimisticId = 'opt-1';
    useChatStore.setState(st => ({
      messages: [...st.messages, { ...msg(optimisticId, 'c1'), optimistic: true }],
    }));
    s._optimisticIds.add(optimisticId);
    s.onMessageCreate(msg(optimisticId, 'c1'));

    const m = useChatStore.getState().messages[0];
    expect(m.optimistic).toBe(false);
    expect(m.streaming).toBe(false);
    expect(useChatStore.getState()._optimisticIds.has(optimisticId)).toBe(false);

    s.onMessageCreate(msg('m2', 'c1'));
    expect(useChatStore.getState().messages.length).toBe(2);
  });

  it('stream 消息带 stream_url 时置空 content 并开始 fetchStream', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    // user_id 必须是别人(u1 是当前用户,自己的流不拉取)。
    const streamMsg = msg('m-s', 'c1', {
      type: 'stream', stream_url: '/api/chats/c1/messages/m-s/stream', user_id: 'u2',
    });
    s.onMessageCreate(streamMsg);

    const st = useChatStore.getState();
    expect(st.messages[0].content).toBe('');
    expect(st.messages[0].streaming).toBe(true);
    expect(fetchStream).toHaveBeenCalledWith('/api/chats/c1/messages/m-s/stream', 'm-s', expect.any(Function));
  });

  it('自己的 stream 消息不触发 fetchStream(本地已在渲染)', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m-s', 'c1', {
      type: 'stream', stream_url: '/api/chats/c1/messages/m-s/stream', user_id: 'u1',
    }));
    expect(fetchStream).not.toHaveBeenCalled();
  });

  it('非 activeChat 的新消息触发通知回调', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c2');
    s.onMessageCreate(msg('m1', 'c1'));
    expect(maybeNotifyMessage).toHaveBeenCalled();
  });
});

describe('onMessageUpdate / onMessageDelete', () => {
  it('message_update 合并新字段', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onMessageUpdate(msg('m1', 'c1', { content: 'edited', edited_at: '2026-01-02T00:00:00Z' }));
    expect(useChatStore.getState().messages[0].content).toBe('edited');
    expect(useChatStore.getState().messages[0].edited_at).toBe('2026-01-02T00:00:00Z');
  });

  it('message_delete 标记删除并清空内容', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onMessageDelete({ message_id: 'm1' });
    const m = useChatStore.getState().messages[0];
    expect(m.deleted).toBe(true);
    expect(m.content).toBe('');
  });
});

describe('onReaction', () => {
  it('新增 reaction 计数与 user_ids 追加', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onReaction({ message_id: 'm1', emoji: '👍', user_id: 'u1' }, true);
    expect(useChatStore.getState().messages[0].reactions).toEqual([
      { emoji: '👍', count: 1, user_ids: ['u1'], me: true },
    ]);
  });

  it('同一用户重复添加同一 emoji 时去重', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onReaction({ message_id: 'm1', emoji: '👍', user_id: 'u1' }, true);
    s.onReaction({ message_id: 'm1', emoji: '👍', user_id: 'u1' }, true);
    const r = useChatStore.getState().messages[0].reactions[0];
    expect(r.count).toBe(1);
    expect(r.user_ids).toEqual(['u1']);
  });

  it('移除 reaction 时计数递减、计数归零后过滤', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onReaction({ message_id: 'm1', emoji: '👍', user_id: 'u1' }, true);
    s.onReaction({ message_id: 'm1', emoji: '👍', user_id: 'u1' }, false);
    expect(useChatStore.getState().messages[0].reactions).toEqual([]);
  });
});

describe('onChatUpdate / onChatDelete', () => {
  it('chat_update 合并已有聊天并重新排序', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1', { last_message_at: '2026-01-01T10:00:00Z' }), chat('c2', { last_message_at: '2026-01-01T09:00:00Z' })]);
    s.onChatUpdate(chat('c2', { last_message_at: '2026-01-02T00:00:00Z' }));
    expect(useChatStore.getState().chats.map(c => c.id)).toEqual(['c2', 'c1']);
  });

  it('chat_update 对未知聊天直接追加', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.onChatUpdate(chat('c9'));
    expect(useChatStore.getState().chats.map(c => c.id)).toContain('c9');
  });

  it('chat_delete 移除聊天;activeChat 被删时清空消息', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    s.onMessageCreate(msg('m1', 'c1'));
    s.onChatDelete({ chat_id: 'c1' });
    const st = useChatStore.getState();
    expect(st.chats).toEqual([]);
    expect(st.messages).toEqual([]);
    expect(st.activeChatId).toBeNull();
  });
});

describe('_mergeMessages / _normalize', () => {
  it('按 id 去重合并(后者覆盖前者字段)', () => {
    const s = useChatStore.getState();
    const a = [msg('m1', 'c1', { content: 'old' })];
    const b = [msg('m1', 'c1', { content: 'new' }), msg('m2', 'c1')];
    const merged = s._mergeMessages(a, b);
    expect(merged).toHaveLength(2);
    expect(merged.find(m => m.id === 'm1').content).toBe('new');
  });

  it('_normalize 对 deleted_at 消息置 deleted 并清空内容', () => {
    const s = useChatStore.getState();
    const n = s._normalize(msg('m1', 'c1', { deleted_at: '2026-01-02T00:00:00Z' }));
    expect(n.deleted).toBe(true);
    expect(n.content).toBe('');
  });
});

describe('sendMessage', () => {
  it('乐观追加 → 成功后替换为服务端消息', async () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    api.sendMessage.mockResolvedValue(msg('server-1', 'c1'));

    const promise = s.sendMessage('tok', 'c1', 'hi');
    const optimistic = useChatStore.getState().messages[0];
    expect(optimistic.optimistic).toBe(true);
    expect(optimistic.content).toBe('hi');

    await promise;
    expect(useChatStore.getState().messages[0].id).toBe('server-1');
    expect(useChatStore.getState().messages[0].optimistic).toBeUndefined();
  });

  it('失败时回滚乐观消息并抛出错误', async () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c1');
    api.sendMessage.mockRejectedValue(new Error('network'));

    await expect(s.sendMessage('tok', 'c1', 'hi')).rejects.toThrow('network');
    expect(useChatStore.getState().messages).toEqual([]);
  });

  it('非 activeChat 时不在 messages 里插乐观消息', async () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1')]);
    s.setActiveChatId('c2');
    api.sendMessage.mockResolvedValue(msg('server-1', 'c1'));

    await s.sendMessage('tok', 'c1', 'hi');
    expect(useChatStore.getState().messages).toEqual([]);
  });
});

describe('coordinator 事件分发', () => {
  it('presence_update 更新 onlineUserIds', () => {
    coordHandlers.onEvent('presence_update', { user_id: 'u7', status: 'online' });
    expect(useChatStore.getState().onlineUserIds).toContain('u7');
    coordHandlers.onEvent('presence_update', { user_id: 'u7', status: 'offline' });
    expect(useChatStore.getState().onlineUserIds).not.toContain('u7');
  });

  it('user_update 更新聊天成员信息', () => {
    const s = useChatStore.getState();
    s.setChats([chat('c1', { members: [{ id: 'u7', username: 'old' }] })]);
    coordHandlers.onEvent('user_update', { id: 'u7', username: 'new' });
    expect(useChatStore.getState().chats[0].members[0].username).toBe('new');
  });
});
