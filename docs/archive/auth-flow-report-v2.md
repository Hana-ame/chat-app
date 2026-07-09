# Auth Flow Test Report v2

## 测试统计
- **总计**: 37 个用例全部 PASS
- **新增**: 4 个测试 (并发竞态 / 登后失效 / Cookie属性 / 多设备隔离)
- **修复**: Refresh handler 添加互斥锁防止并发轮换竞态

## 测试详情

### 1. TestRegisterLoginRefresh (新增)
| # | Method | Path | 预期 |
|---|--------|------|------|
| 1 | POST | /api/auth/register | 200 + access_token + refresh_token(cookie) + user.id |
| 2 | POST | /api/auth/login | 200 + 同一 user.id + 不同 refresh_token |
| 3 | POST | /api/auth/refresh | 200 + 同一 user.id + 新 refresh_token |

### 2. TestAccessDeniedWithoutToken (新增)
7个端点无 Bearer → 401

### 3. TestInvalidAccessToken (新增)
随机JWT / 畸形 / 空 → 401

### 4. TestTamperedRefreshToken (新增)
| Subtest | Cookie | 预期 |
|---------|--------|------|
| random string | `refresh_token=xxx` | 401 |
| empty value | `refresh_token=` | 401 |
| unknown hash | hex string | 401 |

### 5. TestRefreshTokenRotation (既有)
同一旧 token 用两次 → 第一次 200, 第二次 401

### 6. TestRefreshWithoutCookie (新增)
无 Cookie → 400 `bad_request`

### 7. TestRegisterDuplicateEmail | TestRegisterDuplicateUsername (新增)
重复 email/username → 409

### 8. TestLoginWrongPassword (新增)
错误密码 → 401 `invalid_credentials`

### 9. TestRegisterInvalidInput (新增)
| Subtest | 预期 |
|---------|------|
| bad email | 400 |
| short username | 400 |
| short password | 400 |
| empty email | 400 |

### 10. TestConcurrentRefreshRotation (新增, 高优) 🔴
10个 goroutine 同时用同一旧 refresh_token 刷新:
- 1个成功 200
- 9个失败 401

### 11. TestLogoutInvalidatesTokens (新增, 高优) 🔴
- Logout → 200
- 旧 access_token → 200 (JWT 无状态, 设计如此)
- 旧 refresh_token → 401

### 12. TestCookieSecurityAttributes (新增, 中优) 🟡
- `HttpOnly`: true
- `SameSite`: StrictMode
- `Secure`: 测试环境不要求 (根据 TLS 情况动态设置)

### 13. TestMultiDeviceRefreshIsolation (新增, 中优) 🟡
两个独立登录 → 各自 refresh 链互不干扰

### 既有测试 (9个, 已修复 cookie 适配)
TestFixtureSetup / TestUserRegisterLogin / TestDuplicateEmail /
TestUnauthorizedAccess / TestRefreshTokenFlow / TestCreateGroupChatAndSendMessage /
TestAuthEndpoints / TestListChatsWithUnreads / TestUploadNotLoggedIn /
TestRenameChatOnlyOwner / TestChatForbidden / TestMarkRead /
TestDeleteChatOnlyOwner / TestConcurrentRegister / TestHealthz

## 服务端改动
- `handlers/handler.go`: Server 结构体添加 `refreshMu sync.Mutex`
- `handlers/auth.go`: Refresh handler 临界区用 `refreshMu.Lock/Unlock` 保护

## 结果
- `internal/auth` — 0.257s
- `internal/db` — 0.488s
- `internal/testutil` — 2.625s
- `internal/ws` — 0.658s
