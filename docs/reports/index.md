# 报告索引 (Reports Index)

本页面汇总了近期后端架构、模型及代码审查的所有详细报告。

| 报告名称 | 预览链接 | 说明 |
|---|---|---|
| **模型定义与数据生成规范** | [预览](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU72COJ2S7VSXFGKKGNF7DMUX7ZT/models-data-spec-20260708.md.gz&markdown=true) | User, Chat, Message 等核心模型的 JSON 字段、生成规则及存储方案。 |
| **后端代码审查报告** | [预览](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU2WWOAAOV44WRHZFDI7BVKJM4CP/backend-code-review-20260708.md.gz&markdown=true) | 安全风险、性能瓶颈、并发问题及已完成的优化修复。 |
| **DB DAO API 参考** | [预览](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUUYCE66GSBOBUBFY65FJDLNEAKAX/db-dao-api-reference-20260708.md.gz&markdown=true) | `internal/db` 包中所有方法的输入输出、SQL 实现及目标模型。 |
| **Reaction 流程报告** | [预览](https://upload.moonchan.xyz/api/01LLWEUU5LL3WFYI4CNCFAJWQMY5X63P3XW/code_1780484878124.html?url=https://upload.moonchan.xyz/api/01LLWEUU762IKELTI2H5E3X5PWP77EGBO2/reaction-flow-report-20260708.md.gz&markdown=true) | 反应功能的原始存储 → 聚合缓存 → API 返回的完整链路。 |
| **API 接口规范** | [预览](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU5JNNZBGN54GVG3BQ4ZS6LDRMY2/api-handlers-spec-20260709.md.gz&markdown=true) | 35 个 handler 的完整源码、依赖链、条件分支、目的和基本方法。 |
| **Test Suite 规范** | [预览](https://upload.moonchan.xyz/api/01LLWEUU6EH4YQ7VRRARGY2OBPF4DCG7FL/code_page.html?url=https://upload.moonchan.xyz/api/01LLWEUU34BMS3M5FPINDKIZBNXA2QZ33C/test-suite-spec-20260709.md.gz&markdown=true) | 52 个测试的源码、依赖链、条件分支，覆盖 DB/Handler/Auth/WS/集成测试。 |
