# 组件组合方式 Review

## 总览

```
App (Routes)
 └─ ChatPage (路由分发 + 连接管理)
    ├─ ChatList        (sidebar: 聊天列表 / 创建 / DM / 设置 / 公共群组 / ContextMenu)
    ├─ ChatView        (中间: 消息列表 + 输入区)
    │  ├─ MessageItem  (单条消息: 文本, 附件, 编辑, 反应, 操作菜单)
    │  └─ Composer     (文本输入 + 文件上传)
    └─ MemberPanel     (右侧: 成员列表, 添加/踢出)
```

---

## 逐项分析

### ✅ ChatView — 合理

职责单一：加载消息、滚动加载历史、组合 MessageItem + Composer。唯一问题：`getDMName` 与 `ChatList` 重复定义。

### ✅ Composer — 合理

处理文本输入 + 文件上传，粒度合适。`handleFile` 直接调用 `api.upload` 并存储 attachments，未过度抽象。

### ✅ MessageItem — 边界略宽

在一组件内处理：渲染、编辑（inline input+save/cancel）、删除、Reaction 切换、Emoji picker 展开。可以拆出 `MessageEdit` 和 `ReactionPicker`，但目前体量（116 行）尚可接受。

### ✅ MemberPanel — 合理

成员列表 + 搜索添加 + 踢出，与业务高度绑定，独立有意义。但搜索成员逻辑与 `ChatList` 中的 DM 搜索高度重复。

### ⚠️ ChatList — 职责过重 (313 行)

目前在一个组件内完成了：

| 功能 | 应否拆分 |
|------|----------|
| 聊天列表渲染 | ✓ 核心职责 |
| 创建群组弹窗 | 可保留（小） |
| DM 搜索 + 结果 | 与 MemberPanel 搜索重复 |
| 公共群组列表 | 可保留 |
| **设置弹窗（含头像上传、改名）** | **应拆为 `SettingsModal`** |
| 右键菜单 (pin/delete) | 可保留 |

**建议**：将 `SettingsModal` 抽为独立组件，DM 搜索逻辑与 MemberPanel 合并为共用 hook。

### ⚠️ ChatPage — 混合路由与连接逻辑

ChatPage 同时做了：
- 路由参数 `/g/:chatId` 响应
- 实时连接（WS/SSE/Poll）的生命周期
- 移动端判断

可将连接管理抽取为 Hook：

```js
// hooks/useRealtime.js
function useRealtime(token, mode) { ... }
```

移动端判断也可抽取：

```js
// hooks/useMobile.js
function useMobile() { ... }
```

---

## 代码重复

| 重复片段 | 位置 |
|---------|------|
| `getDMName` 函数 | `ChatList.jsx:17-20`, `ChatView.jsx:8-12` |
| 用户搜索 (`api.searchUsers` + 过滤) | `ChatList.jsx:74-82`, `MemberPanel.jsx:20-25` |
| 头像渲染（img 退回到首字母） | 出现在 4+ 个文件，可抽象为 `<UserAvatar>` |

**建议**：抽出 `UserAvatar` 组件 + `useUserSearch` hook。

---

## 缺失组件

| 缺少 | 说明 |
|------|------|
| `UserAvatar.jsx` | 统一处理头像图片 + 首字母 fallback + 颜色背景 |
| `SettingsModal.jsx` | 提取 ChatList 中的设置弹窗 |
| `MessageEdit.jsx` | MessageItem 内编辑部分的 input + button 组 |
| `ReactionPicker.jsx` | Emoji 选择弹窗 |

---

## 修改记录

| 日期 | 变更 |
|------|------|
| 2026-07-06 | 创建本 Review |
