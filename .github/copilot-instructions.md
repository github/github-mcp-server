# GitHub MCP Server - Copilot Instructions

## Project Overview

This is the **GitHub MCP Server**, a Model Context Protocol (MCP) server that connects AI tools to GitHub's platform. It enables AI agents to manage repositories, issues, pull requests, workflows, and more through natural language.

**Key Details:**
- **Language:** Go 1.24+ (~38k lines of code)
- **Type:** MCP server application with CLI interface
- **Primary Package:** github-mcp-server (stdio MCP server - **this is the main focus**)
- **Secondary Package:** mcpcurl (testing utility - don't break it, but not the priority)
- **Framework:** Uses modelcontextprotocol/go-sdk for MCP protocol, google/go-github for GitHub API
- **Size:** ~60MB repository, 70 Go files
- **Library Usage:** This repository is also used as a library by the remote server. Functions that could be called by other repositories should be exported (capitalized), even if not required internally. Preserve existing export patterns.

**Code Quality Standards:**
- **Popular Open Source Repository** - High bar for code quality and clarity
- **Comprehension First** - Code must be clear to a wide audience
- **Clean Commits** - Atomic, focused changes with clear messages
- **Structure** - Always maintain or improve, never degrade
- **Code over Comments** - Prefer self-documenting code; comment only when necessary

## Critical Build & Validation Steps

### Required Commands (Run Before Committing)

**ALWAYS run these commands in this exact order before using report_progress or finishing work:**

1. **Format Code:** `script/lint` (runs `gofmt -s -w .` then `golangci-lint`)
2. **Run Tests:** `script/test` (runs `go test -race ./...`)
3. **Update Documentation:** `script/generate-docs` (if you modified MCP tools/toolsets)

**These commands are FAST:** Lint ~1s, Tests ~1s (cached), Build ~1s

### When Modifying MCP Tools/Endpoints

If you change any MCP tool definitions or schemas:
1. Run tests with `UPDATE_TOOLSNAPS=true go test ./...` to update toolsnaps
2. Commit the updated `.snap` files in `pkg/github/__toolsnaps__/`
3. Run `script/generate-docs` to update README.md
4. Toolsnaps document API surface and ensure changes are intentional

### Common Build Commands

```bash
# Download dependencies (rarely needed - usually cached)
go mod download
## Quick agent guide — GitHub MCP Server

This repository implements a local Model Context Protocol (MCP) server for GitHub. Focus areas for an AI coding agent:

- Primary server: [cmd/github-mcp-server](cmd/github-mcp-server) — build with `go build ./cmd/github-mcp-server` and run `./github-mcp-server stdio`.
- Tools implementation: `pkg/github/` (tool defs, prompts, and tool registration). Tool schema snapshots live in `pkg/github/__toolsnaps__/`.
- Core runtime: `internal/ghmcp/` — server wiring, middleware (see user-agent middleware in `internal/ghmcp/server.go`).

Required quick commands (run before committing):

- `script/lint` — formats and runs `golangci-lint` (always run).
- `script/test` — runs `go test -race ./...` (use `-run TestName` for focused tests).
- If you changed tool schemas: `UPDATE_TOOLSNAPS=true go test ./...` and commit files from `pkg/github/__toolsnaps__/`.
- If you changed tools/docs: `script/generate-docs` to refresh README tool docs.

Important patterns and conventions (project-specific):

- Tool snapshots: every MCP tool has a JSON `.snap` in `pkg/github/__toolsnaps__`. Tests fail on snapshot drift — update intentionally with `UPDATE_TOOLSNAPS=true`.
- Tool registration & prompts: search `pkg/github/*_tools.go`, `prompts.go`, and `workflow_prompts.go` for examples of tool prompts and usage (e.g., `AssignCodingAgentPrompt`).
- Export surface: this repo is consumed as a library by other servers — prefer exporting functions (capitalize) when they might be reused.
- Naming: acronyms use ALL CAPS in identifiers (`ID`, `HTTP`, `API`, `URL`).
- Tests: table-driven tests are common. Mocks used: `go-github-mock` (REST) and `internal/githubv4mock` (GraphQL).

Where to look for quick examples:

- Tool snapshot example: [pkg/github/__toolsnaps__/assign_copilot_to_issue.snap](pkg/github/__toolsnaps__/assign_copilot_to_issue.snap)
- Tool prompts and registration: [pkg/github/issues.go](pkg/github/issues.go) and [pkg/github/prompts.go](pkg/github/prompts.go)
- Main server entry: [cmd/github-mcp-server/main.go](cmd/github-mcp-server/main.go)
- Core wiring and middleware: [internal/ghmcp/server.go](internal/ghmcp/server.go)

CI and workflows:

- CI runs `script/test` and `script/lint`. `docs-check.yml` ensures `script/generate-docs` is run when tools change.
- Don't use `script/tag-release`; releases are managed separately.

Developer tips for agents:

- When adding or modifying a tool: implement in `pkg/github/`, add tests, run `UPDATE_TOOLSNAPS=true go test ./...`, run `script/generate-docs`, then `script/lint` and `script/test`.
- Use focused tests: `go test ./pkg/github -run TestName` or `go test ./pkg/github -run TestGetMe`.
- E2E tests require a GitHub PAT and are in `e2e/` (run with `GITHUB_MCP_SERVER_E2E_TOKEN=<token> go test -v --tags e2e ./e2e`).

If anything in these instructions is unclear or missing, tell me which areas you'd like expanded (examples, common code locations, or sample PR checklist). 
Below is an optional appendix with concrete commands, CI notes, a PR checklist, and a short commit/PR template for changes that affect toolsnaps or generated docs.

---

**Appendix — Scripts, CI notes, PR checklist**

- Quick scripts (run before commit or when CI fails):
	- `script/lint` — formats and runs `golangci-lint` (auto-fixes formatting).
	- `script/test` — full unit test suite (`go test -race ./...`).
	- `UPDATE_TOOLSNAPS=true go test ./...` — regenerate tool schema snapshots (`pkg/github/__toolsnaps__`).
	- `script/generate-docs` — refreshes README sections derived from tools.

- Common commands:
	- Build server: `go build ./cmd/github-mcp-server`
	- Run server: `./github-mcp-server stdio`
	- Run focused tests: `go test ./pkg/github -run TestName`
	- Run e2e (requires PAT): `GITHUB_MCP_SERVER_E2E_TOKEN=<token> go test -v --tags e2e ./e2e`

- CI notes (when a CI job fails):
	- docs-check.yml failing → run `script/generate-docs` and commit README changes.
	- lint.yml failing → run `script/lint`, fix reported issues, re-run tests.
	- license-check.yml failing → run `script/licenses` then commit updated third-party license files.

- PR checklist for changes affecting tools or docs (include in PR description):
	1. Run `script/lint` and `script/test` locally.
	2. If tool schema changed: run `UPDATE_TOOLSNAPS=true go test ./...` and commit updated `.snap` files from `pkg/github/__toolsnaps__/`.
	3. If tools or prompts changed: run `script/generate-docs` and include README diffs.
	4. Ensure table-driven tests cover new behavior; add mocks in `internal/githubv4mock` if needed.
	5. Verify CI passes `docs-check`, `lint`, and `license-check` before requesting review.

- Minimal PR commit/description template for toolsnaps/docs changes:

	Title: `pkg/github: <short description of change>`

	Body:
	- What: one-sentence summary of the change.
	- Why: brief rationale and intended effect on MCP tools.
	- Dev steps performed:
		- `script/lint` ✅
		- `script/test` ✅
		- `UPDATE_TOOLSNAPS=true go test ./...` (if applicable) ✅
		- `script/generate-docs` (if applicable) ✅
	- Files to review: `pkg/github/<file.go>`, `pkg/github/__toolsnaps__/*.snap`, README changes.

If you want, I can also add a small `.github/PULL_REQUEST_TEMPLATE.md` using this template. Say the word and I will create it.
