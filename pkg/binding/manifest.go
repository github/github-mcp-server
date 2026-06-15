package binding

import "fmt"

// ToolBinding declares how a single tool appears and behaves inside a scoped
// mode. It is the per-tool slice of the interface spec: which fixed values are
// injected, which parameters and method values are removed from the advertised
// schema, and how the tool is described to the end user.
//
// Membership is explicit: a tool with no ToolBinding in a Manifest is omitted
// from that mode entirely (fail-closed). Adding a new tool to the server does
// nothing to a scoped surface until it is deliberately admitted here.
type ToolBinding struct {
	// Bind maps a tool input-schema parameter to the Context value that is
	// injected for it. Bound parameters are removed from the advertised schema
	// and may not be supplied by the caller.
	Bind map[string]ctxKey

	// MethodAllow restricts a multi-method tool's "method" parameter to this
	// set. The advertised enum is narrowed to it and the runtime rejects any
	// other value. Empty means "no method restriction".
	MethodAllow []string

	// MethodDeny removes specific "method" values even if otherwise allowed.
	// Applied as defense in depth alongside MethodAllow.
	MethodDeny []string

	// ParamReject lists parameters that must not be supplied and are removed
	// from the advertised schema (e.g. cross-repo target parameters).
	ParamReject []string

	// QueryGuard rejects a "query" parameter that contains a repo:, org:, or
	// user: qualifier, which would otherwise escape the bound context.
	QueryGuard bool

	// Description replaces the tool's advertised description so the surface
	// reads as purpose-built for the bound context rather than as a generic
	// tool with parameters removed. Required for every admitted tool.
	Description string

	// Title optionally overrides the tool's human-facing display title.
	Title string
}

// Manifest is the curated interface spec for one scoped mode: the exact set of
// tools the mode exposes, keyed by canonical tool name.
type Manifest struct {
	Kind  Kind
	Admit map[string]ToolBinding
}

// ManifestFor returns the manifest for a scoped kind.
func ManifestFor(kind Kind) (Manifest, bool) {
	m, ok := manifests[kind]
	return m, ok
}

// bindRepo binds the {owner, repo} pair shared by every repository-targeted
// tool.
var bindRepo = map[string]ctxKey{"owner": keyOwner, "repo": keyRepo}

// bindPull binds {owner, repo, pullNumber} for tools whose subject is the
// bound pull request.
var bindPull = map[string]ctxKey{"owner": keyOwner, "repo": keyRepo, "pullNumber": keyPullNumber}

// bindProject binds {owner, owner_type, project_number} for project-native
// tools.
var bindProject = map[string]ctxKey{"owner": keyOwner, "owner_type": keyOwnerType, "project_number": keyProjectNumber}

var manifests = map[Kind]Manifest{
	KindRepo:        repoManifest,
	KindPullRequest: pullRequestManifest,
	KindProject:     projectManifest,
}

// repoManifest is the "single repository" surface: file, branch, commit, issue,
// and pull request operations confined to one {owner, repo}.
var repoManifest = Manifest{
	Kind: KindRepo,
	Admit: map[string]ToolBinding{
		// Files & contents.
		"get_file_contents": {
			Bind:        bindRepo,
			Description: "Read a file's contents or list a directory in this repository.",
		},
		"create_or_update_file": {
			Bind:        bindRepo,
			Description: "Create a new file or update an existing file in this repository.",
		},
		"delete_file": {
			Bind:        bindRepo,
			Description: "Delete a file from this repository.",
		},
		"push_files": {
			Bind:        bindRepo,
			Description: "Commit and push multiple file changes to a branch in this repository in a single operation.",
		},
		// Branches & history.
		"list_branches": {
			Bind:        bindRepo,
			Description: "List the branches in this repository.",
		},
		"create_branch": {
			Bind:        bindRepo,
			Description: "Create a new branch in this repository.",
		},
		"list_commits": {
			Bind:        bindRepo,
			Description: "List commits on a branch of this repository.",
		},
		"get_commit": {
			Bind:        bindRepo,
			Description: "Get the details and diff of a single commit in this repository.",
		},
		// Issues.
		"list_issues": {
			Bind:        bindRepo,
			Description: "List issues in this repository.",
		},
		"issue_read": {
			Bind:        bindRepo,
			Description: "Read an issue in this repository: its details, comments, sub-issues, or labels.",
		},
		"create_issue": {
			Bind:        bindRepo,
			Description: "Open a new issue in this repository.",
		},
		"add_issue_comment": {
			Bind:        bindRepo,
			Description: "Add a comment to an issue or pull request in this repository.",
		},
		"search_issues": {
			Bind:        bindRepo,
			QueryGuard:  true,
			Description: "Search issues within this repository.",
		},
		// Pull requests.
		"list_pull_requests": {
			Bind:        bindRepo,
			Description: "List pull requests in this repository.",
		},
		"pull_request_read": {
			Bind:        bindRepo,
			Description: "Read a pull request in this repository: its details, diff, changed files, commits, reviews, comments, or status.",
		},
		"create_pull_request": {
			Bind:        bindRepo,
			Description: "Open a new pull request in this repository.",
		},
		"search_pull_requests": {
			Bind:        bindRepo,
			QueryGuard:  true,
			Description: "Search pull requests within this repository.",
		},
	},
}

// pullRequestManifest is the "single pull request" surface: every tool whose
// subject is the bound PR, plus a couple of repository reads for context.
var pullRequestManifest = Manifest{
	Kind: KindPullRequest,
	Admit: map[string]ToolBinding{
		"pull_request_read": {
			Bind:        bindPull,
			Description: "Read this pull request: its details, diff, changed files, commits, reviews, review comments, or status.",
		},
		"update_pull_request_title": {
			Bind:        bindPull,
			Description: "Update this pull request's title.",
		},
		"update_pull_request_body": {
			Bind:        bindPull,
			Description: "Update this pull request's description.",
		},
		"update_pull_request_state": {
			Bind:        bindPull,
			Description: "Open or close this pull request.",
		},
		"update_pull_request_draft_state": {
			Bind:        bindPull,
			Description: "Mark this pull request as a draft or as ready for review.",
		},
		"update_pull_request_branch": {
			Bind:        bindPull,
			Description: "Update this pull request's branch with the latest changes from its base branch.",
		},
		"merge_pull_request": {
			Bind:        bindPull,
			Description: "Merge this pull request.",
		},
		"request_pull_request_reviewers": {
			Bind:        bindPull,
			Description: "Request reviewers on this pull request.",
		},
		"request_copilot_review": {
			Bind:        bindPull,
			Description: "Request a GitHub Copilot review on this pull request.",
		},
		"create_pull_request_review": {
			Bind:        bindPull,
			Description: "Create a review on this pull request.",
		},
		"add_pull_request_review_comment": {
			Bind:        bindPull,
			Description: "Add an inline review comment to a line of this pull request's diff.",
		},
		"add_comment_to_pending_review": {
			Bind:        bindPull,
			Description: "Add a comment to your pending review on this pull request.",
		},
		"add_reply_to_pull_request_comment": {
			Bind:        bindPull,
			Description: "Reply to an existing review comment on this pull request.",
		},
		"submit_pending_pull_request_review": {
			Bind:        bindPull,
			Description: "Submit your pending review on this pull request.",
		},
		"delete_pending_pull_request_review": {
			Bind:        bindPull,
			Description: "Discard your pending review on this pull request.",
		},
		"pull_request_review_write": {
			Bind: bindPull,
			// Thread operations address review threads by global node ID,
			// which is not constrained to this pull request, so they are
			// removed from the advertised method enum and rejected at runtime.
			// threadId only feeds those operations, so it is removed too.
			MethodDeny:  []string{"resolve_thread", "unresolve_thread"},
			ParamReject: []string{"threadId"},
			Description: "Create, submit, or discard a pending review on this pull request.",
		},
		// Repository reads that give a reviewer file and commit context.
		"get_file_contents": {
			Bind:        bindRepo,
			Description: "Read a file's contents or list a directory in this pull request's repository.",
		},
		"get_commit": {
			Bind:        bindRepo,
			Description: "Get the details and diff of a single commit in this pull request's repository.",
		},
	},
}

// projectManifest is the "single project" surface: the project-native read and
// write operations for one ProjectsV2 project. Cross-project enumeration and
// project creation are removed from the method enums.
var projectManifest = Manifest{
	Kind: KindProject,
	Admit: map[string]ToolBinding{
		"projects_get": {
			Bind: bindProject,
			// get_project_status_update addresses a status update by global id,
			// which is not constrained to this project; status_update_id only
			// feeds that method, so it is removed from the schema too.
			MethodAllow: []string{"get_project", "get_project_field", "get_project_item"},
			ParamReject: []string{"status_update_id"},
			Description: "Read this project: the project itself, one of its fields, or one of its items.",
		},
		"projects_list": {
			Bind: bindProject,
			// list_projects enumerates every project owned by the owner,
			// escaping the single bound project.
			MethodAllow: []string{"list_project_fields", "list_project_items", "list_project_status_updates"},
			Description: "List this project's fields, items, or status updates.",
		},
		"projects_write": {
			Bind: bindProject,
			// create_project creates a new project under the owner, outside the
			// bound project.
			MethodAllow: []string{"add_project_item", "update_project_item", "delete_project_item", "create_project_status_update", "create_iteration_field"},
			Description: "Manage this project: add, update, or remove items, post status updates, and create iteration fields.",
		},
	},
}

// ServerTitle returns a human-facing server title for the bound context, used
// to present the scoped server as a distinct product.
func (c Context) ServerTitle() string {
	switch c.Kind {
	case KindRepo:
		return fmt.Sprintf("GitHub Repository · %s/%s", c.Owner, c.Repo)
	case KindPullRequest:
		return fmt.Sprintf("GitHub Pull Request · %s/%s#%d", c.Owner, c.Repo, c.PullNumber)
	case KindProject:
		return fmt.Sprintf("GitHub Project · %s/%d (%s)", c.Owner, c.ProjectNumber, c.OwnerType)
	default:
		return "GitHub"
	}
}

// ServerInstructions returns a one-line description of the bound context for
// the server's instructions, stating that the context is fixed.
func (c Context) ServerInstructions() string {
	switch c.Kind {
	case KindRepo:
		return fmt.Sprintf("This server operates only on the %s/%s repository. The repository is fixed; tools act on it automatically and do not accept an owner or repo.", c.Owner, c.Repo)
	case KindPullRequest:
		return fmt.Sprintf("This server operates only on pull request %s/%s#%d. The repository and pull request are fixed; tools act on them automatically.", c.Owner, c.Repo, c.PullNumber)
	case KindProject:
		return fmt.Sprintf("This server operates only on project number %d owned by %s. The project is fixed; tools act on it automatically.", c.ProjectNumber, c.Owner)
	default:
		return ""
	}
}
