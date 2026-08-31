// 【本地改动 2026-09-02】KaTeX 数学公式渲染（移植 chatto fork 分歧行为）。
//
// 目的：消息正文中 `$...$`（行内）与 `$$...$$`（独立行）渲染为 KaTeX 公式。
// 思路：本组件负责首次加载时懒加载 katex（JS + CSS），后续直接调用
// `katex.renderToString`；渲染失败用 `throwOnError: false` 返回 LaTeX 源码
// 回退显示（避免恶意/畸形输入导致整个消息渲染崩溃）。
// 边界：仅影响 renderContent.jsx 的 math 片段；不改其他渲染逻辑。
// 安全：katex 默认渲染（未启用 mhchem / html 插件），输出不含 <script> /
//      外链，浏览器 XSS 风险可控；输出由 React dangerouslySetInnerHTML
//      嵌入，由 katex 生成的 <span class="katex"> 外壳包裹。
// 懒加载：首屏 bundle 零开销（katex ~200KB JS + ~50KB CSS 仅在遇到公式时加载）。
// 踩坑：
//  - `await import('katex/dist/katex.min.css')` 在 node 环境（如 vitest）抛错，
//    需要 try/catch 兜底（vitest 环境 = node，无 CSS）。
//  - `katex.renderToString` 默认 throw 错误；必须 `throwOnError: false`。
//  - `$$...$$` 内禁止换行（单行公式）；跨行 LaTeX 应改用 `\[...\]`（未实现）。
// 引入背景（2026-09-02）：fork 与上游在 KaTeX 上明确分歧；上游禁用，fork 启用。
// 测试保护：MathRender.test.jsx 覆盖正常渲染 / 错误容忍 / 懒加载路径。

import { useEffect, useRef, useState } from 'react';

const MATH_PLACEHOLDER_PREFIX = 'data-math-content="';

let katexRenderer = null;
let katexLoading = false;
let katexLoadingResolve = null;

// ensureKatexLoader 首次调用时异步加载 katex JS + CSS；后续调用立即返回
// katexRenderer 或 Promise。多线程安全（JS 单线程事件循环，同一时刻只有一个
// 加载任务执行）。
async function ensureKatexLoader() {
  if (katexRenderer) return katexRenderer;
  if (katexLoading) {
    return new Promise((resolve) => {
      const check = () => {
        if (katexRenderer) {
          resolve(katexRenderer);
        } else if (katexLoading) {
          setTimeout(check, 50);
        } else {
          // katexLoading=false 且 katexRenderer=null：加载失败
          resolve(null);
        }
      };
      check();
    });
  }
  katexLoading = true;
  // CSS 通过动态 link 注入（不依赖 Vite CSS plugin），避免影响未用 katex 的
  // 首屏 bundle；同时兜底 `await import(...css)` 以兼容 bundler 环境。
  try {
    await import('katex/dist/katex.min.css');
  } catch {
    // node 环境（vitest）无 CSS，跳过；浏览器环境 Vite 会处理 import() CSS。
  }
  try {
    const katexModule = await import('katex');
    katexRenderer = katexModule.renderToString;
  } catch {
    katexRenderer = null;
  } finally {
    katexLoading = false;
  }
  return katexRenderer;
}

// renderMathSync 在已确保 katex 加载的前提下同步渲染一段 LaTeX 为 HTML 字符串。
// 返回 null 表示 katex 不可用（懒加载失败）。
function renderMathSync(latex, displayMode) {
  if (!katexRenderer) return null;
  try {
    return katexRenderer(latex, { throwOnError: false, displayMode });
  } catch {
    // 兜底：throwOnError=false 已防大部分情况；再保险返回原始 LaTeX 文本
    return `<span class="katex-error">${escapeHtml(latex)}</span>`;
  }
}

function escapeHtml(s) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

// MathRender 组件：接受一段 LaTeX 源码 + 模式（inline/display），懒加载 katex
// 后渲染为 <span class="katex">HTML；加载中显示占位文本；失败则显示原始 LaTeX。
export default function MathRender({ latex, displayMode = false }) {
  const [html, setHtml] = useState(null);
  const [loading, setLoading] = useState(true);
  const latexRef = useRef(latex);
  latexRef.current = latex;

  useEffect(() => {
    let cancelled = false;
    (async () => {
      await ensureKatexLoader();
      if (cancelled) return;
      const rendered = renderMathSync(latexRef.current, displayMode);
      setHtml(rendered);
      setLoading(false);
    })();
    return () => { cancelled = true; };
  }, [displayMode]);

  if (loading) {
    return <span className="katex-placeholder">…</span>;
  }
  if (!html) {
    return <span className="katex-error">{escapeHtml(latex)}</span>;
  }
  // katex 已生成 <span class="katex"> 外壳；外层再包一层 <span class="math">
  // 便于 CSS 选择器与调试。
  return (
    <span className="math">
      <span dangerouslySetInnerHTML={{ __html: html }} />
    </span>
  );
}
