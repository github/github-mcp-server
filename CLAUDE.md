# CLAUDE.md

Guidance for AI assistants (Claude, Copilot, Cursor, etc.) working in this
repository. Keep this file in sync with reality — when conventions change,
update here too.

## What this project is

The **GitHub MCP Server** — a Go implementation of an [MCP](https://modelcontextprotocol.io)
server that exposes GitHub functionality (REST + GraphQL) to AI agents. Ships
as a single binary (`cmd/github-mcp-server`) plus a Docker image; runs either:

- **stdio** transport (local subprocess; the only mode this binary serves —
  see `stdioCmd` in `cmd/github-mcp-server/main.go:30`), or
- **remote HTTP** transport at `https://api.githubcopilot.com/mcp/` (hosted by
  GitHub; not built from this binary but uses the same toolset code).

Key SDKs/libraries (`go.mod`):

| Dependency | Role |
|---|---|
| `github.com/modelcontextprotocol/go-sdk` | MCP protocol |
| `github.com/google/go-github/v79` | GitHub REST client |
| `github.com/shurcooL/githubv4` | GitHub GraphQL client |
| `github.com/spf13/cobra` + `viper` | CLI flags + env var binding |
| `github.com/google/jsonschema-go` | Tool input schemas |
| `github.com/stretchr/testify` | Assertions + mocks |

Go version: **1.24+** (toolchain in `go.mod`).

## Repository layout

```
cmd/
  github-mcp-server/      # Main binary (stdio server, doc generators)
  mcpcurl/                # Dev CLI that talks to a stdio server (testing)
internal/
  ghmcp/                  # Server lifecycle, client construction, registration
  toolsnaps/              # Tool-schema snapshot testing helper
  githubv4mock/           # GraphQL mock transport
  profiler/               # Internal profiling helpers
pkg/
  github/                 # All MCP tools, resources, prompts (most edits land here)
  inventory/              # Registry: filtering by toolset/read-only/scope/feature
  translations/           # i18n helper (TranslationHelperFunc)
  errors/                 # GitHubAPIError, NewGitHubAPIErrorResponse
  scopes/                 # OAuth scope constants + helpers
  lockdown/               # Repo-access cache + lockdown-mode enforcement
  utils/                  # NewToolResultText/Error helpers
  raw/, sanitize/, buffer/, log/, octicons/, tooldiscovery/
e2e/                      # Build-and-run-the-Docker-image tests (build-tag `e2e`)
docs/                     # User + contributor docs (linked from README)
script/                   # test, lint, generate-docs, tag-release, etc.
.github/workflows/        # CI: go, lint, goreleaser, mcp-diff, code-scanning…
```

### `pkg/github/` highlights

- `tools.go` — `ToolsetMetadata*` constants + `AllTools(t)` registry
- `server.go` — `RequiredParam[T]`, `OptionalParam[T]`, `WithPagination`, etc.
- `dependencies.go` — `ToolDependencies` interface (`GetClient`, `GetGQLClient`,
  `GetRawClient`, …) + `ContextWithDeps`
- `inventory.go` — `NewInventory(t).SetTools(AllTools(t))...`
- Per-domain files: `issues.go`, `pullrequests.go`, `repositories.go`,
  `actions.go`, `discussions.go`, `projects.go`, `gists.go`, `labels.go`,
  `code_scanning.go`, `secret_scanning.go`, `dependabot.go`,
  `security_advisories.go`, `notifications.go`, `context_tools.go`,
  `git.go`, `dynamic_tools.go`
- `repository_resource.go`, `prompts.go` — MCP resources + prompts
- `deprecated_tool_aliases.go` — renamed-tool back-compat (read this when
  renaming any tool; see `docs/tool-renaming.md`)
- `__toolsnaps__/` — committed JSON snapshots of every tool schema

## Build, run, test

| Command | Purpose |
|---|---|
| `go build ./cmd/github-mcp-server` | Build the binary |
| `GITHUB_PERSONAL_ACCESS_TOKEN=… go run ./cmd/github-mcp-server stdio` | Run locally |
| `script/test` (= `go test -race ./...`) | Unit tests |
| `UPDATE_TOOLSNAPS=true go test ./...` | Refresh tool-schema snapshots |
| `script/lint` | gofmt + golangci-lint v2.5.0 (auto-downloaded into `bin/`) |
| `script/generate-docs` | Regenerate `README.md` toolset tables |
| `GITHUB_MCP_SERVER_E2E_TOKEN=… go test -v --tags e2e ./e2e` | End-to-end (builds Docker image, hits real GitHub API) |
| `script/tag-release vX.Y.Z` | Cut a release tag (CI publishes via goreleaser) |

E2E debugging: set `GITHUB_MCP_SERVER_E2E_DEBUG=true` to run in-process
(breakpoint-friendly) instead of spawning the container.

## CLI flags / env vars

Defined in `cmd/github-mcp-server/main.go:94`. All flags also bind via viper
with env prefix `GITHUB_` and `_` ↔ `-` normalization.

| Flag | Env | Notes |
|---|---|---|
| `--toolsets` | `GITHUB_TOOLSETS` | Comma list; default = toolsets marked `Default: true` |
| `--tools` | `GITHUB_TOOLS` | Specific tools (additive to toolsets) |
| `--features` | `GITHUB_FEATURES` | Feature-flag gated tools |
| `--dynamic-toolsets` | `GITHUB_DYNAMIC_TOOLSETS` | Adds `enable_toolset`/discovery tools |
| `--read-only` | `GITHUB_READ_ONLY` | Filters out tools with `ReadOnlyHint: false` |
| `--lockdown-mode` | `GITHUB_LOCKDOWN_MODE` | Enforces repo-access checks |
| `--insiders` | `GITHUB_INSIDERS` | Experimental tools |
| `--gh-host` | `GITHUB_HOST` | GHES / ghe.com hostname |
| `--log-file`, `--enable-command-logging` | … | Logging |
| `--content-window-size` | `GITHUB_CONTENT_WINDOW_SIZE` | Default `5000` |
| `--export-translations` | … | Dump translation keys to JSON |
| Required env: `GITHUB_PERSONAL_ACCESS_TOKEN` | | |

## Toolsets

Defined as `inventory.ToolsetMetadata` in `pkg/github/tools.go:19`. IDs:
`context`, `repos`, `git`, `issues`, `pull_requests`, `users`, `orgs`,
`actions`, `code_security`, `secret_protection`, `dependabot`,
`notifications`, `discussions`, `gists`, `security_advisories`, `projects`,
`stargazers`, `labels`, `dynamic` (+ remote-only: `copilot`, `copilot_spaces`,
`github_support_docs_search`).

Defaults: `context`, `repos`, `issues`, `pull_requests`, `users`.

## Adding a tool — the canonical pattern

Use `pkg/github/issues.go:979` (`IssueWrite`) or any existing tool as a model.

1. **Pick the file** that matches the GitHub domain (e.g. `repositories.go`).
2. **Write a constructor** returning `inventory.ServerTool`:

   ```go
   func MyNewTool(t translations.TranslationHelperFunc) inventory.ServerTool {
       return NewTool(
           ToolsetMetadataRepos,                       // toolset membership
           mcp.Tool{
               Name:        "my_new_tool",             // snake_case
               Description: t("TOOL_MY_NEW_TOOL_DESCRIPTION", "Does X."),
               Annotations: &mcp.ToolAnnotations{
                   Title:        t("TOOL_MY_NEW_TOOL_USER_TITLE", "Do X"),
                   ReadOnlyHint: true,                 // false for writes
               },
               InputSchema: &jsonschema.Schema{ /* … */ },
           },
           []scopes.Scope{scopes.Repo},                // minimum OAuth scopes
           func(ctx context.Context, deps ToolDependencies,
                req *mcp.CallToolRequest, args map[string]any,
           ) (*mcp.CallToolResult, any, error) {
               owner, err := RequiredParam[string](args, "owner")
               if err != nil { return utils.NewToolResultError(err.Error()), nil, nil }

               client, err := deps.GetClient(ctx)
               if err != nil {
                   return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
               }

               result, resp, err := client.Foo.Bar(ctx, owner /* … */)
               if err != nil {
                   return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to bar", resp, err), nil, nil
               }

               body, _ := json.Marshal(result)
               return utils.NewToolResultText(string(body)), nil, nil
           },
       )
   }
   ```

3. **Register it** by appending the call to the slice in `AllTools()`
   (`pkg/github/tools.go:158`), in the right section comment block.
4. **Add a test** in `pkg/github/<domain>_test.go` using the
   `MockHTTPClientWithHandlers` pattern (see `issues_test.go:780`) — must
   assert `toolsnaps.Test(tool.Name, tool)` for schema-snapshot stability.
5. **Refresh snapshots**: `UPDATE_TOOLSNAPS=true go test ./...` then commit
   the new file under `pkg/github/__toolsnaps__/`.
6. **Regenerate docs**: `script/generate-docs`.
7. **Renaming an existing tool?** Add an alias in
   `deprecated_tool_aliases.go` (see `docs/tool-renaming.md`) — do NOT remove
   the old name in the same release.

### Parameter helpers (`pkg/github/server.go`)

- `RequiredParam[T](args, name) (T, error)`
- `OptionalParam[T](args, name) (T, error)`
- `RequiredInt`, `OptionalIntParam`, `OptionalIntParamWithDefault`
- `OptionalBoolParamWithDefault`
- `OptionalStringArrayParam`
- `WithPagination(schema)` — adds `page` / `perPage` automatically

### Error returns

Two distinct paths — see `docs/error-handling.md`:

- **User-actionable** (API/auth/404): return `*mcp.CallToolResult` with
  `IsError: true` via `ghErrors.NewGitHubAPIErrorResponse(ctx, msg, resp, err)`
  or `utils.NewToolResultError(msg)`. The `nil` Go-error means "tool ran,
  reported failure to model".
- **Developer/system error** (marshal failure, programmer bug): return a Go
  `error` (third return). Propagates up the MCP framework.

### Client retrieval

Always via the injected `ToolDependencies` from the handler signature, **not**
captured closures:

```go
client, err   := deps.GetClient(ctx)      // REST (*go-github.Client)
gql,    err   := deps.GetGQLClient(ctx)   // GraphQL (*githubv4.Client)
raw,    err   := deps.GetRawClient(ctx)   // raw.googleusercontent etc.
```

### Read-only mode

A tool marks itself read-only via `Annotations.ReadOnlyHint: true`. When the
server runs with `--read-only`, the inventory filter drops any tool whose hint
is `false`. **Never** add side-effecting code paths under a `ReadOnlyHint:
true` tool.

### Translation keys

All user-visible strings go through `t("KEY", "default")`. Keys are
`SCREAMING_SNAKE_CASE`, conventionally `TOOL_<NAME>_<FIELD>`. The default is
the canonical English text; the key enables override via env var
`GITHUB_MCP_<KEY>` or a translations JSON file.

## Testing conventions

- Framework: `testify` (`assert`, `require`).
- REST mocking: `MockHTTPClientWithHandlers` (in `helper_test.go`) — register
  one `http.HandlerFunc` per endpoint with `expectRequestBody(...).andThen(
  mockResponse(...))`.
- GraphQL mocking: `internal/githubv4mock`.
- Every tool test must call `toolsnaps.Test(tool.Name, tool)` so schema drift
  is caught — snapshots live in `pkg/github/__toolsnaps__/<name>.snap`.
- Use `translations.NullTranslationHelper` in tests.

## CI workflows (`.github/workflows/`)

| Workflow | Trigger | What it does |
|---|---|---|
| `go.yml` | push/PR | `script/test` on Linux/macOS/Windows, `go mod tidy -diff` |
| `lint.yml` | push/PR | golangci-lint v2.5 |
| `mcp-diff.yml` | PR | Diffs MCP tool schemas against base branch (toolsets matrix) |
| `goreleaser.yml` | tag `v*` | Cross-platform release, provenance attestations |
| `docker-publish.yml` | release | Push image |
| `docs-check.yml` | PR | Ensures `script/generate-docs` is up to date |
| `license-check.yml`, `code-scanning.yml` | … | License + CodeQL |
| `ai-issue-assessment.yml`, `issue-labeler.yml`, `moderator.yml` | issues/PRs | Triage automation |

## Contribution flow (`CONTRIBUTING.md`)

1. Fork + branch off `main`.
2. `go test -v ./...` + `script/lint` pass locally.
3. If tool schema changed: `UPDATE_TOOLSNAPS=true go test ./...` and
   `script/generate-docs`.
4. PR targets `main`. Keep scope focused; write tests; follow `.golangci.yml`
   style.
5. Renames need a deprecation alias (`docs/tool-renaming.md`).

## Conventions cheat sheet

- **File naming**: one file per GitHub domain in `pkg/github/`, plus a
  matching `_test.go`.
- **Tool name**: snake_case; **Go func**: PascalCase; **translation key**:
  SCREAMING_SNAKE_CASE.
- **Don't** add backwards-compat shims for internal refactors; **do** add
  deprecation aliases when renaming public tool names.
- **Don't** create new top-level packages without a clear reason — most new
  code belongs in `pkg/github/` or under an existing helper package.
- **Don't** introduce a closure-captured client; use `deps.GetClient(ctx)`.
- **Don't** commit a tool addition without its `__toolsnaps__` snapshot and
  regenerated README.
