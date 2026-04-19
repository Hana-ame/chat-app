#!/bin/bash
cd "$(dirname "$0")"

# 防止 Android Doze 杀进程
termux-wake-lock

# 环境变量优化
export MALLOC_ARENA_MAX=2
export PYTHONHASHSEED=0

echo "Starting Chat Server..."
while true; do
    python -m uvicorn main:app --host 0.0.0.0 --port 8000 --workers 1 --no-access-logs
    echo "Server crashed! Restarting in 5s..."
    sleep 5
done