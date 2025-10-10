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
	"github.com/google/go-github/v74/github"
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

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2", owner)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2", owner)
			}
			projects := []github.ProjectV2{}
			minimalProjects := []MinimalProject{}

			opts := listProjectsOptions{PerPage: perPage}

			if queryStr != "" {
				opts.Query = queryStr
			}
			url, err = addOptions(url, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to add options to request: %w", err)
			}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := client.Do(ctx, httpRequest, &projects)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list projects",
					resp,
					err,
				), nil
			}
			defer func() { _ = resp.Body.Close() }()

			for _, project := range projects {
				minimalProjects = append(minimalProjects, *convertToMinimalProject(&project))
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

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d", owner, projectNumber)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d", owner, projectNumber)
			}

			project := github.ProjectV2{}

			httpRequest, err := client.NewRequest("GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := client.Do(ctx, httpRequest, &project)
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

			minimalProject := convertToMinimalProject(&project)
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

			opts := listProjectsOptions{PerPage: perPage}
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

			opts := listProjectsOptions{PerPage: perPage}
			if queryStr != "" {
				opts.Query = queryStr
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
			minimalProjectItems := []MinimalProjectItem{}
			for _, item := range projectItems {
				minimalProjectItems = append(minimalProjectItems, *convertToMinimalProjectItem(&item))
			}
			r, err := json.Marshal(minimalProjectItems)
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

			var url string
			if ownerType == "org" {
				url = fmt.Sprintf("orgs/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
			} else {
				url = fmt.Sprintf("users/%s/projectsV2/%d/items/%d", owner, projectNumber, itemID)
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
			r, err := json.Marshal(convertToMinimalProjectItem(&projectItem))
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
			r, err := json.Marshal(convertToMinimalProjectItem(&addedItem))
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
				mcp.Description("Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set \"value\" to null. Example: {\"id\": 123456, \"value\": \"New Value\"}"),
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
			r, err := json.Marshal(convertToMinimalProjectItem(&updatedItem))
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

type projectV2Item struct {
	ID            *int64            `json:"id,omitempty"`
	Title         *string           `json:"title,omitempty"`
	Description   *string           `json:"description,omitempty"`
	NodeID        *string           `json:"node_id,omitempty"`
	ProjectNodeID *string           `json:"project_node_id,omitempty"`
	ContentNodeID *string           `json:"content_node_id,omitempty"`
	ProjectURL    *string           `json:"project_url,omitempty"`
	ContentType   *string           `json:"content_type,omitempty"`
	Creator       *github.User      `json:"creator,omitempty"`
	CreatedAt     *github.Timestamp `json:"created_at,omitempty"`
	UpdatedAt     *github.Timestamp `json:"updated_at,omitempty"`
	ArchivedAt    *github.Timestamp `json:"archived_at,omitempty"`
	ItemURL       *string           `json:"item_url,omitempty"`
	Fields        []*projectV2Field `json:"fields,omitempty"`
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

type listProjectsOptions struct {
	// For paginated result sets, the number of results to include per page.
	PerPage int `url:"per_page,omitempty"`

	// Query Limit results to projects of the specified type.
	Query string `url:"q,omitempty"`
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
