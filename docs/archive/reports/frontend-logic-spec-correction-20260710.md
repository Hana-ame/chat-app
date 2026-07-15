# frontend-logic-spec 逐行审计修正报告

> 审计日期：2026-07-10
> 审计方法：将 `frontend-logic-spec-20260710.md` 每一条声明与 9 个源文件逐行对照
> 源文件：client.js (240L), mock.js (456L), auth.js (92L), chat.js (361L), stream-source.js (21L), mock-ws.js (99L), dummy.js (352L), App.jsx (34L), ChatPage.jsx (80L)

---

## 统计

| 严重性 | 数量 |
|--------|------|
| CRITICAL — 完全虚构（函数/数据结构/章节不存在） | 6 |
| HIGH — 显著行为差异（错误逻辑、错误条件） | 8 |
| MEDIUM — 描述不完整/有误导性 | 8 |
| LOW — 轻微不准确 | 7 |
| **合计** | **29** |

---

## CRITICAL — 完全虚构

### C1. 数据结构注释虚构（spec line 199）

**声明：** `let data = null; // { users, chats, messages, reactions, chatMembers }`
**实际（mock.js:43,68）：** `let data = null;` 无注释。实际数据结构为 `{ chats, messages }`，`users`、`reactions`、`chatMembers` 字段不存在。
**修正：** 改为 `// { chats, messages } — 由 generateDummyData() 生成`

### C2. "双数据源问题"章节完全虚构（spec lines 322-329）

**声明：** Mock 中 chat 成员同时存在于 `c.members` 和 `d.chatMembers`（扁平数组），两处靠手写同步，`buildChatResponse` 用 `d.chatMembers` 算 `member_count`。
**实际：** `d.chatMembers` 在整个代码库中不存在（grep 确认 0 匹配）。`mockAddMember`/`mockRemoveMember` 直接操作 `chat.members`，`buildChatResponse` 函数不存在。
**修正：** 标记为已确认误报，引用 `frontend-logic-correction-20260710.md`。

### C3. 跨模块问题 #4 虚构（spec line 758）

**声明：** "Mock 双数据源（c.members vs d.chatMembers）"，严重性"中"，状态"⏭️ mock 模式特有"。
**实际：** `d.chatMembers` 不存在，问题为幻觉。
**修正：** 标记为"✅ 已确认为误报（d.chatMembers 不存在）"。

### C4. AI 回复内容选择虚构（spec lines 309-310）

**声明：** `const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)]`（从池中随机选取）
**实际（mock.js:243）：** `const text = AI_RESPONSES[0]`（始终使用第一条回复）
**修正：** 改为 `AI_RESPONSES[0]`，注释说明"始终使用第一条回复"。

### C5. AI 回复触发条件虚构（spec line 309）

**声明：** `if (Math.random() < 0.5)` 包裹 AI 回复块（50% 概率触发）
**实际（mock.js:243-291）：** AI 回复块无条件执行，每次发消息都触发。无 `Math.random()` 判断。
**修正：** 删除 `if (Math.random() < 0.5)`，注释说明"100% 触发，无随机判断"。同步修正差异表中"AI 回复 50% 触发"为"AI 回复 100% 触发"。

### C6. mockDeleteChat 行为描述错误（spec line 257）

**声明：** "从 d.chats 过滤掉 → 从 d.messages 过滤掉该 chat 的消息"
**实际（mock.js:152-157）：** 仅过滤 `d.chats`，**不触碰** `d.messages`。已删除 chat 的消息残留内存。
**修正：** 改为"从 d.chats 过滤掉（**注意：不清理 d.messages，已删除 chat 的消息残留内存**）"。

---

## HIGH — 显著行为差异

### H1. 群组数量错误（spec line 612）

**声明：** "8 个命名群组（General, Random, Dev Team, Gaming, Music Club, Movie Night, Food & Cooking, Travel Pics）"
**实际（dummy.js:21-215）：** GROUP_TOPICS 有 **9** 个键，多出 "Pet Lovers"。
**修正：** 改为"9 个命名群组"，补充 "Pet Lovers"。

### H2. 群组聊天特殊消息索引错误（spec lines 614-618）

**声明：** `mi === 2` → deleted, `mi === 5` → edited
**实际群组代码（dummy.js:320,328）：** `mi === 1` → deleted, `mi === 3` → edited
**修正：** 区分 DM 和群组的不同索引。

### H3. DM 聊天反应模式遗漏（spec line 618）

**声明：** 仅列出 `mi > 5 && mi % 3 === 0` → reactions（单一模式）
**实际 DM 代码（dummy.js:274）：** `mi > 10 && mi % 5 === 0` → reactions（不同条件）
**实际群组代码（dummy.js:336-343）：** `mi > 5 && mi % 3 === 0` → reactions（但 General 用双 emoji，其他用单 emoji）
**修正：** 分别列出 DM 和群组的反应模式，标注 emoji 差异。

### H4. DM 附件模式完全遗漏（spec line 617）

**声明：** `mi === 4` → attachments（仅描述群组模式）
**实际 DM 代码（dummy.js:280-291）：** 最后 15 条消息获得附件，交替使用 alice-photo.jpg 和 dummyFiles。
**修正：** 补充 DM 附件逻辑说明。

### H5. 移动端检测是一次性检查（spec line 725）

**声明：** `const [isMobile] = useState(() => window.innerWidth < 768)`（一次性检查）
**实际（ChatPage.jsx:17-23）：** `const [isMobile, setIsMobile] = useState(window.innerWidth < 768)` 加 resize listener 动态更新。
**修正：** 补充 `setIsMobile` 和 resize effect，说明"动态响应窗口大小变化"。

### H6. listChats API 路径错误（spec line 140）

**声明：** `listChats` → `GET /api/chats/my`
**实际（client.js:87）：** `GET /api/chats`（无 `/my` 后缀）
**修正：** 改为 `GET /api/chats`。

### H7. msgPerChat 默认值错误（spec lines 43, 203, 610）

**声明：** "10 chat × 65 msg"（三处出现）
**实际（mock.js:67）：** `generateDummyData({ chatCount: 10, msgPerChat: 150 })`，mock.js 调用时覆盖为 150。
**修正：** 改为"10 chat × 150 msg"，补充说明 dummy.js 默认参数为 65 但 mock.js 覆盖为 150。

### H8. mock-ws.js presence_update 绕过 Zustand（spec line 590）

**声明：** "useChatStore.getState() → dispatch to handler"（暗示正确调用 handler）
**实际（mock-ws.js:78-79）：** `store.onlineUserIds = payload;` 直接赋值，绕过 `set()`，不触发组件重渲染。
**修正：** 添加警告说明直接赋值问题。

---

## MEDIUM — 描述不完整/有误导性

### M1. mock 函数数量错误（spec line 36）

**声明：** "28 个 mock 函数"
**实际（client.js:190-220）：** MOCKABLE 数组有 29 个条目（含 mockTogglePin）。
**修正：** 改为"29 个 mock 函数"。

### M2. stream-source.js 代码片段不完整（spec lines 553-561）

**声明：** 代码片段省略 `let onDone = null; let onError = null;` 声明，且 `onDone()`/`onError(err)` 无守卫。
**实际（stream-source.js:3-4,14-15）：** 有变量声明，且 `if (onDone) onDone()` 和 `if (onError) onError(err)` 有守卫防止未设置时出错。
**修正：** 补全完整代码。

### M3. mockListChats icon_color 描述有误导性（spec line 253）

**声明：** "分配 icon_color"（暗示总是分配新值）
**实际（mock.js:94）：** `icon_color: c.icon_color || CHAT_COLORS[i % CHAT_COLORS.length]`（优先使用已有值，无则回退）
**修正：** 改为"保留/回退 icon_color（优先使用已有值，无则从 CHAT_COLORS 分配）"。

### M4. 路由表顺序与代码不一致（spec lines 643-649）

**声明：** 表中顺序为 /login, /register, /, /g/:chatId, /*
**实际（App.jsx:27-32）：** 声明顺序为 /login, /register, /*, /g/:chatId（`/*` 在 `/g/:chatId` 之前）
**修正：** 按实际顺序排列，修正说明为"React Router 中 `/*` 匹配所有未匹配路径，`/g/:chatId` 仍能匹配"。

### M5. mock-ws.js 调用上下文错误（spec line 572）

**声明：** "被 chat.js 的 connectWS() 引用（Mock 模式下替代真实 WebSocket）"
**实际：** chat.js 的 `connectWS()` 始终创建真实 `new WebSocket(url)`，不导入或调用 mock-ws.js。mock-ws.js 是独立调试工具。
**修正：** 改为"独立调试工具，不被 chat.js 直接引用"。

### M6. connectWS 代码片段过度简化（spec lines 470-491）

**声明：** 简化版 switch cases 遮蔽了实际逻辑（如 ready case 的 onlineUserIds 设置、presence_update 的 Set 操作、user_update 的 members 映射）。
**实际（chat.js:48-84）：** 包含更复杂的条件逻辑。
**修正：** 标注代码片段为简化版，提醒读者参照源码。

### M7. 差异表 AI 回复率描述（spec line 211）

**声明：** "AI 回复 50% 触发"（在差异表中列为"预期行为"）
**实际：** 100% 触发（已在 C5 修正）。
**修正：** 改为"AI 回复 100% 触发"。

### M8. dummy.js 描述未区分 DM 和群组逻辑（spec line 598）

**声明：** 描述为统一数据生成，未提及 DM 和群组使用不同模板和边界条件。
**实际（dummy.js:255-291 vs 315-346）：** DM 使用简短对话、不同附件模式、不同反应条件；群组使用主题对话。
**修正：** 补充 DM 和群组的差异说明。

---

## LOW — 轻微不准确

### L1. 数据规模计算错误（spec line 610）

**声明：** "~650 条消息"
**实际：** msgPerChat=150（mock.js 覆盖），实际 ~1500 条。
**修正：** 已在 H7 中一并修正。

### L2. mockCreateDM Store 通知遗漏（spec line 259）

**声明：** "查找已有 DM → 存在则返回 → 不存在则创建新 DM"（Store 通知列为空）
**实际（mock.js:178）：** 创建新 DM 后调用 `onChatUpdate(newDM)`。
**修正：** 补充 Store 通知 `onChatUpdate(newDM)`。

### L3. mockJoinChat 错误返回遗漏（spec line 260）

**声明：** "查找 chat → 检查可见性 → 添加当前用户为 member"
**实际（mock.js:372-374）：** 非 public/unlisted chat 返回 `{ error: 'private' }`。
**修正：** 补充"检查可见性（非 public/unlisted 返回 `{error:'private'}`）"。

### L4. auth:unauthorized 处理器代码片段不完整（spec lines 654-662）

**声明：** 独立的 `window.addEventListener` 代码片段。
**实际（App.jsx:14-24）：** 在 `useEffect` 内，有 cleanup（`removeEventListener`），用 `loggingOut` ref 做 500ms 防抖。
**修正：** 替换为包含 useEffect 和 cleanup 的完整代码。

### L5. ChatPage 消息加载代码片段过度简化（spec lines 710-715）

**声明：** 简化版代码，直接使用 `messages.length` 和 `lastMsg.id`。
**实际（ChatPage.jsx:42-51）：** 使用 `useChatStore.getState().messages` 读取，过滤 `chat_id === activeChatId && !deleted`，取最后一条的 `id`。
**修正：** 替换为实际代码。

### L6. swap() console.log 噪声（spec line 182）

**声明：** "swap() 中 console.log 对每个调用都输出，mock 模式下控制台噪声大"
**实际（client.js:178-188）：** 确实每个调用都输出 `[Mock API] xxx(` 和 `=>` 日志。
**修正：** 无需修正，声明准确。

### L7. DmSearchPanel 死代码（spec line 759）

**声明：** "DmSearchPanel 未被引用"
**验证：** 需检查 DmSearchPanel.jsx 是否存在及是否被导入。本次审计范围未包含此文件，保留原声明。

---

## 修正前后对比（关键段落）

### 数据结构注释

```diff
- let data = null;   // { users, chats, messages, reactions, chatMembers }
+ let data = null;   // { chats, messages } — 由 generateDummyData() 生成
```

### AI 自动回复

```diff
- if (Math.random() < 0.5) {  // ← 50% 概率触发 AI
-   const text = AI_RESPONSES[Math.floor(Math.random() * AI_RESPONSES.length)];
+ // 每次发送都触发 AI 回复（100% 触发，无随机判断）
+ const text = AI_RESPONSES[0];  // ← 始终使用第一条回复
```

### mockDeleteChat

```diff
- | `mockDeleteChat` | 从 d.chats 过滤掉 → 从 d.messages 过滤掉该 chat 的消息 |
+ | `mockDeleteChat` | 从 d.chats 过滤掉（**注意：不清理 d.messages，已删除 chat 的消息残留内存**） |
```

### 特殊消息

```diff
- **特殊消息：**
- - `mi === 2` → deleted（空内容）
- - `mi === 5` → edited（设 edited_at）
- - `mi === 4` → attachments（2 个文件）
- - `mi > 5 && mi % 3 === 0` → reactions
+ **特殊消息（DM 聊天 vs 群组聊天不同）：**
+
+ DM 聊天（dummy.js:255-291）：
+ - `mi === 2` → deleted（空内容）
+ - `mi === 5` → edited（设 edited_at）
+ - 最后 15 条消息 → attachments（交替使用 alice-photo.jpg 和 dummyFiles）
+ - `mi > 10 && mi % 5 === 0` → reactions（👍, count=2）
+
+ 群组聊天（dummy.js:315-346）：
+ - `mi === 1` → deleted（空内容）
+ - `mi === 3` → edited（设 edited_at）
+ - `mi === 4` → attachments（photo.png + document.pdf）
+ - `mi > 5 && mi % 3 === 0` → reactions
+   - General (ci===0)：👍 + 🎉（双 emoji）
+   - 其他群组：😂（单 emoji）
```

### 路由表

```diff
- | `/` | `ChatPage` | 已登录，否则 → `/login` |
- | `/g/:chatId` | `ChatPage` | 已登录，否则 → `/login` |
- | `/*` | `ChatPage` | 已登录，否则 → `/login` |
+ | `/*` | `ChatPage` | 已登录，否则 → `/login` |
+ | `/g/:chatId` | `ChatPage` | 已登录，否则 → `/login` |
```

---

## 教训总结

1. **AI 生成的文档必须逐行对照源码验证。** 本次发现的 6 个 CRITICAL 问题（虚构的数据结构、虚构的函数名、虚构的条件分支）全部是 AI 自行编造的内容，源码中完全不存在。

2. **"双数据源"是典型的 AI 幻觉。** `d.chatMembers` 这个字段名听起来合理（chat 有 members，应该有个 chatMembers 表），但实际上 mock.js 只用了 `chat.members`（嵌套在 chat 对象中），从未有过扁平化的 chatMembers 数组。

3. **条件分支容易被 AI 简化或编造。** `Math.random() < 0.5` 和 `AI_RESPONSES[randomIndex]` 听起来很合理（Mock 模式嘛，随机一点），但实际代码是无条件触发 + 固定索引。

4. **DM 和群组的差异容易被忽略。** dummy.js 中 DM 和群组使用完全不同的消息模板、特殊索引和附件模式，AI 文档将它们混为一谈。

5. **数字要逐一核实。** 群组数量（8→9）、消息数量（65→150）、mock 函数数量（28→29）——每一个数字都可能是错的。
