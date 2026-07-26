# DB Migration 方案

## 命名规则

SQL 文件命名：`NNN_xxx.sql`，其中 `NNN` 是三位数字版本号（000 ~ 999），`xxx` 是描述。

示例：
```
000__init.sql
001_add_column.sql
002_create_table.sql
```

## 版本追踪

`schema_migrations` 表记录所有已应用的迁移版本号：

```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
```

`version` = 文件名中的三位数字（转为整数），`001_add_column.sql` → version=1。

## 迁移流程

每次启动执行：

1. 查询 `schema_migrations` 获取当前最大版本（初始 -1）
2. 循环：
   - next = current + 1
   - `fs.Glob("migrations/{next:03d}_*.sql")`
   - 找不到 → break（已最新）
   - 找到 → 执行 SQL → `INSERT INTO schema_migrations (version) VALUES (next)` → 继续

```
current = MAX(version)   // -1 if no rows
loop:
  next = current + 1
  file = glob("migrations/{next:03d}_*.sql")
  if !file: break
  exec(file)
  INSERT INTO schema_migrations (version) VALUES (next)
  current = next
```

## Go 迁移

需要 Go 逻辑的迁移使用版本号 1000+，SQL 迁移完成后执行。

```go
var goMigrations = []goMigration{
    {2, migrateV2EnsureColumns},
}
// schema_migrations 中以 version = 1002 记录
```

## 规则

- 永不重命名或删除已发布的迁移文件
- 只追加新文件，版本号递增
- 幂等：同版本只会执行一次
