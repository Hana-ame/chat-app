# 待办 & 完成情况

## 已完成

- [x] **拖动条控制发送量** — slider 已实现（0–100），`doSendAI` 使用 `f.contextLimit` 正确传参
- [x] **prompt 被发送两次** — 前端 `handleSend` 无并发保护，快速按两次 Enter 会同时发送两条消息。已加 `sending` guard + 按钮 disabled
- [x] **手机点任何 chat 都进入 notify** — redirect effect 原缺少 `!isMobile`，commit `27f4ca2` 已修复
- [x] **非 AI stream 消息不可见** — `onMessageCreate` 设 `streaming: true`，`onMessageUpdate` 剥离 Go 零值字段，`fetchStream` 完成后清理
- [x] **AI request failed 无具体原因** — 所有前端 `notify()` + 后端 `writeError` 现在包含具体错误原因（2026-07-29 第 22 轮）
- [x] **发送失败内容恢复** — `handleSend` 保存 `savedText`/`savedAttachments`，失败后重新拼接复原
- [x] **发送失败文本翻倍** — 原 `setText(text ? savedText + text : savedText)` 闭包中 `text` 是旧值，导致文本翻倍。改为 `setText(prev => prev || savedText)` 使用 functional updater 读最新 state（2026-07-29）
- [x] **其他客户端看不到 AI stream** — `fetchStream` 用 `m.content + json.content` 追加增量，与 WS `onMessageUpdate` 的全量替换冲突导致内容错乱。改为本地积累 `contentAcc` 后用全量替换（同 WS 一致），消除竞态（2026-07-29）
- [x] **API 文档** — `docs/api.md` 已完整列举所有端点、请求体、响应体、错误码、实时事件格式（2026-07-29）
