// Vitest 单元测试配置(与 Playwright E2E 物理隔离)。
//
// - testDir 只扫 src/**/*.test.{js,ts},E2E spec 在 tests/ 下不被加载
// - environment=node:被测模块是纯逻辑(streamAI/schemas/store reducer),
//   不依赖 DOM;若未来需要 DOM,再按文件加 // @vitest-environment jsdom
// - include 排除 tests/ 目录,避免 vitest 误跑 playwright spec
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'node',
    include: ['src/**/*.test.{js,ts,jsx,tsx}'],
    exclude: ['tests/**', 'node_modules/**', 'dist/**'],
    restoreMocks: true,
  },
})
