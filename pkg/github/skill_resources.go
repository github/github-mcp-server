package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
// Each skill covers a domain of tools and provides guidance for using them.
func allSkills() []skillDefinition {
	return []skillDefinition{
		skillContext(),
		skillRepos(),
		skillIssues(),
		skillPullRequests(),
		skillCodeSecurity(),
		skillActions(),
		skillDiscussions(),
		skillProjects(),
		skillNotifications(),
		skillGists(),
		skillUsersOrgs(),
		skillSecurityAdvisories(),
		skillLabels(),
		skillGit(),
		skillStargazers(),
		skillCopilot(),
	}
}

func skillContext() skillDefinition {
	return skillDefinition{
		name:        "context",
		description: "Understand the current user and their GitHub context",
		allowedTools: []string{
			"get_me",
			"get_teams",
			"get_team_members",
		},
		body: `# GitHub Context

Always call ` + "`get_me`" + ` first to understand the current user's permissions and context.

## Available Tools
- ` + "`get_me`" + ` — get the authenticated user's profile and permissions
- ` + "`get_teams`" + ` — list teams the user belongs to
- ` + "`get_team_members`" + ` — list members of a specific team
`,
	}
}

func skillRepos() skillDefinition {
	return skillDefinition{
		name:        "repos",
		description: "Manage GitHub repositories, branches, files, releases, and code search",
		allowedTools: []string{
			"search_repositories",
			"get_file_contents",
			"list_commits",
			"search_code",
			"get_commit",
			"list_branches",
			"list_tags",
			"get_tag",
			"list_releases",
			"get_latest_release",
			"get_release_by_tag",
			"create_or_update_file",
			"create_repository",
			"fork_repository",
			"create_branch",
			"push_files",
			"delete_file",
		},
		body: `# GitHub Repositories

Tools for managing GitHub repositories, browsing code, and working with branches, tags, and releases.

## Available Tools
- ` + "`search_repositories`" + ` — find repos by name, topic, language, org
- ` + "`get_file_contents`" + ` — read files or directories from a repo
- ` + "`list_commits`" + ` — get commit history for a branch or path
- ` + "`search_code`" + ` — search for code patterns across repos
- ` + "`get_commit`" + ` — get details of a specific commit
- ` + "`list_branches`" + ` / ` + "`list_tags`" + ` — list branches or tags
- ` + "`get_tag`" + ` — get details of a specific tag
- ` + "`list_releases`" + ` / ` + "`get_latest_release`" + ` / ` + "`get_release_by_tag`" + ` — browse releases
- ` + "`create_or_update_file`" + ` / ` + "`push_files`" + ` / ` + "`delete_file`" + ` — modify files
- ` + "`create_repository`" + ` / ` + "`fork_repository`" + ` / ` + "`create_branch`" + ` — create repos or branches

## Sorting
For search tools, use separate ` + "`sort`" + ` and ` + "`order`" + ` parameters — do not include 'sort:' syntax in query strings. Query strings should contain only search criteria (e.g., 'org:google language:python').
`,
	}
}

func skillIssues() skillDefinition {
	return skillDefinition{
		name:        "issues",
		description: "Create, read, update, and search GitHub issues with sub-issues and types",
		allowedTools: []string{
			"issue_read",
			"search_issues",
			"list_issues",
			"list_issue_types",
			"issue_write",
			"add_issue_comment",
			"sub_issue_write",
			"get_label",
			// Granular tools (feature-flagged alternatives to issue_write/sub_issue_write)
			"create_issue",
			"update_issue_title",
			"update_issue_body",
			"update_issue_assignees",
			"update_issue_labels",
			"update_issue_milestone",
			"update_issue_type",
			"update_issue_state",
			"add_sub_issue",
			"remove_sub_issue",
			"reprioritize_sub_issue",
			"set_issue_fields",
		},
		body: `# GitHub Issues

Tools for creating, reading, updating, and searching GitHub issues.

## Available Tools
- ` + "`issue_read`" + ` — get details of a specific issue
- ` + "`search_issues`" + ` — search for issues with specific criteria or keywords
- ` + "`list_issues`" + ` — list issues with basic filtering and pagination
- ` + "`list_issue_types`" + ` — list available issue types for an organization
- ` + "`issue_write`" + ` — create or update an issue (composite tool)
- ` + "`add_issue_comment`" + ` — add a comment to an issue
- ` + "`sub_issue_write`" + ` — manage sub-issues (composite tool)
- ` + "`get_label`" + ` — get details of a specific label
- ` + "`create_issue`" + ` — create a new issue
- ` + "`update_issue_title`" + ` — update an issue's title
- ` + "`update_issue_body`" + ` — update an issue's body
- ` + "`update_issue_assignees`" + ` — update an issue's assignees
- ` + "`update_issue_labels`" + ` — update an issue's labels
- ` + "`update_issue_milestone`" + ` — update an issue's milestone
- ` + "`update_issue_type`" + ` — update an issue's type
- ` + "`update_issue_state`" + ` — update an issue's state (open/closed)
- ` + "`add_sub_issue`" + ` — add a sub-issue to a parent issue
- ` + "`remove_sub_issue`" + ` — remove a sub-issue from a parent
- ` + "`reprioritize_sub_issue`" + ` — change the priority of a sub-issue
- ` + "`set_issue_fields`" + ` — set custom project fields on an issue

## Workflow
1. Call ` + "`list_issue_types`" + ` first for organizations to discover proper issue types.
2. Call ` + "`search_issues`" + ` before creating new issues to avoid duplicates.
3. Always set ` + "`state_reason`" + ` when closing issues.
`,
	}
}

func skillPullRequests() skillDefinition {
	return skillDefinition{
		name:        "pull-requests",
		description: "Create, review, merge, and manage GitHub pull requests",
		allowedTools: []string{
			"pull_request_read",
			"list_pull_requests",
			"search_pull_requests",
			"merge_pull_request",
			"update_pull_request_branch",
			"create_pull_request",
			"update_pull_request",
			"pull_request_review_write",
			"add_comment_to_pending_review",
			"add_reply_to_pull_request_comment",
			// Granular tools (feature-flagged alternatives to update_pull_request/pull_request_review_write)
			"update_pull_request_title",
			"update_pull_request_body",
			"update_pull_request_state",
			"update_pull_request_draft_state",
			"request_pull_request_reviewers",
			"create_pull_request_review",
			"submit_pending_pull_request_review",
			"delete_pending_pull_request_review",
			"add_pull_request_review_comment",
			"resolve_review_thread",
			"unresolve_review_thread",
		},
		body: `# GitHub Pull Requests

Tools for creating, reviewing, merging, and managing GitHub pull requests.

## Available Tools
- ` + "`pull_request_read`" + ` — get details of a specific pull request
- ` + "`list_pull_requests`" + ` — list pull requests with basic filtering and pagination
- ` + "`search_pull_requests`" + ` — search for pull requests with specific criteria
- ` + "`merge_pull_request`" + ` — merge a pull request
- ` + "`update_pull_request_branch`" + ` — update a PR branch with the base branch
- ` + "`create_pull_request`" + ` — create a new pull request
- ` + "`update_pull_request`" + ` — update a pull request (composite tool)
- ` + "`pull_request_review_write`" + ` — manage PR reviews (composite tool)
- ` + "`add_comment_to_pending_review`" + ` — add a line comment to a pending review
- ` + "`add_reply_to_pull_request_comment`" + ` — reply to a PR review comment
- ` + "`update_pull_request_title`" + ` — update a PR's title
- ` + "`update_pull_request_body`" + ` — update a PR's body
- ` + "`update_pull_request_state`" + ` — update a PR's state (open/closed)
- ` + "`update_pull_request_draft_state`" + ` — convert between draft and ready
- ` + "`request_pull_request_reviewers`" + ` — request reviewers for a PR
- ` + "`create_pull_request_review`" + ` — create a new PR review
- ` + "`submit_pending_pull_request_review`" + ` — submit a pending review
- ` + "`delete_pending_pull_request_review`" + ` — delete a pending review
- ` + "`add_pull_request_review_comment`" + ` — add a review comment to a PR
- ` + "`resolve_review_thread`" + ` — resolve a review thread
- ` + "`unresolve_review_thread`" + ` — unresolve a review thread

## PR Review Workflow
For complex reviews with line-specific comments:
1. Call ` + "`create_pull_request_review`" + ` or ` + "`pull_request_review_write`" + ` with method 'create' to create a pending review.
2. Call ` + "`add_comment_to_pending_review`" + ` to add line comments.
3. Call ` + "`submit_pending_pull_request_review`" + ` or ` + "`pull_request_review_write`" + ` with method 'submit_pending' to submit.

## Creating Pull Requests
Before creating a PR, search for pull request templates in the repository. Template files are called pull_request_template.md or located in the '.github/PULL_REQUEST_TEMPLATE' directory. Use the template content to structure the PR description.
`,
	}
}

func skillCodeSecurity() skillDefinition {
	return skillDefinition{
		name:        "code-security",
		description: "View code scanning alerts, secret scanning alerts, and Dependabot alerts",
		allowedTools: []string{
			"get_code_scanning_alert",
			"list_code_scanning_alerts",
			"get_secret_scanning_alert",
			"list_secret_scanning_alerts",
			"get_dependabot_alert",
			"list_dependabot_alerts",
		},
		body: `# Code Security

Tools for viewing security alerts across GitHub repositories.

## Available Tools
- ` + "`get_code_scanning_alert`" + ` — get details of a specific code scanning alert
- ` + "`list_code_scanning_alerts`" + ` — list code scanning alerts for a repo
- ` + "`get_secret_scanning_alert`" + ` — get details of a specific secret scanning alert
- ` + "`list_secret_scanning_alerts`" + ` — list secret scanning alerts for a repo
- ` + "`get_dependabot_alert`" + ` — get details of a specific Dependabot alert
- ` + "`list_dependabot_alerts`" + ` — list Dependabot alerts for a repo

Use ` + "`list_*`" + ` tools to get an overview of alerts, then ` + "`get_*`" + ` to inspect specific alerts in detail.
`,
	}
}

func skillActions() skillDefinition {
	return skillDefinition{
		name:        "actions",
		description: "View and trigger GitHub Actions workflows, runs, and job logs",
		allowedTools: []string{
			"actions_list",
			"actions_get",
			"actions_run_trigger",
			"get_job_logs",
		},
		body: `# GitHub Actions

Tools for interacting with GitHub Actions workflows and CI/CD operations.

## Available Tools
- ` + "`actions_list`" + ` — list workflow runs for a repository
- ` + "`actions_get`" + ` — get details of a specific workflow run
- ` + "`actions_run_trigger`" + ` — trigger a workflow run
- ` + "`get_job_logs`" + ` — retrieve logs from a specific job
`,
	}
}

func skillDiscussions() skillDefinition {
	return skillDefinition{
		name:        "discussions",
		description: "Browse and read GitHub Discussions and their categories",
		allowedTools: []string{
			"list_discussions",
			"get_discussion",
			"get_discussion_comments",
			"list_discussion_categories",
		},
		body: `# GitHub Discussions

Tools for browsing and reading GitHub Discussions.

## Available Tools
- ` + "`list_discussions`" + ` — list discussions in a repository
- ` + "`get_discussion`" + ` — get details of a specific discussion
- ` + "`get_discussion_comments`" + ` — get comments on a discussion
- ` + "`list_discussion_categories`" + ` — list available discussion categories

Call ` + "`list_discussion_categories`" + ` to understand available categories before filtering discussions.
`,
	}
}

func skillProjects() skillDefinition {
	return skillDefinition{
		name:        "projects",
		description: "Manage GitHub Projects (v2) — list items, update fields, and track status",
		allowedTools: []string{
			"projects_list",
			"projects_get",
			"projects_write",
		},
		body: `# GitHub Projects

Tools for managing GitHub Projects (v2).

## Available Tools
- ` + "`projects_list`" + ` — list projects for a user, org, or repo
- ` + "`projects_get`" + ` — get project details, fields, items, or status updates
- ` + "`projects_write`" + ` — create/update/delete project items, fields, or status updates

## Workflow
1. Call ` + "`projects_list`" + ` to find projects.
2. Call ` + "`projects_get`" + ` with list_project_fields to understand available fields and get IDs/types.
3. Call ` + "`projects_get`" + ` with list_project_items (with pagination) to browse items.
4. Call ` + "`projects_write`" + ` for updates.

## Status Updates
Use list_project_status_updates to read recent project status updates (newest first). Use get_project_status_update with a node ID to get a single update. Use create_project_status_update to create a new status update.

## Field Usage
- Call list_project_fields first to understand available fields and get IDs/types before filtering.
- Use EXACT returned field names (case-insensitive match). Don't invent names or IDs.
- Only include filters for fields that exist and are relevant.

## Pagination
- Loop while pageInfo.hasNextPage=true using after=pageInfo.nextCursor.
- Keep query, fields, per_page IDENTICAL on every page.

## Query Syntax for list_project_items
- AND: space-separated (label:bug priority:high)
- OR: comma inside one qualifier (label:bug,critical)
- NOT: leading '-' (-label:wontfix)
- Ranges: points:1..3, updated:<@today-30d
- Wildcards: title:*crash*, label:bug*
- Type filters: is:issue, is:pr
- State: state:open, state:closed, state:merged
- Assignment: assignee:@me, assignee:username
`,
	}
}

func skillNotifications() skillDefinition {
	return skillDefinition{
		name:        "notifications",
		description: "View and manage GitHub notifications and subscriptions",
		allowedTools: []string{
			"list_notifications",
			"get_notification_details",
			"dismiss_notification",
			"mark_all_notifications_read",
			"manage_notification_subscription",
			"manage_repository_notification_subscription",
		},
		body: `# GitHub Notifications

Tools for viewing and managing GitHub notifications.

## Available Tools
- ` + "`list_notifications`" + ` — list unread and read notifications
- ` + "`get_notification_details`" + ` — get details of a specific notification
- ` + "`dismiss_notification`" + ` — mark a notification as done
- ` + "`mark_all_notifications_read`" + ` — mark all notifications as read
- ` + "`manage_notification_subscription`" + ` — manage thread subscription settings
- ` + "`manage_repository_notification_subscription`" + ` — manage repository notification settings
`,
	}
}

func skillGists() skillDefinition {
	return skillDefinition{
		name:        "gists",
		description: "Create, read, and update GitHub Gists",
		allowedTools: []string{
			"list_gists",
			"get_gist",
			"create_gist",
			"update_gist",
		},
		body: `# GitHub Gists

Tools for managing GitHub Gists — lightweight code snippets and file sharing.

## Available Tools
- ` + "`list_gists`" + ` — list gists for the authenticated user
- ` + "`get_gist`" + ` — retrieve a specific gist by ID
- ` + "`create_gist`" + ` — create a new gist (public or private)
- ` + "`update_gist`" + ` — update an existing gist's files or description
`,
	}
}

func skillUsersOrgs() skillDefinition {
	return skillDefinition{
		name:        "users-orgs",
		description: "Search for GitHub users and organizations",
		allowedTools: []string{
			"search_users",
			"search_orgs",
		},
		body: `# GitHub Users & Organizations

Tools for searching GitHub users and organizations.

## Available Tools
- ` + "`search_users`" + ` — search for users by username, name, location, or other criteria
- ` + "`search_orgs`" + ` — search for organizations by name or other criteria

For search tools, use separate ` + "`sort`" + ` and ` + "`order`" + ` parameters — do not include 'sort:' syntax in query strings.
`,
	}
}

func skillSecurityAdvisories() skillDefinition {
	return skillDefinition{
		name:        "security-advisories",
		description: "Browse global and repository-level security advisories",
		allowedTools: []string{
			"list_global_security_advisories",
			"get_global_security_advisory",
			"list_repository_security_advisories",
			"list_org_repository_security_advisories",
		},
		body: `# Security Advisories

Tools for browsing security advisories on GitHub.

## Available Tools
- ` + "`list_global_security_advisories`" + ` — search the GitHub Advisory Database
- ` + "`get_global_security_advisory`" + ` — get details of a specific global advisory
- ` + "`list_repository_security_advisories`" + ` — list advisories for a specific repository
- ` + "`list_org_repository_security_advisories`" + ` — list advisories across an organization's repositories
`,
	}
}

func skillLabels() skillDefinition {
	return skillDefinition{
		name:        "labels",
		description: "Manage GitHub issue and PR labels",
		allowedTools: []string{
			"list_label",
			"list_labels",
			"label_write",
		},
		body: `# GitHub Labels

Tools for managing labels on GitHub repositories.

## Available Tools
- ` + "`list_label`" + ` — get a specific label by name
- ` + "`list_labels`" + ` — list all labels for a repository
- ` + "`label_write`" + ` — create, update, or delete labels
`,
	}
}

func skillGit() skillDefinition {
	return skillDefinition{
		name:        "git",
		description: "Low-level Git operations via the GitHub Git API",
		allowedTools: []string{
			"get_repository_tree",
		},
		body: `# GitHub Git API

Low-level Git operations via the GitHub API.

## Available Tools
- ` + "`get_repository_tree`" + ` — retrieve the tree structure of a repository at a given ref, useful for understanding repository layout
`,
	}
}

func skillStargazers() skillDefinition {
	return skillDefinition{
		name:        "stargazers",
		description: "Star and unstar repositories, list starred repositories",
		allowedTools: []string{
			"list_starred_repositories",
			"star_repository",
			"unstar_repository",
		},
		body: `# GitHub Stars

Tools for managing repository stars.

## Available Tools
- ` + "`list_starred_repositories`" + ` — list repositories starred by the authenticated user
- ` + "`star_repository`" + ` — star a repository
- ` + "`unstar_repository`" + ` — unstar a repository
`,
	}
}

func skillCopilot() skillDefinition {
	return skillDefinition{
		name:        "copilot",
		description: "Assign Copilot to issues and request Copilot reviews on pull requests",
		allowedTools: []string{
			"assign_copilot_to_issue",
			"request_copilot_review",
		},
		body: `# GitHub Copilot

Tools for using GitHub Copilot in your workflow.

## Available Tools
- ` + "`assign_copilot_to_issue`" + ` — assign Copilot as a collaborator on an issue
- ` + "`request_copilot_review`" + ` — request a Copilot review on a pull request
`,
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
