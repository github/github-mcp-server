---
description: Add a new MCP tool to pkg/github/ following the canonical pattern (constructor + register + test + snapshot + docs)
argument-hint: <tool_name_snake_case> <github domain e.g. issues|repositories|actions>
allowed-tools: Read, Edit, Write, Bash(go test:*), Bash(go build:*), Bash(go vet:*), Bash(script/test:*), Bash(script/lint:*), Bash(script/generate-docs:*), Bash(UPDATE_TOOLSNAPS=*), Bash(git status:*), Bash(git diff:*), Bash(git add:*), Glob, Grep, Agent
---

Delegate to the `tool-adder` subagent. Arguments: `$ARGUMENTS`.

Brief the subagent with:
1. The proposed tool name (snake_case).
2. The GitHub domain (which file under `pkg/github/`).
3. Whether it's a read or write operation (affects `ReadOnlyHint`).
4. The relevant go-github or githubv4 client method to wrap.

After the subagent reports back, run `script/test` and `script/lint`
yourself to verify, then summarize for the user. Do NOT commit or push.
