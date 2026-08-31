// useLinkCard.js — 消息内链接卡片数据 hook（【本地改动 2026-09-03】）
//
// 背景：消息正文的 URL 目前只渲染为普通链接；chatto FDR-009 的"已发送消息带预览"
// 需要把首 URL 渲染成富卡片。纯前端轻量版（与 Composer 的链接预览同源）：
// 浏览器 fetch OGP，受 CORS 限制失败时降级为仅域名卡片。
//
// 关键设计：模块级共享缓存（Map）。消息列表会反复重渲染/滚动加载同一批消息，
// 若每条消息每次都 fetch，会重复打爆第三方站点。缓存 key=URL，TTL 24h，
// 进程内有效（刷新页面后失效可接受）。降级失败也缓存（短 TTL 1h），避免
// 每次滚动都重试 CORS 失败页。

import { useMemo, useState, useEffect, useRef } from 'react';
import { extractFirstUrl, fetchOgp } from '../utils/linkPreview';

const POSITIVE_TTL = 24 * 3600 * 1000; // 成功缓存 24h
const NEGATIVE_TTL = 1 * 3600 * 1000;  // 失败缓存 1h（防反复重试）
const cache = new Map(); // url -> { ok, meta, at, reason }

function cacheGet(url) {
  const hit = cache.get(url);
  if (!hit) return null;
  const ttl = hit.ok ? POSITIVE_TTL : NEGATIVE_TTL;
  if (Date.now() - hit.at < ttl) return hit;
  cache.delete(url);
  return null;
}
function cacheSet(url, entry) {
  cache.set(url, { ...entry, at: Date.now() });
  if (cache.size > 200) {
    // 简单 LRU 兜底：清掉最旧的一半
    const keys = [...cache.keys()];
    for (let i = 0; i < keys.length / 2; i++) cache.delete(keys[i]);
  }
}

/**
 * 返回消息首 URL 的预览数据（status: loading/ok/fail）。
 * @param {string} content 消息正文
 */
export function useLinkCard(content) {
  const url = useMemo(() => extractFirstUrl(content), [content]);
  // 注意：不给 useState 加 @type 注解。checkJs 下若注解与推断不一致，
  // TS 会把「注解类型」误当成整个 [state,setState] 元组 → 报元组不可赋值。
  // 让 TS 从初始值推断 status 联合即可。
  const [state, setState] = useState(() => {
    if (!url) return null;
    const hit = cacheGet(url);
    if (hit) return { url, status: hit.ok ? 'ok' : 'fail', meta: hit.meta || null, reason: hit.reason };
    return { url, status: 'loading', meta: null };
  });
  const urlRef = useRef(url);
  urlRef.current = url;

  useEffect(() => {
    if (!url) { setState(null); return; }
    const hit = cacheGet(url);
    if (hit) {
      setState({ url, status: hit.ok ? 'ok' : 'fail', meta: hit.meta || null, reason: hit.reason });
      return;
    }
    let cancelled = false;
    setState({ url, status: 'loading', meta: null });
    fetchOgp(url).then(r => {
      if (cancelled || urlRef.current !== url) return;
      cacheSet(url, r);
      setState(r.ok ? { url, status: 'ok', meta: r } : { url, status: 'fail', meta: null, reason: r.reason });
    });
    return () => { cancelled = true; };
  }, [url]);

  return state;
}
