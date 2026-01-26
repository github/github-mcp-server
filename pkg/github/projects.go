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
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

const (
	ProjectUpdateFailedError = "failed to update a project item"
	ProjectAddFailedError    = "failed to add a project item"
	ProjectDeleteFailedError = "failed to delete a project item"
	ProjectListFailedError   = "failed to list project items"
	MaxProjectsPerPage       = 50
)

// FeatureFlagHoldbackConsolidatedProjects is the feature flag that, when enabled, reverts to
// individual project tools instead of the consolidated project tools.
const FeatureFlagHoldbackConsolidatedProjects = "mcp_holdback_consolidated_projects"

// Method constants for consolidated project tools
const (
	projectsMethodListProjects      = "list_projects"
	projectsMethodListProjectFields = "list_project_fields"
	projectsMethodListProjectItems  = "list_project_items"
	projectsMethodGetProject        = "get_project"
	projectsMethodGetProjectField   = "get_project_field"
	projectsMethodGetProjectItem    = "get_project_item"
	projectsMethodAddProjectItem    = "add_project_item"
	projectsMethodUpdateProjectItem = "update_project_item"
	projectsMethodDeleteProjectItem = "delete_project_item"
)

func ListProjects(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func GetProject(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func ListProjectFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func GetProjectField(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func ListProjectItems(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func GetProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.ReadProject},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func AddProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.Project},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func UpdateProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.Project},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

func DeleteProjectItem(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
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
		[]scopes.Scope{scopes.Project},
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
	tool.FeatureFlagEnable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

// ProjectsList returns the tool and handler for listing GitHub Projects resources.
func ProjectsList(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name: "projects_list",
			Description: t("TOOL_PROJECTS_LIST_DESCRIPTION",
				`Tools for listing GitHub Projects resources.
Use this tool to list projects for a user or organization, or list project fields and items for a specific project.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PROJECTS_LIST_USER_TITLE", "List GitHub Projects resources"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The action to perform",
						Enum: []any{
							projectsMethodListProjects,
							projectsMethodListProjectFields,
							projectsMethodListProjectItems,
						},
					},
					"owner_type": {
						Type:        "string",
						Description: "Owner type (user or org). If not provided, will automatically try both.",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "The owner (user or organization login). The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number. Required for 'list_project_fields' and 'list_project_items' methods.",
					},
					"query": {
						Type:        "string",
						Description: `Filter/query string. For list_projects: filter by title text and state (e.g. "roadmap is:open"). For list_project_items: advanced filtering using GitHub's project filtering syntax.`,
					},
					"fields": {
						Type:        "array",
						Description: "Field IDs to include when listing project items (e.g. [\"102589\", \"985201\"]). CRITICAL: Always provide to get field values. Without this, only titles returned. Only used for 'list_project_items' method.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
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
				Required: []string{"method", "owner"},
			},
		},
		[]scopes.Scope{scopes.ReadProject},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := OptionalParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			switch method {
			case projectsMethodListProjects:
				return listProjects(ctx, client, args, owner, ownerType)
			case projectsMethodListProjectFields:
				// Detect owner type if not provided and project_number is available
				if ownerType == "" {
					projectNumber, err := RequiredInt(args, "project_number")
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					ownerType, err = detectOwnerType(ctx, client, owner, projectNumber)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
				}
				return listProjectFields(ctx, client, args, owner, ownerType)
			case projectsMethodListProjectItems:
				// Detect owner type if not provided and project_number is available
				if ownerType == "" {
					projectNumber, err := RequiredInt(args, "project_number")
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					ownerType, err = detectOwnerType(ctx, client, owner, projectNumber)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
				}
				return listProjectItems(ctx, client, args, owner, ownerType)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	tool.FeatureFlagDisable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

// ProjectsGet returns the tool and handler for getting GitHub Projects resources.
func ProjectsGet(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name: "projects_get",
			Description: t("TOOL_PROJECTS_GET_DESCRIPTION", `Get details about specific GitHub Projects resources.
Use this tool to get details about individual projects, project fields, and project items by their unique IDs.
`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PROJECTS_GET_USER_TITLE", "Get details of GitHub Projects resources"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute",
						Enum: []any{
							projectsMethodGetProject,
							projectsMethodGetProjectField,
							projectsMethodGetProjectItem,
						},
					},
					"owner_type": {
						Type:        "string",
						Description: "Owner type (user or org). If not provided, will be automatically detected.",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "The owner (user or organization login). The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"field_id": {
						Type:        "number",
						Description: "The field's ID. Required for 'get_project_field' method.",
					},
					"item_id": {
						Type:        "number",
						Description: "The item's ID. Required for 'get_project_item' method.",
					},
					"fields": {
						Type:        "array",
						Description: "Specific list of field IDs to include in the response when getting a project item (e.g. [\"102589\", \"985201\", \"169875\"]). If not provided, only the title field is included. Only used for 'get_project_item' method.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
				},
				Required: []string{"method", "owner", "project_number"},
			},
		},
		[]scopes.Scope{scopes.ReadProject},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := OptionalParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Detect owner type if not provided
			if ownerType == "" {
				ownerType, err = detectOwnerType(ctx, client, owner, projectNumber)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			switch method {
			case projectsMethodGetProject:
				return getProject(ctx, client, owner, ownerType, projectNumber)
			case projectsMethodGetProjectField:
				fieldID, err := RequiredBigInt(args, "field_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return getProjectField(ctx, client, owner, ownerType, projectNumber, fieldID)
			case projectsMethodGetProjectItem:
				itemID, err := RequiredBigInt(args, "item_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				fields, err := OptionalBigIntArrayParam(args, "fields")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return getProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fields)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	tool.FeatureFlagDisable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

// ProjectsWrite returns the tool and handler for modifying GitHub Projects resources.
func ProjectsWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "projects_write",
			Description: t("TOOL_PROJECTS_WRITE_DESCRIPTION", "Add, update, or delete project items in a GitHub Project."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_PROJECTS_WRITE_USER_TITLE", "Modify GitHub Project items"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The method to execute",
						Enum: []any{
							projectsMethodAddProjectItem,
							projectsMethodUpdateProjectItem,
							projectsMethodDeleteProjectItem,
						},
					},
					"owner_type": {
						Type:        "string",
						Description: "Owner type (user or org). If not provided, will be automatically detected.",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "The project owner (user or organization login). The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"item_id": {
						Type:        "number",
						Description: "The project item ID. Required for 'update_project_item' and 'delete_project_item' methods.",
					},
					"item_type": {
						Type:        "string",
						Description: "The item's type, either issue or pull_request. Required for 'add_project_item' method.",
						Enum:        []any{"issue", "pull_request"},
					},
					"item_owner": {
						Type:        "string",
						Description: "The owner (user or organization) of the repository containing the issue or pull request. Required for 'add_project_item' method.",
					},
					"item_repo": {
						Type:        "string",
						Description: "The name of the repository containing the issue or pull request. Required for 'add_project_item' method.",
					},
					"issue_number": {
						Type:        "number",
						Description: "The issue number (use when item_type is 'issue' for 'add_project_item' method). Provide either issue_number or pull_request_number.",
					},
					"pull_request_number": {
						Type:        "number",
						Description: "The pull request number (use when item_type is 'pull_request' for 'add_project_item' method). Provide either issue_number or pull_request_number.",
					},
					"updated_field": {
						Type:        "object",
						Description: "Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set value to null. Example: {\"id\": 123456, \"value\": \"New Value\"}. Required for 'update_project_item' method.",
					},
				},
				Required: []string{"method", "owner", "project_number"},
			},
		},
		[]scopes.Scope{scopes.Project},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			ownerType, err := OptionalParam[string](args, "owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Detect owner type if not provided
			if ownerType == "" {
				ownerType, err = detectOwnerType(ctx, client, owner, projectNumber)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			switch method {
			case projectsMethodAddProjectItem:
				itemType, err := RequiredParam[string](args, "item_type")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				itemOwner, err := RequiredParam[string](args, "item_owner")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				itemRepo, err := RequiredParam[string](args, "item_repo")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}

				var itemNumber int
				switch itemType {
				case "issue":
					itemNumber, err = RequiredInt(args, "issue_number")
					if err != nil {
						return utils.NewToolResultError("issue_number is required when item_type is 'issue'"), nil, nil
					}
				case "pull_request":
					itemNumber, err = RequiredInt(args, "pull_request_number")
					if err != nil {
						return utils.NewToolResultError("pull_request_number is required when item_type is 'pull_request'"), nil, nil
					}
				default:
					return utils.NewToolResultError("item_type must be either 'issue' or 'pull_request'"), nil, nil
				}

				return addProjectItem(ctx, gqlClient, owner, ownerType, projectNumber, itemOwner, itemRepo, itemNumber, itemType)
			case projectsMethodUpdateProjectItem:
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
					return utils.NewToolResultError("updated_field must be an object"), nil, nil
				}
				return updateProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fieldValue)
			case projectsMethodDeleteProjectItem:
				itemID, err := RequiredBigInt(args, "item_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return deleteProjectItem(ctx, client, owner, ownerType, projectNumber, itemID)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	tool.FeatureFlagDisable = FeatureFlagHoldbackConsolidatedProjects
	return tool
}

// Helper functions for consolidated projects tools

func listProjects(ctx context.Context, client *github.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, any, error) {
	queryStr, err := OptionalParam[string](args, "query")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	pagination, err := extractPaginationOptionsFromArgs(args)
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

	// If owner_type not provided, fetch from both user and org
	switch ownerType {
	case "":
		return listProjectsFromBothOwnerTypes(ctx, client, owner, opts)
	case "org":
		projects, resp, err = client.Projects.ListOrganizationProjects(ctx, owner, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to list projects",
				resp,
				err,
			), nil, nil
		}
	default:
		projects, resp, err = client.Projects.ListUserProjects(ctx, owner, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to list projects",
				resp,
				err,
			), nil, nil
		}
	}

	// For specified owner_type, process normally
	if ownerType != "" {
		defer func() { _ = resp.Body.Close() }()

		for _, project := range projects {
			mp := convertToMinimalProject(project)
			mp.OwnerType = ownerType
			minimalProjects = append(minimalProjects, *mp)
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
	}

	return nil, nil, fmt.Errorf("unexpected state in listProjects")
}

// listProjectsFromBothOwnerTypes fetches projects from both user and org endpoints
// when owner_type is not specified, combining the results with owner_type labels.
func listProjectsFromBothOwnerTypes(ctx context.Context, client *github.Client, owner string, opts *github.ListProjectsOptions) (*mcp.CallToolResult, any, error) {
	var minimalProjects []MinimalProject
	var resp *github.Response

	// Fetch user projects
	userProjects, userResp, userErr := client.Projects.ListUserProjects(ctx, owner, opts)
	if userErr == nil && userResp.StatusCode == http.StatusOK {
		for _, project := range userProjects {
			mp := convertToMinimalProject(project)
			mp.OwnerType = "user"
			minimalProjects = append(minimalProjects, *mp)
		}
		_ = userResp.Body.Close()
	}

	// Fetch org projects
	orgProjects, orgResp, orgErr := client.Projects.ListOrganizationProjects(ctx, owner, opts)
	if orgErr == nil && orgResp.StatusCode == http.StatusOK {
		for _, project := range orgProjects {
			mp := convertToMinimalProject(project)
			mp.OwnerType = "org"
			minimalProjects = append(minimalProjects, *mp)
		}
		resp = orgResp // Use org response for pagination info
	} else if userResp != nil {
		resp = userResp // Fallback to user response
	}

	// If both failed, return error
	if (userErr != nil || userResp == nil || userResp.StatusCode != http.StatusOK) &&
		(orgErr != nil || orgResp == nil || orgResp.StatusCode != http.StatusOK) {
		return utils.NewToolResultError(fmt.Sprintf("failed to list projects for owner '%s': not found as user or organization", owner)), nil, nil
	}

	response := map[string]any{
		"projects": minimalProjects,
		"note":     "Results include both user and org projects. Each project includes 'owner_type' field. Pagination is limited when owner_type is not specified - specify 'owner_type' for full pagination support.",
	}
	if resp != nil {
		response["pageInfo"] = buildPageInfo(resp)
		defer func() { _ = resp.Body.Close() }()
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

func listProjectFields(ctx context.Context, client *github.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, any, error) {
	projectNumber, err := RequiredInt(args, "project_number")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	pagination, err := extractPaginationOptionsFromArgs(args)
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
}

func listProjectItems(ctx context.Context, client *github.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, any, error) {
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
}

func getProject(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int) (*mcp.CallToolResult, any, error) {
	var resp *github.Response
	var project *github.ProjectV2
	var err error

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
}

func getProjectField(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, fieldID int64) (*mcp.CallToolResult, any, error) {
	var resp *github.Response
	var projectField *github.ProjectV2Field
	var err error

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
}

func getProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64, fields []int64) (*mcp.CallToolResult, any, error) {
	var resp *github.Response
	var projectItem *github.ProjectV2Item
	var opts *github.GetProjectItemOptions
	var err error

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

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get project item", resp, body), nil, nil
	}

	r, err := json.Marshal(projectItem)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func updateProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64, fieldValue map[string]any) (*mcp.CallToolResult, any, error) {
	updatePayload, err := buildUpdateProjectItem(fieldValue)
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
}

func deleteProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64) (*mcp.CallToolResult, any, error) {
	var resp *github.Response
	var err error

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
}

// addProjectItem adds an item to a project by resolving the issue/PR number to a node ID
func addProjectItem(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, itemOwner, itemRepo string, itemNumber int, itemType string) (*mcp.CallToolResult, any, error) {
	if itemType != "issue" && itemType != "pull_request" {
		return utils.NewToolResultError("item_type must be either 'issue' or 'pull_request'"), nil, nil
	}

	// Resolve the item number to a node ID
	var nodeID githubv4.ID
	var err error
	if itemType == "issue" {
		nodeID, err = resolveIssueNodeID(ctx, gqlClient, itemOwner, itemRepo, itemNumber)
	} else {
		nodeID, err = resolvePullRequestNodeID(ctx, gqlClient, itemOwner, itemRepo, itemNumber)
	}
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to resolve %s: %v", itemType, err)), nil, nil
	}

	// Use GraphQL to add the item to the project
	var mutation struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID githubv4.ID
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	// First, get the project ID
	var projectIDQuery struct {
		User struct {
			ProjectV2 struct {
				ID githubv4.ID
			} `graphql:"projectV2(number: $projectNumber)"`
		} `graphql:"user(login: $owner)"`
	}
	var projectIDQueryOrg struct {
		Organization struct {
			ProjectV2 struct {
				ID githubv4.ID
			} `graphql:"projectV2(number: $projectNumber)"`
		} `graphql:"organization(login: $owner)"`
	}

	var projectID githubv4.ID
	if ownerType == "org" {
		err = gqlClient.Query(ctx, &projectIDQueryOrg, map[string]any{
			"owner":         githubv4.String(owner),
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small integers
		})
		if err != nil {
			return utils.NewToolResultError(fmt.Sprintf("failed to get project ID: %v", err)), nil, nil
		}
		projectID = projectIDQueryOrg.Organization.ProjectV2.ID
	} else {
		err = gqlClient.Query(ctx, &projectIDQuery, map[string]any{
			"owner":         githubv4.String(owner),
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small integers
		})
		if err != nil {
			return utils.NewToolResultError(fmt.Sprintf("failed to get project ID: %v", err)), nil, nil
		}
		projectID = projectIDQuery.User.ProjectV2.ID
	}

	// Add the item to the project
	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: projectID,
		ContentID: nodeID,
	}

	err = gqlClient.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf(ProjectAddFailedError+": %v", err)), nil, nil
	}

	result := map[string]any{
		"id":      mutation.AddProjectV2ItemByID.Item.ID,
		"message": fmt.Sprintf("Successfully added %s %s/%s#%d to project %s/%d", itemType, itemOwner, itemRepo, itemNumber, owner, projectNumber),
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
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

// resolveIssueNodeID resolves an issue number to its GraphQL node ID
func resolveIssueNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueNumber int) (githubv4.ID, error) {
	var query struct {
		Repository struct {
			Issue struct {
				ID githubv4.ID
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]any{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(int32(issueNumber)), //nolint:gosec // Issue numbers are small integers
	}

	err := gqlClient.Query(ctx, &query, variables)
	if err != nil {
		return "", fmt.Errorf("failed to resolve issue %s/%s#%d: %w", owner, repo, issueNumber, err)
	}

	return query.Repository.Issue.ID, nil
}

// resolvePullRequestNodeID resolves a pull request number to its GraphQL node ID
func resolvePullRequestNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, prNumber int) (githubv4.ID, error) {
	var query struct {
		Repository struct {
			PullRequest struct {
				ID githubv4.ID
			} `graphql:"pullRequest(number: $prNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]any{
		"owner":    githubv4.String(owner),
		"repo":     githubv4.String(repo),
		"prNumber": githubv4.Int(int32(prNumber)), //nolint:gosec // PR numbers are small integers
	}

	err := gqlClient.Query(ctx, &query, variables)
	if err != nil {
		return "", fmt.Errorf("failed to resolve pull request %s/%s#%d: %w", owner, repo, prNumber, err)
	}

	return query.Repository.PullRequest.ID, nil
}

// detectOwnerType attempts to detect the owner type by trying both user and org
// Returns the detected type ("user" or "org") and any error encountered
func detectOwnerType(ctx context.Context, client *github.Client, owner string, projectNumber int) (string, error) {
	// Try user first (more common for personal projects)
	_, resp, err := client.Projects.GetUserProject(ctx, owner, projectNumber)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return "user", nil
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	// If not found (404) or other error, try org
	_, resp, err = client.Projects.GetOrganizationProject(ctx, owner, projectNumber)
	if err == nil && resp.StatusCode == http.StatusOK {
		_ = resp.Body.Close()
		return "org", nil
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	return "", fmt.Errorf("could not determine owner type for %s with project %d: owner is neither a user nor an org with this project", owner, projectNumber)
}
