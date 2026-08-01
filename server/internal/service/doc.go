// Package service 业务逻辑层:权限(authz)、聊天/消息/成员/反应/通知操作、
// 广播与 AI 流协调。聚合 Authz/Chat/User/Message/Member/Reaction/Stream 子服务,
// 被 handlers 与 ws 调用,是唯一触碰 db 的层。
package service
