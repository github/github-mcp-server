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

# Build the server binary
go build -v ./cmd/github-mcp-server

# Run the server
./github-mcp-server stdio

# Run specific package tests
go test ./pkg/github -v

# Run specific test
go test ./pkg/github -run TestGetMe
```

## Project Structure

### Directory Layout

```
.
├── cmd/
│   ├── github-mcp-server/    # Main MCP server entry point (PRIMARY FOCUS)
│   └── mcpcurl/              # MCP testing utility (secondary - don't break it)
├── pkg/                      # Public API packages
│   ├── github/               # GitHub API MCP tools implementation
│   │   └── __toolsnaps__/    # Tool schema snapshots (*.snap files)
│   ├── toolsets/             # Toolset configuration & management
│   ├── errors/               # Error handling utilities
│   ├── sanitize/             # HTML/content sanitization
│   ├── log/                  # Logging utilities
│   ├── raw/                  # Raw data handling
│   ├── buffer/               # Buffer utilities
│   └── translations/         # i18n translation support
├── internal/                 # Internal implementation packages
│   ├── ghmcp/                # GitHub MCP server core logic
│   ├── githubv4mock/         # GraphQL API mocking for tests
│   ├── toolsnaps/            # Toolsnap validation system
│   └── profiler/             # Performance profiling
├── e2e/                      # End-to-end tests (require GitHub PAT)
├── script/                   # Build and maintenance scripts
├── docs/                     # Documentation
├── .github/workflows/        # CI/CD workflows
└── [config files]            # See below
```

### Key Configuration Files

- **go.mod / go.sum:** Go module dependencies (Go 1.24.0+)
- **.golangci.yml:** Linter configuration (v2 format, ~15 linters enabled)
- **Dockerfile:** Multi-stage build (golang:1.25.3-alpine → distroless)
- **server.json:** MCP server metadata for registry
- **.goreleaser.yaml:** Release automation config
- **.gitignore:** Excludes bin/, dist/, vendor/, *.DS_Store, github-mcp-server binary

### Important Scripts (script/ directory)

- **script/lint** - Runs `gofmt` + `golangci-lint`. **MUST RUN** before committing
- **script/test** - Runs `go test -race ./...` (full test suite)
- **script/generate-docs** - Updates README.md tool documentation. Run after tool changes
- **script/licenses** - Updates third-party license files when dependencies change
- **script/licenses-check** - Validates license compliance (runs in CI)
- **script/get-me** - Quick test script for get_me tool
- **script/get-discussions** - Quick test for discussions
- **script/tag-release** - **NEVER USE THIS** - releases are managed separately

## GitHub Workflows (CI/CD)

All workflows run on push/PR unless noted. Located in `.github/workflows/`:

1. **go.yml** - Build and test on ubuntu/windows/macos. Runs `script/test` and builds binary
2. **lint.yml** - Runs golangci-lint-action v2.5 (GitHub Action) with actions/setup-go stable
3. **docs-check.yml** - Verifies README.md is up-to-date by running generate-docs and checking git diff
4. **code-scanning.yml** - CodeQL security analysis for Go and GitHub Actions
5. **license-check.yml** - Runs `script/licenses-check` to validate compliance
6. **docker-publish.yml** - Publishes container image to ghcr.io
7. **goreleaser.yml** - Creates releases (main branch only)
8. **registry-releaser.yml** - Updates MCP registry

**All of these must pass for PR merge.** If docs-check fails, run `script/generate-docs` and commit changes.

## Testing Guidelines

### Unit Tests

- Use `testify` for assertions (`require` for critical checks, `assert` for non-blocking)
- Tests are in `*_test.go` files alongside implementation (internal tests, not `_test` package)
- Mock GitHub API with `go-github-mock` (REST) or `githubv4mock` (GraphQL)
- Test structure for tools:
  1. Test tool snapshot
  2. Verify critical schema properties (e.g., ReadOnly annotation)
  3. Table-driven behavioral tests

### Toolsnaps (Tool Schema Snapshots)

- Every MCP tool has a JSON schema snapshot in `pkg/github/__toolsnaps__/*.snap`
- Tests fail if current schema differs from snapshot (shows diff)
- To update after intentional changes: `UPDATE_TOOLSNAPS=true go test ./...`
- **MUST commit updated .snap files** - they document API changes
- Missing snapshots cause CI failure

### End-to-End Tests

- Located in `e2e/` directory with `e2e_test.go`
- **Require GitHub PAT token** - you usually cannot run these yourself
- Run with: `GITHUB_MCP_SERVER_E2E_TOKEN=<token> go test -v --tags e2e ./e2e`
- Tests interact with live GitHub API via Docker container
- **Keep e2e tests updated when changing MCP tools**
- **Use only the e2e test style** when modifying tests in this directory
- For debugging: `GITHUB_MCP_SERVER_E2E_DEBUG=true` runs in-process (no Docker)

## Code Style & Linting

### Go Code Requirements

- **gofmt with simplify flag (-s)** - Automatically run by `script/lint`
- **golangci-lint** with these linters enabled:
  - bodyclose, gocritic, gosec, makezero, misspell, nakedret, revive
  - errcheck, staticcheck, govet, ineffassign, unused
- Exclusions for: third_party/, builtin/, examples/, generated code

### Go Naming Conventions

- **Acronyms in identifiers:** Use `ID` not `Id`, `API` not `Api`, `URL` not `Url`, `HTTP` not `Http`
- Examples: `userID`, `getAPI`, `parseURL`, `HTTPClient`
- This applies to variable names, function names, struct fields, etc.

### Code Patterns

- **Keep changes minimal and focused** on the specific issue being addressed
- **Prefer clarity over cleverness** - code must be understandable by a wide audience
- **Atomic commits** - each commit should be a complete, logical change
- **Maintain or improve structure** - never degrade code organization
- Use table-driven tests for behavioral testing
- Comment sparingly - code should be self-documenting
- Follow standard Go conventions (Effective Go, Go proverbs)
- **Test changes thoroughly** before committing
- Export functions (capitalize) if they could be used by other repos as a library

## Common Development Workflows

### Adding a New MCP Tool

1. Add tool implementation in `pkg/github/` (e.g., `foo_tools.go`)
2. Register tool in appropriate toolset in `pkg/github/` or `pkg/toolsets/`
3. Write unit tests following the tool test pattern
4. Run `UPDATE_TOOLSNAPS=true go test ./...` to create snapshot
5. Run `script/generate-docs` to update README
6. Run `script/lint` and `script/test` before committing
7. If e2e tests are relevant, update `e2e/e2e_test.go` using existing test style
8. Commit code + snapshots + README changes together

### Fixing a Bug

1. Write a failing test that reproduces the bug
2. Fix the bug with minimal changes
3. Verify test passes and existing tests still pass
4. Run `script/lint` and `script/test`
5. If tool schema changed, update toolsnaps (see above)

### Updating Dependencies

1. Update `go.mod` (e.g., `go get -u ./...` or manually)
2. Run `go mod tidy`
3. Run `script/licenses` to update license files
4. Run `script/test` to verify nothing broke
5. Commit go.mod, go.sum, and third-party-licenses* files

## Common Errors & Solutions

### "Documentation is out of date" in CI

**Fix:** Run `script/generate-docs` and commit README.md changes

### Toolsnap mismatch failures

**Fix:** Run `UPDATE_TOOLSNAPS=true go test ./...` and commit updated .snap files

### Lint failures

**Fix:** Run `script/lint` locally - it will auto-format and show issues. Fix manually reported issues.

### License check failures

**Fix:** Run `script/licenses` to regenerate license files after dependency changes

### Test failures after changing a tool

**Likely causes:**
1. Forgot to update toolsnaps - run with `UPDATE_TOOLSNAPS=true`
2. Changed behavior broke existing tests - verify intent and fix tests
3. Schema change not reflected in test - update test expectations

## Environment Variables

- **GITHUB_PERSONAL_ACCESS_TOKEN** - Required for server operation and e2e tests
- **GITHUB_HOST** - For GitHub Enterprise Server (prefix with `https://`)
- **GITHUB_TOOLSETS** - Comma-separated toolset list (overrides --toolsets flag)
- **GITHUB_READ_ONLY** - Set to "1" for read-only mode
- **GITHUB_DYNAMIC_TOOLSETS** - Set to "1" for dynamic toolset discovery
- **UPDATE_TOOLSNAPS** - Set to "true" when running tests to update snapshots
- **GITHUB_MCP_SERVER_E2E_TOKEN** - Token for e2e tests
- **GITHUB_MCP_SERVER_E2E_DEBUG** - Set to "true" for in-process e2e debugging

## Key Files Reference

### Root Directory Files
```
.dockerignore        - Docker build exclusions
.gitignore          - Git exclusions (includes bin/, dist/, vendor/, binaries)
.golangci.yml       - Linter configuration
.goreleaser.yaml    - Release automation
CODE_OF_CONDUCT.md  - Community guidelines
CONTRIBUTING.md     - Contribution guide (fork, clone, test, lint workflow)
Dockerfile          - Multi-stage Go build
LICENSE             - MIT license
README.md           - Main documentation (auto-generated sections)
SECURITY.md         - Security policy
SUPPORT.md          - Support resources
gemini-extension.json - Gemini CLI configuration
go.mod / go.sum     - Go dependencies
server.json         - MCP server registry metadata
```

### Main Entry Point

`cmd/github-mcp-server/main.go` - Uses cobra for CLI, viper for config, supports:
- `stdio` command (default) - MCP stdio transport
- `generate-docs` command - Documentation generation
- Flags: --toolsets, --read-only, --dynamic-toolsets, --gh-host, --log-file

## Important Reminders

1. **PRIMARY FOCUS:** The local stdio MCP server (github-mcp-server) - this is what you should work on and test with
2. **REMOTE SERVER:** Ignore remote server instructions when making code changes (unless specifically asked). This repo is used as a library by the remote server, so keep functions exported (capitalized) if they could be called by other repos, even if not needed internally.
3. **ALWAYS** trust these instructions first - only search if information is incomplete or incorrect
4. **NEVER** use `script/tag-release` or push tags
5. **NEVER** skip `script/lint` before committing Go code changes
6. **ALWAYS** update toolsnaps when changing MCP tool schemas
7. **ALWAYS** run `script/generate-docs` after modifying tools
8. For specific test files, use `go test ./path -run TestName` not full suite
9. E2E tests require PAT token - you likely cannot run them
10. Toolsnaps are API documentation - treat changes seriously
11. Build/test/lint are very fast (~1s each) - run frequently
12. CI failures for docs-check or license-check have simple fixes (run the script)
13. mcpcurl is secondary - don't break it, but it's not the priority