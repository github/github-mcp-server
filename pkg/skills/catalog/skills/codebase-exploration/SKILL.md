---
name: codebase-exploration
description: "USE FOR: understanding an unfamiliar repository and how it got that way — its layout, where a behaviour is implemented, and when and why a particular function or line last changed. NOT FOR: editing files, or assessing whether a project is healthy overall."
---

# Understanding a repository you have not read

Two questions arrive dressed as one: where does this happen, and why is it like this.
Answer the first from the layout and the code, the second from the commits — and never
guess at either from a file name.

## Sequence

1. **Take the shape before the contents.** `get_repository_tree` with `recursive: true`
   returns the whole layout in one call. On a large repository, set `path_filter` to a
   prefix and take the top level first; the directory names alone usually identify the
   entry points, the tests and the generated code.
2. **Read the documents the project wrote about itself.** `get_file_contents` on the
   README and any contributing guide. They state intent, which no amount of code
   reading recovers.
3. **Establish where the current work is.** `list_branches` shows what exists beside
   the default branch, and `list_tags` shows whether the project versions itself. If it
   does, the newest tag is a better reading point than the default branch, because it
   is a state someone chose to publish.
4. **Find the behaviour, do not browse for it.** `search_code` with `repo:owner/name`
   and a distinctive symbol goes straight there. Qualify with `path:` and
   `language:` to cut noise; quote a phrase for an exact match. Keep the `query` under
   256 characters, and use `fields` to return only `path` and `text_matches` when you
   are locating rather than reading.
5. **Read around every match.** A search result is a line; `get_file_contents` gives it
   the context that decides whether it is the definition, a caller, or a test.
6. **Ask why only once you know where.** `list_commits` with `path` set to the file
   gives that file's history in change order. This is where the layout question ends
   and the history question begins — `references/history-investigation.md` covers the
   rest.
7. **Report a route, not a conclusion.** Name the files, the symbols and the commits
   you actually read, so the next person can start where you finished.

## Decision points

- **Structure first, always.** Reading files chosen by name before seeing the tree is
  how exploration turns into a tour of the wrong directory.
- **Search scope decides everything.** `search_code` spans all of GitHub unless
  `repo:` or `org:` narrows it. An unscoped query returns other people's code, which
  looks plausible and answers nothing.
- **Prefer the smallest sufficient read.** A directory listing, then a search, then one
  file. Pulling whole directories of contents to find one symbol is slow and buries the
  answer.
- **Generated code is not evidence.** Vendored trees, lock files and generated sources
  will match a search and tell you nothing about intent. Exclude them by `path:` once
  you have seen them appear.
- **The default branch may not be the truth.** If the newest tag is well behind the
  default branch, the two describe different software; say which one you read.

## Fields that matter

- `recursive` and `path_filter` — the difference between one call and a hundred, and
  between a readable tree and an unreadable one.
- `ref` versus `sha` on `get_file_contents` — `ref` reads a branch or tag as it is now,
  `sha` reads one exact commit. Only the second is stable while you work.
- `query` qualifiers on `search_code` — `repo:`, `org:`, `path:`, `filename:`,
  `extension:`, `language:`, and `NOT` for exclusions. Ordering belongs in `sort` and
  `order`, never inside the query text.
- `path` on `list_commits` — restricts history to one file, which is the single most
  useful narrowing available.
- `detail` on `get_commit` — `stats` names the changed files, `full_patch` includes the
  diff and can be enormous. Ask for `none` when you only want the message and author.
- `fields` — on both the tree read and the searches, returning fewer fields is what
  keeps a wide exploration affordable.

## Confirm before going wider

Everything here is read-only, so the risk is not damage but scope: an exploration that
quietly becomes an audit, or that reads far more than was asked for.

- **Agree the boundary when a question spans repositories.** Searching an entire
  organisation is a different task from reading one repository, and costs
  correspondingly more. Ask before widening.
- **Ask before reading at volume.** If answering properly means pulling a large number
  of files or a very large diff, say what it would take and get agreement rather than
  starting.
- **Say when a question has become a change.** "Why is it like this" sometimes ends in
  "so fix it". Nothing here writes; stop at the finding and confirm the next step
  separately.

## Empty and failure cases

- **The tree is truncated** — very large repositories exceed what one call returns.
  Narrow with `path_filter` rather than treating the partial tree as complete, and say
  it was partial.
- **A search returns nothing** — the symbol may be constructed rather than written
  literally, spelled differently, or in a language the index does not cover. Try a
  distinctive fragment, then fall back to reading the directory the tree suggests.
- **A search returns hundreds of matches** — the term is too common. Add `path:` or
  `language:`, or quote a longer phrase. Do not page through them.
- **A path is not found** — it may have been moved or deleted rather than never having
  existed. `search_commits` for the old name distinguishes those, and the distinction
  usually is the answer.
- **The repository is empty or has no default branch** — report that rather than an
  empty layout, which reads as "nothing relevant here".
- **A file is too large to return** — say so and work from the tree, the search matches
  and the commit history instead of pretending it was read.
