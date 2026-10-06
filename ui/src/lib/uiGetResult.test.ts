import assert from "node:assert/strict";
import { test } from "node:test";
import { parseUIGetResult } from "./uiGetResult.ts";

const fixtures = {
  labels: { labels: [{ id: "L", name: "bug", color: "red" }] },
  assignees: { assignees: [{ login: "octocat", avatar_url: "avatar.png" }] },
  milestones: { milestones: [{ number: 1, title: "v1", description: "Release" }] },
  issue_types: [{ id: 1, name: "Bug" }, null],
  branches: { branches: [{ name: "main", protected: true }] },
  issue_fields: { fields: [{ id: "F", name: "Priority", data_type: "text" }] },
  reviewers: {
    users: [{ login: "octocat", avatar_url: "avatar.png" }],
    teams: [{ slug: "docs", org: "owner" }],
  },
};

for (const method of Object.keys(fixtures) as Array<keyof typeof fixtures>) {
  test(`${method}: modern and legacy payloads reach the same picker data`, () => {
    const legacy = parseUIGetResult(method, JSON.stringify(fixtures[method]));
    const modern = parseUIGetResult(method, JSON.stringify({ method, [method]: fixtures[method] }));
    assert.deepEqual(modern, legacy);
    assert.deepEqual(modern, fixtures[method]);
  });
}

test("picker arrays and reviewer avatars survive modern decoding", () => {
  assert.equal(parseUIGetResult("labels", JSON.stringify({ method: "labels", labels: fixtures.labels })).labels.map(l => l.name)[0], "bug");
  assert.equal(parseUIGetResult("branches", JSON.stringify({ method: "branches", branches: fixtures.branches })).branches.map(b => b.name)[0], "main");
  const reviewers = parseUIGetResult("reviewers", JSON.stringify({ method: "reviewers", reviewers: fixtures.reviewers }));
  assert.equal(reviewers.users.map(u => u.avatar_url)[0], "avatar.png");
  assert.equal(reviewers.teams.map(t => `${t.org}/${t.slug}`)[0], "owner/docs");
  assert.equal(parseUIGetResult("issue_fields", JSON.stringify({ method: "issue_fields", issue_fields: fixtures.issue_fields })).fields.map(f => f.name)[0], "Priority");
});

test("issue types accept empty, null, wrapped, and nullable-item payloads", () => {
  for (const value of [null, [], { method: "issue_types", issue_types: null }, { method: "issue_types", issue_types: [] }]) {
    assert.deepEqual(parseUIGetResult("issue_types", JSON.stringify(value)), []);
  }
  for (const value of [{ issue_types: fixtures.issue_types }, { types: fixtures.issue_types }]) {
    assert.deepEqual(parseUIGetResult("issue_types", JSON.stringify(value)), fixtures.issue_types);
  }
  const types = parseUIGetResult("issue_types", JSON.stringify({ method: "issue_types", issue_types: fixtures.issue_types }));
  assert.equal(types.filter(t => t !== null).map(t => typeof t === "string" ? t : t.name)[0], "Bug");
});

test("legacy bare branches and empty picker arrays remain supported", () => {
  assert.deepEqual(parseUIGetResult("branches", JSON.stringify(fixtures.branches.branches)), fixtures.branches);
  assert.deepEqual(parseUIGetResult("labels", '{"labels":[]}').labels, []);
});

test("malformed JSON and mismatched modern envelopes fail explicitly", () => {
  assert.throws(() => parseUIGetResult("labels", "not JSON"));
  assert.throws(() => parseUIGetResult("labels", '{"method":"branches","branches":{}}'), /Invalid ui_get/);
  assert.throws(() => parseUIGetResult("labels", '{"method":"labels"}'), /Invalid ui_get/);
});
