import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  define: {
    __APP_VERSION__: JSON.stringify(process.env.APP_VERSION || 'dev'),
  },
  build: {
    sourcemap: true,
    outDir: 'dist',
  },
  server: {
    watch: {
      usePolling: true,
    },
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
      '/uploads': 'http://localhost:8080',
      // 【本地改动 2026-09-02】公开附件 URL 也走代理（本地开发时 /assets/files/
      // 不会命中 Vite 的静态资源，需要转到后端）。
      '/assets': 'http://localhost:8080',
    },
  },
})
