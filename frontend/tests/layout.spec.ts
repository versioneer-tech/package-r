import { expect, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

test("closes the latest overlay first", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const auth = new AuthPage(page);
  await auth.goto();
  await auth.loginAs();
  await expect(page.getByLabel("sample-files", { exact: true })).toBeVisible();

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
  await expect(page.getByLabel("sample-files", { exact: true })).toBeVisible();
  const folder = "S2B_T33UXP_20260218T100524_L2A";
  await page.goto(`/files/vienna-s2l2a-26/${folder}/`);
  await expect(page.getByLabel("overview.tif", { exact: true })).toBeVisible();
  await page
    .locator(".breadcrumbs")
    .getByTitle("vienna-s2l2a-26")
    .click();
  await expect(page.getByLabel(folder, { exact: true })).toBeVisible();
});
