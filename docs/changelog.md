# 修改日志

从 v0.9.4 起。旧日志见 `docs/archive/legacy-20260731/changelog-20260731.md`。

规则：**始终在末尾追加新条目**，不修改历史。

## 2026-08-01 文档系统翻新（v0.9.4）

### 变更
- 旧文档全部归档至 `docs/archive/legacy-20260731/`（README/NOTE/LOCAL_DEPLOYMENT/api.md/changelog/features/reference/reports/colab/todo/ppt 等 60+ 文件）。
- 新文档结构：
  - `README.md` — 项目总览（功能/技术栈/目录/快速开始）
  - `docs/guide/` — quickstart（快速开始）、deployment（生产部署+发布）、development（开发工作流）
  - `docs/architecture/` — overview / backend / frontend / database / realtime
  - `docs/api/` — reference（端点全表）/ error-codes / rate-limiting
  - `docs/security.md` — CSP/CORS/JWT/上传/SSRF/IP
  - `docs/changelog.md` — 新日志（从本条目起）
- 修正旧文档中已失效的信息：
  - 删除已不存在的 `UploadPublicURL`、`CHAT_AI_*`（AI 端点改由消息 `src` 携带，服务端仅 SSRF 校验）
  - 修正 `/api/chats` → `/api/chats/my`、`POST /api/uploads` → `/api/upload`、`/uploads/*` → `/api/local/*`
  - 修正 access token 时长（15 分钟 → 默认 30m）、上传响应（201 → 200，含 `path`/`url`/`delete_url`）
  - 修正 SSE 行格式（仅 ready 带 `event:`/`id:`，其余裸 `data:`）
  - 补全限速表（upload 组 60/min）、数据库动态补列（ensureColumn）、迁移 001-004
  - 前端架构更新为现状（api 层 TS 化、realtime/ 协调器、Mock 模式、Notifications 置顶）
- `.env.example` 与 `config.go` 对齐（移除 AI_*/UPLOAD_PUBLIC_URL，新增 CHAT_AI_ALLOW_PRIVATE/CHAT_CSP_CONNECT_SRC/CHAT_UPLOAD_SALT/CHAT_MAX_UPLOAD）。

### 验证
- 文档链接交叉验证（README ↔ docs 导航 ↔ 各文档）：✅
- 内容与代码核对（router.go / config.go / migrations / sse.go / hub.go / local_upload.go）：✅
