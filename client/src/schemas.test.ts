// schemas.ts 单元测试:zod 校验契约。
//
// 覆盖:
//   - validate() 合法数据通过 / 非法数据抛错
//   - 各 schema 的必填字段与可选字段行为
//   - 与后端 API 实际返回字段的契约对齐(字段缺失时报错)
//
// 运行: cd client && npx vitest run src/schemas.test.ts
import { describe, it, expect } from 'vitest';
import {
  UserSchema, MessageSchema, ChatSchema, AuthResponseSchema, ReactionSchema, validate,
} from './schemas';

describe('schemas', () => {
  it('validate 通过合法数据并返回类型化结果', () => {
    const data = {
      user: { id: 'u1', username: 'alice', avatar_color: '#fff' },
      access_token: 'tok',
      expires_in: 3600,
    };
    const result = validate(AuthResponseSchema, data, 'login');
    expect(result.user.username).toBe('alice');
  });

  it('validate 对非法数据抛错', () => {
    expect(() => validate(AuthResponseSchema, { user: { id: 1 }, access_token: 'x' }, 'login'))
      .toThrow('API response validation failed: login');
  });

  it('MessageSchema 接受完整字段', () => {
    const msg = {
      id: 'm1', chat_id: 'c1', user_id: 'u1', content: 'hi', created_at: '2026-01-01T00:00:00Z',
      type: 'stream', thinking: 'thinking...', stream_url: '/api/chats/c1/messages/m1/stream',
      attachment_count: 1, mention_count: 0, reaction_count: 2,
      edited_at: null, deleted_at: null, deleted: false,
      attachments: [{ id: 'a1', filename: 'f.txt', mime_type: 'text/plain', size: 10, url: '/u/f.txt' }],
      reactions: [{ emoji: '👍', count: 1, user_ids: ['u1'], me: true }],
      mentions: ['u2'],
      reply_to: 'm0',
    };
    const result = MessageSchema.parse(msg);
    expect(result.id).toBe('m1');
    expect(result.attachments).toHaveLength(1);
    expect(result.reactions[0].emoji).toBe('👍');
  });

  it('MessageSchema 拒绝缺必填字段', () => {
    expect(() => MessageSchema.parse({ id: 'm1' })).toThrow();
  });

  it('ChatSchema 接受后端字段(含 deprecated unread_count 与 pinned 指针)', () => {
    const chat = {
      id: 'c1', type: 'group', name: 'Dev', icon_color: '#5865F2',
      created_at: '2026-01-01T00:00:00Z', last_message_at: '2026-01-02T00:00:00Z',
      member_count: 3, unread_count: 5, pinned: true, notify_enabled: true,
      visibility: 'public', owner_id: 'u1',
      pinned_message: { id: 'm9', content: 'announcement', pinned_at: '2026-01-01T00:00:00Z' },
      pinned_updated_at: '2026-01-01T00:00:00Z',
      banner_opacity: 0.5,
      members: [{ id: 'u1', username: 'alice' }],
    };
    const result = ChatSchema.parse(chat);
    expect(result.id).toBe('c1');
    expect(result.pinned_message.content).toBe('announcement');
  });

  it('ReactionSchema 拒绝 count 为字符串(类型安全)', () => {
    expect(() => ReactionSchema.parse({ emoji: '👍', count: '2' })).toThrow();
  });

  it('UserSchema 接受含 notify_blocked 的完整用户', () => {
    const u = {
      id: 'u1', username: 'alice', email: 'a@b.c', status: 'online',
      last_seen: '2026-01-01T00:00:00Z', role: 'admin',
      notify_blocked: ['u2'],
    };
    const result = UserSchema.parse(u);
    expect(result.notify_blocked).toEqual(['u2']);
  });
});
