// LinkPreviewCard.jsx — 输入框链接预览卡片（【本地改动 2026-09-03】轻量版，纯前端）
//
// 显示位置：Composer 输入框上方。触发：输入文本里检测到首个合法 http(s) URL。
// 状态：
//   - loading：fetch 中，显示浅色占位。
//   - ok：OGP 解析成功 → 标题 / 描述 / 站点名 / 缩略图（加载失败或缺图时隐藏图）。
//   - fail（CORS / 非 2xx / 无 OGP）：降级为「仅域名 + URL」占位卡片（不打扰）。
// 交互：右上 × 关闭；关闭后本合成会话内记住该 URL（不再自动重新触发）。
//
// 与 chatto FDR-009 的差异（按用户选定「轻量版」实现，不承认派生）：
//   - 无服务端 SSRF 抓取、无 token、无缓存；fetch 直接在浏览器发。
//   - 预览不随消息持久化（发送的仍是纯 URL 文本，renderContent 渲染为链接）。
//   - 无 YouTube / 社交专卡。

import { useState } from 'react';

function domainOf(url) {
  try { return new URL(url).hostname; } catch { return ''; }
}

export default function LinkPreviewCard({ url, status, meta, onDismiss }) {
  const [imgFailed, setImgFailed] = useState(false);
  const host = domainOf(url);

  const cardStyle = {
    display: 'flex',
    gap: 10,
    padding: '8px 10px',
    marginBottom: 4,
    background: 'var(--bg-secondary)',
    border: '1px solid var(--border)',
    borderRadius: 6,
    fontSize: 12,
    alignItems: 'center',
  };

  return (
    <div style={cardStyle}>
      <div style={{ flex: 1, minWidth: 0 }}>
        {status === 'loading' ? (
          <div style={{ color: 'var(--text-muted)' }}>正在获取链接预览…</div>
        ) : status === 'ok' ? (
          <>
            {meta?.title && <div style={{ fontWeight: 600, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{meta.title}</div>}
            {meta?.description && <div style={{ color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{meta.description}</div>}
            <div style={{ color: 'var(--accent)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
              {(meta?.siteName || host || url)}
            </div>
          </>
        ) : (
          <>
            <div style={{ fontWeight: 600, color: 'var(--text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{host || url}</div>
            <div style={{ color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{url}</div>
          </>
        )}
      </div>

      {status === 'ok' && meta?.image && !imgFailed && (
        <div style={{ flexShrink: 0 }}>
          <img
            src={meta.image}
            alt=""
            loading="lazy"
            referrerPolicy="no-referrer"
            onError={() => setImgFailed(true)}
            style={{ width: 64, height: 64, objectFit: 'cover', borderRadius: 4, background: 'var(--bg-primary)' }}
          />
        </div>
      )}

      <button
        className="btn-ghost"
        onClick={onDismiss}
        title="Remove preview"
        style={{ flexShrink: 0, fontSize: 14, lineHeight: 1, width: 20, height: 20, borderRadius: '50%', padding: 0 }}
      >×</button>
    </div>
  );
}
