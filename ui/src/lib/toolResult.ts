import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";

/**
 * Returns the parsed JSON payload of a *completed* tool result, or `null` when
 * the result is absent, an error, or the deferral sentinel (structured
 * `status` of `"awaiting_user_submission"`, set by the server's
 * `NewToolResultAwaitingFormSubmission`).
 *
 * Form-backed write Views (create_pull_request, issue_write,
 * update_pull_request) use this to tell apart "the server deferred and is
 * waiting for my form submission" from "the server already executed". Without
 * it, a View renders its input form off in-app state alone and will show e.g. a
 * "Create pull request" form for a PR that was already created up-front (when
 * the agent passed `show_ui=false` or parameters the form can't represent).
 *
 * See github/copilot-mcp-core#1864 for the full show/defer state machine.
 */
export function completedToolResult<T = Record<string, unknown>>(
  result: CallToolResult | null,
): T | null {
  if (!result || result.isError) return null;

  const status = (result.structuredContent as { status?: string } | undefined)
    ?.status;
  if (status === "awaiting_user_submission") return null;

  const textContent = result.content?.find((c) => c.type === "text");
  if (!textContent || textContent.type !== "text" || !textContent.text) {
    return null;
  }

  try {
    return JSON.parse(textContent.text) as T;
  } catch {
    return null;
  }
}
