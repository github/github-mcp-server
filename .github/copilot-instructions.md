# Copilot / AI Agent Instructions (concise)

Purpose: quickly orient an AI coding agent to be productive in this repository.

Key concepts
- **What this project is:** a Model Context Protocol (MCP) server that exposes GitHub API functions as named tools. The server is implemented as a CLI in `cmd/github-mcp-server` and registers toolsets and tools from `pkg/github` and `pkg/toolsets`.
- **Major components:**
  - `cmd/github-mcp-server/*` — CLI entry points (`stdio`, `generate-docs`) and helpers.
  - `internal/ghmcp/server.go` — MCP server construction, client wiring (REST + GraphQL), hooks, and stdio orchestration.
  - `pkg/github/` — tool definitions and toolset metadata. Most tool handlers live in files here (e.g., `issues.go`, `pullrequests.go`).
  - `pkg/toolsets/toolsets.go` — toolset abstraction, read/write tool registration, read-only behavior, and group enablement.
  - `__toolsnaps__` / test snapshots — schema snapshots for each tool (used by tests).

Essential developer workflows (commands)
- Build server binary:
  - `go build -o github-mcp-server ./cmd/github-mcp-server` (PowerShell example below)
- Run stdio server (required env var `GITHUB_PERSONAL_ACCESS_TOKEN`):
  - PowerShell:
    ```powershell
    $env:GITHUB_PERSONAL_ACCESS_TOKEN = 'ghp_...'
    .\github-mcp-server.exe stdio --toolsets=default --read-only=false
    ```
  - Flags of interest: `--toolsets` (comma list, see `pkg/github.GetDefaultToolsetIDs()`), `--dynamic-toolsets`, `--read-only`, `--enable-command-logging`, `--export-translations`, `--log-file`, `--gh-host`.
- Generate docs (updates README and `docs/remote-server.md`):
  - `go run ./cmd/github-mcp-server generate-docs` or `./github-mcp-server generate-docs`
  - CI checks expect README automated sections to match; use this prior to committing tool changes.
- Tests and snapshots:
  - Run unit tests: `go test ./...`
  - If changing tool schemas (tool name, input params, annotations), update snapshots: `UPDATE_TOOLSNAPS=true go test ./...` (CI fails if snapshots are missing/stale).

Project-specific patterns and rules
- Tool registration: tools are composed using `toolsets.NewServerTool(...)` and grouped in `pkg/github.DefaultToolsetGroup`. New tools belong in `pkg/github` and must be added to an appropriate toolset in `DefaultToolsetGroup`.
- Read-only contract: read-only tools must be annotated; `toolsets.Toolset.AddReadTools` panics if a tool is not annotated read-only. Conversely, `AddWriteTools` panics if a tool is incorrectly annotated as read-only — follow annotations in tool constructors.
- Dependency injection for tests: handlers accept `GetClientFn` / `GetGQLClientFn` factories so tests can inject mock clients (`go-github-mock` / `githubv4mock`). Prefer this pattern when writing unit tests.
- Translations: use `translations.TranslationHelper()` to obtain `t` when composing tool messages and titles; `--export-translations` causes translations to be dumped after init.
- Dynamic toolsets: enabling `--dynamic-toolsets` causes the `dynamic` toolset to be used. `internal/ghmcp` wires a dynamic registration flow if enabled.

Integration points & external libs
- REST client: `github.com/google/go-github` (REST)
- GraphQL: `github.com/shurcooL/githubv4` (GQL)
- MCP server: `github.com/mark3labs/mcp-go/server`
- Mocks for tests: `migueleliasweb/go-github-mock`, `internal/githubv4mock`

When adding or modifying tools (checklist)
1. Add implementation in `pkg/github/*.go` and register the tool in `DefaultToolsetGroup`.
2. Add unit tests and/or snapshot tests alongside the implementation file.
3. Run `UPDATE_TOOLSNAPS=true go test ./...` and commit updated `__toolsnaps__` files.
4. Run `go run ./cmd/github-mcp-server generate-docs` and verify README/docs changes.
5. Ensure CI (`.github/workflows/*`) passes — docs and snapshots are commonly enforced.

Quick file references (start here)
- CLI entry: `cmd/github-mcp-server/main.go`
- Docs generator: `cmd/github-mcp-server/generate_docs.go`
- Server bootstrap: `internal/ghmcp/server.go`
- Toolset registration: `pkg/github/tools.go` and `pkg/toolsets/toolsets.go`
- Tests & snapshots: `e2e/`, `__toolsnaps__`, `pkg/*/*_test.go`

Warnings & gotchas
- Do NOT rename or change tool input parameter names/types without updating snapshots — tests will fail in CI.
- `AddWriteTools`/`AddReadTools` enforce annotation rules and may panic during initialization if mismatched.
- `viper` reads env vars under `GITHUB_` prefix (see `initConfig` in `main.go`); pass flags accordingly.

## Example: Adding a read-only tool

**Scenario:** Add a tool to fetch a single collaborator by username from a repo.

**Step 1: Implement in `pkg/github/repos.go` (or similar)**
```go
func GetRepositoryCollaborator(getClient GetClientFn, t translations.TranslationHelperFunc) mcp.Tool {
	return mcp.Tool{
		Name: "get_repository_collaborator",
		Description: "Fetch details of a collaborator in a repository",
		InputSchema: mcp.NewObjectSchema(
			map[string]mcp.Schema{
				"owner":    mcp.NewStringSchema("Repository owner"),
				"repo":     mcp.NewStringSchema("Repository name"),
				"username": mcp.NewStringSchema("Collaborator username"),
			},
			[]string{"owner", "repo", "username"},
		),
		Annotations: &mcp.Annotations{
			ReadOnlyHint: ToBoolPtr(true),
		},
	}
}
```

**Step 2: Add handler in same file**
```go
func getRepositoryCollaboratorHandler(getClient GetClientFn) server.ToolHandlerFunc {
	return func(ctx context.Context, input map[string]interface{}) (*mcp.ToolResultContent, error) {
		owner := input["owner"].(string)
		repo := input["repo"].(string)
		username := input["username"].(string)

		client, err := getClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get client: %w", err)
		}

		collab, _, err := client.Repositories.GetCollaborator(ctx, owner, repo, username)
		if err != nil {
			return nil, fmt.Errorf("failed to get collaborator: %w", err)
		}

		content := map[string]interface{}{
			"login": collab.Login,
			"id":    collab.ID,
			"permissions": map[string]bool{
				"pull":  *collab.Permissions["pull"],
				"push":  *collab.Permissions["push"],
				"admin": *collab.Permissions["admin"],
			},
		}
		return &mcp.ToolResultContent{Type: "text", Text: toJSON(content)}, nil
	}
}
```

**Step 3: Register in `pkg/github/tools.go` → `DefaultToolsetGroup()` → `repos` toolset**
```go
repos := toolsets.NewToolset(ToolsetMetadataRepos.ID, ToolsetMetadataRepos.Description).
	AddReadTools(
		// ... existing tools ...
		toolsets.NewServerTool(GetRepositoryCollaborator(getClient, t), getRepositoryCollaboratorHandler(getClient)),
	)
```

**Step 4: Add test in `pkg/github/repos_test.go`**
```go
func TestGetRepositoryCollaborator(t *testing.T) {
	tool := GetRepositoryCollaborator(mockGetClient, mockTranslator)
	
	// Snapshot test
	require.True(t, *tool.Annotations.ReadOnlyHint)
	require.Equal(t, "get_repository_collaborator", tool.Name)
	
	// Behavioral test
	// ... use mocked client to verify handler logic
}
```

**Step 5: Update snapshots and docs**
```powershell
$env:UPDATE_TOOLSNAPS = "true"
go test ./...
go run ./cmd/github-mcp-server generate-docs
git add __toolsnaps__/*.snap README.md docs/remote-server.md
```

**Step 6: CI validation** — `.github/workflows/` will verify snapshots, docs, and lint match.
