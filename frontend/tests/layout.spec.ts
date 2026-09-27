import { expect, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

test("closes the latest overlay first", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const auth = new AuthPage(page);
  await auth.goto();
  await auth.loginAs();
  await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "More", exact: true }).click();
  await page.getByRole("button", { name: "Info", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "File information" })
  ).toBeVisible();

  await page.getByRole("button", { name: "OK", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "File information" })
  ).toBeHidden();
  await expect(page.locator("#dropdown")).toHaveClass(/\bactive\b/);

  await page.locator("header .overlay").click({ position: { x: 1, y: 1 } });
  await expect(page.locator("#dropdown")).not.toHaveClass(/\bactive\b/);
});

test("navigates with nested breadcrumbs", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const auth = new AuthPage(page);
  await auth.goto();
  await auth.loginAs();
  await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
  const folder = "67793f0b9478720001790586";
  await page.goto(`/files/catalog-sample/openaerialmap-assets/${folder}/`);
  await expect(page.getByLabel("thumbnail.png", { exact: true })).toBeVisible();
  await page.locator(".breadcrumbs").getByTitle("openaerialmap-assets").click();
  await expect(page.getByLabel(folder, { exact: true })).toBeVisible();
});
