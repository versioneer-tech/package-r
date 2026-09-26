import { defineConfig, devices } from "@playwright/test";

const backendPort = process.env.PACKAGE_R_PORT || "8888";
const frontendPort = process.env.PLAYWRIGHT_FRONTEND_PORT || "5173";
const backendURL = `http://127.0.0.1:${backendPort}`;
const frontendURL = `http://127.0.0.1:${frontendPort}`;

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig({
  testDir: "./tests",
  /* The tests share one backend user and run in sequence. */
  fullyParallel: false,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: 1,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: "html",
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: frontendURL,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",

    /* Set default locale to English (US) */
    locale: "en-US",
  },

  /* Configure projects for major browsers */
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },

    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },

    // {
    //   name: "webkit",
    //   use: { ...devices["Desktop Safari"] },
    // },

    /* Test against mobile viewports. */
    // {
    //   name: 'Mobile Chrome',
    //   use: { ...devices['Pixel 5'] },
    // },
    // {
    //   name: 'Mobile Safari',
    //   use: { ...devices['iPhone 12'] },
    // },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: { ...devices['Desktop Edge'], channel: 'msedge' },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    // },
  ],

  /* Run local backend and frontend dev servers before starting the tests */
  webServer: [
    {
      command: `PACKAGE_R_PORT=${backendPort} exec ../scripts/playwright_backend.sh`,
      url: `${backendURL}/health`,
      reuseExistingServer: !process.env.CI,
      gracefulShutdown: { signal: "SIGTERM", timeout: 5000 },
      timeout: 180 * 1000,
    },
    {
      command: `PACKAGE_R_PORT=${backendPort} exec node node_modules/vite/bin/vite.js --port ${frontendPort}`,
      url: frontendURL,
      reuseExistingServer: !process.env.CI,
      gracefulShutdown: { signal: "SIGTERM", timeout: 5000 },
      timeout: 180 * 1000,
    },
  ],
});
