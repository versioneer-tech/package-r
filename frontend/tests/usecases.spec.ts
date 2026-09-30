import { expect, type Locator, type Page, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

const dataset = "vienna-s2l2a-26";
const itemId = "S2B_T33UXP_20260218T100524_L2A";
const publicShare = dataset;
const publicSampleShare = "sample-share";
const backendBaseURL = `http://127.0.0.1:${process.env.PACKAGE_R_PORT || "8888"}`;
const screenshotBackendBaseURL = "http://127.0.0.1:8888";
const overviewPath = `${dataset}/${itemId}/overview.tif`;
const publicShareOverviewPath = `/share/${publicShare}/${itemId}/overview.tif`;
const stacBrowserBaseURL = "http://localhost:8080/external/";
const screenshotOptions = { animations: "disabled", caret: "hide" } as const;
const publicShareScreenshotOptions = {
  ...screenshotOptions,
  // Allow small Chromium text rasterization differences in documentation images.
  maxDiffPixels: 750,
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

async function expectTiffRendered(canvas: Locator) {
  await expect(canvas).toBeVisible({ timeout: 30_000 });
  await expect
    .poll(
      () =>
        canvas.evaluate(
          (element) =>
            element instanceof HTMLCanvasElement &&
            element.width > 0 &&
            element.height > 0 &&
            element.dataset.rendered === "true"
        ),
      { timeout: 30_000 }
    )
    .toBe(true);
}

async function openPublicShareOverview(page: Page) {
  await page.goto(publicShareOverviewPath);
  await expectTiffRendered(page.locator("#previewer .tiff-preview canvas"));
  await page.getByRole("button", { name: "Info", exact: true }).click();
  const infoBox = page.locator(".card.floating").filter({
    has: page.getByRole("heading", { name: "File information" }),
  });
  await expect(infoBox).toContainText("overview.tif");
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

    for (const name of [dataset, "sample-files"]) {
      await expect(page.getByLabel(name, { exact: true })).toBeVisible();
    }

    await page.getByLabel("sample-files").dblclick();
    for (const name of [
      "sample.html",
      "sample.jpg",
      "sample.js",
      "sample.json",
      "sample.md",
      "sample.pdf",
      "sample.png",
      "sample.py",
      "sample.tif",
      "sample.txt",
      "sample.yaml",
    ]) {
      await expect(page.getByLabel(name, { exact: true })).toBeVisible();
    }
  });

  test("streams the COG preview with byte-range requests", async ({ page }) => {
    const objectSize = 69_335_343;
    const objectGets: Array<{
      range: string | undefined;
      status: number;
      contentLength: number;
    }> = [];

    page.on("response", (response) => {
      const request = response.request();
      const range = request.headers()["range"];
      if (
        request.method() === "GET" &&
        new URL(response.url()).pathname.endsWith("/sample-files/sample.tif") &&
        range
      ) {
        objectGets.push({
          range,
          status: response.status(),
          contentLength: Number(response.headers()["content-length"]),
        });
      }
    });

    await loginAsInitialUser(page);
    await page.goto("/files/sample-files/sample.tif");

    const canvas = page.locator(".tiff-preview canvas");
    await expectTiffRendered(canvas);
    await expect
      .poll(
        () =>
          canvas.evaluate(
            (element) =>
              element instanceof HTMLCanvasElement && element.width === 768
          ),
        { timeout: 30_000 }
      )
      .toBe(true);

    const transfer = page.getByTestId("tiff-transfer");
    await expect(transfer).toBeVisible();
    await expect(transfer).toContainText("/");
    await expect(
      page.getByRole("button", { name: "Info", exact: true })
    ).toBeVisible();

    expect(objectGets.length).toBeGreaterThan(1);
    expect(objectGets.every(({ range }) => range?.startsWith("bytes="))).toBe(
      true
    );
    expect(objectGets.every(({ status }) => [200, 206].includes(status))).toBe(
      true
    );
    expect(
      objectGets.every(({ range, contentLength }) => {
        const match = /^bytes=(\d+)-(\d+)$/.exec(range ?? "");
        if (!match) return false;

        const start = Number(match[1]);
        const end = Math.min(Number(match[2]), objectSize - 1);
        return contentLength === end - start + 1;
      })
    ).toBe(true);
  });

  test("renders authenticated shared data listing", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsInitialUser(page);

    await page.goto("/files/");

    await expect(page.getByLabel(dataset)).toBeVisible();
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
      name: `/${dataset}`,
      exact: true,
    });
    await expect(pathLink).toHaveAttribute(
      "href",
      new RegExp(`/share/${publicShare}/$`)
    );
    await expect(settings).toContainText(publicShare);
    await expect(settings).toContainText(
      `/${dataset}/${dataset}.parquet`
    );
    await expect(settings).toContainText("No");
    for (const heading of [
      "Name",
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

  test("hides share management when no shares are configured", async ({
    page,
  }) => {
    await page.route("**/api/shares", (route) => route.fulfill({ json: [] }));
    await loginAsInitialUser(page);

    await expect(
      page.getByRole("button", { name: "Share Management", exact: true })
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Profile Settings", exact: true })
    ).toBeVisible();
  });

  test("renders authenticated image preview", async ({ page }) => {
    await prepareStableScreenshot(page);
    await loginAsInitialUser(page);

    await page.goto(`/files/${overviewPath}`);

    const preview = page.locator("#previewer .preview");
    await expectTiffRendered(preview.locator(".tiff-preview canvas"));
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

    await expect(page.getByLabel(itemId)).toBeVisible();
    for (const action of [
      "New folder",
      "New file",
      "Upload",
      "Download",
      "Rename",
      "Move",
      "Copy",
      "Delete",
      "Shell",
      "Profile Settings",
      "Share Management",
      "Logout",
    ]) {
      await expect(
        page.getByRole("button", { name: action, exact: true })
      ).toHaveCount(0);
    }
    await normalizeRelativeTimes(page);
    await expect(page.locator("#listing").first()).toHaveScreenshot(
      "vienna-s2l2a-26-directory.png",
      publicShareScreenshotOptions
    );

    await page.getByLabel(itemId).dblclick();
    await expect(page).toHaveURL(new RegExp(`/share/${publicShare}/${itemId}/$`));
    await expect(page.getByLabel("rgb.tif")).toBeVisible();
  });

  test("previews a public file and creates a presigned URL", async ({
    page,
  }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareOverview(page);
    const md5Link = infoBox.locator("p", { hasText: "MD5:" }).locator("a");
    await md5Link.click();
    await expect(md5Link).toHaveText(/^[a-f0-9]{32}$/);
    await expect(
      infoBox.locator("a.button", { hasText: "Download" })
    ).toHaveCount(0);
    await expect(
      infoBox.locator("a.button", { hasText: "Open file" })
    ).toHaveCount(0);

    const presignedLink = infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await expect(presignedLink).toHaveText("Show");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      overviewPath,
      `https://object-storage.example/${overviewPath}`
    );
    await normalizeRelativeTimes(page);
    await expect(infoBox).toHaveScreenshot(
      "vienna-s2l2a-26-presign.png",
      publicShareScreenshotOptions
    );
  });

  test("creates a presigned URL for a password-protected share", async ({
    page,
  }) => {
    await page.goto("/share/protected-share/overview.tif");
    await page.getByPlaceholder("Password").fill("wrong-password");
    await page.getByRole("button", { name: "Submit" }).click();
    await expect(page.getByText("Wrong credentials")).toBeVisible();

    await page.getByPlaceholder("Password").fill("my-share-password");
    await page.getByRole("button", { name: "Submit" }).click();

    await expectTiffRendered(page.locator("#previewer .tiff-preview canvas"));
    await page.getByRole("button", { name: "Info", exact: true }).click();
    const infoBox = page.locator(".card.floating").filter({
      has: page.getByRole("heading", { name: "File information" }),
    });
    await expect(infoBox).toContainText("overview.tif");

    const presignedLink = infoBox
      .locator("p", { hasText: "Presigned URL:" })
      .getByRole("link");
    await presignedLink.click();
    await expectAndNormalizePresignedURL(
      presignedLink,
      overviewPath,
      `https://object-storage.example/${overviewPath}`
    );
  });

  test("initializes authentication after opening a public share", async ({
    page,
  }) => {
    await page.goto(`/share/${publicShare}/`);
    await expect(page.getByLabel(itemId)).toBeVisible();

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

  test("keeps a public share read-only for an authenticated user", async ({
    page,
  }) => {
    await loginAsInitialUser(page);
    await page.goto(`/share/${publicShare}/`);

    await expect(page.getByLabel(itemId)).toBeVisible();
    for (const action of [
      "New folder",
      "New file",
      "Upload",
      "Download",
      "Rename",
      "Move",
      "Copy",
      "Delete",
      "Shell",
      "Profile Settings",
      "Share Management",
      "Logout",
    ]) {
      await expect(
        page.getByRole("button", { name: action, exact: true })
      ).toHaveCount(0);
    }
  });

  test("previews a share whose root is one image", async ({ page }) => {
    await page.goto("/share/image-share/");

    await expectImageLoaded(page.locator("#previewer img.image-ex-img"));
    await expect(
      page.getByRole("button", { name: "Close", exact: true })
    ).toHaveCount(0);
  });

  test("opens a single text-file share in read-only mode", async ({ page }) => {
    let resourceWriteCount = 0;
    await page.route("**/api/resources/**", async (route) => {
      if (route.request().method() !== "GET") resourceWriteCount++;
      await route.continue();
    });

    await page.goto("/share/text-share/");

    const viewer = page.getByTestId("text-viewer");
    await expect(viewer).toContainText(
      "This file is a packageR development fixture."
    );
    await expect(page.locator(".ace_editor")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Save", exact: true })
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Close", exact: true })
    ).toHaveCount(0);

    await page.keyboard.press("Control+s");
    expect(resourceWriteCount).toBe(0);
  });

  test("opens a single JSON-file share in the text viewer", async ({
    page,
  }) => {
    await page.goto("/share/json-share/");

    const viewer = page.getByTestId("text-viewer");
    await expect(viewer).toContainText('"name": "packageR test fixture"');
    await expect(viewer).toContainText('"purpose":');
    await expect(viewer).toHaveText(
      JSON.stringify(
        {
          name: "packageR test fixture",
          purpose: "Exercise JSON browsing and previews",
          items: ["image", "json", "pdf", "text"],
        },
        null,
        2
      )
    );
    await expect(page.locator(".ace_editor")).toHaveCount(0);
  });

  test("renders Markdown in an anonymous share", async ({ page }) => {
    await page.goto(`/share/${publicSampleShare}/sample.md`);

    const viewer = page.getByTestId("markdown-viewer");
    await expect(
      viewer.getByRole("heading", { name: "PackageR Fixture", level: 1 })
    ).toBeVisible();
    await expect(
      viewer.locator("strong", { hasText: "bold text" })
    ).toBeVisible();
    await expect(viewer.getByRole("listitem")).toHaveCount(2);
    await expect(viewer.getByRole("table")).toContainText("Feature");
    await expect(page.locator(".ace_editor")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Save", exact: true })
    ).toHaveCount(0);
  });

  test("opens YAML as structured read-only text", async ({ page }) => {
    await page.goto(`/share/${publicSampleShare}/sample.yaml`);

    const viewer = page.getByTestId("text-viewer");
    await expect(viewer).toContainText("name: packageR YAML fixture");
    await expect(viewer).toContainText("preserveFormatting: true");
    await expect(page.locator(".ace_editor")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Save", exact: true })
    ).toHaveCount(0);
  });

  test("opens XML as structured read-only text", async ({ page }) => {
    await page.goto(`/share/${publicShare}/${itemId}/product_metadata.xml`);

    const viewer = page.getByTestId("text-viewer");
    await expect(viewer).toContainText("<n1:Level-2A_User_Product");
    await expect(viewer).toContainText(
      "<PROCESSING_LEVEL>Level-2A</PROCESSING_LEVEL>"
    );
    await expect(page.locator(".ace_editor")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Save", exact: true })
    ).toHaveCount(0);
  });

  test("shows public share STAC Browser URL", async ({ page }) => {
    await prepareStableScreenshot(page);

    const infoBox = await openPublicShareOverview(page);
    const urlLabels = await infoBox.locator("p strong").allTextContents();
    expect(urlLabels.indexOf("STAC Browser URL: ")).toBeLessThan(
      urlLabels.indexOf("Presigned URL: ")
    );
    await infoBox
      .locator("p", { hasText: "STAC Browser URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(stacBrowserBaseURL);
    await expect(infoBox).toContainText(
      `/api/public/catalog/${publicShare}/${itemId}/`
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
      "vienna-s2l2a-26-preview-url.png",
      publicShareScreenshotOptions
    );
  });

  test("shows STAC Browser URLs for the share and Item directories", async ({
    page,
  }) => {
    await page.goto(`/share/${publicShare}/`);
    await page.getByRole("button", { name: "Info", exact: true }).click();
    let infoBox = page.locator(".card.floating").filter({
      has: page.getByRole("heading", { name: "File information" }),
    });
    await infoBox
      .locator("p", { hasText: "STAC Browser URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(
      `/api/public/catalog/${publicShare}`
    );
    await page.getByRole("button", { name: "OK", exact: true }).click();

    await page.goto(`/share/${publicShare}/${itemId}/`);
    await page.getByRole("button", { name: "Info", exact: true }).click();
    infoBox = page.locator(".card.floating").filter({
      has: page.getByRole("heading", { name: "File information" }),
    });
    await infoBox
      .locator("p", { hasText: "STAC Browser URL:" })
      .getByRole("link", { name: "Show" })
      .click();
    await expect(infoBox).toContainText(
      `/api/public/catalog/${publicShare}/${itemId}/`
    );
  });

  test("shows authenticated file info with rclone presigned URL", async ({
    page,
  }) => {
    await prepareStableScreenshot(page);
    await loginAsInitialUser(page);

    await page.goto(`/files/${dataset}/${itemId}/`);
    await expect(page.getByLabel("overview.tif")).toBeVisible();
    await page.getByLabel("overview.tif").click();
    await expect(page.getByLabel("overview.tif")).toHaveAttribute(
      "aria-selected",
      "true"
    );

    await page.getByRole("button", { name: "Info", exact: true }).click();
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
      overviewPath,
      `${screenshotBackendBaseURL}/api/raw/${overviewPath}`
    );
    await normalizeRelativeTimes(page);
    await expect(modal).toHaveScreenshot(
      "authenticated-file-info-presign.png",
      screenshotOptions
    );
  });
});
