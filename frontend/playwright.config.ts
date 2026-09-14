import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:18080', trace: 'retain-on-failure' },
  webServer: {
    command: 'go run ../cmd/testfixture',
    url: 'http://127.0.0.1:18080/api/session',
    reuseExistingServer: false,
    timeout: 120_000,
  },
})
