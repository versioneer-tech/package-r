import { expect, type Locator, type Page, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

const itemId = "67793f0b9478720001790586";
const publicShare = "my-share";
const sharedPrefix = "catalog-sample";
const backendBaseURL = `http://127.0.0.1:${process.env.PACKAGE_R_SERVER_PORT || "8888"}`;
const screenshotBackendBaseURL = "http://127.0.0.1:8888";
const thumbnailPath = `openaerialmap-assets/${itemId}/thumbnail.png`;
const publicShareThumbnailPath = `/share/${publicShare}/${thumbnailPath}`;
const catalogPreviewBaseURL =
  "https://radiantearth.github.io/stac-browser/#/external/";
const screenshotOptions = { animations: "disabled", caret: "hide" } as const;
const stableRelativeTime = "a few seconds ago";
const stableScreenshotStyle = `
  *,
  *::before,
  *::after {
    animation-duration: 0s !important;
    animation-delay: 0s !important;
    transition-duration: 0s !important;
    transition-delay: 0s !important;
  }
`;

async function prepareStableScreenshot(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.addInitScript((content) => {
    const style = document.createElement("style");
    style.textContent = content;
    document.documentElement.appendChild(style);
  }, stableScreenshotStyle);
  await page.addStyleTag({ content: stableScreenshotStyle });
}

async function normalizeRelativeTimes(page: Page) {
  await page.evaluate((relativeTime) => {
    for (const time of document.querySelectorAll("time")) {
      time.textContent = relativeTime;
    }

    for (const paragraph of document.querySelectorAll("p[title]")) {
      const strong = paragraph.querySelector("strong");
      if (!strong || !strong.textContent?.toLowerCase().includes("last")) {
        continue;
      }

      for (const node of Array.from(paragraph.childNodes)) {
        if (node !== strong) {
          node.remove();
        }
      }
      paragraph.append(` ${relativeTime}`);
    }
  }, stableRelativeTime);
}

async function loginAsAdmin(page: Page) {
  const authPage = new AuthPage(page);
  await authPage.goto();
  await authPage.loginAs("admin", "admin");
  await expect(page).toHaveTitle(/.*Files - packageR$/);
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

async function expectAndNormalizePresignedURL(
  link: Locator,
  expectedObjectPath: string,
  screenshotText: string
) {
  await expect(link).toHaveAttribute("href", /X-Amz-Signature=/);
  const href = await link.getAttribute("href");
  if (href === null) {
    throw new Error("presigned link has no href");
  }

  const url = new URL(href);
  expect(url.hostname).toBe("127.0.0.1");
  expect(url.pathname).toBe(`/my-bucket/${expectedObjectPath}`);
  expect(url.searchParams.get("X-Amz-Algorithm")).toBe("AWS4-HMAC-SHA256");
  expect(url.searchParams.get("X-Amz-Signature")).toBeTruthy();

  await link.evaluate((element, text) => {
    element.textContent = text;
    element.setAttribute("href", text);
  }, screenshotText);
}

test.describe("packageR use-case UI", () => {
  test.skip(
    ({ browserName }) => browserName !== "chromium",
    "screenshots run on chromium"
  );

  test("lists root development fixtures", async ({ page }) => {
    await loginAsAdmin(page);

    await page.goto("/files/");

    for (const name of [
      sharedPrefix,
      "sample.jpg",
      "sample.json",
      "sample.pdf",
      "sample.txt",
    ]) {
      await expect(page.getByLabel(name)).toBeVisible();
    }
  });

  test("renders authenticated shared data listing", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto(`/files/${sharedPrefix}/`);

    await expect(page.getByLabel("openaerialmap-assets")).toBeVisible();
    await expect(page.getByLabel("catalog.parquet")).toBeVisible();
    await normalizeRelativeTimes(page);
    await expect(page.locator("#listing").first()).toHaveScreenshot(
      "authenticated-catalog-sample-listing.png",
      screenshotOptions
    );
  });

  test("shows configured shares as read-only", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/files/");

    await page.getByLabel(sharedPrefix, { exact: true }).click();
    const shareButton = page.getByRole("button", {
      name: "Share",
      exact: true,
    });
    await expect(shareButton).toBeVisible();
    await shareButton.click();

    const dialog = page.locator("#share");
    const link = dialog.getByRole("link", { name: /my-share/ });
    await expect(link).toHaveAttribute("href", /\/share\/my-share\/$/);
    await expect(dialog.getByRole("button", { name: "New" })).toHaveCount(0);
    await expect(dialog.getByRole("button", { name: "Delete" })).toHaveCount(0);

    const response = await page.request.post(`/api/share/${sharedPrefix}/`);
    expect(response.status()).toBe(404);
  });

  test("renders authenticated image preview", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto(`/files/${sharedPrefix}/${thumbnailPath}`);

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
    await normalizeRelativeTimes(page);
    await expect(page.locator(".share").first()).toHaveScreenshot(
      "my-share-directory.png",
      screenshotOptions
    );
  });

  test("shows public share rclone presigned URL", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareThumbnail(page);
    const presignedLink = infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await expect(presignedLink).toHaveText("Show");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      `${sharedPrefix}/${thumbnailPath}`,
      `${screenshotBackendBaseURL}/api/public/dl/${publicShare}/${thumbnailPath}`
    );
    await normalizeRelativeTimes(page);
    await expect(infoBox).toHaveScreenshot(
      "my-share-presign.png",
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
    await infoBox
      .locator("p", { hasText: "Preview URL:" })
      .evaluate((element) => {
        for (const link of element.querySelectorAll("a")) {
          link.textContent =
            link.textContent?.replace(
              /(https?:\/\/127\.0\.0\.1):\d+/g,
              "$1:8888"
            ) || "";
          link.setAttribute(
            "href",
            (link.getAttribute("href") || "").replace(
              /(https?:\/\/127\.0\.0\.1):\d+/g,
              "$1:8888"
            )
          );
        }
      });
    await normalizeRelativeTimes(page);
    await expect(infoBox).toHaveScreenshot(
      "my-share-preview-url.png",
      screenshotOptions
    );
  });

  test("shows authenticated file info with rclone presigned URL", async ({
    page,
  }) => {
    await prepareStableScreenshot(page);
    await loginAsAdmin(page);

    await page.goto(`/files/${sharedPrefix}/openaerialmap-assets/${itemId}/`);
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
    const presignedLink = modal
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await expect(presignedLink).toHaveText("Show");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      `${sharedPrefix}/${thumbnailPath}`,
      `${screenshotBackendBaseURL}/api/raw/${sharedPrefix}/${thumbnailPath}`
    );
    await normalizeRelativeTimes(page);
    await expect(modal).toHaveScreenshot(
      "authenticated-file-info-presign.png",
      screenshotOptions
    );
  });
});
