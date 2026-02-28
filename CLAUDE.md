# CLAUDE.md - Claude Code Configuration

## Project Overview

This is a fork of the **GitHub MCP Server**, a Model Context Protocol (MCP) server that connects AI tools to GitHub. It enables AI agents to manage repositories, issues, pull requests, workflows, and more.

- **Language:** Go 1.24+
- **Primary binary:** `cmd/github-mcp-server` (stdio MCP server)
- **Secondary binary:** `cmd/mcpcurl` (testing utility)
- **Frameworks:** `modelcontextprotocol/go-sdk`, `google/go-github/v82`, `cobra`, `viper`

## Quick Reference Commands

```bash
# Build
go build -v ./cmd/github-mcp-server

# Test (with race detection)
script/test

# Lint (gofmt + golangci-lint)
script/lint

# Update tool snapshots (after changing MCP tool definitions)
UPDATE_TOOLSNAPS=true go test ./...

# Regenerate README docs (after modifying tools/toolsets)
script/generate-docs

# Run a specific test
go test ./pkg/github -run TestName
```

## Before Every Commit

Always run in this order:

1. `script/lint`
2. `script/test`
3. `script/generate-docs` (only if MCP tools were modified)

All three commands are fast (~1s each when cached).

## Project Structure

```
cmd/github-mcp-server/    Main MCP server entry point
cmd/mcpcurl/               MCP curl testing utility
pkg/github/                GitHub API MCP tools (~70 files, main implementation)
pkg/github/__toolsnaps__/  Tool schema snapshots (*.snap files)
internal/ghmcp/            Core MCP server logic
internal/githubv4mock/     GraphQL API mocking for tests
e2e/                       End-to-end tests (require PAT)
script/                    Build/test/lint scripts
docs/                      Documentation
ui/                        React-based UI (Vite + TypeScript)
```

## Key Patterns

- **Toolsnaps:** Every MCP tool has a JSON schema snapshot in `pkg/github/__toolsnaps__/`. If you change tool definitions, run `UPDATE_TOOLSNAPS=true go test ./...` and commit the updated `.snap` files.
- **Testing:** Use `testify` (`require` for critical, `assert` for non-blocking). Mock REST with `go-github-mock`, GraphQL with `githubv4mock`. Use table-driven tests.
- **Naming:** Use `ID` not `Id`, `API` not `Api`, `URL` not `Url`, `HTTP` not `Http`.
- **Exports:** Keep functions exported (capitalized) if they could be used as a library by other repos.
- **E2E tests:** Require `GITHUB_MCP_SERVER_E2E_TOKEN` — usually can't run locally.

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GITHUB_PERSONAL_ACCESS_TOKEN` | Required for server operation |
| `GITHUB_HOST` | GitHub Enterprise host (prefix with `https://`) |
| `GITHUB_TOOLSETS` | Comma-separated toolsets to enable |
| `GITHUB_READ_ONLY` | Set to `1` for read-only mode |
| `GITHUB_DYNAMIC_TOOLSETS` | Set to `1` for dynamic toolset discovery |
| `GITHUB_API_KEY` | Pre-shared API key for HTTP endpoint (clients send via `X-API-Key` header) |
| `PORT` | HTTP listen port (auto-set by Railway/Cloud Run, falls back to `GITHUB_PORT` or `8082`) |

## Railway Deployment

The repo includes `railway.json` for one-click Railway deployment. Required env vars in Railway dashboard:

- `GITHUB_PERSONAL_ACCESS_TOKEN` — your GitHub PAT
- `GITHUB_API_KEY` — pre-shared key to protect the endpoint

The server auto-reads Railway's `PORT` env var. HTTPS is provided by Railway.

MCP clients connect via:
```
https://<your-app>.up.railway.app/
```
with headers:
```
X-API-Key: <your-api-key>
Authorization: Bearer <github-pat>
```

## Common CI Failures

- **"Documentation is out of date"** → Run `script/generate-docs` and commit
- **Toolsnap mismatch** → Run `UPDATE_TOOLSNAPS=true go test ./...` and commit `.snap` files
- **Lint failures** → Run `script/lint` to auto-format, fix remaining issues
- **License check** → Run `script/licenses` after dependency changes
