---
name: release-preparation
description: "USE FOR: cutting a release of a repository — picking the version, assembling the changelog since the last tag, confirming the branch is releasable, and drafting release notes. NOT FOR: assessing overall repository health, triaging security alerts, or working an issue backlog."
---

# Preparing a release

Work out what is about to ship, whether it is safe to ship, what to call it, and what
to say about it — then hand a publishable draft to a person.

**This server cannot publish.** No tool here creates a tag or a release. The output of
this workflow is a decision and a draft, not a published release.

## Sequence

1. **Find the baseline.** `get_latest_release` gives the last published release with
   its `tag_name` and `published_at`. If there is none, `list_tags` shows whether the
   project tags without releasing. If neither exists, this is a first release and the
   baseline is the start of history.
2. **Confirm what you are releasing from.** `list_commits` for the release branch,
   bounded with `since` set to the baseline `published_at`, gives the range. Record
   the head `sha` — the draft is only meaningful against a specific commit.
3. **Assemble the change list from merged pull requests**, not from raw commits.
   `search_pull_requests` with a query of the form `is:pr is:merged base:<branch>
   merged:>=<baseline date>`, scoped to the repository, yields titles written for
   people. Fall back to `list_commits` only where a change arrived without a pull
   request.
4. **Check the branch is releasable.** `actions_list` with
   `method: list_workflow_runs`, filtered to the release branch, then `actions_get`
   with `method: get_workflow_run` for the run matching your head `sha`. You want a
   completed run whose `conclusion` is `success` on that exact commit.
5. **Check what is still in flight.** `list_pull_requests` with `state: open` and
   `base` set to the release branch shows work that will miss the cut. Anything a
   maintainer expects in the release must merge first or be explicitly dropped.
6. **Choose the version** using the decision points below.
7. **Draft the notes** following `references/release-notes-outline.md`.
8. **Hand off.** Present the version, the exact commit, the notes, and any caveats,
   and let a maintainer publish.

## Decision points

- **Cut or wait.** Red or missing CI on the head commit means do not cut. Say what
  failed and stop; a release built on an unverified commit is the expensive kind of
  mistake.
- **Version bump.** Any removal or incompatible change to a published interface is a
  major. New capability that keeps existing behaviour is a minor. Everything else is a
  patch. When the change list is ambiguous, present both candidates with the changes
  that would justify each, and ask.
- **Prerelease or full.** If the change list contains anything the project wants
  exercised before it becomes the default, propose a prerelease.
- **What to leave out.** Internal refactoring, test-only changes and dependency bumps
  with no visible effect belong in a collapsed section or nowhere. Security fixes
  always appear, however small.

## Fields that matter

- `tag_name` and `published_at` on the baseline release — the version anchor and the
  time window for everything else.
- `prerelease` and `draft` on prior releases — they reveal the project's conventions,
  including whether it ships candidates first.
- `sha` of the head commit — the notes describe one commit, and that commit belongs in
  the handoff.
- `conclusion` and `head_sha` on a workflow run — a successful run on a different
  commit says nothing about this one.
- `merged_at` on a pull request — the field that decides whether it falls inside the
  window, not `created_at` or `updated_at`.

## Confirm before anything is announced

Preparation is read-only, so nothing here needs approval. The handoff does: present
the version and notes for review before they are published anywhere, and never
describe a release as done on the strength of a draft.

## Empty and failure cases

- **No prior release or tag** — treat it as the first release, say so, and base the
  notes on the whole history.
- **No changes since the baseline** — report that there is nothing to release and
  stop.
- **Workflow information unavailable** — if the actions tools are not enabled or the
  call fails, report CI as *not verified* and let the maintainer decide. Never infer a
  green build from the absence of evidence.
- **Merged pull requests missing from the search** — a repository that merges without
  pull requests still has commits; fall back to `list_commits` and say the change list
  came from commits, which is noisier.
