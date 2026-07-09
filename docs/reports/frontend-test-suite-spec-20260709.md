# 前端测试套件规范 (Frontend Test Suite Spec)

> 原始来源：
> - `client/tests/ci.spec.js`
> - `client/tests/e2e.spec.js`
> - `client/src/api/mock.js`
> - `client/src/api/client.js`
> - `client/src/store/auth.js`
>
> 依赖骨架：`client/src/api/client.js`（request + MOCKABLE + enableMock/disableMock）

---

## 一、测试骨架

### 1.1 Mock API 基础设施

**文件:** `client/src/api/mock.js`

```js
class MockAPI {
  constructor() {
    this.data = null; // 由 resetMockData 初始化
  }
  // 28 个 mock 方法，每个签名与真实 API 一致
  mockRegister(email, username, password) { /* ... */ }
  mockLogin(email, password) { /* ... */ }
  mockRefresh() { /* ... */ }
  mockLogout(token) { /* ... */ }
  mockMe(token) { /* ... */ }
  mockUpdateProfile(token, data) { /* ... */ }
  mockSearchUsers(token, q) { /* ... */ }
  mockListChats(token) { /* ... */ }
  mockListPublicChats(token) { /* ... */ }
  mockCreateChat(token, name, memberIds, visibility) { /* ... */ }
  mockGetChat(token, id) { /* ... */ }
  mockDeleteChat(token, id) { /* ... */ }
  mockRenameChat(token, id, name) { /* ... */ }
  mockCreateDM(token, userId) { /* ... */ }
  mockJoinChat(token, chatId) { /* ... */ }
  mockSetPinnedMessage(token, chatId, content) { /* ... */ }
  mockClearPinnedMessage(token, chatId) { /* ... */ }
  mockAddMember(token, chatId, userId) { /* ... */ }
  mockRemoveMember(token, chatId, userId) { /* ... */ }
  mockListMessages(token, chatId, before, limit) { /* ... */ }
  mockSendMessage(token, chatId, content, attachments) { /* ... */ }
  mockEditMessage(token, chatId, msgId, content) { /* ... */ }
  mockDeleteMessage(token, chatId, msgId) { /* ... */ }
  mockMarkRead(token, chatId, messageId) { /* ... */ }
  mockAddReaction(token, chatId, msgId, emoji) { /* ... */ }
  mockRemoveReaction(token, chatId, msgId, emoji) { /* ... */ }
  mockUpload(file) { /* ... */ }
  mockUploadAvatar(token, file) { /* ... */ }
}
```

**Mock 数据初始化:**
```
generateDummyData({ chatCount: 10, msgPerChat: 150 })
  ├── chats: 10 个聊天（群组 + DM + 公开，含 pinnedMessage）
  └── messages: 1500 条消息（带 @提及、Markdown、code block、reactions）
```

**每个 mock 请求延迟 ~50ms**（模拟网络往返）。

### 1.2 Mock 开关系统

**文件:** `client/src/api/client.js`

```js
const MOCKABLE = [
  ['register', mockRegister],
  ['login', mockLogin],
  ['refresh', mockRefresh],
  ['logout', mockLogout],
  ['me', mockMe],
  ['updateProfile', mockUpdateProfile],
  ['searchUsers', mockSearchUsers],
  ['listChats', mockListChats],
  ['listPublicChats', mockListPublicChats],
  ['createChat', mockCreateChat],
  ['getChat', mockGetChat],
  ['deleteChat', mockDeleteChat],
  ['renameChat', mockRenameChat],
  ['createDM', mockCreateDM],
  ['joinChat', mockJoinChat],
  ['setPinnedMessage', mockSetPinnedMessage],
  ['clearPinnedMessage', mockClearPinnedMessage],
  ['addMember', mockAddMember],
  ['removeMember', mockRemoveMember],
  ['listMessages', mockListMessages],
  ['sendMessage', mockSendMessage],
  ['editMessage', mockEditMessage],
  ['deleteMessage', mockDeleteMessage],
  ['markRead', mockMarkRead],
  ['addReaction', mockAddReaction],
  ['removeReaction', mockRemoveReaction],
  ['upload', mockUpload],
  ['uploadAvatar', mockUploadAvatar],
]; // 共 28 对

let _originals = {};
let _mockEnabled = false;

api.enableMock = () => {
  if (_mockEnabled) return;
  _mockEnabled = true;
  resetMockData();
  for (const [key, mock] of MOCKABLE) {
    _originals[key] = api[key];
    api[key] = mock;
  }
};

api.disableMock = () => {
  if (!_mockEnabled) return;
  _mockEnabled = false;
  for (const [key] of MOCKABLE) {
    api[key] = _originals[key];
  }
  _originals = {};
};
```

**条件分支:**
- 已启用 → 直接 return
- 启用时：保存 28 个原函数到 `_originals`，替换为 mock，调用 `resetMockData()`
- 禁用时：从 `_originals` 恢复所有原函数

### 1.3 Mock 登录自动恢复

**文件:** `client/src/store/auth.js`

```js
// 初始化时检测 mock-token
const saved = JSON.parse(localStorage.getItem('auth') || '{}');
if (saved.accessToken === 'mock-token') {
  api.enableMock();
}

// mockLogin
mockLogin: () => {
  api.enableMock();
  set({ mode: 'poll' });
  // 写入 mock user 和 accessToken: 'mock-token'
  save({ user: mockUser, accessToken: 'mock-token' });
  set({ user: mockUser, accessToken: 'mock-token', loading: false, error: null });
}

// logout
logout: () => {
  api.disableMock();
  clearStorage();
  resetStore();
}
```

**条件分支:**
- `saved.accessToken === 'mock-token'` → 自动 `api.enableMock()`（页面刷新恢复）
- `mockLogin` 调用 → `api.enableMock()` + `setMode('poll')` + 写入 `accessToken: 'mock-token'`
- `logout` 调用 → `api.disableMock()` + 清除所有 store + localStorage

### 1.4 测试前置条件

**CI 测试（`ci.spec.js`）:**
```js
test.beforeEach(async ({ page }) => {
  await page.goto('/login');
  await page.waitForSelector('.form-box');
});
```

每个 CI 测试独立运行：
1. 导航到 `/login`
2. 等待表单渲染
3. 勾选 Debug mode → 点击 Quick Enter (mock)
4. 测试 Mock API 行为

**E2E 测试（`e2e.spec.js`）:**
每个测试独立注册新用户（随机 email `test${Date.now()}@e2e.dev`），创建独立群聊。后端 SQLite 由服务端自动隔离。

---

## 二、测试总表

### 2.1 `ci.spec.js` — CI Mock 测试（9 个）

**文件:** `client/tests/ci.spec.js`（115 行）

| # | 测试名 | 行号 | 覆盖场景 | 验证点 |
|---|--------|------|---------|--------|
| 1 | `debug mode toggle shows mock button` | 14 | Debug 模式 UI | Debug mode 复选框 → Quick Enter 按钮出现 |
| 2 | `mock login enters chat page` | 19 | Mock 登录路由 | Quick Enter → URL `/` + `.sidebar` 可见 |
| 3 | `mock login shows chat list` | 27 | 聊天列表渲染 | `.chat-item` ≥ 1 个 |
| 4 | `mock mode persists after page reload` | 36 | localStorage 持久化 | 刷新后 `.chat-item` ≥ 1 个 |
| 5 | `mock send message and see AI reply` | 46 | 消息发送 + AI 回复 | 输入文本 → 发送 → 消息出现在 `.msg-content` |
| 6 | `mock notice board: set, edit, clear` | 62 | 公告栏 CRUD | Set → 显示 📌 Notice: / Edit → 内容更新 / Clear → 消失 |
| 7 | `mock reaction buttons exist` | 89 | Reaction UI | `button.msg-btn:has-text("😀")` 可见 |
| 8 | `logout from mock mode returns to login` | 101 | 登出路由 | Logout → URL `/login` + `h1: "Welcome back!"` |

### 2.2 `e2e.spec.js` — E2E 全链路测试（8 个）

**文件:** `client/tests/e2e.spec.js`（108 行）

| # | 测试名 | 行号 | 覆盖场景 | 验证点 |
|---|--------|------|---------|--------|
| 1 | `home redirects to login when not authenticated` | 4 | 未认证重定向 | `/` → URL 包含 `/login` |
| 2 | `login form renders correctly` | 9 | 登录页 UI | `h1: "Welcome back!"` + email/password input + Log In 按钮 |
| 3 | `register form renders correctly` | 17 | 注册页 UI | `h1: "Create an account"` + email/text/password input |
| 4 | `full auth flow` | 25 | 注册 → 自动登录 | 填表 → Continue → URL `/` + `.sidebar` 可见 |
| 5 | `create group chat` | 36 | 创建群聊 | Create Group → 填名 → Create → `.chat-header` 含群名 |
| 6 | `send and receive message` | 52 | 消息收发 | textarea 输入 → Send → `.msg-content` 含文本 |
| 7 | `responsive layout on mobile` | 72 | 375px 视口适配 | `setViewportSize(375, 667)` → `form.form-box` 可见 |
| 8 | `notice board functionality as owner` | 78 | Owner 公告栏 CRUD | + Set Notice → 输入 → Save → 📌 可见 → Edit → Update → Clear → 消失 |

---

## 三、测试详细说明

### 3.1 `ci.spec.js` — CI Mock 测试

---

#### `debug mode toggle shows mock button` (line 14)

```js
test('debug mode toggle shows mock button', async ({ page }) => {
  await page.click('text=Debug mode');
  await expect(page.locator('text=Quick Enter (mock)')).toBeVisible();
});
```

**依赖链:** `page.goto(/login) → page.click(Debug mode) → page.locator`

**条件分支:**
- `Debug mode` 复选框不存在 → test 失败
- `Quick Enter (mock)` 不出现 → `toBeVisible` 断言失败

---

#### `mock login enters chat page` (line 19)

```js
test('mock login enters chat page', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');
  await page.waitForSelector('.sidebar');
  await expect(page.locator('.chat-list')).toBeVisible();
});
```

**依赖链:** `page.goto → toggle Debug → Quick Enter → waitForURL(/) → waitForSelector(.sidebar)`

**条件分支:**
- `Quick Enter (mock)` 不可点击 → 测试失败
- 未跳转到 `/` → `waitForURL` 超时 → 测试失败
- `.sidebar` 不存在 → `waitForSelector` 超时 → 测试失败
- `.chat-list` 不可见 → `toBeVisible` 断言失败

**内部触发的 Mock 方法:**
| 步骤 | Mock 方法 | 说明 |
|------|-----------|------|
| mockLogin | `enableMock()` | 启用 28 个 Mock 方法 |
| | `mockRegister` + `mockLogin` + `mockMe` | 创建 mock user 并返回 token |
| ChatPage mount | `mockListChats` | 获取聊天列表 |

---

#### `mock login shows chat list` (line 27)

```js
test('mock login shows chat list', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');
  await page.waitForSelector('.chat-item');
  const items = await page.locator('.chat-item').count();
  expect(items).toBeGreaterThanOrEqual(1);
});
```

**依赖链:** `page.goto → toggle Debug → Quick Enter → waitForURL → waitForSelector(.chat-item)`

**条件分支:**
- `.chat-item` 数量 < 1 → `toBeGreaterThanOrEqual(1)` 断言失败

---

#### `mock mode persists after page reload` (line 36)

```js
test('mock mode persists after page reload', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');
  await page.reload();
  await page.waitForSelector('.chat-item');
  const items = await page.locator('.chat-item').count();
  expect(items).toBeGreaterThanOrEqual(1);
});
```

**依赖链:** `page.goto → Quick Enter → page.reload → waitForSelector(.chat-item)`

**验证:** 刷新后 localStorage 中 `accessToken === 'mock-token'` → auth store 初始化时自动 `api.enableMock()` → ChatPage mount 调用 `mockListChats`

**条件分支:**
- 刷新后 `.chat-item` 不出现 → `waitForSelector` 超时
- `.chat-item` 数量 < 1 → 断言失败

---

#### `mock send message and see AI reply` (line 46)

```js
test('mock send message and see AI reply', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');

  await page.locator('.chat-item').first().click();
  await page.waitForSelector('.chat-input textarea');

  await page.fill('.chat-input textarea', 'Hello from CI!');
  await page.click('button:has-text("Send")');

  await expect(page.locator('.msg-content').first()).toContainText('Hello from CI!', { timeout: 5000 });
});
```

**依赖链:** `Quick Enter → click first chat → waitForSelector textarea → fill → click Send → toContainText`

**内部触发的 Mock 方法:**
| 步骤 | Mock 方法 | 说明 |
|------|-----------|------|
| 点击聊天 | `mockListMessages` | 获取该聊天的消息列表 |
| 点击 Send | `mockSendMessage` | 创建消息 + 触发 store.onMessageCreate + AI 自动回复（"Thanks for your message! 🤖"）|

**条件分支:**
- `.chat-item` 不存在 → `first().click()` 失败
- `.chat-input textarea` 不存在 → `waitForSelector` 超时
- 5 秒内 `.msg-content` 不包含 `Hello from CI!` → `toContainText` 超时断言失败

---

#### `mock notice board: set, edit, clear` (line 62)

```js
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
```

**依赖链:** `Quick Enter → click first chat → Set Notice → fill → Save → Edit → fill → Save → Clear`

**内部触发的 Mock 方法:**
| 步骤 | Mock 方法 | 说明 |
|------|-----------|------|
| 点击聊天 | `mockGetChat` | 获取聊天详情（含 pinnedMessage） |
| Save (Set) | `mockSetPinnedMessage` | POST /api/chats/{id}/pin + store.onChatUpdate |
| Save (Edit) | `mockSetPinnedMessage` | 同上，覆盖内容 |
| Clear | `mockClearPinnedMessage` | DELETE /api/chats/{id}/pin + store.onChatUpdate |

**条件分支:**
- 无 `+ Set Notice` 按钮 → 跳过测试（非 Owner 场景）
- Set 后 `📌 Notice:` 不出现 → `toBeVisible` 断言失败
- Edit 后 `Updated CI Notice` 不出现 → 断言失败
- Clear 后 `📌 Notice:` 仍可见 → `not.toBeVisible` 断言失败

---

#### `mock reaction buttons exist` (line 89)

```js
test('mock reaction buttons exist', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');

  await page.locator('.chat-item').first().click();
  await page.waitForSelector('.msg-content');

  await expect(page.locator('button.msg-btn:has-text("😀")').first()).toBeVisible({ timeout: 5000 });
});
```

**条件分支:**
- 5 秒内 `button.msg-btn:has-text("😀")` 不出现 → `toBeVisible` 断言失败

---

#### `logout from mock mode returns to login` (line 101)

```js
test('logout from mock mode returns to login', async ({ page }) => {
  await page.click('text=Debug mode');
  await page.click('text=Quick Enter (mock)');
  await page.waitForURL('/');

  const logoutBtn = page.locator('button:has-text("Log Out"), button:has-text("Logout")');
  if (await logoutBtn.isVisible()) {
    await logoutBtn.click();
    await page.waitForURL('/login');
    await expect(page.locator('h1')).toHaveText('Welcome back!');
  }
});
```

**内部触发的 Mock 方法:**
| 步骤 | Mock 方法 | 说明 |
|------|-----------|------|
| Logout | `mockLogout` | POST /api/auth/logout |
| | `api.disableMock()` | 恢复 28 个真实 API 方法 |
| | 清除 localStorage | accessToken/storage.user 被移除 |

**条件分支:**
- 无 Logout 按钮 → 跳过
- 点击 Logout 后未跳转到 `/login` → `waitForURL` 超时
- `h1` 内容不是 `Welcome back!` → `toHaveText` 断言失败

---

### 3.2 `e2e.spec.js` — E2E 全链路测试

---

#### `home redirects to login when not authenticated` (line 4)

```js
test('home redirects to login when not authenticated', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveURL(/\/login/);
});
```

**依赖链:** `page.goto(/) → 前端路由判断 accessToken 不存在 → Navigate to /login`

**条件分支:**
- URL 不包含 `/login` → `toHaveURL` 断言失败

---

#### `login form renders correctly` (line 9)

```js
test('login form renders correctly', async ({ page }) => {
  await page.goto('/login');
  await expect(page.locator('h1')).toHaveText('Welcome back!');
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
  await expect(page.locator('button:has-text("Log In")')).toBeVisible();
});
```

**条件分支:**
- 任一元素不匹配 → 对应 `toHaveText` / `toBeVisible` 断言失败

---

#### `register form renders correctly` (line 17)

```js
test('register form renders correctly', async ({ page }) => {
  await page.goto('/register');
  await expect(page.locator('h1')).toHaveText('Create an account');
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="text"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
});
```

**条件分支:**
- 任一元素不匹配 → 对应断言失败

---

#### `full auth flow` (line 25)

```js
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
```

**依赖链:** `page.goto(/register) → fill 3 fields → click Continue → waitForURL(/) → waitForSelector(.sidebar)`

**调用的后端端点:** `POST /api/auth/register`

**条件分支:**
- 未跳转到 `/` → `waitForURL` 超时
- `.sidebar` 不存在 → `waitForSelector` 超时

---

#### `create group chat` (line 36)

```js
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
```

**调用的后端端点:** `POST /api/auth/register` → `POST /api/chats`

**条件分支:**
- 注册失败 → `waitForURL(/)` 超时
- `.chat-header` 不包含 `E2E Group` → `toContainText` 断言失败

---

#### `send and receive message` (line 52)

```js
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
```

**调用的后端端点:** `POST /api/auth/register` → `POST /api/chats` → `POST */messages` → `GET */messages`

**条件分支:**
- `.chat-input textarea` 不存在 → `waitForSelector` 超时
- `.msg-content` 不包含 `Hello E2E` → `toContainText` 断言失败

---

#### `responsive layout on mobile` (line 72)

```js
test('responsive layout on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 667 });
  await page.goto('/login');
  await expect(page.locator('form.form-box')).toBeVisible();
});
```

**条件分支:**
- 375px 视口下 `form.form-box` 不可见 → `toBeVisible` 断言失败

---

#### `notice board functionality as owner` (line 78)

```js
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
```

**调用的后端端点:** `POST /api/auth/register` → `POST /api/chats` → `PUT/PATCH */pin` → `DELETE */pin`

**条件分支:**
- 无 `+ Set Notice` 按钮 → 跳过（创建者一定是 Owner，不应发生）
- Set 后 `📌 Notice:` 不出现 → 断言失败
- Edit 后 `Updated notice!` 不出现 → 断言失败
- Clear 后 `📌 Notice:` 仍可见 → 断言失败

---

## 四、Mock API 覆盖矩阵

| 类别 | 方法 | Mock 实现 | CI 测试覆盖 | E2E 覆盖 |
|------|------|----------|-----------|---------|
| **Auth** | register | ✅ | 间接 | ✅ |
| | login | ✅ | ✅ | — |
| | refresh | ✅ | — | — |
| | logout | ✅ | ✅ | — |
| **User** | me | ✅ | ✅ | — |
| | updateProfile | ✅ | — | — |
| | searchUsers | ✅ | — | — |
| **Chat** | listChats | ✅ | ✅ | — |
| | listPublicChats | ✅ | — | — |
| | createChat | ✅ | — | ✅ |
| | getChat | ✅ | ✅ | — |
| | deleteChat | ✅ | — | — |
| | renameChat | ✅ | — | — |
| | createDM | ✅ | — | — |
| | joinChat | ✅ | — | — |
| **Pin** | setPinnedMessage | ✅ | ✅ | ✅ |
| | clearPinnedMessage | ✅ | ✅ | ✅ |
| **Member** | addMember | ✅ | — | — |
| | removeMember | ✅ | — | — |
| **Message** | listMessages | ✅ | ✅ | — |
| | sendMessage | ✅ | ✅ | ✅ |
| | editMessage | ✅ | — | — |
| | deleteMessage | ✅ | — | — |
| | markRead | ✅ | — | — |
| **Reaction** | addReaction | ✅ | 间接（UI 验证） | — |
| | removeReaction | ✅ | — | — |
| **Upload** | upload | ✅ | — | — |
| | uploadAvatar | ✅ | — | — |

**未直接测试的方法（14 个）：** refresh, updateProfile, searchUsers, listPublicChats, deleteChat, renameChat, createDM, joinChat, addMember, removeMember, editMessage, deleteMessage, markRead, removeReaction, upload, uploadAvatar

---

## 五、后端端点覆盖率（E2E）

| 端点 | 覆盖 | 测试 |
|------|------|------|
| `POST /api/auth/register` | ✅ | `full auth flow`, `create group chat`, `send and receive message`, `notice board` |
| `POST /api/chats` | ✅ | `create group chat`, `send and receive message`, `notice board` |
| `POST */messages` | ✅ | `send and receive message` |
| `PUT/PATCH */pin` | ✅ | `notice board` |
| `DELETE */pin` | ✅ | `notice board` |

**未覆盖端点（27 个）：** login, refresh, logout, me, updateProfile, searchUsers, listChats, listPublicChats, getChat, deleteChat, renameChat, createDM, joinChat, addMember, removeMember, getMembers, listMessages, editMessage, deleteMessage, markRead, addReaction, removeReaction, upload, uploadAvatar, healthz, SSE, WebSocket

---

## 六、测试约束汇总

| 约束 | CI 测试 | E2E 测试 |
|------|---------|---------|
| 后端依赖 | 无（纯 Mock） | 需要（真实 Go 服务） |
| 数据隔离 | 每个 `page` 实例独立 localStorage | 每个测试独立注册（随机 email） |
| Mock 数据 | `resetMockData()` 每次 `enableMock()` 重置 | 不适用 |
| AI 回复 | 固定 `"Thanks for your message! 🤖"` | 不适用 |
| 状态恢复 | `accessToken === 'mock-token'` localStorage 持久化 | HttpOnly cookie |
| 超时 | 默认 5s（部分断言指定 `timeout: 5000`） | 默认 |
| 视口 | 默认 1280×720 | 默认 1280×720（`responsive layout` 测试除外 375×667） |

---

## 七、运行方法

```bash
# CI Mock 测试（纯前端，不依赖后端）
cd client
npx playwright test tests/ci.spec.js          # 9 个测试

# E2E 全链路测试（需后端运行）
cd client && npx playwright test tests/e2e.spec.js  # 8 个测试

# npm scripts
npm test              # → playwright test tests/ci.spec.js
npm run test:full     # → playwright test tests/e2e.spec.js
npm run test:all      # → 串行 CI + E2E

# CI 流水线（GitHub Actions）
# push → main → Job1: mock-test → Job2: full-e2e
```

### 运行时特征

```
CI Mock 测试：  ~30s   |  9 个测试  |  无后端依赖
E2E 测试：      ~60s   |  8 个测试  |  需后端运行
-------------------------------
合计：          ~90s   |  17 个测试
```