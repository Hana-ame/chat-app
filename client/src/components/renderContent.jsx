// renderContent.jsx — 消息正文渲染（mentions + URLs + LaTeX 公式）
// 【本地改动 2026-09-02】新增 LaTeX 公式渲染（$...$ 行内 / $$...$$ 独立行），
// 通过 <MathRender> 组件懒加载 KaTeX（首屏 bundle 零开销）。
//
// 处理顺序：mentions → LaTeX 公式 → URLs。URLs 在 LaTeX 之外展开，保证
// 公式内的 URL 不会被错误解析（实际场景中公式不含 URL，但防御为先）。

import MathRender from './MathRender';

const MENTION_RE = /(<@[a-f0-9-]{36}>)/;
const MENTION_MATCH = /^<@([a-f0-9-]{36})>$/;
const URL_RE = /(https?:\/\/[^\s<>[\]{}|\\^`]+)/g;

// 【本地改动 2026-09-02】图片代理 base URL：所有消息正文 ![]() 图片 src
// 重写到该代理，隐藏观看者的 IP / Referer，避免原图 host 直接暴露。
// 与 chatto 的 IMAGE_PROXY_BASE 同源，使用同一 proxy.moonchan.xyz 域名。
const IMAGE_PROXY_BASE = 'https://proxy.moonchan.xyz';

// 【本地改动 2026-09-02】把原始图片 src 重写为代理 URL。
// 仅允许 http(s)；非 http(s)（含 javascript:/data:/file:/相对路径等）一律返回 '#'。
// 已指向 proxy.moonchan.xyz 自身的 URL 直通（避免二次代理环，2026-08-31 修复踩坑）。
// 思路：URL + searchParams 保留原始 path/query，追加 proxy_host / proxy_scheme，
// fragment 留在最末尾（与用户约定一致）。
// 边界：仅影响消息正文 inline image；附件（走 /assets/files/ 独立签名 URL）不受影响。
export function proxyImageSource(src) {
  let original;
  try {
    original = new URL(src);
  } catch {
    return '#';
  }
  if (original.protocol !== 'http:' && original.protocol !== 'https:') {
    return '#';
  }

  const proxyHostname = new URL(IMAGE_PROXY_BASE).hostname;
  if (original.hostname === proxyHostname) {
    return src;
  }

  const proxy = new URL(IMAGE_PROXY_BASE);
  proxy.pathname = original.pathname;
  proxy.search = original.search;
  proxy.searchParams.set('proxy_host', original.host);
  proxy.searchParams.set('proxy_scheme', original.protocol === 'https:' ? 'https' : 'http');
  proxy.hash = original.hash;
  return proxy.toString();
}

// 【本地改动 2026-09-02】Markdown inline image 正则：![alt](src)。
// alt 不含 `]`（CommonMark 安全子集）；src 不含 `)`、空格。
// 注意：src 中的括号会被截断，由后续 URL 校验兜底（非法 src → '#'）。
const IMAGE_RE = /!\[([^\]]*)\]\(([^)\s]+)\)/g;

// 【本地改动 2026-09-02】LaTeX 公式正则。
//
// 行内（inline）：$...$
//   - 首字符必须为字母、反斜杠或 LaTeX 运算符（+ - * / ^ _ < > =）——防止 `$10`
//     等金额被误识别（对齐 fork 2026-09-01 变更）。
//   - 内容不含 `$`（不允许嵌套）。
//
// 独立行（display）：$$...$$
//   - 首字符不为 `$` 且不为换行；内容不含 `$`、`\n`。
//   - 处理顺序：先匹配 display 再匹配 inline，避免 `$$...$$` 被误拆为两个 inline。
//
// 转义：`\$` 不参与匹配（聊天中极少，不做 lookbehind 兼容）。
const MATH_DISPLAY_RE = /\$\$([^$\n][^$\n]*?)\$\$/g;
const MATH_INLINE_RE = /\$([a-zA-Z\\+\-\*\/\^_<>=][^$\n]*?)\$/g;

// tokenizeMath 将一段纯文本切分为 [text, math-inline, math-display] 三种片段。
// 顺序：先处理 display，再对剩余 text 处理 inline，保证 $$ 优先。
export function tokenizeMath(text) {
  let tokens = [];
  let lastIndex = 0;

  for (const m of text.matchAll(MATH_DISPLAY_RE)) {
    if (m.index > lastIndex) {
      tokens.push({ type: 'text', value: text.slice(lastIndex, m.index) });
    }
    tokens.push({ type: 'math-display', value: m[1] });
    lastIndex = m.index + m[0].length;
  }
  if (lastIndex < text.length) {
    tokens.push({ type: 'text', value: text.slice(lastIndex) });
  }

  const result = [];
  for (const tok of tokens) {
    if (tok.type !== 'text') {
      result.push(tok);
      continue;
    }
    let lastTextIndex = 0;
    for (const m of tok.value.matchAll(MATH_INLINE_RE)) {
      if (m.index > lastTextIndex) {
        result.push({ type: 'text', value: tok.value.slice(lastTextIndex, m.index) });
      }
      result.push({ type: 'math-inline', value: m[1] });
      lastTextIndex = m.index + m[0].length;
    }
    if (lastTextIndex < tok.value.length) {
      result.push({ type: 'text', value: tok.value.slice(lastTextIndex) });
    }
  }
  return result;
}

// 【本地改动 2026-09-02】将文本切分为 [text, image] 片段。
// 顺序：先匹配 ![]()，text 保留，image 携带 alt + src。
export function tokenizeImages(text) {
  if (typeof text !== 'string') return [];
  const tokens = [];
  let lastIndex = 0;
  for (const m of text.matchAll(IMAGE_RE)) {
    if (m.index > lastIndex) {
      tokens.push({ type: 'text', value: text.slice(lastIndex, m.index) });
    }
    tokens.push({ type: 'image', alt: m[1], src: m[2] });
    lastIndex = m.index + m[0].length;
  }
  if (lastIndex < text.length) {
    tokens.push({ type: 'text', value: text.slice(lastIndex) });
  }
  return tokens.length > 0 ? tokens : [];
}

function renderMathTokens(mathTokens, keyPrefix) {
  const children = [];
  let textBuf = [];
  let i = 0;

  const flushTextBuf = () => {
    if (textBuf.length > 0) {
      const text = textBuf.join('');
      const urlParts = text.split(URL_RE);
      for (let j = 0; j < urlParts.length; j++) {
        const sub = urlParts[j];
        if (!sub) continue;
        if (j % 2 === 1) {
          children.push(
            <a key={`${keyPrefix}_${i}_l${j}`} href={sub} target="_blank" rel="noreferrer">
              {sub}
            </a>
          );
        } else {
          children.push(sub);
        }
      }
      textBuf = [];
    }
  };

  for (const tok of mathTokens) {
    if (tok.type === 'text') {
      textBuf.push(tok.value);
    } else {
      flushTextBuf();
      children.push(
        <MathRender
          key={`${keyPrefix}_${i}_m${children.length}`}
          latex={tok.value}
          displayMode={tok.type === 'math-display'}
        />
      );
    }
    i++;
  }
  flushTextBuf();
  return children;
}

export function renderContent(content, userMap, currentUserId) {
  if (!content) return null;
  const mentionParts = content.split(MENTION_RE);
  const children = [];

  for (let i = 0; i < mentionParts.length; i++) {
    const part = mentionParts[i];
    if (!part) continue;

    const m = part.match(MENTION_MATCH);
    if (m) {
      const username = userMap[m[1]] || 'unknown';
      // 【本地改动 2026-09-03】自我提及高亮：@自己 附加 mention-self 样式
      // （对齐 chatto FDR-006「self-mentions get additional styling」）。
      const isSelf = currentUserId && m[1] === currentUserId;
      children.push(<span key={`m${i}`} className={'mention' + (isSelf ? ' mention-self' : '')}>@{username}</span>);
      continue;
    }

    // 【本地改动 2026-09-02】管线：images → math → URLs。
    // 1) 切分为 [text, image] 片段；image 走 proxyImageSource 重写 src。
    // 2) 对 text 片段再做 LaTeX 公式 tokenization。
    // 3) 对 leaf text 片段展开 URL 链接。
    // 处理顺序（images 优先）：markdown-it 的 image 规则在 link 之前注册，
    // 这里保持一致：先切图片再处理数学公式，避免 URL 中的 ![]() 冲突。
    const imageTokens = tokenizeImages(part);
    for (const tok of imageTokens) {
      if (tok.type === 'image') {
        children.push(
          <img
            key={`i${i}_${children.length}`}
            src={proxyImageSource(tok.src)}
            alt={tok.alt || ''}
            loading="lazy"
            referrerPolicy="no-referrer"
          />
        );
      } else {
        const mathTokens = tokenizeMath(tok.value);
        const rendered = renderMathTokens(mathTokens, `p${i}`);
        children.push(...rendered);
      }
    }
  }

  return children.length > 0 ? children : null;
}
