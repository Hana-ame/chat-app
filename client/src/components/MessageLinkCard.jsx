// MessageLinkCard.jsx — 消息正文内的链接富卡片（【本地改动 2026-09-03】）
//
// 在消息内容下方渲染首 URL 的 OGP 卡片（标题/描述/站点/缩略图）。
// 数据来自 useLinkCard（共享缓存 + 浏览器 fetch）。CORS 失败时降级为
// 「仅域名 + URL」的浅卡片，不打扰阅读。点击整卡在新标签打开原 URL。
//
// 与 chatto 差异（轻量版，与 Composer 链接预览同源）：无服务端抓取/存储，
// 预览图直接引用第三方 URL（referrerPolicy=no-referrer）。

import { useState } from 'react';
import { useLinkCard } from '../hooks/useLinkCard';

function domainOf(url) {
  try { return new URL(url).hostname; } catch { return ''; }
}

export default function MessageLinkCard({ content }) {
  const [imgFailed, setImgFailed] = useState(false);
  const state = useLinkCard(content);
  if (!state) return null;

  const host = domainOf(state.url);
  const meta = state.meta || {};

  return (
    <a
      href={state.url}
      target="_blank"
      rel="noreferrer"
      style={{
        display: 'block',
        marginTop: 4,
        maxWidth: 420,
        textDecoration: 'none',
        color: 'inherit',
        border: '1px solid var(--border)',
        borderRadius: 8,
        overflow: 'hidden',
        background: 'var(--bg-secondary)',
      }}
    >
      {state.status === 'loading' ? (
        <div style={{ padding: '10px 12px', fontSize: 12, color: 'var(--text-muted)' }}>正在获取链接预览…</div>
      ) : state.status === 'ok' && meta.title ? (
        <div style={{ display: 'flex' }}>
          <div style={{ flex: 1, minWidth: 0, padding: '8px 10px' }}>
            <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{meta.title}</div>
            {meta.description && (
              <div style={{ fontSize: 12, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginTop: 2 }}>{meta.description}</div>
            )}
            <div style={{ fontSize: 11, color: 'var(--accent)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginTop: 2 }}>{meta.siteName || host || state.url}</div>
          </div>
          {meta.image && !imgFailed && (
            <div style={{ flexShrink: 0 }}>
              <img
                src={meta.image}
                alt=""
                loading="lazy"
                referrerPolicy="no-referrer"
                onError={() => setImgFailed(true)}
                style={{ width: 84, height: 84, objectFit: 'cover', display: 'block' }}
              />
            </div>
          )}
        </div>
      ) : (
        <div style={{ padding: '8px 10px' }}>
          <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{host || state.url}</div>
          <div style={{ fontSize: 11, color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{state.url}</div>
        </div>
      )}
    </a>
  );
}
