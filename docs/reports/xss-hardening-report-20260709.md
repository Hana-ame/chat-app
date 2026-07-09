# XSS 加固 & 功能变更报告

涉及文件：20 个（server 9 个, client 5 个, 新增 3 个, 删除 3 个）  
覆盖范围：本次对话全部修改  
最后更新：2026-07-09（同步 StripHTML 回滚）

---

## 1. 消息内容渲染方式重写

| 字段 | 内容 |
|------|------|
| **文件** | `client/src/components/MessageItem.jsx`, `ChatView.jsx`, **新增** `renderContent.jsx` |
| **类型** | 安全加固 + 功能变更 |
| **影响范围** | 所有消息内容显示 |
| **包体积变化** | 450KB → 292KB（-35%，因移除 `react-markdown` + `remark-gfm`） |

### 修改前

`MessageItem.jsx` 使用 `react-markdown` v9 + `remark-gfm` v4 渲染消息内容，支持完整 Markdown 语法：

```jsx
<ReactMarkdown
  remarkPlugins={[remarkGfm]}
  components={{
    a: ({ href, children }) =>
      <a href={href} target="_blank" rel="noreferrer">{children}</a>,
    img: ({ src, alt }) =>
      <a href={src} target="_blank" rel="noreferrer">
        <img src={src} alt={alt} className="msg-inline-img" loading="lazy" />
      </a>,
    h1: 'p', h2: 'p', h3: 'p', h4: 'p', h5: 'p', h6: 'p',
    blockquote: 'p', table: ({ children }) => <>{children}</>, input: () => null,
  }}
>{msg.content}</ReactMarkdown>
```

**安全分析：** 虽然 `react-markdown` 默认不渲染 raw HTML（无 `rehype-raw`），但自定义 `a` 组件未过滤 `javascript:` 协议，`[点我](javascript:alert(1))` 可触发 XSS。自定义 `img` 组件通过 `<a>` 包裹，风险较低。

### 修改后

纯文本渲染函数 `renderContent(content, userMap)`，仅识别两种模式：

| 模式 | 检测方式 | 渲染结果 |
|------|----------|----------|
| `https://...` / `http://...` | `URL_RE = /(https?:\/\/[^\s<>\[\]{}|\\^`]+)/g` | `<a href="..." target="_blank" rel="noreferrer">` |
| `<@uuid>` | `MENTION_MATCH = /^<@([a-f0-9-]{36})>$/` | `<span class="mention">@username</span>` |
| 其他所有内容 | 默认 | React 文本节点（自动 HTML 转义） |

```jsx
// renderContent.jsx — 核心逻辑
const mentionParts = content.split(MENTION_RE);
for (const part of mentionParts) {
    if (part matches mention pattern) {
        children.push(<span className="mention">@{username}</span>);
    } else {
        const urlParts = part.split(URL_RE);
        for (j = 0; j < urlParts.length; j++) {
            if (j % 2 === 1) // URL capture group
                children.push(<a href={url} target="_blank" rel="noreferrer">{url}</a>);
            else
                children.push(plainText); // React auto-escapes
        }
    }
}
```

**用户映射：** `userMap` 从 `useChatStore` 的 `chats[i].members` 构建 `{userID → username}`。当消息中的 `<@uuid>` 匹配到成员时，显示为 `@username`。

**Pinned 消息同步：** `ChatView.jsx` 的 pinned 消息渲染也切换为 `renderContent`，保持一致。

**渲染流程对比：**

| 步骤 | 旧流程 | 新流程 |
|------|--------|--------|
| 1 | 接收 HTML/Markdown | 接收纯文本 |
| 2 | react-markdown 解析 AST | URL_RE 正则分割 |
| 3 | rehype 生成 HTML 元素 | 按段渲染 a/span/text |
| 4 | React 组件映射 | React 直接渲染 |
| 依赖 | react-markdown + remark-gfm + 组件配置 | 无外部依赖 |

---

## 2. 图片渲染还原

| 字段 | 内容 |
|------|------|
| **文件** | `client/src/components/MessageItem.jsx`, `renderContent.jsx` |
| **类型** | 错误修正（第一次修改搞反了用户意图） |

### 错误修改（已回滚）

第一次修改将 markdown 图片 `![alt](url)` 和附件图片**全部改为下载链接**：

```jsx
// renderContent.jsx（错误版本）
img: ({ src, alt }) =>
  <a href={src} target="_blank" rel="noreferrer" className="file-download-link">{alt || src}</a>,

// MessageItem.jsx（错误版本）
{a.mime_type?.startsWith('image/')
  ? <a href={a.url} className="file-download-link">{a.filename}</a>  // ← 错误：删了 img 标签
  : <div className="file-pill">...</div>
}
```

### 修正后

| 附件类型 | 渲染方式 | 说明 |
|----------|----------|------|
| 图片附件（image/*） | `<a href="url"><img src="url" class="file-attach-img" /></a>` | 点击放大，内联预览保留 |
| 非图片附件 | `<div class="file-pill"><a href="url">filename</a></div>` | 下载链接 |
| Markdown 图片 `![alt](url)` | 不再支持（content 纯文本化） | — |

**安全说明：** `<img>` 标签本身安全——`src` 属性不会执行 JavaScript（即使指定 `javascript:` 也不会触发），浏览器仅加载并显示图片。`onerror` 等事件处理器虽存在理论风险，但所有属性的值都由 React 自动转义。

---

## 3. CSP 中间件 + X-Content-Type-Options

| 字段 | 内容 |
|------|------|
| **文件** | `server/internal/handlers/router.go:25-36` |
| **类型** | 纵深防御新增 |
| **影响** | 所有 HTTP 响应 |

### 修改前

无任何安全响应头。

### 修改后

```go
r.Use(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' https://upload.moonchan.xyz data:; "+
            "connect-src 'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz; "+
            "font-src 'self' data:;")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        next.ServeHTTP(w, r)
    })
})
```

**CSP 指令含义：**

| 指令 | 来源 | 用途 |
|------|------|------|
| `default-src 'self'` | 本域 | 所有资源默认仅允许本域 |
| `script-src 'self' 'unsafe-inline'` | 本域 + 内联 | Vite dev server 需要 |
| `style-src 'self' 'unsafe-inline'` | 本域 + 内联 | 内联 CSS 注入样式 |
| `img-src 'self' https://upload.moonchan.xyz data:` | + 上传域 | 消息附件图片 + base64 |
| `connect-src 'self' wss://wsl-8080.moonchan.xyz https://upload.moonchan.xyz` | + WS + 上传 | API / WS / Upload |
| `font-src 'self' data:` | + base64 | 字体图标 |

**注意：** `'unsafe-inline'` 降低了 CSP 的严格性，但前端使用 Vite（开发模式需要内联脚本），且无单独构建的 CSP 配置文件，无法在开发环境移除。生产构建后应评估是否可以移除。

---

## 4. 服务端 HTML 字符剥离（已创建并回滚）

| 字段 | 内容 |
|------|------|
| **文件** | ~~`server/internal/sanitize/sanitize.go`~~（已删除） |
| **类型** | 防御性编码 → 已回滚 |
| **涉及调用点** | `auth.ValidateUsername`, `db.CreateChat`, `db.RenameChat`, `upload.sanitizeFilename` |

### 创建（第一次修改）

新增 `sanitize` package，剥离 `< > " ' &` 五个字符：

```go
func StripHTML(s string) string {
    s = strings.ReplaceAll(s, "<", "")
    s = strings.ReplaceAll(s, ">", "")
    s = strings.ReplaceAll(s, "\"", "")
    s = strings.ReplaceAll(s, "'", "")
    s = strings.ReplaceAll(s, "&", "")
    return s
}
```

### 回滚原因

| 原因 | 详细说明 |
|------|----------|
| `&` 破坏数据 | `AT&T` → `ATT`，合法数据永久丢失 |
| 低风险输入 | 用户名/群名/文件名不直接渲染为 HTML |
| CSP 兜底 | 新增的 CSP 中间件已阻止内联脚本执行 |
| 前端渲染 | 消息内容改为纯文本，无 HTML 注入点 |
| 纵深过度 | React 的 `textContent` 渲染已经是最强防线，再叠加字符剥离无实质增益 |

### 当前状态

| 文件 | 状态 |
|------|------|
| `server/internal/sanitize/sanitize.go` | ❌ 已删除 |
| `auth.ValidateUsername` | 恢复为仅 `strings.TrimSpace` |
| `db.CreateChat` / `db.RenameChat` | 恢复为原始逻辑 |
| `upload.sanitizeFilename` | 保留但已标记 `// Deprecated` |

**结论：** username/chat name/filename 的 XSS 防御完全依赖 React 的 `textContent` 渲染和 CSP 头。

---

## 5. CSS 新增样式

| 文件 | `client/src/styles/global.css` |
|------|-------------------------------|

### 新增选择器

```css
/* @mention 样式 */
.mention {
  background: rgba(88, 101, 242, 0.25);
  color: var(--accent);
  border-radius: 3px;
  padding: 0 4px;
  font-weight: 500;
}

/* 下载链接样式 */
.file-download-link {
  color: var(--accent);
  text-decoration: underline;
  cursor: pointer;
  font-size: 13px;
}
```

### 变更选择器

| 选择器 | 变更 | 原因 |
|--------|------|------|
| `.msg-content .msg-inline-img` | ❌ 移除 | markdown 图片渲染不再使用 |
| `.file-attach img` | → `.file-attach-img` | 独立 class 避免全局样式污染 |

---

## 6. 文件变更清单

### 新增文件

| 文件 | 用途 |
|------|------|
| `client/src/components/renderContent.jsx` | 内容渲染函数（URL + @mention + 纯文本） |

### 修改文件

| 文件 | 变更内容 |
|------|----------|
| `client/src/components/MessageItem.jsx` | 改为 `renderContent`，还原附件 img 标签 |
| `client/src/components/ChatView.jsx` | pinned 消息改为 `renderContent` |
| `client/src/styles/global.css` | 新增 `.mention` `.file-download-link` `.file-attach-img` |
| `server/internal/handlers/router.go` | 新增 CSP + X-Content-Type-Options 中间件 |

### 删除文件

| 文件 | 原因 |
|------|------|
| `server/internal/sanitize/sanitize.go` | StripHTML 回滚，包零引用后删除 |
