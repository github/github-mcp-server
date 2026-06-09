---
description: Refresh pkg/github/__toolsnaps__ JSON snapshots after a tool schema change
allowed-tools: Read, Bash(go test:*), Bash(UPDATE_TOOLSNAPS=*), Bash(git status:*), Bash(git diff:*), Bash(git add:*), Agent
---

Delegate to the `snapshot-keeper` subagent.

Brief it with:
- Which tool(s) you changed (file paths + identifiers).
- Whether the schema change was intentional (description, schema fields,
  read-only hint, name).

The subagent will:
1. Run `UPDATE_TOOLSNAPS=true go test ./...`.
2. Inspect each modified `__toolsnaps__/*.snap`.
3. Flag any unintentional drift before staging.

After it reports, run `script/test` to confirm tests pass against the
refreshed snapshots. Do NOT commit.
