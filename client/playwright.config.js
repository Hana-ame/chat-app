import { defineConfig } from '@playwright/test';

export default defineConfig({
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:5173',
    headless: true,
  },
  // CI 有两个配置文件，用环境变量 PLAYWRIGHT_PROJECT 区分
  // 默认 ci.spec.js（不依赖后端，纯前端 Mock 测试）
  // PLAYWRIGHT_PROJECT=full 跑 e2e.spec.js（需要后端运行）
});
