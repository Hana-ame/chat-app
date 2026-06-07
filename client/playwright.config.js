import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:8080',
    headless: true,
  },
  webServer: {
    command: 'cd .. && go run server/cmd/chatd',
    port: 8080,
    timeout: 60000,
    reuseExistingServer: true,
  },
});
