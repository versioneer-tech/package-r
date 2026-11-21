try {
  const { chromium } = await import("@playwright/test");
  const browser = await chromium.launch();
  await browser.close();
  console.log("Playwright Chromium is available.");
} catch (error) {
  console.error("Playwright Chromium check failed.");
  console.error(error.message);
  console.error(
    "From frontend, run: pnpm install --frozen-lockfile && pnpm exec playwright install chromium"
  );
  process.exitCode = 1;
}
