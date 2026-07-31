# 后端测试覆盖报告

日期: 2026-07-16

## 新增测试文件

| 文件 | 测试数 | 覆盖包 |
|------|--------|--------|
| `internal/orderedmap/orderedmap_test.go` | 23 | orderedmap |
| `internal/config/config_test.go` | 6 | config |
| `internal/service/service_test.go` | 83 | service |
| `internal/handlers/util_test.go` | 17 | handlers (util) |

## 包级覆盖率

| 包 | 覆盖率 | 说明 |
|----|--------|------|
| `internal/config` | **100%** | 全部覆盖（Load / getenv / randomHex） |
| `internal/service` | **80.4%** | chat / message / member / user / authz |
| `internal/orderedmap` | **71.4%** | 核心操作 + JSON 序列化 |
| `internal/handlers` | 5.5% | 仅 util.go 函数（mapServiceError / writeJSON / decodeJSON / bearerToken / cookie helpers） — HTTP handler 由 `testutil/handler_test.go` 集成测试覆盖 |

## 各包详细覆盖

### orderedmap (71.4%)

```
NewPair            100%
Key                100%
Value              100%
Len                100%
Swap               100%
Less               100%
New                100%
NewOrderedMap      100%
NewFromPairs       100%
NewFromMap         100%
SetEscapeHTML      100%
Get                100%
GetOrDefault       100%
Set                100%
Delete             100%
Keys               100%
Values             100%
SortKeys           100%
Sort               100%
UnmarshalJSON       80%
decodeOrderedMap    66%
decodeSlice         20%
MarshalJSON         86%
Reader              79%
```

### config (100%)

```
getenv             100%
getenvInt64        100%
getenvDuration     100%
randomHex          100%
Load               100%
```

### service (80.4%)

```
New                100%
WithTx             100%
Chat.ListForUser   100%
Chat.GetByID        63%
Chat.Create         81%
Chat.CreateOrGetDM  83%
Chat.Rename         82%
Chat.Delete         86%
Chat.ListPublic    100%
Chat.Join            0%
Chat.SetAnnouncement   85%
Chat.ClearAnnouncement 88%
Chat.MarkAnnouncementRead 100%
Chat.SetPinned      88%
Chat.Visit         100%
Chat.MarkRead      100%
Member.List        100%
Member.Add          79%
Member.Remove       79%
Message.List       100%
Message.Send        95%
Message.Edit        75%
Message.Delete      82%
Message.MarkRead   100%
extractMentions    100%
User.GetByID        83%
User.GetByEmail     83%
User.Create         83%
User.UpdateProfile  88%
User.Search        100%
MustBeMember        88%
RequireOwnerOrAdmin 67%
isNotFound         100%
isConflict         100%
isContentTooLong   100%
modelsUser           0%
```

### handlers (5.5% — 仅 util 函数)

```
mapServiceError      100%
writeJSON            100%
writeError           100%
decodeJSON            83%
bearerToken          100%
setAuthCookie        100%
setRefreshCookie     100%
clearRefreshCookie   100%
clearAccessTokenCookie 100%
intQueryParam        100%
timeNow                0% (未测试，仅一行 time.Now().UTC())
```

## 服务层测试场景

### ChatService (27 个测试)

| 测试 | 场景 |
|------|------|
| ListForUser | 成功列出用户聊天 / 空列表 |
| GetByID | 成功获取 / 非成员 forbidden / 不存在的聊天 forbidden |
| Create | 成功创建（自动加入 owner）/ 空名称 / 空白名称 |
| Rename | 成功 / DM 禁止 / 非 owner 禁止 / 不存在 |
| Delete | 成功 / DM 禁止 / 非 owner 禁止 / 不存在 |
| ListPublic | 正常返回 |
| Visit / MarkRead | 成功 / 非成员 forbidden |
| SetPinned | 成功 / 非成员 forbidden |
| SetAnnouncement | 成功 / 非 owner/admin 禁止 / 少于 3 人禁止 |
| ClearAnnouncement | 成功 / 非 owner/admin 禁止 |
| MarkAnnouncementRead | 成功 |
| CreateOrGetDM | 新建 / 已存在 / 目标用户不存在 |

### MessageService (12 个测试)

| 测试 | 场景 |
|------|------|
| List | 列出消息 / 非成员 forbidden |
| Send | 发送成功 / 非成员 / 空内容 / 空白内容 / 仅附件 / 非法附件 URL / 无 URL 附件 / 默认 MIME / @提及提取 / 内容超长 |
| Edit | 编辑成功 / 非成员 / 聊天 ID 不匹配 |
| Delete | 自己删除 / 他人消息禁止 / 聊天 ID 不匹配 |
| MarkRead | 成功 / 非成员 forbidden |

### MemberService (7 个测试)

| 测试 | 场景 |
|------|------|
| List | 列出成员 / 非成员 forbidden |
| Add | 添加成功 / DM 禁止 / 目标不存在 / 重复添加 / 聊天不存在 |
| Remove | 自己离开 / owner 保护 / DM 禁止 / 聊天不存在 |

### UserService (9 个测试)

| 测试 | 场景 |
|------|------|
| Create | 创建成功 / 邮箱重复冲突 |
| GetByID | 成功 / 不存在 |
| GetByEmail | 成功 / 不存在 |
| UpdateProfile | 成功 / 不存在 / 用户名重复冲突 |
| Search | 模糊搜索 / 空查询 |

## 总计

- 新增测试函数: **129** 个
- 文件总数: 12 个测试文件（含原有）
- 服务层覆盖率: **80.4%**
- config 覆盖率: **100%**
