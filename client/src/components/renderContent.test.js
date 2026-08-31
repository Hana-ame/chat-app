// renderContent.test.js — 测试纯 JS 层的 tokenizer（不需要 DOM）。
// 【本地改动 2026-09-02】覆盖 LaTeX 公式检测的各种边界：行内、独立行、
// 金额不应误识别、$$ 优先于 $、嵌套 $ 不匹配。

import { describe, it, expect } from 'vitest';
import { tokenizeMath } from './renderContent.jsx';

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
