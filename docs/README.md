# 文档导航

本文档树是唯一权威文档来源。旧版文档（v0.9.4 之前）已归档至 `archive/legacy-20260731/`，不再维护。

## 指南（Guide）

| 文档 | 内容 |
|---|---|
| [guide/quickstart.md](guide/quickstart.md) | 5 分钟跑起来：环境、配置、构建、启动、验证 |
| [guide/deployment.md](guide/deployment.md) | 生产部署（nginx、环境变量）与发布流程（版本、CI） |
| [guide/development.md](guide/development.md) | 开发工作流：构建、CI、Mock 模式、代码约定 |

## 测试（Testing）

| 文档 | 内容 |
|---|---|
| [testing.md](testing.md) | 测试体系总纲：金字塔、运行命令、命名/断言/注释规范、同步审计 |
| [mock-strategy.md](mock-strategy.md) | Mock 三层体系：边界、数据流、历史迁移 |

## 架构（Architecture）

| 文档 | 内容 |
|---|---|
| [architecture/overview.md](architecture/overview.md) | 系统总览、请求链路、目录结构 |
| [architecture/backend.md](architecture/backend.md) | Go 后端分层（handlers/service/db/ws/ai）、中间件、配置 |
| [architecture/frontend.md](architecture/frontend.md) | 前端状态、实时协调器、Mock 模式 |
| [architecture/database.md](architecture/database.md) | 数据库 schema、迁移、设计决策 |
| [architecture/realtime.md](architecture/realtime.md) | WS / SSE / Poll 协议：事件、payload、SSE 行格式 |

## API 参考（API）

| 文档 | 内容 |
|---|---|
| [api/reference.md](api/reference.md) | 全部端点、认证、错误格式、上传响应 |
| [api/error-codes.md](api/error-codes.md) | 错误码表 |
| [api/rate-limiting.md](api/rate-limiting.md) | 限速规则 |

## 其他

| 文档 | 内容 |
|---|---|
| [security.md](security.md) | 安全：CSP、CORS、JWT、Cookie、真实 IP |
| [testing.md](testing.md) | 测试体系总纲（金字塔、命令、断言规范） |
| [mock-strategy.md](mock-strategy.md) | Mock 三层体系与边界 |
| [changelog.md](changelog.md) | 修改日志（从 v0.9.4 起，末尾追加） |

## Agent 上下文

- 根目录 `AGENTS.md` — AI 代理项目约定（构建/测试/CI/CD）
- `.claude/AGENT.md` — 英文快速指南（结构 + 链接同上）

## 变更日志规则

- 每次代码修改后，在 `docs/changelog.md` **末尾**追加条目。
- 归档目录只进不出：任何被新文档替代的内容移入 `archive/legacy-*/`，不删除历史。
