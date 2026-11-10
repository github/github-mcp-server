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

// repoAllowlist restricts tool calls, resource reads and completions to a
// fixed set of repositories. Entries are "owner/repo" or "owner/<glob>", where
// the repo part may use path.Match wildcards (for example "octo-org/*" or
// "octo-org/svc-*"). Matching is case-insensitive, like GitHub names.
//
// The check is applied to the arguments of each request before the tool runs.
// It fails closed: a call is only allowed when every repository it can reach
// is known from its arguments and is in the list. Owner-wide calls need an
// "owner/*" entry, search queries must carry allowed repo:, org: or user:
// qualifiers, and tools whose target cannot be told from their arguments
// (notification threads, project items, enterprise-level settings) are
// rejected, except for a small set of tools that never return repository
// content.
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

// allowlistBlockedTools address their target by an opaque identifier (a
// notification thread), so the repository involved cannot be checked here.
var allowlistBlockedTools = map[string]string{
	"dismiss_notification":             "notification threads are not tied to a repository",
	"get_notification_details":         "notification threads are not tied to a repository",
	"manage_notification_subscription": "notification threads are not tied to a repository",
}

// allowlistRepoRequiredTools fall back to account-wide endpoints unless both
// owner and repo are given, so an owner-only call is not enough for them.
var allowlistRepoRequiredTools = map[string]bool{
	"list_notifications":          true,
	"mark_all_notifications_read": true,
}

// allowlistBlockedProjectMethods read or write project items whose
// repositories are not known from the arguments, or address a project
// resource by node ID alone.
var allowlistBlockedProjectMethods = map[string]string{
	"list_project_items":        "project items may belong to repositories outside the allowlist",
	"get_project_item":          "project items may belong to repositories outside the allowlist",
	"get_project_status_update": "the project cannot be told from a node ID",
	"get_project_view":          "the project cannot be told from a node ID",
}

// allowlistItemRefProjectMethods act on a single project item, which must be
// named by item_owner, item_repo and issue_number so its repository can be
// checked.
var allowlistItemRefProjectMethods = map[string]bool{
	"add_project_item":    true,
	"update_project_item": true,
	"delete_project_item": true,
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

// checkPair validates an owner/repo pair taken from tool arguments. An owner
// without a repository is an owner-wide call and needs an owner/* entry.
func (l *repoAllowlist) checkPair(owner, repo string) string {
	switch {
	case owner != "" && repo != "":
		if !l.allowsRepo(owner, repo) {
			return fmt.Sprintf("repository %s/%s is not in the allowed repositories", owner, repo)
		}
	case owner != "":
		if !l.allowsOwner(owner) {
			return fmt.Sprintf("owner-wide access to %s requires %s/* in the allowed repositories", owner, owner)
		}
	}
	return ""
}

// checkToolCall returns a non-empty reason when the call must be rejected.
func (l *repoAllowlist) checkToolCall(name string, rawArgs json.RawMessage) string {
	if allowlistExemptTools[name] {
		return ""
	}
	if reason, blocked := allowlistBlockedTools[name]; blocked {
		return fmt.Sprintf("%s is not available when allowed repositories are configured: %s", name, reason)
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

	owner, repo := str("owner"), str("repo")
	scoped := owner != "" || str("org") != ""

	// Tools with a "level" argument (rulesets, custom properties) only use the
	// arguments of the selected level, so other arguments must not count.
	if level := str("level"); level != "" {
		switch level {
		case "repository":
			if owner == "" || repo == "" {
				return fmt.Sprintf("%s at the repository level needs owner and repo for one of the allowed repositories", name)
			}
		case "organization":
			if str("org") == "" {
				return fmt.Sprintf("%s at the organization level needs org, with an org/* entry in the allowed repositories", name)
			}
		default:
			return fmt.Sprintf("%s at the %s level is not available when allowed repositories are configured", name, level)
		}
	}

	if allowlistRepoRequiredTools[name] && (owner == "" || repo == "") {
		return fmt.Sprintf("%s needs both owner and repo when allowed repositories are configured; without them it covers every repository", name)
	}

	if reason := l.checkPair(owner, repo); reason != "" {
		return reason
	}
	if org := str("org"); org != "" {
		if reason := l.checkPair(org, ""); reason != "" {
			return reason
		}
	}

	// Secondary repository targets. Missing parts default the way the
	// handlers default them.
	if po, pr := str("parent_owner"), str("parent_repo"); po != "" || pr != "" {
		if po == "" {
			po = owner
		}
		if pr == "" {
			pr = repo
		}
		if reason := l.checkPair(po, pr); reason != "" {
			return reason
		}
	}
	if ro, rr := str("related_owner"), str("related_repo"); ro != "" || rr != "" {
		if ro == "" {
			ro = owner
		}
		if rr == "" {
			rr = repo
		}
		if reason := l.checkPair(ro, rr); reason != "" {
			return reason
		}
	}
	if reason := l.checkPair(str("item_owner"), str("item_repo")); reason != "" {
		return reason
	}

	switch name {
	case "create_repository":
		org := str("organization")
		if org == "" {
			return "create_repository needs an organization when allowed repositories are configured"
		}
		if reason := l.checkPair(org, str("name")); reason != "" {
			return reason
		}
		scoped = true
	case "fork_repository":
		org := str("organization")
		if org == "" {
			return "fork_repository needs an organization when allowed repositories are configured; forks into the personal account cannot be checked"
		}
		if reason := l.checkPair(org, repo); reason != "" {
			return reason
		}
	case "projects_list", "projects_get", "projects_write":
		if reason := l.checkProjectCall(name, args, str); reason != "" {
			return reason
		}
	}

	if query, ok := args["query"].(string); ok && strings.HasPrefix(name, "search_") {
		found, reason := l.checkSearchQuery(query)
		if reason != "" {
			return reason
		}
		// search_issues and search_pull_requests add repo:owner/repo to the
		// query for the owner/repo arguments, but only when the query has no
		// repo: qualifier of its own. Every other search tool ignores them.
		argsScope := (name == "search_issues" || name == "search_pull_requests") &&
			owner != "" && repo != "" && !found.anyRepoQualifier
		scoped = found.scoped || argsScope
	}

	if !scoped {
		return fmt.Sprintf("%s is not scoped to a repository; pass owner and repo (or repo:owner/name in a search query) for one of the allowed repositories", name)
	}
	return ""
}

// checkProjectCall rejects project methods whose items cannot be tied to an
// allowed repository. Projects are owner-wide, so the owner argument has
// already been checked against an owner/* entry.
func (l *repoAllowlist) checkProjectCall(name string, args map[string]any, str func(string) string) string {
	method := str("method")
	if reason, blocked := allowlistBlockedProjectMethods[method]; blocked {
		return fmt.Sprintf("%s %s is not available when allowed repositories are configured: %s", name, method, reason)
	}
	if allowlistItemRefProjectMethods[method] && (str("item_owner") == "" || str("item_repo") == "") {
		return fmt.Sprintf("%s %s needs item_owner and item_repo for one of the allowed repositories", name, method)
	}
	if method != "update_project_items" {
		return ""
	}
	items, _ := args["items"].([]any)
	if len(items) == 0 {
		return fmt.Sprintf("%s %s needs items that name item_owner and item_repo for allowed repositories", name, method)
	}
	for i, raw := range items {
		entry, ok := raw.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s %s: item %d could not be read to check the repository allowlist", name, method, i)
		}
		itemOwner, _ := entry["item_owner"].(string)
		itemRepo, _ := entry["item_repo"].(string)
		itemOwner, itemRepo = strings.TrimSpace(itemOwner), strings.TrimSpace(itemRepo)
		if itemOwner == "" || itemRepo == "" {
			return fmt.Sprintf("%s %s: item %d must be named by item_owner, item_repo and issue_number when allowed repositories are configured", name, method, i)
		}
		if !l.allowsRepo(itemOwner, itemRepo) {
			return fmt.Sprintf("%s %s: item %d is in %s/%s, which is not in the allowed repositories", name, method, i, itemOwner, itemRepo)
		}
	}
	return ""
}

// searchQueryScope is what checkSearchQuery learned about a query.
type searchQueryScope struct {
	// scoped is true when a positive, unquoted repo:, org:, user: or owner:
	// qualifier restricts the query to allowed repositories.
	scoped bool
	// anyRepoQualifier is true when the query carries any repo: qualifier,
	// positive or negated, which stops the issue search tools from adding
	// their own.
	anyRepoQualifier bool
}

// checkSearchQuery validates the scope qualifiers in a GitHub search query.
// Multiple qualifiers are OR-ed by GitHub search, so every repo:, org:, user:
// and owner: qualifier must be allowed, and only positive, unquoted ones count
// as scope: a negated qualifier only narrows the results, and a quoted one is
// free text to the lexical search. Boolean operators and parentheses are
// rejected because a qualifier in one branch does not constrain the others.
func (l *repoAllowlist) checkSearchQuery(query string) (searchQueryScope, string) {
	var found searchQueryScope
	for field := range strings.FieldsSeq(query) {
		if field == "OR" || field == "NOT" || field == "AND" || strings.ContainsAny(field, "()") {
			return found, "search queries with boolean operators or parentheses are not supported when allowed repositories are configured; use plain qualifiers"
		}
		quoted := strings.ContainsAny(field, `"'`)
		negated := strings.HasPrefix(field, "-")
		key, value, ok := strings.Cut(strings.TrimPrefix(field, "-"), ":")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"'`)
		switch strings.ToLower(key) {
		case "repo":
			found.anyRepoQualifier = true
			if negated {
				continue
			}
			owner, repo, ok := strings.Cut(value, "/")
			if !ok || !l.allowsRepo(owner, repo) {
				return found, fmt.Sprintf("search qualifier repo:%s is not in the allowed repositories", value)
			}
			found.scoped = found.scoped || !quoted
		case "org", "user", "owner":
			if negated {
				continue
			}
			if !l.allowsOwner(value) {
				return found, fmt.Sprintf("search qualifier %s:%s requires %s/* in the allowed repositories", key, value, value)
			}
			found.scoped = found.scoped || !quoted
		}
	}
	return found, ""
}

// uiResourceScheme is the scheme of the bundled MCP App resources, which only
// serve static HTML and never touch a repository.
const uiResourceScheme = "ui"

// checkResourceURI validates resource URIs. repo:// URIs must name an allowed
// repository; the static ui:// resources are always allowed; anything else is
// rejected.
func (l *repoAllowlist) checkResourceURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Sprintf("resource %s is not scoped to an allowed repository", uri)
	}
	switch u.Scheme {
	case uiResourceScheme:
		return ""
	case "repo":
		owner := u.Host
		repo, _, _ := strings.Cut(strings.TrimPrefix(u.Path, "/"), "/")
		if owner == "" || repo == "" || !l.allowsRepo(owner, repo) {
			return fmt.Sprintf("repository %s/%s is not in the allowed repositories", owner, repo)
		}
		return ""
	default:
		return fmt.Sprintf("resource %s is not scoped to an allowed repository", uri)
	}
}

// checkCompletion validates a completion request for a repo:// resource
// template. Owner suggestions and repository suggestions for an owner without
// an owner/* entry would list repositories outside the allowlist, so they are
// rejected; every other argument is completed only for an allowed repository.
func (l *repoAllowlist) checkCompletion(params *mcp.CompleteParams) string {
	if params == nil || params.Ref == nil || params.Ref.Type != "ref/resource" {
		return ""
	}
	if !strings.HasPrefix(params.Ref.URI, "repo://") {
		return ""
	}
	var owner, repo string
	if params.Context != nil {
		owner = strings.TrimSpace(params.Context.Arguments["owner"])
		repo = strings.TrimSpace(params.Context.Arguments["repo"])
	}
	switch params.Argument.Name {
	case "owner":
		return "owner completions are not available when allowed repositories are configured"
	case "repo":
		if owner == "" || !l.allowsOwner(owner) {
			return fmt.Sprintf("repository completions for %q require %s/* in the allowed repositories", owner, owner)
		}
		return ""
	default:
		if owner == "" || repo == "" {
			return "completions need a resolved owner and repo for one of the allowed repositories"
		}
		return l.checkPair(owner, repo)
	}
}

// allowedReposMiddleware enforces the allowlist on tools/call, resources/read
// and completion/complete before any GitHub API request is made.
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
			case *mcp.CompleteRequest:
				if reason := list.checkCompletion(req.Params); reason != "" {
					return nil, fmt.Errorf("access denied: %s (allowed: %s)", reason, list)
				}
			}
			return next(ctx, method, request)
		}
	}
}
