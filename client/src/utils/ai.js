export function streamAI(response, onChunk, onDone, onError) {
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  function parseSSE(line) {
    if (!line.startsWith('data: ')) return;
    const payload = line.slice(6);
    if (payload === '[DONE]') return;
    try {
      const json = JSON.parse(payload);
      if (json.content) {
        onChunk?.({ type: 'content', content: json.content });
      }
    } catch {}
  }

  function pump() {
    reader.read().then(({ done, value }) => {
      if (done) {
        if (buffer.trim()) parseSSE(buffer.trim());
        onDone?.();
        return;
      }
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';
      for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed) parseSSE(trimmed);
      }
      pump();
    }).catch(err => onError?.(err));
  }

  pump();
  return { cancel: () => reader.cancel() };
}
