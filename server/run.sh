#!/bin/bash
cd "$(dirname "$0")"

echo "Starting Chat Server on 0.0.0.0:8000 ..."
exec python -m uvicorn main:app --host 0.0.0.0 --port 8000 --workers 1 --no-access-log
