import { defineConfig, devices } from '@playwright/test'

const artifactPrefix = process.env.PLAYWRIGHT_ARTIFACT_PREFIX ?? 'playwright'

export default defineConfig({
  testDir: './playwright',
  globalSetup: './playwright.global.setup.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: `playwright-report/${artifactPrefix}` }],
    ['junit', { outputFile: `test-results/${artifactPrefix}/junit.xml` }],
  ],
  outputDir: `test-results/${artifactPrefix}/artifacts`,
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'mocked',
      grep: /@mocked/,
      use: {
        ...devices['Desktop Chrome'],
      },
    },
    {
      name: 'smoke',
      grep: /@smoke/,
      use: {
        ...devices['Desktop Chrome'],
      },
    },
    {
      name: 'screenshots',
      grep: /@screenshot/,
      use: {
        ...devices['Desktop Chrome'],
      },
    },
  ],
  webServer: {
    command: 'npm run build && npm run preview -- --host 127.0.0.1 --port 4173',
    port: 4173,
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000,
  },
})
