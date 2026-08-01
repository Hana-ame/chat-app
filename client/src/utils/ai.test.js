// streamAI 单元测试:SSE 流解析器。
//
// 覆盖:
//   - data: 行解析(content / reasoning_content / type 字段分派)
//   - [DONE] 结束信号
//   - chunk 跨边界切割(缓冲拼接)
//   - 非 JSON 行静默忽略
//   - 无 body 报错 / AbortError 静默 / cancel 停止
//
// 运行: cd client && npx vitest run src/utils/ai.test.js
import { describe, it, expect, vi } from 'vitest';
import { streamAI } from './ai';

const enc = new TextEncoder();

/** 构造一个按顺序吐出各 chunk 的 ReadableStream(字符串输入自动编码)。 */
function makeStream(chunks) {
  return new ReadableStream({
    start(controller) {
      (async () => {
        for (const c of chunks) controller.enqueue(enc.encode(c));
        controller.close();
      })();
    },
  });
}

/** 收集 streamAI 的全部回调,返回 { chunks, done, errors } 和 stream 句柄。 */
function collect(stream) {
  const chunks = [];
  const done = vi.fn();
  const errors = [];
  const h = streamAI({ body: stream }, c => chunks.push(c), done, e => errors.push(e));
  return { chunks, done, errors, handle: h };
}

describe('streamAI', () => {
  it('解析 content 行并按顺序回调', async () => {
    const { chunks, done, errors } = collect(makeStream([
      'data: {"content":"Hello"}\n\n',
      'data: {"content":" world"}\n\n',
      'data: [DONE]\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([
      { type: 'content', content: 'Hello' },
      { type: 'content', content: ' world' },
    ]);
    expect(done).toHaveBeenCalled();
    expect(errors).toEqual([]);
  });

  it('分派 reasoning_content 为 thinking', async () => {
    const { chunks } = collect(makeStream([
      'data: {"reasoning_content":"think step 1"}\n\n',
      'data: {"content":"answer"}\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([
      { type: 'thinking', content: 'think step 1' },
      { type: 'content', content: 'answer' },
    ]);
  });

  it('支持显式 type: reasoning/content 字段', async () => {
    const { chunks } = collect(makeStream([
      'data: {"type":"reasoning","content":"r1"}\n\n',
      'data: {"type":"content","content":"c1"}\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([
      { type: 'thinking', content: 'r1' },
      { type: 'content', content: 'c1' },
    ]);
  });

  it('chunk 跨边界切割时缓冲拼接正确', async () => {
    // 故意把一行 data 拆成 3 个 chunk,验证内部 buffer 拼接。
    const { chunks, done } = collect(makeStream([
      'data: {"conten',
      't":"half split"',
      '}\n\ndata: [DONE]\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([{ type: 'content', content: 'half split' }]);
    expect(done).toHaveBeenCalled();
  });

  it('忽略非 data 行与空行', async () => {
    const { chunks, errors } = collect(makeStream([
      ': comment\n',
      'event: message\n\n',
      'data: {"content":"real"}\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([{ type: 'content', content: 'real' }]);
    expect(errors).toEqual([]);
  });

  it('忽略非 JSON 的 data 行(容错,不报错)', async () => {
    const { chunks, errors } = collect(makeStream([
      'data: not-json\n\n',
      'data: {"content":"ok"}\n\n',
    ]));
    await new Promise(r => setTimeout(r, 20));
    expect(chunks).toEqual([{ type: 'content', content: 'ok' }]);
    expect(errors).toEqual([]);
  });

  it('无 body 时走 onError', async () => {
    const errors = [];
    const done = vi.fn();
    streamAI({ body: null }, () => {}, done, e => errors.push(e));
    expect(errors).toHaveLength(1);
    expect(errors[0].message).toBe('Response body is null');
    expect(done).not.toHaveBeenCalled();
  });

  it('AbortError 静默忽略(不算错误)', async () => {
    const stream = new ReadableStream({
      start(c) {
        c.error(Object.assign(new Error('abort'), { name: 'AbortError' }));
      },
    });
    const errors = [];
    await new Promise(r => setTimeout(r, 20));
    streamAI({ body: stream }, () => {}, () => {}, e => errors.push(e));
    await new Promise(r => setTimeout(r, 20));
    expect(errors).toEqual([]);
  });

  it('流内真实错误回调 onError', async () => {
    const stream = new ReadableStream({
      start(c) { c.error(new Error('network broke')); },
    });
    const errors = [];
    streamAI({ body: stream }, () => {}, () => {}, e => errors.push(e));
    await new Promise(r => setTimeout(r, 20));
    expect(errors).toHaveLength(1);
    expect(errors[0].message).toBe('network broke');
  });

  it('cancel 停止后续读取', async () => {
    const { handle, done } = collect(makeStream([
      'data: {"content":"a"}\n\n',
      'data: {"content":"b"}\n\n',
    ]));
    handle.cancel();
    await new Promise(r => setTimeout(r, 20));
    // cancel 后不再触发 done(读取被中止)。
    expect(done).not.toHaveBeenCalled();
  });
});
