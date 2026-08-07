import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";

/**
 * Completed form submission persisted so MCP Apps can remount the SuccessView
 * after the host tears down and recreates the iframe (e.g. switching chat
 * sessions). Keyed by CallToolResult._meta.viewUUID from the deferral result.
 *
 * See MCP Apps "Persisting view state" and github/github-mcp-server#2965.
 */
export interface CompletedViewState<T, TExtras = Record<string, never>> {
  status: "completed";
  result: T;
  /** Title shown on SuccessView when the PR/issue payload omits it. */
  submittedTitle?: string;
  extras?: TExtras;
  /** Epoch ms when this entry was written (for TTL). */
  savedAt: number;
}

/** Namespace to avoid colliding with other storage on the same origin. */
const KEY_PREFIX = "github-mcp-server:mcp-app-view:";
/** Completed views older than this are treated as gone and evicted. */
const MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000; // 7 days

function storageKey(viewUUID: string): string {
  return KEY_PREFIX + viewUUID;
}

/** Reads the stable view persistence key from a tool result's `_meta`. */
export function getViewUUID(result: CallToolResult | null | undefined): string | undefined {
  const raw = result?._meta?.viewUUID;
  return typeof raw === "string" && raw.length > 0 ? raw : undefined;
}

export function loadViewState<T, TExtras = Record<string, never>>(
  viewUUID: string,
): CompletedViewState<T, TExtras> | null {
  const key = storageKey(viewUUID);
  try {
    const saved = localStorage.getItem(key);
    if (!saved) return null;
    const parsed = JSON.parse(saved) as CompletedViewState<T, TExtras>;
    if (parsed?.status !== "completed" || parsed.result == null) return null;
    if (typeof parsed.savedAt === "number" && Date.now() - parsed.savedAt > MAX_AGE_MS) {
      localStorage.removeItem(key);
      return null;
    }
    return parsed;
  } catch (err) {
    console.error("Failed to load MCP App view state:", err);
    return null;
  }
}

export function saveViewState<T, TExtras = Record<string, never>>(
  viewUUID: string,
  state: Omit<CompletedViewState<T, TExtras>, "savedAt">,
): void {
  try {
    localStorage.setItem(
      storageKey(viewUUID),
      JSON.stringify({ ...state, savedAt: Date.now() }),
    );
  } catch (err) {
    console.error("Failed to save MCP App view state:", err);
  }
}
