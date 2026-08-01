// Playwright E2E 配置。
//
// 两个 project 物理隔离:
//   - mock: 不依赖后端的应用内 Mock API 模式(ci / real-time / ai-panel)
//   - e2e:  需要真实 Go 后端(见 tests/e2e.spec.mjs)
//
// Vite dev server 由 webServer 托管(reuseExistingServer: CI 已手动起
// vite → 复用;本地缺失 → Playwright 自动拉起,避免后台进程被环境回收
// 导致"dev server 服务旧代码/连不上")。Go 后端 e2e 需要手动启动
// (见 docs/testing.md)。
//
// 运行方式:
//   npm run test:e2e        # 全部
//   npm run test:e2e:mock   # 仅 mock 模式
//   npm run test:e2e:full   # 仅真实后端
import { defineConfig } from '@playwright/test';

// 本机开发环境可能导出 HTTP(S)_PROXY(例如 Windows 侧代理)。Node/Playwright
// 的 fetch 会把 127.0.0.1 也代理出去 → webServer 健康检查拿到代理的 500,
// 误判"服务不可用"然后自己再起一个 vite(端口冲突,永远超时)。
// 这里强制本地直连;CI 无代理环境变量,此行为是 no-op。
process.env.NO_PROXY = [process.env.NO_PROXY, '127.0.0.1,localhost'].filter(Boolean).join(',');
process.env.no_proxy = process.env.NO_PROXY;

export default defineConfig({
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
  },
  webServer: {
    command: 'npx vite --port 5173',
    // 用 127.0.0.1 而非 localhost:Node 的 fetch 会把 localhost 解析为
    // IPv6 ::1,而 vite 默认只监听 IPv4 → 健康检查永远失败(ECONNREFUSED)。
    url: 'http://127.0.0.1:5173',
    reuseExistingServer: true,
    // WSL 挂载盘上依赖预构建较慢,给足启动时间
    timeout: 120000,
  },
  projects: [
    {
      name: 'mock',
      testMatch: /(ci|real-time|ai-panel)\.spec\.mjs/,
    },
    {
      name: 'e2e',
      testMatch: /e2e\.spec\.mjs/,
      // 串行执行:真实后端有 5 次/分钟/IP 的注册限流,beforeAll 在多个
      // worker 里会重复执行并叠加注册次数,串行可保证注册总数可控。
      workers: 1,
      // beforeAll 用户池在限流窗口内可能多次重试(10s/次),超时放宽。
      timeout: 180000,
    },
  ],
});
