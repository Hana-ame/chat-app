const IS_PAGES = typeof window !== 'undefined' && window.location.hostname.endsWith('pages.dev');

export function validateEnv() {
  const missing = [];
  if (!import.meta.env.VITE_API_BASE && !IS_PAGES) missing.push('VITE_API_BASE');
  if (!import.meta.env.VITE_UPLOAD_BASE) missing.push('VITE_UPLOAD_BASE');
  if (!import.meta.env.VITE_WS_URL && !IS_PAGES) missing.push('VITE_WS_URL');
  if (missing.length) {
    console.error(`[config] Missing env vars: ${missing.join(', ')}. Create client/.env (see client/.env.example).`);
  }
}

export const API_BASE = import.meta.env.VITE_API_BASE || (IS_PAGES ? 'https://chat.moonchan.xyz' : '');
export const UPLOAD_BASE = import.meta.env.VITE_UPLOAD_BASE || 'https://upload.moonchan.xyz';
