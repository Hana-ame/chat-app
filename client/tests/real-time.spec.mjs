// real-time.spec.mjs — Mock 传输层实时事件测试(mock project)。
//
// 走 __mockLogin 进入应用内 Mock API 模式(mock transport 每 500ms 轮询),
// 验证消息增删改、反应、聊天增删、公告等事件驱动的 UI 行为。
// 不依赖 Go 后端;详见 docs/mock-strategy.md。
//
// 可靠性约定(第 29 轮起,消灭"条件跳过"假绿):
//   - 需要权限的操作(公告/删除聊天)先通过 UI 创建新群:mock 中创建者必为
//     owner,相关按钮必然出现,不再用 if(isVisible) 静默跳过
//   - 编辑/删除消息作用于"刚发送的消息"(作者必为自己)
//   - 原生 confirm 弹窗用 page.on('dialog') 处理(Playwright 无法点击原生弹窗)
//   - 断言一律硬断言;UI 轮询类等待用固定轮询周期(500ms)推导
//
// 运行: cd client && npm run test:e2e:mock
import { test, expect } from '@playwright/test';

async function mockLogin(page) {
  await page.addInitScript(() => localStorage.clear());
  await page.goto('/login');
  await page.waitForSelector('.form-box');
  await page.evaluate(() => window.__mockLogin());
  await page.waitForSelector('.sidebar');
}

async function openFirstChat(page) {
  await mockLogin(page);
  await page.waitForSelector('.chat-item', { timeout: 5000 });
  await page.locator('.chat-item').nth(1).click();
  await page.waitForSelector('[data-testid="chat-input"]', { timeout: 5000 });
}

async function createGroup(page, name) {
  await page.click('button[title="Create Group"]');
  await page.fill('input[placeholder="Group name..."]', name);
  await page.click('button:has-text("Create")');
  await page.waitForSelector('.chat-header', { timeout: 5000 });
}

async function sendMessage(page, text) {
  await page.fill('[data-testid="chat-input"]', text);
  await page.click('button[title="Send"]');
  const msg = page.locator('.msg-content', { hasText: text });
  await expect(msg.first()).toBeVisible({ timeout: 5000 });
  return msg.first();
}

test.describe('Real-time Events (WS / SSE / Polling)', () => {

  test('WS ready event populates chat list', async ({ page }) => {
    await mockLogin(page);
    const items = await page.locator('.chat-item').count();
    expect(items).toBeGreaterThanOrEqual(1);
  });

  test('polling mode keeps chat list stable', async ({ page }) => {
    // mock transport 每 500ms 轮询 listChats;种子数据固定,count 不应漂移。
    // 列表异步填充,先等 count 稳定(两次相隔 >500ms 相同)再取基准值。
    await mockLogin(page);
    await expect.poll(async () => {
      const a = await page.locator('.chat-item').count();
      await page.waitForTimeout(600);
      const b = await page.locator('.chat-item').count();
      return a === b ? a : -1;
    }, { timeout: 10000 }).toBeGreaterThan(1);
    const before = await page.locator('.chat-item').count();
    await page.waitForTimeout(1500);
    const after = await page.locator('.chat-item').count();
    expect(after).toBe(before);
  });

  test('sending message triggers onMessageCreate', async ({ page }) => {
    await openFirstChat(page);
    await sendMessage(page, 'Real-time test message');
  });

  test('editing message triggers onMessageUpdate', async ({ page }) => {
    await openFirstChat(page);
    const msg = await sendMessage(page, 'Edit me via onMessageUpdate');
    // 限定在刚发送消息的 msg-group 内(种子数据里也有自己的消息,全局 .first()
    // 会匹配到未 hover 的按钮 → not visible)。
    const group = page.locator('.msg-group', { hasText: 'Edit me via onMessageUpdate' }).last();
    await group.hover();
    await group.locator('.msg-actions button:has-text("Edit")').click();
    await group.locator('textarea.input-field').fill('Updated via onMessageUpdate');
    await group.locator('button:has-text("Save")').click();
    await expect(
      page.locator('.msg-content', { hasText: 'Updated via onMessageUpdate' }).first()
    ).toBeVisible({ timeout: 5000 });
  });

  test('deleting message triggers onMessageDelete', async ({ page }) => {
    await openFirstChat(page);
    await sendMessage(page, 'Delete me via onMessageDelete');
    const group = page.locator('.msg-group', { hasText: 'Delete me via onMessageDelete' }).last();
    await group.hover();
    page.once('dialog', d => d.accept());
    await group.locator('.msg-actions button:has-text("Delete")').click();
    await expect(page.locator('.msg-deleted', { hasText: 'message deleted' }).first())
      .toBeVisible({ timeout: 5000 });
  });

  test('adding reaction updates UI', async ({ page }) => {
    await openFirstChat(page);
    await sendMessage(page, 'React to me');
    const group = page.locator('.msg-group', { hasText: 'React to me' }).last();
    await group.hover();
    // 点开 emoji picker,选 👍 → 该消息出现 reaction chip。
    await group.locator('.msg-actions button:has-text("😀")').click();
    await page.locator('.emoji-btn:has-text("👍")').first().click();
    await expect(page.locator('.reaction-chip', { hasText: '👍' }).first())
      .toBeVisible({ timeout: 5000 });
  });

  test('chat create event adds new chat to list', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    const before = await page.locator('.chat-item').count();
    await createGroup(page, 'New Chat from Event');
    const after = await page.locator('.chat-item').count();
    expect(after).toBe(before + 1);
  });

  test('notice board: set, edit, clear as creator', async ({ page }) => {
    // mock 中新建群 owner 必为自己 → Set announcement 必然出现。
    await mockLogin(page);
    await createGroup(page, 'Notice Test Chat');
    await page.click('button[title="Set announcement"]');
    await page.getByTestId('notice-input').fill('Mode switch test notice');
    await page.getByTestId('notice-save').click();
    await expect(page.locator('text=📢 公告')).toBeVisible();
    await expect(page.locator('text=Mode switch test notice')).toBeVisible();

    await page.getByTestId('notice-edit').click();
    await page.getByTestId('notice-input').fill('Edited notice text');
    await page.getByTestId('notice-save').click();
    await expect(page.locator('text=Edited notice text')).toBeVisible();

    await page.getByTestId('notice-clear').click();
    await expect(page.locator('text=📢 公告')).not.toBeVisible();
  });

  test('unread badges visible for unread chats', async ({ page }) => {
    // 种子数据 chats[1].unread_count=4,mock 无 mark-read API 不清零。
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    await expect(page.locator('.unread-badge').first()).toBeVisible();
  });

  test('member count visible in chat header', async ({ page }) => {
    await openFirstChat(page);
    const header = page.locator('.chat-header');
    await expect(header).toBeVisible();
    await expect(header).toContainText(/[0-9]/);
  });

  test('chat delete event removes chat from list', async ({ page }) => {
    // 创建自己的群再删除:右键菜单必然出现(owner),count 确定 -1。
    // 新建群不保证排首位,按名称定位右键目标。
    await mockLogin(page);
    await expect.poll(async () => {
      const a = await page.locator('.chat-item').count();
      await page.waitForTimeout(600);
      const b = await page.locator('.chat-item').count();
      return a === b ? a : -1;
    }, { timeout: 10000 }).toBeGreaterThan(1);
    const before = await page.locator('.chat-item').count();
    await createGroup(page, 'To Be Deleted');
    await page.locator('.chat-item', { hasText: 'To Be Deleted' }).click({ button: 'right' });
    page.once('dialog', d => d.accept());
    await page.locator('.context-menu button:has-text("Delete")').first().click();
    await expect
      .poll(async () => page.locator('.chat-item').count(), { timeout: 5000 })
      .toBe(before);
  });

  test('transport mode button cycles WS/SSE/Poll', async ({ page }) => {
    // mock 模式下 transport 恒为 mock;这里只验证 UI 模式切换器行为。
    await mockLogin(page);
    const btn = page.locator('button[title^="Click to switch"]');
    await expect(btn).toBeVisible();
    const first = await btn.textContent();
    await btn.click();
    const second = await btn.textContent();
    expect(second).not.toBe(first);
  });
});
