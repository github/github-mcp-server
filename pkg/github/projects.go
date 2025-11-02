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
			mcp.WithDescription(t("TOOL_PROJECT_READ_DESCRIPTION", `GitHub Projects V2 read operations.

Methods: get_project | list_projects | list_project_fields | get_project_field | list_project_items | get_project_item

Key distinctions:
- list_projects: ONLY project metadata (title, open/closed). Never use item filters.
- list_project_items: Issues/PRs inside ONE project. Prefer explicit is:issue or is:pr.

Field usage:
- Call list_project_fields first to get IDs/types.
- Use EXACT returned field names (case-insensitive match). Don't invent names or IDs.
- Iteration synonyms (sprint/cycle/iteration) only if that field exists; map to the actual name (e.g. sprint:@current).
- Only include filters for fields that exist and are relevant.

Item query syntax:
AND = space | OR = comma (label:bug,critical) | NOT = prefix - ( -label:wontfix )
Quote multi-word values: status:"In Review" team-name:"Backend Team"
Hyphenate multi-word field names (story-points).
Ranges: points:1..3  dates:2025-01-01..2025-12-31
Comparisons: updated:>@today-7d priority:>1 points:<=10
Wildcards: title:*crash* label:bug*
Temporal shortcuts: @today @today-7d @today-30d
Iteration shortcuts: @current @next @previous

Pagination (mandatory):
Loop while pageInfo.hasNextPage=true using after=nextCursor. Keep query, fields, per_page IDENTICAL each page.

Fields parameter:
Include field IDs on EVERY paginated list_project_items call if you need values. Omit → title only.

Counting rules:
- Count items array length after full pagination.
- If multi-page: collect all pages, dedupe by item.id (fallback node_id) before totals.
- Never count field objects, content, or nested arrays as separate items.
- item.id = project item ID (for updates/deletes). item.content.id = underlying issue/PR ID.

Summary vs list:
- Summaries ONLY if user uses verbs: analyze | summarize | summary | report | overview | insights.
- Listing verbs (list/show/get/fetch/display/enumerate) → just enumerate + total.

Examples:
list_projects: "roadmap is:open"
list_project_items: state:open is:issue sprint:@current priority:high updated:>@today-7d

Self-check before returning:
☑ Paginated fully ☑ Dedupe by id/node_id ☑ Correct IDs used ☑ Field names valid ☑ Summary only if requested.

Return COMPLETE data or state what's missing (e.g. pages skipped).`)),
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
     - "list all P1 priority items" → priority:p1 (omit state if user wants all, omit type if user speciifies "items")
     - "list all open P2 issues" → is:issue state:open priority:p2 (include state if user wants open or closed, include type if user speciifies "issues" or "PRs")
   Query Construction Heuristics:
     a. Extract type nouns: issues → is:issue | PRs, Pulls, or Pull Requests → is:pr | tasks/tickets → is:issue (ask if ambiguity)
     b. Map temporal phrases: "this week" → updated:>@today-7d
     c. Map negations: "excluding wontfix" → -label:wontfix
     d. Map priority adjectives: "high/sev1/p1" → priority:high OR priority:p1 (choose based on field presence)

Syntax Essentials (items):
   AND: space-separated. (label:bug priority:high).
   OR: comma inside one qualifier (label:bug,critical).
   NOT: leading '-' (-label:wontfix).
   Hyphenate multi-word field names. (team-name:"Backend Team", story-points:>5).
   Quote multi-word values. (status:"In Review" team-name:"Backend Team").
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

	filteredFields := filterSpecialTypes(projectFields)

	response := map[string]any{
		"fields":   filteredFields,
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

	if len(projectItem.Fields) > 0 {
		projectItem.Fields = filterSpecialTypes(projectItem.Fields)
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

	if len(projectItems) > 0 {
		for i := range projectItems {
			if len(projectItems[i].Fields) > 0 {
				projectItems[i].Fields = filterSpecialTypes(projectItems[i].Fields)
			}
		}
	}

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
	Configuration *any              `json:"configuration,omitempty"` // For iteration fields
	CreatedAt     *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt     *github.Timestamp `json:"updated_at,omitempty"`
}

func (f *projectV2Field) getDataType() string {
	if f == nil {
		return ""
	}
	return strings.ToLower(f.DataType)
}

type projectV2ItemFieldValue struct {
	ID       *int64 `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	DataType string `json:"data_type,omitempty"`
	Value    any    `json:"value,omitempty"`
}

func (v *projectV2ItemFieldValue) getDataType() string {
	if v == nil {
		return ""
	}
	return strings.ToLower(v.DataType)
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
	Body        *string                         `json:"body,omitempty"`
	ClosedAt    *github.Timestamp               `json:"closed_at,omitempty"`
	CreatedAt   *github.Timestamp               `json:"created_at,omitempty"`
	ID          *int64                          `json:"id,omitempty"`
	Number      *int                            `json:"number,omitempty"`
	Repository  *projectV2ItemContentRepository `json:"repository,omitempty"`
	State       *string                         `json:"state,omitempty"`
	StateReason *string                         `json:"stateReason,omitempty"`
	Title       *string                         `json:"title,omitempty"`
	UpdatedAt   *github.Timestamp               `json:"updated_at,omitempty"`
	URL         *string                         `json:"url,omitempty"`
	Type        *any                            `json:"type,omitempty"`
	Labels      []*any                          `json:"labels,omitempty"`
	Assignees   []*MinimalUser                  `json:"assignees,omitempty"`
	Milestone   *any                            `json:"milestone,omitempty"`
}

type projectV2ItemContentRepository struct {
	ID          *int64  `json:"id"`
	Name        *string `json:"name"`
	FullName    *string `json:"full_name"`
	Description *string `json:"description,omitempty"`
	HTMLURL     *string `json:"html_url"`
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

func buildPageInfo(resp *github.Response) pageInfo {
	return pageInfo{
		HasNextPage:     resp.After != "",
		HasPreviousPage: resp.Before != "",
		NextCursor:      resp.After,
		PrevCursor:      resp.Before,
	}
}

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

// "special" data types that are present in the project item's content object.
var specialFieldDataTypes = map[string]struct{}{
	"assignees":            {},
	"labels":               {},
	"linked_pull_requests": {},
	"milestone":            {},
	"parent_issue":         {},
	"repository":           {},
	"reviewers":            {},
	"sub_issues_progress":  {},
	"title":                {},
}

// filterSpecialTypes returns a new slice containing only those field definitions
// or field values whose DataType is NOT in the specialFieldDataTypes set. The
// input must be a slice whose element type implements getDataType() string.
//
// Applicable to:
//
//	[]*projectV2Field
//	[]*projectV2ItemFieldValue
//
// Example:
//
//	filtered := filterSpecialTypes(fields)
func filterSpecialTypes[T interface{ getDataType() string }](fields []T) []T {
	if len(fields) == 0 {
		return fields
	}
	out := make([]T, 0, len(fields))
	for _, f := range fields {
		dt := f.getDataType()
		if _, isSpecial := specialFieldDataTypes[dt]; isSpecial {
			continue
		}
		out = append(out, f)
	}
	return out
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
					Content: mcp.NewTextContent(`System guide: GitHub Projects V2.
Goal: Pick correct method, fetch COMPLETE data (no early pagination stop), apply accurate filters, and count items correctly.

Method quick map:
list_projects (metadata only) | get_project (single) | list_project_fields (define IDs) | get_project_field | list_project_items (issues/PRs) | get_project_item | project_write (mutations) | issue_read / pull_request_read (deep details)

Core rules:
- list_projects: NEVER include item-level filters.
- Before filtering on custom fields call list_project_fields.
- Always paginate until pageInfo.hasNextPage=false.
- Keep query, fields, per_page identical across pages.
- Include fields IDs on every list_project_items page if you need values.
- Prefer explicit is:issue / is:pr unless mixed set requested.
- Only summarize if verbs like analyze / summarize / report / overview / insights appear; otherwise enumerate.

Field resolution:
- Use exact returned field names; don't invent.
- Iteration synonyms map to actual existing name (Sprint → sprint:@current, etc.). If none exist, omit.
- Only add filters for fields that exist and matter to the user goal.

Query syntax essentials:
AND space | OR comma | NOT prefix - | quote multi-word values | hyphenate names | ranges points:1..5 | comparisons updated:>@today-7d priority:>1 | wildcards title:*crash*

Pagination pattern:
Call list_project_items → if hasNextPage true, repeat with after=nextCursor → stop only when false → then count/deduplicate.

Counting:
- Items array length after full pagination (dedupe by item.id or node_id).
- Never count fields array, content, assignees, labels as separate items.
- item.id = project item identifier; content.id = underlying issue/PR id.

Edge handling:
Empty pages → total=0 still return pageInfo.
Duplicates → keep first for totals.
Missing field values → null/omit, never fabricate.

Self-check: paginated? deduped? correct IDs? field names valid? summary allowed?`),
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
					Content: mcp.NewTextContent("Start by listing projects: project_read method=\"list_projects\"."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I work with fields and items?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Fields & items:
1. list_project_fields first → map lowercased name -> {id,type}.
2. Use only existing field names; no invention.
3. Iteration mapping: pick sprint/cycle/iteration only if present (sprint:@current etc.).
4. Include only relevant fields (e.g. Priority + Label for high priority bugs).
5. Build query after resolving fields ("last week" → updated:>@today-7d).
6. Paginate until hasNextPage=false; keep query/fields/per_page stable.
7. Include fields IDs every page when you need their values.
Missing field? Omit or clarify—never guess.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I update item field values?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Updating fields (project_write update_project_item):
Examples: text {"id":123,"value":"hello"} | select {"id":456,"value":789} (option ID) | number {"id":321,"value":5} | date {"id":654,"value":"2025-03-15"} | clear {"id":123,"value":null}
Rules: item_id = project item wrapper ID; confirm field IDs via list_project_fields; select/iteration = pass option/iteration ID (not name).`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Show me a workflow example."),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Workflow quick path:
1 list_projects → pick project_number.
2 list_project_fields → build field map.
3 Build query (e.g. is:issue sprint:@current priority:high updated:>@today-7d).
4 list_project_items (include field IDs) → paginate fully.
5 Optional deep dive: issue_read / pull_request_read per item.
6 Optional update: project_write update_project_item.
Reminders: iteration filter must match existing field; keep fields consistent; summarize only if asked.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I handle pagination?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Pagination:
Loop while hasNextPage=true using after=nextCursor.
Do NOT change query/fields/per_page.
Include same fields IDs every page.
Only count/summarize after final page.`),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I get more details about items?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Deep details:
Use issue_read or pull_request_read for comments/reviews/diffs after enumeration.
Inputs: repository + item content.number.
Confirm type (is:issue vs is:pr) before choosing which tool.`),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(`Query patterns:
blocked issues → is:issue (label:blocked OR status:"Blocked")
overdue tasks → is:issue due-date:<@today state:open
PRs ready for review → is:pr review-status:"Ready for Review" state:open
stale issues → is:issue updated:<@today-30d state:open
high priority bugs → is:issue label:bug priority:high state:open
team sprint PRs → is:pr team-name:"Backend Team" sprint:@current
Rules: summarize only if asked; dedupe before counts; quote multi-word values; never invent field names or IDs.`),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
