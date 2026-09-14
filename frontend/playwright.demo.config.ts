import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  workers: 1,
  use: { baseURL: 'http://127.0.0.1:19080', trace: 'retain-on-failure' },
  webServer: {
    command:
      'go run ../cmd/localdemo -database ../.local/demo-browser/resources.db -web-directory dist',
    url: 'http://127.0.0.1:19080/api/session',
    reuseExistingServer: false,
    timeout: 120_000,
  },
})
