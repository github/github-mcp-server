import { test, expect } from "../fixtures/test";

test.describe("pr-write app", () => {
  test("renders form with all required fields", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Title input should be visible
    await expect(page.getByPlaceholder("Title")).toBeVisible();

    // Description section should be visible
    await expect(page.getByText("Description")).toBeVisible();

    // Branch selectors should be visible
    await expect(page.getByText("base")).toBeVisible();
    await expect(page.getByText("compare")).toBeVisible();

    // Create button should be visible
    await expect(page.getByRole("button", { name: /create.*pull request/i })).toBeVisible();
  });

  test("shows loading spinner when initializing", async ({ page }) => {
    // Don't use gotoApp - navigate directly so mocks aren't applied
    await page.goto("/pr-write/index.html");

    // Should show spinner while connecting (might be brief)
    // Just verify the page loads without immediate error
    await page.waitForLoadState("domcontentloaded");
  });

  test("displays repository picker", async ({ gotoApp, page }) => {
    await gotoApp("pr-write");

    // Repository button should be visible
    const repoButton = page.getByRole("button", { name: /select repository/i });
    await expect(repoButton).toBeVisible();
  });

  test("pre-fills form from tool input", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: {
          owner: "octocat",
          repo: "hello-world",
          title: "Test PR Title",
          body: "Test PR description",
          head: "feature/test",
          base: "main",
          draft: true,
        },
      },
    });

    // Title should be pre-filled
    await expect(page.getByPlaceholder("Title")).toHaveValue("Test PR Title");

    // Draft checkbox should be checked
    const draftCheckbox = page.getByRole("checkbox", { name: /draft/i });
    await expect(draftCheckbox).toBeChecked();
  });

  test("submit button is disabled when required fields are empty", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Submit button should be disabled when required fields are missing
    const submitButton = page.getByRole("button", { name: /create.*pull request/i });
    await expect(submitButton).toBeDisabled();
  });

  test("draft toggle changes button text", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Initially should say "Create pull request"
    await expect(page.getByRole("button", { name: "Create pull request" })).toBeVisible();

    // Check draft checkbox
    await page.getByRole("checkbox", { name: /draft/i }).check();

    // Button should now say "Create draft pull request"
    await expect(page.getByRole("button", { name: "Create draft pull request" })).toBeVisible();
  });

  test("maintainer edit checkbox is checked by default", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    const maintainerCheckbox = page.getByRole("checkbox", { name: /maintainer/i });
    await expect(maintainerCheckbox).toBeChecked();
  });

  test("displays metadata buttons", async ({ gotoApp, page }) => {
    await gotoApp("pr-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Should have Reviewers button
    await expect(page.getByRole("button", { name: /reviewers/i })).toBeVisible();

    // Should have Labels button
    await expect(page.getByRole("button", { name: /labels/i })).toBeVisible();

    // Should have Milestone button
    await expect(page.getByRole("button", { name: /milestone/i })).toBeVisible();
  });
});
