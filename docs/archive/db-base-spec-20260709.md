# DB 基类规范

> 原始来源：`server/internal/db/db.go`

---

## 一、原始代码

```go
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func isDupColumnErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}

//go:embed migrations/*.sql
var migrationFS embed.FS

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_time_format=sqlite", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	if err := conn.PingContext(context.Background()); err != nil {
		conn.Close()
		return nil, err
	}
	d := &DB{DB: conn}
	if err := d.Migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Migrate() error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		b, err := migrationFS.ReadFile("migrations/" + n)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", n, err)
		}
		if _, err := d.ExecContext(context.Background(), string(b)); err != nil {
			if !isDupColumnErr(err) {
				return fmt.Errorf("apply migration %s: %w", n, err)
			}
		}
	}
	return nil
}
```

---

## 二、类型总表

| 类型 | 说明 |
|------|------|
| `DB` | 封装 `*sql.DB`，提供 `Open`、`Migrate` 方法 |
| `sql.DB`（嵌入） | 标准库 database/sql 接口，所有 ".DB" 方法直接暴露 |

---

## 三、函数总表

| 函数 | 签名 | 说明 |
|------|------|------|
| `Open` | `(path string) (*DB, error)` | 打开 SQLite 连接 + 运行迁移 |
| `Migrate` | `() error` | 读取 `migrations/*.sql` 按文件名顺序执行 |
| `isDupColumnErr` | `(error) bool` | 判断是否为 `duplicate column name` 错误（ALERT TABLE 幂等）|

---

## 四、DSN 参数

```
file:<path>?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_time_format=sqlite
```

| 参数 | 值 | 说明 |
|------|-----|------|
| `journal_mode` | `WAL` | Write-Ahead Logging 提升并发 |
| `busy_timeout` | `5000` | 忙等待超时 5 秒 |
| `foreign_keys` | `ON` | 启用外键约束 |
| `_time_format` | `sqlite` | 时间格式由程序控制 |

---

## 五、依赖链

```
Open(path)
  ├─ sql.Open("sqlite", dsn)
  ├─ conn.SetMaxOpenConns(1)
  ├─ conn.PingContext(ctx)
  ├─ d.Migrate()
  │   ├─ fs.ReadDir("migrations")
  │   ├─ sort by name
  │   └─ for each *.sql:
  │       ├─ ReadFile → string
  │       └─ ExecContext → 忽略 dup column 错误
  └─ return d, nil
```

---

## 六、条件分支

| 条件 | 行为 |
|------|------|
| `sql.Open` 失败 | 返回 error |
| `conn.PingContext` 失败 | 关闭连接，返回 error |
| `d.Migrate` 失败 | 关闭连接，返回 error |
| 迁移 SQL 执行失败且不是 `duplicate column name` | 返回 error（迁移中断） |
| 迁移 SQL 执行失败且是 `duplicate column name` | 忽略（ALTER TABLE ADD COLUMN 幂等） |

---

## 七、约束汇总

| 约束 | 说明 |
|------|------|
| 驱动 | `modernc.org/sqlite`（纯 Go 实现，无需 CGO） |
| 并发 | `SetMaxOpenConns(1)` — 单连接串行化 |
| 迁移 | 嵌入 migrations 目录，按文件名排序执行 |
| 幂等 | `isDupColumnErr` 允许重复执行 ALTER TABLE |
| 时间 | `_time_format=sqlite` — 用 TEXT 存储 ISO 8601 |
| 外键 | `foreign_keys=ON` — 需在每条连接上设置", "filePath": "/mnt/d/WorkPlace/chat-app/docs/reports/db-base-spec-20260709.md"}