import { defineConfig, devices } from "@playwright/test";

const backendPort = process.env.PACKAGE_R_PORT || "8888";
const frontendPort = process.env.PLAYWRIGHT_FRONTEND_PORT || "5173";
const backendURL = `http://127.0.0.1:${backendPort}`;
const frontendURL = `http://127.0.0.1:${frontendPort}`;

export default defineConfig({
  testDir: "./tests",
  // Tests share one backend user.
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  // Screenshot tests maintain the documentation images.
  snapshotPathTemplate: "../docs/imgs/screenshots/{arg}{ext}",
  use: {
    baseURL: frontendURL,

    trace: "on-first-retry",

    locale: "en-US",
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },

    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
  ],

  webServer: [
    {
      command: `PACKAGE_R_PORT=${backendPort} exec ../scripts/playwright_backend.sh`,
      url: `${backendURL}/health`,
      reuseExistingServer: false,
      gracefulShutdown: { signal: "SIGTERM", timeout: 5000 },
      timeout: 180 * 1000,
    },
    {
      command: `PLAYWRIGHT_STAC_BROWSER_URL=http://localhost:8080/external/ PACKAGE_R_PORT=${backendPort} exec node node_modules/vite/bin/vite.js --host 127.0.0.1 --port ${frontendPort} --strictPort`,
      url: frontendURL,
      reuseExistingServer: false,
      gracefulShutdown: { signal: "SIGTERM", timeout: 5000 },
      timeout: 180 * 1000,
    },
  ],
});
