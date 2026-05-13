# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -v ./cmd/github-mcp-server

# Lint (runs gofmt -s -w . then golangci-lint) — must run before committing
script/lint

# Test (runs go test -race ./...)
script/test

# Run a single test
go test ./pkg/github -run TestGetMe

# Update tool schema snapshots after changing any MCP tool definition
UPDATE_TOOLSNAPS=true go test ./...

# Regenerate README.md after modifying tools or toolsets
script/generate-docs

# Update third-party license files after changing dependencies
script/licenses
```

**Never use `script/tag-release`** — releases are managed separately.

## Architecture

This is a **Go MCP server** that exposes GitHub APIs as Model Context Protocol tools. The primary binary is `cmd/github-mcp-server`, which runs as a stdio MCP server. The repo is also consumed as a library by a remote server hosted by GitHub, so **exported symbols must remain exported** even if unused internally.

### Request flow

```
stdio → mcp.Server (modelcontextprotocol/go-sdk)
           ↓  middleware (user-agent injection, error context)
        tool handler (from context: MustDepsFromContext)
           ↓
        GitHub REST (google/go-github) or GraphQL (shurcooL/githubv4)
```

Startup sequence in `internal/ghmcp/server.go`:
1. Parse API host (dotcom / GHEC / GHES)
2. Create GitHub REST + GraphQL + raw clients
3. Build an `inventory.Inventory` via `inventory.Builder` — this is where toolset filtering, read-only mode, feature flags, and OAuth scope filtering are applied
4. Create `mcp.Server`, attach middlewares, inject `BaseDeps` into context via `ContextWithDeps`
5. Call `inventory.RegisterAll(ctx, server, deps)` to register all filtered tools/resources/prompts

### Tool definition pattern

Every tool lives in `pkg/github/` and follows this structure:

```go
func GetMe(t translations.TranslationHelperFunc) inventory.ServerTool {
    return NewTool(
        ToolsetMetadataContext,           // which toolset this belongs to
        mcp.Tool{Name: "get_me", ...},    // MCP tool definition with schema
        []scopes.Scope{scopes.ReadOrg},  // required OAuth scopes (nil = none)
        func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
            client, _ := deps.GetClient(ctx)
            // ... call GitHub API, return MarshalledTextResult(result)
        },
    )
}
```

All tools are registered in `AllTools()` in `pkg/github/tools.go`. A new tool must be added there to be discoverable.

### Toolset system

Toolsets (`pkg/github/tools.go` vars `ToolsetMetadata*`) are named groups (e.g. `repos`, `issues`, `pull_requests`). Tools declare their toolset via the first argument to `NewTool`. The `inventory.Builder` (`pkg/inventory/builder.go`) filters the registered set based on enabled toolsets, read-only mode, and feature flags. The special keyword `"default"` expands to toolsets marked `Default: true`.

### Dependency injection

`ToolDependencies` (interface in `pkg/github/dependencies.go`) provides the GitHub REST client, GraphQL client, raw content client, and feature flags. For the local server, `BaseDeps` implements this. Dependencies are injected into the `context.Context` by a middleware at server startup; handlers retrieve them via `MustDepsFromContext(ctx)`. This design lets the remote server inject per-request deps with the same handler code.

### Parameter helpers

`pkg/github/server.go` provides typed helpers for extracting MCP arguments:
- `RequiredParam[T]`, `OptionalParam[T]` — generic typed extraction
- `RequiredInt`, `OptionalIntParam`, `OptionalIntParamWithDefault` — numeric helpers (MCP sends numbers as `float64`)
- `OptionalStringArrayParam`, `OptionalBigIntArrayParam` — array helpers
- `WithPagination`, `WithUnifiedPagination`, `OptionalPaginationParams` — pagination schema + extraction

### Tool schema snapshots (toolsnaps)

Every tool has a `.snap` file in `pkg/github/__toolsnaps__/`. Tests fail if the current schema differs from the snapshot. **After any tool schema change**, run `UPDATE_TOOLSNAPS=true go test ./...` and commit the updated `.snap` files. Missing or mismatched snapshots fail CI.

### Testing patterns

Tests in `pkg/github/` mock the GitHub REST API using the helpers in `helper_test.go`:
- `NewMockedHTTPClient(WithRequestMatch(...))` — maps endpoint patterns to response fixtures
- `WithRequestMatchHandler(pattern, handler)` — for request body inspection
- `mockResponse(t, statusCode, body)` — simple JSON response fixture
- `createMCPRequest(args)` — builds a `mcp.CallToolRequest` from a map
- `getTextResult`, `getErrorResult` — unwrap tool call results in assertions

GraphQL is mocked via `internal/githubv4mock/`.

Standard test structure per tool:
1. Snapshot test (validates schema hasn't changed unexpectedly)
2. Annotation check (e.g. `ReadOnlyHint` must be set for read-only tools)
3. Table-driven behavioral tests

### Feature flags

Tools can be gated behind feature flags using `FeatureFlagEnable` on the toolset metadata and `deps.IsFeatureEnabled(ctx, flagName)` in the handler. Flags are enabled at startup via `--features` CLI flag or `MCPServerConfig.EnabledFeatures`.

## Key conventions

- **Acronyms**: `ID` not `Id`, `API` not `Api`, `URL` not `Url`, `HTTP` not `Http`
- **Write tools** must NOT set `ReadOnlyHint: true` on `mcp.ToolAnnotations`
- **Error returns**: use `ghErrors.NewGitHubAPIErrorResponse(ctx, msg, res, err)` for REST errors and `ghErrors.NewGitHubGraphQLErrorResponse` for GraphQL errors — do not return raw `error` from tool handlers
- **Result encoding**: `MarshalledTextResult(v)` for JSON responses; `utils.NewToolResultText(s)` for plain text; `utils.NewToolResultError(msg)` / `utils.NewToolResultErrorFromErr` for tool-level errors
- **Translations**: wrap all user-visible strings in `t("KEY", "fallback")` — the fallback is used if no translation is loaded

## CI checks that must pass

- `script/lint` — gofmt + golangci-lint
- `script/test` — full test suite with race detector
- `script/generate-docs` — README must be up to date (run and commit if not)
- `script/licenses-check` — run `script/licenses` after dependency changes
- Toolsnap validation — snapshots must match current tool schemas

## Environment variables

| Variable | Purpose |
|---|---|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Required for server operation |
| `GITHUB_HOST` | GitHub Enterprise Server (e.g. `https://github.example.com`) |
| `GITHUB_TOOLSETS` | Comma-separated toolset list |
| `GITHUB_READ_ONLY` | Set to `1` for read-only mode |
| `GITHUB_DYNAMIC_TOOLSETS` | Set to `1` for dynamic toolset discovery |
| `UPDATE_TOOLSNAPS` | Set to `true` when running tests to update snapshots |
| `GITHUB_MCP_SERVER_E2E_TOKEN` | PAT for e2e tests |
| `GITHUB_MCP_SERVER_E2E_DEBUG` | Set to `true` for in-process e2e debugging (no Docker) |
