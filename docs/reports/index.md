# 报告索引 (Reports Index)

## 规范文档

| 文件名 | 说明 |
|---|---|
| [**models-data-spec-20260708**](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU72COJ2S7VSXFGKKGNF7DMUX7ZT/models-data-spec-20260708.md.gz&markdown=true) | User, Chat, Message 等核心模型字段、生成规则、约束 |
| [**config-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU3BQ46VWIQIPBEKRNCM7RXSJLJL/config-spec-20260709.md.gz&markdown=true) | 9 项配置参数表 + `Load()` 加载逻辑 |
| [**auth-logic-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU7GEY6Q7TUNQVHINVU7XZVURX6F/auth-logic-spec-20260709.md.gz&markdown=true) | JWT 签发/解析、bcrypt 密码哈希、refresh token |
| [**ws-architecture-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU3PO57QRWPMEVGJ6LYREN4XSICZ/ws-architecture-spec-20260709.md.gz&markdown=true) | Gateway/Client/Hub 架构 + Envelope 协议 |
| [**ws-client-gateway-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUUZUMHC3KQGDDBE3V6GEW2SES6JB/ws-client-gateway-spec-20260709.md.gz&markdown=true) | client.go + gateway.go：连接生命周期、read/write pump |
| [**sse-event-stream-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUUZW4N3DH3A7PZDL26V2FSXMUFAR/sse-event-stream-spec-20260709.md.gz&markdown=true) | SSE 连接、事件格式、全量广播事件表 |
| [**api-handlers-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU5JNNZBGN54GVG3BQ4ZS6LDRMY2/api-handlers-spec-20260709.md.gz&markdown=true) | 35 个 handler 的源码、依赖链、条件分支 |
| [**db-base-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU6OQQBRWZMZ6RDLJO5Q6WHF6WSV/db-base-spec-20260709.md.gz&markdown=true) | db.go：DSN 参数、连接初始化、迁移运行器 |
| [**db-dao-api-reference-20260708**](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUUYCE66GSBOBUBFY65FJDLNEAKAX/db-dao-api-reference-20260708.md.gz&markdown=true) | 所有 DAO 方法的 SQL 实现及输入输出 |
| [**test-suite-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU34BMS3M5FPINDKIZBNXA2QZ33C/test-suite-spec-20260709.md.gz&markdown=true) | 52 个测试的源码、依赖链、条件分支 |
| [**testutil-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU442ETNIFFYEJCYG7LNLMHZJN6B/testutil-spec-20260709.md.gz&markdown=true) | Fixture 初始化、Session、Do/Register/Login 方法 |
| [**orderedmap-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU5HCBXPWR5T7FD3HV6YKIWH4TYK/orderedmap-spec-20260709.md.gz&markdown=true) | 有序 JSON map 实现，仅用于 /healthz |
| [**cmd-main-spec-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU366OVB2JWTHBF2G72QRS2W3VFS/cmd-main-spec-20260709.md.gz&markdown=true) | main.go 初始化流程、优雅关闭、HTTP 超时配置 |

## 独立文档

| 文件名 | 说明 |
|---|---|
| [**backend-code-review-20260708**](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU2WWOAAOV44WRHZFDI7BVKJM4CP/backend-code-review-20260708.md.gz&markdown=true) | 安全风险、性能瓶颈、并发优化修复 |
| [**reaction-flow-report-20260708**](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNCFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU762IKELTI2H5E3X5PWP77EGBO2/reaction-flow-report-20260708.md.gz&markdown=true) | 反应功能原始存储 → 聚合缓存 → API |
| [**index-analysis-20260708**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU3GXPHD4N4PFNAK3RVEVPLLCZGG/index-analysis-20260708.md.gz&markdown=true) | 数据库索引分析及优化建议 |
| [**lock-analysis-20260708**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU4RMVG4UVSH6BFYSLCDZFPWLEEA/lock-analysis-20260708.md.gz&markdown=true) | 锁竞争分析及并发优化建议 |
| [**member-presence-analysis**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUUZ4TE6DGRRZDVCJOHU27ZDVF5CZ/member-presence-analysis.md.gz&markdown=true) | 成员在线状态管理方案分析 |
| [**frontend-update-report-20260707**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUUZ4QWF3XQRLNJFINQ5YAUNZBGBI/frontend-update-report-20260707.md.gz&markdown=true) | 前端更新及对接注意事项 |
| [**v1-deployment-hardening**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU6CNCGFS7ENFRF3CNONGC3R7FH3/v1-deployment-hardening.md.gz&markdown=true) | v1 部署加固建议 |
| [**components-signature**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU7USIAOOXMV7ZGZUXKYC2A6CMOV/components-signature.md.gz&markdown=true) | 各组件接口签名汇总 |
| [**dependency-analysis-20260709**](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUUZEXQTYWUVI5ZHKX5FLARXGQ332/dependency-analysis-20260709.md.gz&markdown=true) | Tree、models/handlers 影响文件、未处理文件 |
