package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// checkRepoVisibility returns true if the repository is private, false if public.
// Returns (false, err) on any API error — callers must treat errors as fail-closed.
//
// IMPORTANT: Do NOT use determineRepoAccessType() or RepoAccessCache for this purpose.
// Those functions check authenticated access success, not actual repo visibility.
// A PAT with access to a public repo returns RepoAccessPrivate from those functions,
// which would incorrectly allow writes to public repos.
// This function reads the actual .Private field from the API response.
func checkRepoVisibility(ctx context.Context, client *gogithub.Client, owner, repo string) (bool, error) {
	repoData, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return false, fmt.Errorf("failed to get repository %s/%s: %w", owner, repo, err)
	}
	return repoData.GetPrivate(), nil
}

// WritePrivateOnlyMiddleware returns MCP middleware that blocks repository write
// tool calls targeting public repositories. Must be registered when
// GITHUB_WRITE_PRIVATE_ONLY=true.
//
// Scope: only repo-scoped write tools are guarded. Non-repo writes (gists,
// notifications, etc.) pass through because they have no repository visibility
// to check.
//
// The middleware is fail-closed: any error verifying repository visibility
// (API failure, 404, 403, network error) blocks the write.
//
// Special cases:
//   - create_repository: blocked unless args contain private=true
//   - fork_repository: blocked entirely (GitHub fork API has no visibility param)
//   - read-only tools (ReadOnlyHint=true): passed through without API call
//   - non-tools/call requests: passed through unchanged
//   - write tools without owner/repo (gists, some notifications): passed through
//
// The deps parameter provides the GitHub client used for visibility checks.
// The inv parameter is used to look up tool read-only status.
func WritePrivateOnlyMiddleware(deps ToolDependencies, inv *inventory.Inventory) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only intercept tool calls — pass everything else through unchanged.
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// CallToolRequest = ServerRequest[*CallToolParamsRaw]
			toolReq, ok := req.(*mcp.CallToolRequest)
			if !ok {
				// Unexpected type — fail-open (let the handler deal with it).
				return next(ctx, method, req)
			}

			switch toolReq.Params.Name {
			case "create_repository":
				return handleCreateRepositoryGuard(ctx, method, req, toolReq, next)
			case "fork_repository":
				return blockForkRepository()
			default:
				return handleWriteGuard(ctx, method, req, toolReq, deps, inv, next)
			}
		}
	}
}

// handleWriteGuard enforces the write-private-only policy for standard write tools.
// Read-only tools pass through without any API call.
// Write tools without owner+repo (e.g., create_gist, update_gist) pass through —
// they are not repo-scoped and cannot be checked for visibility.
// Write tools with owner+repo have their repo visibility checked; public repos are blocked.
func handleWriteGuard(
	ctx context.Context,
	method string,
	req mcp.Request,
	toolReq *mcp.CallToolRequest,
	deps ToolDependencies,
	inv *inventory.Inventory,
	next mcp.MethodHandler,
) (mcp.Result, error) {
	// Check ReadOnlyHint via inventory. Treat "not found in inventory" as a write
	// tool (fail-closed for unknown tools — unknown write tools should not bypass guard).
	tool, _, err := inv.FindToolByName(toolReq.Params.Name)
	if err == nil && tool.IsReadOnly() {
		// Read-only tool — pass through without any API call.
		return next(ctx, method, req)
	}

	// Parse arguments to extract owner and repo.
	var args map[string]any
	if err := json.Unmarshal(toolReq.Params.Arguments, &args); err != nil {
		// Malformed arguments — let the handler deal with it.
		return next(ctx, method, req)
	}

	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)

	// Write tools without owner+repo (e.g., create_gist, update_gist) are not
	// repo-scoped and cannot be checked for visibility. Pass through.
	if owner == "" || repo == "" {
		return next(ctx, method, req)
	}

	// Get the GitHub client from deps (captured at middleware construction time).
	client, err := deps.GetClient(ctx)
	if err != nil || client == nil {
		slog.WarnContext(ctx, "write guard: failed to get GitHub client",
			"tool", toolReq.Params.Name, "owner", owner, "repo", repo, "error", err)
		return utils.NewToolResultError(
			"Write blocked: unable to verify repository visibility. " +
				"Ensure your token has repo read access and try again.",
		), nil
	}

	isPrivate, err := checkRepoVisibility(ctx, client, owner, repo)
	if err != nil {
		slog.WarnContext(ctx, "write guard: failed to check repo visibility",
			"tool", toolReq.Params.Name, "owner", owner, "repo", repo, "error", err)
		return utils.NewToolResultError(
			"Write blocked: unable to verify repository visibility. " +
				"Ensure your token has repo read access and try again.",
		), nil
	}

	if !isPrivate {
		slog.WarnContext(ctx, "write guard: blocked write to public repo",
			"tool", toolReq.Params.Name, "owner", owner, "repo", repo)
		return utils.NewToolResultError(fmt.Sprintf(
			"Write blocked: %s/%s is a public repository. "+
				"The server is configured with GITHUB_WRITE_PRIVATE_ONLY=true, which restricts "+
				"repository write operations to private repositories only. "+
				"To proceed: use a private repository, or ask the administrator to unset GITHUB_WRITE_PRIVATE_ONLY.",
			owner, repo,
		)), nil
	}

	return next(ctx, method, req)
}

// handleCreateRepositoryGuard enforces the write-private-only policy for create_repository.
// There is no existing repository to check — the guard performs a pre-flight parameter check.
// Blocked if private=false or if the private param is absent (defaults to false).
// Never silently overrides private=false to private=true.
func handleCreateRepositoryGuard(
	ctx context.Context,
	method string,
	req mcp.Request,
	toolReq *mcp.CallToolRequest,
	next mcp.MethodHandler,
) (mcp.Result, error) {
	var args map[string]any
	if err := json.Unmarshal(toolReq.Params.Arguments, &args); err != nil {
		return utils.NewToolResultError(
			"Write blocked: unable to parse create_repository arguments.",
		), nil
	}

	// If private is absent, Go zero value is false — blocked.
	private, _ := args["private"].(bool)
	if !private {
		return utils.NewToolResultError(
			"Write blocked: create_repository requires private=true when GITHUB_WRITE_PRIVATE_ONLY is set. " +
				"The server will not silently create a public repository. " +
				"Set private=true to create a private repository.",
		), nil
	}

	return next(ctx, method, req)
}

// blockForkRepository blocks fork_repository entirely when GITHUB_WRITE_PRIVATE_ONLY is set.
// GitHub's CreateFork API has no visibility parameter — fork visibility is determined by
// source repo visibility and the user's GitHub plan. There is no way to guarantee a fork
// will be private, so we block entirely rather than risk creating a public fork.
func blockForkRepository() (mcp.Result, error) {
	return utils.NewToolResultError(
		"Write blocked: fork_repository cannot guarantee the fork will be private when " +
			"GITHUB_WRITE_PRIVATE_ONLY is set. GitHub's fork API does not expose a visibility parameter. " +
			"To create a private fork, use the GitHub web UI or API directly.",
	), nil
}
