// @ts-check
const { test, expect } = require('@playwright/test');

// CI 专用测试：不依赖后端，用 Mock API 验证前端逻辑
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

});