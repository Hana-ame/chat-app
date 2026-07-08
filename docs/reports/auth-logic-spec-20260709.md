# 认证逻辑规范 (Auth Logic Spec)

> 原始来源：`server/internal/auth/auth.go`

---

## 一、核心组件

### 1. Access Token (JWT)
- **算法**：`HS256`
- **Payload (Claims)**：
    - `uid`: 用户 ID
    - `exp`: 过期时间
    - `iat`: 签发时间
    - `sub`: 用户 ID
- **有效期**：由 `Config.AccessTokenTTL` 决定。

### 2. Refresh Token
- **生成**：`crypto/rand(32 bytes)` $\rightarrow$ `base64.RawURLEncoding` $\rightarrow$ `raw_token`
- **存储**：数据库存储 `sha256(raw_token)` $\rightarrow$ `hash_token`
- **有效期**：由 `Config.RefreshTokenTTL` 决定。

---

## 二、流程定义

### 1. 密码处理流程
- **哈希**：`bcrypt.GenerateFromPassword` (DefaultCost)
- **截断**：若密码长度 $> 72$ 字节，则截断至 72 字节（bcrypt 限制）。
- **验证**：`bcrypt.CompareHashAndPassword`。

### 2. Token 签发与校验
- **签发 Access Token**：`jwt.NewWithClaims` $\rightarrow$ `SignedString(jwtSecret)`。
- **校验 Access Token**：`jwt.ParseWithClaims` $\rightarrow$ 校验签名 $\rightarrow$ 校验有效期 $\rightarrow$ 校验 `uid` 非空。

### 3. 用户名与邮箱规范化
- **Email**：`strings.ToLower` + `strings.TrimSpace`。
- **Username**：`strings.TrimSpace` $\rightarrow$ 非空检查。

---

## 三、安全约束

| 项目 | 策略 | 说明 |
|---|---|---|
| 密码长度限制 | $\le 72$ 字节 | 超过则截断，不报错 |
| Token 签名 | `HS256` | 使用 `JWTSecret` 密钥 |
| 随机性 | `crypto/rand` | 生成 Refresh Token 时使用加密级随机数 |
