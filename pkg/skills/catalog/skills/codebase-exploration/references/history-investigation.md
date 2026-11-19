# Tracing a change back to its reason

Use this once the question has moved from "where is this" to "why is it this way", or
"when did this stop working". The route is always the same: narrow to a commit, read
what it changed, then find what it was for.

## Narrowing to a commit

Three different starting points, three different calls:

- **You know the file.** `list_commits` with `path` set to it returns only the commits
  that touched it, newest first. Add `since` and `until` to bracket a period, or
  `author` when the question is about a person's work.
- **You know roughly what was said.** `search_commits` matches commit messages, scoped
  with `repo:owner/name`. It searches the default branch only, so a change that landed
  on a release branch and was never merged forward is invisible to it — `list_commits`
  with `sha` set to that branch is the way to see it.
- **You know a fragment of the hash.** `search_commits` accepts `hash:` with a short
  hash, which resolves an abbreviated identifier from a log or a report.

For a version rather than a commit, `list_tags` enumerates what the project published
and `get_tag` resolves one to the commit behind it. That commit is the anchor for "what
changed between these two releases".

## Reading the commit

`get_commit` with the `sha`, and `detail` chosen for the question:

- `none` — the message, the author and the date. Enough to place a change in time.
- `stats` — which files changed and by how much. Enough to tell a rename from a
  rewrite, and a wide mechanical change from a narrow deliberate one.
- `full_patch` — the diff itself. Correct when you need to read the change, and
  expensive on anything large. Ask for it once you have chosen the commit, never while
  still searching for it.

Read the message before the diff. A good message says why; the diff only says what.

## Bisecting by hand

When a behaviour worked once and does not now, and nothing names the change:

1. `list_commits` with `path` over the range where it must have happened, and read the
   messages in order. Most of the time the change announces itself.
2. If it does not, pick a commit near the middle and read the file as of that commit —
   `get_file_contents` with `sha` set to it, not `ref`, so the read is pinned.
3. Move earlier or later depending on what you find, and repeat. Each step halves the
   range, so even a long history resolves in a handful of reads.
4. Stop when two adjacent commits differ in behaviour. That pair is the answer.

Say how far you got if you stop early. "Introduced somewhere between these two commits"
is a real finding; a guess dressed as a conclusion is not.

## What each source is good for

| Question | Where the answer is |
| --- | --- |
| What changed here, in order? | Commits filtered to the file's `path` |
| Who has been working on this? | The same list, filtered by `author` |
| Which change mentioned this ticket or symbol? | Commit message search, scoped to the repository |
| What shipped between two versions? | The tags, resolved to commits, then the range between them |
| What did this file look like then? | File contents pinned to a `sha` |
| Was it a rename or a rewrite? | The commit's per-file stats |

## Cautions

- **A commit search only sees the default branch.** Absence there is not absence.
- **Merge commits hide their contents.** A merge whose stats look enormous is usually
  carrying someone else's work; follow the branch it merged rather than reading it.
- **Mass reformatting destroys the trail.** When every line of a file changed in one
  commit, the change you want is almost always just before it.
- **Dates are not order.** Commit dates can precede the commit that came before them.
  Trust the sequence a listing returns over the timestamps it carries.
- **Stop at the finding.** Tracing why something changed is read-only work. If it ends
  in something needing a fix, say so and confirm that as a separate step.
