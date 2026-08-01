// Package db SQLite 数据访问层:users/chats/messages/members/reactions/refresh_tokens
// 等表读写;启动时执行幂等迁移(db_fixups.go 动态补列 + migrations/ 一次性结构变更)。
// 被 service 调用,不感知 HTTP/WS。
package db
