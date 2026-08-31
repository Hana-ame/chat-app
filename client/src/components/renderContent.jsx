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

export function renderContent(content, userMap) {
  if (!content) return null;
  const mentionParts = content.split(MENTION_RE);
  const children = [];

  for (let i = 0; i < mentionParts.length; i++) {
    const part = mentionParts[i];
    if (!part) continue;

    const m = part.match(MENTION_MATCH);
    if (m) {
      const username = userMap[m[1]] || 'unknown';
      children.push(<span key={`m${i}`} className="mention">@{username}</span>);
      continue;
    }

    // 【本地改动 2026-09-02】先将文本切分为 math/text 片段，再对 text 片段
    // 做 URL 展开（URL 展开在 MathRender 外部完成）。
    const mathTokens = tokenizeMath(part);
    const rendered = renderMathTokens(mathTokens, `p${i}`);
    children.push(...rendered);
  }

  return children.length > 0 ? children : null;
}
