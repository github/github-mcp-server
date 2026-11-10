package ghmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// repoAllowlist restricts tool calls and resource reads to a fixed set of
// repositories. Entries are "owner/repo" or "owner/<glob>", where the repo part
// may use path.Match wildcards (for example "octo-org/*" or "octo-org/svc-*").
// Matching is case-insensitive, like GitHub names.
//
// The check is applied to the arguments of each request before the tool runs:
// owner/repo arguments must match an entry, owner-wide arguments (an
// organization or a project owner) need an "owner/*" entry, and search queries
// must carry repo:, org: or user: qualifiers that stay inside the allowlist.
// Tools that cannot be tied to a repository are rejected, except for a small
// set that never returns repository content.
type repoAllowlist struct {
	entries []repoAllowlistEntry
}

type repoAllowlistEntry struct {
	owner string
	repo  string // path.Match pattern, lower case
}

// allowlistExemptTools never read or write repository content, so they keep
// working when an allowlist is configured.
var allowlistExemptTools = map[string]bool{
	"get_me":                          true,
	"get_teams":                       true,
	"get_team_members":                true,
	"search_users":                    true,
	"search_orgs":                     true,
	"get_global_security_advisory":    true,
	"list_global_security_advisories": true,
	"create_gist":                     true,
	"get_gist":                        true,
	"list_gists":                      true,
	"update_gist":                     true,
}

// parseRepoAllowlist validates the configured entries. It returns nil when no
// entries are given, which disables the check.
func parseRepoAllowlist(values []string) (*repoAllowlist, error) {
	var list repoAllowlist
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		owner, repo, ok := strings.Cut(value, "/")
		if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
			return nil, fmt.Errorf("invalid allowed repository %q: expected owner/repo or owner/*", value)
		}
		if strings.ContainsAny(owner, "*?[]\\") {
			return nil, fmt.Errorf("invalid allowed repository %q: wildcards are only supported in the repository name", value)
		}
		repo = strings.ToLower(repo)
		if _, err := path.Match(repo, ""); err != nil {
			return nil, fmt.Errorf("invalid allowed repository %q: %w", value, err)
		}
		list.entries = append(list.entries, repoAllowlistEntry{owner: strings.ToLower(owner), repo: repo})
	}
	if len(list.entries) == 0 {
		return nil, nil
	}
	return &list, nil
}

func (l *repoAllowlist) allowsRepo(owner, repo string) bool {
	owner, repo = strings.ToLower(owner), strings.ToLower(repo)
	for _, e := range l.entries {
		if e.owner != owner {
			continue
		}
		if ok, _ := path.Match(e.repo, repo); ok {
			return true
		}
	}
	return false
}

// allowsOwner reports whether every repository of owner is allowed, which is
// required for calls scoped to an owner rather than a single repository.
func (l *repoAllowlist) allowsOwner(owner string) bool {
	owner = strings.ToLower(owner)
	for _, e := range l.entries {
		if e.owner == owner && e.repo == "*" {
			return true
		}
	}
	return false
}

func (l *repoAllowlist) String() string {
	parts := make([]string, len(l.entries))
	for i, e := range l.entries {
		parts[i] = e.owner + "/" + e.repo
	}
	return strings.Join(parts, ", ")
}

// checkToolCall returns a non-empty reason when the call must be rejected.
func (l *repoAllowlist) checkToolCall(name string, rawArgs json.RawMessage) string {
	if allowlistExemptTools[name] {
		return ""
	}

	args := map[string]any{}
	if len(rawArgs) > 0 {
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "tool arguments could not be read to check the repository allowlist"
		}
	}
	str := func(key string) string {
		s, _ := args[key].(string)
		return strings.TrimSpace(s)
	}

	scoped := false
	checkPair := func(owner, repo string) string {
		switch {
		case owner != "" && repo != "":
			scoped = true
			if !l.allowsRepo(owner, repo) {
				return fmt.Sprintf("repository %s/%s is not in the allowed repositories", owner, repo)
			}
		case owner != "":
			scoped = true
			if !l.allowsOwner(owner) {
				return fmt.Sprintf("owner-wide access to %s requires %s/* in the allowed repositories", owner, owner)
			}
		}
		return ""
	}

	if reason := checkPair(str("owner"), str("repo")); reason != "" {
		return reason
	}
	if reason := checkPair(str("item_owner"), str("item_repo")); reason != "" {
		return reason
	}
	if org := str("org"); org != "" {
		if reason := checkPair(org, ""); reason != "" {
			return reason
		}
	}
	if name == "create_repository" {
		org := str("organization")
		if org == "" {
			return "create_repository needs an organization when allowed repositories are configured"
		}
		if reason := checkPair(org, str("name")); reason != "" {
			return reason
		}
	}

	if query, ok := args["query"].(string); ok && strings.HasPrefix(name, "search_") {
		found, reason := l.checkSearchQuery(query)
		if reason != "" {
			return reason
		}
		scoped = scoped || found
	}

	if !scoped {
		return fmt.Sprintf("%s is not scoped to a repository; pass owner and repo (or repo:owner/name in a search query) for one of the allowed repositories", name)
	}
	return ""
}

// checkSearchQuery validates the scope qualifiers in a GitHub search query.
// Multiple qualifiers are OR-ed by GitHub search, so every positive repo:,
// org: and user: qualifier must be allowed. Negated qualifiers only narrow the
// results and are ignored.
func (l *repoAllowlist) checkSearchQuery(query string) (scoped bool, reason string) {
	for field := range strings.FieldsSeq(query) {
		key, value, ok := strings.Cut(field, ":")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"'()`)
		switch strings.ToLower(strings.TrimLeft(key, "(")) {
		case "repo":
			owner, repo, ok := strings.Cut(value, "/")
			if !ok || !l.allowsRepo(owner, repo) {
				return false, fmt.Sprintf("search qualifier repo:%s is not in the allowed repositories", value)
			}
			scoped = true
		case "org", "user", "owner":
			if !l.allowsOwner(value) {
				return false, fmt.Sprintf("search qualifier %s requires %s/* in the allowed repositories", field, value)
			}
			scoped = true
		}
	}
	return scoped, ""
}

// checkResourceURI validates repo:// resource URIs.
func (l *repoAllowlist) checkResourceURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "repo" {
		return fmt.Sprintf("resource %s is not scoped to an allowed repository", uri)
	}
	owner := u.Host
	repo, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
	if owner == "" || repo == "" || !l.allowsRepo(owner, repo) {
		return fmt.Sprintf("repository %s/%s is not in the allowed repositories", owner, repo)
	}
	return ""
}

// allowedReposMiddleware enforces the allowlist on tools/call and
// resources/read before any GitHub API request is made.
func allowedReposMiddleware(list *repoAllowlist) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			switch req := request.(type) {
			case *mcp.CallToolRequest:
				if req.Params != nil {
					if reason := list.checkToolCall(req.Params.Name, req.Params.Arguments); reason != "" {
						return utils.NewToolResultError(fmt.Sprintf("access denied: %s (allowed: %s)", reason, list)), nil
					}
				}
			case *mcp.ReadResourceRequest:
				if req.Params != nil {
					if reason := list.checkResourceURI(req.Params.URI); reason != "" {
						return nil, fmt.Errorf("access denied: %s (allowed: %s)", reason, list)
					}
				}
			}
			return next(ctx, method, request)
		}
	}
}
