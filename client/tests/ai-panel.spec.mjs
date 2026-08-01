// ai-panel.spec.mjs — AI 面板与 SSE 流式请求测试(mock project)。
//
// 走 __mockLogin 进入 Mock 模式;其中 3 个用例用 page.route 拦截
// /api/chats/*/messages 并返回真实格式的 SSE 响应,验证前端流式渲染与
// 请求体构造。不依赖 Go 后端。
//
// 面板 UI 元素通过 data-testid 定位(见 src/components/Composer.jsx):
//   ai-toggle        启用/停用 AI 面板的 🤖 按钮
//   ai-endpoint / ai-key / ai-model      基本模式的输入框(Key 为 password)
//   ai-temperature / ai-top-p / ai-max-tokens   数值输入
//   ai-context-limit 上下文滑杆(0 = 不发送,>0 = 发送最近 N 条)
//   ai-mode-basic / ai-mode-json          Basic/JSON 模式切换按钮
//   ai-json-body      JSON 模式的请求体 textarea
//
// 运行: cd client && npm run test:e2e:mock
// @ts-check
import { test, expect } from '@playwright/test';

const TESTID = (name) => `[data-testid="${name}"]`;

// 对 React 受控的 range 输入设置值并派发 input 事件(Playwright 的 fill 不支持 range)。
async function setRange(page, testid, value) {
  await page.locator(TESTID(testid)).evaluate((el, v) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setter.call(el, String(v));
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }, value);
}

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
    // nth(0) 是 Notifications 聊天(AI 发送按设计被禁用),必须用普通聊天
    await page.locator('.chat-item').nth(1).click();
    await page.waitForSelector('[data-testid="chat-input"]', { timeout: 5000 });
  }

  // 拦截流式 POST,返回真实格式的 SSE 响应;并暴露 postData.source.body 供断言。
  async function interceptStream(page, onCaptured) {
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
      if (onCaptured) onCaptured(postData.source?.body);
      await route.fulfill({
        status: 200,
        headers: { 'Content-Type': 'text/event-stream' },
        body: `data: {"content":"Hello"}\n\ndata: {"content":","}\n\ndata: {"content":"world"}\n\ndata: {"content":"!"}\n\ndata: [DONE]\n\n`,
      });
    });
  }

  test('toggle AI panel open and closed', async ({ page }) => {
    await openFirstChat(page);
    const aiToggle = page.locator(TESTID('ai-toggle'));
    await expect(aiToggle).toBeVisible();

    // Open: 面板显示 Endpoint/Key 输入
    await aiToggle.click();
    await expect(page.locator(TESTID('ai-endpoint'))).toBeVisible();
    await expect(page.locator(TESTID('ai-key'))).toBeVisible();

    // Close
    await aiToggle.click();
    await expect(page.locator(TESTID('ai-endpoint'))).not.toBeVisible();
  });

  test('Basic mode: shows model/temperature/top_p/max_tokens/context slider', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    await expect(page.locator(TESTID('ai-model'))).toBeVisible();
    await expect(page.locator(TESTID('ai-temperature'))).toBeVisible();
    await expect(page.locator(TESTID('ai-top-p'))).toBeVisible();
    await expect(page.locator(TESTID('ai-max-tokens'))).toBeVisible();
    await expect(page.locator(TESTID('ai-context-limit'))).toBeVisible();
    // 上下文滑杆默认 50 → 显示"最近50条"
    await expect(page.locator('text=最近50条')).toBeVisible();

    // Basic 是默认模式,JSON 面板隐藏
    await expect(page.locator(TESTID('ai-json-body'))).not.toBeVisible();
  });

  test('Basic mode: fill and persist input values', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    const endpointInput = page.locator(TESTID('ai-endpoint'));
    const authKeyInput = page.locator(TESTID('ai-key'));
    const modelInput = page.locator(TESTID('ai-model'));

    await endpointInput.fill('https://test.api/v1/chat/completions');
    await authKeyInput.fill('sk-test-key-12345');
    await modelInput.fill('test-model-v1');

    await expect(endpointInput).toHaveValue('https://test.api/v1/chat/completions');
    await expect(authKeyInput).toHaveValue('sk-test-key-12345');
    await expect(modelInput).toHaveValue('test-model-v1');

    // Auth key 应为 password 类型
    await expect(authKeyInput).toHaveAttribute('type', 'password');
  });

  test('JSON mode: hides Basic fields, shows body textarea', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    // 切到 JSON 模式
    await page.locator(TESTID('ai-mode-json')).click();

    await expect(page.locator(TESTID('ai-model'))).not.toBeVisible();
    await expect(page.locator(TESTID('ai-temperature'))).not.toBeVisible();
    const jsonTextarea = page.locator(TESTID('ai-json-body'));
    await expect(jsonTextarea).toBeVisible();

    // 默认 body 包含 model/messages 字段
    const value = await jsonTextarea.inputValue();
    expect(value).toContain('"model"');
    expect(value).toContain('"messages"');

    // 切回 Basic
    await page.locator(TESTID('ai-mode-basic')).click();
    await expect(page.locator(TESTID('ai-model'))).toBeVisible();
  });

  test('context slider toggles on/off', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    const context = page.locator(TESTID('ai-context-limit'));
    await expect(context).toHaveValue('50');
    await expect(page.locator('text=最近50条')).toBeVisible();

    // 滑到 0 → 不发送上下文
    await setRange(page, 'ai-context-limit', 0);
    await expect(page.locator('text=不发送')).toBeVisible();

    // 滑回 50 → 恢复
    await setRange(page, 'ai-context-limit', 50);
    await expect(page.locator('text=最近50条')).toBeVisible();
  });

  test('send stream message creates AI placeholder message', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    // 填 AI 配置
    await page.locator(TESTID('ai-endpoint')).fill('https://api.test/v1/chat/completions');
    await page.locator(TESTID('ai-key')).fill('sk-test');

    // 拦截流式 POST 并返回 SSE 分片
    await interceptStream(page);

    // 发消息
    await page.fill('[data-testid="chat-input"]', 'Test AI please');
    await page.click('button[title="Send + AI reply"]');

    // AI 消息应出现且内容随流式累积
    const aiMsg = page.locator('.msg-content');
    await expect(aiMsg.last()).toHaveText(/Hello/);
  });

  test('JSON mode sends raw body to server', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    // 切到 JSON
    await page.locator(TESTID('ai-mode-json')).click();

    // 填 endpoint/key
    await page.locator(TESTID('ai-endpoint')).fill('https://api.test/v1/chat/completions');
    await page.locator(TESTID('ai-key')).fill('sk-json-test');

    // 填 JSON body
    const jsonBody = JSON.stringify({
      model: 'custom-model',
      messages: [{ role: 'user', content: 'Hello from JSON' }],
      temperature: 0.5,
    }, null, 2);
    await page.locator(TESTID('ai-json-body')).fill(jsonBody);

    // 拦截并捕获请求体
    let capturedBody = null;
    await interceptStream(page, (body) => { capturedBody = body; });

    await page.fill('[data-testid="chat-input"]', 'any text');
    await page.click('button[title="Send + AI reply"]');

    await expect(page.locator('.msg-content').last()).toHaveText(/Hello/);
    expect(capturedBody).toBeTruthy();
    expect(capturedBody.model).toBe('custom-model');
    expect(capturedBody.messages[0].content).toBe('Hello from JSON');
    expect(capturedBody.temperature).toBe(0.5);
  });

  test('AI send without context: only current message sent', async ({ page }) => {
    await openFirstChat(page);
    await page.locator(TESTID('ai-toggle')).click();

    // 填 AI 配置
    await page.locator(TESTID('ai-endpoint')).fill('https://api.test/v1/chat/completions');
    await page.locator(TESTID('ai-key')).fill('sk-test');

    // 关闭上下文(滑杆到 0)
    await setRange(page, 'ai-context-limit', 0);
    await expect(page.locator('text=不发送')).toBeVisible();

    let capturedBody = null;
    await interceptStream(page, (body) => { capturedBody = body; });

    await page.fill('[data-testid="chat-input"]', 'No context');
    await page.click('button[title="Send + AI reply"]');

    await expect(page.locator('.msg-content').last()).toHaveText(/Hello/);
    expect(capturedBody).toBeTruthy();
    // messages 应只有当前用户消息,无历史
    expect(capturedBody.messages.length).toBe(1);
    expect(capturedBody.messages[0].role).toBe('user');
    expect(capturedBody.messages[0].content).toBe('No context');
  });
});
