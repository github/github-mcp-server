package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// skillsExtensionID is the MCP capabilities extension key (SEP-2640) that signals a server
// publishes a skill discovery index. Clients that recognise it read skillIndexURI to enumerate
// skills and the tools each one governs without first fetching every SKILL.md.
const skillsExtensionID = "io.modelcontextprotocol/skills"

// skillIndexURI is the well-known resource enumerating the server's skills (SEP-2640).
const skillIndexURI = "skill://index.json"

// skillIndexSchema is the Agent Skills discovery-index schema URI advertised in index.json.
const skillIndexSchema = "https://schemas.agentskills.io/discovery/0.2.0/index.json"

// skillIndexEntry is one entry in the skill discovery index. `allowedTools` is an additive
// (passthrough) hint, so SEP-2640-aware clients can gate/defer those tools before a skill is
// loaded WITHOUT first fetching SKILL.md frontmatter.
type skillIndexEntry struct {
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	URL          string   `json:"url"`
	AllowedTools []string `json:"allowedTools"`
}

// skillIndexDocument is the full `skill://index.json` payload.
type skillIndexDocument struct {
	Schema string            `json:"$schema"`
	Skills []skillIndexEntry `json:"skills"`
}

// buildSkillIndex constructs the SEP-2640 discovery index from the in-memory skill set. Each entry
// points at the skill's individually readable SKILL.md resource and carries its allowed-tools list.
func buildSkillIndex() skillIndexDocument {
	skills := allSkills()
	doc := skillIndexDocument{
		Schema: skillIndexSchema,
		Skills: make([]skillIndexEntry, 0, len(skills)),
	}
	for _, skill := range skills {
		doc.Skills = append(doc.Skills, skillIndexEntry{
			Type:         "skill-md",
			Name:         skill.name,
			Description:  skill.description,
			URL:          fmt.Sprintf("skill://github/%s/SKILL.md", skill.name),
			AllowedTools: skill.allowedTools,
		})
	}
	return doc
}

// buildSkillIndexJSON serialises the discovery index returned by skill://index.json.
func buildSkillIndexJSON() (string, error) {
	body, err := json.MarshalIndent(buildSkillIndex(), "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal skill index: %w", err)
	}
	return string(body), nil
}

// skillDefinition holds the metadata and content for a single skill resource.
type skillDefinition struct {
	// name is the skill identifier used in frontmatter and URI
	name string
	// description is a short summary of the skill's purpose
	description string
	// allowedTools lists the MCP tool names associated with this skill
	allowedTools []string
	// body is the markdown instruction content (after frontmatter)
	body string
}

// allSkills returns all skill definitions for the GitHub MCP Server.
// Each skill maps to a user workflow and provides targeted guidance.
func allSkills() []skillDefinition {
	return []skillDefinition{
		skillGetContext(),
		skillExploreRepo(),
		skillSearchCode(),
		skillTraceHistory(),
		skillCreatePR(),
		skillReviewPR(),
		skillSelfReviewPR(),
		skillAddressPRFeedback(),
		skillMergePR(),
		skillTriageIssues(),
		skillCreateIssue(),
		skillManageSubIssues(),
		skillDebugCI(),
		skillTriggerWorkflow(),
		skillSecurityAudit(),
		skillFixDependabot(),
		skillResearchVulnerability(),
		skillManageProject(),
		skillHandleNotifications(),
		skillPrepareRelease(),
		skillManageRepo(),
		skillManageLabels(),
		skillContributeOSS(),
		skillBrowseDiscussions(),
		skillDelegateCopilot(),
		skillDiscoverGitHub(),
		skillShareSnippet(),
	}
}

func skillGetContext() skillDefinition {
	return skillDefinition{
		name:        "get-context",
		description: "Understand the current user, their permissions, and team membership. Use when starting any workflow, checking who you are, what you can access, or looking up team membership.",
		allowedTools: []string{
			"get_me",
			"get_teams",
			"get_team_members",
		},
		body: "# Get Context\n\nAlways call `get_me` first to establish who you are and what you can access.\n\n## Available Tools\n- `get_me` — your authenticated profile and permissions\n- `get_teams` — teams you belong to\n- `get_team_members` — members of a specific team\n",
	}
}

func skillExploreRepo() skillDefinition {
	return skillDefinition{
		name:        "explore-repo",
		description: "Understand an unfamiliar codebase quickly. Use when exploring a new repo, understanding project structure, finding entry points, or getting oriented in code you haven't seen before.",
		allowedTools: []string{
			"get_repository_tree",
			"get_file_contents",
			"search_code",
			"list_commits",
			"list_branches",
			"list_tags",
		},
		body: "# Explore Repository\n\nUnderstand a new codebase systematically without reading every file.\n\n## Available Tools\n- `get_repository_tree` — full directory tree at any ref\n- `get_file_contents` — read files and directories\n- `search_code` — find patterns across the codebase\n- `list_commits` — recent commit history\n- `list_branches` / `list_tags` — branches and tags\n\n## Workflow\n1. `get_repository_tree` at root for structure overview.\n2. Read README.md, CONTRIBUTING.md, and build/config files.\n3. `list_commits` on main branch to find actively-changing areas.\n4. `search_code` for imports and entry points to understand architecture.\n\nStart with structure, then drill into active areas. Don't read every file.\n",
	}
}

func skillSearchCode() skillDefinition {
	return skillDefinition{
		name:        "search-code",
		description: "Find code patterns, symbols, and examples across GitHub. Use when searching for code, finding how something is implemented, locating files, or looking for usage examples across repositories.",
		allowedTools: []string{
			"search_code",
			"search_repositories",
			"get_file_contents",
		},
		body: "# Search Code\n\nFind specific code patterns across GitHub repositories.\n\n## Available Tools\n- `search_code` — search code with language:, org:, path: qualifiers\n- `search_repositories` — find repos by name, topic, language\n- `get_file_contents` — read full file context around matches\n\n## Query Tips\n- Use qualifiers in query: `language:go`, `org:github`, `path:src/`.\n- Do NOT put `sort:` in the query string — use the separate `sort` parameter.\n- After finding matches, read the full file with `get_file_contents` for context.\n",
	}
}

func skillTraceHistory() skillDefinition {
	return skillDefinition{
		name:        "trace-history",
		description: "Understand why code changed by tracing commits and PRs. Use when investigating git history, finding who changed something, understanding the motivation behind a change, or tracking down when a bug was introduced.",
		allowedTools: []string{
			"list_commits",
			"get_commit",
			"search_pull_requests",
			"pull_request_read",
		},
		body: "# Trace Code History\n\nUnderstand why code changed by following the commit to PR to discussion chain.\n\n## Available Tools\n- `list_commits` — commit history, filterable by path\n- `get_commit` — full commit details and diff\n- `search_pull_requests` — find PRs by commit SHA or keywords\n- `pull_request_read` — read PR description and review discussion\n\n## Workflow\n1. `list_commits` with path filter to find relevant commits.\n2. `get_commit` to see what changed.\n3. `search_pull_requests` to find the PR (search by commit SHA or title keywords).\n4. `pull_request_read` for the PR description and review comments — this has the *why*.\n\nCommit messages say *what*. PR descriptions say *why*. Review comments say *what was considered*.\n",
	}
}

func skillCreatePR() skillDefinition {
	return skillDefinition{
		name:        "create-pr",
		description: "Create a well-structured pull request that reviews smoothly. Use when opening a new PR, pushing changes for review, or submitting code changes to a repository.",
		allowedTools: []string{
			"create_pull_request",
			"get_file_contents",
			"create_branch",
			"push_files",
			"request_pull_request_reviewers",
			"list_pull_requests",
			"search_pull_requests",
		},
		body: "# Create Pull Request\n\nCreate a PR that communicates intent clearly and reviews smoothly.\n\n## Available Tools\n- `create_pull_request` — create the PR\n- `get_file_contents` — read PR templates from repo\n- `create_branch` — create a feature branch\n- `push_files` — push multiple files in one commit\n- `request_pull_request_reviewers` — request reviewers\n- `list_pull_requests` / `search_pull_requests` — check for existing PRs\n\n## Workflow\n1. Look for PR template in `.github/`, `docs/`, or root (`pull_request_template.md`).\n2. Check for existing PRs on the same branch with `list_pull_requests`.\n3. Create PR with template-structured description.\n4. Link issues using \"Closes #N\" or \"Fixes #N\" in the body.\n5. Request reviewers who know the affected code areas.\n\nNever create a PR without a description. Use the template if one exists.\n",
	}
}

func skillReviewPR() skillDefinition {
	return skillDefinition{
		name:        "review-pr",
		description: "Conduct a thorough code review of a pull request. Use when reviewing someone else's PR, checking code changes, leaving review comments, approving or requesting changes.",
		allowedTools: []string{
			"pull_request_read",
			"get_file_contents",
			"search_code",
			"pull_request_review_write",
			"create_pull_request_review",
			"add_pull_request_review_comment",
			"add_comment_to_pending_review",
			"submit_pending_pull_request_review",
			"delete_pending_pull_request_review",
			"add_reply_to_pull_request_comment",
			"resolve_review_thread",
			"unresolve_review_thread",
		},
		body: "# Review Pull Request\n\nYou are reviewing someone else's PR. Be thorough, constructive, and decisive.\n\n## Available Tools\n- `pull_request_read` — get diff, files, status, review comments, check runs\n- `get_file_contents` / `search_code` — read context beyond the diff\n- `create_pull_request_review` — start a pending review\n- `add_pull_request_review_comment` / `add_comment_to_pending_review` — add line comments\n- `submit_pending_pull_request_review` — submit with verdict\n- `delete_pending_pull_request_review` — discard pending review\n- `add_reply_to_pull_request_comment` — reply to existing comments\n- `resolve_review_thread` / `unresolve_review_thread` — manage threads\n\n## Workflow\n1. Read PR description and linked issues to understand intent.\n2. Check CI status with `pull_request_read` (method: get_status).\n3. Read the full diff with `pull_request_read` (method: get_diff).\n4. Create a pending review, add all comments, then submit once with a verdict.\n5. Always submit with approve, request_changes, or comment — don't leave orphan comments.\n\n## Anti-Patterns\n- Don't approve with failing CI.\n- Don't leave comments without submitting a review — pending reviews are invisible to the author.\n- Don't resolve threads you didn't start — that's the author's responsibility.\n- Read ALL changed files before commenting — your concern may be addressed elsewhere in the diff.\n",
	}
}

func skillSelfReviewPR() skillDefinition {
	return skillDefinition{
		name:        "self-review-pr",
		description: "Review your own PR before requesting team review. Use when you want to self-check your PR, verify CI status, polish description, or prepare your changes for review.",
		allowedTools: []string{
			"pull_request_read",
			"get_file_contents",
			"search_code",
			"actions_get",
			"get_job_logs",
			"update_pull_request",
			"update_pull_request_body",
			"update_pull_request_title",
			"request_pull_request_reviewers",
		},
		body: "# Self-Review PR\n\nReview your own PR before asking others. Catch what you can so reviewers focus on what matters.\n\n## Available Tools\n- `pull_request_read` — read your diff, CI status, and files\n- `get_file_contents` — check PR template compliance\n- `search_code` — verify changes match codebase patterns\n- `actions_get` / `get_job_logs` — investigate CI failures\n- `update_pull_request` / `update_pull_request_body` / `update_pull_request_title` — fix PR metadata\n- `request_pull_request_reviewers` — request reviewers when ready\n\n## Checklist\n1. Read your own diff — look for debug code, TODOs, unintended changes.\n2. Check CI passes — if failing, fix before requesting review.\n3. Verify description links relevant issues and follows the PR template.\n4. Verify title follows repo conventions (conventional commits, etc.).\n5. Request reviewers who own the affected code.\n\nDon't request review with failing CI. Reviewers notice when you haven't self-reviewed.\n",
	}
}

func skillAddressPRFeedback() skillDefinition {
	return skillDefinition{
		name:        "address-pr-feedback",
		description: "Handle review comments on your PR and push fixes. Use when you received PR feedback, need to respond to reviewer comments, resolve threads, or push fixes based on review.",
		allowedTools: []string{
			"pull_request_read",
			"add_reply_to_pull_request_comment",
			"resolve_review_thread",
			"push_files",
			"create_or_update_file",
			"update_pull_request_branch",
			"request_pull_request_reviewers",
		},
		body: "# Address PR Feedback\n\nYou received review feedback. Address it systematically, not piecemeal.\n\n## Available Tools\n- `pull_request_read` — read all review comments and threads\n- `add_reply_to_pull_request_comment` — respond to reviewer comments\n- `resolve_review_thread` — mark threads as resolved\n- `push_files` / `create_or_update_file` — push fixes\n- `update_pull_request_branch` — rebase/merge with base branch\n- `request_pull_request_reviewers` — re-request review after addressing\n\n## Workflow\n1. Read ALL comments before responding — comments may be related.\n2. Group related feedback and address together in one commit.\n3. Reply to each comment explaining what you changed (or why you disagree).\n4. Resolve threads only after addressing the concern — not before.\n5. Push fixes, then re-request review.\n\nDon't resolve threads without responding. Don't push fixes without explaining them in the thread.\n",
	}
}

func skillMergePR() skillDefinition {
	return skillDefinition{
		name:        "merge-pr",
		description: "Get a PR to merge-ready state and merge it. Use when merging a pull request, checking if a PR is ready to merge, updating a PR branch, or converting a draft PR.",
		allowedTools: []string{
			"pull_request_read",
			"merge_pull_request",
			"update_pull_request_branch",
			"update_pull_request_state",
			"update_pull_request_draft_state",
			"actions_get",
		},
		body: "# Merge Pull Request\n\nVerify a PR is ready and merge it.\n\n## Available Tools\n- `pull_request_read` — check status, reviews, and CI\n- `merge_pull_request` — merge the PR\n- `update_pull_request_branch` — update branch if behind base\n- `update_pull_request_draft_state` — convert draft to ready\n- `actions_get` — check workflow run details\n\n## Pre-Merge Checklist\n1. CI: all checks must pass (use `pull_request_read` with get_status).\n2. Reviews: required approvals present, no outstanding changes_requested.\n3. Branch: if behind base, call `update_pull_request_branch`.\n4. Draft: convert to ready with `update_pull_request_draft_state` if needed.\n5. Merge method: match repo conventions (merge, squash, or rebase).\n\nNever merge with failing checks. Never merge draft PRs without converting first.\n",
	}
}

func skillTriageIssues() skillDefinition {
	return skillDefinition{
		name:        "triage-issues",
		description: "Categorize, deduplicate, and prioritize incoming issues. Use when triaging issues, labeling bugs, organizing a backlog, closing duplicates, or processing new issue reports.",
		allowedTools: []string{
			"list_issues",
			"search_issues",
			"issue_read",
			"list_issue_types",
			"issue_write",
			"update_issue_labels",
			"update_issue_type",
			"update_issue_milestone",
			"update_issue_state",
			"update_issue_title",
			"update_issue_body",
			"update_issue_assignees",
			"add_issue_comment",
			"set_issue_fields",
			"list_labels",
			"get_label",
		},
		body: "# Triage Issues\n\nSystematically process incoming issues: categorize, deduplicate, and prioritize.\n\n## Available Tools\n- `list_issues` / `search_issues` / `issue_read` — find and read issues\n- `list_issue_types` — discover org issue types\n- `update_issue_labels` / `update_issue_type` / `update_issue_milestone` — categorize\n- `update_issue_state` — close duplicates or invalid issues\n- `add_issue_comment` — ask for info or note triage decisions\n- `list_labels` / `get_label` — check available labels\n\n## Workflow\n1. `list_issue_types` to understand the org's issue taxonomy.\n2. For each new issue:\n   a. `search_issues` for duplicates before doing anything else.\n   b. Apply labels for type (bug, feature, docs) and priority.\n   c. Set issue type if the org uses typed issues.\n   d. Assign to milestone if applicable.\n   e. Close duplicates with state_reason not_planned and link to the original.\n3. Comment on issues that need more info from the reporter.\n\nAlways set state_reason when closing: completed or not_planned. Never close without a reason.\n",
	}
}

func skillCreateIssue() skillDefinition {
	return skillDefinition{
		name:        "create-issue",
		description: "Create well-structured, searchable, actionable issues. Use when filing a bug report, requesting a feature, creating a task, or opening any new GitHub issue.",
		allowedTools: []string{
			"create_issue",
			"search_issues",
			"list_issue_types",
			"get_file_contents",
			"list_labels",
		},
		body: "# Create Issue\n\nCreate issues that are easy to find, understand, and act on.\n\n## Available Tools\n- `create_issue` — create the issue\n- `search_issues` — check for duplicates first\n- `list_issue_types` — discover available issue types\n- `get_file_contents` — read issue templates in .github/ISSUE_TEMPLATE/\n- `list_labels` — see available labels\n\n## Workflow\n1. Search for existing issues to avoid duplicates.\n2. Check .github/ISSUE_TEMPLATE/ for templates and use them.\n3. `list_issue_types` if the org supports typed issues.\n4. Create with appropriate type, labels, and milestone.\n\nWrite actionable titles: \"Fix X when Y\" not \"X is broken\". Include reproduction steps for bugs.\n",
	}
}

func skillManageSubIssues() skillDefinition {
	return skillDefinition{
		name:        "manage-sub-issues",
		description: "Break down large issues into trackable sub-tasks. Use when decomposing epics, creating task breakdowns, organizing work into smaller pieces, or managing parent-child issue relationships.",
		allowedTools: []string{
			"issue_read",
			"create_issue",
			"sub_issue_write",
			"add_sub_issue",
			"remove_sub_issue",
			"reprioritize_sub_issue",
			"search_issues",
		},
		body: "# Manage Sub-Issues\n\nBreak down epics and large issues into small, trackable sub-tasks.\n\n## Available Tools\n- `issue_read` — read parent issue details\n- `create_issue` — create sub-issue\n- `add_sub_issue` — link sub-issue to parent\n- `remove_sub_issue` — unlink a sub-issue\n- `reprioritize_sub_issue` — reorder sub-issues by priority\n- `search_issues` — find related issues\n\n## Workflow\n1. Read the parent issue to understand full scope.\n2. Break into small, independently completable pieces — each should map to one PR.\n3. `add_sub_issue` to link each to the parent.\n4. `reprioritize_sub_issue` to order by dependency (do X before Y).\n\nKeep parent issue description updated as the breakdown evolves.\n",
	}
}

func skillDebugCI() skillDefinition {
	return skillDefinition{
		name:        "debug-ci",
		description: "Investigate and fix failing GitHub Actions workflows. Use when CI is failing, a workflow run errored, you need to read build logs, or debug why tests aren't passing.",
		allowedTools: []string{
			"actions_get",
			"get_job_logs",
			"actions_list",
			"get_file_contents",
			"pull_request_read",
		},
		body: "# Debug CI Failure\n\nInvestigate failing GitHub Actions systematically.\n\n## Available Tools\n- `actions_get` — workflow run details, job list (use get_workflow_run, list_workflow_jobs)\n- `get_job_logs` — logs from a specific failed job\n- `actions_list` — list recent runs for comparison\n- `get_file_contents` — read workflow YAML definitions\n- `pull_request_read` — check PR-linked CI status\n\n## Workflow\n1. `actions_get` with get_workflow_run for the failed run.\n2. `actions_get` with list_workflow_jobs to find which jobs failed.\n3. `get_job_logs` for EACH failed job — don't stop at the first one.\n4. Read the workflow file in .github/workflows/ to understand the pipeline.\n5. Compare with recent passing runs via `actions_list` to spot what changed.\n\n## Anti-Patterns\n- Don't just rerun without reading logs — flaky tests need fixes, not retries.\n- Don't read only the first failure — later jobs may reveal the root cause.\n- Check if the failure is in workflow config vs application code.\n",
	}
}

func skillTriggerWorkflow() skillDefinition {
	return skillDefinition{
		name:        "trigger-workflow",
		description: "Run, rerun, or cancel GitHub Actions workflow runs. Use when triggering a deployment, rerunning failed jobs, canceling a stuck workflow, or dispatching a workflow manually.",
		allowedTools: []string{
			"actions_run_trigger",
			"actions_get",
			"actions_list",
			"get_job_logs",
		},
		body: "# Trigger Workflow\n\nRun, rerun, or cancel GitHub Actions workflows.\n\n## Available Tools\n- `actions_run_trigger` — run_workflow, rerun_workflow_run, rerun_failed_jobs, cancel_workflow_run\n- `actions_get` — list_workflows, get_workflow details\n- `actions_list` — list recent runs\n- `get_job_logs` — check results after run completes\n\n## Tips\n- Use rerun_failed_jobs instead of full rerun when only some jobs failed — faster.\n- Check workflow definition for required inputs before triggering with run_workflow.\n- Use cancel_workflow_run for stuck or unnecessary in-progress runs.\n",
	}
}

func skillSecurityAudit() skillDefinition {
	return skillDefinition{
		name:        "security-audit",
		description: "Systematically review code scanning, secret, and dependency alerts. Use when auditing repo security, checking for vulnerabilities, reviewing CodeQL alerts, or investigating exposed secrets.",
		allowedTools: []string{
			"list_code_scanning_alerts",
			"get_code_scanning_alert",
			"list_secret_scanning_alerts",
			"get_secret_scanning_alert",
			"list_dependabot_alerts",
			"get_dependabot_alert",
			"get_file_contents",
			"search_code",
		},
		body: "# Security Audit\n\nSystematically review all security alerts across a repository.\n\n## Available Tools\n- `list_code_scanning_alerts` / `get_code_scanning_alert` — static analysis findings\n- `list_secret_scanning_alerts` / `get_secret_scanning_alert` — exposed credentials\n- `list_dependabot_alerts` / `get_dependabot_alert` — vulnerable dependencies\n- `get_file_contents` / `search_code` — review code around alerts\n\n## Triage Order\n1. Secret scanning first — exposed credentials need immediate rotation.\n2. Code scanning — static analysis alerts, prioritize critical/high severity.\n3. Dependabot — vulnerable dependencies, prioritize by CVSS score.\n\nFor each alert: read full details, review the affected code, check if the same pattern exists elsewhere with `search_code`.\n\nDon't dismiss alerts without understanding them. Check if previously-dismissed alerts were properly triaged.\n",
	}
}

func skillFixDependabot() skillDefinition {
	return skillDefinition{
		name:        "fix-dependabot",
		description: "Handle vulnerable dependency alerts and update PRs. Use when fixing Dependabot alerts, updating vulnerable packages, reviewing dependency update PRs, or managing supply chain security.",
		allowedTools: []string{
			"list_dependabot_alerts",
			"get_dependabot_alert",
			"search_pull_requests",
			"list_pull_requests",
			"get_file_contents",
		},
		body: "# Fix Dependabot Alerts\n\nHandle vulnerable dependency alerts systematically.\n\n## Available Tools\n- `list_dependabot_alerts` / `get_dependabot_alert` — list and inspect alerts\n- `search_pull_requests` / `list_pull_requests` — find existing Dependabot PRs\n- `get_file_contents` — read dependency files\n\n## Workflow\n1. List alerts sorted by severity — fix critical/high first.\n2. Check if Dependabot already opened a PR for each alert.\n3. For alerts with PRs: review the PR and merge if CI passes.\n4. For alerts without PRs: check if the fix requires a major version bump.\n5. Group related dependency updates into logical batches.\n\nCheck the alert's fixed_in version to understand the required update scope before acting.\n",
	}
}

func skillResearchVulnerability() skillDefinition {
	return skillDefinition{
		name:        "research-vulnerability",
		description: "Query the GitHub Advisory Database for security advisories. Use when researching CVEs, looking up GHSA IDs, checking if a package has known vulnerabilities, or reviewing security advisories for a repo or org.",
		allowedTools: []string{
			"list_global_security_advisories",
			"get_global_security_advisory",
			"list_repository_security_advisories",
			"list_org_repository_security_advisories",
		},
		body: "# Research Vulnerability\n\nQuery the GitHub Advisory Database for known vulnerabilities.\n\n## Available Tools\n- `list_global_security_advisories` — search the GitHub Advisory Database\n- `get_global_security_advisory` — get advisory details by GHSA ID\n- `list_repository_security_advisories` — advisories for a specific repo\n- `list_org_repository_security_advisories` — advisories across an org\n\nUse GHSA IDs (e.g., GHSA-xxxx-xxxx-xxxx) for specific lookups. Filter by ecosystem (npm, pip, go) and severity.\n",
	}
}

func skillManageProject() skillDefinition {
	return skillDefinition{
		name:        "manage-project",
		description: "Track and update work items in GitHub Projects (v2). Use when managing a project board, updating issue status fields, adding items to a project, querying project items, or posting project status updates.",
		allowedTools: []string{
			"projects_list",
			"projects_get",
			"projects_write",
			"search_issues",
			"search_pull_requests",
		},
		body: "# Manage Project Board\n\nTrack and update work items in GitHub Projects (v2).\n\n## Available Tools\n- `projects_list` — find projects for a user, org, or repo\n- `projects_get` — get project details, fields, items, status updates\n- `projects_write` — update project items, fields, and status\n- `search_issues` / `search_pull_requests` — find items to add\n\n## Workflow\n1. `projects_list` to find the project.\n2. `projects_get` with list_project_fields to understand field names, IDs, and types.\n3. `projects_get` with list_project_items to browse current items.\n4. `projects_write` to update fields, add items, or post status updates.\n\n## Critical Rules\n- Always call list_project_fields first — use EXACT field names (case-insensitive). Never guess field IDs.\n- Paginate: loop while pageInfo.hasNextPage=true using after=pageInfo.nextCursor.\n- Keep query, fields, and per_page identical across pages.\n\n## Query Syntax for list_project_items\n- AND: space-separated (label:bug priority:high)\n- OR: comma inside qualifier (label:bug,critical)\n- NOT: leading dash (-label:wontfix)\n- State: state:open, state:closed, state:merged\n- Type: is:issue, is:pr\n- Assignment: assignee:@me\n",
	}
}

func skillHandleNotifications() skillDefinition {
	return skillDefinition{
		name:        "handle-notifications",
		description: "Process your GitHub notification queue efficiently. Use when checking notifications, clearing your inbox, managing subscriptions, or finding out what needs your attention on GitHub.",
		allowedTools: []string{
			"list_notifications",
			"get_notification_details",
			"dismiss_notification",
			"mark_all_notifications_read",
			"manage_notification_subscription",
			"manage_repository_notification_subscription",
		},
		body: "# Handle Notifications\n\nProcess notifications by priority, not just mark them read.\n\n## Available Tools\n- `list_notifications` — list by unread, repo, or reason\n- `get_notification_details` — full context for a notification\n- `dismiss_notification` — mark as done\n- `mark_all_notifications_read` — mark all read\n- `manage_notification_subscription` — subscribe/unsubscribe from threads\n- `manage_repository_notification_subscription` — per-repo notification settings\n\n## Triage by Reason\n1. review_requested — someone needs your review (act first).\n2. mention / assign — you are directly involved (act next).\n3. ci_activity — check if your CI is failing.\n4. subscribed — threads you are watching (lowest priority).\n\nUse `get_notification_details` before acting — don't dismiss blindly.\nUnsubscribe from noisy repos with `manage_repository_notification_subscription`.\n\nDon't use `mark_all_notifications_read` without triaging — you will miss action items.\n",
	}
}

func skillPrepareRelease() skillDefinition {
	return skillDefinition{
		name:        "prepare-release",
		description: "Compile release notes from commits and merged PRs. Use when preparing a release, writing a changelog, summarizing changes since last version, or reviewing what shipped.",
		allowedTools: []string{
			"list_releases",
			"get_latest_release",
			"get_release_by_tag",
			"list_tags",
			"get_tag",
			"list_commits",
			"search_pull_requests",
		},
		body: "# Prepare Release\n\nCompile release notes from merged PRs and commits since the last release.\n\n## Available Tools\n- `list_releases` / `get_latest_release` / `get_release_by_tag` — browse releases\n- `list_tags` / `get_tag` — version tags\n- `list_commits` — commits since last release\n- `search_pull_requests` — find merged PRs in the range\n\n## Workflow\n1. `get_latest_release` to find the last version tag.\n2. `list_commits` since that tag to see all changes.\n3. `search_pull_requests` for merged PRs in the range — PR descriptions are richer than commits.\n4. Group changes: breaking changes, features, bug fixes, docs.\n5. Link PR numbers in release notes for traceability.\n\nUse PR titles and labels for categorization — commit messages alone are often too terse.\n",
	}
}

func skillManageRepo() skillDefinition {
	return skillDefinition{
		name:        "manage-repo",
		description: "Create repos, manage branches, and push file changes. Use when creating a new repository, making a branch, committing files via the API, forking a repo, or managing repository contents.",
		allowedTools: []string{
			"create_repository",
			"fork_repository",
			"create_branch",
			"create_or_update_file",
			"push_files",
			"delete_file",
			"get_file_contents",
			"search_repositories",
		},
		body: "# Manage Repository\n\nCreate repos, branches, and manage file contents.\n\n## Available Tools\n- `create_repository` — create a new repo\n- `fork_repository` — fork an existing repo\n- `create_branch` — create a branch\n- `create_or_update_file` — single file create/update with commit\n- `push_files` — push multiple files in one commit\n- `delete_file` — delete a file with commit\n- `get_file_contents` — read files and directories\n- `search_repositories` — find existing repos\n\n## Tips\n- Use `push_files` for multi-file changes — creates a single atomic commit.\n- Use `create_or_update_file` only for single-file operations.\n- Include README, LICENSE, and .gitignore when creating new repos.\n- Fork for contributing to others' projects. Create new repos for new projects.\n",
	}
}

func skillManageLabels() skillDefinition {
	return skillDefinition{
		name:        "manage-labels",
		description: "Set up and maintain a consistent label scheme. Use when creating labels, organizing a label system, cleaning up labels, or standardizing label naming across a repository.",
		allowedTools: []string{
			"list_labels",
			"list_label",
			"label_write",
			"search_issues",
		},
		body: "# Manage Labels\n\nCreate a consistent, useful label system for a repository.\n\n## Available Tools\n- `list_labels` / `list_label` — browse existing labels\n- `label_write` — create, update, or delete labels\n- `search_issues` — check label usage before deleting\n\n## Best Practices\n- Use prefixed names: type:bug, type:feature, priority:high, status:needs-triage.\n- Use consistent colors within categories (all type: labels same color family).\n- Write helpful descriptions — they appear in the label picker.\n- Check label usage with `search_issues` before deleting or renaming.\n- Aim for 15-25 labels total. Too many means none get used consistently.\n",
	}
}

func skillContributeOSS() skillDefinition {
	return skillDefinition{
		name:        "contribute-oss",
		description: "Fork, branch, and submit PRs to external repositories. Use when contributing to open source, forking a repo to make changes, or submitting a pull request to a project you don't own.",
		allowedTools: []string{
			"fork_repository",
			"create_branch",
			"push_files",
			"create_pull_request",
			"get_file_contents",
			"search_repositories",
			"pull_request_read",
		},
		body: "# Contribute to Open Source\n\nWorkflow for contributing to repos you don't have write access to.\n\n## Available Tools\n- `fork_repository` — fork upstream to your account\n- `create_branch` — create feature branch on your fork\n- `push_files` — push changes to your fork\n- `create_pull_request` — PR from your fork to upstream\n- `get_file_contents` — read CONTRIBUTING.md and templates\n- `search_repositories` — find the repo\n- `pull_request_read` — track your PR status\n\n## Workflow\n1. Read CONTRIBUTING.md and CODE_OF_CONDUCT.md first.\n2. Fork the repo, create a feature branch (not main).\n3. Keep changes small and focused — one concern per PR.\n4. Follow the project's existing code style.\n5. Create PR with clear description linking related issues.\n\nLook for good-first-issue labels to find starter tasks. Don't submit large PRs without discussing scope first in an issue.\n",
	}
}

func skillBrowseDiscussions() skillDefinition {
	return skillDefinition{
		name:        "browse-discussions",
		description: "Read and explore GitHub Discussions and categories. Use when browsing discussions, reading community conversations, checking discussion categories, or looking for answers in a project's discussions.",
		allowedTools: []string{
			"list_discussions",
			"get_discussion",
			"get_discussion_comments",
			"list_discussion_categories",
		},
		body: "# Browse Discussions\n\nRead and explore GitHub Discussions.\n\n## Available Tools\n- `list_discussions` — list discussions in a repo\n- `get_discussion` — get discussion details\n- `get_discussion_comments` — read comments and replies\n- `list_discussion_categories` — list available categories\n\nCall `list_discussion_categories` first to understand the discussion structure. Filter by category to find relevant conversations.\n",
	}
}

func skillDelegateCopilot() skillDefinition {
	return skillDefinition{
		name:        "delegate-to-copilot",
		description: "Assign Copilot to issues and request Copilot PR reviews. Use when you want Copilot to work on an issue, get an automated code review, or delegate tasks to GitHub Copilot.",
		allowedTools: []string{
			"assign_copilot_to_issue",
			"request_copilot_review",
			"issue_read",
			"pull_request_read",
		},
		body: "# Delegate to Copilot\n\nUse GitHub Copilot for automated issue work and PR reviews.\n\n## Available Tools\n- `assign_copilot_to_issue` — assign Copilot to work on an issue\n- `request_copilot_review` — request Copilot review on a PR\n- `issue_read` — check issue details before assigning\n- `pull_request_read` — check PR before requesting review\n\n## Tips\n- Write clear, specific issue descriptions — vague issues produce vague results.\n- Ensure the issue is well-scoped (single concern) before assigning Copilot.\n- Use Copilot review for initial feedback, then follow up with human review for nuanced concerns.\n",
	}
}

func skillDiscoverGitHub() skillDefinition {
	return skillDefinition{
		name:        "discover-github",
		description: "Search for users, organizations, and repositories. Use when finding GitHub users, looking up organizations, discovering repos by topic or language, or managing your starred repositories.",
		allowedTools: []string{
			"search_users",
			"search_orgs",
			"search_repositories",
			"list_starred_repositories",
			"star_repository",
			"unstar_repository",
		},
		body: "# Discover GitHub\n\nSearch for users, organizations, and repositories across GitHub.\n\n## Available Tools\n- `search_users` — find users by name, location, or profile\n- `search_orgs` — find organizations\n- `search_repositories` — find repos by name, topic, language, org\n- `list_starred_repositories` — your starred repos\n- `star_repository` / `unstar_repository` — manage stars\n\n## Search Tips\n- Use qualifiers: language:go, org:github, topic:mcp, stars:>100.\n- Use separate `sort` and `order` parameters — don't put sort: in query strings.\n- Star useful repos to build a personal reference library.\n",
	}
}

func skillShareSnippet() skillDefinition {
	return skillDefinition{
		name:        "share-snippet",
		description: "Create and manage code snippets via GitHub Gists. Use when sharing a code snippet, creating a quick paste, saving notes as a gist, or managing your existing gists.",
		allowedTools: []string{
			"create_gist",
			"update_gist",
			"list_gists",
			"get_gist",
		},
		body: "# Share Snippet\n\nCreate and manage code snippets via GitHub Gists.\n\n## Available Tools\n- `create_gist` — create a new gist (public or private)\n- `update_gist` — update files or description\n- `list_gists` — list your gists\n- `get_gist` — retrieve a specific gist\n\nGists support multiple files per gist. Use descriptive filenames with proper extensions for syntax highlighting.\n",
	}
}

// buildSkillContent builds the full SKILL.md content with YAML frontmatter.
func buildSkillContent(skill skillDefinition) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", skill.name)
	fmt.Fprintf(&b, "description: %s\n", skill.description)
	b.WriteString("allowed-tools:\n")
	for _, tool := range skill.allowedTools {
		fmt.Fprintf(&b, "  - %s\n", tool)
	}
	b.WriteString("---\n\n")
	b.WriteString(skill.body)
	return b.String()
}

// RegisterSkillResources registers all skill resources with the MCP server.
// Each skill is a static resource with a skill:// URI that can be discovered
// by MCP clients supporting the skills pattern.
func RegisterSkillResources(s *mcp.Server) {
	// Publish the discovery index first so SEP-2640 clients can enumerate skills (and the tools
	// each governs) from a single resource read, without fetching every SKILL.md.
	s.AddResource(
		&mcp.Resource{
			URI:         skillIndexURI,
			Name:        "skill-index",
			Title:       "Skill index",
			Description: "SEP-2640 skill discovery index enumerating available skills and their allowed tools.",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			body, err := buildSkillIndexJSON()
			if err != nil {
				return nil, err
			}
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      skillIndexURI,
						MIMEType: "application/json",
						Text:     body,
					},
				},
			}, nil
		},
	)

	for _, skill := range allSkills() {
		content := buildSkillContent(skill)
		uri := fmt.Sprintf("skill://github/%s/SKILL.md", skill.name)

		s.AddResource(
			&mcp.Resource{
				URI:         uri,
				Name:        fmt.Sprintf("%s/SKILL.md", skill.name),
				Description: skill.description,
				MIMEType:    "text/markdown",
			},
			func(skillContent string, skillURI string) mcp.ResourceHandler {
				return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
					return &mcp.ReadResourceResult{
						Contents: []*mcp.ResourceContents{
							{
								URI:      skillURI,
								MIMEType: "text/markdown",
								Text:     skillContent,
							},
						},
					}, nil
				}
			}(content, uri),
		)
	}
}
