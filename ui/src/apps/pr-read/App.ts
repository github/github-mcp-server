import {
  App,
  applyDocumentTheme,
  applyHostStyleVariables,
  applyHostFonts,
} from "@modelcontextprotocol/ext-apps";
import type { CallToolResult } from "@modelcontextprotocol/ext-apps";
import { parseDiff } from "./diff-parser";
import { renderDiff, setViewMode, getViewMode } from "./diff-renderer";
import { renderPRDetails } from "./pr-details-renderer";
import "./styles.css";

type Tab = "details" | "diff";

let app: App | null = null;
let activeTab: Tab = "details";

// Stored params for making subsequent tool calls when switching tabs
let prOwner = "";
let prRepo = "";
let prPullNumber = 0;

// Cache fetched data to avoid re-fetching on tab switch
let cachedDetails: Record<string, unknown> | null = null;
let cachedDiff: string | null = null;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function handleHostContextChanged(ctx: any): void {
  if (ctx.theme) {
    applyDocumentTheme(ctx.theme);
  }
  if (ctx.styles?.variables) {
    applyHostStyleVariables(ctx.styles.variables);
  }
  if (ctx.styles?.css?.fonts) {
    applyHostFonts(ctx.styles.css.fonts);
  }

  // Apply safe area insets
  if (ctx.safeAreaInsets) {
    document.body.style.paddingTop = `${ctx.safeAreaInsets.top}px`;
    document.body.style.paddingBottom = `${ctx.safeAreaInsets.bottom}px`;
    document.body.style.paddingLeft = `${ctx.safeAreaInsets.left}px`;
    document.body.style.paddingRight = `${ctx.safeAreaInsets.right}px`;
  }

  // Update fullscreen button visibility and state
  const fullscreenBtn = document.getElementById("fullscreen-btn");
  if (fullscreenBtn) {
    if (ctx.availableDisplayModes) {
      const canFullscreen = ctx.availableDisplayModes.includes("fullscreen");
      fullscreenBtn.style.display = canFullscreen ? "flex" : "none";
    }
    if (ctx.displayMode) {
      const isFullscreen = ctx.displayMode === "fullscreen";
      fullscreenBtn.textContent = isFullscreen ? "✕" : "⛶";
      fullscreenBtn.title = isFullscreen ? "Exit fullscreen" : "Fullscreen";
      document.body.classList.toggle("fullscreen", isFullscreen);
    }
  }
}

async function toggleFullscreen(): Promise<void> {
  if (!app) return;
  const ctx = app.getHostContext();
  const currentMode = ctx?.displayMode || "inline";
  const newMode = currentMode === "fullscreen" ? "inline" : "fullscreen";
  if (ctx?.availableDisplayModes?.includes(newMode)) {
    await app.requestDisplayMode({ mode: newMode });
  }
}

function toggleViewMode(): void {
  const currentMode = getViewMode();
  const newMode = currentMode === "unified" ? "split" : "unified";
  setViewMode(newMode);
  updateViewModeButton();
}

function updateViewModeButton(): void {
  const btn = document.getElementById("view-mode-btn");
  if (btn) {
    const mode = getViewMode();
    btn.textContent = mode === "unified" ? "Split" : "Unified";
    btn.title = mode === "unified" ? "Switch to split view" : "Switch to unified view";
  }
}

function updateTitle(owner: string, repo: string, pullNumber: number): void {
  const title = document.getElementById("title");
  if (title) {
    const prUrl = `https://github.com/${owner}/${repo}/pull/${pullNumber}`;
    title.innerHTML = `<span class="pr-link">${escapeHtml(owner)}/${escapeHtml(repo)} #${pullNumber}</span>`;
    const link = title.querySelector(".pr-link");
    if (link) {
      link.addEventListener("click", () => {
        app?.openLink({ url: prUrl });
      });
    }
  }
}

function escapeHtml(text: string): string {
  const div = document.createElement("div");
  div.textContent = text;
  return div.innerHTML;
}

function switchTab(tab: Tab): void {
  if (tab === activeTab) return;
  activeTab = tab;

  // Update tab bar
  document.querySelectorAll(".tab").forEach((el) => {
    el.classList.toggle("active", (el as HTMLElement).dataset.tab === tab);
  });

  // Toggle content visibility
  const contentArea = document.getElementById("content-area");
  const diffContainer = document.getElementById("diff-container");
  const viewModeBtn = document.getElementById("view-mode-btn");

  if (contentArea) contentArea.style.display = tab === "details" ? "block" : "none";
  if (diffContainer) diffContainer.style.display = tab === "diff" ? "flex" : "none";
  if (viewModeBtn) viewModeBtn.style.display = tab === "diff" ? "flex" : "none";

  // Fetch data if not cached
  if (tab === "diff" && cachedDiff === null) {
    fetchDiff();
  } else if (tab === "details" && cachedDetails === null) {
    fetchDetails();
  }
}

function parseToolResultText(result: CallToolResult): string | null {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const content = result.content as any[];
  if (!content || content.length === 0) return null;
  const textBlock = content.find((c) => c.type === "text");
  return textBlock?.text ?? null;
}

async function fetchDiff(): Promise<void> {
  if (!app || !prOwner || !prRepo || !prPullNumber) return;

  const diffContainer = document.getElementById("diff-container");
  if (diffContainer) diffContainer.innerHTML = '<div class="loading">Loading diff...</div>';

  const result = await app.callTool("pull_request_read", {
    method: "get_diff",
    owner: prOwner,
    repo: prRepo,
    pullNumber: prPullNumber,
  });

  const text = parseToolResultText(result);
  if (text) {
    cachedDiff = text;
    const parsed = parseDiff(text);
    renderDiff(parsed);
  }
}

async function fetchDetails(): Promise<void> {
  if (!app || !prOwner || !prRepo || !prPullNumber) return;

  const contentArea = document.getElementById("content-area");
  if (contentArea) contentArea.innerHTML = '<div class="loading">Loading details...</div>';

  const result = await app.callTool("pull_request_read", {
    method: "get",
    owner: prOwner,
    repo: prRepo,
    pullNumber: prPullNumber,
  });

  const text = parseToolResultText(result);
  if (text) {
    try {
      cachedDetails = JSON.parse(text);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      renderPRDetails(cachedDetails as any);
    } catch {
      if (contentArea) contentArea.innerHTML = `<div class="empty-state">Failed to parse PR details</div>`;
    }
  }
}

function handleInitialResult(result: CallToolResult, method: string): void {
  const text = parseToolResultText(result);
  if (!text) return;

  if (method === "get_diff") {
    cachedDiff = text;
    const parsed = parseDiff(text);
    renderDiff(parsed);
    switchTab("diff");
  } else if (method === "get") {
    try {
      cachedDetails = JSON.parse(text);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      renderPRDetails(cachedDetails as any);
      switchTab("details");
    } catch {
      // fall through
    }
  }
}

function init(): void {
  app = new App({ name: "github-mcp-server-pr-read", version: "1.0.0" });

  // Handle tool input to extract params and determine initial tab
  app.ontoolinput = (input: Record<string, unknown>) => {
    const owner = input.owner as string;
    const repo = input.repo as string;
    const pullNumber = input.pullNumber as number;
    const method = (input.method as string) || "get";

    if (owner) prOwner = owner;
    if (repo) prRepo = repo;
    if (pullNumber) prPullNumber = pullNumber;

    if (prOwner && prRepo && prPullNumber) {
      updateTitle(prOwner, prRepo, prPullNumber);
    }

    // Set initial tab based on method
    if (method === "get_diff") {
      switchTab("diff");
    } else {
      switchTab("details");
    }
  };

  // Handle tool results
  app.ontoolresult = (result: CallToolResult) => {
    // Determine which method this result is for based on active tab / cached state
    // If we don't have either cached, this is the initial result
    if (cachedDetails === null && cachedDiff === null) {
      // Peek at the content to determine the type
      const text = parseToolResultText(result);
      if (text) {
        // If it looks like a unified diff, it's a diff result
        if (text.startsWith("diff --git") || text.includes("\n---\n")) {
          cachedDiff = text;
          const parsed = parseDiff(text);
          renderDiff(parsed);
          if (activeTab === "diff") {
            // Already on diff tab, just render
          }
        } else {
          // Try to parse as JSON (PR details)
          try {
            cachedDetails = JSON.parse(text);
            // eslint-disable-next-line @typescript-eslint/no-explicit-any
            renderPRDetails(cachedDetails as any);
          } catch {
            // Unknown format - show as text
            const contentArea = document.getElementById("content-area");
            if (contentArea) contentArea.textContent = text;
          }
        }
      }
    }
  };

  // Handle streaming partial input for progressive diff rendering
  app.ontoolinputpartial = (input: Record<string, unknown>) => {
    const diff = input.diff as string | undefined;
    if (diff && activeTab === "diff") {
      const parsed = parseDiff(diff);
      renderDiff(parsed);
    }
  };

  // Handle host context changes (theme, etc.)
  app.onhostcontextchanged = handleHostContextChanged;

  // Set up tab bar
  document.querySelectorAll(".tab").forEach((tab) => {
    tab.addEventListener("click", () => {
      const tabName = (tab as HTMLElement).dataset.tab as Tab;
      if (tabName) switchTab(tabName);
    });
  });

  // Set up view mode toggle button
  const viewModeBtn = document.getElementById("view-mode-btn");
  if (viewModeBtn) {
    viewModeBtn.addEventListener("click", toggleViewMode);
  }

  // Set up fullscreen button
  const fullscreenBtn = document.getElementById("fullscreen-btn");
  if (fullscreenBtn) {
    fullscreenBtn.addEventListener("click", toggleFullscreen);
  }

  // Connect to host
  app.connect().then(() => {
    const ctx = app?.getHostContext();
    if (ctx) {
      handleHostContextChanged(ctx);
    }
  });
}

// Initialize when DOM is ready
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init);
} else {
  init();
}
