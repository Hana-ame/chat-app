# 对话历史

## 2026-04-26 ~ 04-27

### 文档翻译 + 启动说明
- `guide.md` 和 `ARCHITECTURE.md` 翻译成中文
- 添加如何运行说明

### vsCode 配置
- `.vscode/settings.json` 添加 `python.defaultInterpreterPath` 指向 miniconda3

### 前端 Bug 修复
- `loadHistory` 缺 `token` 参数 → 422
- `startPolling` 闭包 stale `logout`
- Vite 开启 sourcemap

### WS 问题修复
- `ws.transport` AttributeError → `getattr` 安全访问
- WS 广播发二进制帧 → 改 `send_text`
- WS 收发测试 (4 个集成测试)

### 文件上传 (upload.moonchan.xyz)
- 参考 `忘记说了.txt` 接入 `https://upload.moonchan.xyz`
- API: `PUT /api/upload` → `{"id","key"}` / `GET /api/{id}/{name}` / `DELETE /api/delete/{id}/{key}`
- 后端 `msg_type` 支持 (REST + WS + db)
- React SPA: 粘贴/拖拽/按钮上传 → 图片预览/文件链接
- Widget: 同上

### Cloudflare Pages CORS
- `chat-app-fastapi.pages.dev` 是静态站点，API 请求 405
- SPA 加 `API_BASE`/`WS_BASE` 自动检测 pages.dev → 指向 `wsl-8000.moonchan.xyz`

### 测试完善
- 从 17 个 → 29 个测试
- +8 个 CORS 测试（所有端点 + 401 也返回 CORS 头）
- +4 个 WS 集成测试 (发送/接收、文本帧、ping/pong、双连接)
- `TEST_DOC.md` 测试文档

### 在线人数
- `online_count` 嵌入 system 消息，移除重复广播
- WS 进/退房时自动更新

### Widget 调试
- `setAttribute` 无限递归 → 改用 `document.createElementNS` 创建 SVG
- 窗口不跟随气泡 → 动态 `repositionWin()`
- z-index 交换
- 滚动条不显示: `min-height:0` + `overflow-y:scroll` + 亮色 thumb
- 演示页上传到 upload.moonchan.xyz

### Git 管理
- `client/dist/` 移入 `.gitignore` (Cloudflare Pages 自动构建)
- 本地 `bash build_client.sh` 服务 `wsl-8000`

### 当前状态
- 29/29 测试全过
- Widget 可通过 `<script>` 嵌入任意页面
- 演示页: upload.moonchan.xyz 多个版本
