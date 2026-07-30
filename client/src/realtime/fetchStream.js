import { useAuthStore } from '../store/auth';

const getState = () => useAuthStore.getState();

export async function fetchStream(url, msgId, onChunk) {
  const token = getState().accessToken;
  let contentAcc = '';
  let thinkingAcc = '';
  try {
    const res = await fetch(url, { headers: token ? { Authorization: 'Bearer ' + token } : {} });
    if (!res.ok) { console.error('fetchStream: HTTP', res.status); return; }
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = '';
    let streamDone = false;
    while (!streamDone) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += decoder.decode(value, { stream: true });
      const lines = buf.split('\n');
      buf = lines.pop() || '';
      for (const line of lines) {
        const t = line.trim();
        if (!t.startsWith('data: ')) continue;
        const p = t.slice(6);
        if (p === '[DONE]') { streamDone = true; break; }
        try {
          const json = JSON.parse(p);
          if (json.type === 'reasoning' && json.content) {
            thinkingAcc += json.content;
            onChunk(msgId, contentAcc, thinkingAcc);
          } else if (json.content) {
            contentAcc += json.content;
            onChunk(msgId, contentAcc, thinkingAcc);
          }
        } catch {}
      }
    }
    onChunk(msgId, contentAcc, thinkingAcc, true);
  } catch (e) { console.error('fetchStream error:', e); }
}
