import { test, expect } from "./fixtures/auth";

test("redirect to login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);

  await page.goto("/files/");
  await expect(page).toHaveURL(/\/login\?redirect=\/files\//);
});

test("login supports browser password managers", async ({ authPage, page }) => {
  await authPage.goto();

  await expect(page.locator('input[name="username"]')).toHaveAttribute(
    "autocomplete",
    "username"
  );
  await expect(page.locator('input[name="password"]')).toHaveAttribute(
    "autocomplete",
    "current-password"
  );
});

test("login and logout", async ({ authPage, page, context }) => {
  await authPage.goto();
  await expect(page).toHaveTitle(/Login - packageR$/);

  await authPage.loginAs("fake", "fake");
  await expect(authPage.wrongCredentials).toBeVisible();

  await authPage.loginAs();
  await expect(authPage.wrongCredentials).toBeHidden();
  await expect(page).toHaveURL(/\/files\/$/);
  await expect(page).toHaveTitle(/.*Files - packageR$/);

  let cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeDefined();

  await authPage.logout();
  await expect(page).toHaveURL(/\/login$/);
  await expect(page).toHaveTitle(/Login - packageR$/);

  cookies = await context.cookies();
  expect(cookies.find((c) => c.name == "auth")?.value).toBeUndefined();
});
