// linkPreview.test.js — 链接预览纯函数测试（【本地改动 2026-09-03】轻量版）
// 发现背景：URL 提取 / OGP 解析是链接预览的核心纯逻辑，单测覆盖防回归
// （尤其尾部标点、尖括号包裹、属性顺序、实体解码这些边界）。

import { describe, it, expect } from 'vitest';
import { extractFirstUrl, parseOgpHtml, fetchOgp } from './linkPreview.js';

describe('extractFirstUrl — 提取首个可预览 URL', () => {
  it('纯 URL 返回自身', () => {
    expect(extractFirstUrl('https://example.com')).toBe('https://example.com');
  });

  it('文本中的 URL', () => {
    expect(extractFirstUrl('see https://example.com/page now')).toBe('https://example.com/page');
  });

  it('只取第一个 URL', () => {
    expect(extractFirstUrl('https://a.com and https://b.com')).toBe('https://a.com');
  });

  it('http 也接受', () => {
    expect(extractFirstUrl('http://example.com')).toBe('http://example.com');
  });

  it('非 http(s) 不触发', () => {
    expect(extractFirstUrl('ftp://example.com/file')).toBeNull();
    expect(extractFirstUrl('mailto:user@example.com')).toBeNull();
  });

  it('无 URL 返回 null', () => {
    expect(extractFirstUrl('just text no link')).toBeNull();
    expect(extractFirstUrl('')).toBeNull();
    expect(extractFirstUrl(null)).toBeNull();
    expect(extractFirstUrl(undefined)).toBeNull();
  });

  it('去除尾部句号', () => {
    expect(extractFirstUrl('go to https://example.com.')).toBe('https://example.com');
  });

  it('去除尾部括号逗号引号', () => {
    expect(extractFirstUrl('(https://example.com), ok')).toBe('https://example.com');
    expect(extractFirstUrl('see "https://example.com" now')).toBe('https://example.com');
  });

  it('尖括号包裹的 URL 不触发预览', () => {
    expect(extractFirstUrl('<https://example.com>')).toBeNull();
  });

  it('普通文本里紧跟在 < 前的 URL 不触发', () => {
    expect(extractFirstUrl('see <https://example.com/page>')).toBeNull();
  });

  it('不以 < 开头的 URL 正常触发', () => {
    expect(extractFirstUrl('abc https://example.com')).toBe('https://example.com');
  });
});

describe('parseOgpHtml — OGP 元数据解析', () => {
  const html = `<!doctype html><html><head>
    <title>Fallback Title</title>
    <meta property="og:title" content="Real Title" />
    <meta property="og:description" content="A description here" />
    <meta property="og:image" content="https://img.example.com/cover.jpg" />
    <meta property="og:site_name" content="Example Site" />
  </head></html>`;

  it('解析 og: 字段', () => {
    const m = parseOgpHtml(html);
    expect(m.title).toBe('Real Title');
    expect(m.description).toBe('A description here');
    expect(m.image).toBe('https://img.example.com/cover.jpg');
    expect(m.siteName).toBe('Example Site');
  });

  it('og:title 缺失时回退 <title>', () => {
    const m = parseOgpHtml('<html><head><title>Plain</title></head></html>');
    expect(m.title).toBe('Plain');
  });

  it('description 回退 twitter:description', () => {
    const m = parseOgpHtml('<html><head><meta name="twitter:description" content="tw desc"/></head></html>');
    expect(m.description).toBe('tw desc');
  });

  it('属性顺序 content 在前也识别', () => {
    const m = parseOgpHtml('<html><head><meta content="Back Order" property="og:title"/></head></html>');
    expect(m.title).toBe('Back Order');
  });

  it('解码 HTML 实体', () => {
    const m = parseOgpHtml('<html><head><meta property="og:title" content="A &amp; B &#39;quote&#39;"/></head></html>');
    expect(m.title).toBe("A & B 'quote'");
  });

  it('无 meta 返回空字段', () => {
    const m = parseOgpHtml('<html><body>nothing</body></html>');
    expect(m.title).toBe('');
    expect(m.description).toBe('');
    expect(m.image).toBe('');
    expect(m.siteName).toBe('');
  });

  it('空输入返回空字段', () => {
    const m = parseOgpHtml('');
    expect(m.title).toBe('');
  });
});

describe('fetchOgp — CORS 受限下的降级', () => {
  it('网络错误返回 ok:false reason:cors', async () => {
    // 无 fetch 全局时（node 环境），fetchOgp 应安全失败
    const globalFetch = globalThis.fetch;
    globalThis.fetch = undefined;
    const r = await fetchOgp('https://example.com');
    expect(r.ok).toBe(false);
    expect(r.reason).toBe('cors');
    if (globalFetch !== undefined) globalThis.fetch = globalFetch;
  });

  it('非 2xx 返回 http', async () => {
    const original = globalThis.fetch;
    globalThis.fetch = async () => ({ ok: false, status: 404, text: async () => '' });
    const r = await fetchOgp('https://example.com');
    expect(r.ok).toBe(false);
    expect(r.reason).toBe('http');
    globalThis.fetch = original;
  });
});
