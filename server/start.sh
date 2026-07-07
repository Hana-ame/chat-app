#!/bin/bash

# --- 配置区 ---
BINARY="./chatd"
LOG_FILE="chatd.log"
ENV_FILE=".env"

echo "🚀 Starting Chat App Server..."

# 1. 检查二进制文件是否存在
if [ ! -f "$BINARY" ]; then
    echo "❌ Error: Binary $BINARY not found. Please build the project first: 'go build -o chatd ./cmd/chatd/main.go'"
    exit 1
fi

# 2. 提升文件描述符限制 (ulimit)
# 尝试将软限制提升至 65535，如果失败则打印警告
if ! ulimit -n 65535; then
    echo "⚠️  Warning: Failed to set ulimit -n 65535. The server may crash under high load."
    echo "    Please run 'sudo ulimit -n 65535' or configure /etc/security/limits.conf"
fi

# 3. 检查环境变量文件
if [ ! -f "$ENV_FILE" ]; then
    echo "❌ Error: Environment file $ENV_FILE not found!"
    exit 1
fi

# 4. 启动服务器
echo "📅 $(date)" >> "$LOG_FILE"
echo "------------------------------------------------" >> "$LOG_FILE"
echo "Starting server with ulimit -n $(ulimit -n)..." >> "$LOG_FILE"

# 使用 exec 替换 shell 进程，以便正确接收 SIGTERM 信号
# 将标准输出和错误全部重定向到日志文件
exec $BINARY >> "$LOG_FILE" 2>&1
