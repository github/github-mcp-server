package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ProjectUpdateFailedError = "failed to update a project item"
	ProjectAddFailedError    = "failed to add a project item"
	ProjectDeleteFailedError = "failed to delete a project item"
	ProjectListFailedError   = "failed to list project items"
	MaxProjectsPerPage       = 50
)

func ListProjects(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "list_projects",
			Description: t("TOOL_LIST_PROJECTS_DESCRIPTION", `List Projects for a user or organization`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_PROJECTS_USER_TITLE", "List projects"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"query": {
						Type:        "string",
						Description: `Filter projects by title text and open/closed state; permitted qualifiers: is:open, is:closed; examples: "roadmap is:open", "is:open feature planning".`,
					},
					"per_page": {
						Type:        "number",
						Description: fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage),
					},
					"after": {
						Type:        "string",
						Description: "Forward pagination cursor from previous pageInfo.nextCursor.",
					},
					"before": {
						Type:        "string",
						Description: "Backward pagination cursor from previous pageInfo.prevCursor (rare).",
					},
				},
				Required: []string{"owner_type", "owner"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			queryStr, err := OptionalParam[string](args, "query")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := extractPaginationOptionsFromArgs(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
				), nil, nil
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
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func GetProject(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "get_project",
			Description: t("TOOL_GET_PROJECT_DESCRIPTION", "Get Project for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PROJECT_USER_TITLE", "Get project"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"project_number": {
						Type:        "number",
						Description: "The project's number",
					},
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
				},
				Required: []string{"project_number", "owner_type", "owner"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get project", resp, body), nil, nil
			}

			minimalProject := convertToMinimalProject(project)
			r, err := json.Marshal(minimalProject)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListProjectFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "list_project_fields",
			Description: t("TOOL_LIST_PROJECT_FIELDS_DESCRIPTION", "List Project fields for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_PROJECT_FIELDS_USER_TITLE", "List project fields"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"per_page": {
						Type:        "number",
						Description: fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage),
					},
					"after": {
						Type:        "string",
						Description: "Forward pagination cursor from previous pageInfo.nextCursor.",
					},
					"before": {
						Type:        "string",
						Description: "Backward pagination cursor from previous pageInfo.prevCursor (rare).",
					},
				},
				Required: []string{"owner_type", "owner", "project_number"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := extractPaginationOptionsFromArgs(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var resp *github.Response
			var projectFields []*github.ProjectV2Field

			opts := &github.ListProjectsOptions{
				ListProjectsPaginationOptions: pagination,
			}

			if ownerType == "org" {
				projectFields, resp, err = client.Projects.ListOrganizationProjectFields(ctx, owner, projectNumber, opts)
			} else {
				projectFields, resp, err = client.Projects.ListUserProjectFields(ctx, owner, projectNumber, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to list project fields",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			response := map[string]any{
				"fields":   projectFields,
				"pageInfo": buildPageInfo(resp),
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func GetProjectField(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "get_project_field",
			Description: t("TOOL_GET_PROJECT_FIELD_DESCRIPTION", "Get Project field for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PROJECT_FIELD_USER_TITLE", "Get project field"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"field_id": {
						Type:        "number",
						Description: "The field's id.",
					},
				},
				Required: []string{"owner_type", "owner", "project_number", "field_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			fieldID, err := RequiredBigInt(args, "field_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get project field", resp, body), nil, nil
			}
			r, err := json.Marshal(projectField)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func ListProjectItems(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "list_project_items",
			Description: t("TOOL_LIST_PROJECT_ITEMS_DESCRIPTION", `Search project items with advanced filtering`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_PROJECT_ITEMS_USER_TITLE", "List project items"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"query": {
						Type:        "string",
						Description: `Query string for advanced filtering of project items using GitHub's project filtering syntax.`,
					},
					"per_page": {
						Type:        "number",
						Description: fmt.Sprintf("Results per page (max %d)", MaxProjectsPerPage),
					},
					"after": {
						Type:        "string",
						Description: "Forward pagination cursor from previous pageInfo.nextCursor.",
					},
					"before": {
						Type:        "string",
						Description: "Backward pagination cursor from previous pageInfo.prevCursor (rare).",
					},
					"fields": {
						Type:        "array",
						Description: "Field IDs to include (e.g. [\"102589\", \"985201\"]). CRITICAL: Always provide to get field values. Without this, only titles returned.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
				},
				Required: []string{"owner_type", "owner", "project_number"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			queryStr, err := OptionalParam[string](args, "query")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			fields, err := OptionalBigIntArrayParam(args, "fields")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := extractPaginationOptionsFromArgs(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var resp *github.Response
			var projectItems []*github.ProjectV2Item
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

			if ownerType == "org" {
				projectItems, resp, err = client.Projects.ListOrganizationProjectItems(ctx, owner, projectNumber, opts)
			} else {
				projectItems, resp, err = client.Projects.ListUserProjectItems(ctx, owner, projectNumber, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					ProjectListFailedError,
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			response := map[string]any{
				"items":    projectItems,
				"pageInfo": buildPageInfo(resp),
			}

			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func GetProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "get_project_item",
			Description: t("TOOL_GET_PROJECT_ITEM_DESCRIPTION", "Get a specific Project item for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PROJECT_ITEM_USER_TITLE", "Get project item"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"item_id": {
						Type:        "number",
						Description: "The item's ID.",
					},
					"fields": {
						Type:        "array",
						Description: "Specific list of field IDs to include in the response (e.g. [\"102589\", \"985201\", \"169875\"]). If not provided, only the title field is included.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
				},
				Required: []string{"owner_type", "owner", "project_number", "item_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			itemID, err := RequiredBigInt(args, "item_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			fields, err := OptionalBigIntArrayParam(args, "fields")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var resp *github.Response
			var projectItem *github.ProjectV2Item
			var opts *github.GetProjectItemOptions

			if len(fields) > 0 {
				opts = &github.GetProjectItemOptions{
					Fields: fields,
				}
			}

			if ownerType == "org" {
				projectItem, resp, err = client.Projects.GetOrganizationProjectItem(ctx, owner, projectNumber, itemID, opts)
			} else {
				projectItem, resp, err = client.Projects.GetUserProjectItem(ctx, owner, projectNumber, itemID, opts)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					"failed to get project item",
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			r, err := json.Marshal(projectItem)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func AddProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "add_project_item",
			Description: t("TOOL_ADD_PROJECT_ITEM_DESCRIPTION", "Add a specific Project item for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_PROJECT_ITEM_USER_TITLE", "Add project item"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"item_type": {
						Type:        "string",
						Description: "The item's type, either issue or pull_request.",
						Enum:        []any{"issue", "pull_request"},
					},
					"item_id": {
						Type:        "number",
						Description: "The numeric ID of the issue or pull request to add to the project.",
					},
				},
				Required: []string{"owner_type", "owner", "project_number", "item_type", "item_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			itemID, err := RequiredBigInt(args, "item_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			itemType, err := RequiredParam[string](args, "item_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if itemType != "issue" && itemType != "pull_request" {
				return utils.NewToolResultError("item_type must be either 'issue' or 'pull_request'"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, ProjectAddFailedError, resp, body), nil, nil
			}
			r, err := json.Marshal(addedItem)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func UpdateProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "update_project_item",
			Description: t("TOOL_UPDATE_PROJECT_ITEM_DESCRIPTION", "Update a specific Project item for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_UPDATE_PROJECT_ITEM_USER_TITLE", "Update project item"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"item_id": {
						Type:        "number",
						Description: "The unique identifier of the project item. This is not the issue or pull request ID.",
					},
					"updated_field": {
						Type:        "object",
						Description: "Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set value to null. Example: {\"id\": 123456, \"value\": \"New Value\"}",
					},
				},
				Required: []string{"owner_type", "owner", "project_number", "item_id", "updated_field"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			itemID, err := RequiredBigInt(args, "item_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			rawUpdatedField, exists := args["updated_field"]
			if !exists {
				return utils.NewToolResultError("missing required parameter: updated_field"), nil, nil
			}

			fieldValue, ok := rawUpdatedField.(map[string]any)
			if !ok || fieldValue == nil {
				return utils.NewToolResultError("field_value must be an object"), nil, nil
			}

			updatePayload, err := buildUpdateProjectItem(fieldValue)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var resp *github.Response
			var updatedItem *github.ProjectV2Item

			if ownerType == "org" {
				updatedItem, resp, err = client.Projects.UpdateOrganizationProjectItem(ctx, owner, projectNumber, itemID, updatePayload)
			} else {
				updatedItem, resp, err = client.Projects.UpdateUserProjectItem(ctx, owner, projectNumber, itemID, updatePayload)
			}

			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx,
					ProjectUpdateFailedError,
					resp,
					err,
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, ProjectUpdateFailedError, resp, body), nil, nil
			}
			r, err := json.Marshal(updatedItem)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
}

func DeleteProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "delete_project_item",
			Description: t("TOOL_DELETE_PROJECT_ITEM_DESCRIPTION", "Delete a specific Project item for a user or org"),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_DELETE_PROJECT_ITEM_USER_TITLE", "Delete project item"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner_type": {
						Type:        "string",
						Description: "Owner type",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "If owner_type == user it is the handle for the GitHub user account. If owner_type == org it is the name of the organization. The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"item_id": {
						Type:        "number",
						Description: "The internal project item ID to delete from the project (not the issue or pull request ID).",
					},
				},
				Required: []string{"owner_type", "owner", "project_number", "item_id"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			ownerType, err := RequiredParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			itemID, err := RequiredBigInt(args, "item_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
				), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusNoContent {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, ProjectDeleteFailedError, resp, body), nil, nil
			}
			return utils.NewToolResultText("project item successfully deleted"), nil, nil
		},
	)
}

type pageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor,omitempty"`
	PrevCursor      string `json:"prevCursor,omitempty"`
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

// validateAndConvertToInt64 ensures the value is a number and converts it to int64.
func validateAndConvertToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		// Validate that the float64 can be safely converted to int64
		intVal := int64(v)
		if float64(intVal) != v {
			return 0, fmt.Errorf("value must be a valid integer (got %v)", v)
		}
		return intVal, nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("value must be a number (got %T)", v)
	}
}

// buildUpdateProjectItem constructs UpdateProjectItemOptions from the input map.
func buildUpdateProjectItem(input map[string]any) (*github.UpdateProjectItemOptions, error) {
	if input == nil {
		return nil, fmt.Errorf("updated_field must be an object")
	}

	idField, ok := input["id"]
	if !ok {
		return nil, fmt.Errorf("updated_field.id is required")
	}

	fieldID, err := validateAndConvertToInt64(idField)
	if err != nil {
		return nil, fmt.Errorf("updated_field.id: %w", err)
	}

	valueField, ok := input["value"]
	if !ok {
		return nil, fmt.Errorf("updated_field.value is required")
	}

	payload := &github.UpdateProjectItemOptions{
		Fields: []*github.UpdateProjectV2Field{{
			ID:    fieldID,
			Value: valueField,
		}},
	}

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

func extractPaginationOptionsFromArgs(args map[string]any) (github.ListProjectsPaginationOptions, error) {
	perPage, err := OptionalIntParamWithDefault(args, "per_page", MaxProjectsPerPage)
	if err != nil {
		return github.ListProjectsPaginationOptions{}, err
	}
	if perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}

	after, err := OptionalParam[string](args, "after")
	if err != nil {
		return github.ListProjectsPaginationOptions{}, err
	}

	before, err := OptionalParam[string](args, "before")
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
