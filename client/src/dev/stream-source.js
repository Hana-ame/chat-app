export function createStreamSource(asyncFn) {
  let onChunk = null;
  let onDone = null;
  let onError = null;

  const emit = (chunk) => { if (onChunk) onChunk(chunk); };

  const promise = new Promise((resolve, reject) => {
    onDone = resolve;
    onError = reject;
  });

  asyncFn(emit)
    .then(() => { if (onDone) onDone(); })
    .catch(err => { if (onError) onError(err); });

  return {
    onChunk(cb) { onChunk = cb; return this; },
    done: promise,
  };
}
