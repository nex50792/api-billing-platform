import { expect, test } from "@playwright/test";

test.describe("top page", () => {
  test("renders the login form when unauthenticated", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "ログイン" })).toBeVisible();
    await expect(page.getByLabel("メールアドレス")).toBeVisible();
    await expect(page.getByLabel("パスワード")).toBeVisible();
  });

  test("exposes links to register and provider login", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("button", { name: "新規登録はこちら" })).toBeVisible();
    await expect(page.getByRole("button", { name: "プロバイダー向け" })).toBeVisible();
  });
});
