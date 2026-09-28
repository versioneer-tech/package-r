import { expect, type Locator, type Page, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

const itemId = "67793f0b9478720001790586";
const publicShare = "my-share";
const sharedPrefix = "catalog-sample";
const backendBaseURL = `http://127.0.0.1:${process.env.PACKAGE_R_PORT || "8888"}`;
const screenshotBackendBaseURL = "http://127.0.0.1:8888";
const thumbnailPath = `openaerialmap-assets/${itemId}/thumbnail.png`;
const publicShareThumbnailPath = `/share/${publicShare}/${thumbnailPath}`;
const stacBrowserBaseURL = "https://browser.moregeo.it/external/";
const screenshotOptions = { animations: "disabled", caret: "hide" } as const;
const publicShareScreenshotOptions = {
  ...screenshotOptions,
  // Allow small Chromium text rasterization differences around object names.
  maxDiffPixels: 350,
} as const;
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

async function loginAsInitialUser(page: Page) {
  const authPage = new AuthPage(page);
  await authPage.goto();
  await authPage.loginAs("admin", "my-password");
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
    await loginAsInitialUser(page);

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
    await loginAsInitialUser(page);

    await page.goto(`/files/${sharedPrefix}/`);

    await expect(page.getByLabel("openaerialmap-assets")).toBeVisible();
    await expect(page.getByLabel("catalog.parquet")).toBeVisible();
    await normalizeRelativeTimes(page);
    await expect(page.locator("#listing").first()).toHaveScreenshot(
      "authenticated-public-listing.png",
      screenshotOptions
    );
  });

  test("saves user settings and lists shares without management actions", async ({
    page,
  }) => {
    await loginAsInitialUser(page);
    await page.goto("/files/");

    const profileSettingsButton = page.getByRole("button", {
      name: "Profile Settings",
      exact: true,
    });
    const shareManagementButton = page.getByRole("button", {
      name: "Share Management",
      exact: true,
    });
    await expect(profileSettingsButton).toBeVisible();
    await expect(shareManagementButton).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Share", exact: true })
    ).toHaveCount(0);

    await profileSettingsButton.click();
    await expect(page).toHaveURL(/\/settings$/);

    const singleClick = page.getByRole("checkbox", {
      name: "Use single clicks to open files and directories",
    });
    const initialSingleClick = await singleClick.isChecked();
    await singleClick.setChecked(!initialSingleClick);
    await page.getByRole("button", { name: "Save" }).click();
    await expect(
      page.getByText("Settings updated!", { exact: true })
    ).toBeVisible();

    await page.reload();
    await expect(singleClick).toBeChecked({ checked: !initialSingleClick });

    await singleClick.setChecked(initialSingleClick);
    await page.getByRole("button", { name: "Save" }).click();

    await shareManagementButton.click();
    await expect(page).toHaveURL(/\/shares$/);
    const settings = page.locator("#settings-shares");
    const pathLink = settings.getByRole("link", {
      name: "/catalog-sample",
      exact: true,
    });
    await expect(pathLink).toHaveAttribute("href", /\/share\/my-share\/$/);
    await expect(settings).toContainText("my-bucket");
    await expect(settings).toContainText("/catalog-sample");
    await expect(settings).toContainText("/catalog-sample/catalog.parquet");
    await expect(settings).toContainText("No");
    for (const heading of [
      "Source",
      "Path",
      "Share Duration",
      "Description",
      "Catalog",
      "Asset mapping",
      "Password protected",
    ]) {
      await expect(
        settings.getByRole("columnheader", { name: heading })
      ).toBeVisible();
    }
    await expect(settings.getByRole("button", { name: "New" })).toHaveCount(0);
    await expect(settings.getByRole("button", { name: "Delete" })).toHaveCount(
      0
    );
    await expect(settings.getByRole("button", { name: "Edit" })).toHaveCount(0);
    await expect(
      settings.getByRole("button", { name: "Copy to clipboard" })
    ).toHaveCount(0);
  });

  test("renders authenticated image preview", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsInitialUser(page);

    await page.goto(`/files/${sharedPrefix}/${thumbnailPath}`);

    const preview = page.locator("#previewer .preview");
    await expectImageLoaded(preview.locator("img.image-ex-img"));
    await expect(
      page.getByRole("button", { name: "Profile Settings", exact: true })
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Share Management", exact: true })
    ).toHaveCount(0);
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
      publicShareScreenshotOptions
    );
  });

  test("opens a public file through a presigned redirect", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareThumbnail(page);
    await expect(infoBox.getByText("MD5:")).toBeVisible();
    await expect(
      infoBox.locator("a.button", { hasText: "Download" })
    ).toHaveCount(0);
    await expect(
      infoBox.locator("a.button", { hasText: "Open file" })
    ).toHaveCount(0);

    const openLink = infoBox.getByRole("link", { name: "Open in browser" });
    const openHref = await openLink.getAttribute("href");
    if (openHref === null) {
      throw new Error("Open in browser link has no href");
    }
    const openURL = new URL(openHref);
    expect(openURL.pathname).toBe(
      `/api/public/share/${publicShare}/${thumbnailPath}`
    );
    expect(openURL.searchParams.get("presign")).toBe("true");
    expect(openURL.searchParams.get("follow")).toBe("true");

    const presignedLink = infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await expect(presignedLink).toHaveText("Show");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      `${sharedPrefix}/${thumbnailPath}`,
      `https://object-storage.example/${sharedPrefix}/${thumbnailPath}`
    );
    await normalizeRelativeTimes(page);
    await expect(infoBox).toHaveScreenshot(
      "my-share-presign.png",
      publicShareScreenshotOptions
    );
  });

  test("creates a presigned URL for a password-protected share", async ({
    page,
  }) => {
    await page.goto(`/share/protected-share/${itemId}/thumbnail.png`);
    await page.getByPlaceholder("Password").fill("my-share-password");
    await page.getByRole("button", { name: "Submit" }).click();

    const infoBox = page.locator(".share__box__info");
    await expect(infoBox).toContainText("thumbnail.png");

    const presignedLink = infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      `${sharedPrefix}/${thumbnailPath}`,
      `https://object-storage.example/${sharedPrefix}/${thumbnailPath}`
    );
  });

  test("initializes authentication after opening a public share", async ({
    page,
  }) => {
    await page.goto(`/share/${publicShare}/`);
    await expect(page.getByText("catalog.parquet")).toBeVisible();

    const response = await page.request.post("/api/login", {
      data: { username: "admin", password: "my-password", recaptcha: "" },
    });
    expect(response.ok()).toBe(true);
    const token = await response.text();
    await page.evaluate((jwt) => localStorage.setItem("jwt", jwt), token);

    await page.getByRole("img", { name: "Home" }).click();
    await expect(page).toHaveURL(/\/files\/$/);
    await expect(page).toHaveTitle(/.*Files - packageR$/);
  });

  test("shows public share STAC Browser URL", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareThumbnail(page);
    await infoBox
      .locator("p", { hasText: "STAC Browser URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(stacBrowserBaseURL);
    await expect(infoBox).toContainText(
      `/api/public/catalog/${publicShare}/${thumbnailPath}`
    );
    await infoBox
      .locator("p", { hasText: "STAC Browser URL:" })
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
      publicShareScreenshotOptions
    );
  });

  test("shows authenticated file info with rclone presigned URL", async ({
    page,
  }) => {
    await prepareStableScreenshot(page);
    await loginAsInitialUser(page);

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
