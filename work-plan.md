Use this file as a place to stash updated plans, knowledge, results or references so that even if you stop work you can continue without losing all context. Record decisions here too.

---

## Progress & Decisions

### Phase 1 - COMPLETED ✅

**Completed:**
- [x] Read OAuth scopes documentation: https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/scopes-for-oauth-apps
- [x] Created `pkg/scopes/scopes.go` with OAuth scope constants and hierarchy utilities
- [x] Added scopes to ALL tool definitions using `mcp.Tool.Meta` field
- [x] Updated `cmd/github-mcp-server/generate_docs.go` to include scopes in README
- [x] Generated updated README.md with scope information
- [x] Updated all toolsnaps to reflect Meta field with scopes

**Key Decision - Where to Store Scopes:**
- ❌ REJECTED: Adding `RequiredScopes` to `ServerTool` struct in toolsets package
  - Reason: Adds complexity to toolsets package, requires changes to how tools are registered
  - Creates tight coupling between toolsets and scopes packages
- ✅ ACCEPTED: Use `mcp.Tool.Meta` field to store scopes directly on tool definitions
  - The MCP SDK's `Tool` struct has a `Meta map[string]any` field for custom metadata
  - Scopes should be defined where tools are defined (repositories.go, issues.go, etc.)
  - Cleaner separation of concerns - each tool knows its own requirements
  - Easier to maintain - scope info is next to the API calls that require them

**OAuth Scope Mapping (from docs):**
| API Area | Read Scope | Write Scope |
|----------|------------|-------------|
| Repos (public) | (no scope) | `public_repo` |
| Repos (private) | `repo` | `repo` |
| Issues | `repo` | `repo` |
| Pull Requests | `repo` | `repo` |
| Actions | `repo` | `repo` |
| Notifications | `notifications` | `notifications` |
| Gists (public) | (no scope) | `gist` |
| Users (public) | (no scope) | `user` |
| Organizations | `read:org` | `admin:org` |
| Code Scanning | `security_events` | `security_events` |
| Secret Scanning | `repo` | `repo` |
| Projects | `read:project` | `project` |
| Discussions | `repo` | `repo` |

**Files Updated with Scopes (90+ tools):**
- [x] pkg/github/repositories.go - repos tools (repo, public_repo scopes)
- [x] pkg/github/git.go - git tools (repo scope)
- [x] pkg/github/issues.go - issues tools (repo scope)
- [x] pkg/github/pullrequests.go - PR tools (repo scope)
- [x] pkg/github/actions.go - actions tools (repo scope)
- [x] pkg/github/notifications.go - notifications tools (notifications scope)
- [x] pkg/github/discussions.go - discussions tools (repo scope)
- [x] pkg/github/gists.go - gists tools (gist scope for write)
- [x] pkg/github/search.go - search tools (repo scope)
- [x] pkg/github/code_scanning.go - code security (security_events scope)
- [x] pkg/github/secret_scanning.go - secret protection (repo scope)
- [x] pkg/github/dependabot.go - dependabot (repo scope)
- [x] pkg/github/projects.go - projects (project/read:project scope)
- [x] pkg/github/context_tools.go - context tools (no scope, read:org for teams)
- [x] pkg/github/labels.go - labels (repo scope)
- [x] pkg/github/security_advisories.go - security advisories (repo scope)

**Package Structure:**
- `pkg/scopes/scopes.go` - Scope constants, hierarchy map, helper functions
  - `Scope` type and constants (Repo, PublicRepo, Notifications, etc.)
  - `ScopeHierarchy` map showing which scopes include others
  - `WithScopes()` - creates Meta map for tool definitions
  - `GetScopesFromMeta()` - extracts scopes from tool Meta
  - `ScopeIncludes()`, `HasRequiredScopes()` - scope checking utilities

**Documentation:**
- README.md now shows `(scopes: \`repo\`)` after each tool description
- Toolsnaps include `_meta.requiredOAuthScopes` array

---

### Phase 2 - COMPLETED ✅

**Completed:**
- [x] Read fine-grained permissions documentation: https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens
- [x] Extended `pkg/scopes/scopes.go` with fine-grained permission types and helpers:
  - `Permission` type with constants for all fine-grained permissions (actions, contents, issues, etc.)
  - `PermissionLevel` type (read, write, admin)
  - `FineGrainedPermission` struct combining permission and level
  - `WithScopesAndPermissions()` helper for tool metadata
  - `AddPermissions()` helper to add permissions to existing meta
  - `GetPermissionsFromMeta()` to extract permissions
  - `ReadPerm()`, `WritePerm()`, `AdminPerm()` convenience functions
- [x] Added comprehensive tests for fine-grained permissions in `pkg/scopes/scopes_test.go`
- [x] Created `docs/tool-permissions.md` with comprehensive documentation:
  - OAuth scope hierarchy table
  - Fine-grained permission levels explanation
  - Complete tool-by-category permission mapping tables
  - Minimum required scopes by use case
  - Notes about limitations (notifications, metadata, etc.)
- [x] Updated README.md with links to tool-permissions.md:
  - Added link in Prerequisites section
  - Added callout note before Tools section

**Key Decision - Documentation vs Code Changes:**
- ✅ ACCEPTED: Create comprehensive documentation in `docs/tool-permissions.md`
  - Documents all ~90 tools with both OAuth scopes AND fine-grained permissions
  - Easier to maintain as a single reference document
  - Doesn't require modifying every tool file
  - Users can look up permissions by tool or by category
- ⏸️ DEFERRED: Adding fine-grained permissions to every tool's Meta field
  - Would require changes to ~90 tool definitions
  - Phase 1 OAuth scopes are sufficient for tool metadata
  - Documentation approach provides same info with less risk

---

### Phase 3 - COMPLETED ✅

**Completed:**
- [x] Created `cmd/github-mcp-server/list_scopes.go` - new command to list required OAuth scopes
- [x] Created `script/list-scopes` - convenience wrapper script
- [x] Command respects all the same flags as stdio command (--toolsets, --read-only, etc.)
- [x] Three output formats: text (default), json, summary
- [x] JSON output includes: tools, unique_scopes, scopes_by_tool, tools_by_scope
- [x] Lint passes, tests pass

**Implementation Details:**
- Added `list-scopes` subcommand to github-mcp-server binary
- Uses same toolset configuration logic as stdio server
- Creates toolset group with mock clients (no API calls needed)
- Extracts scopes from tool Meta field using existing scopes package
- Calculates accepted scopes (parent scopes that satisfy requirements)

**Usage Examples:**
```bash
# List scopes for default toolsets
github-mcp-server list-scopes

# List scopes for specific toolsets
github-mcp-server list-scopes --toolsets=repos,issues,pull_requests

# List scopes for all toolsets
github-mcp-server list-scopes --toolsets=all

# Output as JSON (for programmatic use)
github-mcp-server list-scopes --output=json

# Just show unique scopes needed
github-mcp-server list-scopes --output=summary

# Read-only mode (excludes write tools)
github-mcp-server list-scopes --read-only --output=summary
```

---

### Phase 4 - COMPLETED ✅

**Completed:**
- [x] Created `pkg/scopes/tool_scope_map.go` with exported types for library use
- [x] Created `pkg/scopes/tool_scope_map_test.go` with comprehensive tests
- [x] Lint passes, tests pass

**Exported Types:**
- `ToolScopeMap` - map[string]*ToolScopeInfo for fast tool name -> scopes lookup
- `ToolScopeInfo` - contains RequiredScopes and AcceptedScopes as ScopeSet
- `ScopeSet` - map[string]bool for O(1) scope lookup
- `ToolMeta` - minimal struct for building scope maps

**Key Methods:**
- `NewToolScopeInfo(required []Scope)` - creates info from required scopes, auto-calculates accepted
- `BuildToolScopeMapFromMeta(tools []ToolMeta)` - builds map from tool definitions
- `GetToolScopeInfo(meta map[string]any)` - creates info from tool Meta field
- `ToolScopeInfo.HasAcceptedScope(userScopes ...string)` - checks if token has access
- `ToolScopeInfo.MissingScopes(userScopes ...string)` - returns missing required scopes
- `ToolScopeMap.AllRequiredScopes()` - returns all unique required scopes
- `ToolScopeMap.ToolsRequiringScope(scope)` - returns tools that require a scope
- `ToolScopeMap.ToolsAcceptingScope(scope)` - returns tools that accept a scope

**Usage Example:**
```go
import "github.com/github/github-mcp-server/pkg/scopes"

// Build scope map from tool definitions
tools := []scopes.ToolMeta{
    {Name: "get_repo", Meta: someToolMeta},
    {Name: "create_issue", Meta: anotherToolMeta},
}
scopeMap := scopes.BuildToolScopeMapFromMeta(tools)

// Check if user's token can use a tool
if info, ok := scopeMap["create_issue"]; ok {
    userScopes := []string{"repo", "user"}
    if info.HasAcceptedScope(userScopes...) {
        // User can use this tool
    } else {
        missing := info.MissingScopes(userScopes...)
        fmt.Printf("Missing scopes: %v\n", missing)
    }
}

// Get all required scopes
allRequired := scopeMap.AllRequiredScopes()
fmt.Printf("All required: %v\n", allRequired.ToSlice())
```

---

## Original Requirements

The phases can be a stacked PR, so each one should have a new branch, and when a phase is complete we want a full pull request based on the previous, with the base branch of phase 1 being omgitsads/go-sdk

IMPORTANT you MUST check all API calls and GraphQL calls etc. and verify what scopes are required. If unsure you must check before proceeding.

These changes must be clean and clear.