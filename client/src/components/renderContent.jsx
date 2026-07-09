const MENTION_RE = /(<@[a-f0-9-]{36}>)/;
const MENTION_MATCH = /^<@([a-f0-9-]{36})>$/;
const URL_RE = /(https?:\/\/[^\s<>[\]{}|\\^`]+)/g;

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

    const urlParts = part.split(URL_RE);
    for (let j = 0; j < urlParts.length; j++) {
      const sub = urlParts[j];
      if (!sub) continue;
      if (j % 2 === 1) {
        children.push(
          <a key={`l${i}_${j}`} href={sub} target="_blank" rel="noreferrer">{sub}</a>
        );
      } else {
        children.push(sub);
      }
    }
  }

  return children.length > 0 ? children : null;
}
