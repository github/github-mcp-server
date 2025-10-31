package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v76/github"
	"github.com/google/go-querystring/query"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ProjectUpdateFailedError = "failed to update a project item"
	ProjectAddFailedError    = "failed to add a project item"
	ProjectDeleteFailedError = "failed to delete a project item"
	ProjectListFailedError   = "failed to list project items"
	MaxProjectsPerPage       = 50
)

// ProjectRead creates a tool to perform read operations on GitHub Projects V2.
// Supports getting and listing projects, project fields, and project items.
func ProjectRead(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("project_read",
			mcp.WithDescription(t("TOOL_PROJECT_READ_DESCRIPTION", `Read operations for GitHub Projects.

DECISION GUIDE (choose method):
- get_project: You have a project number; you need its metadata.
- list_projects: User wants to discover or filter projects (TITLE / OPEN STATE ONLY).
- get_project_field: You know a field_id and need its definition.
- list_project_fields: MUST call before fetching item field values (get IDs & types).
- get_project_item: You have an item_id (project item) and want its details.
- list_project_items: User wants issues/PRs inside a project filtered by criteria.

INTENT TOKENS (map user phrasing → method):
[INTENT:DISCOVER_PROJECTS] → list_projects
[INTENT:INSPECT_PROJECT] → get_project
[INTENT:ENUM_FIELDS] → list_project_fields
[INTENT:FIELD_DETAILS] → get_project_field
[INTENT:LIST_ITEMS] → list_project_items
[INTENT:ITEM_DETAILS] → get_project_item

CRITICAL DISTINCTION:
Projects ≠ Project Items.
- list_projects filters ONLY project metadata (title, open/closed).
  DO NOT use item-level qualifiers (is:issue, is:pr, assignee:, label:, status:, parent-issue:, sprint-name:, etc).
- list_project_items filters ISSUES or PRs inside ONE project. Strongly prefer explicit type: is:issue OR is:pr unless user requests a mixed set.

FAILURE MODES TO AVOID:
1. Missing pagination (stops early) → ALWAYS loop while pageInfo.hasNextPage=true.
2. Missing 'fields' when listing items → only title returned; no field values.
3. Using item filters in list_projects → returns zero or irrelevant results.
4. Ambiguous item type (issues vs PRs) → default to clarifying OR supply both (omit type only if user truly wants both).
5. Inventing field IDs → fetch via list_project_fields first.
6. INVENTING FIELD NAMES (NEW) → MUST use exact names returned by list_project_fields (case-insensitive match, preserve original spelling/hyphenation).

FIELD NAME RESOLUTION (CRITICAL – ALWAYS DO BEFORE BUILDING QUERY WITH CUSTOM FIELDS):
1. Call list_project_fields → build a map of lowercased field name → original field name + type.
2. When user mentions a concept (e.g. "current sprint", "this iteration", "in the cycle"):
   - Identify iteration-type fields (type == iteration).
   - Accept synonyms in user phrasing: sprint, iteration, cycle.
   - If user uses a generic phrase ("current sprint") and the existing iteration field is named "Sprint" → use sprint:@current.
   - If the field is named "Cycle" → cycle:@current.
   - If the field is named "Iteration" → iteration:@current.
   - NEVER substitute a synonym that does not exist among field names.
3. For any other custom fields (e.g. "dev phase", "story points", "team name"):
   - Normalize user phrase → lower-case, replace spaces with hyphens.
   - Match against available field names in lower-case.
   - Use the ORIGINAL field name in the query exactly (including hyphenation and case if needed).
4. If multiple iteration-type fields exist and the user intent is ambiguous → ask for clarification OR pick the one whose name best matches the user phrase.
5. INVALID if you use a field name not present in list_project_fields.

VALID vs INVALID (Iteration Example):
User request: "Analyze the last week's activity ... for issues in the current sprint"
Fields contain iteration field named "sprint":
  VALID:  is:issue updated:>@today-7d sprint:@current
  INVALID: is:issue updated:>@today-7d iteration:@current
Fields contain iteration field named "cycle":
  VALID:  is:issue updated:>@today-7d cycle:@current
  INVALID: is:issue updated:>@today-7d iteration:@current
Fields contain iteration field named "iteration":
  VALID:  is:issue updated:>@today-7d iteration:@current
  INVALID: is:issue updated:>@today-7d sprint:@current (if 'sprint' not defined)

If NO iteration-type field exists → omit that qualifier OR clarify with user ("No iteration field found; continue without sprint filter?").

QUERY TRANSLATION (items):
User: "Open sprint issues assigned to me" →
   state:open is:issue assignee:@me sprint:@current
User: "PRs waiting for review" →
   is:pr status:"Ready for Review"
User: "High priority bugs updated this week" →
   is:issue label:bug priority:high updated:>@today-7d

SYNTAX RULES (items):
- AND: space-separated qualifiers.
- OR: comma inside one qualifier (label:bug,critical).
- NOT: prefix qualifier with '-' (-label:wontfix).
- Hyphenate multi-word field names: sprint-name, team-name, parent-issue.
- Quote multi-word values: status:"In Review".
- Comparison & ranges: priority:1..3 updated:<@today-14d.
- Wildcards: title:*search*, label:bug*.
- Presence: has:assignee, no:label, -no:assignee (force presence).

GOOD PROJECT QUERIES (list_projects):
  roadmap is:open
  is:open feature planning
BAD (reject for list_projects — item filters present):
  is:issue state:open
  assignee:@me sprint-name:"Q3"
  label:bug priority:high

VALID ITEM QUERIES (list_project_items):
  state:open is:issue priority:high sprint:@current
  is:pr status:"In Review" team-name:"Backend Team"
  is:issue -label:wontfix updated:>@today-30d
  is:issue parent-issue:"github/repo#123"

PAGINATION LOOP (ALL list_*):
1. Call list_*.
2. Read pageInfo.hasNextPage.
3. If true → call again with after=pageInfo.nextCursor (same query, fields, per_page).
4. Repeat until hasNextPage=false.
5. Aggregate ALL pages BEFORE summarizing.

DATA COMPLETENESS RULE:
Never summarize, infer trends, or perform counts until all pages are retrieved.

DEEP DETAILS:
Project item = lightweight wrapper. For full issue/PR inspection use issue_read or pull_request_read after enumerating items.

DO:
- Normalize user intent → precise filters.
- Fetch fields first → pass IDs every page.
- Preserve consistency across pagination.
- Resolve and validate field names from list_project_fields BEFORE using them.

DON'T:
- Mix project-only and item-only filters.
- Omit type when user scope is explicit.
- Invent field IDs or option IDs.
- Invent field names (e.g. use iteration:@current when only sprint exists).
- Stop early on pagination.`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_PROJECT_READ_USER_TITLE", "Read project information"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description(`Read operation: get_project, list_projects, get_project_field, list_project_fields (call FIRST for IDs), get_project_item, list_project_items (use query + fields)`),
				mcp.Enum("get_project", "list_projects", "get_project_field", "list_project_fields", "get_project_item", "list_project_items"),
			),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type: 'user' or 'org'"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("GitHub username or org name (case-insensitive)"),
			),
			mcp.WithNumber("project_number",
				mcp.Description("Project number (required for most methods)"),
			),
			mcp.WithNumber("field_id",
				mcp.Description("Field ID (required for get_project_field)"),
			),
			mcp.WithNumber("item_id",
				mcp.Description("Item ID (required for get_project_item)"),
			),
			mcp.WithString("query",
				mcp.Description(`Query string (used ONLY with list_projects and list_project_items). 

Pattern Split:

1. list_projects (project metadata only):
   Scope: title text + open/closed state.
   PERMITTED qualifiers: is:open, is:closed (state), simple title terms.
   FORBIDDEN: is:issue, is:pr, assignee:, label:, status:, sprint-name:, parent-issue:, team-name:, priority:, etc.
   Examples:
     - roadmap is:open
     - is:open feature planning
   Reject & switch method if user intends items.

2. list_project_items (issues / PRs inside ONE project):
   MUST reflect user intent; strongly prefer explicit content type if narrowed:
     - "open issues" → state:open is:issue
     - "merged PRs" → state:merged is:pr
     - "items updated this week" → updated:>@today-7d (omit type only if mixed desired)
   Query Construction Heuristics:
     a. Extract type nouns: issues → is:issue | PRs, Pulls, or Pull Requests → is:pr | tasks/tickets → is:issue (ask if ambiguity)
     b. Map temporal phrases: "this week" → updated:>@today-7d
     c. Map negations: "excluding wontfix" → -label:wontfix
     d. Map priority adjectives: "high/sev1/p1" → priority:high OR priority:p1 (choose based on field presence)
     e. Map blocking relations: "blocked by 123" → parent-issue:"owner/repo#123"

Syntax Essentials (items):
   AND: space-separated.
   OR: comma inside one qualifier (label:bug,critical).
   NOT: leading '-' (-label:wontfix).
   Hyphenate multi-word field names.
   Quote multi-word values.
   Ranges: points:1..3, updated:<@today-30d.
   Wildcards: title:*crash*, label:bug*.

Common Qualifier Glossary (items):
   is:issue | is:pr | state:open|closed|merged | assignee:@me|username | label:NAME | status:VALUE |
   priority:p1|high | sprint-name:@current | team-name:"Backend Team" | parent-issue:"org/repo#123" |
   updated:>@today-7d | title:*text* | -label:wontfix | label:bug,critical | no:assignee | has:label

Pagination Mandate:
   Do not analyze until ALL pages fetched (loop while pageInfo.hasNextPage=true). Always reuse identical query, fields, per_page.

Recovery Guidance:
   If user provides ambiguous request ("show project activity") → ask clarification OR return mixed set (omit is:issue/is:pr). If user mixes project + item qualifiers in one phrase → split: run list_projects for discovery, then list_project_items for detail.

Never:
   - Infer field IDs; fetch via list_project_fields.
   - Drop 'fields' param on subsequent pages if field values are needed.`),
			),
			mcp.WithNumber("per_page",
				mcp.Description(fmt.Sprintf("Results per page (max %d). Keep constant across paginated requests; changing mid-sequence can complicate page traversal.", MaxProjectsPerPage)),
			),
			mcp.WithString("after",
				mcp.Description("Forward pagination cursor. Use when the previous response's pageInfo.hasNextPage=true. Supply pageInfo.nextCursor as 'after' and immediately request the next page. LOOP UNTIL pageInfo.hasNextPage=false (don't stop early). Keep query, fields, and per_page identical for every page."),
			),
			mcp.WithString("before",
				mcp.Description("Backward pagination cursor (rare): supply to move to the preceding page using pageInfo.prevCursor. Not needed for normal forward iteration."),
			),
			mcp.WithArray("fields",
				mcp.Description("Field IDs to include (e.g. [\"102589\", \"985201\"]). CRITICAL: Always provide to get field values. Without this, only titles returned. Get IDs from list_project_fields first."),
				mcp.WithStringItems(),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			method, err := RequiredParam[string](request, "method")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](request, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			switch method {
			case "get_project":
				projectNumber, err := RequiredInt(request, "project_number")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return getProject(ctx, client, owner, ownerType, projectNumber)

			case "list_projects":
				queryStr, err := OptionalParam[string](request, "query")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				pagination, err := extractPaginationOptions(request)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				return listProjects(ctx, client, owner, ownerType, queryStr, pagination)

			case "get_project_field":
				projectNumber, err := RequiredInt(request, "project_number")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				fieldID, err := RequiredInt(request, "field_id")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return getProjectField(ctx, client, owner, ownerType, projectNumber, fieldID)

			case "list_project_fields":
				projectNumber, err := RequiredInt(request, "project_number")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				pagination, err := extractPaginationOptions(request)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				return listProjectFields(ctx, client, owner, ownerType, projectNumber, pagination)

			case "get_project_item":
				projectNumber, err := RequiredInt(request, "project_number")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				itemID, err := RequiredInt(request, "item_id")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				fields, err := OptionalStringArrayParam(request, "fields")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return getProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fields)

			case "list_project_items":
				projectNumber, err := RequiredInt(request, "project_number")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				queryStr, err := OptionalParam[string](request, "query")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				pagination, err := extractPaginationOptions(request)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				fields, err := OptionalStringArrayParam(request, "fields")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}

				return listProjectItems(ctx, client, owner, ownerType, projectNumber, queryStr, pagination, fields)

			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil
			}
		}
}

// ProjectWrite creates a tool to perform write operations on GitHub Projects V2.
// Supports adding, updating, and deleting project items.
func ProjectWrite(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("project_write",
			mcp.WithDescription(t("TOOL_PROJECT_WRITE_DESCRIPTION", `Write operations for GitHub Projects.

Methods: add_project_item (add issue/PR), update_project_item (update fields), delete_project_item (remove item).
Note: item_id for add is the issue/PR ID; for update/delete it's the project item ID.`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_PROJECT_WRITE_USER_TITLE", "Modify project items"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description(`Write operation: add_project_item (needs item_type, item_id), update_project_item (needs item_id, updated_field), delete_project_item (needs item_id)`),
				mcp.Enum("add_project_item", "update_project_item", "delete_project_item"),
			),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type: 'user' or 'org'"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("GitHub username or org name (case-insensitive)"),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("Project number"),
			),
			mcp.WithNumber("item_id",
				mcp.Required(),
				mcp.Description("For add: issue/PR ID. For update/delete: project item ID (not issue/PR ID)"),
			),
			mcp.WithString("item_type",
				mcp.Description("Type to add: 'issue' or 'pull_request' (required for add_project_item)"),
				mcp.Enum("issue", "pull_request"),
			),
			mcp.WithObject("updated_field",
				mcp.Description("Field update object (required for update_project_item). Format: {\"id\": 123456, \"value\": <value>}. Value types: text=string, single-select=option ID (number), date=ISO string, number=number. Set value to null to clear."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			method, err := RequiredParam[string](request, "method")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](request, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			projectNumber, err := RequiredInt(request, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			itemID, err := RequiredInt(request, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			switch method {
			case "add_project_item":
				itemType, err := RequiredParam[string](request, "item_type")
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				if itemType != "issue" && itemType != "pull_request" {
					return mcp.NewToolResultError("item_type must be either 'issue' or 'pull_request'"), nil
				}
				return addProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, itemType)

			case "update_project_item":
				rawUpdatedField, exists := request.GetArguments()["updated_field"]
				if !exists {
					return mcp.NewToolResultError("missing required parameter: updated_field"), nil
				}
				fieldValue, ok := rawUpdatedField.(map[string]any)
				if !ok || fieldValue == nil {
					return mcp.NewToolResultError("updated_field must be an object"), nil
				}
				updatePayload, err := buildUpdateProjectItem(fieldValue)
				if err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
				return updateProjectItemHelper(ctx, client, owner, ownerType, projectNumber, itemID, updatePayload)

			case "delete_project_item":
				return deleteProjectItem(ctx, client, owner, ownerType, projectNumber, itemID)

			default:
				return mcp.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil
			}
		}
}

func getProject(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int) (*mcp.CallToolResult, error) {
	var project *github.ProjectV2
	var resp *github.Response
	var err error

	if ownerType == "org" {
		project, resp, err = client.Projects.GetProjectForOrg(ctx, owner, projectNumber)
	} else {
		project, resp, err = client.Projects.GetProjectForUser(ctx, owner, projectNumber)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get project",
			resp,
			err,
		), nil
	}

	minimalProject := convertToMinimalProject(project)
	r, err := json.Marshal(minimalProject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func listProjects(ctx context.Context, client *github.Client, owner string, ownerType string, queryStr string, pagination paginationOptions) (*mcp.CallToolResult, error) {
	opts := &github.ListProjectsOptions{
		ListProjectsPaginationOptions: github.ListProjectsPaginationOptions{
			PerPage: pagination.PerPage,
			After:   pagination.After,
			Before:  pagination.Before,
		},
		Query: queryStr,
	}

	var projects []*github.ProjectV2
	var resp *github.Response
	var err error

	if ownerType == "org" {
		projects, resp, err = client.Projects.ListProjectsForOrg(ctx, owner, opts)
	} else {
		projects, resp, err = client.Projects.ListProjectsForUser(ctx, owner, opts)
	}

	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to list projects",
			resp,
			err,
		), nil
	}

	minimalProjects := []MinimalProject{}
	for _, project := range projects {
		minimalProjects = append(minimalProjects, *convertToMinimalProject(project))
	}

	// Create response with pagination info
	response := map[string]any{
		"projects": minimalProjects,
		"pageInfo": buildPageInfo(resp),
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func getProjectField(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, fieldID int) (*mcp.CallToolResult, error) {
	var url string
	if ownerType == "org" {
		url = fmt.Sprintf("orgs/%s/projectsV2/%d/fields/%d", owner, projectNumber, fieldID)
	} else {
		url = fmt.Sprintf("users/%s/projectsV2/%d/fields/%d", owner, projectNumber, fieldID)
	}

	var projectField projectV2Field

	httpRequest, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(ctx, httpRequest, &projectField)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get project field",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get project field: %s", string(body))), nil
	}

	r, err := json.Marshal(projectField)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func listProjectFields(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, pagination paginationOptions) (*mcp.CallToolResult, error) {
	var url string
	if ownerType == "org" {
		url = fmt.Sprintf("orgs/%s/projectsV2/%d/fields", owner, projectNumber)
	} else {
		url = fmt.Sprintf("users/%s/projectsV2/%d/fields", owner, projectNumber)
	}

	type listProjectFieldsOptions struct {
		paginationOptions
	}

	opts := listProjectFieldsOptions{
		paginationOptions: pagination,
	}

	url, err := addOptions(url, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to add options to request: %w", err)
	}

	httpRequest, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var projectFields []*projectV2Field
	resp, err := client.Do(ctx, httpRequest, &projectFields)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to list project fields",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to list project fields: %s", string(body))), nil
	}

	// Create response with pagination info
	response := map[string]any{
		"fields":   projectFields,
		"pageInfo": buildPageInfo(resp),
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func getProjectItem(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, itemID int, fields []string) (*mcp.CallToolResult, error) {
	var url string
	if ownerType == "org" {
		url = fmt.Sprintf("orgs/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	} else {
		url = fmt.Sprintf("users/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	}

	opts := fieldSelectionOptions{}

	if len(fields) > 0 {
		opts.Fields = strings.Join(fields, ",")
	}

	url, err := addOptions(url, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	projectItem := projectV2Item{}

	httpRequest, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(ctx, httpRequest, &projectItem)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get project item",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to get project item: %s", string(body))), nil
	}
	r, err := json.Marshal(projectItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func listProjectItems(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, queryStr string, pagination paginationOptions, fields []string) (*mcp.CallToolResult, error) {
	var url string
	if ownerType == "org" {
		url = fmt.Sprintf("orgs/%s/projectsV2/%d/items", owner, projectNumber)
	} else {
		url = fmt.Sprintf("users/%s/projectsV2/%d/items", owner, projectNumber)
	}
	projectItems := []projectV2Item{}

	opts := listProjectItemsOptions{
		paginationOptions:     pagination,
		filterQueryOptions:    filterQueryOptions{Query: queryStr},
		fieldSelectionOptions: fieldSelectionOptions{Fields: strings.Join(fields, ",")},
	}

	url, err := addOptions(url, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to add options to request: %w", err)
	}

	fmt.Println("URL for listProjectItems:")
	fmt.Println(url)

	httpRequest, err := client.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(ctx, httpRequest, &projectItems)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			ProjectListFailedError,
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", ProjectListFailedError, string(body))), nil
	}

	// Create response with pagination info
	response := map[string]any{
		"items":    projectItems,
		"pageInfo": buildPageInfo(resp),
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

// Helper functions for ProjectWrite

func addProjectItem(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, itemID int, itemType string) (*mcp.CallToolResult, error) {
	var projectsURL string
	if ownerType == "org" {
		projectsURL = fmt.Sprintf("orgs/%s/projectsV2/%d/items", owner, projectNumber)
	} else {
		projectsURL = fmt.Sprintf("users/%s/projectsV2/%d/items", owner, projectNumber)
	}

	newItem := &newProjectItem{
		ID:   int64(itemID),
		Type: toNewProjectType(itemType),
	}
	httpRequest, err := client.NewRequest("POST", projectsURL, newItem)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	addedItem := projectV2Item{}

	resp, err := client.Do(ctx, httpRequest, &addedItem)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			ProjectAddFailedError,
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", ProjectAddFailedError, string(body))), nil
	}
	r, err := json.Marshal(addedItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func updateProjectItemHelper(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, itemID int, updatePayload *updateProjectItem) (*mcp.CallToolResult, error) {
	var projectsURL string
	if ownerType == "org" {
		projectsURL = fmt.Sprintf("orgs/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	} else {
		projectsURL = fmt.Sprintf("users/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	}
	httpRequest, err := client.NewRequest("PATCH", projectsURL, updateProjectItemPayload{
		Fields: []updateProjectItem{*updatePayload},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	updatedItem := projectV2Item{}

	resp, err := client.Do(ctx, httpRequest, &updatedItem)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			ProjectUpdateFailedError,
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", ProjectUpdateFailedError, string(body))), nil
	}
	r, err := json.Marshal(updatedItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(r)), nil
}

func deleteProjectItem(ctx context.Context, client *github.Client, owner string, ownerType string, projectNumber int, itemID int) (*mcp.CallToolResult, error) {
	var projectsURL string
	if ownerType == "org" {
		projectsURL = fmt.Sprintf("orgs/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	} else {
		projectsURL = fmt.Sprintf("users/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
	}

	httpRequest, err := client.NewRequest("DELETE", projectsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(ctx, httpRequest, nil)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			ProjectDeleteFailedError,
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return mcp.NewToolResultError(fmt.Sprintf("%s: %s", ProjectDeleteFailedError, string(body))), nil
	}
	return mcp.NewToolResultText("project item successfully deleted"), nil
}

type newProjectItem struct {
	ID   int64  `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
}

type updateProjectItemPayload struct {
	Fields []updateProjectItem `json:"fields"`
}

type updateProjectItem struct {
	ID    int `json:"id"`
	Value any `json:"value"`
}

type projectV2Field struct {
	ID            *int64            `json:"id,omitempty"`
	NodeID        string            `json:"node_id,omitempty"`
	Name          string            `json:"name,omitempty"`
	DataType      string            `json:"data_type,omitempty"`
	URL           string            `json:"url,omitempty"`
	Options       []*any            `json:"options,omitempty"`       // For single-select fields
	Configuration []*any            `json:"configuration,omitempty"` // For iteration fields
	CreatedAt     *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt     *github.Timestamp `json:"updated_at,omitempty"`
}

type projectV2ItemFieldValue struct {
	ID       *int64 `json:"id,omitempty"`        // The unique identifier for this field.
	Name     string `json:"name,omitempty"`      // The display name of the field.
	DataType string `json:"data_type,omitempty"` // The data type of the field (e.g., "text", "number", "date", "single_select", "multi_select").
	Value    any    `json:"value,omitempty"`     // The value of the field for a specific project item.
}

type projectV2Item struct {
	ArchivedAt  *github.Timestamp          `json:"archived_at,omitempty"`
	Content     *projectV2ItemContent      `json:"content,omitempty"`
	ContentType *string                    `json:"content_type,omitempty"`
	CreatedAt   *github.Timestamp          `json:"created_at,omitempty"`
	Creator     *github.User               `json:"creator,omitempty"`
	Description *string                    `json:"description,omitempty"`
	Fields      []*projectV2ItemFieldValue `json:"fields,omitempty"`
	ID          *int64                     `json:"id,omitempty"`
	ItemURL     *string                    `json:"item_url,omitempty"`
	NodeID      *string                    `json:"node_id,omitempty"`
	ProjectURL  *string                    `json:"project_url,omitempty"`
	Title       *string                    `json:"title,omitempty"`
	UpdatedAt   *github.Timestamp          `json:"updated_at,omitempty"`
}

type projectV2ItemContent struct {
	Body        *string           `json:"body,omitempty"`
	ClosedAt    *github.Timestamp `json:"closed_at,omitempty"`
	CreatedAt   *github.Timestamp `json:"created_at,omitempty"`
	ID          *int64            `json:"id,omitempty"`
	Number      *int              `json:"number,omitempty"`
	Repository  MinimalRepository `json:"repository,omitempty"`
	State       *string           `json:"state,omitempty"`
	StateReason *string           `json:"stateReason,omitempty"`
	Title       *string           `json:"title,omitempty"`
	UpdatedAt   *github.Timestamp `json:"updated_at,omitempty"`
	URL         *string           `json:"url,omitempty"`
}

type pageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor,omitempty"`
	PrevCursor      string `json:"prevCursor,omitempty"`
}

type paginationOptions struct {
	PerPage int    `url:"per_page,omitempty"`
	After   string `url:"after,omitempty"`
	Before  string `url:"before,omitempty"`
}

type filterQueryOptions struct {
	Query string `url:"q,omitempty"`
}

type fieldSelectionOptions struct {
	// Specific list of field IDs to include in the response. If not provided, only the title field is included.
	// Example: fields=102589,985201,169875 or fields[]=102589&fields[]=985201&fields[]=169875
	Fields string `url:"fields,omitempty"`
}

type listProjectItemsOptions struct {
	paginationOptions
	filterQueryOptions
	fieldSelectionOptions
}

func toNewProjectType(projType string) string {
	switch strings.ToLower(projType) {
	case "issue":
		return "Issue"
	case "pull_request":
		return "PullRequest"
	default:
		return ""
	}
}

func buildUpdateProjectItem(input map[string]any) (*updateProjectItem, error) {
	if input == nil {
		return nil, fmt.Errorf("updated_field must be an object")
	}

	idField, ok := input["id"]
	if !ok {
		return nil, fmt.Errorf("updated_field.id is required")
	}

	idFieldAsFloat64, ok := idField.(float64) // JSON numbers are float64
	if !ok {
		return nil, fmt.Errorf("updated_field.id must be a number")
	}

	valueField, ok := input["value"]
	if !ok {
		return nil, fmt.Errorf("updated_field.value is required")
	}
	payload := &updateProjectItem{ID: int(idFieldAsFloat64), Value: valueField}

	return payload, nil
}

// buildPageInfo creates a pageInfo struct from the GitHub API response
func buildPageInfo(resp *github.Response) pageInfo {
	return pageInfo{
		HasNextPage:     resp.After != "",
		HasPreviousPage: resp.Before != "",
		NextCursor:      resp.After,
		PrevCursor:      resp.Before,
	}
}

// extractPaginationOptions extracts and validates pagination parameters from a tool request
func extractPaginationOptions(request mcp.CallToolRequest) (paginationOptions, error) {
	perPage, err := OptionalIntParamWithDefault(request, "per_page", MaxProjectsPerPage)
	if err != nil {
		return paginationOptions{}, err
	}
	if perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}

	after, err := OptionalParam[string](request, "after")
	if err != nil {
		return paginationOptions{}, err
	}

	before, err := OptionalParam[string](request, "before")
	if err != nil {
		return paginationOptions{}, err
	}

	return paginationOptions{
		PerPage: perPage,
		After:   after,
		Before:  before,
	}, nil
}

// addOptions adds the parameters in opts as URL query parameters to s. opts
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opts any) (string, error) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	u, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	qs, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	fmt.Println("URL for listProjectItems:")
	fmt.Println(qs.Encode())

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

func ManageProjectItemsPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("ManageProjectItems",
			mcp.WithPromptDescription(t("PROMPT_MANAGE_PROJECT_ITEMS_DESCRIPTION", "Guide for GitHub Projects V2: discovery, fields, querying, updates.")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("The owner of the project (user or organization name)"), mcp.RequiredArgument()),
			mcp.WithArgument("owner_type", mcp.ArgumentDescription("Type of owner: 'user' or 'org'"), mcp.RequiredArgument()),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			ownerType := request.Params.Arguments["owner_type"]

			task := ""
			if t, exists := request.Params.Arguments["task"]; exists {
				task = fmt.Sprintf("%v", t)
			}

			messages := []mcp.PromptMessage{
				{
					Role: "system",
					Content: mcp.NewTextContent(`You are an assistant for GitHub Projects V2.
PRIMARY GOAL: Select correct method and produce COMPLETE results (no pagination truncation).

METHOD DECISION FLOW:
1. Need list of projects? → project_read list_projects
2. Have project_number → need its metadata? → project_read get_project
3. Need field definitions (IDs/types)? → project_read list_project_fields
4. Have field_id → single field details? → project_read get_project_field
5. Need issues/PRs inside one project? → project_read list_project_items
6. Have item_id → full item wrapper? → project_read get_project_item
7. Need to modify items? → project_write (add/update/delete)
8. Need deep issue/PR details beyond wrapper? → issue_read / pull_request_read

CORE RULES (NON-NEGOTIABLE):
- Call list_project_fields BEFORE querying items for field values.
- ALWAYS include 'query' when calling list_project_items.
- ALWAYS include 'fields' (IDs) on EVERY paginated call if field values matter.
- ALWAYS paginate until pageInfo.hasNextPage=false.
- NEVER use item-level qualifiers with list_projects.
- STRONGLY prefer is:issue or is:pr in item queries when scope is clear.
- DO NOT summarize or count until all pages fetched.

QUERY BUILDING (ITEMS):
Translate user intent → structured filters:
- "open sprint issues assigned to me" → state:open is:issue assignee:@me sprint-name:@current
- "recent merged PRs backend team" → state:merged is:pr team-name:"Backend Team" updated:>@today-7d
- "exclude wontfix high priority bugs" → is:issue label:bug priority:high -label:wontfix state:open

SYNTAX:
AND: space. OR: comma (label:bug,critical). NOT: -label:wontfix.
Quote multi-word values. Hyphenate multi-word field names (sprint-name).
Ranges: points:1..3, updated:<@today-14d.
Temporal shortcuts: @today @today-7d @today-30d.
Iteration shortcuts: @current @next @previous.
Comparison Operators: (For number, date, and iteration field types)
- field:>VALUE	priority:>1 will show items with a priority greater than 1.
- field:>=VALUE	date:>=2022-06-01 will show items with a date of "2022-06-01" or later.
- field:<VALUE	<iteration-field-name>:<"Iteration 5" will show items with an iteration before "Iteration 5."
- field:<=VALUE	points:<=10 will show items with 10 or less points.

PAGINATION LOOP:
1. Call list_project_items
2. If pageInfo.hasNextPage=true → repeat with after=nextCursor (same query, fields, per_page)
3. Aggregate until hasNextPage=false.

RECOVERY / AMBIGUITY:
- If user says “show items” with no type: omit is:issue/is:pr OR ask clarifying question.
- If unknown field name appears: fetch list_project_fields, match by normalized lower-case; never guess.

DO / DON'T:
DO normalize user phrases → filters.
DO preserve parameter consistency when paginating.
DON'T invent field IDs or option IDs.
DON'T halt early on pagination.
DON'T mix discovery (list_projects) with item filtering.

HANDOFF:
For deeper issue/PR details (comments, diff, reviews) → use issue_read / pull_request_read after enumerating items.

QUALITY GUARANTEE:
Return COMPLETE data sets or explicitly state what's missing (e.g., user withheld pagination).`),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I want to work with GitHub Projects for %s (owner_type: %s).%s",
						owner,
						ownerType,
						func() string {
							if task != "" {
								return fmt.Sprintf(" Focus: %s.", task)
							}
							return ""
						}())),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("I'll help manage GitHub Projects. First, let's list projects using **project_read** with method=\"list_projects\"."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I work with fields and items?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**Working with Fields & Items (Field Name Resolution Is Critical)**

1. Enumerate fields first:
   Use project_read method="list_project_fields" project_number=<num>.
   Build a lookup: lowercased field name -> original name + data_type.

2. Resolve user phrases to actual field names:
   - Normalize user phrase: lowercase, trim, replace spaces with hyphens for matching.
   - Only use EXACT existing field names returned by list_project_fields.
   - Never invent synonyms; if user says "sprint" but only "Iteration" exists, use iteration:@current (NOT sprint:@current).

3. Iteration field synonyms:
   User may say: "sprint", "iteration", "cycle", "current sprint", "this cycle".
   VALID substitution depends on actual field:
     - If field list contains "Sprint": sprint:@current
     - If field list contains "Cycle":  cycle:@current
     - If field list contains "Iteration": iteration:@current
   INVALID examples:
     - iteration:@current when only "Sprint" exists
     - sprint:@current when only "Cycle" exists

4. Other custom fields (examples):
   - "dev phase" → dev-phase:<value> (if field name is "Dev Phase" or "dev-phase")
   - "story points" → story-points:<range or value>
   - Preserve hyphenation EXACTLY as in the original field name.

5. If ambiguous or multiple candidates (e.g., both "Sprint" and "Iteration"):
   - Prefer the one that matches the user's wording.
   - Ask for clarification if intent is unclear.

6. Build the query AFTER resolving field names:
   Example request: "Analyze last week's activity for issues in the current sprint"
     - Temporal phrase "last week's activity" → updated:>@today-7d
     - Content type "issues" → is:issue
     - "current sprint" (field name?):
        sprint:@current   (if 'Sprint')
        cycle:@current    (if 'Cycle')
        iteration:@current (if 'Iteration')
   Final VALID queries depend entirely on actual field names.

7. Always paginate:
   Check pageInfo.hasNextPage. If true, repeat with after=<nextCursor> (same query, fields, per_page).

8. Include fields parameter:
   Pass fields=["<id1>", "<id2>", ...] on EVERY page if you want field values.

Remember: Field presence governs filter legality. If a field doesn’t exist, either omit that filter or ask for clarification.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I update item field values?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**Update Item Field Values:** project_write method="update_project_item" with updated_field:

Text: {"id": 123456, "value": "text"}
Single-select: {"id": 198354254, "value": 18498754} (value = option ID)
Iteration: {"id": 198354254, "value": 18498754} (value = configuration's iteration ID)
Date: {"id": 789012, "value": "2025-03-15"}
Number: {"id": 345678, "value": 5}
Clear: {"id": 123456, "value": null}

Requirements:
- Use the project item_id (wrapper), NOT the issue/PR number.
- Confirm field ID from list_project_fields before updating.
- Single-select requires the numeric option ID (do not pass the name).
- Iteration requires the iteration ID from the field configuration (do not pass the name).`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Show me a workflow example."),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**Workflow (Including Field Name Resolution):**

1. Discover projects:
   project_read method="list_projects" owner=<owner> owner_type=<owner_type>

2. Get fields (MUST before filtering on custom fields):
   project_read method="list_project_fields" project_number=<project_number>
   → Build map: lowercased field name -> {originalName, type, id}

3. Resolve user intent → query:
   - User phrase: "current sprint high priority issues updated this week"
   - Fields: has 'Sprint' (iteration type), has 'Priority'
   - Query: is:issue sprint:@current priority:high updated:>@today-7d

4. List items (FIRST page):
   project_read method="list_project_items"
     project_number=<project_number>
     query="is:issue sprint:@current priority:high updated:>@today-7d"
     fields=["<sprintFieldID>", "<priorityFieldID>", "<statusFieldID>"]

5. Pagination:
   If pageInfo.hasNextPage=true → repeat step 4 with after=<nextCursor> (same query, fields, per_page).
   Continue until hasNextPage=false. Aggregate all pages.

6. Deeper inspection:
   Only when needed: for each item → extract repository + issue/PR number → call issue_read or pull_request_read for comments, reviews, etc.

7. Update a field:
   project_write method="update_project_item" project_number=<project_number> item_id=<projectItemID>
     updated_field={"id": <priorityFieldID>, "value": "high"}

**CRITICAL REMINDERS:**
- Never use iteration:@current when only 'Sprint' exists.
- Fields parameter MUST be identical across pagination calls.
- Don't summarize until all pages are collected.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I handle pagination?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**⚠️ Pagination Is Mandatory**

Rules:
1. Inspect pageInfo.hasNextPage on EVERY response.
2. If true → call again with after=pageInfo.nextCursor.
3. Keep query, fields, per_page EXACTLY the same.
4. Loop until hasNextPage=false.
5. Aggregate all items BEFORE analysis or summarization.

Example:
Page 1: hasNextPage=true → after="abc123"
Page 2: hasNextPage=true → after="def456"
Page 3: hasNextPage=false → DONE

Field Value Integrity:
- If you include fields=["123","456"] on page 1, you MUST include them on subsequent pages.
- Omitting fields mid-pagination yields inconsistent item data.

Never:
- Stop early.
- Change filters mid-sequence.
- Drop fields array after the first page.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I get more details about items?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**Deep Details Beyond Project Items**

Project item wrapper gives: title, item URL, basic content state, and selected field values.
For full issue/PR context (comments, reviews, diff, labels):

Issues:
  issue_read method="get"
  issue_read method="get_comments"
  issue_read method="get_labels"
  issue_read method="get_sub_issues"

Pull Requests:
  pull_request_read method="get"
  pull_request_read method="get_reviews"
  pull_request_read method="get_review_comments"
  pull_request_read method="get_files"
  pull_request_read method="get_diff"
  pull_request_read method="get_status"

Workflow:
1. Enumerate with list_project_items (capture repository + number).
2. Use repository.owner.login + repository.name + content.number for deeper calls.
3. Combine field-derived status + external discussions for a richer report.

Always confirm item type (is:issue vs is:pr) before selecting downstream method.`),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`**Query Building for Reports (Field Name Integrity)**

Preparation:
- Run list_project_fields first.
- Normalize user terms to actual field names (lowercase match).
- Use returned names; preserve hyphens.

Patterns:
- "blocked issues" → is:issue (label:blocked OR status:"Blocked" OR dev-phase:"Blocked" depending on existing fields)
- "overdue tasks" (field 'due-date') → is:issue due-date:<@today state:open
- "PRs ready for review" (field 'review-status') → is:pr review-status:"Ready for Review" state:open
- "stale issues" → is:issue updated:<@today-30d state:open
- "high priority bugs" → is:issue label:bug priority:high state:open
- "team PRs current sprint" (fields: 'team-name', 'Sprint') → is:pr team-name:"Backend Team" sprint:@current
- "iteration tracking last week" (field 'Iteration') → is:issue updated:>@today-7d iteration:@current state:open

Rules:
- Content type first: is:issue or is:pr unless mixed set requested.
- Temporal: "last week" → updated:>@today-7d; "last 30 days" → updated:>@today-30d
- Multi-word values must be quoted: team-name:"Backend Team"
- OR logic: label:bug,critical
- NOT logic: -label:wontfix
- Comparisons: 
	- Greater than:
		- number-field:>5
		- date-field:>2024-06-01
		- iteration-field:>"iteration 2"
	- Less than:
		- number-field:<3
		- date-field:<2024-12-31
		- iteration-field:<"iteration 2"
	- Greater than or equal to:
		- number-field:>=4
		- date-field:>=2024-05-15
		- iteration-field:>="iteration 1"
	- Less than or equal to:
		- number-field:<=8
		- date-field:<=2024-11-30
		- iteration-field:<="iteration 3"
- Range: 
	- number-field:1..10
	- date-field:2024-01-01..2024-12-31
	- iteration-field:"iteration 1..iteration 3"

INVALID examples:
- sprint:@current when only 'Iteration' exists
- iteration:@current when only 'Sprint' exists
- Using dev-phase:"In Progress" when no 'dev-phase' field exists (must clarify)

Golden Rule:
Never invent field names or IDs. Always source from list_project_fields.`),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
