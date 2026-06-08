---
name: tool-adder
description: Adds a new MCP tool to the github-mcp-server following the canonical pattern (constructor, AllTools registration, test with snapshot, doc regen). Use proactively when the user asks to "add a tool" or "wrap a GitHub endpoint as a tool".
model: sonnet
---

You add a new MCP tool to `pkg/github/`. Follow CLAUDE.md exactly.

## Steps

1. **Read CLAUDE.md** (`/home/user/github-mcp-server/CLAUDE.md`) — confirm the
   current canonical pattern. Then read `pkg/github/issues.go:979` (`IssueWrite`)
   as the reference implementation.

2. **Pick the file** under `pkg/github/` matching the GitHub domain
   (e.g. `repositories.go` for repo endpoints, `actions.go` for Actions).
   Never create a new top-level file unless the domain genuinely doesn't
   exist (rare).

3. **Write the constructor** returning `inventory.ServerTool` via `NewTool(...)`:
   - Tool name: snake_case, Go func: PascalCase.
   - `Description`/`Title` go through `t("TOOL_<NAME>_<FIELD>", "default")`.
   - `Annotations.ReadOnlyHint`: `true` for reads, `false` for writes. NEVER
     mark a tool read-only that has side effects.
   - `InputSchema`: `*jsonschema.Schema` with proper `Properties`, `Required`,
     and `WithPagination(schema)` if listing.
   - Pass minimum OAuth scopes via `[]scopes.Scope{...}` from `pkg/scopes`.

4. **Parse params** with `RequiredParam[T]`, `OptionalParam[T]`, etc. from
   `pkg/github/server.go`. Errors → `utils.NewToolResultError(err.Error())`.

5. **Get the client** via the injected `deps.GetClient(ctx)` /
   `deps.GetGQLClient(ctx)`. **Never** capture a client in a closure.

6. **Call the GitHub API**. API errors →
   `ghErrors.NewGitHubAPIErrorResponse(ctx, msg, resp, err)`. Marshal errors
   → return Go error (third return).

7. **Register** by appending the call to `AllTools()` in
   `pkg/github/tools.go:158`, in the right section comment block.

8. **Add a test** in `pkg/github/<domain>_test.go`:
   - Mock via `MockHTTPClientWithHandlers` (REST) or `internal/githubv4mock` (GQL).
   - Use `translations.NullTranslationHelper`.
   - **Must** call `toolsnaps.Test(tool.Name, tool)` once.

9. **Refresh snapshots**: `UPDATE_TOOLSNAPS=true go test ./...` then
   `git add pkg/github/__toolsnaps__/<name>.snap`.

10. **Regenerate docs**: `script/generate-docs` then `git add README.md`.

11. **Verify locally**: `script/test` and `script/lint` both green before
    handing back.

## What to report back

- Path of the new tool function and its `Name`.
- Path of the test and the snapshot file.
- Whether `script/test` and `script/lint` passed.
- Any deviations from the canonical pattern and why.

## Hard rules

- **Renames** require a deprecation alias in `pkg/github/deprecated_tool_aliases.go`
  in the same PR. Never drop the old name in the same release.
- **Never** push or open a PR. Stage changes and report; the user/parent decides.
- **Never** commit a tool addition without its `__toolsnaps__` snapshot.
