import assert from "node:assert/strict";
import { test } from "node:test";
import { completedToolResult, parseNumberFromURL } from "./toolResult.ts";

test("completedToolResult: handles null, error, deferral, and completed results", () => {
  assert.equal(completedToolResult(null), null);
  assert.equal(completedToolResult({ isError: true, content: [{ type: "text", text: "{}" }] }), null);
  assert.equal(
    completedToolResult({
      isError: false,
      structuredContent: { status: "awaiting_user_submission" },
      content: [{ type: "text", text: "{}" }],
    }),
    null,
  );
  assert.equal(completedToolResult({ isError: false, content: [] }), null);
  assert.equal(completedToolResult({ isError: false, content: [{ type: "text", text: "invalid-json" }] }), null);
  assert.deepEqual(
    completedToolResult({
      isError: false,
      content: [{ type: "text", text: '{"id":"123","url":"https://github.com/owner/repo/pull/42"}' }],
    }),
    { id: "123", url: "https://github.com/owner/repo/pull/42" },
  );
});

test("parseNumberFromURL: extracts PR numbers from various GitHub URLs", () => {
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/123"), 123);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/123/"), 123);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/123/files"), 123);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/123#issuecomment-456"), 123);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/123?diff=unified"), 123);
});

test("parseNumberFromURL: extracts issue numbers from various GitHub URLs", () => {
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/issues/789"), 789);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/issues/789/"), 789);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/issues/789#issuecomment-999"), 789);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/issues/789?foo=bar"), 789);
});

test("parseNumberFromURL: returns undefined for non-PR/issue URLs or invalid inputs", () => {
  assert.equal(parseNumberFromURL(undefined), undefined);
  assert.equal(parseNumberFromURL(""), undefined);
  assert.equal(parseNumberFromURL("#"), undefined);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo"), undefined);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/pull/notanumber"), undefined);
  assert.equal(parseNumberFromURL("https://github.com/owner/repo/issues/notanumber"), undefined);
});
