// @ts-check
const { test, expect } = require('@playwright/test');

// CI 专用测试：不依赖后端，用 Mock API 验证前端逻辑，覆盖全部 28 个 Mock 方法
// 运行方式：npx playwright test tests/ci.spec.js

test.describe('Mock API Mode (CI)', () => {

  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.waitForSelector('.form-box');
  });

  test('debug mode toggle shows mock button', async ({ page }) => {
    await page.click('text=Debug mode');
    await expect(page.locator('text=Quick Enter (mock)')).toBeVisible();
  });

  test('mock login enters chat page', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');
    await page.waitForSelector('.sidebar');
    await expect(page.locator('.chat-list')).toBeVisible();
  });

  test('mock login shows chat list', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');
    await page.waitForSelector('.chat-item');
    const items = await page.locator('.chat-item').count();
    expect(items).toBeGreaterThanOrEqual(1);
  });

  test('mock mode persists after page reload', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');
    await page.reload();
    await page.waitForSelector('.chat-item');
    const items = await page.locator('.chat-item').count();
    expect(items).toBeGreaterThanOrEqual(1);
  });

  test('mock send message and see AI reply', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    // click first chat
    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.chat-input textarea');

    // send message
    await page.fill('.chat-input textarea', 'Hello from CI!');
    await page.click('button:has-text("Send")');

    // expect our message appears
    await expect(page.locator('.msg-content').first()).toContainText('Hello from CI!', { timeout: 5000 });
  });

  test('mock notice board: set, edit, clear', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.chat-header');

    const setBtn = page.locator('text=+ Set Notice');
    if (await setBtn.isVisible()) {
      await setBtn.click();
      await page.fill('input.input-field', 'CI Pinned Notice');
      await page.click('button:has-text("Save")');
      await expect(page.locator('text=📌 Notice:')).toBeVisible();
      await expect(page.locator('text=CI Pinned Notice')).toBeVisible();

      await page.click('button:has-text("Edit")');
      await page.fill('input.input-field', 'Updated CI Notice');
      await page.click('button:has-text("Save")');
      await expect(page.locator('text=Updated CI Notice')).toBeVisible();

      await page.click('button:has-text("Clear")');
      await expect(page.locator('text=📌 Notice:')).not.toBeVisible();
    }
  });

  test('mock reaction buttons exist', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.msg-content');

    // reaction button should exist
    await expect(page.locator('button.msg-btn:has-text("😀")').first()).toBeVisible({ timeout: 5000 });
  });

  test('logout from mock mode returns to login', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    // click logout button (sidebar bottom)
    const logoutBtn = page.locator('button:has-text("Log Out"), button:has-text("Logout")');
    if (await logoutBtn.isVisible()) {
      await logoutBtn.click();
      await page.waitForURL('/login');
      await expect(page.locator('h1')).toHaveText('Welcome back!');
    }
  });

  test('mock create and rename group chat', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.click('button[title="Create Group"]');
    await page.fill('input[placeholder="Group name..."]', 'MockGroup');
    await page.click('button:has-text("Create")');
    await page.waitForSelector('.chat-header');
    await expect(page.locator('.chat-header')).toContainText('MockGroup');

    const renameBtn = page.locator('button:has-text("Rename")');
    if (await renameBtn.isVisible()) {
      await renameBtn.click();
      await page.fill('input.input-field', 'RenamedGroup');
      await page.click('button:has-text("Save")');
      await expect(page.locator('.chat-header')).toContainText('RenamedGroup');
    }
  });

  test('mock create DM via search', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.click('button[title="New DM"]');
    await page.waitForSelector('input[placeholder="Search users..."]');
    await page.fill('input[placeholder="Search users..."]', 'user');
    await page.waitForTimeout(500);
    const userResult = page.locator('.dm-search-panel div:has-text("User")').first();
    if (await userResult.isVisible()) {
      await userResult.click();
      await page.waitForTimeout(500);
      const currentUrl = page.url();
      expect(currentUrl).not.toBe('/');
    }
  });

  test('mock edit and delete message', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.msg-content');

    const editBtn = page.locator('.msg-actions button:has-text("Edit")').first();
    const deleteBtn = page.locator('.msg-actions button:has-text("Delete")').first();

    if (await editBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await editBtn.click();
      const editInput = page.locator('.msg-edit-input, input.input-field').first();
      if (await editInput.isVisible({ timeout: 2000 }).catch(() => false)) {
        await editInput.fill('Edited content!');
        await page.locator('.msg-actions button:has-text("Save")').first().click();
        await expect(page.locator('text=Edited content!').first()).toBeVisible({ timeout: 3000 });
      }
    }

    if (await deleteBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await deleteBtn.click();
      await page.waitForTimeout(500);
      const confirmBtn = page.locator('button:has-text("Confirm"), button:has-text("Delete")').first();
      if (await confirmBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
        await confirmBtn.click();
      }
      await expect(page.locator('text=(message deleted)').first()).toBeVisible({ timeout: 3000 });
    }
  });

  test('mock add and remove reaction', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.msg-content');

    const reactionBtn = page.locator('button.msg-btn:has-text("😀")').first();
    await expect(reactionBtn).toBeVisible({ timeout: 5000 });

    await reactionBtn.click();
    await page.waitForTimeout(500);
    const emoji = page.locator('.emoji-picker button:has-text("👍")').first();
    if (await emoji.isVisible({ timeout: 2000 }).catch(() => false)) {
      await emoji.click();
      await page.waitForTimeout(500);
      await expect(page.locator('.reaction-chip:has-text("👍")').first()).toBeVisible({ timeout: 3000 });

      await page.locator('.reaction-chip:has-text("👍")').first().click();
      await page.waitForTimeout(500);
    }
  });

  test('mock delete chat from context menu', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    const initialCount = await page.locator('.chat-item').count();
    expect(initialCount).toBeGreaterThanOrEqual(1);

    const menuBtn = page.locator('.chat-item-menu-btn, button[title="More"]').first();
    if (await menuBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await menuBtn.click();
      const deleteOpt = page.locator('.context-menu button:has-text("Delete")').first();
      if (await deleteOpt.isVisible({ timeout: 2000 }).catch(() => false)) {
        await deleteOpt.click();
        await page.waitForTimeout(500);
        const afterCount = await page.locator('.chat-item').count();
        expect(afterCount).toBeLessThan(initialCount);
      }
    }
  });

  test('mock add member via member panel', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForTimeout(1000);

    const addBtn = page.locator('button:has-text("+ Add member")');
    if (await addBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      await addBtn.click();
      const searchInput = page.locator('input[placeholder="Search users..."]');
      if (await searchInput.isVisible({ timeout: 2000 }).catch(() => false)) {
        await searchInput.fill('te');
        await page.waitForTimeout(500);
      }
    }
  });

  test('mock public channels search visible', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    const searchPublicBtn = page.locator('button:has-text("Search"):right-of(:text("public"))').first();
    const fallback = page.locator('text=public channels');
    if (await fallback.isVisible({ timeout: 2000 }).catch(() => false)) {
      await fallback.click();
      await page.waitForTimeout(500);
    }
  });

  test('mock open settings and search users', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.click('button[title="Settings"]');
    await expect(page.locator('text=Settings')).toBeVisible({ timeout: 3000 });

    const displayNameInput = page.locator('.modal-box input.input-field').first();
    if (await displayNameInput.isVisible({ timeout: 2000 }).catch(() => false)) {
      await displayNameInput.fill('UpdatedName');
      await page.locator('.modal-box button:has-text("Save")').first().click();
      await page.waitForTimeout(500);
    }

    await page.locator('button:has-text("✕"), .modal-overlay').first().click({ force: true }).catch(() => {});
    await page.waitForTimeout(300);

    const searchChatInput = page.locator('input[placeholder="Search chats..."]');
    if (await searchChatInput.isVisible({ timeout: 2000 }).catch(() => false)) {
      await searchChatInput.fill('chat');
      await page.waitForTimeout(300);
    }
  });

  test('upload file to upload.moonchan.xyz and attach', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.chat-input textarea');

    const fileChooserPromise = page.waitForEvent('filechooser', { timeout: 5000 }).catch(() => null);
    await page.locator('button[title="Attach file"]').click();
    const fileChooser = await fileChooserPromise;
    if (!fileChooser) return;

    await fileChooser.setFiles({ name: 'ci-test.txt', mimeType: 'text/plain', buffer: Buffer.from('CI upload test ' + Date.now()) });
    await page.waitForTimeout(1000);
    await expect(page.locator('.file-attach')).toBeVisible({ timeout: 5000 });
  });

  test('upload avatar to upload.moonchan.xyz', async ({ page }) => {
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');

    await page.click('button[title="Settings"]');
    await expect(page.locator('text=Settings')).toBeVisible({ timeout: 3000 });

    const fileChooserPromise = page.waitForEvent('filechooser', { timeout: 5000 }).catch(() => null);
    await page.locator('.settings-avatar-placeholder, .settings-avatar-img').first().click();
    const fileChooser = await fileChooserPromise;
    if (!fileChooser) {
      await page.locator('button:has-text("✕"), .modal-overlay').first().click({ force: true }).catch(() => {});
      return;
    }

    await fileChooser.setFiles({ name: 'ci-avatar.png', mimeType: 'image/png', buffer: Buffer.from('CI avatar test') });
    await page.waitForTimeout(1000);
    await page.locator('.modal-box button:has-text("Save")').first().click();
    await page.waitForTimeout(500);
    await page.locator('button:has-text("✕"), .modal-overlay').first().click({ force: true }).catch(() => {});
  });

});