// renderContent.test.js — 测试纯 JS 层的 tokenizer（不需要 DOM）。
// 【本地改动 2026-09-02】覆盖 LaTeX 公式检测的各种边界：行内、独立行、
// 金额不应误识别、$$ 优先于 $、嵌套 $ 不匹配。

import { describe, it, expect } from 'vitest';
import { tokenizeMath, proxyImageSource, tokenizeImages } from './renderContent.jsx';

describe('tokenizeMath — LaTeX 公式检测', () => {
  it('无公式文本原样返回', () => {
    const out = tokenizeMath('hello world 123');
    expect(out).toEqual([{ type: 'text', value: 'hello world 123' }]);
  });

  it('行内公式 $E=mc^2$ 被识别为 math-inline', () => {
    const out = tokenizeMath('hello $E=mc^2$ world');
    expect(out).toEqual([
      { type: 'text', value: 'hello ' },
      { type: 'math-inline', value: 'E=mc^2' },
      { type: 'text', value: ' world' },
    ]);
  });

  it('独立行公式 $$E=mc^2$$ 被识别为 math-display', () => {
    const out = tokenizeMath('hello $$E=mc^2$$ world');
    expect(out).toEqual([
      { type: 'text', value: 'hello ' },
      { type: 'math-display', value: 'E=mc^2' },
      { type: 'text', value: ' world' },
    ]);
  });

  it('金额 $10 不被误识别为公式（首字符纯数字）', () => {
    const out = tokenizeMath('cost is $10 or $20');
    expect(out).toEqual([{ type: 'text', value: 'cost is $10 or $20' }]);
  });

  it('$$ 优先于 $（$$...$$ 内不会被误拆为两个 inline）', () => {
    const out = tokenizeMath('$$E=mc^2$$ plus $x+y$');
    expect(out).toEqual([
      { type: 'math-display', value: 'E=mc^2' },
      { type: 'text', value: ' plus ' },
      { type: 'math-inline', value: 'x+y' },
    ]);
  });

  it('多段公式连续出现', () => {
    const out = tokenizeMath('$a+b$ then $c-d$');
    expect(out).toEqual([
      { type: 'math-inline', value: 'a+b' },
      { type: 'text', value: ' then ' },
      { type: 'math-inline', value: 'c-d' },
    ]);
  });

  it('空内容返回空数组', () => {
    expect(tokenizeMath('')).toEqual([]);
  });

  it('转义反斜杠开头公式 \\sqrt{x} 可匹配', () => {
    const out = tokenizeMath('use $\\sqrt{x}$ here');
    expect(out).toEqual([
      { type: 'text', value: 'use ' },
      { type: 'math-inline', value: '\\sqrt{x}' },
      { type: 'text', value: ' here' },
    ]);
  });

  it('$$ 含换行不被识别为公式', () => {
    const out = tokenizeMath('$$E=mc^2\nmore$$');
    expect(out).toEqual([{ type: 'text', value: '$$E=mc^2\nmore$$' }]);
  });

  it('运算符首字符 +x 可匹配', () => {
    const out = tokenizeMath('$+1$ dollar');
    expect(out).toEqual([
      { type: 'math-inline', value: '+1' },
      { type: 'text', value: ' dollar' },
    ]);
  });
});


describe('proxyImageSource — 图片代理 URL 重写', () => {
  it('http(s) 源经代理重写，保留 path/query/fragment', () => {
    const out = proxyImageSource('https://images.example.com/photos/cat.png?token=abc#section');
    expect(out).toContain('https://proxy.moonchan.xyz/photos/cat.png');
    expect(out).toContain('token=abc');
    expect(out).toContain('proxy_host=images.example.com');
    expect(out).toContain('proxy_scheme=https');
    expect(out).toContain('#section');
  });

  it('http 源记录 proxy_scheme=http', () => {
    const out = proxyImageSource('http://images.example.com/cat.png');
    expect(out).toContain('proxy_scheme=http');
  });

  it('无原始 query 时附加 proxy_host/proxy_scheme', () => {
    const out = proxyImageSource('https://images.example.com/cat.png');
    expect(out).toContain('proxy_host=images.example.com');
    expect(out).toContain('proxy_scheme=https');
  });

  it('fragment 位于 proxy 参数之后', () => {
    const out = proxyImageSource('https://images.example.com/cat.png#section');
    expect(out.indexOf('#section')).toBeGreaterThan(out.indexOf('proxy_scheme=https'));
  });

  it('指向 proxy.moonchan.xyz 自身的 URL 直通（避免二次代理环）', () => {
    const src = 'https://proxy.moonchan.xyz/some/image.png';
    expect(proxyImageSource(src)).toBe(src);
  });

  it('指向 proxy.moonchan.xyz 的任意 http 路径也直通（scheme 不校验）', () => {
    const src = 'http://proxy.moonchan.xyz/a/b?c=d';
    expect(proxyImageSource(src)).toBe(src);
  });

  it('相对路径降级为 #', () => {
    expect(proxyImageSource('/relative/cat.png')).toBe('#');
  });

  it('javascript: 降级为 #', () => {
    expect(proxyImageSource('javascript:alert(1)')).toBe('#');
  });

  it('data: 降级为 #', () => {
    expect(proxyImageSource('data:image/png;base64,xyz')).toBe('#');
  });

  it('file: 降级为 #', () => {
    expect(proxyImageSource('file:///etc/passwd')).toBe('#');
  });

  it('ftp: 降级为 #', () => {
    expect(proxyImageSource('ftp://host/file')).toBe('#');
  });

  it('mailto: 降级为 #', () => {
    expect(proxyImageSource('mailto:user@example.com')).toBe('#');
  });

  it('无法解析的 URL 降级为 #', () => {
    expect(proxyImageSource('not-a-url')).toBe('#');
  });

  it('带端口的 host 正确传递 proxy_host', () => {
    const out = proxyImageSource('https://images.example.com:8080/cat.png');
    expect(out).toContain('proxy_host=images.example.com%3A8080');
  });
});

describe('tokenizeImages — inline image 切分', () => {
  it('无图片文本返回空数组', () => {
    expect(tokenizeImages('hello world')).toEqual([{ type: 'text', value: 'hello world' }]);
  });

  it('纯图片返回单个 image token', () => {
    const out = tokenizeImages('![cat](https://example.com/cat.png)');
    expect(out).toEqual([{ type: 'image', alt: 'cat', src: 'https://example.com/cat.png' }]);
  });

  it('图片前后有文本', () => {
    const out = tokenizeImages('hello ![cat](https://example.com/cat.png) world');
    expect(out).toEqual([
      { type: 'text', value: 'hello ' },
      { type: 'image', alt: 'cat', src: 'https://example.com/cat.png' },
      { type: 'text', value: ' world' },
    ]);
  });

  it('多张图片连续出现', () => {
    const out = tokenizeImages('![a](https://a.png) and ![b](https://b.png)');
    expect(out).toEqual([
      { type: 'image', alt: 'a', src: 'https://a.png' },
      { type: 'text', value: ' and ' },
      { type: 'image', alt: 'b', src: 'https://b.png' },
    ]);
  });

  it('alt 可为空字符串', () => {
    const out = tokenizeImages('![](https://example.com/cat.png)');
    expect(out).toEqual([{ type: 'image', alt: '', src: 'https://example.com/cat.png' }]);
  });

  it('alt 不含 ] 字符时整体作为 text 保留（正则安全子集）', () => {
    const out = tokenizeImages('![a]b](https://a.png)');
    expect(out).toEqual([{ type: 'text', value: '![a]b](https://a.png)' }]);
  });

  it('javascript: 源被匹配但不影响切分（src 校验交给 proxyImageSource）', () => {
    const out = tokenizeImages('![x](javascript:alert(1)');
    // 正则 `[^)\s]+` 截断到 `)`，所以 src = `javascript:alert(1`
    expect(out.length).toBe(1);
    expect(out[0].type).toBe('image');
    expect(out[0].src).toBe('javascript:alert(1');
  });

  it('空字符串返回空数组', () => {
    expect(tokenizeImages('')).toEqual([]);
  });

  it('非字符串返回空数组', () => {
    expect(tokenizeImages(null)).toEqual([]);
    expect(tokenizeImages(undefined)).toEqual([]);
    expect(tokenizeImages(123)).toEqual([]);
  });
});

// 【本地改动 2026-09-03】自我提及高亮渲染测试（FDR-006 self-mention styling）。
// 用 react-dom/server 渲染 JSX，验证 @自己 带 mention-self 类、他人提及不带。
import { renderToStaticMarkup } from 'react-dom/server';
import { renderContent } from './renderContent.jsx';

describe('renderContent — 自我提及高亮', () => {
  const SELF = '11111111-1111-4111-8111-111111111111';
  const OTHER = '22222222-2222-4222-8222-222222222222';
  const userMap = { [SELF]: 'me', [OTHER]: 'bob' };

  it('@自己 渲染 mention-self 类', () => {
    const html = renderToStaticMarkup(renderContent(`hello <@${SELF}>`, userMap, SELF));
    expect(html).toContain('mention-self');
    expect(html).toContain('@me');
  });

  it('@他人 不渲染 mention-self 类', () => {
    const html = renderToStaticMarkup(renderContent(`hello <@${OTHER}>`, userMap, SELF));
    expect(html).not.toContain('mention-self');
    expect(html).toContain('@bob');
  });

  it('不传 currentUserId 时不加 mention-self（保持旧行为）', () => {
    const html = renderToStaticMarkup(renderContent(`hi <@${SELF}>`, userMap));
    expect(html).not.toContain('mention-self');
  });

  it('消息中同时有自己与他人提及', () => {
    const html = renderToStaticMarkup(renderContent(`<@${SELF}> and <@${OTHER}>`, userMap, SELF));
    expect(html).toContain('mention-self');
    expect((html.match(/mention-self/g) || []).length).toBe(1);
  });
});
