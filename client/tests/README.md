# tests — 测试目录

## 文件说明

| 文件 | 类型 | 用途 |
|------|------|------|
| `e2e.spec.js` | Playwright E2E | 前端核心流程测试（登录、注册、建群、发消息、响应式布局） |
| `upload_test.sh` | Bash + curl | 外部上传服务 `upload.moonchan.xyz` 的可用性测试 |

---

## 前置条件

- **Go 1.21+**（运行后端）
- **Node 18+**（运行前端 dev server 或 build）
- **curl**（`upload_test.sh` 需要）
- 外部服务域名 `upload.moonchan.xyz` 可解析

---

## 运行方式

### E2E 测试 (Playwright)

```bash
# 安装 playwright 依赖（首次）
cd client && npx playwright install chromium

# 运行测试（自动启动 Go 后端）
cd client && npx playwright test
```

配置见 `client/playwright.config.js`:
- 后端启动命令: `go run server/cmd/chatd`
- 默认连接 `http://localhost:8080`

### 上传服务测试

```bash
./client/tests/upload_test.sh
```

脚本测试:
1. 上传文本文件 → 返回 `id`
2. 上传 PNG 图片 → 返回 `id`
3. OPTIONS 预检 → CORS 正常

---

## 注意事项

- E2E 测试依赖 mock 无用户的后端数据库（独立 SQLite），不会影响真实数据。
- `upload_test.sh` 依赖外部服务 `upload.moonchan.xyz`；若该服务不可用或变更 API，测试可能失败。
- 新增测试文件时，Playwright 会自动发现 `tests/**/*.spec.{js,ts}` 下的文件。

---

## 修改记录

| 日期 | 变更 |
|------|------|
| 2026-07-06 | 创建 `upload_test.sh`：测试外部上传服务的可用性 |
| 2026-07-06 | 创建 `README.md` |
