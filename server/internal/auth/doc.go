// Package auth 认证服务:密码哈希校验、JWT 签发/解析/过期处理、refresh token 轮换。
// 被 service 调用;JWT 密钥与时效来自 config。
package auth
