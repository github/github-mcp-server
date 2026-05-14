package github

import "github.com/github/github-mcp-server/pkg/inventory"

// Toolset instruction functions - these generate context-aware instructions for each toolset.
// They are called during inventory build to generate server instructions.

func generateContextToolsetInstructions(_ *inventory.Inventory) string {
	return "Always call 'get_me' first to understand current user permissions and context."
}

func generateIssuesToolsetInstructions(_ *inventory.Inventory) string {
	return `## Issues

Check 'list_issue_types' first for organizations to use proper issue types. Use 'search_issues' before creating new issues to avoid duplicates. Always set 'state_reason' when closing issues.`
}

func generatePullRequestsToolsetInstructions(inv *inventory.Inventory) string {
	instructions := `## Pull Requests

PR review workflow: Always use 'pull_request_review_write' with method 'create' to create a pending review, then 'add_comment_to_pending_review' to add comments, and finally 'pull_request_review_write' with method 'submit_pending' to submit the review for complex reviews with line-specific comments.`

	if inv.HasToolset("repos") {
		instructions += `

Before creating a pull request, search for pull request templates in the repository. Template files are called pull_request_template.md or they're located in '.github/PULL_REQUEST_TEMPLATE' directory. Use the template content to structure the PR description and then call create_pull_request tool.`
	}
	return instructions
}

func generateDiscussionsToolsetInstructions(_ *inventory.Inventory) string {
	return `## Discussions

Use 'list_discussion_categories' to understand available categories before creating discussions. Filter by category for better organization.`
}

func generateProjectsToolsetInstructions(_ *inventory.Inventory) string {
	return `## Projects

Workflow: 1) list_project_fields (get field IDs), 2) list_project_items (with pagination), 3) optional updates.

Status updates: Use list_project_status_updates to read recent project status updates (newest first). Use get_project_status_update with a node ID to get a single update. Use create_project_status_update to create a new status update for a project.

Field usage:
	- Call list_project_fields first to understand available fields and get IDs/types before filtering.
	- Use EXACT returned field names (case-insensitive match). Don't invent names or IDs.
	- Iteration synonyms (sprint/cycle) only if that field exists; map to the actual name (e.g. sprint:@current).
	- Only include filters for fields that exist and are relevant.

Pagination (mandatory):
	- Loop while pageInfo.hasNextPage=true using after=pageInfo.nextCursor.
	- Keep query, fields, per_page IDENTICAL on every page.
	- Use before=pageInfo.prevCursor only when explicitly navigating to a previous page.

Counting rules:
	- Count items array length after full pagination.
	- Never count field objects, content, or nested arrays as separate items.

Summary vs list:
	- Summaries ONLY if user uses verbs: analyze | summarize | summary | report | overview | insights.
	- Listing verbs (list/show/get/fetch/display/enumerate) → enumerate + total.

Self-check before returning:
	- Paginated fully
	- Correct IDs used
	- Field names valid
	- Summary only if requested.

Return COMPLETE data or state what's missing (e.g. pages skipped).

list_project_items query rules:
Query string - For advanced filtering of project items using GitHub's project filtering syntax:

MUST reflect user intent; strongly prefer explicit content type if narrowed:
	- "open issues" → state:open is:issue
	- "merged PRs" → state:merged is:pr
	- "items updated this week" → updated:>@today-7d (omit type only if mixed desired)
	- "list all P1 priority items" → priority:p1 (omit state if user wants all, omit type if user specifies "items")
	- "list all open P2 issues" → is:issue state:open priority:p2 (include state if user wants open or closed, include type if user specifies "issues" or "PRs")
	- "all open issues I'm working on" → is:issue state:open assignee:@me

Query Construction Heuristics:
	a. Extract type nouns: issues → is:issue | PRs, Pulls, or Pull Requests → is:pr | tasks/tickets → is:issue (ask if ambiguity)
	b. Map temporal phrases: "this week" → updated:>@today-7d
	c. Map negations: "excluding wontfix" → -label:wontfix
	d. Map priority adjectives: "high/sev1/p1" → priority:high OR priority:p1 (choose based on field presence)
	e. When filtering by label, always use wildcard matching to account for cross-repository differences or emojis: (e.g. "bug 🐛" → label:*bug*)
	f. When filtering by milestone, always use wildcard matching to account for cross-repository differences: (e.g. "v1.0" → milestone:*v1.0*)

Syntax Essentials (items):
   AND: space-separated. (label:bug priority:high).
   OR: comma inside one qualifier (label:bug,critical).
   NOT: leading '-' (-label:wontfix).
   Hyphenate multi-word field names. (team-name:"Backend Team", story-points:>5).
   Quote multi-word values. (status:"In Review" team-name:"Backend Team").
   Ranges: points:1..3, updated:<@today-30d.
   Wildcards: title:*crash*, label:bug*.
   Assigned to User: assignee:@me | assignee:username | no:assignee

Common Qualifier Glossary (items):
   is:issue | is:pr | state:open|closed|merged | assignee:@me|username | label:NAME | status:VALUE |
   priority:p1|high | sprint-name:@current | team-name:"Backend Team" | parent-issue:"org/repo#123" |
   updated:>@today-7d | title:*text* | -label:wontfix | label:bug,critical | no:assignee | has:label

Never:
   - Infer field IDs; fetch via list_project_fields.
   - Drop 'fields' param on subsequent pages if field values are needed.`
}
