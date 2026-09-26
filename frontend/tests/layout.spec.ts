import { expect, test } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

for (const viewport of [
  { width: 320, height: 568 },
  { width: 568, height: 320 },
]) {
  test(`login controls stay clear of the footer at ${viewport.width}px`, async ({
    page,
  }) => {
    await page.setViewportSize(viewport);
    const auth = new AuthPage(page);
    await auth.goto();
    await page.evaluate(() => document.fonts.ready);
    const button = page.getByRole("button", { name: "Login", exact: true });
    const buttonBox = await button.boundingBox();
    const footerBox = await page.locator("#about").boundingBox();
    expect(buttonBox).not.toBeNull();
    expect(footerBox).not.toBeNull();
    expect(buttonBox!.y + buttonBox!.height).toBeLessThan(footerBox!.y);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth)
    ).toBeLessThanOrEqual(viewport.width);
    await auth.loginAs();
    await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
  });
}

test("login has a visible keyboard focus state", async ({ page }) => {
  const auth = new AuthPage(page);
  await auth.goto();
  await page.getByPlaceholder("Username").fill("admin");
  await page.getByPlaceholder("Password").fill("admin");
  await page.keyboard.press("Tab");
  const button = page.getByRole("button", { name: "Login", exact: true });
  await expect(button).toBeFocused();
  await expect(button).toHaveCSS("outline-style", "solid");
  await expect(button).toHaveCSS("outline-width", "2px");
  await page.keyboard.press("Enter");
  await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
});

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

for (const width of [768, 1440]) {
  test(`long names do not overlap file sizes at ${width}px`, async ({
    page,
  }) => {
    await page.setViewportSize({ width, height: 900 });
    const auth = new AuthPage(page);
    await auth.goto();
    await auth.loginAs();
    await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
    const name = "xyz-" + "abcdefghij".repeat(14) + ".txt";
    await page.route(
      (url) => url.pathname === "/api/resources/",
      async (route) => {
        const response = await route.fetch();
        const json = await response.json();
        const item = json.items.find(
          (entry: { name: string }) => entry.name === "sample.txt"
        );
        item.name = name;
        item.path = "/" + name;
        await route.fulfill({ response, json });
      }
    );
    await page.goto("/files/");
    await expect(page.locator("#listing")).toHaveClass(/\blist\b/);
    const row = page.getByLabel(name, { exact: true });
    await expect(row).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    const nameBox = await row.locator(".name").boundingBox();
    const sizeBox = await row.locator(".size").boundingBox();
    const headingBox = await page
      .locator("#listing .item.header .size")
      .boundingBox();
    expect(nameBox).not.toBeNull();
    expect(sizeBox).not.toBeNull();
    expect(headingBox).not.toBeNull();
    expect(nameBox!.x + nameBox!.width).toBeLessThanOrEqual(sizeBox!.x + 1);
    expect(Math.abs(headingBox!.x - sizeBox!.x)).toBeLessThan(2);
  });
}

for (const width of [320, 390]) {
  test(`nested breadcrumbs fit the screen at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 844 });
    const auth = new AuthPage(page);
    await auth.goto();
    await auth.loginAs();
    await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
    const folder = "67793f0b9478720001790586";
    await page.goto(`/files/catalog-sample/openaerialmap-assets/${folder}/`);
    await expect(
      page.getByLabel("thumbnail.png", { exact: true })
    ).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth)
    ).toBeLessThanOrEqual(width);
    await expect(page.locator(".breadcrumbs").getByTitle(folder)).toBeVisible();
    await page
      .locator(".breadcrumbs")
      .getByTitle("openaerialmap-assets")
      .click();
    await expect(page.getByLabel(folder, { exact: true })).toBeVisible();
    await page.goto(
      `/share/my-share/openaerialmap-assets/${folder}/thumbnail.png`
    );
    await expect(page.locator(".share__box__info")).toContainText(
      "thumbnail.png"
    );
    await page.evaluate(() => document.fonts.ready);
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth)
    ).toBeLessThanOrEqual(width);
  });
}

test("Inter loads without replacing the icon or editor fonts", async ({
  page,
}) => {
  const auth = new AuthPage(page);
  await auth.goto();
  await auth.loginAs();
  await expect(page.getByLabel("sample.txt", { exact: true })).toBeVisible();
  expect(
    await page.evaluate(
      async () => (await document.fonts.load("16px Inter")).length
    )
  ).toBeGreaterThan(0);
  await expect(page.locator("body")).toHaveCSS("font-family", /Inter/);
  await expect(page.locator("body")).toHaveCSS("font-size", "15px");
  await expect(page.locator("header .material-icons").first()).toHaveCSS(
    "font-size",
    "24px"
  );
  await expect(
    page.getByLabel("sample.txt", { exact: true }).locator("i")
  ).toHaveCSS("font-size", "32px");
  await expect(page.locator(".material-icons").first()).toHaveCSS(
    "font-family",
    /Material Icons/
  );
  await page.goto("/files/sample.txt");
  await expect(page.locator(".ace_editor")).toBeVisible();
  await expect(page.locator(".ace_editor")).not.toHaveCSS(
    "font-family",
    /Inter/
  );
});
