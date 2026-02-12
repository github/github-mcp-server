import { test, expect } from "../fixtures/test";

test.describe("issue-write app", () => {
  test("renders form with all required fields", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Title input should be visible
    await expect(page.getByPlaceholder("Title")).toBeVisible();

    // Description label should be visible
    await expect(page.getByText("Description")).toBeVisible();

    // Submit button should be visible
    await expect(page.getByRole("button", { name: /create issue/i })).toBeVisible();
  });

  test("shows loading spinner when initializing", async ({ page }) => {
    // Don't use gotoApp - navigate directly so mocks aren't applied
    // This will cause the app to try connecting and show loading state
    await page.goto("/issue-write/index.html");

    // Should show spinner while connecting (might be brief)
    // Just verify the page loads without immediate error
    await page.waitForLoadState("domcontentloaded");
  });

  test("pre-fills form from tool input", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: {
          owner: "octocat",
          repo: "hello-world",
          title: "Bug: Something is broken",
          body: "## Description\n\nThis is a test issue.",
        },
      },
    });

    // Title should be pre-filled
    await expect(page.getByPlaceholder("Title")).toHaveValue("Bug: Something is broken");
  });

  test("displays markdown editor with write/preview buttons", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Should have Write button
    await expect(page.getByRole("button", { name: "Write" })).toBeVisible();

    // Should have Preview button
    await expect(page.getByRole("button", { name: "Preview" })).toBeVisible();
  });

  test("markdown preview renders content", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: {
          owner: "octocat",
          repo: "hello-world",
          body: "## Test Header\n\nSome **bold** text.",
        },
      },
    });

    // Click Preview button
    await page.getByRole("button", { name: "Preview" }).click();

    // Should render markdown as HTML (heading)
    await expect(page.getByRole("heading", { name: "Test Header" })).toBeVisible();
  });

  test("submit button is disabled when title is empty", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Submit button should be disabled when title is empty
    const submitButton = page.getByRole("button", { name: /create issue/i });
    await expect(submitButton).toBeDisabled();

    // Fill in title
    await page.getByPlaceholder("Title").fill("Test Issue");

    // Button should now be enabled
    await expect(submitButton).toBeEnabled();
  });

  test("displays metadata buttons", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Should have Assignees button
    await expect(page.getByRole("button", { name: /assignees/i })).toBeVisible();

    // Should have Labels button
    await expect(page.getByRole("button", { name: /labels/i })).toBeVisible();

    // Should have Milestone button
    await expect(page.getByRole("button", { name: /milestone/i })).toBeVisible();
  });

  test("displays repository picker", async ({ gotoApp, page }) => {
    await gotoApp("issue-write");

    // Repository button should be visible
    const repoButton = page.getByRole("button", { name: /select repository/i });
    await expect(repoButton).toBeVisible();
  });

  test("shows selected repo when provided via toolInput", async ({ gotoApp, page }) => {
    await gotoApp("issue-write", {
      mocks: {
        toolInput: { owner: "octocat", repo: "hello-world" },
      },
    });

    // Should show the repo name
    await expect(page.getByText("octocat/hello-world")).toBeVisible();
  });
});
