# Documentation

Technical specifications, feature breakdowns, and development references for the WebChat App.

## Release Checklist

每次推送前检查：
1. **版本号** — `client/package.json` 的 `version` 是否已递增？
2. **构建** — `npm run build` 是否通过
3. **后端测试** — `go test ./...` 是否通过
4. **修改日志** — [`changelog.md`](./changelog.md) 是否已追加本次变更

## Directory

```
docs/
├── README.md               # 本文档
├── changelog.md            # 修改日志
├── quickstart.md           # 快速开始（环境变量、运行、测试）
├── deployment-guide.md     # 部署指南
├── features/               # 功能特性 + API 文档
│   ├── toc.md
│   ├── visibility.md       # 可见性等级
│   ├── search.md           # 搜索机制
│   ├── create-group.md     # 创建群组
│   ├── chat-list.md        # 侧栏逻辑
│   ├── ai-stream.md        # AI 流式输出
│   ├── add-member.md       # 成员管理
│   ├── go-api-routes.md    # Go 路由表
│   ├── go-api-models.md    # 请求/响应模型
│   ├── api-endpoints.md    # 前端 API 概览
│   ├── mock-vs-go-api-report.md  # Mock vs Go 对照
│   └── frontend-architecture.md  # 前端架构
├── reports/                # 架构规格（见 index.md）
├── reference/              # 跨维度参考文档
│   ├── security.md         # 安全模型（CSP、CORS、JWT、Cookie）
│   ├── rate-limiting.md    # 速率限制
│   ├── realtime-protocol.md # WS/SSE 实时协议格式
│   ├── error-codes.md      # 错误码参考
│   └── database.md         # 数据库 Schema
└── archive/                # 已归档
    ├── reports/            # 39 份过时报告
    ├── sessions/           # 9 份会话文档
    └── temp/               # 6 个临时文件
```
