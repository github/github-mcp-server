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
)

func ListProjects(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_projects",
			mcp.WithDescription(t("TOOL_LIST_PROJECTS_DESCRIPTION", "List Projects for a user or org")),
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
				mcp.Description("Filter projects by a search query (matches title and description)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of results per page (max 100, default: 30)"),
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
			perPage, err := OptionalIntParamWithDefault(req, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var resp *github.Response
			var projects []*github.ProjectV2
			minimalProjects := []MinimalProject{}

			opts := &github.ListProjectsOptions{
				ListProjectsPaginationOptions: github.ListProjectsPaginationOptions{PerPage: perPage},
				Query:                         queryStr,
			}

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
			defer func() { _ = resp.Body.Close() }()

			for _, project := range projects {
				minimalProjects = append(minimalProjects, *convertToMinimalProject(project))
			}

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %s", string(body))), nil
			}
			r, err := json.Marshal(minimalProjects)
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
				mcp.Description("Number of results per page (max 100, default: 30)"),
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
			perPage, err := OptionalIntParamWithDefault(req, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/fields", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/fields", owner, projectNumber)
			}
			projectFields := []projectV2Field{}

			opts := paginationOptions{PerPage: perPage}

			url, err = addOptions(url, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to add options to request: %w", err)
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

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
			r, err := json.Marshal(projectFields)
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
			fieldID, err := RequiredInt(req, "field_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/fields/%d", owner, projectNumber, fieldID)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/fields/%d", owner, projectNumber, fieldID)
			}

			projectField := projectV2Field{}

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
}

func ListProjectItems(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_project_items",
			mcp.WithDescription(t("TOOL_LIST_PROJECT_ITEMS_DESCRIPTION", "List Project items for a user or org")),
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
				mcp.Description("Search query to filter items"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of results per page (max 100, default: 30)"),
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
			perPage, err := OptionalIntParamWithDefault(req, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			queryStr, err := OptionalParam[string](req, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fields, err := OptionalStringArrayParam(req, "fields")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/items", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/items", owner, projectNumber)
			}
			projectItems := []projectV2Item{}

			opts := listProjectItemsOptions{
				paginationOptions:     paginationOptions{PerPage: perPage},
				filterQueryOptions:    filterQueryOptions{Query: queryStr},
				fieldSelectionOptions: fieldSelectionOptions{Fields: fields},
			}

			url, err = addOptions(url, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to add options to request: %w", err)
			}

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

			r, err := json.Marshal(projectItems)
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
			itemID, err := RequiredInt(req, "item_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			fields, err := OptionalStringArrayParam(req, "fields")
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
			itemID, err := RequiredInt(req, "item_id")
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
			itemID, err := RequiredInt(req, "item_id")
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
	ID        *int64            `json:"id,omitempty"`         // The unique identifier for this field.
	NodeID    string            `json:"node_id,omitempty"`    // The GraphQL node ID for this field.
	Name      string            `json:"name,omitempty"`       // The display name of the field.
	DataType  string            `json:"data_type,omitempty"`  // The data type of the field (e.g., "text", "number", "date", "single_select", "multi_select").
	URL       string            `json:"url,omitempty"`        // The API URL for this field.
	Options   []*any            `json:"options,omitempty"`    // Available options for single_select and multi_select fields.
	CreatedAt *github.Timestamp `json:"created_at,omitempty"` // The time when this field was created.
	UpdatedAt *github.Timestamp `json:"updated_at,omitempty"` // The time when this field was last updated.
}

type projectV2ItemFieldValue struct {
	ID       *int64      `json:"id,omitempty"`        // The unique identifier for this field.
	Name     string      `json:"name,omitempty"`      // The display name of the field.
	DataType string      `json:"data_type,omitempty"` // The data type of the field (e.g., "text", "number", "date", "single_select", "multi_select").
	Value    interface{} `json:"value,omitempty"`     // The value of the field for a specific project item.
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

type paginationOptions struct {
	PerPage int `url:"per_page,omitempty"`
}

type filterQueryOptions struct {
	Query string `url:"q,omitempty"`
}

type fieldSelectionOptions struct {
	// Specific list of field IDs to include in the response. If not provided, only the title field is included.
	// Example: fields=102589,985201,169875 or fields[]=102589&fields[]=985201&fields[]=169875
	Fields []string `url:"fields,omitempty"`
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

	u.RawQuery = qs.Encode()
	return u.String(), nil
}

func ManageProjectItemsPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("ManageProjectItems",
			mcp.WithPromptDescription(t("PROMPT_MANAGE_PROJECT_ITEMS_DESCRIPTION", "Interactive guide for managing GitHub Projects V2, including discovery, field management, querying, and updates.")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("The owner of the project (user or organization name)"), mcp.RequiredArgument()),
			mcp.WithArgument("owner_type", mcp.ArgumentDescription("Type of owner: 'user' or 'org'"), mcp.RequiredArgument()),
			mcp.WithArgument("task", mcp.ArgumentDescription("Optional: specific task to focus on (e.g., 'discover_projects', 'update_items', 'create_reports')")),
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
					Content: mcp.NewTextContent("You are a GitHub Projects V2 management assistant. Your expertise includes:\n\n" +
						"**Core Capabilities:**\n" +
						"- Project discovery and field analysis\n" +
						"- Item querying with advanced filters\n" +
						"- Field value updates and management\n" +
						"- Progress reporting and insights\n\n" +
						"**Key Rules:**\n" +
						"- ALWAYS use the 'query' parameter in **list_project_items** to filter results effectively\n" +
						"- ALWAYS include 'fields' parameter with specific field IDs to retrieve field values\n" +
						"- Use proper field IDs (not names) when updating items\n" +
						"- Provide step-by-step workflows with concrete examples\n\n" +
						"**Understanding Project Items:**\n" +
						"- Project items reference underlying content (issues or pull requests)\n" +
						"- Project tools provide: project fields, item metadata, and basic content info\n" +
						"- For detailed information about an issue or pull request (comments, events, etc.), use issue/PR specific tools\n" +
						"- The 'content' field in project items includes: repository, issue/PR number, title, state\n" +
						"- Use this info to fetch full details: **get_issue**, **list_comments**, **list_issue_events**\n\n" +
						"**Available Tools:**\n" +
						"- **list_projects**: Discover available projects\n" +
						"- **get_project**: Get detailed project information\n" +
						"- **list_project_fields**: Get field definitions and IDs\n" +
						"- **list_project_items**: Query items with filters and field selection\n" +
						"- **get_project_item**: Get specific item details\n" +
						"- **add_project_item**: Add issues/PRs to projects\n" +
						"- **update_project_item**: Update field values\n" +
						"- **delete_project_item**: Remove items from projects"),
				},
				{
					Role: "user",
					Content: mcp.NewTextContent(fmt.Sprintf("I want to work with GitHub Projects for %s (owner_type: %s).%s\n\n"+
						"Help me get started with project management tasks.",
						owner,
						ownerType,
						func() string {
							if task != "" {
								return fmt.Sprintf(" I'm specifically interested in: %s.", task)
							}
							return ""
						}())),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("Perfect! I'll help you manage GitHub Projects for %s. Let me guide you through the essential workflows.\n\n"+
						"**🔍 Step 1: Project Discovery**\n"+
						"First, let's see what projects are available using **list_projects**.", owner)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Great! After seeing the projects, I want to understand how to work with project fields and items."),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent("**📋 Step 2: Understanding Project Structure**\n\n" +
						"Once you select a project, I'll help you:\n\n" +
						"1. **Get field information** using **list_project_fields**\n" +
						"   - Find field IDs, names, and data types\n" +
						"   - Understand available options for select fields\n" +
						"   - Identify required vs. optional fields\n\n" +
						"2. **Query project items** using **list_project_items**\n" +
						"   - Filter by assignees: query=\"assignee:@me\"\n" +
						"   - Filter by status: query=\"status:In Progress\"\n" +
						"   - Filter by labels: query=\"label:bug\"\n" +
						"   - Include specific fields: fields=[\"198354254\", \"198354255\"]\n\n" +
						"**💡 Pro Tip:** Always specify the 'fields' parameter to get field values, not just titles!"),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("How do I update field values? What about the different field types?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent("**✏️ Step 3: Updating Field Values**\n\n" +
						"Use **update_project_item** with the updated_field parameter. The format varies by field type:\n\n" +
						"**Text fields:**\n" +
						"```json\n" +
						"{\"id\": 123456, \"value\": \"Updated text content\"}\n" +
						"```\n\n" +
						"**Single-select fields:**\n" +
						"```json\n" +
						"{\"id\": 198354254, \"value\": 18498754}\n" +
						"```\n" +
						"*(Use option ID, not option name)*\n\n" +
						"**Date fields:**\n" +
						"```json\n" +
						"{\"id\": 789012, \"value\": \"2024-03-15\"}\n" +
						"```\n\n" +
						"**Number fields:**\n" +
						"```json\n" +
						"{\"id\": 345678, \"value\": 5}\n" +
						"```\n\n" +
						"**Clear a field:**\n" +
						"```json\n" +
						"{\"id\": 123456, \"value\": null}\n" +
						"```\n\n" +
						"**⚠️ Important:** Use the internal project item_id (not issue/PR number) for updates!"),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Can you show me a complete workflow example?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("**🔄 Complete Workflow Example**\n\n"+
						"Here's how to find and update your assigned items:\n\n"+
						"**Step 1:** Discover projects\n\n"+
						"**list_projects** owner=\"%s\" owner_type=\"%s\"\n\n\n"+
						"**Step 2:** Get project fields (using project #123)\n\n"+
						"**list_project_fields** owner=\"%s\" owner_type=\"%s\" project_number=123\n\n"+
						"*(Note the Status field ID, e.g., 198354254)*\n\n"+
						"**Step 3:** Query your assigned items\n\n"+
						"**list_project_items**\n"+
						"  owner=\"%s\"\n"+
						"  owner_type=\"%s\"\n"+
						"  project_number=123\n"+
						"  query=\"assignee:@me\"\n"+
						"  fields=[\"198354254\", \"other_field_ids\"]\n\n\n"+
						"**Step 4:** Update item status\n\n"+
						"**update_project_item**\n"+
						"  owner=\"%s\"\n"+
						"  owner_type=\"%s\"\n"+
						"  project_number=123\n"+
						"  item_id=789123\n"+
						"  updated_field={\"id\": 198354254, \"value\": 18498754}\n\n\n"+
						"Let me start by listing your projects now!", owner, ownerType, owner, ownerType, owner, ownerType, owner, ownerType)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("What if I need more details about the items, like recent comments or linked pull requests?"),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent("**📝 Accessing Underlying Issue/PR Details**\n\n" +
						"Project items contain basic content info, but for detailed information you need to use issue/PR tools:\n\n" +
						"**From project items, extract:**\n" +
						"- content.repository.name and content.repository.owner.login\n" +
						"- content.number (the issue/PR number)\n" +
						"- content_type (\"Issue\" or \"PullRequest\")\n\n" +
						"**Then use these tools for details:**\n\n" +
						"1. **Get full issue/PR details:**\n" +
						"   - **get_issue** owner=repo_owner repo=repo_name issue_number=123\n" +
						"   - Returns: full body, labels, assignees, milestone, etc.\n\n" +
						"2. **Get recent comments:**\n" +
						"   - **list_comments** owner=repo_owner repo=repo_name issue_number=123\n" +
						"   - Add since parameter to filter recent comments\n\n" +
						"3. **Get issue events:**\n" +
						"   - **list_issue_events** owner=repo_owner repo=repo_name issue_number=123\n" +
						"   - Shows timeline: assignments, label changes, status updates\n\n" +
						"4. **For pull requests specifically:**\n" +
						"   - **get_pull_request** owner=repo_owner repo=repo_name pull_number=123\n" +
						"   - **list_pull_request_reviews** for review status\n\n" +
						"**💡 Example:** To check for blockers in comments:\n" +
						"1. Get project items with query=\"assignee:@me is:open\"\n" +
						"2. For each item, extract repository and issue number from content\n" +
						"3. Use **list_comments** to get recent comments\n" +
						"4. Search comments for keywords like \"blocked\", \"blocker\", \"waiting\""),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
