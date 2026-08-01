// Playwright E2E 配置。
//
// 两个 project 物理隔离:
//   - mock: 不依赖后端的应用内 Mock API 模式(ci / real-time / ai-panel)
//   - e2e:  需要真实 Go 后端(见 tests/e2e.spec.mjs)
//
// 运行方式(均需要先起 Vite dev server,baseURL=http://localhost:5173):
//   npm run test:e2e        # 全部
//   npm run test:e2e:mock   # 仅 mock 模式
//   npm run test:e2e:full   # 仅真实后端
//
// 历史说明:早期配置注释声称的 PLAYWRIGHT_PROJECT 环境变量机制从未实现,
// 已在 projects 中落地;README 声称的 webServer 也从未配置,CI 里手动起
// Vite,这里保持一致(不在配置里隐式起服务)。
import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
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
    },
  ],
});
