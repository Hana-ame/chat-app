// @ts-check
import { test, expect } from '@playwright/test';

test.describe('AI Panel', () => {

  async function mockLogin(page) {
    await page.addInitScript(() => localStorage.clear());
    await page.goto('/login');
    await page.waitForSelector('.form-box');
    await page.evaluate(() => window.__mockLogin());
    await page.waitForURL('/');
    await page.waitForSelector('.sidebar');
  }

  async function openFirstChat(page) {
    await mockLogin(page);
    await page.waitForSelector('.chat-item', { timeout: 5000 });
    await page.locator('.chat-item').first().click();
    await page.waitForSelector('.chat-input textarea', { timeout: 5000 });
  }

  test('toggle AI panel open and closed', async ({ page }) => {
    await openFirstChat(page);
    const aiBtn = page.locator('button:has-text("AI")');
    await expect(aiBtn).toBeVisible();

    // Open
    await aiBtn.click();
    await expect(page.locator('text=endpoint:')).toBeVisible();
    await expect(page.locator('text=auth_key:')).toBeVisible();

    // Close
    await aiBtn.click();
    await expect(page.locator('text=endpoint:')).not.toBeVisible();
  });

  test('Simple mode: shows model/temperature/max_tokens/top_p/context checkbox', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    await expect(page.locator('text=model:')).toBeVisible();
    await expect(page.locator('text=temperature:')).toBeVisible();
    await expect(page.locator('text=max_tokens:')).toBeVisible();
    await expect(page.locator('text=top_p:')).toBeVisible();
    await expect(page.locator('text=发送 50 条上下文')).toBeVisible();

    // Simple radio is selected by default
    await expect(page.locator('input[name="aiBodyMode"][value="simple"]')).toBeChecked();
  });

  test('Simple mode: fill and persist input values', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    const endpointInput = page.locator('label:has-text("endpoint:") input');
    const authKeyInput = page.locator('label:has-text("auth_key:") input');
    const modelInput = page.locator('label:has-text("model:") input');

    await endpointInput.fill('https://test.api/v1/chat/completions');
    await authKeyInput.fill('sk-test-key-12345');
    await modelInput.fill('test-model-v1');

    await expect(endpointInput).toHaveValue('https://test.api/v1/chat/completions');
    await expect(authKeyInput).toHaveValue('sk-test-key-12345');
    await expect(modelInput).toHaveValue('test-model-v1');

    // Auth key should be password type
    await expect(authKeyInput).toHaveAttribute('type', 'password');
  });

  test('JSON mode: hides Simple fields, shows body textarea', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    // Switch to JSON mode
    await page.locator('input[name="aiBodyMode"][value="json"]').click();

    await expect(page.locator('label:has-text("model:")')).not.toBeVisible();
    await expect(page.locator('text=temperature:')).not.toBeVisible();
    await expect(page.locator('text=body (JSON):')).toBeVisible();

    const jsonTextarea = page.locator('label:has-text("body (JSON):") textarea');
    await expect(jsonTextarea).toBeVisible();
    const value = await jsonTextarea.inputValue();
    expect(value).toContain('"model"');
    expect(value).toContain('"messages"');
  });

  test('context checkbox toggles on/off', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    const checkbox = page.locator('input[type="checkbox"]');
    await expect(checkbox).toBeChecked();

    await checkbox.click();
    await expect(checkbox).not.toBeChecked();

    await checkbox.click();
    await expect(checkbox).toBeChecked();
  });

  test('send stream message creates AI placeholder message', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    // Fill AI config
    await page.locator('label:has-text("endpoint:") input').fill('https://api.test/v1/chat/completions');
    await page.locator('label:has-text("auth_key:") input').fill('sk-test');

    // Intercept the stream POST and return SSE chunks
    await page.route('**/api/chats/*/messages', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.continue();
        return;
      }
      const postData = JSON.parse(route.request().postData() || '{}');
      if (postData.type !== 'stream') {
        await route.continue();
        return;
      }
      const body = `data: {"content":"Hello"}\n\ndata: {"content":","}\n\ndata: {"content":"world"}\n\ndata: {"content":"!"}\n\ndata: [DONE]\n\n`;
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body,
      });
    });

    // Type message and send
    await page.fill('.chat-input textarea', 'Test AI please');
    await page.click('button[title="Send + AI reply"]');

    // Should show AI message with streaming content
    const aiMsg = page.locator('.msg-content');
    // Wait for AI placeholder to appear and content to accumulate
    await expect(aiMsg.last()).toHaveText(/Hello/);
  });

  test('JSON mode sends raw body to server', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    // Switch to JSON
    await page.locator('input[name="aiBodyMode"][value="json"]').click();

    // Fill endpoint/auth
    await page.locator('label:has-text("endpoint:") input').fill('https://api.test/v1/chat/completions');
    await page.locator('label:has-text("auth_key:") input').fill('sk-json-test');

    // Fill JSON body
    const jsonBody = JSON.stringify({
      model: 'custom-model',
      messages: [{ role: 'user', content: 'Hello from JSON' }],
      temperature: 0.5,
    }, null, 2);
    await page.locator('label:has-text("body (JSON):") textarea').fill(jsonBody);

    // Intercept and capture the request body
    let capturedBody = null;
    await page.route('**/api/chats/*/messages', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.continue();
        return;
      }
      const postData = JSON.parse(route.request().postData() || '{}');
      if (postData.type !== 'stream') {
        await route.continue();
        return;
      }
      capturedBody = postData.source?.body;
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: `data: {"content":"ok"}\n\ndata: [DONE]\n\n`,
      });
    });

    // Enable AI mode to ensure button shows "Send + AI reply"
    // Send message (needs chat input for the normal send, but AI send uses JSON body)
    await page.fill('.chat-input textarea', 'any text');
    await page.click('button[title="Send + AI reply"]');

    // Wait briefly for the route handler
    await page.waitForTimeout(1000);
    expect(capturedBody).toBeTruthy();
    expect(capturedBody.model).toBe('custom-model');
    expect(capturedBody.messages[0].content).toBe('Hello from JSON');
    expect(capturedBody.temperature).toBe(0.5);
  });

  test('AI send without context: only current message sent', async ({ page }) => {
    await openFirstChat(page);
    await page.locator('button:has-text("AI")').click();

    // Fill AI config
    await page.locator('label:has-text("endpoint:") input').fill('https://api.test/v1/chat/completions');
    await page.locator('label:has-text("auth_key:") input').fill('sk-test');

    // Uncheck context
    const checkbox = page.locator('input[type="checkbox"]');
    await checkbox.click();
    await expect(checkbox).not.toBeChecked();

    let capturedBody = null;
    await page.route('**/api/chats/*/messages', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.continue();
        return;
      }
      const postData = JSON.parse(route.request().postData() || '{}');
      if (postData.type !== 'stream') {
        await route.continue();
        return;
      }
      capturedBody = postData.source?.body;
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: `data: {"content":"ok"}\n\ndata: [DONE]\n\n`,
      });
    });

    await page.fill('.chat-input textarea', 'No context');
    await page.click('button[title="Send + AI reply"]');
    await page.waitForTimeout(1000);

    expect(capturedBody).toBeTruthy();
    // messages should have only the current user message, no history
    expect(capturedBody.messages.length).toBe(1);
    expect(capturedBody.messages[0].role).toBe('user');
    expect(capturedBody.messages[0].content).toBe('No context');
  });
});
