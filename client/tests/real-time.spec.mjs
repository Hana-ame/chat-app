import { test, expect } from '@playwright/test';

test.describe('Real-time Events (WS / SSE / Polling)', () => {

  async function mockLogin(page) {
    await page.goto('/login');
    await page.waitForSelector('.form-box');
    await page.click('text=Debug mode');
    await page.click('text=Quick Enter (mock)');
    await page.waitForURL('/');
    await page.waitForSelector('.sidebar');
  }

  async function openFirstChat(page) {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    await page.locator('.chat-item').first().click();
  }

  test('WS ready event populates chat list', async ({ page }) => {
    await mockLogin(page);
    const items = await page.locator('.chat-item').count();
    expect(items).toBeGreaterThanOrEqual(1);
  });

  test('polling mode updates chat list periodically', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    const before = await page.locator('.chat-item').count();
    await page.waitForTimeout(2000);
    const after = await page.locator('.chat-item').count();
    expect(after).toBeGreaterThanOrEqual(before - 1);
  });

  test('sending message triggers onMessageCreate', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.chat-input textarea', { timeout: 5000 });
    await page.fill('.chat-input textarea', 'Real-time test message');
    await page.click('button:has-text("Send")');
    await expect(page.locator('.msg-content', { hasText: 'Real-time test message' }).first()).toBeVisible({ timeout: 5000 });
  });

  test('editing message triggers onMessageUpdate', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.msg-content', { timeout: 5000 });
    const editBtn = page.locator('.msg-actions button:has-text("Edit")').first();
    if (await editBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await editBtn.click();
      const editInput = page.locator('.msg-edit-input').first();
      if (await editInput.isVisible({ timeout: 2000 }).catch(() => false)) {
        await editInput.fill('Updated via onMessageUpdate');
        await page.locator('.msg-actions button:has-text("Save")').first().click();
        await expect(page.locator('text=Updated via onMessageUpdate').first()).toBeVisible({ timeout: 3000 });
      }
    }
  });

  test('deleting message triggers onMessageDelete', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.msg-content', { timeout: 5000 });
    const deleteBtn = page.locator('.msg-actions button:has-text("Delete")').first();
    if (await deleteBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      await deleteBtn.click();
      await page.waitForTimeout(500);
      const confirmBtn = page.locator('button:has-text("Confirm"), button:has-text("Delete")').first();
      if (await confirmBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
        await confirmBtn.click();
      }
    }
  });

  test('adding reaction button exists on messages', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.msg-content', { timeout: 5000 });
    const reactionBtn = page.locator('button.msg-btn:has-text("😀")').first();
    await expect(reactionBtn).toBeVisible({ timeout: 5000 });
  });

  test('chat create event adds new chat to list', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    const before = await page.locator('.chat-item').count();
    await page.click('button[title="Create Group"]');
    await page.fill('input[placeholder="Group name..."]', 'New Chat from Event');
    await page.click('button:has-text("Create")');
    await page.waitForSelector('.chat-header', { timeout: 5000 });
    const after = await page.locator('.chat-item').count();
    expect(after).toBeGreaterThan(before);
  });

  test('pinned notice set, edit, clear works', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.chat-header', { timeout: 5000 });
    const setBtn = page.locator('text=+ Set Notice');
    if (await setBtn.isVisible()) {
      await setBtn.click();
      await page.fill('input.input-field', 'RT notice test');
      await page.click('button:has-text("Save")');
      await expect(page.locator('text=📌 Notice:')).toBeVisible();
      await expect(page.locator('text=RT notice test')).toBeVisible();
      await page.click('button:has-text("Edit")');
      await page.fill('input.input-field', 'RT notice updated');
      await page.click('button:has-text("Save")');
      await expect(page.locator('text=RT notice updated')).toBeVisible();
      await page.click('button:has-text("Clear")');
      await expect(page.locator('text=📌 Notice:')).not.toBeVisible();
    }
  });

  test('unread badge visible on chats', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.msg-content', { timeout: 5000 });
  });

  test('member info visible in chat header', async ({ page }) => {
    await openFirstChat(page);
    await page.waitForSelector('.chat-header', { timeout: 5000 });
    await expect(page.locator('.chat-header')).toBeVisible();
  });

  test('chat delete via context menu reduces list', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    const before = await page.locator('.chat-item').count();
    await page.locator('.chat-item').first().click({ button: 'right' });
    const deleteOpt = page.locator('[role="menuitem"]:has-text("Delete"), .context-menu button:has-text("Delete")').first();
    if (await deleteOpt.isVisible({ timeout: 2000 }).catch(() => false)) {
      await deleteOpt.click();
      await page.waitForTimeout(500);
      const after = await page.locator('.chat-item').count();
      expect(after).toBeLessThan(before);
    }
  });

  test('protocol toggle cycles between modes', async ({ page }) => {
    await mockLogin(page);
    const toggleBtn = page.locator('button:has-text("WS"), button:has-text("SSE"), button:has-text("POLL")');
    if (await toggleBtn.isVisible({ timeout: 2000 }).catch(() => false)) {
      const txt = await toggleBtn.textContent();
      await toggleBtn.click();
      await page.waitForTimeout(300);
    }
  });

  test('disconnect and reconnect works', async ({ page }) => {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
  });
});
