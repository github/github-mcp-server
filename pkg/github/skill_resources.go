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

Always call **get_me** first to understand the current user's permissions and context.

- **get_me** — returns the authenticated user's profile and permissions
- **get_teams** — lists teams the user belongs to
- **get_team_members** — lists members of a specific team
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

## Tool Selection
- Use **search_repositories** for finding repos by criteria and **search_code** for finding code containing specific patterns.
- Use **list_*** tools for broad retrieval with pagination (e.g., all branches, all commits).

## Context Management
- Use pagination with batches of 5-10 items.
- Set **minimal_output** to true when full details are not needed.

## Sorting
For search tools, use separate **sort** and **order** parameters — do not include 'sort:' syntax in query strings. Query strings should contain only search criteria (e.g., 'org:google language:python').
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

## Workflow
1. Check **list_issue_types** first for organizations to discover proper issue types.
2. Use **search_issues** before creating new issues to avoid duplicates.
3. Always set **state_reason** when closing issues.

## Tool Selection
- Use **search_issues** for targeted queries with specific criteria or keywords.
- Use **list_issues** for broad retrieval of all issues with basic filtering and pagination.
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

## PR Review Workflow
For complex reviews with line-specific comments:
1. Use **pull_request_review_write** with method 'create' to create a pending review.
2. Use **add_comment_to_pending_review** to add line comments.
3. Use **pull_request_review_write** with method 'submit_pending' to submit the review.

## Creating Pull Requests
Before creating a PR, search for pull request templates in the repository. Template files are called pull_request_template.md or located in the '.github/PULL_REQUEST_TEMPLATE' directory. Use the template content to structure the PR description.

## Tool Selection
- Use **search_pull_requests** for targeted queries with specific criteria.
- Use **list_pull_requests** for broad retrieval with basic filtering and pagination.
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

## Alert Types
- **Code Scanning** — static analysis alerts (CodeQL, third-party tools)
- **Secret Scanning** — detected secrets and credentials in code
- **Dependabot** — vulnerable dependency alerts

Use **list_*** tools to get an overview of alerts, then **get_*** to inspect specific alerts in detail.
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

- **actions_list** — list workflow runs for a repository
- **actions_get** — get details of a specific workflow run
- **actions_run_trigger** — trigger a workflow run
- **get_job_logs** — retrieve logs from a specific job
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

Use **list_discussion_categories** to understand available categories before creating discussions. Filter by category for better organization.
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

## Workflow
1. Call **projects_list** to find projects.
2. Use **projects_get** with list_project_fields to understand available fields and get IDs/types.
3. Use **projects_get** with list_project_items (with pagination) to browse items.
4. Use **projects_write** for updates.

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

- **list_notifications** — list unread and read notifications
- **get_notification_details** — get details of a specific notification
- **dismiss_notification** — mark a notification as done
- **mark_all_notifications_read** — mark all notifications as read
- **manage_notification_subscription** — manage thread subscription settings
- **manage_repository_notification_subscription** — manage repository notification settings
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

- **list_gists** — list gists for the authenticated user
- **get_gist** — retrieve a specific gist by ID
- **create_gist** — create a new gist (public or private)
- **update_gist** — update an existing gist's files or description
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

- **search_users** — search for users by username, name, location, or other criteria
- **search_orgs** — search for organizations by name or other criteria

Use separate **sort** and **order** parameters for sorting results — do not include 'sort:' syntax in query strings.
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

- **list_global_security_advisories** — search the GitHub Advisory Database
- **get_global_security_advisory** — get details of a specific global advisory
- **list_repository_security_advisories** — list advisories for a specific repository
- **list_org_repository_security_advisories** — list advisories across an organization's repositories
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

- **list_label** / **list_labels** — list labels for a repository
- **label_write** — create, update, or delete labels
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

- **get_repository_tree** — retrieve the tree structure of a repository at a given ref, useful for understanding repository layout
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

- **list_starred_repositories** — list repositories starred by the authenticated user
- **star_repository** — star a repository
- **unstar_repository** — unstar a repository
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

- **assign_copilot_to_issue** — assign Copilot as a collaborator on an issue
- **request_copilot_review** — request a Copilot review on a pull request
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
