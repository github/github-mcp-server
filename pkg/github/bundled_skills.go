package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/skills"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bundledSkills builds the registry of Agent Skills this server ships.
//
// All bundled skills load uniformly: always-on, no per-skill toolset
// gating, no icons. Their `allowed-tools` frontmatter is advisory only.
// The Registry's `Enabled` closure is still available for future use
// (e.g. feature-flagging a skill behind an experimental toolset).
//
// Adding a new server-bundled skill is one entry here plus a //go:embed
// line in package skills.
func bundledSkills(_ *inventory.Inventory) *skills.Registry {
	return skills.New().
		Add(skills.Bundled{
			Name:        "review-pr",
			Description: "Submit a multi-comment GitHub pull request review using the pending-review workflow (pull_request_review_write → add_comment_to_pending_review → submit_pending). Use when leaving line-specific feedback on a pull request, when asked to review a PR, or whenever creating any review with more than one comment.",
			Content:     skills.ReviewPRSKILL,
		}).
		Add(skills.Bundled{
			Name:        "handle-notifications",
			Description: "Systematically triage the current user's GitHub notifications inbox — enumerate unread items, prioritize by notification reason (review requests, mentions, assignments, security alerts), act on the high-priority ones, then dismiss the rest. Use when the user asks \"what should I work on?\", \"catch me up on GitHub\", \"triage my inbox\", \"what needs my attention?\", or otherwise wants to clear their notifications backlog.",
			Content:     skills.HandleNotificationsSKILL,
		}).
		Add(skills.Bundled{
			Name:        "get-context",
			Description: "Understand the current user, their permissions, and team membership. Use when starting any workflow, checking who you are, what you can access, or looking up team membership.",
			Content:     skills.GetContextSKILL,
		}).
		Add(skills.Bundled{
			Name:        "explore-repo",
			Description: "Understand an unfamiliar codebase quickly. Use when exploring a new repo, understanding project structure, finding entry points, or getting oriented in code you haven't seen before.",
			Content:     skills.ExploreRepoSKILL,
		}).
		Add(skills.Bundled{
			Name:        "search-code",
			Description: "Find code patterns, symbols, and examples across GitHub. Use when searching for code, finding how something is implemented, locating files, or looking for usage examples across repositories.",
			Content:     skills.SearchCodeSKILL,
		}).
		Add(skills.Bundled{
			Name:        "trace-history",
			Description: "Understand why code changed by tracing commits and PRs. Use when investigating git history, finding who changed something, understanding the motivation behind a change, or tracking down when a bug was introduced.",
			Content:     skills.TraceHistorySKILL,
		}).
		Add(skills.Bundled{
			Name:        "create-pr",
			Description: "Create a well-structured pull request that reviews smoothly. Use when opening a new PR, pushing changes for review, or submitting code changes to a repository.",
			Content:     skills.CreatePRSKILL,
		}).
		Add(skills.Bundled{
			Name:        "self-review-pr",
			Description: "Review your own PR before requesting team review. Use when you want to self-check your PR, verify CI status, polish description, or prepare your changes for review.",
			Content:     skills.SelfReviewPRSKILL,
		}).
		Add(skills.Bundled{
			Name:        "address-pr-feedback",
			Description: "Handle review comments on your PR and push fixes. Use when you received PR feedback, need to respond to reviewer comments, resolve threads, or push fixes based on review.",
			Content:     skills.AddressPRFeedbackSKILL,
		}).
		Add(skills.Bundled{
			Name:        "merge-pr",
			Description: "Get a PR to merge-ready state and merge it. Use when merging a pull request, checking if a PR is ready to merge, updating a PR branch, or converting a draft PR.",
			Content:     skills.MergePRSKILL,
		}).
		Add(skills.Bundled{
			Name:        "triage-issues",
			Description: "Categorize, deduplicate, and prioritize incoming issues. Use when triaging issues, labeling bugs, organizing a backlog, closing duplicates, or processing new issue reports.",
			Content:     skills.TriageIssuesSKILL,
		}).
		Add(skills.Bundled{
			Name:        "create-issue",
			Description: "Create well-structured, searchable, actionable issues. Use when filing a bug report, requesting a feature, creating a task, or opening any new GitHub issue.",
			Content:     skills.CreateIssueSKILL,
		}).
		Add(skills.Bundled{
			Name:        "manage-sub-issues",
			Description: "Break down large issues into trackable sub-tasks. Use when decomposing epics, creating task breakdowns, organizing work into smaller pieces, or managing parent-child issue relationships.",
			Content:     skills.ManageSubIssuesSKILL,
		}).
		Add(skills.Bundled{
			Name:        "debug-ci",
			Description: "Investigate and fix failing GitHub Actions workflows. Use when CI is failing, a workflow run errored, you need to read build logs, or debug why tests aren't passing.",
			Content:     skills.DebugCISKILL,
		}).
		Add(skills.Bundled{
			Name:        "trigger-workflow",
			Description: "Run, rerun, or cancel GitHub Actions workflow runs. Use when triggering a deployment, rerunning failed jobs, canceling a stuck workflow, or dispatching a workflow manually.",
			Content:     skills.TriggerWorkflowSKILL,
		}).
		Add(skills.Bundled{
			Name:        "security-audit",
			Description: "Systematically review code scanning, secret, and dependency alerts. Use when auditing repo security, checking for vulnerabilities, reviewing CodeQL alerts, or investigating exposed secrets.",
			Content:     skills.SecurityAuditSKILL,
		}).
		Add(skills.Bundled{
			Name:        "fix-dependabot",
			Description: "Handle vulnerable dependency alerts and update PRs. Use when fixing Dependabot alerts, updating vulnerable packages, reviewing dependency update PRs, or managing supply chain security.",
			Content:     skills.FixDependabotSKILL,
		}).
		Add(skills.Bundled{
			Name:        "research-vulnerability",
			Description: "Query the GitHub Advisory Database for security advisories. Use when researching CVEs, looking up GHSA IDs, checking if a package has known vulnerabilities, or reviewing security advisories for a repo or org.",
			Content:     skills.ResearchVulnerabilitySKILL,
		}).
		Add(skills.Bundled{
			Name:        "manage-project",
			Description: "Track and update work items in GitHub Projects (v2). Use when managing a project board, updating issue status fields, adding items to a project, querying project items, or posting project status updates.",
			Content:     skills.ManageProjectSKILL,
		}).
		Add(skills.Bundled{
			Name:        "prepare-release",
			Description: "Compile release notes from commits and merged PRs. Use when preparing a release, writing a changelog, summarizing changes since last version, or reviewing what shipped.",
			Content:     skills.PrepareReleaseSKILL,
		}).
		Add(skills.Bundled{
			Name:        "manage-repo",
			Description: "Create repos, manage branches, and push file changes. Use when creating a new repository, making a branch, committing files via the API, forking a repo, or managing repository contents.",
			Content:     skills.ManageRepoSKILL,
		}).
		Add(skills.Bundled{
			Name:        "manage-labels",
			Description: "Set up and maintain a consistent label scheme. Use when creating labels, organizing a label system, cleaning up labels, or standardizing label naming across a repository.",
			Content:     skills.ManageLabelsSKILL,
		}).
		Add(skills.Bundled{
			Name:        "contribute-oss",
			Description: "Fork, branch, and submit PRs to external repositories. Use when contributing to open source, forking a repo to make changes, or submitting a pull request to a project you don't own.",
			Content:     skills.ContributeOSSSKILL,
		}).
		Add(skills.Bundled{
			Name:        "browse-discussions",
			Description: "Read and explore GitHub Discussions and categories. Use when browsing discussions, reading community conversations, checking discussion categories, or looking for answers in a project's discussions.",
			Content:     skills.BrowseDiscussionsSKILL,
		}).
		Add(skills.Bundled{
			Name:        "delegate-to-copilot",
			Description: "Assign Copilot to issues and request Copilot PR reviews. Use when you want Copilot to work on an issue, get an automated code review, or delegate tasks to GitHub Copilot.",
			Content:     skills.DelegateToCopilotSKILL,
		}).
		Add(skills.Bundled{
			Name:        "discover-github",
			Description: "Search for users, organizations, and repositories. Use when finding GitHub users, looking up organizations, discovering repos by topic or language, or managing your starred repositories.",
			Content:     skills.DiscoverGitHubSKILL,
		}).
		Add(skills.Bundled{
			Name:        "share-snippet",
			Description: "Create and manage code snippets via GitHub Gists. Use when sharing a code snippet, creating a quick paste, saving notes as a gist, or managing your existing gists.",
			Content:     skills.ShareSnippetSKILL,
		})
}

// DeclareSkillsExtensionIfEnabled adds the skills-over-MCP extension
// (SEP-2133) to the server's capabilities when any bundled skill is
// currently enabled. Must be called before mcp.NewServer.
func DeclareSkillsExtensionIfEnabled(opts *mcp.ServerOptions, inv *inventory.Inventory) {
	bundledSkills(inv).DeclareCapability(opts)
}

// RegisterBundledSkills registers all enabled server-bundled skills and
// the skill://index.json discovery document on the given server.
func RegisterBundledSkills(s *mcp.Server, inv *inventory.Inventory) {
	bundledSkills(inv).Install(s)
}
