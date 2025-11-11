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
	"github.com/google/go-github/v77/github"
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

func ListProjects(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_projects",
			mcp.WithDescription(t("TOOL_LIST_PROJECTS_DESCRIPTION", `List Projects for a user or organization`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PROJECTS_USER_TITLE", "List projects"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner_type",
				mcp.Required(), mcp.Description("Owner type"), mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithString("query",
				mcp.Description(`Filter projects by title text and open/closed state; permitted qualifiers: is:open, is:closed; examples: "roadmap is:open", "is:open feature planning".`),
			),
			mcp.WithNumber("per_page",
				mcp.Description(fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage)),
			),
			mcp.WithString("after",
				mcp.Description("Forward pagination cursor from previous pageInfo.nextCursor."),
			),
			mcp.WithString("before",
				mcp.Description("Backward pagination cursor from previous pageInfo.prevCursor (rare)."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			queryStr, err := OptionalParam[string](req, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			pagination, err := extractPaginationOptions(req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			var projects []*github.ProjectV2
			var queryPtr *string

			if queryStr != "" {
				queryPtr = &queryStr
			}

			minimalProjects := []MinimalProject{}
			opts := &github.ListProjectsOptions{
				ListProjectsPaginationOptions: pagination,
				Query:                         queryPtr,
			}

			if ownerType == "org" {
				projects, resp, err = client.Projects.ListOrganizationProjects(ctx, owner, opts)
			} else {
				projects, resp, err = client.Projects.ListUserProjects(ctx, owner, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list projects",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

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
}

func GetProject(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project",
			mcp.WithDescription(t("TOOL_GET_PROJECT_DESCRIPTION", "Get Project for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PROJECT_USER_TITLE", "Get project"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number"),
			),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {

			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			var project *github.ProjectV2

			if ownerType == "org" {
				project, resp, err = client.Projects.GetOrganizationProject(ctx, owner, projectNumber)
			} else {
				project, resp, err = client.Projects.GetUserProject(ctx, owner, projectNumber)
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get project",
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get project: %s", string(body))), nil
			}

			minimalProject := convertToMinimalProject(project)
			r, err := json.Marshal(minimalProject)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func ListProjectFields(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_fields",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_FIELDS_DESCRIPTION", "List Project fields for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PROJECT_FIELDS_USER_TITLE", "List project fields"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"),
				mcp.Enum("user", "org")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithNumber("per_page",
				mcp.Description(fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage)),
			),
			mcp.WithString("after",
				mcp.Description("Forward pagination cursor from previous pageInfo.nextCursor."),
			),
			mcp.WithString("before",
				mcp.Description("Backward pagination cursor from previous pageInfo.prevCursor (rare)."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			pagination, err := extractPaginationOptions(req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.ListProjectsOptions{
				ListProjectsPaginationOptions: pagination,
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/fields", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/fields", owner, projectNumber)
			}

			url, err = addOptions(url, opts)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			var projectFields []projectV2Field
			resp, err := client.Do(ctx, httpRequest, &projectFields)

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list project fields",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

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
}

func GetProjectField(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_field",
			mcp.WithDescription(t("TOOL_GET_PROJECT_FIELD_DESCRIPTION", "Get Project field for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PROJECT_FIELD_USER_TITLE", "Get project field"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"), mcp.Enum("user", "org")),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number.")),
			mcp.WithNumber("field_id",
				mcp.Required(),
				mcp.Description("The field's id."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fieldID, err := RequiredBigInt(req, "field_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			var projectField *github.ProjectV2Field

			if ownerType == "org" {
				projectField, resp, err = client.Projects.GetOrganizationProjectField(ctx, owner, projectNumber, fieldID)
			} else {
				projectField, resp, err = client.Projects.GetUserProjectField(ctx, owner, projectNumber, fieldID)
			}

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
}

func ListProjectItems(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_items",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_ITEMS_DESCRIPTION", `Search project items with advanced filtering`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_PROJECT_ITEMS_USER_TITLE", "List project items"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number", mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithString("query",
				mcp.Description(`Query string for advanced filtering of project items using GitHub's project filtering syntax.`),
			),
			mcp.WithNumber("per_page",
				mcp.Description(fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage)),
			),
			mcp.WithString("after",
				mcp.Description("Forward pagination cursor from previous pageInfo.nextCursor."),
			),
			mcp.WithString("before",
				mcp.Description("Backward pagination cursor from previous pageInfo.prevCursor (rare)."),
			),
			mcp.WithArray("fields",
				mcp.Description("Field IDs to include (e.g. [\"102589\", \"985201\"]). CRITICAL: Always provide to get field values. Without this, only titles returned."),
				mcp.WithStringItems(),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			queryStr, err := OptionalParam[string](req, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			fields, err := OptionalBigIntArrayParam(req, "fields")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			pagination, err := extractPaginationOptions(req)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			var projectItems []projectV2Item
			var queryPtr *string

			if queryStr != "" {
				queryPtr = &queryStr
			}

			opts := &github.ListProjectItemsOptions{
				Fields: fields,
				ListProjectsOptions: github.ListProjectsOptions{
					ListProjectsPaginationOptions: pagination,
					Query:                         queryPtr,
				},
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/items", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/items", owner, projectNumber)
			}

			url, err = addOptions(url, opts)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err = client.Do(ctx, httpRequest, &projectItems)

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					ProjectListFailedError,
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

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
}

func GetProjectItem(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_project_item",
			mcp.WithDescription(t("TOOL_GET_PROJECT_ITEM_DESCRIPTION", "Get a specific Project item for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PROJECT_ITEM_USER_TITLE", "Get project item"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithNumber("item_id",
				mcp.Required(),
				mcp.Description("The item's ID."),
			),
			mcp.WithArray("fields",
				mcp.Description("Specific list of field IDs to include in the response (e.g. [\"102589\", \"985201\", \"169875\"]). If not provided, only the title field is included."),
				mcp.WithStringItems(),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredBigInt(req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fields, err := OptionalBigIntArrayParam(req, "fields")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
			}

			opts := fieldSelectionOptions{}

			if len(fields) > 0 {
				opts.Fields = fields
			}

			url, err = addOptions(url, opts)
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

			r, err := json.Marshal(projectItem)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

func AddProjectItem(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_project_item",
			mcp.WithDescription(t("TOOL_ADD_PROJECT_ITEM_DESCRIPTION", "Add a specific Project item for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_PROJECT_ITEM_USER_TITLE", "Add project item"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"), mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithString("item_type",
				mcp.Required(),
				mcp.Description("The item's type, either issue or pull_request."),
				mcp.Enum("issue", "pull_request"),
			),
			mcp.WithNumber("item_id",
				mcp.Required(),
				mcp.Description("The numeric ID of the issue or pull request to add to the project."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredBigInt(req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			itemType, err := RequiredParam[string](req, "item_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if itemType != "issue" && itemType != "pull_request" {
				return mcp.NewToolResultError("item_type must be either 'issue' or 'pull_request'"), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			newItem := &github.AddProjectItemOptions{
				ID:   itemID,
				Type: toNewProjectType(itemType),
			}

			var resp *github.Response
			var addedItem *github.ProjectV2Item

			if ownerType == "org" {
				addedItem, resp, err = client.Projects.AddOrganizationProjectItem(ctx, owner, projectNumber, newItem)
			} else {
				addedItem, resp, err = client.Projects.AddUserProjectItem(ctx, owner, projectNumber, newItem)
			}

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
}

func UpdateProjectItem(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_project_item",
			mcp.WithDescription(t("TOOL_UPDATE_PROJECT_ITEM_DESCRIPTION", "Update a specific Project item for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_PROJECT_ITEM_USER_TITLE", "Update project item"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner_type",
				mcp.Required(), mcp.Description("Owner type"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithNumber("item_id",
				mcp.Required(),
				mcp.Description("The unique identifier of the project item. This is not the issue or pull request ID."),
			),
			mcp.WithObject("updated_field",
				mcp.Required(),
				mcp.Description("Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set value to null. Example: {\"id\": 123456, \"value\": \"New Value\"}"),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredInt(req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			rawUpdatedField, exists := req.GetArguments()["updated_field"]
			if !exists {
				return mcp.NewToolResultError("missing required parameter: updated_field"), nil
			}

			fieldValue, ok := rawUpdatedField.(map[string]any)
			if !ok || fieldValue == nil {
				return mcp.NewToolResultError("field_value must be an object"), nil
			}

			updatePayload, err := buildUpdateProjectItem(fieldValue)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

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
}

func DeleteProjectItem(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("delete_project_item",
			mcp.WithDescription(t("TOOL_DELETE_PROJECT_ITEM_DESCRIPTION", "Delete a specific Project item for a user or org")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_DELETE_PROJECT_ITEM_USER_TITLE", "Delete project item"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner_type",
				mcp.Required(),
				mcp.Description("Owner type"),
				mcp.Enum("user", "org"),
			),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive."),
			),
			mcp.WithNumber("project_number",
				mcp.Required(),
				mcp.Description("The project's number."),
			),
			mcp.WithNumber("item_id",
				mcp.Required(),
				mcp.Description("The internal project item ID to delete from the project (not the issue or pull request ID)."),
			),
		), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](req, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			ownerType, err := RequiredParam[string](req, "owner_type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			projectNumber, err := RequiredInt(req, "project_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			itemID, err := RequiredBigInt(req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			if ownerType == "org" {
				resp, err = client.Projects.DeleteOrganizationProjectItem(ctx, owner, projectNumber, itemID)
			} else {
				resp, err = client.Projects.DeleteUserProjectItem(ctx, owner, projectNumber, itemID)
			}

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
}

type fieldSelectionOptions struct {
	// Specific list of field IDs to include in the response. If not provided, only the title field is included.
	// The comma tag encodes the slice as comma-separated values: fields=102589,985201,169875
	Fields []int64 `url:"fields,omitempty,comma"`
}

type updateProjectItemPayload struct {
	Fields []updateProjectItem `json:"fields"`
}

type updateProjectItem struct {
	ID    int `json:"id"`
	Value any `json:"value"`
}

type projectV2ItemFieldValue struct {
	ID       *int64 `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	DataType string `json:"data_type,omitempty"`
	Value    any    `json:"value,omitempty"`
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

type projectV2Field struct {
	ID            *int64            `json:"id,omitempty"`
	NodeID        *string           `json:"node_id,omitempty"`
	Name          *string           `json:"name,omitempty"`
	DataType      *string           `json:"data_type,omitempty"`
	ProjectURL    *string           `json:"project_url,omitempty"`
	Options       []any             `json:"options,omitempty"`
	Configuration any               `json:"configuration,omitempty"`
	CreatedAt     *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt     *github.Timestamp `json:"updated_at,omitempty"`
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

func extractPaginationOptions(request mcp.CallToolRequest) (github.ListProjectsPaginationOptions, error) {
	perPage, err := OptionalIntParamWithDefault(request, "per_page", MaxProjectsPerPage)
	if err != nil {
		return github.ListProjectsPaginationOptions{}, err
	}
	if perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}

	after, err := OptionalParam[string](request, "after")
	if err != nil {
		return github.ListProjectsPaginationOptions{}, err
	}

	before, err := OptionalParam[string](request, "before")
	if err != nil {
		return github.ListProjectsPaginationOptions{}, err
	}

	opts := github.ListProjectsPaginationOptions{
		PerPage: &perPage,
	}

	// Only set After/Before if they have non-empty values
	if after != "" {
		opts.After = &after
	}

	if before != "" {
		opts.Before = &before
	}

	return opts, nil
}

// addOptions adds the parameters in opts as URL query parameters to s. opts
// must be a struct whose fields may contain "url" tags.
func addOptions(s string, opts any) (string, error) {
	v := reflect.ValueOf(opts)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return s, nil
	}

	origURL, err := url.Parse(s)
	if err != nil {
		return s, err
	}

	origValues := origURL.Query()

	// Use the github.com/google/go-querystring library to parse the struct
	newValues, err := query.Values(opts)
	if err != nil {
		return s, err
	}

	// Merge the values
	for key, values := range newValues {
		for _, value := range values {
			origValues.Add(key, value)
		}
	}

	origURL.RawQuery = origValues.Encode()
	return origURL.String(), nil
}
