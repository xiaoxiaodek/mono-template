import { expect, test } from "@playwright/test";

test("shows the administrator sign-in form", async ({ page }) => {
  await page.goto(process.env.OPERATION_WEB_BASE_URL ?? "http://127.0.0.1:5173");

  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});
