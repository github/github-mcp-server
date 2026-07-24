package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

const (
	ProjectUpdateFailedError             = "failed to update a project item"
	ProjectAddFailedError                = "failed to add a project item"
	ProjectDeleteFailedError             = "failed to delete a project item"
	ProjectListFailedError               = "failed to list project items"
	ProjectStatusUpdateListFailedError   = "failed to list project status updates"
	ProjectStatusUpdateGetFailedError    = "failed to get project status update"
	ProjectStatusUpdateCreateFailedError = "failed to create project status update"
	ProjectResolveIDFailedError          = "failed to resolve project ID"
	MaxProjectsPerPage                   = 50
	maxProjectItemsPerBatch              = 50
)

// Method constants for consolidated project tools
const (
	projectsMethodListProjects              = "list_projects"
	projectsMethodListProjectFields         = "list_project_fields"
	projectsMethodListProjectItems          = "list_project_items"
	projectsMethodGetProject                = "get_project"
	projectsMethodGetProjectField           = "get_project_field"
	projectsMethodGetProjectItem            = "get_project_item"
	projectsMethodAddProjectItem            = "add_project_item"
	projectsMethodUpdateProjectItem         = "update_project_item"
	projectsMethodUpdateProjectItems        = "update_project_items"
	projectsMethodDeleteProjectItem         = "delete_project_item"
	projectsMethodListProjectStatusUpdates  = "list_project_status_updates"
	projectsMethodGetProjectStatusUpdate    = "get_project_status_update"
	projectsMethodCreateProjectStatusUpdate = "create_project_status_update"
	projectsMethodCreateProject             = "create_project"
	projectsMethodCreateIterationField      = "create_iteration_field"
)

// GraphQL types for ProjectV2 status updates

type statusUpdateNode struct {
	ID         githubv4.ID
	Body       *githubv4.String
	Status     *githubv4.String
	CreatedAt  githubv4.DateTime
	StartDate  *githubv4.String
	TargetDate *githubv4.String
	Creator    struct {
		Login githubv4.String
	}
}

type projectVisibility struct {
	Public githubv4.Boolean
}

type statusUpdateNodeWithProject struct {
	statusUpdateNode
	Project projectVisibility
}

type statusUpdateConnection struct {
	Nodes    []statusUpdateNode
	PageInfo PageInfoFragment
}

type statusUpdatesProject struct {
	Public        githubv4.Boolean
	StatusUpdates statusUpdateConnection `graphql:"statusUpdates(first: $first, after: $after, orderBy: {field: CREATED_AT, direction: DESC})"`
}

// statusUpdatesUserQuery is the GraphQL query for listing status updates on a user-owned project.
type statusUpdatesUserQuery struct {
	User struct {
		ProjectV2 statusUpdatesProject `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"user(login: $owner)"`
}

// statusUpdatesOrgQuery is the GraphQL query for listing status updates on an org-owned project.
type statusUpdatesOrgQuery struct {
	Organization struct {
		ProjectV2 statusUpdatesProject `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $owner)"`
}

// statusUpdateNodeQuery is the GraphQL query for fetching a single status update by node ID.
type statusUpdateNodeQuery struct {
	Node struct {
		StatusUpdate statusUpdateNodeWithProject `graphql:"... on ProjectV2StatusUpdate"`
	} `graphql:"node(id: $id)"`
}

// CreateProjectV2StatusUpdateInput is the input for the createProjectV2StatusUpdate mutation.
// Defined locally because the shurcooL/githubv4 library does not include this type.
type CreateProjectV2StatusUpdateInput struct {
	ProjectID        githubv4.ID      `json:"projectId"`
	Body             *githubv4.String `json:"body,omitempty"`
	Status           *githubv4.String `json:"status,omitempty"`
	StartDate        *githubv4.String `json:"startDate,omitempty"`
	TargetDate       *githubv4.String `json:"targetDate,omitempty"`
	ClientMutationID *githubv4.String `json:"clientMutationId,omitempty"`
}

// validProjectV2StatusUpdateStatuses is the set of valid status values for the createProjectV2StatusUpdate mutation.
var validProjectV2StatusUpdateStatuses = map[string]bool{
	"INACTIVE":  true,
	"ON_TRACK":  true,
	"AT_RISK":   true,
	"OFF_TRACK": true,
	"COMPLETE":  true,
}

func convertToMinimalStatusUpdate(node statusUpdateNode) MinimalProjectStatusUpdate {
	var creator *MinimalUser
	if login := string(node.Creator.Login); login != "" {
		creator = &MinimalUser{Login: login}
	}

	return MinimalProjectStatusUpdate{
		ID:         fmt.Sprintf("%v", node.ID),
		Body:       derefString(node.Body),
		Status:     derefString(node.Status),
		CreatedAt:  node.CreatedAt.Time.Format(time.RFC3339),
		StartDate:  derefString(node.StartDate),
		TargetDate: derefString(node.TargetDate),
		Creator:    creator,
	}
}

func derefString(s *githubv4.String) string {
	if s == nil {
		return ""
	}
	return string(*s)
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
							projectsMethodListProjectStatusUpdates,
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
						Description: "The project's number. Required for 'list_project_fields', 'list_project_items', and 'list_project_status_updates' methods.",
					},
					"query": {
						Type:        "string",
						Description: `Filter/query string. For list_projects: filter by title text and state (e.g. "roadmap is:open"). For list_project_items: advanced filtering using GitHub's project filtering syntax.`,
					},
					"fields": {
						Type:        "array",
						Description: "Field IDs to include when listing project items (e.g. [\"102589\", \"985201\"]). CRITICAL: Always provide to get field values. Without this (and without 'field_names'), only titles returned. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'list_project_items' method.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"field_names": {
						Type:        "array",
						Description: "Field names to include when listing project items (e.g. [\"Status\", \"Priority\"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Names that fail to resolve return a structured error. Mutually exclusive with 'fields' — provide one, not both. Only used for 'list_project_items' method.",
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
				result, visibilities, payload, err := listProjects(ctx, client, args, owner, ownerType)
				result = attachJoinedIFCLabel(ctx, deps, result, visibilities, ifc.LabelProjectList)
				return result, payload, err
			case projectsMethodListProjectFields, projectsMethodListProjectItems, projectsMethodListProjectStatusUpdates:
				// All other methods require project_number and ownerType detection
				projectNumber, err := RequiredInt(args, "project_number")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if ownerType == "" {
					ownerType, err = detectOwnerType(ctx, client, owner, projectNumber)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
				}

				switch method {
				case projectsMethodListProjectFields:
					result, payload, err := listProjectFields(ctx, client, args, owner, ownerType)
					if shouldAttachIFCLabel(ctx, deps, result) {
						isPrivate, visibilityErr := FetchProjectIsPrivate(ctx, client, owner, ownerType, projectNumber)
						if visibilityErr == nil {
							result = attachProjectVisibilityIFCLabel(ctx, deps, result, isPrivate, ifc.LabelProject)
						}
					}
					return result, payload, err
				case projectsMethodListProjectItems:
					gqlClient, gqlErr := deps.GetGQLClient(ctx)
					if gqlErr != nil {
						return utils.NewToolResultError(gqlErr.Error()), nil, nil
					}
					result, payload, err := listProjectItems(ctx, client, gqlClient, args, owner, ownerType)
					if shouldAttachIFCLabel(ctx, deps, result) {
						isPrivate, visibilityErr := FetchProjectIsPrivate(ctx, client, owner, ownerType, projectNumber)
						if visibilityErr == nil {
							result = attachProjectVisibilityIFCLabel(ctx, deps, result, isPrivate, ifc.LabelProjectContent)
						}
					}
					return result, payload, err
				case projectsMethodListProjectStatusUpdates:
					gqlClient, err := deps.GetGQLClient(ctx)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					result, isPrivate, payload, err := listProjectStatusUpdates(ctx, gqlClient, args, owner, ownerType)
					result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelProjectContent(isPrivate))
					return result, payload, err
				default:
					return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
				}
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
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
							projectsMethodGetProjectStatusUpdate,
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
						Description: "Specific list of field IDs to include in the response when getting a project item (e.g. [\"102589\", \"985201\", \"169875\"]). If neither 'fields' nor 'field_names' is provided, only the title field is included. Mutually exclusive with 'field_names' — provide one, not both. Only used for 'get_project_item' method.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"field_names": {
						Type:        "array",
						Description: "Specific list of field names to include in the response when getting a project item (e.g. [\"Status\", \"Priority\"]). Resolved server-side to field IDs — pass this instead of 'fields' when you only know the human-readable names. Mutually exclusive with 'fields' — provide one, not both. Only used for 'get_project_item' method.",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"status_update_id": {
						Type:        "string",
						Description: "The node ID of the project status update. Required for 'get_project_status_update' method.",
					},
				},
				Required: []string{"method"},
			},
		},
		[]scopes.Scope{scopes.ReadProject},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Handle get_project_status_update early — it only needs status_update_id
			if method == projectsMethodGetProjectStatusUpdate {
				statusUpdateID, err := RequiredParam[string](args, "status_update_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, isPrivate, payload, err := getProjectStatusUpdate(ctx, gqlClient, statusUpdateID)
				result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelProjectContent(isPrivate))
				return result, payload, err
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
				result, isPrivate, payload, err := getProject(ctx, client, owner, ownerType, projectNumber)
				result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelProject(isPrivate))
				return result, payload, err
			case projectsMethodGetProjectField:
				fieldID, err := RequiredBigInt(args, "field_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, payload, err := getProjectField(ctx, client, owner, ownerType, projectNumber, fieldID)
				if shouldAttachIFCLabel(ctx, deps, result) {
					isPrivate, visibilityErr := FetchProjectIsPrivate(ctx, client, owner, ownerType, projectNumber)
					if visibilityErr == nil {
						result = attachProjectVisibilityIFCLabel(ctx, deps, result, isPrivate, ifc.LabelProject)
					}
				}
				return result, payload, err
			case projectsMethodGetProjectItem:
				itemID, err := RequiredBigInt(args, "item_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				fields, err := OptionalBigIntArrayParam(args, "fields")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				fieldNames, err := OptionalStringArrayParam(args, "field_names")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if len(fields) > 0 && len(fieldNames) > 0 {
					return utils.NewToolResultError("provide either 'fields' or 'field_names', not both"), nil, nil
				}
				if len(fieldNames) > 0 {
					gqlClient, gqlErr := deps.GetGQLClient(ctx)
					if gqlErr != nil {
						return utils.NewToolResultError(gqlErr.Error()), nil, nil
					}
					resolvedIDs, resolveErr := resolveFieldNamesToIDs(ctx, gqlClient, owner, ownerType, projectNumber, fieldNames)
					if resolveErr != nil {
						var structured *ghErrors.StructuredResolutionError
						if errors.As(resolveErr, &structured) {
							return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
						}
						return utils.NewToolResultError(resolveErr.Error()), nil, nil
					}
					fields = append(fields, resolvedIDs...)
				}
				result, payload, err := getProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fields)
				if shouldAttachIFCLabel(ctx, deps, result) {
					isPrivate, visibilityErr := FetchProjectIsPrivate(ctx, client, owner, ownerType, projectNumber)
					if visibilityErr == nil {
						result = attachProjectVisibilityIFCLabel(ctx, deps, result, isPrivate, ifc.LabelProjectContent)
					}
				}
				return result, payload, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	return tool
}

func updateProjectItemsItemSchema() *jsonschema.Schema {
	variant := func(required []string, properties map[string]*jsonschema.Schema) *jsonschema.Schema {
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           properties,
			Required:             required,
		}
	}

	return &jsonschema.Schema{
		Type: "object",
		OneOf: []*jsonschema.Schema{
			variant([]string{"node_id"}, map[string]*jsonschema.Schema{
				"node_id": {
					Type:        "string",
					Description: "The project item's GraphQL node ID, as returned by 'list_project_items' or 'add_project_item'.",
				},
			}),
			variant([]string{"item_id"}, map[string]*jsonschema.Schema{
				"item_id": {
					Type:        "integer",
					Description: "The numeric project item ID.",
				},
			}),
			variant([]string{"item_owner", "item_repo", "issue_number"}, map[string]*jsonschema.Schema{
				"item_owner": {
					Type:        "string",
					Description: "Owner of the repository containing the issue.",
				},
				"item_repo": {
					Type:        "string",
					Description: "Repository containing the issue.",
				},
				"issue_number": {
					Type:        "integer",
					Description: "Issue number used to resolve the project item.",
				},
			}),
		},
	}
}

func projectUpdatedFieldSchema() *jsonschema.Schema {
	variant := func(required []string, properties map[string]*jsonschema.Schema, valueDescription string) *jsonschema.Schema {
		properties["value"] = &jsonschema.Schema{Description: valueDescription}
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           properties,
			Required:             required,
		}
	}

	return &jsonschema.Schema{
		Type:        "object",
		Description: "The field to update and its new value. Required for 'update_project_item' and 'update_project_items'. For 'update_project_items', one top-level field/value applies to every item. Set value to null to clear the field.",
		OneOf: []*jsonschema.Schema{
			variant([]string{"id", "value"}, map[string]*jsonschema.Schema{
				"id": {
					Type:        "integer",
					Description: "The numeric ID of a standard Project field. Attached Issue Fields must be updated by name with singular 'update_project_item'.",
				},
			}, "The new value, or null to clear the field. For SINGLE_SELECT, use the option ID."),
			variant([]string{"name", "value"}, map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "The case-insensitive field name. Supports standard Project fields. Singular 'update_project_item' also supports attached Issue Fields; 'update_project_items' does not.",
				},
			}, "The new value, or null to clear the field. Use a string for TEXT, a finite number for NUMBER, a YYYY-MM-DD string for DATE, and an option name (case-insensitive) or option ID for SINGLE_SELECT."),
		},
	}
}

// ProjectsWrite returns the tool and handler for modifying GitHub Projects resources.
func ProjectsWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "projects_write",
			Description: t("TOOL_PROJECTS_WRITE_DESCRIPTION", "Create and manage GitHub Projects: create projects, add/update/delete items, bulk-update many items at once, create status updates, and add iteration fields."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_PROJECTS_WRITE_USER_TITLE", "Manage GitHub Projects"),
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
							projectsMethodUpdateProjectItems,
							projectsMethodDeleteProjectItem,
							projectsMethodCreateProjectStatusUpdate,
							projectsMethodCreateProject,
							projectsMethodCreateIterationField,
						},
					},
					"owner_type": {
						Type:        "string",
						Description: "Owner type (user or org). Required for 'create_project' method. If not provided for other methods, will be automatically detected.",
						Enum:        []any{"user", "org"},
					},
					"owner": {
						Type:        "string",
						Description: "The project owner (user or organization login). The name is not case sensitive.",
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number. Required for all methods except 'create_project'.",
					},
					"title": {
						Type:        "string",
						Description: "The project title. Required for 'create_project' method.",
					},
					"item_id": {
						Type:        "number",
						Description: "The project item ID. Required for 'delete_project_item'. For 'update_project_item', provide either item_id, or (item_owner + item_repo + issue_number) to resolve the item by issue.",
					},
					"item_type": {
						Type:        "string",
						Description: "The item's type, either issue or pull_request. Required for 'add_project_item' method.",
						Enum:        []any{"issue", "pull_request"},
					},
					"item_owner": {
						Type:        "string",
						Description: "The owner (user or organization) of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number.",
					},
					"item_repo": {
						Type:        "string",
						Description: "The name of the repository containing the issue or pull request. Required for 'add_project_item' method. Also accepted by 'update_project_item' when resolving the item by issue number.",
					},
					"issue_number": {
						Type:        "number",
						Description: "The issue number. Required for 'add_project_item' when item_type is 'issue'. Also accepted by 'update_project_item' to resolve the item by issue number (combine with item_owner and item_repo).",
					},
					"pull_request_number": {
						Type:        "number",
						Description: "The pull request number (use when item_type is 'pull_request' for 'add_project_item' method). Provide either issue_number or pull_request_number.",
					},
					"updated_field": projectUpdatedFieldSchema(),
					"items": {
						Type:        "array",
						Description: "The items to update with the top-level 'updated_field'. Required for 'update_project_items'; prefer it over calling 'update_project_item' in a loop. Each entry must match exactly one reference variant: 'node_id', numeric 'item_id', or 'item_owner' + 'item_repo' + 'issue_number'. Limit: " + strconv.Itoa(maxProjectItemsPerBatch) + " items per call.",
						Items:       updateProjectItemsItemSchema(),
					},
					"body": {
						Type:        "string",
						Description: "The body of the status update (markdown). Used for 'create_project_status_update' method.",
					},
					"status": {
						Type:        "string",
						Description: "The status of the project. Used for 'create_project_status_update' method.",
						Enum:        []any{"INACTIVE", "ON_TRACK", "AT_RISK", "OFF_TRACK", "COMPLETE"},
					},
					"start_date": {
						Type:        "string",
						Description: "Start date in YYYY-MM-DD format. Used for 'create_project_status_update' and 'create_iteration_field' methods.",
					},
					"target_date": {
						Type:        "string",
						Description: "The target date of the status update in YYYY-MM-DD format. Used for 'create_project_status_update' method.",
					},
					"field_name": {
						Type:        "string",
						Description: "The name of the iteration field (e.g. 'Sprint'). Required for 'create_iteration_field' method.",
					},
					"iteration_duration": {
						Type:        "number",
						Description: "Duration in days for iterations of the field (e.g. 7 for weekly, 14 for bi-weekly). Required for 'create_iteration_field' method.",
					},
					"iterations": {
						Type:        "array",
						Description: "Custom iterations for 'create_iteration_field' method. Only set this when you need iterations with varying durations, breaks between them, or specific titles. Otherwise omit it: GitHub auto-creates three iterations of 'iteration_duration' days starting on 'start_date', which is the right choice for most cases.",
						Items: &jsonschema.Schema{
							Type:                 "object",
							AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
							Properties: map[string]*jsonschema.Schema{
								"title": {
									Type:        "string",
									Description: "Iteration title (e.g. 'Sprint 1')",
								},
								"start_date": {
									Type:        "string",
									Description: "Start date in YYYY-MM-DD format",
								},
								"duration": {
									Type:        "number",
									Description: "Duration in days",
								},
							},
							Required: []string{"title", "start_date", "duration"},
						},
					},
				},
				Required: []string{"method", "owner"},
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

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// create_project does not require project_number or a REST client
			if method == projectsMethodCreateProject {
				return createProject(ctx, gqlClient, owner, ownerType, args)
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
				var itemID int64
				if _, hasItemID := args["item_id"]; hasItemID {
					id, err := RequiredBigInt(args, "item_id")
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					itemID = id
				} else {
					// Resolve the item by (item_owner, item_repo, issue_number).
					resolvedItemID, resolveErr := resolveItemIDFromIssueArgs(ctx, gqlClient, owner, ownerType, projectNumber, args)
					if resolveErr != nil {
						var structured *ghErrors.StructuredResolutionError
						if errors.As(resolveErr, &structured) {
							return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
						}
						return utils.NewToolResultError(resolveErr.Error()), nil, nil
					}
					itemID = resolvedItemID
				}

				rawUpdatedField, exists := args["updated_field"]
				if !exists {
					return utils.NewToolResultError("missing required parameter: updated_field"), nil, nil
				}
				fieldValue, ok := rawUpdatedField.(map[string]any)
				if !ok || fieldValue == nil {
					return utils.NewToolResultError("updated_field must be an object"), nil, nil
				}
				return updateProjectItem(ctx, client, gqlClient, owner, ownerType, projectNumber, itemID, fieldValue)
			case projectsMethodUpdateProjectItems:
				return updateProjectItemsBatch(ctx, client, gqlClient, owner, ownerType, projectNumber, args)
			case projectsMethodDeleteProjectItem:
				itemID, err := RequiredBigInt(args, "item_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return deleteProjectItem(ctx, client, owner, ownerType, projectNumber, itemID)
			case projectsMethodCreateProjectStatusUpdate:
				body, err := OptionalParam[string](args, "body")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				status, err := OptionalParam[string](args, "status")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				startDate, err := OptionalParam[string](args, "start_date")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				targetDate, err := OptionalParam[string](args, "target_date")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return createProjectStatusUpdate(ctx, gqlClient, owner, ownerType, projectNumber, body, status, startDate, targetDate)
			case projectsMethodCreateIterationField:
				return createIterationField(ctx, gqlClient, owner, ownerType, projectNumber, args)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	return tool
}

// Helper functions for consolidated projects tools

func listProjects(ctx context.Context, client *github.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, []bool, any, error) {
	queryStr, err := OptionalParam[string](args, "query")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil, nil
	}

	pagination, err := extractPaginationOptionsFromArgs(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil, nil
	}

	var resp *github.Response
	var projects []*github.ProjectV2

	minimalProjects := []MinimalProject{}
	opts := &github.ListProjectsOptions{
		ListProjectsPaginationOptions: pagination,
		Query:                         queryStr,
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
			), nil, nil, nil
		}
	default:
		projects, resp, err = client.Projects.ListUserProjects(ctx, owner, opts)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx,
				"failed to list projects",
				resp,
				err,
			), nil, nil, nil
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
			return nil, nil, nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		return utils.NewToolResultText(string(r)), projectVisibilities(minimalProjects), nil, nil
	}

	return nil, nil, nil, fmt.Errorf("unexpected state in listProjects")
}

// listProjectsFromBothOwnerTypes fetches projects from both user and org endpoints
// when owner_type is not specified, combining the results with owner_type labels.
func listProjectsFromBothOwnerTypes(ctx context.Context, client *github.Client, owner string, opts *github.ListProjectsOptions) (*mcp.CallToolResult, []bool, any, error) {
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
		return utils.NewToolResultError(fmt.Sprintf("failed to list projects for owner '%s': not found as user or organization", owner)), nil, nil, nil
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
		return nil, nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), projectVisibilities(minimalProjects), nil, nil
}

func projectVisibilities(projects []MinimalProject) []bool {
	visibilities := make([]bool, 0, len(projects))
	for _, project := range projects {
		isPrivate := true
		if project.Public != nil {
			isPrivate = !*project.Public
		}
		visibilities = append(visibilities, isPrivate)
	}
	return visibilities
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

func listProjectItems(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, any, error) {
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

	fieldNames, err := OptionalStringArrayParam(args, "field_names")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	if len(fields) > 0 && len(fieldNames) > 0 {
		return utils.NewToolResultError("provide either 'fields' or 'field_names', not both"), nil, nil
	}
	if len(fieldNames) > 0 {
		resolvedIDs, resolveErr := resolveFieldNamesToIDs(ctx, gqlClient, owner, ownerType, projectNumber, fieldNames)
		if resolveErr != nil {
			var structured *ghErrors.StructuredResolutionError
			if errors.As(resolveErr, &structured) {
				return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
			}
			return utils.NewToolResultError(resolveErr.Error()), nil, nil
		}
		fields = append(fields, resolvedIDs...)
	}

	pagination, err := extractPaginationOptionsFromArgs(args)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	var resp *github.Response
	var projectItems []*github.ProjectV2Item

	opts := &github.ListProjectItemsOptions{
		Fields: fields,
		ListProjectsOptions: github.ListProjectsOptions{
			ListProjectsPaginationOptions: pagination,
			Query:                         queryStr,
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

	minimalItems := make([]MinimalProjectItem, 0, len(projectItems))
	for _, item := range projectItems {
		minimalItems = append(minimalItems, convertToMinimalProjectItem(item))
	}

	response := map[string]any{
		"items":    minimalItems,
		"pageInfo": buildPageInfo(resp),
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func fetchProjectV2(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int) (*github.ProjectV2, *github.Response, error) {
	if ownerType == "org" {
		return client.Projects.GetOrganizationProject(ctx, owner, projectNumber)
	}
	return client.Projects.GetUserProject(ctx, owner, projectNumber)
}

// FetchProjectIsPrivate returns whether a GitHub Project is private.
func FetchProjectIsPrivate(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int) (bool, error) {
	project, resp, err := fetchProjectV2(ctx, client, owner, ownerType, projectNumber)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return false, err
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to fetch project visibility")
	}
	return !project.GetPublic(), nil
}

func getProject(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int) (*mcp.CallToolResult, bool, any, error) {
	project, resp, err := fetchProjectV2(ctx, client, owner, ownerType, projectNumber)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to get project",
			resp,
			err,
		), false, nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get project", resp, body), false, nil, nil
	}

	minimalProject := convertToMinimalProject(project)
	r, err := json.Marshal(minimalProject)
	if err != nil {
		return nil, false, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), !project.GetPublic(), nil, nil
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

func fetchProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64, fields []int64) (*github.ProjectV2Item, *github.Response, error) {
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
	return projectItem, resp, err
}

func getProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64, fields []int64) (*mcp.CallToolResult, any, error) {
	projectItem, resp, err := fetchProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fields)

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

	r, err := json.Marshal(convertToMinimalProjectItem(projectItem))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func updateProjectItem(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, itemID int64, fieldValue map[string]any) (*mcp.CallToolResult, any, error) {
	update, err := buildUpdateProjectItem(ctx, gqlClient, owner, ownerType, projectNumber, fieldValue)
	if err != nil {
		var structured *ghErrors.StructuredResolutionError
		if errors.As(err, &structured) {
			return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
		}
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	if update.IssueField != nil {
		projectItem, resp, fetchErr := fetchProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, nil)
		if fetchErr != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to inspect project item content", resp, fetchErr), nil, nil
		}
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if resp == nil || resp.StatusCode != http.StatusOK {
			return utils.NewToolResultError("failed to inspect project item content"), nil, nil
		}

		issueID, resolveErr := projectItemIssueNodeID(projectItem)
		if resolveErr != nil {
			var structured *ghErrors.StructuredResolutionError
			if errors.As(resolveErr, &structured) {
				return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
			}
			return utils.NewToolResultError(resolveErr.Error()), nil, nil
		}
		response, mutationErr := SetIssueFieldValues(ctx, gqlClient, issueID, []IssueFieldCreateOrUpdateInput{*update.IssueField})
		if mutationErr != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to update Issue Field", mutationErr), nil, nil
		}
		r, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			return nil, nil, fmt.Errorf("failed to marshal response: %w", marshalErr)
		}
		return utils.NewToolResultText(string(r)), nil, nil
	}

	var resp *github.Response
	var updatedItem *github.ProjectV2Item

	if ownerType == "org" {
		updatedItem, resp, err = client.Projects.UpdateOrganizationProjectItem(ctx, owner, projectNumber, itemID, update.Project)
	} else {
		updatedItem, resp, err = client.Projects.UpdateUserProjectItem(ctx, owner, projectNumber, itemID, update.Project)
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
	r, err := json.Marshal(convertToMinimalProjectItem(updatedItem))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func projectItemIssueNodeID(item *github.ProjectV2Item) (githubv4.ID, error) {
	if item == nil || item.ContentType == nil {
		return "", ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			"",
			"the project item response did not identify its content type; Issue Fields can only be updated on Issue items",
			nil,
		)
	}

	contentType := string(*item.ContentType)
	if contentType != "Issue" {
		return "", ghErrors.NewStructuredResolutionError(
			"unsupported_item_type",
			contentType,
			"Issue Fields can only be updated on Issue project items, not pull requests or draft issues",
			nil,
		)
	}
	if item.Content == nil || item.Content.Issue == nil || item.Content.Issue.GetNodeID() == "" {
		return "", ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			"Issue",
			"the project item response did not include the underlying Issue node ID needed to update the Issue Field",
			nil,
		)
	}
	return githubv4.ID(item.Content.Issue.GetNodeID()), nil
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

// resolveProjectNodeID resolves (owner, ownerType, projectNumber) to a project node ID via GraphQL.
func resolveProjectNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int) (githubv4.ID, error) {
	var projectIDQueryUser struct {
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

	queryVars := map[string]any{
		"owner":         githubv4.String(owner),
		"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small integers
	}

	if ownerType == "org" {
		err := gqlClient.Query(ctx, &projectIDQueryOrg, queryVars)
		if err != nil {
			return "", fmt.Errorf("%s: %w", ProjectResolveIDFailedError, err)
		}
		return projectIDQueryOrg.Organization.ProjectV2.ID, nil
	}

	err := gqlClient.Query(ctx, &projectIDQueryUser, queryVars)
	if err != nil {
		return "", fmt.Errorf("%s: %w", ProjectResolveIDFailedError, err)
	}
	return projectIDQueryUser.User.ProjectV2.ID, nil
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
				ID             githubv4.ID
				FullDatabaseID string `graphql:"fullDatabaseId"`
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	// Resolve the project number to a node ID
	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
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
	if fullDatabaseID := mutation.AddProjectV2ItemByID.Item.FullDatabaseID; fullDatabaseID != "" {
		result["full_database_id"] = fullDatabaseID
		if itemID, err := strconv.ParseInt(fullDatabaseID, 10, 64); err == nil {
			result["item_id"] = itemID
		}
	}

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

// validateDateFormat checks that a date string is in YYYY-MM-DD format.
func validateDateFormat(value, fieldName string) error {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("invalid %s %q: must be YYYY-MM-DD format", fieldName, value)
	}
	return nil
}

// createProjectStatusUpdate creates a new status update for a project via GraphQL.
func createProjectStatusUpdate(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, body, status, startDate, targetDate string) (*mcp.CallToolResult, any, error) {
	// Validate inputs
	if ownerType != "user" && ownerType != "org" {
		return utils.NewToolResultError(fmt.Sprintf("invalid owner_type %q: must be \"user\" or \"org\"", ownerType)), nil, nil
	}
	if status != "" && !validProjectV2StatusUpdateStatuses[status] {
		return utils.NewToolResultError(fmt.Sprintf("invalid status %q: must be one of INACTIVE, ON_TRACK, AT_RISK, OFF_TRACK, COMPLETE", status)), nil, nil
	}
	if startDate != "" {
		if err := validateDateFormat(startDate, "start_date"); err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
	}
	if targetDate != "" {
		if err := validateDateFormat(targetDate, "target_date"); err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
	}

	// Resolve project number to project node ID
	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	// Build mutation input
	input := CreateProjectV2StatusUpdateInput{
		ProjectID: projectID,
	}

	if body != "" {
		s := githubv4.String(body)
		input.Body = &s
	}
	if status != "" {
		s := githubv4.String(status)
		input.Status = &s
	}
	if startDate != "" {
		s := githubv4.String(startDate)
		input.StartDate = &s
	}
	if targetDate != "" {
		s := githubv4.String(targetDate)
		input.TargetDate = &s
	}

	// Execute mutation
	var mutation struct {
		CreateProjectV2StatusUpdate struct {
			StatusUpdate statusUpdateNode
		} `graphql:"createProjectV2StatusUpdate(input: $input)"`
	}

	err = gqlClient.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateCreateFailedError, err)), nil, nil
	}

	// Convert and return
	result := convertToMinimalStatusUpdate(mutation.CreateProjectV2StatusUpdate.StatusUpdate)

	r, err := json.Marshal(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

// listProjectStatusUpdates lists status updates for a project via GraphQL.
func listProjectStatusUpdates(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, bool, any, error) {
	if ownerType != "user" && ownerType != "org" {
		return utils.NewToolResultError(fmt.Sprintf("invalid owner_type %q: must be \"user\" or \"org\"", ownerType)), false, nil, nil
	}

	projectNumber, err := RequiredInt(args, "project_number")
	if err != nil {
		return utils.NewToolResultError(err.Error()), false, nil, nil
	}

	perPage, err := OptionalIntParamWithDefault(args, "per_page", MaxProjectsPerPage)
	if err != nil {
		return utils.NewToolResultError(err.Error()), false, nil, nil
	}
	if perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}
	if perPage < 1 {
		perPage = MaxProjectsPerPage
	}

	afterCursor, err := OptionalParam[string](args, "after")
	if err != nil {
		return utils.NewToolResultError(err.Error()), false, nil, nil
	}

	vars := map[string]any{
		"owner":         githubv4.String(owner),
		"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small integers
		"first":         githubv4.Int(int32(perPage)),       //nolint:gosec // perPage is bounded by MaxProjectsPerPage
	}
	if afterCursor != "" {
		vars["after"] = githubv4.String(afterCursor)
	} else {
		vars["after"] = (*githubv4.String)(nil)
	}

	var nodes []statusUpdateNode
	var pi PageInfoFragment
	var isPrivate bool

	if ownerType == "org" {
		var q statusUpdatesOrgQuery
		if err := gqlClient.Query(ctx, &q, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateListFailedError, err)), false, nil, nil
		}
		project := q.Organization.ProjectV2
		nodes = project.StatusUpdates.Nodes
		pi = project.StatusUpdates.PageInfo
		isPrivate = !bool(project.Public)
	} else {
		var q statusUpdatesUserQuery
		if err := gqlClient.Query(ctx, &q, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateListFailedError, err)), false, nil, nil
		}
		project := q.User.ProjectV2
		nodes = project.StatusUpdates.Nodes
		pi = project.StatusUpdates.PageInfo
		isPrivate = !bool(project.Public)
	}

	updates := make([]MinimalProjectStatusUpdate, 0, len(nodes))
	for _, n := range nodes {
		updates = append(updates, convertToMinimalStatusUpdate(n))
	}

	response := map[string]any{
		"statusUpdates": updates,
		"pageInfo": map[string]any{
			"hasNextPage":     pi.HasNextPage,
			"hasPreviousPage": pi.HasPreviousPage,
			"nextCursor":      string(pi.EndCursor),
			"prevCursor":      string(pi.StartCursor),
		},
	}

	r, err := json.Marshal(response)
	if err != nil {
		return nil, false, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), isPrivate, nil, nil
}

// getProjectStatusUpdate fetches a single status update by its node ID via GraphQL.
func getProjectStatusUpdate(ctx context.Context, gqlClient *githubv4.Client, statusUpdateID string) (*mcp.CallToolResult, bool, any, error) {
	var q statusUpdateNodeQuery
	vars := map[string]any{
		"id": githubv4.ID(statusUpdateID),
	}

	if err := gqlClient.Query(ctx, &q, vars); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateGetFailedError, err)), false, nil, nil
	}

	if q.Node.StatusUpdate.ID == nil || q.Node.StatusUpdate.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: node is not a ProjectV2StatusUpdate or was not found", ProjectStatusUpdateGetFailedError)), false, nil, nil
	}

	update := convertToMinimalStatusUpdate(q.Node.StatusUpdate.statusUpdateNode)
	isPrivate := !bool(q.Node.StatusUpdate.Project.Public)

	r, err := json.Marshal(update)
	if err != nil {
		return nil, false, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), isPrivate, nil, nil
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

type resolvedProjectItemUpdate struct {
	Project    *github.UpdateProjectItemOptions
	IssueField *IssueFieldCreateOrUpdateInput
}

// buildUpdateProjectItem resolves the target field and builds the matching Project or Issue Field write.
func buildUpdateProjectItem(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, input map[string]any) (*resolvedProjectItemUpdate, error) {
	if input == nil {
		return nil, fmt.Errorf("updated_field must be an object")
	}

	valueField, hasValue := input["value"]
	if !hasValue {
		return nil, fmt.Errorf("updated_field.value is required")
	}

	idField, hasID := input["id"]
	nameField, hasName := input["name"]

	switch {
	case hasID && hasName:
		return nil, fmt.Errorf("updated_field must set either id or name, not both")
	case !hasID && !hasName:
		return nil, fmt.Errorf("updated_field requires either id or name")
	}

	var (
		fieldID  int64
		resolved *ResolvedField
	)

	if hasID {
		var err error
		fieldID, err = validateAndConvertToInt64(idField)
		if err != nil {
			return nil, fmt.Errorf("updated_field.id: %w", err)
		}
	} else {
		fieldName, ok := nameField.(string)
		if !ok || fieldName == "" {
			return nil, fmt.Errorf("updated_field.name must be a non-empty string")
		}
		if gqlClient == nil {
			return nil, fmt.Errorf("internal error: gqlClient is required to resolve updated_field.name")
		}
		var err error
		resolved, err = resolveProjectFieldByName(ctx, gqlClient, owner, ownerType, projectNumber, fieldName, "")
		if err != nil {
			return nil, err
		}
		if resolved.IsIssueField {
			issueField, buildErr := buildIssueFieldUpdate(resolved, valueField)
			if buildErr != nil {
				return nil, buildErr
			}
			return &resolvedProjectItemUpdate{IssueField: issueField}, nil
		}
		parsedID, parseErr := parseInt64(resolved.ID)
		if parseErr != nil {
			return nil, fmt.Errorf("resolved field %q has non-numeric ID %q; pass updated_field.id directly", resolved.Name, resolved.ID)
		}
		fieldID = parsedID
	}

	// SINGLE_SELECT: resolve option name to ID; pass through if it's already a known option ID.
	if resolved != nil && resolved.DataType == "SINGLE_SELECT" {
		if str, ok := valueField.(string); ok && str != "" {
			if optID, optErr := resolveSingleSelectOptionByName(resolved, str); optErr == nil {
				valueField = optID
			} else {
				// Fall back: if the string is already a known option ID, accept it.
				known := false
				for _, opt := range resolved.Options {
					if opt.ID == str {
						known = true
						break
					}
				}
				if !known {
					return nil, optErr
				}
			}
		}
	}

	payload := &github.UpdateProjectItemOptions{
		Fields: []*github.UpdateProjectV2Field{{
			ID:    fieldID,
			Value: valueField,
		}},
	}

	return &resolvedProjectItemUpdate{Project: payload}, nil
}

func buildIssueFieldUpdate(field *ResolvedField, raw any) (*IssueFieldCreateOrUpdateInput, error) {
	if field == nil || field.IssueFieldNodeID == "" {
		name := ""
		if field != nil {
			name = field.Name
		}
		return nil, ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			name,
			"the attached Project field did not include the underlying Issue Field node ID; refresh field metadata and retry",
			nil,
		)
	}

	input := &IssueFieldCreateOrUpdateInput{FieldID: githubv4.ID(field.IssueFieldNodeID)}
	if raw == nil {
		deleteValue := githubv4.Boolean(true)
		input.Delete = &deleteValue
		return input, nil
	}

	invalidValue := func(hint string) (*IssueFieldCreateOrUpdateInput, error) {
		return nil, ghErrors.NewStructuredResolutionError("invalid_field_value", field.Name, hint, nil)
	}

	switch field.DataType {
	case "TEXT":
		value, ok := raw.(string)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is TEXT; value must be a string or null to clear it", field.Name))
		}
		input.TextValue = githubv4.NewString(githubv4.String(value))
	case "NUMBER":
		value, ok := toFloat64(raw)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is NUMBER; value must be a finite number or null to clear it", field.Name))
		}
		number := githubv4.Float(value)
		input.NumberValue = &number
	case "DATE":
		value, ok := raw.(string)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is DATE; value must be a YYYY-MM-DD string or null to clear it", field.Name))
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return invalidValue(fmt.Sprintf("Issue Field %q is DATE; value %q must use YYYY-MM-DD format", field.Name, value))
		}
		input.DateValue = githubv4.NewString(githubv4.String(value))
	case "SINGLE_SELECT":
		value, ok := raw.(string)
		if !ok || value == "" {
			return invalidValue(fmt.Sprintf("Issue Field %q is SINGLE_SELECT; value must be a non-empty option name or ID, or null to clear it", field.Name))
		}
		optionID, err := resolveSingleSelectOptionByName(field, value)
		if err != nil {
			for _, option := range field.Options {
				if option.ID == value {
					optionID = value
					err = nil
					break
				}
			}
			if err != nil {
				return nil, err
			}
		}
		id := githubv4.ID(optionID)
		input.SingleSelectOptionID = &id
	default:
		return nil, ghErrors.NewStructuredResolutionError(
			"unsupported_field_type",
			field.Name,
			fmt.Sprintf("Issue Field %q has unsupported data type %q; supported types are TEXT, NUMBER, DATE, and SINGLE_SELECT", field.Name, field.DataType),
			nil,
		)
	}
	return input, nil
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
		PerPage: perPage,
		After:   after,
		Before:  before,
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

// createProject handles the create_project method for ProjectsWrite.
func createProject(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, args map[string]any) (*mcp.CallToolResult, any, error) {
	if ownerType == "" {
		return utils.NewToolResultError("owner_type is required for create_project"), nil, nil
	}
	if ownerType != "user" && ownerType != "org" {
		return utils.NewToolResultError(fmt.Sprintf("invalid owner_type %q: must be \"user\" or \"org\"", ownerType)), nil, nil
	}

	title, err := RequiredParam[string](args, "title")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	ownerID, err := getOwnerNodeID(ctx, gqlClient, owner, ownerType)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to get owner ID: %v", err)), nil, nil
	}

	var mutation struct {
		CreateProjectV2 struct {
			ProjectV2 struct {
				ID     string
				Number int
				Title  string
				URL    string
			}
		} `graphql:"createProjectV2(input: $input)"`
	}

	input := githubv4.CreateProjectV2Input{
		OwnerID: githubv4.ID(ownerID),
		Title:   githubv4.String(title),
	}

	err = gqlClient.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to create project: %v", err)), nil, nil
	}

	result := struct {
		ID     string `json:"id"`
		Number int    `json:"number"`
		Title  string `json:"title"`
		URL    string `json:"url"`
	}{
		ID:     mutation.CreateProjectV2.ProjectV2.ID,
		Number: mutation.CreateProjectV2.ProjectV2.Number,
		Title:  mutation.CreateProjectV2.ProjectV2.Title,
		URL:    mutation.CreateProjectV2.ProjectV2.URL,
	}

	return MarshalledTextResult(result), nil, nil
}

// createIterationField handles the create_iteration_field method for ProjectsWrite.
//
// GitHub's GraphQL API requires two mutations to fully configure an iteration field:
//  1. createProjectV2Field creates the field with DataType=ITERATION (no schedule yet).
//  2. updateProjectV2Field sets the start date, duration, and optional named iterations.
//
// If step 2 fails, the field already exists with default settings and can be reconfigured
// by calling this method again (the create will fail with a duplicate-name error, which
// surfaces clearly) or by deleting the field via the GitHub UI.
func createIterationField(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, args map[string]any) (*mcp.CallToolResult, any, error) {
	fieldName, err := RequiredParam[string](args, "field_name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	duration, err := RequiredInt(args, "iteration_duration")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	startDateStr, err := RequiredParam[string](args, "start_date")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to get project ID: %v", err)), nil, nil
	}

	// Step 1: Create the iteration field.
	var createMutation struct {
		CreateProjectV2Field struct {
			ProjectV2Field struct {
				ProjectV2IterationField struct {
					ID   string
					Name string
				} `graphql:"... on ProjectV2IterationField"`
			} `graphql:"projectV2Field"`
		} `graphql:"createProjectV2Field(input: $input)"`
	}

	createInput := githubv4.CreateProjectV2FieldInput{
		ProjectID: githubv4.ID(projectID),
		DataType:  githubv4.ProjectV2CustomFieldType("ITERATION"),
		Name:      githubv4.String(fieldName),
	}

	err = gqlClient.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to create iteration field: %v", err)), nil, nil
	}

	fieldID := createMutation.CreateProjectV2Field.ProjectV2Field.ProjectV2IterationField.ID

	// Step 2: Configure the iteration field with start date and duration.
	var updateMutation struct {
		UpdateProjectV2Field struct {
			ProjectV2Field struct {
				ProjectV2IterationField struct {
					ID            string
					Name          string
					Configuration struct {
						Iterations []struct {
							ID        string
							Title     string
							StartDate string
							Duration  int
						}
					}
				} `graphql:"... on ProjectV2IterationField"`
			} `graphql:"projectV2Field"`
		} `graphql:"updateProjectV2Field(input: $input)"`
	}

	parsedStartDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to parse start_date %s: %v", startDateStr, err)), nil, nil
	}

	// GitHub's ProjectV2IterationFieldConfigurationInput requires `iterations` as a
	// non-null array, so we always send at least an empty slice. When omitted, GitHub
	// generates a default set of iterations from start_date and duration.
	iterationsInput := []ProjectV2IterationFieldIterationInput{}

	if rawIterations, ok := args["iterations"].([]any); ok && len(rawIterations) > 0 {
		for i, item := range rawIterations {
			iterMap, ok := item.(map[string]any)
			if !ok {
				return utils.NewToolResultError(fmt.Sprintf("iterations[%d] must be an object", i)), nil, nil
			}
			iterTitle, ok := iterMap["title"].(string)
			if !ok || iterTitle == "" {
				return utils.NewToolResultError(fmt.Sprintf("iterations[%d]: title is required and must be a non-empty string", i)), nil, nil
			}
			iterStartDate, ok := iterMap["start_date"].(string)
			if !ok || iterStartDate == "" {
				return utils.NewToolResultError(fmt.Sprintf("iterations[%d]: start_date is required and must be a non-empty string", i)), nil, nil
			}
			iterDuration, ok := iterMap["duration"].(float64)
			if !ok || iterDuration <= 0 {
				return utils.NewToolResultError(fmt.Sprintf("iterations[%d]: duration is required and must be a positive number", i)), nil, nil
			}

			parsedIterStartDate, err := time.Parse("2006-01-02", iterStartDate)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("iterations[%d]: failed to parse start_date %q: %v", i, iterStartDate, err)), nil, nil
			}

			iterationsInput = append(iterationsInput, ProjectV2IterationFieldIterationInput{
				Title:     githubv4.String(iterTitle),
				StartDate: githubv4.Date{Time: parsedIterStartDate},
				Duration:  githubv4.Int(int32(iterDuration)), //nolint:gosec // Iteration durations are small day counts
			})
		}
	}

	configInput := ProjectV2IterationFieldConfigurationInput{
		Duration:   githubv4.Int(int32(duration)), //nolint:gosec // Iteration durations are small day counts
		StartDate:  githubv4.Date{Time: parsedStartDate},
		Iterations: iterationsInput,
	}

	updateInput := UpdateProjectV2FieldInput{
		FieldID:                githubv4.ID(fieldID),
		IterationConfiguration: &configInput,
	}

	err = gqlClient.Mutate(ctx, &updateMutation, updateInput, nil)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("failed to update iteration configuration: %v", err)), nil, nil
	}

	field := updateMutation.UpdateProjectV2Field.ProjectV2Field.ProjectV2IterationField
	iterResults := make([]map[string]any, 0, len(field.Configuration.Iterations))
	for _, iter := range field.Configuration.Iterations {
		iterResults = append(iterResults, map[string]any{
			"id":         iter.ID,
			"title":      iter.Title,
			"start_date": iter.StartDate,
			"duration":   iter.Duration,
		})
	}

	result := map[string]any{
		"id":   field.ID,
		"name": field.Name,
		"configuration": map[string]any{
			"iterations": iterResults,
		},
	}

	return MarshalledTextResult(result), nil, nil
}

// getOwnerNodeID resolves a GitHub user or organization login to its GraphQL node ID.
func getOwnerNodeID(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string) (string, error) {
	if ownerType == "org" {
		var query struct {
			Organization struct {
				ID string
			} `graphql:"organization(login: $login)"`
		}
		variables := map[string]any{
			"login": githubv4.String(owner),
		}
		err := gqlClient.Query(ctx, &query, variables)
		return query.Organization.ID, err
	}

	var query struct {
		User struct {
			ID string
		} `graphql:"user(login: $login)"`
	}
	variables := map[string]any{
		"login": githubv4.String(owner),
	}
	err := gqlClient.Query(ctx, &query, variables)
	return query.User.ID, err
}

// UpdateProjectV2FieldInput is the GraphQL input for the updateProjectV2Field mutation.
// These types are defined locally because the pinned shurcooL/githubv4 release
// (v0.0.0-20240727222349) does not yet expose them. Upstream master now generates
// equivalent types, so this block can be removed when the dependency is next bumped.
type UpdateProjectV2FieldInput struct {
	FieldID                githubv4.ID                                `json:"fieldId"`
	IterationConfiguration *ProjectV2IterationFieldConfigurationInput `json:"iterationConfiguration,omitempty"`
}

// ProjectV2IterationFieldConfigurationInput is the GraphQL input for configuring an iteration field.
// GitHub's schema marks iterations as a required non-null list, so the field is not omitempty.
type ProjectV2IterationFieldConfigurationInput struct {
	Duration   githubv4.Int                            `json:"duration"`
	StartDate  githubv4.Date                           `json:"startDate"`
	Iterations []ProjectV2IterationFieldIterationInput `json:"iterations"`
}

// ProjectV2IterationFieldIterationInput is the GraphQL input for a single iteration definition.
type ProjectV2IterationFieldIterationInput struct {
	StartDate githubv4.Date   `json:"startDate"`
	Duration  githubv4.Int    `json:"duration"`
	Title     githubv4.String `json:"title"`
}

// detectOwnerType attempts to detect whether the project owner is a user or org.
// It first asks GitHub for the account type, then falls back to project probes
// for older or mocked clients where the account type is unavailable.
func detectOwnerType(ctx context.Context, client *github.Client, owner string, projectNumber int) (string, error) {
	user, resp, err := client.Users.Get(ctx, owner)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
		switch user.GetType() {
		case "User":
			return "user", nil
		case "Organization":
			return "org", nil
		}
	}

	// Try user first (more common for personal projects)
	_, resp, err = client.Projects.GetUserProject(ctx, owner, projectNumber)
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
