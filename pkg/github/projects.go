package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v82/github"
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
	projectsMethodDeleteProjectItem         = "delete_project_item"
	projectsMethodListProjectStatusUpdates  = "list_project_status_updates"
	projectsMethodGetProjectStatusUpdate    = "get_project_status_update"
	projectsMethodCreateProjectStatusUpdate = "create_project_status_update"
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

type statusUpdateConnection struct {
	Nodes    []statusUpdateNode
	PageInfo PageInfoFragment
}

// statusUpdatesUserQuery is the GraphQL query for listing status updates on a user-owned project.
type statusUpdatesUserQuery struct {
	User struct {
		ProjectV2 struct {
			StatusUpdates statusUpdateConnection `graphql:"statusUpdates(first: $first, after: $after, orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"user(login: $owner)"`
}

// statusUpdatesOrgQuery is the GraphQL query for listing status updates on an org-owned project.
type statusUpdatesOrgQuery struct {
	Organization struct {
		ProjectV2 struct {
			StatusUpdates statusUpdateConnection `graphql:"statusUpdates(first: $first, after: $after, orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $owner)"`
}

// statusUpdateNodeQuery is the GraphQL query for fetching a single status update by node ID.
type statusUpdateNodeQuery struct {
	Node struct {
		StatusUpdate statusUpdateNode `graphql:"... on ProjectV2StatusUpdate"`
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
			default:
				// All other methods require project_number and ownerType detection
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

				switch method {
				case projectsMethodListProjectFields:
					return listProjectFields(ctx, client, args, owner, ownerType)
				case projectsMethodListProjectItems:
					return listProjectItems(ctx, client, args, owner, ownerType)
				case projectsMethodListProjectStatusUpdates:
					gqlClient, err := deps.GetGQLClient(ctx)
					if err != nil {
						return utils.NewToolResultError(err.Error()), nil, nil
					}
					return listProjectStatusUpdates(ctx, gqlClient, args, owner, ownerType)
				default:
					return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
				}
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
						Description: "Specific list of field IDs to include in the response when getting a project item (e.g. [\"102589\", \"985201\", \"169875\"]). If not provided, only the title field is included. Only used for 'get_project_item' method.",
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
				return getProjectStatusUpdate(ctx, gqlClient, statusUpdateID)
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
	return tool
}

// ProjectsWrite returns the tool and handler for modifying GitHub Projects resources.
func ProjectsWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	tool := NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "projects_write",
			Description: t("TOOL_PROJECTS_WRITE_DESCRIPTION", "Add, update, or delete project items, or create status updates in a GitHub Project."),
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
							projectsMethodCreateProjectStatusUpdate,
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
						Description: "The numeric project item ID. Required for 'delete_project_item'. For 'update_project_item', provide this or identify the content with item_owner, item_repo, item_type, and issue_number or pull_request_number.",
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
						Description: "Object consisting of the ID of the project field to update and the new value for the field. To clear the field, set value to null. Example: {\"id\": 123456, \"value\": \"New Value\"}. For 'update_project_item', provide updated_field or updated_fields.",
					},
					"updated_fields": {
						Type:        "array",
						Description: "List of project field updates, each with the field ID and new value. Example: [{\"id\": 123456, \"value\": \"In Progress\"}, {\"id\": 234567, \"value\": \"P1\"}].",
						Items:       &jsonschema.Schema{Type: "object"},
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
						Description: "The start date of the status update in YYYY-MM-DD format. Used for 'create_project_status_update' method.",
					},
					"target_date": {
						Type:        "string",
						Description: "The target date of the status update in YYYY-MM-DD format. Used for 'create_project_status_update' method.",
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
				itemID, err := resolveProjectItemIDForUpdate(ctx, client, owner, ownerType, projectNumber, args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				fieldValues, err := buildProjectFieldValues(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				return updateProjectItem(ctx, client, owner, ownerType, projectNumber, itemID, fieldValues)
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
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		},
	)
	return tool
}

// CreateProjectIssue creates an issue, adds it to a Project V2 board, and sets initial project fields.
func CreateProjectIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataProjects,
		mcp.Tool{
			Name:        "create_project_issue",
			Description: t("TOOL_CREATE_PROJECT_ISSUE_DESCRIPTION", "Create a GitHub issue, add it to a GitHub Project, and set initial project Status and Priority fields."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_CREATE_PROJECT_ISSUE_TITLE", "Create project issue"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(false),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body content",
					},
					"labels": {
						Type:        "array",
						Description: "Labels to apply to the issue",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"assignees": {
						Type:        "array",
						Description: "GitHub usernames to assign to this issue",
						Items:       &jsonschema.Schema{Type: "string"},
					},
					"type": {
						Type:        "string",
						Description: "Issue type name, when the repository supports issue types",
					},
					"project_owner": {
						Type:        "string",
						Description: "Project owner login. Defaults to the repository owner when omitted.",
					},
					"project_owner_type": {
						Type:        "string",
						Description: "Project owner type.",
						Enum:        []any{"user", "org"},
					},
					"project_number": {
						Type:        "number",
						Description: "The project's number.",
					},
					"status_field_id": {
						Type:        "number",
						Description: "Project Status field ID.",
					},
					"status_value": {
						Type:        "string",
						Description: "Initial Status field value or option ID. For github-workflow this should be Backlog's option ID.",
					},
					"priority_field_id": {
						Type:        "number",
						Description: "Project Priority field ID.",
					},
					"priority_value": {
						Type:        "string",
						Description: "Initial Priority field value or option ID.",
					},
				},
				Required: []string{"owner", "repo", "title", "project_number", "status_field_id", "status_value", "priority_field_id", "priority_value"},
			},
		},
		[]scopes.Scope{scopes.Repo, scopes.Project},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			title, err := RequiredParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			assignees, err := OptionalStringArrayParam(args, "assignees")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueType, err := OptionalParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectOwner, err := OptionalParam[string](args, "project_owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if projectOwner == "" {
				projectOwner = owner
			}
			projectOwnerType, err := OptionalParam[string](args, "project_owner_type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			projectNumber, err := RequiredInt(args, "project_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			statusFieldID, err := RequiredBigInt(args, "status_field_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			statusValue, ok := args["status_value"]
			if !ok {
				return utils.NewToolResultError("missing required parameter: status_value"), nil, nil
			}
			priorityFieldID, err := RequiredBigInt(args, "priority_field_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			priorityValue, ok := args["priority_value"]
			if !ok {
				return utils.NewToolResultError("missing required parameter: priority_value"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if projectOwnerType == "" {
				projectOwnerType, err = detectOwnerType(ctx, client, projectOwner, projectNumber)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			issueRequest := &github.IssueRequest{
				Title:     github.Ptr(title),
				Body:      github.Ptr(body),
				Labels:    &labels,
				Assignees: &assignees,
			}
			if issueType != "" {
				issueRequest.Type = github.Ptr(issueType)
			}

			issue, resp, err := client.Issues.Create(ctx, owner, repo, issueRequest)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create issue", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create issue", resp, body), nil, nil
			}

			addedItem, err := addProjectItemData(ctx, gqlClient, projectOwner, projectOwnerType, projectNumber, owner, repo, issue.GetNumber(), "issue")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if addedItem.ProjectItemID == 0 {
				return utils.NewToolResultError("created project item did not include a numeric project_item_id"), nil, nil
			}

			updateResult, _, err := updateProjectItem(ctx, client, projectOwner, projectOwnerType, projectNumber, addedItem.ProjectItemID, []map[string]any{
				{"id": statusFieldID, "value": statusValue},
				{"id": priorityFieldID, "value": priorityValue},
			})
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if updateResult != nil && updateResult.IsError {
				return updateResult, nil, nil
			}

			response := map[string]any{
				"issue_number":         issue.GetNumber(),
				"issue_id":             issue.GetID(),
				"issue_node_id":        issue.GetNodeID(),
				"html_url":             issue.GetHTMLURL(),
				"project_item_id":      addedItem.ProjectItemID,
				"project_item_node_id": addedItem.ProjectItemNodeID,
			}
			r, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}
			return utils.NewToolResultText(string(r)), nil, nil
		},
	)
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

	items, err := convertProjectItemsToResponse(projectItems)
	if err != nil {
		return nil, nil, err
	}

	response := map[string]any{
		"items":    items,
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

	item, err := convertProjectItemToResponse(projectItem)
	if err != nil {
		return nil, nil, err
	}

	r, err := json.Marshal(item)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil, nil
}

func updateProjectItem(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemID int64, fieldValues []map[string]any) (*mcp.CallToolResult, any, error) {
	updatePayload, err := buildUpdateProjectItem(fieldValues)
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
	item, err := convertProjectItemToResponse(updatedItem)
	if err != nil {
		return nil, nil, err
	}

	r, err := json.Marshal(item)
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
type projectItemAddResult struct {
	ID                string `json:"id"`
	ProjectItemID     int64  `json:"project_item_id,omitempty"`
	ProjectItemNodeID string `json:"project_item_node_id"`
	ContentType       string `json:"content_type"`
	ContentOwner      string `json:"content_owner"`
	ContentRepo       string `json:"content_repo"`
	ContentNumber     int    `json:"content_number"`
	ContentNodeID     string `json:"content_node_id"`
	Message           string `json:"message"`
}

func addProjectItemData(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, itemOwner, itemRepo string, itemNumber int, itemType string) (*projectItemAddResult, error) {
	if itemType != "issue" && itemType != "pull_request" {
		return nil, fmt.Errorf("item_type must be either 'issue' or 'pull_request'")
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
		return nil, fmt.Errorf("failed to resolve %s: %w", itemType, err)
	}

	// Use GraphQL to add the item to the project
	var mutation struct {
		AddProjectV2ItemByID struct {
			Item struct {
				ID         githubv4.ID
				DatabaseID githubv4.Int `graphql:"databaseId"`
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	// Resolve the project number to a node ID
	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return nil, err
	}

	// Add the item to the project
	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: projectID,
		ContentID: nodeID,
	}

	err = gqlClient.Mutate(ctx, &mutation, input, nil)
	if err != nil {
		return nil, fmt.Errorf(ProjectAddFailedError+": %w", err)
	}

	itemNodeID := fmt.Sprintf("%v", mutation.AddProjectV2ItemByID.Item.ID)
	return &projectItemAddResult{
		ID:                itemNodeID,
		ProjectItemID:     int64(mutation.AddProjectV2ItemByID.Item.DatabaseID),
		ProjectItemNodeID: itemNodeID,
		ContentType:       itemType,
		ContentOwner:      itemOwner,
		ContentRepo:       itemRepo,
		ContentNumber:     itemNumber,
		ContentNodeID:     fmt.Sprintf("%v", nodeID),
		Message:           fmt.Sprintf("Successfully added %s %s/%s#%d to project %s/%d", itemType, itemOwner, itemRepo, itemNumber, owner, projectNumber),
	}, nil
}

func addProjectItem(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, itemOwner, itemRepo string, itemNumber int, itemType string) (*mcp.CallToolResult, any, error) {
	result, err := addProjectItemData(ctx, gqlClient, owner, ownerType, projectNumber, itemOwner, itemRepo, itemNumber, itemType)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
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
func listProjectStatusUpdates(ctx context.Context, gqlClient *githubv4.Client, args map[string]any, owner, ownerType string) (*mcp.CallToolResult, any, error) {
	if ownerType != "user" && ownerType != "org" {
		return utils.NewToolResultError(fmt.Sprintf("invalid owner_type %q: must be \"user\" or \"org\"", ownerType)), nil, nil
	}

	projectNumber, err := RequiredInt(args, "project_number")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	perPage, err := OptionalIntParamWithDefault(args, "per_page", MaxProjectsPerPage)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	if perPage > MaxProjectsPerPage {
		perPage = MaxProjectsPerPage
	}
	if perPage < 1 {
		perPage = MaxProjectsPerPage
	}

	afterCursor, err := OptionalParam[string](args, "after")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
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

	if ownerType == "org" {
		var q statusUpdatesOrgQuery
		if err := gqlClient.Query(ctx, &q, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateListFailedError, err)), nil, nil
		}
		nodes = q.Organization.ProjectV2.StatusUpdates.Nodes
		pi = q.Organization.ProjectV2.StatusUpdates.PageInfo
	} else {
		var q statusUpdatesUserQuery
		if err := gqlClient.Query(ctx, &q, vars); err != nil {
			return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateListFailedError, err)), nil, nil
		}
		nodes = q.User.ProjectV2.StatusUpdates.Nodes
		pi = q.User.ProjectV2.StatusUpdates.PageInfo
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
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return utils.NewToolResultText(string(r)), nil, nil
}

// getProjectStatusUpdate fetches a single status update by its node ID via GraphQL.
func getProjectStatusUpdate(ctx context.Context, gqlClient *githubv4.Client, statusUpdateID string) (*mcp.CallToolResult, any, error) {
	var q statusUpdateNodeQuery
	vars := map[string]any{
		"id": githubv4.ID(statusUpdateID),
	}

	if err := gqlClient.Query(ctx, &q, vars); err != nil {
		return utils.NewToolResultError(fmt.Sprintf("%s: %v", ProjectStatusUpdateGetFailedError, err)), nil, nil
	}

	if q.Node.StatusUpdate.ID == nil || q.Node.StatusUpdate.ID == "" {
		return utils.NewToolResultError(fmt.Sprintf("%s: node is not a ProjectV2StatusUpdate or was not found", ProjectStatusUpdateGetFailedError)), nil, nil
	}

	update := convertToMinimalStatusUpdate(q.Node.StatusUpdate)

	r, err := json.Marshal(update)
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

func convertProjectItemsToResponse(projectItems []*github.ProjectV2Item) ([]map[string]any, error) {
	items := make([]map[string]any, 0, len(projectItems))
	for _, projectItem := range projectItems {
		item, err := convertProjectItemToResponse(projectItem)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func convertProjectItemToResponse(projectItem any) (map[string]any, error) {
	raw, err := projectItemToMap(projectItem)
	if err != nil {
		return nil, err
	}
	addProjectItemIdentityFields(raw)
	return raw, nil
}

func projectItemToMap(projectItem any) (map[string]any, error) {
	r, err := json.Marshal(projectItem)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal project item: %w", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(r, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project item: %w", err)
	}
	return raw, nil
}

func addProjectItemIdentityFields(item map[string]any) {
	if id, ok := item["id"]; ok {
		item["project_item_id"] = id
	}
	if nodeID, ok := item["node_id"].(string); ok && nodeID != "" {
		item["project_item_node_id"] = nodeID
	}

	content, ok := item["content"].(map[string]any)
	if !ok || content == nil {
		return
	}

	if contentType := contentTypeFromContent(content); contentType != "" {
		item["content_type"] = contentType
	}
	if id, ok := content["id"]; ok {
		item["content_id"] = id
	}
	if nodeID, ok := content["node_id"].(string); ok && nodeID != "" {
		item["content_node_id"] = nodeID
	}
	if number, ok := numberFromAny(content["number"]); ok {
		item["content_number"] = number
	}
	if owner, repo := ownerRepoFromContent(content); owner != "" && repo != "" {
		item["content_owner"] = owner
		item["content_repo"] = repo
	}
	if labels := labelsFromContent(content); len(labels) > 0 {
		item["content_labels"] = labels
	}
}

func contentTypeFromContent(content map[string]any) string {
	for _, key := range []string{"type", "__typename"} {
		if value, ok := content[key].(string); ok && value != "" {
			normalized := strings.ToLower(value)
			if normalized == "pullrequest" {
				return "pull_request"
			}
			return normalized
		}
	}
	if htmlURL, ok := content["html_url"].(string); ok {
		if strings.Contains(htmlURL, "/pull/") {
			return "pull_request"
		}
		if strings.Contains(htmlURL, "/issues/") {
			return "issue"
		}
	}
	return ""
}

func ownerRepoFromContent(content map[string]any) (string, string) {
	if repositoryURL, ok := content["repository_url"].(string); ok && repositoryURL != "" {
		if owner, repo := ownerRepoFromURL(repositoryURL, "/repos/"); owner != "" && repo != "" {
			return owner, repo
		}
	}
	if htmlURL, ok := content["html_url"].(string); ok && htmlURL != "" {
		return ownerRepoFromURL(htmlURL, "github.com/")
	}
	return "", ""
}

func ownerRepoFromURL(rawURL, marker string) (string, string) {
	index := strings.Index(rawURL, marker)
	if index == -1 {
		return "", ""
	}
	path := strings.Trim(rawURL[index+len(marker):], "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func labelsFromContent(content map[string]any) []string {
	rawLabels, ok := content["labels"].([]any)
	if !ok {
		return nil
	}
	labels := make([]string, 0, len(rawLabels))
	for _, rawLabel := range rawLabels {
		switch label := rawLabel.(type) {
		case string:
			labels = append(labels, label)
		case map[string]any:
			if name, ok := label["name"].(string); ok && name != "" {
				labels = append(labels, name)
			}
		}
	}
	return labels
}

func numberFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), float64(int(v)) == v
	case int:
		return v, true
	case int64:
		return int(v), int64(int(v)) == v
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func resolveProjectItemIDForUpdate(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, args map[string]any) (int64, error) {
	if _, exists := args["item_id"]; exists {
		return RequiredBigInt(args, "item_id")
	}

	itemType, itemNumber, itemOwner, itemRepo, err := projectContentIdentityFromArgs(args)
	if err != nil {
		return 0, err
	}

	return findProjectItemIDByContent(ctx, client, owner, ownerType, projectNumber, itemOwner, itemRepo, itemNumber, itemType)
}

func projectContentIdentityFromArgs(args map[string]any) (string, int, string, string, error) {
	itemOwner, err := RequiredParam[string](args, "item_owner")
	if err != nil {
		return "", 0, "", "", err
	}
	itemRepo, err := RequiredParam[string](args, "item_repo")
	if err != nil {
		return "", 0, "", "", err
	}
	itemType, err := OptionalParam[string](args, "item_type")
	if err != nil {
		return "", 0, "", "", err
	}

	if itemType == "" || itemType == "issue" {
		if _, exists := args["issue_number"]; exists {
			itemNumber, err := RequiredInt(args, "issue_number")
			return "issue", itemNumber, itemOwner, itemRepo, err
		}
	}
	if itemType == "" || itemType == "pull_request" {
		if _, exists := args["pull_request_number"]; exists {
			itemNumber, err := RequiredInt(args, "pull_request_number")
			return "pull_request", itemNumber, itemOwner, itemRepo, err
		}
	}

	return "", 0, "", "", fmt.Errorf("provide item_id or identify content with item_owner, item_repo, and issue_number or pull_request_number")
}

func findProjectItemIDByContent(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, itemOwner, itemRepo string, itemNumber int, itemType string) (int64, error) {
	opts := &github.ListProjectItemsOptions{
		ListProjectsOptions: github.ListProjectsOptions{
			ListProjectsPaginationOptions: github.ListProjectsPaginationOptions{
				PerPage: github.Ptr(MaxProjectsPerPage),
			},
		},
	}

	for {
		var projectItems []*github.ProjectV2Item
		var resp *github.Response
		var err error
		if ownerType == "org" {
			projectItems, resp, err = client.Projects.ListOrganizationProjectItems(ctx, owner, projectNumber, opts)
		} else {
			projectItems, resp, err = client.Projects.ListUserProjectItems(ctx, owner, projectNumber, opts)
		}
		if err != nil {
			return 0, err
		}

		for _, projectItem := range projectItems {
			item, err := convertProjectItemToResponse(projectItem)
			if err != nil {
				return 0, err
			}
			if projectItemMatchesContent(item, itemOwner, itemRepo, itemNumber, itemType) {
				id, ok := validateAndConvertProjectItemID(item["project_item_id"])
				if !ok {
					return 0, fmt.Errorf("matched project item has no numeric project_item_id")
				}
				if resp != nil && resp.Body != nil {
					_ = resp.Body.Close()
				}
				return id, nil
			}
		}

		next := ""
		if resp != nil {
			next = resp.After
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
		}
		if next == "" {
			break
		}
		opts.ListProjectsOptions.ListProjectsPaginationOptions.After = github.Ptr(next)
	}

	return 0, fmt.Errorf("project item not found for %s %s/%s#%d", itemType, itemOwner, itemRepo, itemNumber)
}

func projectItemMatchesContent(item map[string]any, owner, repo string, number int, itemType string) bool {
	contentOwner, _ := item["content_owner"].(string)
	contentRepo, _ := item["content_repo"].(string)
	contentType, _ := item["content_type"].(string)
	contentNumber, ok := numberFromAny(item["content_number"])

	return ok &&
		strings.EqualFold(contentOwner, owner) &&
		strings.EqualFold(contentRepo, repo) &&
		contentNumber == number &&
		(contentType == "" || contentType == itemType)
}

func validateAndConvertProjectItemID(value any) (int64, bool) {
	id, err := validateAndConvertToInt64(value)
	return id, err == nil
}

func buildProjectFieldValues(args map[string]any) ([]map[string]any, error) {
	if rawUpdatedFields, exists := args["updated_fields"]; exists {
		fields, ok := rawUpdatedFields.([]any)
		if !ok || len(fields) == 0 {
			return nil, fmt.Errorf("updated_fields must be a non-empty array")
		}
		fieldValues := make([]map[string]any, 0, len(fields))
		for _, rawField := range fields {
			fieldValue, ok := rawField.(map[string]any)
			if !ok || fieldValue == nil {
				return nil, fmt.Errorf("updated_fields entries must be objects")
			}
			fieldValues = append(fieldValues, fieldValue)
		}
		return fieldValues, nil
	}

	rawUpdatedField, exists := args["updated_field"]
	if !exists {
		return nil, fmt.Errorf("missing required parameter: updated_field")
	}
	fieldValue, ok := rawUpdatedField.(map[string]any)
	if !ok || fieldValue == nil {
		return nil, fmt.Errorf("updated_field must be an object")
	}
	return []map[string]any{fieldValue}, nil
}

// buildUpdateProjectItem constructs UpdateProjectItemOptions from the input maps.
func buildUpdateProjectItem(inputs []map[string]any) (*github.UpdateProjectItemOptions, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("updated_field must be an object")
	}

	fields := make([]*github.UpdateProjectV2Field, 0, len(inputs))
	for index, input := range inputs {
		if input == nil {
			return nil, fmt.Errorf("updated_fields[%d] must be an object", index)
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

		fields = append(fields, &github.UpdateProjectV2Field{
			ID:    fieldID,
			Value: valueField,
		})
	}

	payload := &github.UpdateProjectItemOptions{
		Fields: fields,
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
