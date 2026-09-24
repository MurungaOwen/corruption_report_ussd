// @ts-check
const { defineConfig, devices } = require('@playwright/test');

// The Go server is started and seeded separately (see e2e/README.md and
// Makefile target `make e2e`) rather than via Playwright's own `webServer`
// auto-start, so the same fixed test database can be seeded with
// deterministic fixtures (cmd/e2eseed) before any test runs.
const baseURL = process.env.E2E_BASE_URL || 'http://localhost:8097';

module.exports = defineConfig({
  testDir: './tests',
  fullyParallel: false, // shared server-side state (reports, officials) — keep it simple and deterministic
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never', outputFolder: 'report' }]],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  // PLAYWRIGHT_CHROMIUM_PATH lets a pre-provisioned environment (like this
  // one) point at an already-installed browser instead of `npx playwright
  // install` downloading one; unset it anywhere else and Playwright finds
  // its own managed browser as usual.
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        ...(process.env.PLAYWRIGHT_CHROMIUM_PATH
          ? { launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH } }
          : {}),
      },
    },
  ],
});
