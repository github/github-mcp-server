/**
 * Remount-persistence regression for github/github-mcp-server#2965.
 *
 * Simulates the MCP App lifecycle without VS Code:
 * 1. Host delivers a deferred tool-result (awaiting_user_submission + _meta.viewUUID)
 * 2. User submits → app saves SuccessView payload to localStorage under that UUID
 * 3. Host remounts the iframe and re-delivers the *same* deferral (not the submit result)
 * 4. App must restore SuccessView from localStorage — not fall back to the create form
 *
 * Run: node --experimental-strip-types --test src/lib/viewState.test.ts
 * (from ui/, after npm ci)
 */
import { test } from "node:test";
import assert from "node:assert/strict";
import { completedToolResult } from "./toolResult.ts";
import {
  getViewUUID,
  loadViewState,
  saveViewState,
  type CompletedViewState,
} from "./viewState.ts";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";

type PRResult = {
  number: number;
  title: string;
  html_url: string;
};

function installMemoryLocalStorage() {
  const store = new Map<string, string>();
  const localStorage = {
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    setItem(key: string, value: string) {
      store.set(key, value);
    },
    removeItem(key: string) {
      store.delete(key);
    },
    clear() {
      store.clear();
    },
  };
  Object.defineProperty(globalThis, "localStorage", {
    value: localStorage,
    configurable: true,
  });
  return store;
}

function deferredToolResult(viewUUID: string): CallToolResult {
  return {
    isError: true,
    content: [{ type: "text", text: "An interactive form has been shown to the user." }],
    structuredContent: {
      status: "awaiting_user_submission",
      reason: "An interactive form is being shown to the user. The operation has not been performed.",
    },
    _meta: { viewUUID },
  };
}

/**
 * Mirrors pr-write / pr-edit / issue-write shownX = successX ?? completedToolResult(toolResult)
 * plus the remount restore effect keyed by viewUUID.
 */
function resolveShownResult<T>(
  toolResult: CallToolResult | null,
  successFromReactState: T | null,
): { shown: T | null; source: "react" | "completed-result" | "localStorage" | "none" } {
  if (successFromReactState) {
    return { shown: successFromReactState, source: "react" };
  }
  const fromResult = completedToolResult<T>(toolResult);
  if (fromResult) {
    return { shown: fromResult, source: "completed-result" };
  }
  const viewUUID = getViewUUID(toolResult);
  if (viewUUID) {
    const saved = loadViewState<T>(viewUUID);
    if (saved) {
      return { shown: saved.result, source: "localStorage" };
    }
  }
  return { shown: null, source: "none" };
}

test("getViewUUID reads _meta.viewUUID from host-delivered deferral", () => {
  const viewUUID = "11111111-2222-4333-8444-555555555555";
  assert.equal(getViewUUID(deferredToolResult(viewUUID)), viewUUID);
  assert.equal(getViewUUID(null), undefined);
  assert.equal(getViewUUID({ content: [], _meta: {} }), undefined);
  assert.equal(getViewUUID({ content: [], _meta: { viewUUID: 123 } as never }), undefined);
});

test("completedToolResult treats deferral as incomplete (form stays open)", () => {
  const deferral = deferredToolResult("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee");
  assert.equal(completedToolResult(deferral), null);
});

test("#2965 remount: without persistence, SuccessView is lost (pre-fix behavior)", () => {
  installMemoryLocalStorage();
  const viewUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const deferral = deferredToolResult(viewUUID);
  const createdPR: PRResult = {
    number: 42,
    title: "Fix stuff",
    html_url: "https://github.com/o/r/pull/42",
  };

  // In-session: React success state shows SuccessView.
  let successPR: PRResult | null = createdPR;
  assert.equal(resolveShownResult(deferral, successPR).source, "react");

  // Remount: React state wiped; host re-sends deferral; no localStorage save.
  successPR = null;
  const afterRemount = resolveShownResult(deferral, successPR);
  assert.equal(afterRemount.source, "none");
  assert.equal(afterRemount.shown, null, "BUG #2965: form would show again");
});

test("#2965 remount: with viewUUID localStorage, SuccessView is restored (fix)", () => {
  installMemoryLocalStorage();
  const viewUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const deferral = deferredToolResult(viewUUID);
  const createdPR: PRResult = {
    number: 42,
    title: "Fix stuff",
    html_url: "https://github.com/o/r/pull/42",
  };

  // Submit success → persist under deferral viewUUID (what the apps do now).
  saveViewState(viewUUID, {
    status: "completed",
    result: createdPR,
    submittedTitle: "Fix stuff",
  } satisfies Omit<CompletedViewState<PRResult>, "savedAt">);

  // Remount: React state gone; host re-delivers same deferral.
  const afterRemount = resolveShownResult(deferral, null);
  assert.equal(afterRemount.source, "localStorage");
  assert.deepEqual(afterRemount.shown, createdPR);
});

test("saveViewState stores namespaced key with savedAt timestamp", () => {
  const store = installMemoryLocalStorage();
  const viewUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  saveViewState(viewUUID, {
    status: "completed",
    result: { number: 1, title: "t", html_url: "https://example.com/1" },
  });

  const keys = [...store.keys()];
  assert.equal(keys.length, 1);
  assert.ok(keys[0].startsWith("github-mcp-server:mcp-app-view:"), `unexpected key: ${keys[0]}`);

  const raw = store.get(keys[0])!;
  const parsed = JSON.parse(raw);
  assert.equal(typeof parsed.savedAt, "number");
  assert.ok(Date.now() - parsed.savedAt < 60_000);
});

test("loadViewState evicts entries older than TTL", () => {
  const store = installMemoryLocalStorage();
  const viewUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const key = `github-mcp-server:mcp-app-view:${viewUUID}`;
  const stale = {
    status: "completed",
    result: { number: 9, title: "old", html_url: "https://example.com/9" },
    savedAt: Date.now() - 8 * 24 * 60 * 60 * 1000, // 8 days
  };
  store.set(key, JSON.stringify(stale));

  assert.equal(loadViewState(viewUUID), null);
  assert.equal(store.get(key), undefined, "expired entry should be evicted");
});

test("#2965 new invocation with a different viewUUID still shows the form", () => {
  installMemoryLocalStorage();
  const firstUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const secondUUID = "ffffffff-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  saveViewState(firstUUID, {
    status: "completed",
    result: { number: 1, title: "old", html_url: "https://example.com/1" },
  });

  const newDeferral = deferredToolResult(secondUUID);
  const shown = resolveShownResult(newDeferral, null);
  assert.equal(shown.source, "none");
  assert.equal(shown.shown, null);
});

test("loadViewState rejects corrupt or incomplete payloads", () => {
  const store = installMemoryLocalStorage();
  const viewUUID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee";
  const key = `github-mcp-server:mcp-app-view:${viewUUID}`;
  store.set(key, "{not-json");
  assert.equal(loadViewState(viewUUID), null);

  store.set(key, JSON.stringify({ status: "pending", result: { number: 1 }, savedAt: Date.now() }));
  assert.equal(loadViewState(viewUUID), null);

  store.set(key, JSON.stringify({ status: "completed", savedAt: Date.now() }));
  assert.equal(loadViewState(viewUUID), null);
});
