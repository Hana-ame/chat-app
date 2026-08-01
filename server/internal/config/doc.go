// Package config 配置加载:从环境变量读取 CHAT_* 配置,带默认值与非法值校验,
// JWT secret 为空时自动生成。进程启动时初始化一次,全局共享。
package config
