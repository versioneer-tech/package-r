import { expect, type Locator, type Page, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

const itemId = "67793f0b9478720001790586";
const publicShare = "public-share";
const thumbnailPath = `openaerialmap-assets/${itemId}/thumbnail.png`;
const publicShareThumbnailPath = `/share/${publicShare}/${thumbnailPath}`;
const catalogPreviewBaseURL =
  "https://radiantearth.github.io/stac-browser/#/external/";
const screenshotOptions = { animations: "disabled", caret: "hide" } as const;

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

async function expectImageLoaded(image: Locator) {
  await expect
    .poll(
      () =>
        image.evaluate(
          (element) =>
            element instanceof HTMLImageElement &&
            element.complete &&
            element.naturalWidth > 0 &&
            element.naturalHeight > 0 &&
            element.classList.contains("image-ex-img-ready")
        ),
      {
        timeout: 15_000,
      }
    )
    .toBe(true);
  await expect(image).toBeVisible({ timeout: 10_000 });
}

async function openPublicShareThumbnail(page: Page) {
  await page.goto(publicShareThumbnailPath);
  const infoBox = page.locator(".share__box__info");
  await expect(infoBox).toContainText("thumbnail.png");
  return infoBox;
}

test.describe("packageR use-case UI", () => {
  test.skip(
    ({ browserName }) => browserName !== "chromium",
    "screenshots run on chromium"
  );

  test("renders authenticated public data listing", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto("/files/public/");

    await expect(page.getByLabel("openaerialmap-assets")).toBeVisible();
    await expect(page.getByLabel("catalog.parquet")).toBeVisible();
    await expect(page.locator("#listing").first()).toHaveScreenshot(
      "authenticated-public-listing.png",
      screenshotOptions
    );
  });

  test("renders authenticated image preview", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto(`/files/public/${thumbnailPath}`);

    const preview = page.locator("#previewer .preview");
    await expectImageLoaded(preview.locator("img.image-ex-img"));
    await expect(preview).toHaveScreenshot(
      "authenticated-image-preview.png",
      screenshotOptions
    );
  });

  test("renders public share directory listing", async ({ page }) => {
    await prepareStableScreenshot(page);

    await page.goto(`/share/${publicShare}/`);

    await expect(page.getByText("openaerialmap-assets")).toBeVisible();
    await expect(page.getByText("catalog.parquet")).toBeVisible();
    await expect(page.locator(".share").first()).toHaveScreenshot(
      "public-share-directory.png",
      screenshotOptions
    );
  });

  test("shows public share presigned URL fallback", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareThumbnail(page);
    await infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(
      `/api/public/dl/${publicShare}/${thumbnailPath}`
    );
    await expect(infoBox).toHaveScreenshot(
      "public-share-presign.png",
      screenshotOptions
    );
  });

  test("shows public share catalog preview URL", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareThumbnail(page);
    await infoBox
      .locator("p", { hasText: "Preview URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(catalogPreviewBaseURL);
    await expect(infoBox).toContainText(
      `/api/public/catalog/${publicShare}/${thumbnailPath}`
    );
    await expect(infoBox).toHaveScreenshot(
      "public-share-preview-url.png",
      screenshotOptions
    );
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
    await expect(modal).toHaveScreenshot(
      "authenticated-file-info-presign.png",
      screenshotOptions
    );
  });
});
