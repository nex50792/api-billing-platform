import { expect, test } from "@playwright/test";

test.describe("/provider/login", () => {
  test("shows the login form fields", async ({ page }) => {
    await page.goto("/provider/login");
    await expect(page.getByRole("heading", { name: "プロバイダーログイン" })).toBeVisible();
    const email = page.getByLabel("メールアドレス");
    const password = page.getByLabel("パスワード");
    await expect(email).toBeVisible();
    await expect(password).toBeVisible();
    await expect(email).toHaveAttribute("required", "");
    await expect(password).toHaveAttribute("required", "");
  });

  test("rejects invalid credentials with an error message", async ({ page }) => {
    // ページ GET ではなく API の POST だけを 401 に差し替え。
    await page.route("**/provider/login", route => {
      if (route.request().method() === "POST") {
        return route.fulfill({ status: 401, contentType: "application/json", body: '{"error":"invalid credentials"}' });
      }
      return route.continue();
    });
    await page.goto("/provider/login");
    await page.getByLabel("メールアドレス").fill("nobody@example.com");
    await page.getByLabel("パスワード").fill("wrong-password");
    await page.getByRole("button", { name: /ログイン/ }).click();
    await expect(page.getByRole("alert")).toContainText("ログインに失敗しました");
  });
});
