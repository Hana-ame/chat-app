# 本地源码部署指南

## 前置要求

- Go 1.23+
- Node.js 20+
- Git

## 步骤

### 1. 克隆并进入项目

```bash
git clone https://github.com/Hana-ame/chat-app.git
cd chat-app
```

### 2. 构建前端

```bash
cd client
npm ci
npm run build
cd ..
```

产物在 `client/dist/`。

### 3. 构建后端

```bash
cd server
go build -ldflags="-s -w -X main.Version=$(git describe --tags --always)" -o ../chatd ./cmd/chatd/
cd ..
```

### 4. 配置环境变量

```bash
cp .env.example .env
# 编辑 .env，至少设置：
#   CHAT_JWT_SECRET=<随机字符串>
#   CHAT_DB_PATH=chat.db
#   CHAT_STATIC_DIR=client/dist
#   CHAT_ADDR=:8080
```

### 5. 启动

```bash
# 从 .env 读取配置启动
set -o allexport; source .env; set +o allexport  # Linux/macOS
# 或
$env:CHAT_JWT_SECRET="..."; $env:CHAT_ADDR=":8080"; ...  # Windows PowerShell

./chatd
```

### 6. 验证

```bash
curl http://localhost:8080/api/version
# 应返回 {"version":"v0.8.15"}
```

浏览器打开 `http://localhost:8080`。

## 快速调试

构建并启动（适用于开发迭代）：

```bash
# Windows
cd server; go build -ldflags="-s -w -X main.Version=dev" -o ../chatd.exe ./cmd/chatd/; cd ..
./chatd.exe
```

```bash
# Linux/macOS
cd server && go build -ldflags="-s -w -X main.Version=dev" -o ../chatd ./cmd/chatd/ && cd ..
./chatd
```
