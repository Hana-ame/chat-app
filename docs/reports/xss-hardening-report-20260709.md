# XSS 加固 & 功能变更报告

涉及文件：20 个（server 15 个, client 5 个）  
覆盖范围：本次对话全部修改

---

## 1. 消息内容渲染方式重写

### 修改前
`MessageItem.jsx` 使用 `react-markdown` + `remark-gfm` 渲染消息内容，支持完整 Markdown 语法（标题、列表、表格、代码块、图片、链接等）。

```
<ReactMarkdown remarkPlugins={[remarkGfm]} components={{...}}>{msg.content}</ReactMarkdown>
```

### 修改后
纯文本渲染，仅识别：

| 模式 | 渲染结果 |
|------|----------|
| `https://...` / `http://...` | `<a href="..." target="_blank" rel="noreferrer">` 可点击链接 |
| `<@uuid>` | `<span class="mention">@username</span>` 蓝色背景样式 |
| 其他所有内容 | React 文本节点（自动 HTML 转义） |

**关键变化：** 彻底移除 `react-markdown` 和 `remark-gfm` 依赖，消息内容不再支持任何 Markdown 语法。前端 JS 包体积从 450KB → 292KB（-35%）。

**新文件：** `renderContent.jsx` — 共享渲染组件。

#### 文件列表
- **新增** `client/src/components/renderContent.jsx`
- **修改** `client/src/components/MessageItem.jsx`
- **修改** `client/src/components/ChatView.jsx`

---

## 2. 图片渲染还原

### 修改前（这次的错误修改）
Markdown 图片 `![alt](url)` 和附件图片都渲染为下载链接：
```jsx
// renderContent.jsx（错误版本）
img: ({ src, alt }) =>
  <a href={src} target="_blank" rel="noreferrer" className="file-download-link">{alt || src}</a>,

// MessageItem.jsx（错误版本）
{a.mime_type?.startsWith('image/')
  ? <a href={a.url} ...>{a.filename}</a>
  : <div className="file-pill">...</div>
}
```

### 修改后
- 附件图片 → `<a href="..."><img src="..." class="file-attach-img" /></a>`（点击放大，内联预览）
- 非图片附件 → 下载链接
- Markdown 图片语法不再支持（content 为纯文本）

#### 文件
- **修改** `client/src/components/MessageItem.jsx`（还原附件 img）
- **修改** `client/src/styles/global.css`（`.file-attach-img` 替换 `.file-attach img`）

---

## 3. 服务端 HTML 字符剥离

### 修改前
```go
// auth.go
func ValidateUsername(username string) (string, error) {
    username = strings.TrimSpace(username)
    if username == "" {
        return "", errors.New("username required")
    }
    return username, nil  // ← 直接返回，含 <script> 等
}

// chats.go
func CreateChat(...) {
    name = strings.TrimSpace(name)  // ← 只去空格
}
```

### 修改后
新增 `sanitize.StripHTML` 剥离 `< > " ' &` 五个字符。

```go
// sanitize/sanitize.go
func StripHTML(s string) string {
    s = strings.ReplaceAll(s, "<", "")
    s = strings.ReplaceAll(s, ">", "")
    s = strings.ReplaceAll(s, "\"", "")
    s = strings.ReplaceAll(s, "'", "")
    s = strings.ReplaceAll(s, "&", "")
    return s
}
```

调用点：
- `auth.ValidateUsername` — 注册/改用户名前剥离
- `db.CreateChat` — 创建群聊前剥离
- `db.RenameChat` — 重命名前剥离
- `handlers.sanitizeFilename` — 上传文件名返回前剥离

#### 文件
- **新增** `server/internal/sanitize/sanitize.go`
- **修改** `server/internal/auth/auth.go`
- **修改** `server/internal/db/chats.go`
- **修改** `server/internal/handlers/uploads.go`

---

## 4. CSP 中间件 + X-Content-Type-Options

### 修改前
无任何安全响应头。

### 修改后
```go
w.Header().Set("Content-Security-Policy",
    "default-src 'self'; "+
    "script-src 'self' 'unsafe-inline'; "+
    "style-src 'self' 'unsafe-inline'; "+
    "img-src 'self' https://upload.moonchan.xyz data:; "+
    "connect-src 'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz; "+
    "font-src 'self' data:;")
w.Header().Set("X-Content-Type-Options", "nosniff")
```

#### 文件
- **修改** `server/internal/handlers/router.go`

---

## 5. WebSocket Hub register 锁优化

### 修改前
```go
func (h *Hub) register(c *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()
    // ... 更新 clients map ...
    if wasOffline && h.db != nil {
        _ = h.db.UpdateUserStatus(...)  // ← DB 操作在锁内
        _ = h.db.UpdateUserLastSeen(...)
        go h.broadcastPresence(...)
    }
}
```

### 修改后
```go
func (h *Hub) register(c *Client) {
    h.mu.Lock()
    // ... 更新 clients map ...
    wasOffline := len(set) == 1
    h.mu.Unlock()  // ← 先释放锁
    if wasOffline && h.db != nil {
        _ = h.db.UpdateUserStatus(...)  // ← DB 操作在锁外
        _ = h.db.UpdateUserLastSeen(...)
        h.broadcastPresence(...)
    }
}
```

**效果：** 高并发下 register 不再长时间阻塞其他 hub 操作。

#### 文件
- **修改** `server/internal/ws/hub.go`

---

## 6. refreshMu 竞态条件注释

### 修改前
无注释。

### 修改后
在 `Refresh` 和 `Logout` 关键路径添加了竞态条件说明和修复指引（共 12 行注释）。

#### 文件
- **修改** `server/internal/handlers/auth.go`

---

## 7. URL query token deprecated 注释

### 修改前
无注释。

### 修改后
- `handler.go` — `bearerToken` 函数内 query token 路径加 deprecated 注释
- `sse.go` — SSE handler 加 deprecated 注释

#### 文件
- **修改** `server/internal/handlers/handler.go`
- **修改** `server/internal/handlers/sse.go`

---

## 8. CSS 新增样式

| 选择器 | 用途 |
|--------|------|
| `.mention` | @mention 蓝色背景 + accent 色文字 |
| `.file-download-link` | 下载链接样式 |
| `.file-attach-img` | 附件图片（限制 200×150，圆角） |

#### 文件
- **修改** `client/src/styles/global.css`
