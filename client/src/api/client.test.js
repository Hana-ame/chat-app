// api/client.ts 单元测试:HTTP 请求层。
//
// 覆盖:
//   - 401 → refresh → 重试成功链路
//   - 并发 401 共享一次 refresh(refreshPromise 防重入)
//   - refresh 失败 → dispatch auth:unauthorized
//   - 429 抛 too_many_requests
//   - 非 2xx 抛 { status, ...data }
//   - upload:url 字段优先,UPLOAD_BASE + path 兜底
//   - login 响应通过 zod 校验
//
// Mock 说明:全局 fetch 用 vi.stubGlobal 替换;../store/auth 换成假 store
// (auth.js 顶层访问 localStorage,DOM 环境不可用);window/localStorage 打桩。
//
// 运行: cd client && npx vitest run src/api/client.test.js
import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('../store/auth', () => ({
  useAuthStore: {
    getState: () => ({ user: null, accessToken: 'tok' }),
    setState: vi.fn(),
  },
}));

import { UPLOAD_BASE } from '../config';
import { api } from './client';

/** 构造一个 fetch 可返回的 Response 形状(json 方法返回给定数据)。 */
function jsonRes(status, data) {
  return { ok: status >= 200 && status < 300, status, json: async () => data };
}

function stubFetch(...responses) {
  const calls = [];
  const fn = vi.fn(async (url, opts) => {
    calls.push({ url, opts });
    const r = responses.shift();
    if (!r) throw new Error('unexpected fetch call: ' + url);
    return r;
  });
  vi.stubGlobal('fetch', fn);
  return { fn, calls };
}

beforeEach(() => {
  vi.unstubAllGlobals();
  // refreshToken 读写 localStorage;dispatchEvent 用于 auth:unauthorized。
  const storage = {};
  vi.stubGlobal('localStorage', {
    getItem: k => storage[k] ?? null,
    setItem: (k, v) => { storage[k] = String(v); },
    removeItem: k => { delete storage[k]; },
  });
  vi.stubGlobal('window', { dispatchEvent: vi.fn() });
});

describe('request 401 → refresh 重试', () => {
  it('refresh 成功后重试原请求并返回结果', async () => {
    const { calls } = stubFetch(
      jsonRes(401, { error: 'token_expired' }),
      jsonRes(200, { access_token: 'new-token', user: { id: 'u1', username: 'a' } }),
      jsonRes(200, { chats: [{ id: 'c1' }] }),
    );
    const data = await api.listChats('old-token');
    expect(data.chats).toHaveLength(1);
    // 三次 fetch:原请求、refresh、重试。
    expect(calls.map(c => c.url)).toEqual([
      '/api/chats/my', '/api/auth/refresh', '/api/chats/my',
    ]);
  });

  it('并发 401 只触发一次 refresh', async () => {
    const { calls } = stubFetch(
      jsonRes(401, {}), jsonRes(401, {}),
      jsonRes(200, { access_token: 'new' }),
      jsonRes(200, { chats: [] }), jsonRes(200, { chats: [] }),
    );
    await Promise.all([api.listChats('t'), api.listChats('t')]);
    const refreshCalls = calls.filter(c => c.url === '/api/auth/refresh');
    expect(refreshCalls).toHaveLength(1);
  });

  it('refresh 失败时派发 auth:unauthorized 并抛错', async () => {
    stubFetch(
      jsonRes(401, { error: 'token_expired' }),
      jsonRes(401, { error: 'refresh_failed' }),
    );
    await expect(api.listChats('t')).rejects.toMatchObject({ status: 401 });
    expect(window.dispatchEvent).toHaveBeenCalled();
    const [evt] = window.dispatchEvent.mock.calls[0];
    expect(evt.type).toBe('auth:unauthorized');
  });

  it('refresh 端点本身不触发重试循环', async () => {
    const { calls } = stubFetch(
      jsonRes(401, { error: 'bad_refresh' }),
    );
    // 直接调用 refresh()(不走 request),一次 fetch 即可。
    const data = await api.refresh().catch(() => null);
    expect(data).toBeNull();
    expect(calls).toHaveLength(1);
  });
});

describe('request 错误处理', () => {
  it('429 抛 too_many_requests', async () => {
    stubFetch(jsonRes(429, {}));
    await expect(api.listChats('t')).rejects.toMatchObject({
      status: 429, error: 'too_many_requests',
    });
  });

  it('非 2xx 抛带后端错误码的 ApiError', async () => {
    stubFetch(jsonRes(403, { error: 'forbidden', message: 'no' }));
    await expect(api.listChats('t')).rejects.toMatchObject({ status: 403, error: 'forbidden' });
  });
});

describe('upload', () => {
  it('优先使用响应中的 url 字段(绝对 URL)', async () => {
    stubFetch(jsonRes(200, { id: 'h1', path: '/2026/a.png', url: 'https://files.example.com/api/local/2026/a.png' }));
    const file = new File(['x'], 'a.png', { type: 'image/png' });
    const out = await api.upload(file, 'tok');
    expect(out.url).toBe('https://files.example.com/api/local/2026/a.png');
    expect(out.filename).toBe('a.png');
    expect(out.mime_type).toBe('image/png');
  });

  it('公开稳定 URL（/assets/files/{uuid}，【本地改动 2026-09-02】fork 附件模式）', async () => {
    stubFetch(jsonRes(200, {
      id: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890',
      filename: 'photo.jpg',
      mime_type: 'image/jpeg',
      size: 12345,
      url: 'https://files.example.com/assets/files/a1b2c3d4-e5f6-7890-abcd-ef1234567890/photo.jpg',
    }));
    const file = new File(['x'], 'photo.jpg', { type: 'image/jpeg' });
    const out = await api.upload(file, 'tok');
    expect(out.url).toBe('https://files.example.com/assets/files/a1b2c3d4-e5f6-7890-abcd-ef1234567890/photo.jpg');
    expect(out.filename).toBe('photo.jpg');
    expect(out.mime_type).toBe('image/jpeg');
  });

  it('无 url 字段时回退 UPLOAD_BASE + path(去除前导斜杠,不产生双斜杠)', async () => {
    stubFetch(jsonRes(200, { id: 'h1', path: '/2026/a.bin' }));
    const out = await api.upload(new File(['x'], 'a.bin', { type: 'application/octet-stream' }));
    // UPLOAD_BASE 来自运行时环境(CI 无 .env 时为空,本地/生产为绝对前缀)。
    expect(out.url).toBe(UPLOAD_BASE + '/api/local/2026/a.bin');
    expect(out.url).not.toContain('//api');
  });

  it('无 url 且无 path 时兜底不拼出 undefined', async () => {
    stubFetch(jsonRes(200, { id: 'h1' }));
    const out = await api.upload(new File(['x'], 'a.bin', { type: 'application/octet-stream' }));
    expect(out.url).not.toContain('undefined');
    expect(out.url).toBe(UPLOAD_BASE + '/api/local/');
  });

  it('上传失败抛 Upload failed', async () => {
    stubFetch(jsonRes(500, {}));
    await expect(api.upload(new File(['x'], 'a.bin'))).rejects.toMatchObject({
      status: 500, message: 'Upload failed',
    });
  });
});

describe('auth 响应校验', () => {
  it('login 合法响应通过 zod 校验', async () => {
    stubFetch(jsonRes(200, {
      user: { id: 'u1', username: 'alice' },
      access_token: 'tok', expires_in: 3600,
    }));
    const out = await api.login('a@b.c', 'pw');
    expect(out.user.username).toBe('alice');
    expect(out.access_token).toBe('tok');
  });

  it('login 非法响应被 zod 拒绝', async () => {
    stubFetch(jsonRes(200, { user: { id: 1 } }));
    await expect(api.login('a@b.c', 'pw')).rejects.toThrow('API response validation failed: login');
  });
});
