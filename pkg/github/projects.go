package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/sanitize"
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
	ProjectViewListFailedError           = "failed to list project views"
	ProjectViewGetFailedError            = "failed to get project view"
	ProjectViewCreateFailedError         = "failed to create project view"
	ProjectViewUpdateFailedError         = "failed to update project view"
	ProjectViewDeleteFailedError         = "failed to delete project view"
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
	projectsMethodListProjectViews          = "list_project_views"
	projectsMethodGetProjectView            = "get_project_view"
	projectsMethodCreateProjectView         = "create_project_view"
	projectsMethodUpdateProjectView         = "update_project_view"
	projectsMethodDeleteProjectView         = "delete_project_view"
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

type projectViewNode struct {
	ID            githubv4.ID
	Number        githubv4.Int
	Name          githubv4.String
	Layout        githubv4.ProjectV2ViewLayout
	Filter        *githubv4.String
	Configuration projectViewConfiguration
}

type projectViewConfiguration struct {
	VisibleFields projectViewVisibleFieldsConnection `graphql:"visibleFields(first: 100)"`
}

type projectViewVisibleFieldsConnection struct {
	Nodes []projectViewVisibleFieldNode
}

type projectViewVisibleFieldNode struct {
	ProjectV2Field struct {
		DatabaseID githubv4.Int `graphql:"databaseId"`
	} `graphql:"... on ProjectV2Field"`
	ProjectV2IterationField struct {
		DatabaseID githubv4.Int `graphql:"databaseId"`
	} `graphql:"... on ProjectV2IterationField"`
	ProjectV2MultiSelectField struct {
		DatabaseID githubv4.Int `graphql:"databaseId"`
	} `graphql:"... on ProjectV2MultiSelectField"`
	ProjectV2SingleSelectField struct {
		DatabaseID githubv4.Int `graphql:"databaseId"`
	} `graphql:"... on ProjectV2SingleSelectField"`
}

type projectViewNodeWithProject struct {
	projectViewNode
	Project projectVisibility
}

type projectViewConnection struct {
	Nodes    []projectViewNode
	PageInfo PageInfoFragment
}

type projectViewsProject struct {
	ID     githubv4.ID
	Public githubv4.Boolean
	Views  projectViewConnection `graphql:"views(first: $first, after: $after, last: $last, before: $before)"`
}

type projectViewsUserQuery struct {
	User struct {
		ProjectV2 projectViewsProject `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"user(login: $owner)"`
}

type projectViewsOrgQuery struct {
	Organization struct {
		ProjectV2 projectViewsProject `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $owner)"`
}

type projectViewNodeQuery struct {
	Node struct {
		ProjectView projectViewNodeWithProject `graphql:"... on ProjectV2View"`
	} `graphql:"node(id: $id)"`
}

type projectViewParentQuery struct {
	Node struct {
		ProjectView struct {
			ID      githubv4.ID
			Layout  githubv4.ProjectV2ViewLayout
			Project struct {
				ID githubv4.ID
			}
		} `graphql:"... on ProjectV2View"`
	} `graphql:"node(id: $id)"`
}

// ProjectV2ViewConfigurationInput is the GraphQL view configuration input.
type ProjectV2ViewConfigurationInput struct {
	VisibleFieldIDs []githubv4.ID `json:"visibleFieldIds"`
}

// CreateProjectV2ViewInput is the GraphQL input for creating a project view.
type CreateProjectV2ViewInput struct {
	ProjectID     githubv4.ID                      `json:"projectId"`
	Name          githubv4.String                  `json:"name"`
	Layout        githubv4.ProjectV2ViewLayout     `json:"layout"`
	Configuration *ProjectV2ViewConfigurationInput `json:"configuration,omitempty"`
}

// UpdateProjectV2ViewInput is the GraphQL input for updating a project view.
type UpdateProjectV2ViewInput struct {
	ViewID        githubv4.ID                      `json:"viewId"`
	Name          *githubv4.String                 `json:"name,omitempty"`
	Layout        *githubv4.ProjectV2ViewLayout    `json:"layout,omitempty"`
	Filter        *githubv4.String                 `json:"filter,omitempty"`
	Configuration *ProjectV2ViewConfigurationInput `json:"configuration,omitempty"`
}

type createProjectV2ViewMutation struct {
	CreateProjectV2View struct {
		ProjectV2View projectViewNode `graphql:"projectV2View"`
	} `graphql:"createProjectV2View(input: $input)"`
}

type updateProjectV2ViewMutation struct {
	UpdateProjectV2View struct {
		ProjectV2View projectViewNode `graphql:"projectV2View"`
	} `graphql:"updateProjectV2View(input: $input)"`
}

// DeleteProjectV2ViewInput is the GraphQL input for deleting a project view.
type DeleteProjectV2ViewInput struct {
	ViewID githubv4.ID `json:"viewId"`
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
		Body:       sanitize.Sanitize(derefString(node.Body)),
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
Use this tool to list projects for a user or organization, or list project fields, items, views, and status updates for a specific project.
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
							projectsMethodListProjectViews,
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
						Description: "The project's number. Required for 'list_project_fields', 'list_project_items', 'list_project_views', and 'list_project_status_updates' methods.",
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
			case projectsMethodListProjectFields, projectsMethodListProjectItems, projectsMethodListProjectStatusUpdates, projectsMethodListProjectViews:
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
				case projectsMethodListProjectViews:
					gqlClient, err := deps.GetGQLClient(ctx)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					result, isPrivate, payload, err := listProjectViews(ctx, gqlClient, args, owner, ownerType)
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
Use this tool to get details about individual projects, project fields, project items, and project views by their unique IDs.
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
							projectsMethodGetProjectView,
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
					"view_id": {
						Type:        "string",
						Description: "The node ID of the project view. Required for 'get_project_view' method.",
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

			// Handle node-ID-only methods before requiring owner and project_number.
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
			if method == projectsMethodGetProjectView {
				viewID, err := RequiredParam[string](args, "view_id")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				gqlClient, err := deps.GetGQLClient(ctx)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, isPrivate, payload, err := getProjectView(ctx, gqlClient, viewID)
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
					resolvedIDs, resolveErr := resolveFieldNamesToIDs(ctx, gqlClient, owner, ownerType, projectNumber, fieldNames, "fields")
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
	value := &jsonschema.Schema{
		Description: "The value to apply. Any JSON value is accepted; use null to clear the field.",
	}
	variant := func(required []string, properties map[string]*jsonschema.Schema) *jsonschema.Schema {
		properties["value"] = value
		return &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           properties,
			Required:             required,
		}
	}

	return &jsonschema.Schema{
		Type:        "object",
		Description: "The field/value to apply, using {\"id\": 123, \"value\": ...} or {\"name\": \"Status\", \"value\": ...}; null clears the field. Required for 'update_project_item' and 'update_project_items', where one top-level field/value applies to every item in a batch. For 'update_project_item' SINGLE_SELECT fields, the name form accepts option names; the ID form expects an option ID.",
		OneOf: []*jsonschema.Schema{
			variant([]string{"id", "value"}, map[string]*jsonschema.Schema{
				"id": {
					Type:        "integer",
					Description: "The numeric project field ID.",
				},
			}),
			variant([]string{"name", "value"}, map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					Description: "The project field name. Matching is case-insensitive.",
				},
			}),
		},
	}
}

// ProjectsWrite returns the tool and handler for modifying GitHub Projects resources.
func ProjectsWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "projects_write",
			Description: t("TOOL_PROJECTS_WRITE_DESCRIPTION", "Create and manage GitHub Projects: create projects, add/update/delete items, bulk-update many items at once, manage views, create status updates, and add iteration fields."),
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
							projectsMethodCreateProjectView,
							projectsMethodUpdateProjectView,
							projectsMethodDeleteProjectView,
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
					"view_id": {
						Type:        "string",
						Description: "Project view node ID for update or delete; must belong to owner/project_number.",
					},
					"name": {
						Type:        "string",
						Description: "View name; required when creating a view.",
					},
					"layout": {
						Type:        "string",
						Description: "View layout; required when creating a view.",
						Enum:        []any{"table", "board", "roadmap"},
					},
					"filter": {
						AnyOf: []*jsonschema.Schema{
							{Type: "string"},
							{Type: "null"},
						},
						Description: "Saved view filter; omit on update to preserve it, or pass null to clear it.",
					},
					"visible_fields": {
						Type:        "array",
						Description: "Ordered project field database IDs to show on create or replace on update; omit on update to preserve, or pass [] to reset. Mutually exclusive with visible_field_names. Roadmap accepts only [].",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"visible_field_names": {
						Type:        "array",
						Description: "Ordered project field names to show on create or replace on update; omit on update to preserve, or pass [] to reset. Mutually exclusive with visible_fields. Roadmap accepts only [].",
						Items: &jsonschema.Schema{
							Type: "string",
						},
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

			// create_project does not require project_number or a REST client
			if method == projectsMethodCreateProject {
				gqlClient, gqlErr := deps.GetGQLClient(ctx)
				if gqlErr != nil {
					return utils.NewToolResultError(gqlErr.Error()), nil, nil
				}
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

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
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
			case projectsMethodCreateProjectView:
				return createProjectView(ctx, gqlClient, args, owner, ownerType, projectNumber)
			case projectsMethodUpdateProjectView:
				return updateProjectView(ctx, gqlClient, args, owner, ownerType, projectNumber)
			case projectsMethodDeleteProjectView:
				return deleteProjectView(ctx, gqlClient, args, owner, ownerType, projectNumber)
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
		resolvedIDs, resolveErr := resolveFieldNamesToIDs(ctx, gqlClient, owner, ownerType, projectNumber, fieldNames, "fields")
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

func updateProjectItem(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, itemID int64, fieldValue map[string]any) (*mcp.CallToolResult, any, error) {
	updatePayload, issueField, err := buildUpdateProjectItem(ctx, gqlClient, owner, ownerType, projectNumber, fieldValue)
	if err != nil {
		var structured *ghErrors.StructuredResolutionError
		if errors.As(err, &structured) {
			return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
		}
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	if issueField != nil {
		projectItem, resp, fetchErr := fetchProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, nil)
		if fetchErr != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get project item", resp, fetchErr), nil, nil
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, nil, fmt.Errorf("failed to read response body: %w", readErr)
			}
			return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get project item", resp, body), nil, nil
		}

		issueID, resolveErr := projectItemIssueID(projectItem)
		if resolveErr != nil {
			var structured *ghErrors.StructuredResolutionError
			if errors.As(resolveErr, &structured) {
				return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
			}
			return utils.NewToolResultError(resolveErr.Error()), nil, nil
		}

		// The setIssueFieldValue mutation is gated behind the update_issue_suggestions
		// GraphQL feature flag, matching the set_issue_fields tool.
		ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "update_issue_suggestions")
		response, mutationErr := SetIssueFieldValues(ctxWithFeatures, gqlClient, SetIssueFieldValueInput{
			IssueID:     issueID,
			IssueFields: []IssueFieldCreateOrUpdateInput{*issueField},
		})
		if mutationErr != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to set issue field value", mutationErr), nil, nil
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
	r, err := json.Marshal(convertToMinimalProjectItem(updatedItem))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func projectItemIssueID(item *github.ProjectV2Item) (githubv4.ID, error) {
	if item == nil {
		return nil, ghErrors.NewStructuredResolutionError(
			"missing_metadata",
			"",
			"project item metadata is missing",
			nil,
		)
	}

	contentType := ""
	if item.ContentType != nil {
		contentType = string(*item.ContentType)
	}
	if contentType != string(github.ProjectV2ItemContentTypeIssue) {
		return nil, ghErrors.NewStructuredResolutionError(
			"unsupported_item_type",
			contentType,
			"attached Issue Fields can only be updated on Issue project items",
			nil,
		)
	}

	content := item.GetContent()
	if content == nil || content.GetIssue() == nil || content.GetIssue().GetNodeID() == "" {
		return nil, ghErrors.NewStructuredResolutionError(
			"missing_metadata",
			contentType,
			"project Issue item is missing its Issue node ID",
			nil,
		)
	}

	return githubv4.ID(content.GetIssue().GetNodeID()), nil
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

func convertToMinimalProjectView(node projectViewNode) MinimalProjectView {
	visibleFields := make([]int64, 0, len(node.Configuration.VisibleFields.Nodes))
	for _, field := range node.Configuration.VisibleFields.Nodes {
		switch {
		case field.ProjectV2SingleSelectField.DatabaseID != 0:
			visibleFields = append(visibleFields, int64(field.ProjectV2SingleSelectField.DatabaseID))
		case field.ProjectV2MultiSelectField.DatabaseID != 0:
			visibleFields = append(visibleFields, int64(field.ProjectV2MultiSelectField.DatabaseID))
		case field.ProjectV2IterationField.DatabaseID != 0:
			visibleFields = append(visibleFields, int64(field.ProjectV2IterationField.DatabaseID))
		default:
			visibleFields = append(visibleFields, int64(field.ProjectV2Field.DatabaseID))
		}
	}
	return MinimalProjectView{
		ID:            fmt.Sprintf("%v", node.ID),
		Number:        int(node.Number),
		Name:          string(node.Name),
		Layout:        projectViewLayoutName(node.Layout),
		Filter:        derefString(node.Filter),
		VisibleFields: visibleFields,
	}
}

func projectViewLayoutName(layout githubv4.ProjectV2ViewLayout) string {
	switch layout {
	case githubv4.ProjectV2ViewLayoutTableLayout:
		return "table"
	case githubv4.ProjectV2ViewLayoutBoardLayout:
		return "board"
	case githubv4.ProjectV2ViewLayoutRoadmapLayout:
		return "roadmap"
	default:
		return strings.ToLower(strings.TrimSuffix(string(layout), "_LAYOUT"))
	}
}

func parseProjectViewLayout(layout string) (githubv4.ProjectV2ViewLayout, error) {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "table":
		return githubv4.ProjectV2ViewLayoutTableLayout, nil
	case "board":
		return githubv4.ProjectV2ViewLayoutBoardLayout, nil
	case "roadmap":
		return githubv4.ProjectV2ViewLayoutRoadmapLayout, nil
	default:
		return "", fmt.Errorf("invalid layout %q: must be \"table\", \"board\", or \"roadmap\"", layout)
	}
}

func listProjectViews(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, bool, any, error) {
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
	if perPage < 1 || perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}
	after, err := OptionalParam[string](args, "after")
	if err != nil {
		return utils.NewToolResultError(err.Error()), false, nil, nil
	}
	before, err := OptionalParam[string](args, "before")
	if err != nil {
		return utils.NewToolResultError(err.Error()), false, nil, nil
	}
	if after != "" && before != "" {
		return utils.NewToolResultError("provide either 'after' or 'before', not both"), false, nil, nil
	}

	vars := map[string]any{
		"owner":         githubv4.String(owner),
		"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small integers
		"first":         (*githubv4.Int)(nil),
		"after":         (*githubv4.String)(nil),
		"last":          (*githubv4.Int)(nil),
		"before":        (*githubv4.String)(nil),
	}
	if before != "" {
		last := githubv4.Int(int32(perPage)) //nolint:gosec // perPage is bounded by MaxProjectsPerPage
		cursor := githubv4.String(before)
		vars["last"] = &last
		vars["before"] = &cursor
	} else {
		first := githubv4.Int(int32(perPage)) //nolint:gosec // perPage is bounded by MaxProjectsPerPage
		vars["first"] = &first
		if after != "" {
			cursor := githubv4.String(after)
			vars["after"] = &cursor
		}
	}

	var project projectViewsProject
	if ownerType == "org" {
		var query projectViewsOrgQuery
		if err := gqlClient.Query(ctx, &query, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewListFailedError, err)), false, nil, nil
		}
		project = query.Organization.ProjectV2
	} else {
		var query projectViewsUserQuery
		if err := gqlClient.Query(ctx, &query, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewListFailedError, err)), false, nil, nil
		}
		project = query.User.ProjectV2
	}
	if project.ID == nil || project.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: project was not found", ProjectViewListFailedError)), false, nil, nil
	}

	views := make([]MinimalProjectView, 0, len(project.Views.Nodes))
	for _, node := range project.Views.Nodes {
		views = append(views, convertToMinimalProjectView(node))
	}
	response := map[string]any{
		"views": views,
		"pageInfo": map[string]any{
			"hasNextPage":     project.Views.PageInfo.HasNextPage,
			"hasPreviousPage": project.Views.PageInfo.HasPreviousPage,
			"nextCursor":      string(project.Views.PageInfo.EndCursor),
			"prevCursor":      string(project.Views.PageInfo.StartCursor),
		},
	}
	result, err := json.Marshal(response)
	if err != nil {
		return nil, false, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(result)), !bool(project.Public), nil, nil
}

func getProjectView(ctx context.Context, gqlClient *githubv4.Client, viewID string) (*mcp.CallToolResult, bool, any, error) {
	var query projectViewNodeQuery
	vars := map[string]any{"id": githubv4.ID(viewID)}
	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewGetFailedError, err)), false, nil, nil
	}
	if query.Node.ProjectView.ID == nil || query.Node.ProjectView.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: node is not a ProjectV2View or was not found", ProjectViewGetFailedError)), false, nil, nil
	}

	view := convertToMinimalProjectView(query.Node.ProjectView.projectViewNode)
	result, err := json.Marshal(view)
	if err != nil {
		return nil, false, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(result)), !bool(query.Node.ProjectView.Project.Public), nil, nil
}

func projectViewVisibleFieldsInput(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string, projectNumber int) (*ProjectV2ViewConfigurationInput, error) {
	_, hasVisibleFields := args["visible_fields"]
	_, hasVisibleFieldNames := args["visible_field_names"]
	if !hasVisibleFields && !hasVisibleFieldNames {
		return nil, nil
	}

	databaseIDs, err := OptionalBigIntArrayParam(args, "visible_fields")
	if err != nil {
		return nil, err
	}
	names, err := OptionalStringArrayParam(args, "visible_field_names")
	if err != nil {
		return nil, err
	}
	if len(databaseIDs) > 0 && len(names) > 0 {
		return nil, errors.New("provide either 'visible_fields' or 'visible_field_names', not both")
	}
	if len(databaseIDs) == 0 && len(names) == 0 {
		return &ProjectV2ViewConfigurationInput{VisibleFieldIDs: []githubv4.ID{}}, nil
	}

	all, err := listAllProjectFields(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return nil, err
	}

	var resolved []ResolvedField
	if len(names) > 0 {
		resolved, err = resolveFieldsByName(all, owner, projectNumber, names, "visible_fields")
		if err != nil {
			return nil, err
		}
	} else {
		byDatabaseID := make(map[int64]ResolvedField, len(all))
		for _, field := range all {
			id, parseErr := parseInt64(field.ID)
			if parseErr != nil {
				continue
			}
			byDatabaseID[id] = field
		}
		resolved = make([]ResolvedField, 0, len(databaseIDs))
		for _, id := range databaseIDs {
			field, ok := byDatabaseID[id]
			if !ok {
				return nil, fmt.Errorf("project field database ID %d was not found on project %s#%d", id, owner, projectNumber)
			}
			resolved = append(resolved, field)
		}
	}

	nodeIDs := make([]githubv4.ID, 0, len(resolved))
	seen := make(map[string]struct{}, len(resolved))
	for _, field := range resolved {
		if _, ok := seen[field.NodeID]; ok {
			return nil, fmt.Errorf("project field %q is included more than once", field.Name)
		}
		seen[field.NodeID] = struct{}{}
		nodeIDs = append(nodeIDs, githubv4.ID(field.NodeID))
	}
	return &ProjectV2ViewConfigurationInput{VisibleFieldIDs: nodeIDs}, nil
}

// projectViewRequestsVisibleFields reports whether the caller asked for a non-empty
// set of visible fields, without resolving them against the project.
func projectViewRequestsVisibleFields(args map[string]any) bool {
	if databaseIDs, err := OptionalBigIntArrayParam(args, "visible_fields"); err == nil && len(databaseIDs) > 0 {
		return true
	}
	names, err := OptionalStringArrayParam(args, "visible_field_names")
	return err == nil && len(names) > 0
}

func createProjectView(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string, projectNumber int) (*mcp.CallToolResult, any, error) {
	name, err := RequiredParam[string](args, "name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	if strings.TrimSpace(name) == "" {
		return utils.NewToolResultError("name must not be empty"), nil, nil
	}
	layoutName, err := RequiredParam[string](args, "layout")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	layout, err := parseProjectViewLayout(layoutName)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	filter, hasFilter, err := OptionalNullableStringParam(args, "filter")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	if layout == githubv4.ProjectV2ViewLayoutRoadmapLayout && projectViewRequestsVisibleFields(args) {
		return utils.NewToolResultError("visible fields are not supported for roadmap views"), nil, nil
	}
	configuration, err := projectViewVisibleFieldsInput(ctx, gqlClient, args, owner, ownerType, projectNumber)
	if err != nil {
		var structured *ghErrors.StructuredResolutionError
		if errors.As(err, &structured) {
			return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
		}
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: failed to resolve project: %v", ProjectViewCreateFailedError, err)), nil, nil
	}
	if projectID == nil || projectID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: project was not found", ProjectViewCreateFailedError)), nil, nil
	}

	input := CreateProjectV2ViewInput{
		ProjectID:     projectID,
		Name:          githubv4.String(name),
		Layout:        layout,
		Configuration: configuration,
	}
	var mutation createProjectV2ViewMutation
	if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewCreateFailedError, err)), nil, nil
	}
	view := mutation.CreateProjectV2View.ProjectV2View
	if view.ID == nil || view.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: response did not include a project view", ProjectViewCreateFailedError)), nil, nil
	}

	if hasFilter && filter != nil {
		filterValue := githubv4.String(*filter)
		updateInput := UpdateProjectV2ViewInput{
			ViewID: githubv4.ID(fmt.Sprintf("%v", view.ID)),
			Filter: &filterValue,
		}
		var updateMutation updateProjectV2ViewMutation
		if err := gqlClient.Mutate(ctx, &updateMutation, updateInput, nil); err != nil {
			cleanupErr := deleteProjectViewByID(ctx, gqlClient, updateInput.ViewID)
			if cleanupErr != nil {
				return utils.NewToolResultError(fmt.Sprintf("%s: failed to set filter: %v; failed to clean up created view %v: %v", ProjectViewCreateFailedError, err, updateInput.ViewID, cleanupErr)), nil, nil
			}
			return utils.NewToolResultError(fmt.Sprintf("%s: failed to set filter: %v; created view was cleaned up", ProjectViewCreateFailedError, err)), nil, nil
		}
		view = updateMutation.UpdateProjectV2View.ProjectV2View
	}
	return MarshalledTextResult(convertToMinimalProjectView(view)), nil, nil
}

func verifyProjectViewParent(ctx context.Context, gqlClient *githubv4.Client, viewID, owner, ownerType string, projectNumber int) (githubv4.ProjectV2ViewLayout, error) {
	expectedProjectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return "", fmt.Errorf("failed to resolve requested project: %w", err)
	}
	if expectedProjectID == nil || expectedProjectID == "" {
		return "", fmt.Errorf("requested project was not found")
	}

	var query projectViewParentQuery
	if err := gqlClient.Query(ctx, &query, map[string]any{"id": githubv4.ID(viewID)}); err != nil {
		return "", fmt.Errorf("failed to resolve project view: %w", err)
	}
	if query.Node.ProjectView.ID == nil || query.Node.ProjectView.ID == "" {
		return "", fmt.Errorf("node is not a ProjectV2View or was not found")
	}
	if query.Node.ProjectView.Project.ID != expectedProjectID {
		return "", fmt.Errorf("project view does not belong to the requested project")
	}
	return query.Node.ProjectView.Layout, nil
}

func updateProjectView(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string, projectNumber int) (*mcp.CallToolResult, any, error) {
	viewID, err := RequiredParam[string](args, "view_id")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	name, hasName, err := OptionalParamOK[string](args, "name")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	layoutName, hasLayout, err := OptionalParamOK[string](args, "layout")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	filter, hasFilter, err := OptionalNullableStringParam(args, "filter")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	_, hasVisibleFields := args["visible_fields"]
	_, hasVisibleFieldNames := args["visible_field_names"]
	if !hasName && !hasLayout && !hasFilter && !hasVisibleFields && !hasVisibleFieldNames {
		return utils.NewToolResultError("update_project_view requires at least one of name, layout, filter, visible_fields, or visible_field_names"), nil, nil
	}
	if hasName && strings.TrimSpace(name) == "" {
		return utils.NewToolResultError("name must not be empty"), nil, nil
	}

	input := UpdateProjectV2ViewInput{ViewID: githubv4.ID(viewID)}
	if hasName {
		value := githubv4.String(name)
		input.Name = &value
	}
	if hasLayout {
		layout, err := parseProjectViewLayout(layoutName)
		if err != nil {
			return utils.NewToolResultError(err.Error()), nil, nil
		}
		input.Layout = &layout
	}
	if hasFilter {
		// The API clears a filter with an empty string, so a null filter is sent as "".
		value := githubv4.String("")
		if filter != nil {
			value = githubv4.String(*filter)
		}
		input.Filter = &value
	}
	currentLayout, err := verifyProjectViewParent(ctx, gqlClient, viewID, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewUpdateFailedError, err)), nil, nil
	}
	effectiveLayout := currentLayout
	if input.Layout != nil {
		effectiveLayout = *input.Layout
	}
	if effectiveLayout == githubv4.ProjectV2ViewLayoutRoadmapLayout && projectViewRequestsVisibleFields(args) {
		return utils.NewToolResultError("visible fields are not supported for roadmap views"), nil, nil
	}

	configuration, err := projectViewVisibleFieldsInput(ctx, gqlClient, args, owner, ownerType, projectNumber)
	if err != nil {
		var structured *ghErrors.StructuredResolutionError
		if errors.As(err, &structured) {
			return ghErrors.NewStructuredResolutionErrorResponse(structured), nil, nil
		}
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	input.Configuration = configuration

	var mutation updateProjectV2ViewMutation
	if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewUpdateFailedError, err)), nil, nil
	}
	if mutation.UpdateProjectV2View.ProjectV2View.ID == nil || mutation.UpdateProjectV2View.ProjectV2View.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: response did not include a project view", ProjectViewUpdateFailedError)), nil, nil
	}
	return MarshalledTextResult(convertToMinimalProjectView(mutation.UpdateProjectV2View.ProjectV2View)), nil, nil
}

func deleteProjectViewByID(ctx context.Context, gqlClient *githubv4.Client, viewID githubv4.ID) error {
	input := DeleteProjectV2ViewInput{ViewID: viewID}
	var mutation struct {
		DeleteProjectV2View struct {
			ProjectV2View struct {
				ID githubv4.ID
			} `graphql:"projectV2View"`
		} `graphql:"deleteProjectV2View(input: $input)"`
	}
	if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
		return err
	}
	if id := mutation.DeleteProjectV2View.ProjectV2View.ID; id == nil || id == "" {
		return errors.New("response did not include the deleted project view")
	}
	return nil
}

func deleteProjectView(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string, projectNumber int) (*mcp.CallToolResult, any, error) {
	viewID, err := RequiredParam[string](args, "view_id")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	if _, err := verifyProjectViewParent(ctx, gqlClient, viewID, owner, ownerType, projectNumber); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewDeleteFailedError, err)), nil, nil
	}
	if err := deleteProjectViewByID(ctx, gqlClient, githubv4.ID(viewID)); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectViewDeleteFailedError, err)), nil, nil
	}
	return MarshalledTextResult(map[string]string{"deleted_view_id": viewID}), nil, nil
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

// buildUpdateProjectItem builds either a standard Project update or an attached Issue Field update.
func buildUpdateProjectItem(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, input map[string]any) (*github.UpdateProjectItemOptions, *IssueFieldCreateOrUpdateInput, error) {
	if input == nil {
		return nil, nil, fmt.Errorf("updated_field must be an object")
	}

	valueField, hasValue := input["value"]
	if !hasValue {
		return nil, nil, fmt.Errorf("updated_field.value is required")
	}

	idField, hasID := input["id"]
	nameField, hasName := input["name"]

	switch {
	case hasID && hasName:
		return nil, nil, fmt.Errorf("updated_field must set either id or name, not both")
	case !hasID && !hasName:
		return nil, nil, fmt.Errorf("updated_field requires either id or name")
	}

	var (
		fieldID  int64
		resolved *ResolvedField
	)

	if hasID {
		var err error
		fieldID, err = validateAndConvertToInt64(idField)
		if err != nil {
			return nil, nil, fmt.Errorf("updated_field.id: %w", err)
		}
	} else {
		fieldName, ok := nameField.(string)
		if !ok || fieldName == "" {
			return nil, nil, fmt.Errorf("updated_field.name must be a non-empty string")
		}
		if gqlClient == nil {
			return nil, nil, fmt.Errorf("internal error: gqlClient is required to resolve updated_field.name")
		}
		var err error
		resolved, err = resolveProjectFieldByName(ctx, gqlClient, owner, ownerType, projectNumber, fieldName, "")
		if err != nil {
			return nil, nil, err
		}
		if supportsIssueFieldUpdate(resolved.DataType) {
			resolved, err = resolveIssueFieldForUpdate(ctx, gqlClient, owner, ownerType, projectNumber, resolved)
			if err != nil {
				return nil, nil, err
			}
			if resolved.IsIssueField {
				issueField, buildErr := buildIssueFieldUpdate(resolved, valueField)
				if buildErr != nil {
					return nil, nil, buildErr
				}
				return nil, issueField, nil
			}
		}
		parsedID, parseErr := parseInt64(resolved.ID)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("resolved field %q has non-numeric ID %q; pass updated_field.id directly", resolved.Name, resolved.ID)
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
					return nil, nil, optErr
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

	return payload, nil, nil
}

func supportsIssueFieldUpdate(dataType string) bool {
	switch dataType {
	case "TEXT", "NUMBER", "DATE", "SINGLE_SELECT":
		return true
	default:
		return false
	}
}

func buildIssueFieldUpdate(field *ResolvedField, value any) (*IssueFieldCreateOrUpdateInput, error) {
	if !supportsIssueFieldUpdate(field.DataType) {
		return nil, ghErrors.NewStructuredResolutionError(
			"unsupported_field_type",
			field.Name,
			fmt.Sprintf("attached Issue Field %q has unsupported data type %q", field.Name, field.DataType),
			nil,
		)
	}

	if field.IssueFieldID == "" {
		return nil, ghErrors.NewStructuredResolutionError(
			"missing_field_metadata",
			field.Name,
			fmt.Sprintf("attached Issue Field %q is missing its Issue Field node ID", field.Name),
			nil,
		)
	}

	input := &IssueFieldCreateOrUpdateInput{FieldID: githubv4.ID(field.IssueFieldID)}
	if value == nil {
		input.Delete = githubv4.NewBoolean(githubv4.Boolean(true))
		return input, nil
	}

	switch field.DataType {
	case "TEXT":
		text, ok := value.(string)
		if !ok {
			return nil, invalidIssueFieldValue(field, "value must be a string")
		}
		input.TextValue = githubv4.NewString(githubv4.String(text))
	case "NUMBER":
		number, ok := toFloat64(value)
		if !ok {
			return nil, invalidIssueFieldValue(field, "value must be a number")
		}
		input.NumberValue = githubv4.NewFloat(githubv4.Float(number))
	case "DATE":
		date, ok := value.(string)
		if !ok {
			return nil, invalidIssueFieldValue(field, "value must be a date string in YYYY-MM-DD format")
		}
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return nil, invalidIssueFieldValue(field, "value must be a valid date in YYYY-MM-DD format")
		}
		input.DateValue = githubv4.NewString(githubv4.String(date))
	case "SINGLE_SELECT":
		optionName, ok := value.(string)
		if !ok || optionName == "" {
			return nil, invalidIssueFieldValue(field, "value must be a non-empty option name")
		}
		optionID, err := resolveSingleSelectOptionByName(field, optionName)
		if err != nil {
			return nil, err
		}
		input.SingleSelectOptionID = githubv4.NewID(githubv4.ID(optionID))
	}

	return input, nil
}

func invalidIssueFieldValue(field *ResolvedField, hint string) error {
	return ghErrors.NewStructuredResolutionError(
		"invalid_field_value",
		field.Name,
		fmt.Sprintf("invalid value for attached Issue Field %q: %s", field.Name, hint),
		nil,
	)
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
