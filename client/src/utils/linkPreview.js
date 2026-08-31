// linkPreview.js — 消息输入框的链接预览（【本地改动 2026-09-03】轻量版，纯前端）
//
// 背景：chatto FDR-009 允许在输入框为 URL 渲染 OGP 预览卡片，发送前可关闭。
// 用户选定「轻量版」：不走后端（无 SSRF 抓取、无 token、无服务端缓存），
// 直接在浏览器 fetch 目标页读 OGP meta。受 CORS 限制：多数站点不带
// Access-Control-Allow-Origin，fetch 会失败 → 卡片降级为「仅域名 + URL」占位
// （不报错、不打扰）。发送时仅发送 URL 文本（已有 renderContent 渲染为链接），
// 预览卡片不随消息持久化（轻量取舍，与 chatto 的服务端存储不同——此处按
// 用户选定方案实现，不承认派生）。
//
// 本模块只含纯函数（URL 提取 + OGP HTML 解析），可单测；fetch 在 Composer 里调用。

// 【本地改动 2026-09-03】URL 提取正则：与 renderContent.URL_RE 对齐，
// 排除空格 / 尖括号 / 方括号 / 反引号等渲染层视为边界字符。
const URL_RE = /(https?:\/\/[^\s<>[\]{}|\\^`]+)/;

// extractFirstUrl 返回文本中第一个合法 http(s) URL，去除尾部常见标点
// （句号/括号/逗号/引号等聊天语境会粘贴进来的噪音）。
// 返回 null 表示无可预览 URL。
// 边界：
//   - 仅 http/https（chatto 也只对这两类触发预览；ftp:/mailto: 不触发）。
//   - 排除被尖括号包裹的 URL（`<https://...>`）：chatto 里这种写法「保持可点
//     击但不触发预览」；聊天中常见粘贴风格，故按同样语义处理。
//   - 只取第一个 URL（FDR: one preview per message, first URL only）。
export function extractFirstUrl(text) {
  if (!text || typeof text !== 'string') return null;
  URL_RE.lastIndex = 0;
  const m = URL_RE.exec(text);
  if (!m) return null;
  const before = text.slice(0, m.index);
  // 被 `<` 直接包裹（如 `<https://x>`）→ 不触发预览
  if (before.endsWith('<')) return null;
  let url = m[0];
  // 去除尾部标点：`http://x.` / `http://x),` / `http://x"...` 等
  url = url.replace(/[)\].,;:!?'"]+$/, '');
  return url || null;
}

// decodeEntities 解码 HTML 实体（&amp; &lt; &gt; &quot; &#39; &nbsp; 及数字实体）。
function decodeEntities(s) {
  return s
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/gi, ' ')
    .replace(/&#(\d+);/g, (_, n) => String.fromCharCode(Number(n)))
    .replace(/&#x([0-9a-f]+);/gi, (_, n) => String.fromCharCode(parseInt(n, 16)));
}

// metaContent 从 HTML 中提取第一个匹配 name/property 的 meta 标签 content。
// 兼容两种属性顺序：`property="og:title" content="..."` 与 `content="..." property="og:title"`。
function metaContent(html, prop) {
  const p = prop.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const reOrder1 = new RegExp(`<meta[^>]*\\b(?:property|name)=["']${p}["'][^>]*\\bcontent=["']([^"']*)["']`, 'i');
  let m = html.match(reOrder1);
  if (m) return decodeEntities(m[1]);
  const reOrder2 = new RegExp(`<meta[^>]*\\bcontent=["']([^"']*)["'][^>]*\\b(?:property|name)=["']${p}["']`, 'i');
  m = html.match(reOrder2);
  return m ? decodeEntities(m[1]) : '';
}

// parseOgpHtml 从 HTML 字符串解析 OGP 预览所需字段。
// 返回 { title, description, image, siteName }（字段缺失时为空字符串）。
// OGP 缺 title 时回退到 <title> 标签；description 回退到 twitter:description / name=description。
export function parseOgpHtml(html) {
  if (!html || typeof html !== 'string') {
    return { title: '', description: '', image: '', siteName: '' };
  }
  const title =
    metaContent(html, 'og:title') ||
    (html.match(/<title[^>]*>([^<]*)<\/title>/i) || [])[1] ||
    '';
  const description =
    metaContent(html, 'og:description') ||
    metaContent(html, 'twitter:description') ||
    metaContent(html, 'description') ||
    '';
  const image = metaContent(html, 'og:image') || metaContent(html, 'twitter:image') || '';
  const siteName = metaContent(html, 'og:site_name') || '';
  return { title: decodeEntities(title).trim(), description: decodeEntities(description).trim(), image: decodeEntities(image).trim(), siteName: decodeEntities(siteName).trim() };
}

// fetchOgp 在浏览器侧 fetch URL 并解析 OGP。
// 返回 { ok: true, url, ...meta } 或 { ok: false, reason: 'cors'|'http'|'abort'|'error' }。
// 轻量版特意不走后端：无 SSRF 面；代价是 CORS 拦截时多数站点读不到 OGP，
// 由调用方降级渲染（仅显示域名 + URL）。
/**
 * @param {string} url
 * @param {{ signal?: AbortSignal }} [opts]
 */
export async function fetchOgp(url, { signal } = {}) {
  try {
    const res = await fetch(url, { signal, mode: 'cors', redirect: 'follow' });
    if (!res.ok) return { ok: false, reason: 'http' };
    const html = await res.text();
    const meta = parseOgpHtml(html);
    // 完全没有标题 = 大概率被 CORS 拦截返回的是错误页/空白 → 按失败处理
    if (!meta.title && !meta.description && !meta.image) {
      return { ok: false, reason: 'cors' };
    }
    return { ok: true, url, ...meta };
  } catch (e) {
    return { ok: false, reason: e && e.name === 'AbortError' ? 'abort' : 'cors' };
  }
}
