// @ts-check
import { test, expect } from '@playwright/test';

// ── 共享账号 ──
// 后端有硬编码限流:register 5 次/分钟/IP、login 10 次/分钟/IP
// (router.go: httprate.LimitByIP),且本文件串行执行(playwright.config.js
// 中 e2e project 设 workers:1)。因此注册账号的用例共用 beforeAll 用户池:
// 池用固定邮箱,先登录复用(大多数运行 0 次注册),用户不存在才注册 →
// 反复重跑不会撞 register 限流窗口。注册/登录接口均返回 200。
const POOL = [
  { name: 'ui', email: 'e2e-pool-ui@e2e.dev' },
  { name: 'owner', email: 'e2e-pool-owner@e2e.dev' },
  { name: 'member', email: 'e2e-pool-member@e2e.dev' },
];

/** POST 带 429 限流重试:按 Retry-After(缺省 10s)退避,最多 12 次。 */
async function postWithRateLimitRetry(request, path, data) {
  for (let attempt = 0; ; attempt++) {
    const res = await request.post(path, { data });
    if (res.status() !== 429 || attempt >= 12) return res;
    const retryAfter = Number(res.headers()['retry-after'] ?? 10);
    await new Promise(r => setTimeout(r, Math.max(retryAfter, 5) * 1000));
  }
}

/** 优先登录复用固定邮箱用户,不存在则注册(429 自动重试)。 */
async function getOrCreateUser(request, email) {
  const login = await request.post('/api/auth/login', {
    data: { email, password: 'testtest123' },
  });
  if (login.status() === 200) {
    const body = await login.json();
    return { email, token: body.access_token, userId: body.user.id };
  }
  const res = await postWithRateLimitRetry(request, '/api/auth/register', {
    email,
    username: email.split('@')[0],
    password: 'testtest123',
  });
  expect(res.status()).toBe(200);
  const body = await res.json();
  return { email, token: body.access_token, userId: body.user.id };
}

let users = null;
test.beforeAll(async ({ request }) => {
  const registered = {};
  for (const p of POOL) registered[p.name] = await getOrCreateUser(request, p.email);
  users = registered;
});

/** 通过登录表单进入应用(复用 beforeAll 注册的账号)。 */
async function loginViaUI(page, email) {
  await page.goto('/login');
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', 'testtest123');
  await page.click('button:has-text("Log In")');
  await page.waitForURL(url => new URL(url).pathname === '/');
  await page.waitForSelector('.sidebar');
}

test('home redirects to login when not authenticated', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login/);
});

test('login form renders correctly', async ({ page }) => {
  await page.goto('/login');
  await expect(page.locator('h1')).toHaveText('Welcome back!');
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
  await expect(page.locator('button:has-text("Log In")')).toBeVisible();
});

test('register form renders correctly', async ({ page }) => {
  await page.goto('/register');
  await expect(page.locator('h1')).toHaveText('Create an account');
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="text"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
});

test('full auth flow', async ({ page }) => {
  const stamp = Date.now();
  const email = `test${stamp}@e2e.dev`;
  // UI 注册也可能撞 register 限流(5/分钟/IP):失败则等 10s 重填重试。
  for (let attempt = 0; ; attempt++) {
    await page.goto('/register');
    await page.fill('input[type="email"]', email);
    await page.fill('input[type="text"]', `E2E${stamp}`);
    await page.fill('input[type="password"]', 'testtest123');
    await page.click('button:has-text("Continue")');
    try {
      await page.waitForURL(url => new URL(url).pathname === '/', { timeout: 15000 });
      break;
    } catch {
      if (attempt >= 3) throw new Error('register flow kept failing (rate limited?)');
      await page.waitForTimeout(10000);
    }
  }
  await page.waitForSelector('.sidebar');
});

test('create group chat', async ({ page }) => {
  await loginViaUI(page, users.ui.email);
  const stamp = Date.now();

  await page.click('button[title="Create Group"]');
  const groupName = `E2E Group ${stamp}`;
  await page.fill('input[placeholder="Group name..."]', groupName);
  await page.click('button:has-text("Create")');
  await page.waitForSelector('.chat-header');
  await expect(page.locator('.chat-header')).toContainText(groupName);
});

test('send and receive message', async ({ page }) => {
  await loginViaUI(page, users.ui.email);
  const stamp = Date.now();

  await page.click('button[title="Create Group"]');
  const groupName = `M Test ${stamp}`;
  await page.fill('input[placeholder="Group name..."]', groupName);
  await page.click('button:has-text("Create")');
  await page.waitForSelector('[data-testid="chat-input"]');

  await page.fill('[data-testid="chat-input"]', 'Hello E2E!');
  await page.click('button[title="Send"]');
  await expect(page.locator('.msg-content').first()).toContainText('Hello E2E', { timeout: 15000 });
});

test('responsive layout on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto('/login');
  await expect(page.locator('form.form-box')).toBeVisible();
});

test('notice board functionality as owner', async ({ page }) => {
  await loginViaUI(page, users.ui.email);
  const stamp = Date.now();

  await page.click('button[title="Create Group"]');
  await page.fill('input[placeholder="Group name..."]', `Notice Group ${stamp}`);
  await page.click('button:has-text("Create")');
  await page.waitForSelector('.chat-header');

  const noticeBtn = page.locator('text=+ Set Notice');
  if (await noticeBtn.isVisible()) {
    await noticeBtn.click();
    await page.fill('input.input-field', 'This is a pinned notice!');
    await page.click('button:has-text("Save")');
    await expect(page.locator('text=📌 Notice:')).toBeVisible();
    await expect(page.locator('text=This is a pinned notice!')).toBeVisible();

    await page.click('button:has-text("Edit")');
    await page.fill('input.input-field', 'Updated notice!');
    await page.click('button:has-text("Save")');
    await expect(page.locator('text=Updated notice!')).toBeVisible();

    await page.click('button:has-text("Clear")');
    await expect(page.locator('text=📌 Notice:')).not.toBeVisible();
  }
});

// ── 边界场景(从 boundary.spec.mjs 迁移,改为真实后端 API 断言)──
// 原 boundary.spec.mjs 在 mock 模式下用户恒为 owner,无法验证真实错误
// 路径;这里用 Playwright 的 request fixture 直连后端 API,对超长消息与
// 越权操作做精确断言。
//
// 注意:后端 /api/auth/register 有硬编码限流 5 次/分钟/IP(router.go:
// httprate.LimitByIP(5, 1min)),共享用户池见文件头 beforeAll。429 路径
// 本身不做测试(触发它需要先把限流窗口耗尽,会连累其他用例)。

test('boundary: message content over limit is rejected', async ({ request }) => {
  // 服务端 MaxMessageContentLength=4000,超长消息必须被拒绝且不落库。
  const user = users.owner;

  const chatRes = await request.post('/api/chats', {
    headers: { Authorization: `Bearer ${user.token}` },
    data: { type: 'group', name: 'Limit Test', member_ids: [] },
  });
  expect(chatRes.status()).toBe(201);
  const chat = await chatRes.json();

  const tooLong = 'x'.repeat(4001);
  const sendRes = await request.post(`/api/chats/${chat.id}/messages`, {
    headers: { Authorization: `Bearer ${user.token}` },
    data: { content: tooLong },
  });
  expect(sendRes.status()).not.toBe(201);
  const errBody = await sendRes.json().catch(() => ({}));
  expect(errBody.error).toBe('content_too_long');

  // 消息列表里也不应出现这条超长消息。
  const listRes = await request.get(`/api/chats/${chat.id}/messages?limit=50`, {
    headers: { Authorization: `Bearer ${user.token}` },
  });
  const list = await listRes.json();
  expect(list.messages.every(m => m.content.length <= 4000)).toBe(true);

  // 恰好 4000 字节能正常发送(边界值)。
  const okRes = await request.post(`/api/chats/${chat.id}/messages`, {
    headers: { Authorization: `Bearer ${user.token}` },
    data: { content: 'x'.repeat(4000) },
  });
  expect(okRes.status()).toBe(201);
});

test('security: non-owner cannot modify notice or delete chat', async ({ request }) => {
  // 越权断言:成员(非 owner)不能设置公告、不能删除聊天;owner 可以。
  const { owner, member } = users;

  const chatRes = await request.post('/api/chats', {
    headers: { Authorization: `Bearer ${owner.token}` },
    data: { type: 'group', name: 'Authz Test', member_ids: [member.userId] },
  });
  expect(chatRes.status()).toBe(201);
  const chat = await chatRes.json();

  // 成员设置公告 → 403 forbidden。
  const memberAnnounce = await request.post(`/api/chats/${chat.id}/announcement`, {
    headers: { Authorization: `Bearer ${member.token}` },
    data: { content: 'hijacked' },
  });
  expect(memberAnnounce.status()).toBe(403);

  // 成员删除聊天 → 403 forbidden。
  const memberDelete = await request.delete(`/api/chats/${chat.id}`, {
    headers: { Authorization: `Bearer ${member.token}` },
  });
  expect(memberDelete.status()).toBe(403);

  // 对照组:owner 设置公告 → 200;成员清公告 → 403。
  const ownerAnnounce = await request.post(`/api/chats/${chat.id}/announcement`, {
    headers: { Authorization: `Bearer ${owner.token}` },
    data: { content: 'legit notice' },
  });
  expect(ownerAnnounce.status()).toBe(200);

  const memberClear = await request.delete(`/api/chats/${chat.id}/announcement`, {
    headers: { Authorization: `Bearer ${member.token}` },
  });
  expect(memberClear.status()).toBe(403);

  // 对照组:owner 删除聊天 → 200,聊天彻底不存在。
  const ownerDelete = await request.delete(`/api/chats/${chat.id}`, {
    headers: { Authorization: `Bearer ${owner.token}` },
  });
  expect(ownerDelete.status()).toBe(200);
  // 删除后聊天已消失:GetByID 先做 MustBeMember(防探测设计,不泄露聊天
  // 是否存在),成员记录被删后对任何人都是 403;若存在则 owner 会拿到 200。
  const gone = await request.get(`/api/chats/${chat.id}`, {
    headers: { Authorization: `Bearer ${owner.token}` },
  });
  expect([403, 404]).toContain(gone.status());
});
