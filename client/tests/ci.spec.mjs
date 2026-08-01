// ci.spec.mjs — Mock API 模式冒烟测试(mock project)。
//
// 走 __mockLogin 进入应用内 Mock API 模式,验证登录/发消息/公告/退出/
// 设置/上传等核心 UI 流程。不依赖 Go 后端;详见 docs/mock-strategy.md。
//
// 可靠性约定(第 29 轮起,消灭"条件跳过"假绿):
//   - 需要权限的操作先通过 UI 创建新群(mock 中创建者必为 owner)
//   - 编辑/删除消息作用于"刚发送的消息";原生 confirm 用 page.on('dialog')
//   - 选择器必须是现存 UI 元素:退出按钮是 ↪(无文本),公告显示为 📢 公告,
//     设置弹窗 aria-label="Settings",Rename 按钮已不存在于 UI
//
// 运行: cd client && npm run test:e2e:mock
// @ts-check
import { test, expect } from '@playwright/test';

test.describe('Mock API Mode (CI)', () => {

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

  test('mock login shows sidebar', async ({ page }) => {
    await mockLogin(page);
    await expect(page.locator('.sidebar')).toBeVisible();
  });

  test('mock login shows chat items', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    const items = await page.locator('.chat-item').count();
    expect(items).toBeGreaterThanOrEqual(1);
  });

  test('mock send message', async ({ page }) => {
    await openFirstChat(page);
    await sendMessage(page, 'Hello from CI!');
  });

  test('mock notice board: set, edit, clear as creator', async ({ page }) => {
    // 新建群 owner 必为自己 → Set announcement / Edit / Clear 必然出现。
    await mockLogin(page);
    await createGroup(page, 'CI Notice Chat');
    await page.click('button[title="Set announcement"]');
    await page.getByTestId('notice-input').fill('CI Pinned Notice');
    await page.getByTestId('notice-save').click();
    await expect(page.locator('text=📢 公告')).toBeVisible();
    await expect(page.locator('text=CI Pinned Notice')).toBeVisible();

    await page.getByTestId('notice-edit').click();
    await page.getByTestId('notice-input').fill('Updated CI Notice');
    await page.getByTestId('notice-save').click();
    await expect(page.locator('text=Updated CI Notice')).toBeVisible();

    await page.getByTestId('notice-clear').click();
    await expect(page.locator('text=📢 公告')).not.toBeVisible();
  });

  test('logout returns to login', async ({ page }) => {
    await mockLogin(page);
    await page.getByRole('button', { name: '↪' }).click();
    await page.waitForURL('/login');
    await expect(page.locator('h1')).toHaveText('Welcome back!');
  });

  test('mock create group chat', async ({ page }) => {
    await mockLogin(page);
    await createGroup(page, 'MockGroup');
    await expect(page.locator('.chat-header')).toContainText('MockGroup');
  });

  test('mock edit and delete own message', async ({ page }) => {
    await openFirstChat(page);
    const msg = await sendMessage(page, 'Editable CI message');
    // 编辑:作者是自己 → Edit 按钮必然出现。限定在刚发送消息的 msg-group 内,
    // 避免 .first() 误匹配种子数据里其他消息的按钮(hover 未生效 → not visible)。
    const group = page.locator('.msg-group', { hasText: 'Editable CI message' }).last();
    await group.hover();
    await group.locator('.msg-actions button:has-text("Edit")').click();
    await page.locator('.msg-group', { hasText: 'Editable CI message' }).last()
      .locator('textarea.input-field').fill('Edited content!');
    await page.locator('.msg-group', { hasText: 'Editable CI message' }).last()
      .locator('button:has-text("Save")').click();
    await expect(page.locator('.msg-content', { hasText: 'Edited content!' }).first())
      .toBeVisible({ timeout: 5000 });

    // 删除:原生 confirm 弹窗 → dialog accept。
    const group2 = page.locator('.msg-group', { hasText: 'Edited content!' }).last();
    await group2.hover();
    page.once('dialog', d => d.accept());
    await group2.locator('.msg-actions button:has-text("Delete")').click();
    await expect(page.locator('.msg-deleted', { hasText: 'message deleted' }).first())
      .toBeVisible({ timeout: 5000 });
  });

  test('mock delete own chat from context menu', async ({ page }) => {
    // 创建自己的群 → 右键菜单 Delete 必然出现(owner),count 确定 -1。
    // 新建群不保证排在列表首位,按名称定位,不依赖 first()。
    await mockLogin(page);
    await expect.poll(async () => {
      const a = await page.locator('.chat-item').count();
      await page.waitForTimeout(600);
      const b = await page.locator('.chat-item').count();
      return a === b ? a : -1;
    }, { timeout: 10000 }).toBeGreaterThan(1);
    const before = await page.locator('.chat-item').count();
    await createGroup(page, 'CI Delete Me');
    await page.locator('.chat-item', { hasText: 'CI Delete Me' }).click({ button: 'right' });
    page.once('dialog', d => d.accept());
    await page.locator('.context-menu button:has-text("Delete")').first().click();
    await expect
      .poll(async () => page.locator('.chat-item').count(), { timeout: 5000 })
      .toBe(before);
  });

  test('mock public channels visible on search focus', async ({ page }) => {
    await mockLogin(page);
    const search = page.locator('.sidebar-search-row input').first();
    // focus 触发 loadAllPublicChats → 渲染 Public Channels 列表。
    await search.focus();
    await expect(page.locator('text=Public Channels')).toBeVisible({ timeout: 5000 });
    // 输入关键词后列表切换为匹配结果(列表组件隐藏,回到聊天列表)。
    await search.fill('te');
    await expect(page.locator('text=Public Channels')).not.toBeVisible({ timeout: 5000 });
  });

  test('mock open settings and close', async ({ page }) => {
    await mockLogin(page);
    await page.click('button[title="Settings"]');
    await expect(page.locator('[aria-label="Settings"]')).toBeVisible({ timeout: 3000 });
    // overlay 中心被 modal-box 覆盖(force click 点中 box → stopPropagation),
    // 用弹窗内 ✕ 关闭按钮。
    await page.locator('.modal-box button:has-text("✕")').click();
    await expect(page.locator('[aria-label="Settings"]')).not.toBeVisible({ timeout: 3000 });
  });

  test('mock upload file to composer', async ({ page }) => {
    await openFirstChat(page);
    const fileChooserPromise = page.waitForEvent('filechooser', { timeout: 5000 });
    await page.locator('button[title="Attach file"]').click();
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles({ name: 'ci-test.txt', mimeType: 'text/plain', buffer: Buffer.from('CI upload test ' + Date.now()) });
    await expect(page.locator('.file-attach').first()).toBeVisible({ timeout: 5000 });
  });

  test('mock upload avatar in settings', async ({ page }) => {
    await mockLogin(page);
    await page.click('button[title="Settings"]');
    await expect(page.locator('[aria-label="Settings"]')).toBeVisible({ timeout: 3000 });
    // dev-self 无 avatar_url → 显示 "Click to upload"。
    const fileChooserPromise = page.waitForEvent('filechooser', { timeout: 3000 });
    await page.locator('text=Click to upload').first().click();
    const fileChooser = await fileChooserPromise;
    await fileChooser.setFiles({ name: 'ci-avatar.png', mimeType: 'image/png', buffer: Buffer.from('CI avatar test') });
    await expect(page.locator('.modal-box button:has-text("Save")').first()).toBeVisible({ timeout: 3000 });
    await page.locator('.modal-box button:has-text("✕")').click();
    await expect(page.locator('[aria-label="Settings"]')).not.toBeVisible({ timeout: 3000 });
  });

});
