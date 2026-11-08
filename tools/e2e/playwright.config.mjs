import { defineConfig, devices } from '@playwright/test';

// The browser suite runs against the demo (make e2e builds it as ./demo). The demo keeps its state in memory and the scenarios share it (a sync job, the
// upload history), so they run one at a time.
export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['github']] : 'list',
  use: {
    ...devices['Desktop Chrome'],
    baseURL: 'http://localhost:7792',
    trace: 'off',
  },
  webServer: {
    command: './demo -addr :7792',
    url: 'http://localhost:7792/login',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000,
    // The demo has pages that fail on purpose (/loading/failures), and its Logger says so on stderr, as it should. That is not a failure of the suite, and
    // in its output it reads as one, so it is left out. E2E_LOGS=1 brings it back, to see why a scenario fails.
    stderr: process.env.E2E_LOGS ? 'pipe' : 'ignore',
  },
});
