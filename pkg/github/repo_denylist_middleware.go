package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// searchToolNames is the set of MCP tool names that perform GitHub searches.
// Used by SearchDenylistMiddleware to identify which tools to inspect for query qualifiers.
var searchToolNames = map[string]bool{
	"search_repositories":  true,
	"search_code":          true,
	"search_issues":        true,
	"search_users":         true,
	"search_pull_requests": true,
	"search_orgs":          true,
}

// repoURIPattern extracts owner and repo from a resource URI of the form repo://{owner}/{repo}/...
// The (?i) flag makes matching case-insensitive for defensive correctness.
var repoURIPattern = regexp.MustCompile(`(?i)^repo://([^/]+)/([^/]+)/`)

// RepoDenylistMiddleware returns MCP middleware that blocks tool calls targeting
// denied repositories. Checks owner/repo from tool arguments.
//
// Blocks both reads and writes. Runs before any GitHub API call.
// For search tools, use SearchDenylistMiddleware instead (or in addition).
//
// The denylist is checked before any API call — denied repos are rejected
// without revealing whether they exist (no 404 leakage).
//
// Limitation: create_repository and fork_repository without an "organization"
// param target the user's personal namespace, which is not checked against the
// denylist. Resolving the current user's login would require an API call in the
// middleware hot path. Personal namespace denylisting is not a supported use case.
//
// Nil-safe: a nil or empty denylist returns a no-op pass-through middleware.
func RepoDenylistMiddleware(denylist *RepoDenylist) mcp.Middleware {
	if denylist == nil || denylist.IsEmpty() {
		return func(next mcp.MethodHandler) mcp.MethodHandler { return next }
	}

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			toolReq, ok := req.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, req)
			}

			var args map[string]any
			if err := json.Unmarshal(toolReq.Params.Arguments, &args); err != nil {
				// Malformed args — let the handler report the error.
				return next(ctx, method, req)
			}

			owner, _ := args["owner"].(string)
			repo, _ := args["repo"].(string)

			// Check standard owner/repo (used by most tools).
			if owner != "" && repo != "" {
				if denylist.IsDenied(owner, repo) {
					slog.WarnContext(ctx, "denylist: blocked tool call to denied repo",
						"owner", owner, "repo", repo, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: %s/%s is on the repository denylist.", owner, repo,
					)), nil
				}
			}

			// Also check "organization" param for destination org enforcement.
			// Used by create_repository (organization + name) and fork_repository
			// (organization only — forks inherit the source repo name).
			//
			// For fork_repository both owner/repo (source) AND organization (dest)
			// may be present — we must check both independently.
			organization, _ := args["organization"].(string)
			if organization != "" {
				// Check org wildcard (org/*) — blocks creating/forking into denied orgs.
				if denylist.IsOrgDenied(organization) {
					slog.WarnContext(ctx, "denylist: blocked tool call to denied organization",
						"organization", organization, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: org %s is on the repository denylist.", organization,
					)), nil
				}

				// Check exact match (org/repo). Use "name" if present (create_repository),
				// otherwise fall back to "repo" (fork_repository inherits source repo name).
				repoName, _ := args["name"].(string)
				if repoName == "" {
					repoName = repo // source repo name — forks inherit this by default
				}
				if repoName != "" && denylist.IsDenied(organization, repoName) {
					slog.WarnContext(ctx, "denylist: blocked tool call to denied repo",
						"owner", organization, "repo", repoName, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: %s/%s is on the repository denylist.", organization, repoName,
					)), nil
				}
			}

			// Check owner-only org wildcard (owner/* entries). Catches tools that
			// pass owner without repo (e.g., list operations on an org).
			if owner != "" && repo == "" && denylist.IsOrgDenied(owner) {
				slog.WarnContext(ctx, "denylist: blocked tool call to denied org via owner param",
					"owner", owner, "tool", toolReq.Params.Name)
				return utils.NewToolResultError(fmt.Sprintf(
					"Access denied: org %s is on the repository denylist.", owner,
				)), nil
			}

			// Check "org" param for org-scoped tools (e.g., get_team_members,
			// list_org_repository_security_advisories). These tools don't use
			// owner+repo but still access org-level data that should be denied.
			orgParam, _ := args["org"].(string)
			if orgParam != "" && denylist.IsOrgDenied(orgParam) {
				slog.WarnContext(ctx, "denylist: blocked tool call to denied org",
					"org", orgParam, "tool", toolReq.Params.Name)
				return utils.NewToolResultError(fmt.Sprintf(
					"Access denied: org %s is on the repository denylist.", orgParam,
				)), nil
			}

			return next(ctx, method, req)
		}
	}
}

// SearchDenylistMiddleware returns MCP middleware that blocks search tool calls
// with query qualifiers targeting denied repositories or organizations.
//
// Blocks queries with explicit repo: or org:/user: qualifiers matching denied entries.
// Also blocks if owner+repo args are passed directly (some search tools accept these).
// Does NOT filter search results — unscoped searches that happen to return results
// from denied repos are not blocked. This is a deliberate design decision:
// post-response filtering would require wrapping every search tool handler and
// parsing GitHub API response formats, which is fragile and architecturally
// expensive. The denylist prevents intentional targeting of denied repos via
// qualifiers; incidental appearance in broad search results is accepted.
//
// Nil-safe: a nil or empty denylist returns a no-op pass-through middleware.
func SearchDenylistMiddleware(denylist *RepoDenylist) mcp.Middleware {
	if denylist == nil || denylist.IsEmpty() {
		return func(next mcp.MethodHandler) mcp.MethodHandler { return next }
	}

	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			toolReq, ok := req.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, req)
			}

			// Only apply to search tools.
			if !searchToolNames[toolReq.Params.Name] {
				return next(ctx, method, req)
			}

			var args map[string]any
			if err := json.Unmarshal(toolReq.Params.Arguments, &args); err != nil {
				// Malformed args — let the handler report the error.
				return next(ctx, method, req)
			}

			// Check (a): direct owner+repo args (some search tools accept these).
			owner, _ := args["owner"].(string)
			repo, _ := args["repo"].(string)
			if owner != "" && repo != "" {
				if denylist.IsDenied(owner, repo) {
					slog.WarnContext(ctx, "denylist: blocked search tool call with denied owner/repo args",
						"owner", owner, "repo", repo, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: %s/%s is on the repository denylist.", owner, repo,
					)), nil
				}
			}

			// Check (b): query string for repo: and org:/user: qualifiers.
			query, _ := args["query"].(string)
			if query == "" {
				// No query string — nothing to inspect.
				return next(ctx, method, req)
			}

			// Check all repo: qualifiers (catches multi-qualifier bypass attempts).
			for _, ri := range extractAllReposFromQuery(query) {
				if denylist.IsDenied(ri.owner, ri.repo) {
					slog.WarnContext(ctx, "denylist: blocked search with denied repo: qualifier",
						"owner", ri.owner, "repo", ri.repo, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: %s/%s is on the repository denylist.", ri.owner, ri.repo,
					)), nil
				}
			}

			// Check all org:/user: qualifiers (catches multi-qualifier bypass
			// attempts like "user:allowed org:denied").
			for _, org := range extractAllOrgsFromQuery(query) {
				if denylist.IsOrgDenied(org) {
					slog.WarnContext(ctx, "denylist: blocked search with denied org: qualifier",
						"org", org, "tool", toolReq.Params.Name)
					return utils.NewToolResultError(fmt.Sprintf(
						"Access denied: org %s is on the repository denylist.", org,
					)), nil
				}
			}

			return next(ctx, method, req)
		}
	}
}

// DenylistResourceHandler wraps a resource handler and blocks access to denied repositories.
// Parses the owner and repo from the resource URI using the pattern repo://{owner}/{repo}/...
//
// If the URI cannot be parsed or the repo is not denied, the request is passed through
// to the underlying handler unchanged.
//
// Nil-safe: a nil or empty denylist returns the handler unwrapped.
func DenylistResourceHandler(denylist *RepoDenylist, handler mcp.ResourceHandler) mcp.ResourceHandler {
	if denylist == nil || denylist.IsEmpty() {
		return handler
	}

	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI

		matches := repoURIPattern.FindStringSubmatch(uri)
		if len(matches) == 3 {
			owner, repo := matches[1], matches[2]
			if denylist.IsDenied(owner, repo) {
				slog.WarnContext(ctx, "denylist: blocked resource access to denied repo",
					"owner", owner, "repo", repo, "uri", uri)
				return nil, fmt.Errorf("access denied: %s/%s is on the repository denylist", owner, repo)
			}
		}

		return handler(ctx, req)
	}
}
