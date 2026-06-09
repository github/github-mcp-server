---
description: Run script/lint and fix any failures via the lint-fixer subagent
allowed-tools: Read, Edit, Bash(script/lint:*), Bash(script/test:*), Bash(gofmt:*), Bash(go vet:*), Bash(git status:*), Bash(git diff:*), Bash(git add:*), Agent, Grep, Glob
---

1. Run `script/lint` and capture output.
2. If clean → report "lint clean" and stop.
3. If failures → delegate to the `lint-fixer` subagent with the captured
   output. Brief it with:
   - The exact lint output.
   - Which files are in-scope for the current change vs. pre-existing.
4. After the subagent reports back, re-run `script/lint` and `script/test`
   to verify both are green.
5. Stage the fixes (`git add` on the modified files) but do NOT commit.
6. Summarize: files touched, rules suppressed (if any), test+lint status.
