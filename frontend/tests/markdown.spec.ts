import { test, expect } from "@playwright/test";
import { AuthPage } from "./fixtures/auth";

test("Markdown preview removes active HTML and keeps formatting", async ({
  page,
}) => {
  const auth = new AuthPage(page);
  await auth.goto();
  await auth.loginAs();
  await expect(page).toHaveURL(/\/files\/$/);

  await page.route(
    (url) => url.pathname === "/api/resources/xyz.md",
    async (route) => {
      await route.fulfill({
        json: {
          name: "xyz.md",
          path: "/xyz.md",
          type: "text",
          isDir: false,
          size: 1,
          modified: "2026-01-01T00:00:00Z",
          content: [
            "# xyz heading",
            "**xyz bold**",
            '<img src="/xyz-missing-image" onerror="document.body.dataset.xyz = \'executed\'">',
            "<a href=\"javascript:document.body.dataset.xyz='executed'\">xyz link</a>",
            "<iframe srcdoc=\"<script>parent.document.body.dataset.xyz='executed'</script>\"></iframe>",
          ].join("\n\n"),
        },
      });
    }
  );
  await page.goto("/files/xyz.md");
  await expect(page.locator("#editor .ace_content")).toBeVisible();
  await page.getByRole("button", { name: "Preview", exact: true }).click();
  const preview = page.locator("#preview-container");
  await expect(
    preview.getByRole("heading", { name: "xyz heading" })
  ).toBeVisible();
  await expect(preview.locator("strong")).toHaveText("xyz bold");
  await expect(
    preview.locator("[onerror], iframe, script, [href^='javascript:']")
  ).toHaveCount(0);
  await expect(page.locator("body")).not.toHaveAttribute(
    "data-xyz",
    "executed"
  );
});
