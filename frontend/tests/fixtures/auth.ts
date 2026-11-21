import {
  type Page,
  type Locator,
  test as base,
  expect,
} from "@playwright/test";

export class AuthPage {
  public readonly wrongCredentials: Locator;

  constructor(public readonly page: Page) {
    this.wrongCredentials = this.page.locator("div.wrong");
  }

  async goto() {
    await this.page.goto("/login");
  }

  async loginAs(username = "admin", password = "my-password") {
    await this.page.getByPlaceholder("Username").fill(username);
    await this.page.getByPlaceholder("Password").fill(password);
    await this.page.getByRole("button", { name: "Login" }).click();
  }

  async logout() {
    await this.page.getByRole("button", { name: "Logout" }).click();
  }
}

const test = base.extend<{ authPage: AuthPage }>({
  authPage: async ({ page }, use) => {
    await use(new AuthPage(page));
  },
});

export { test, expect };
