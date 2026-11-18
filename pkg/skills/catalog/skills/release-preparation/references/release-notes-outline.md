# Release notes outline

A structure to fill in, not a template to paste. Drop any section that would be
empty; an empty heading reads worse than no heading.

## Ordering

Lead with what changes a reader's decision to upgrade:

1. **Breaking changes** — what broke, and the smallest edit that fixes it.
2. **Security fixes** — what was vulnerable, and whether anything beyond upgrading is
   needed.
3. **Highlights** — the two or three changes worth upgrading for.
4. **Other changes** — grouped by area.
5. **Fixes** — one line each.
6. **Internal** — collapsed, or omitted.

## Writing each entry

- Say what changed for the reader, not what the author did. *Tags with a leading `v`
  now resolve* beats *refactor tag parsing*.
- One line each. If an entry needs a paragraph, it belongs in Highlights.
- Reference the pull request number so a reader can reach the detail.
- Attribute first-time contributors by handle if the project does that elsewhere.
- Keep the project's existing voice and heading names; consistency across releases
  matters more than any particular structure.

## Facts to state exactly once

- The version being released.
- The exact commit it is built from.
- The previous version the change list is measured against.
- Any manual step an upgrade requires — configuration, data migration, or a minimum
  version of a dependency.

## Before handing the draft over

- Every breaking change has an upgrade note.
- Every security fix names the severity and whether upgrading is sufficient.
- No entry describes work that is still open.
- Nothing claims the release is published.
