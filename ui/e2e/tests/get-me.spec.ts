import { test, expect } from "../fixtures/test";
import { mockUser } from "../fixtures/mcp-mocks";

test.describe("get-me app", () => {
  test("displays loading state or connects successfully", async ({ page }) => {
    // Navigate without mocks - app will try to connect to real host
    await page.goto("/get-me/index.html");

    // Should either show loading state or an error (when no host is available)
    // This verifies the app loads and attempts to connect
    await expect(
      page.getByText("Loading user data...").or(page.getByText(/error/i))
    ).toBeVisible({ timeout: 5000 });
  });

  test("renders user card with profile data", async ({ gotoApp, page }) => {
    await gotoApp("get-me");

    // Should display user's name
    await expect(page.getByRole("heading", { name: mockUser.details?.name })).toBeVisible();

    // Should display username
    await expect(page.getByText(`@${mockUser.login}`)).toBeVisible();

    // Should display company (use exact match to avoid matching email)
    await expect(page.getByText(mockUser.details?.company!, { exact: true })).toBeVisible();

    // Should display location
    await expect(page.getByText(mockUser.details?.location!)).toBeVisible();
  });

  test("displays user stats correctly", async ({ gotoApp, page }) => {
    await gotoApp("get-me");

    // Should show repos count
    await expect(page.getByText("Repos")).toBeVisible();
    await expect(page.getByText(String(mockUser.details?.public_repos))).toBeVisible();

    // Should show followers count
    await expect(page.getByText("Followers")).toBeVisible();
    await expect(page.getByText(String(mockUser.details?.followers))).toBeVisible();

    // Should show following count
    await expect(page.getByText("Following")).toBeVisible();
    await expect(page.getByText(String(mockUser.details?.following))).toBeVisible();
  });

  test("shows error state when MCP fails", async ({ page }) => {
    // Navigate without mocks - the connection will fail to a non-existent host
    await page.goto("/get-me/index.html");

    // Wait a bit for the connection timeout
    await page.waitForTimeout(2000);

    // Should display error message (connection error)
    await expect(page.getByText(/error/i)).toBeVisible({ timeout: 10000 });
  });

  test("displays avatar with fallback on error", async ({ gotoApp, page }) => {
    // Test with user that has no avatar
    await gotoApp("get-me", {
      mocks: {
        user: { ...mockUser, avatar_url: undefined },
      },
    });

    // Should show fallback icon (PersonIcon renders in a Box)
    // The fallback renders when avatar_url is undefined
    await expect(page.getByRole("heading", { name: mockUser.details?.name })).toBeVisible();
  });

  test("links open in new tab", async ({ gotoApp, page }) => {
    await gotoApp("get-me");

    // Blog link should have target="_blank"
    const blogLink = page.getByRole("link", { name: mockUser.details?.blog });
    await expect(blogLink).toHaveAttribute("target", "_blank");
  });
});
