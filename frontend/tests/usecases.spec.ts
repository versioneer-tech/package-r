import { expect, type Page, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

const itemId = "67793f0b9478720001790586";
const publicShare = "public-share";
const thumbnailPath = `openaerialmap-assets/${itemId}/thumbnail.png`;

async function prepareStableScreenshot(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.addStyleTag({
    content: `
      *,
      *::before,
      *::after {
        animation-duration: 0s !important;
        animation-delay: 0s !important;
        transition-duration: 0s !important;
        transition-delay: 0s !important;
      }
      time {
        visibility: hidden !important;
      }
    `,
  });
}

async function loginAsAdmin(page: Page) {
  const authPage = new AuthPage(page);
  await authPage.goto();
  await authPage.loginAs("admin", "admin");
  await expect(page).toHaveTitle(/.*Files - File Browser$/);
}

test.describe("packageR use-case UI", () => {
  test.skip(
    ({ browserName }) => browserName !== "chromium",
    "screenshots run on chromium"
  );

  test("renders public share directory listing", async ({ page }) => {
    await prepareStableScreenshot(page);

    await page.goto(`/share/${publicShare}/`);

    await expect(page.getByText("openaerialmap-assets")).toBeVisible();
    await expect(page.getByText("catalog.parquet")).toBeVisible();
    await expect(page.locator(".share").first()).toHaveScreenshot(
      "public-share-directory.png",
      { animations: "disabled", caret: "hide" }
    );
  });

  test("shows public share presigned URL fallback", async ({ page }) => {
    await prepareStableScreenshot(page);

    await page.goto(`/share/${publicShare}/${thumbnailPath}`);

    const infoBox = page.locator(".share__box__info");
    await expect(infoBox).toContainText("thumbnail.png");
    await infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(
      `/api/public/dl/${publicShare}/${thumbnailPath}`
    );
    await expect(infoBox).toHaveScreenshot("public-share-presign.png", {
      animations: "disabled",
      caret: "hide",
    });
  });

  test("shows authenticated file info with presigned URL fallback", async ({
    page,
  }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto(`/files/public/openaerialmap-assets/${itemId}/`);
    await expect(page.getByLabel("thumbnail.png")).toBeVisible();
    await page.getByLabel("thumbnail.png").click();
    await expect(page.getByLabel("thumbnail.png")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    await page.getByLabel("Info").click();
    const modal = page.locator(".card.floating").filter({
      has: page.getByRole("heading", { name: "File information" }),
    });
    await expect(modal).toBeVisible();
    await modal
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(modal).toContainText(
      `/api/raw/public/openaerialmap-assets/${itemId}/thumbnail.png`
    );
    await expect(modal).toHaveScreenshot("authenticated-file-info-presign.png", {
      animations: "disabled",
      caret: "hide",
    });
  });
});
