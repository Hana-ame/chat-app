# 文档导航

本文档树是唯一权威文档来源。旧版文档（v0.9.4 之前）已归档至 `archive/legacy-20260731/`，不再维护。

## Agent 首轮会话路径

1. 读根目录 `README.md`（产品概览）
2. 读本文档（索引，唯一权威入口）
3. 读 `changelog.md` 末尾 3 条（最近改动，避免重做）
4. 按需深入：`architecture/` → `api/` → `testing.md` / `mock-strategy.md`

## 指南（Guide）

| 文档 | 何时读 | 内容 |
|---|---|---|
| [guide/quickstart.md](guide/quickstart.md) | 新环境要跑起来 | 5 分钟跑起来：环境、配置、构建、启动、验证 |
| [guide/deployment.md](guide/deployment.md) | 要部署/发版 | 生产部署（nginx、环境变量）与发布流程（版本、CI） |
| [guide/development.md](guide/development.md) | 日常开发 | 开发工作流：构建、CI、Mock 模式、代码约定 |

## 测试（Testing）

| 文档 | 何时读 | 内容 |
|---|---|---|
| [testing.md](testing.md) | 要写/跑测试 | 测试体系总纲：金字塔、运行命令、命名/断言/注释规范、同步审计 |
| [mock-strategy.md](mock-strategy.md) | 改 mock 数据或 API 字段 | Mock 三层体系：边界、数据流、历史迁移 |

## 架构（Architecture）

| 文档 | 何时读 | 内容 |
|---|---|---|
| [architecture/overview.md](architecture/overview.md) | 首次了解系统 | 系统总览、请求链路、目录结构 |
| [architecture/backend.md](architecture/backend.md) | 改后端代码前 | Go 后端分层（handlers/service/db/ws/ai）、中间件、配置 |
| [architecture/frontend.md](architecture/frontend.md) | 改前端代码前 | 前端状态、实时协调器、Mock 模式 |
| [architecture/database.md](architecture/database.md) | 改 DB/schema 前 | 数据库 schema、迁移、设计决策 |
| [architecture/realtime.md](architecture/realtime.md) | 改实时协议前 | WS / SSE / Poll 协议：事件、payload、SSE 行格式 |

## API 参考（API）

| 文档 | 何时读 | 内容 |
|---|---|---|
| [api/reference.md](api/reference.md) | 查端点/改接口 | 全部端点、认证、错误格式、上传响应 |
| [api/error-codes.md](api/error-codes.md) | 报错时查码 | 错误码表 |
| [api/rate-limiting.md](api/rate-limiting.md) | 撞限流/加限流 | 限速规则 |

## 其他

| 文档 | 何时读 | 内容 |
|---|---|---|
| [security.md](security.md) | 涉及安全配置 | 安全：CSP、CORS、JWT、Cookie、真实 IP |
| [changelog.md](changelog.md) | 每次改动后 | 修改日志（从 v0.9.4 起，末尾追加） |

## Agent 上下文

- 根目录 `AGENTS.md` — AI 代理项目约定（唯一来源：会话仪式/构建/测试/CI/CD）

## 变更日志规则

- 每次代码修改后，在 `docs/changelog.md` **末尾**追加条目。
- 归档目录只进不出：任何被新文档替代的内容移入 `archive/legacy-*/`，不删除历史。
- changelog 每满 30 轮归档一次：把最旧的轮次移入 `archive/`，保持主文件可读。
