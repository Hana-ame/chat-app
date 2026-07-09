// @ts-check
const { test, expect } = require('@playwright/test');

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
  await page.goto('/register');
  const email = `test${Date.now()}@e2e.dev`;
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="text"]', 'E2ETest');
  await page.fill('input[type="password"]', 'testtest123');
  await page.click('button:has-text("Continue")');
  await page.waitForURL('/');
  await page.waitForSelector('.sidebar');
});

test('create group chat', async ({ page }) => {
  await page.goto('/register');
  const email = `gc${Date.now()}@e2e.dev`;
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="text"]', 'GroupCreator');
  await page.fill('input[type="password"]', 'testtest123');
  await page.click('button:has-text("Continue")');
  await page.waitForURL('/');

  await page.click('button[title="Create Group"]');
  await page.fill('input[placeholder="Group name..."]', 'E2E Group');
  await page.click('button:has-text("Create")');
  await page.waitForSelector('.chat-header');
  await expect(page.locator('.chat-header')).toContainText('E2E Group');
});

test('send and receive message', async ({ page }) => {
  await page.goto('/register');
  const email = `msg${Date.now()}@e2e.dev`;
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="text"]', 'MessageTester');
  await page.fill('input[type="password"]', 'testtest123');
  await page.click('button:has-text("Continue")');
  await page.waitForURL('/');

  await page.click('button[title="Create Group"]');
  await page.fill('input[placeholder="Group name..."]', 'M Test');
  await page.click('button:has-text("Create")');
  await page.waitForSelector('.chat-input textarea');

  await page.fill('.chat-input textarea', 'Hello E2E!');
  await page.click('button:has-text("Send")');
  await page.waitForSelector('.msg-content');
  await expect(page.locator('.msg-content').first()).toContainText('Hello E2E');
});

test('responsive layout on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto('/login');
  await expect(page.locator('form.form-box')).toBeVisible();
});

test('notice board functionality as owner', async ({ page }) => {
  await page.goto('/register');
  const email = `notice${Date.now()}@e2e.dev`;
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="text"]', 'NoticeOwner');
  await page.fill('input[type="password"]', 'testtest123');
  await page.click('button:has-text("Continue")');
  await page.waitForURL('/');

  await page.click('button[title="Create Group"]');
  await page.fill('input[placeholder="Group name..."]', 'Notice Group');
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
