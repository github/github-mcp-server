package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// IssueField represents a repository issue field definition.
type IssueField struct {
	ID          string                         `json:"id"`
	Name        string                         `json:"name"`
	Description string                         `json:"description,omitempty"`
	DataType    string                         `json:"data_type"`
	Visibility  string                         `json:"visibility"`
	Options     []IssueSingleSelectFieldOption `json:"options,omitempty"`
}

// IssueSingleSelectFieldOption represents an option for a single_select issue field.
type IssueSingleSelectFieldOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color"`
	Priority    *int   `json:"priority,omitempty"`
}

// issueFieldNode is the GraphQL fragment for a single issue field in the IssueFields union.
type issueFieldNode struct {
	TypeName githubv4.String `graphql:"__typename"`
	// All field types share these fields; any populated fragment gives the same values.
	IssueFieldText struct {
		ID          githubv4.ID
		Name        githubv4.String
		Description githubv4.String
		DataType    githubv4.String
		Visibility  githubv4.String
	} `graphql:"... on IssueFieldText"`
	IssueFieldNumber struct {
		ID          githubv4.ID
		Name        githubv4.String
		Description githubv4.String
		DataType    githubv4.String
		Visibility  githubv4.String
	} `graphql:"... on IssueFieldNumber"`
	IssueFieldDate struct {
		ID          githubv4.ID
		Name        githubv4.String
		Description githubv4.String
		DataType    githubv4.String
		Visibility  githubv4.String
	} `graphql:"... on IssueFieldDate"`
	IssueFieldSingleSelect struct {
		ID          githubv4.ID
		Name        githubv4.String
		Description githubv4.String
		DataType    githubv4.String
		Visibility  githubv4.String
		Options     []struct {
			ID          githubv4.ID
			Name        githubv4.String
			Description githubv4.String
			Color       githubv4.String
			Priority    *int
		}
	} `graphql:"... on IssueFieldSingleSelect"`
}

// issueFieldsRepoQuery is the GraphQL query for listing issue fields on a repository.
type issueFieldsRepoQuery struct {
	Repository struct {
		IssueFields struct {
			Nodes []issueFieldNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// issueFieldsOrgQuery is the GraphQL query for listing issue fields on an organization.
type issueFieldsOrgQuery struct {
	Organization struct {
		IssueFields struct {
			Nodes []issueFieldNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"organization(login: $login)"`
}

// ListIssueFields creates a tool to list issue field definitions for a repository or organization.
func ListIssueFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issue_fields",
			Description: t("TOOL_LIST_ISSUE_FIELDS_DESCRIPTION", "List issue fields for a repository or organization. Returns field definitions including name, type (text, number, date, single_select), and for single_select fields the list of valid option names. When repo is omitted, returns org-level fields directly."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUE_FIELDS_USER_TITLE", "List issue fields"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The account owner of the repository or organization. The name is not case sensitive.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository. When provided, returns fields for this specific repository (inherited from its organization). When omitted, returns org-level fields directly.",
					},
				},
				Required: []string{"owner"},
			},
		},
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields")
			var nodes []issueFieldNode
			if repo != "" {
				var query issueFieldsRepoQuery
				vars := map[string]any{
					"owner": githubv4.String(owner),
					"name":  githubv4.String(repo),
				}
				if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
					return utils.NewToolResultErrorFromErr("failed to list issue fields", err), nil, nil
				}
				nodes = query.Repository.IssueFields.Nodes
			} else {
				var query issueFieldsOrgQuery
				vars := map[string]any{
					"login": githubv4.String(owner),
				}
				if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
					return utils.NewToolResultErrorFromErr("failed to list issue fields", err), nil, nil
				}
				nodes = query.Organization.IssueFields.Nodes
			}

			fields := make([]IssueField, 0, len(nodes))
			for _, node := range nodes {
				var f IssueField
				// Use TypeName to discriminate; shurcooL populates all matching fragment structs,
				// so any non-SingleSelect struct gives the correct shared field values.
				switch string(node.TypeName) {
				case "IssueFieldSingleSelect":
					opts := make([]IssueSingleSelectFieldOption, 0, len(node.IssueFieldSingleSelect.Options))
					for _, o := range node.IssueFieldSingleSelect.Options {
						opts = append(opts, IssueSingleSelectFieldOption{
							ID:          fmt.Sprintf("%v", o.ID),
							Name:        string(o.Name),
							Description: string(o.Description),
							Color:       string(o.Color),
							Priority:    o.Priority,
						})
					}
					f = IssueField{
						ID:          fmt.Sprintf("%v", node.IssueFieldSingleSelect.ID),
						Name:        string(node.IssueFieldSingleSelect.Name),
						Description: string(node.IssueFieldSingleSelect.Description),
						DataType:    string(node.IssueFieldSingleSelect.DataType),
						Visibility:  string(node.IssueFieldSingleSelect.Visibility),
						Options:     opts,
					}
				case "IssueFieldText", "IssueFieldNumber", "IssueFieldDate":
					f = IssueField{
						ID:          fmt.Sprintf("%v", node.IssueFieldText.ID),
						Name:        string(node.IssueFieldText.Name),
						Description: string(node.IssueFieldText.Description),
						DataType:    string(node.IssueFieldText.DataType),
						Visibility:  string(node.IssueFieldText.Visibility),
					}
				default:
					continue
				}
				fields = append(fields, f)
			}

			r, err := json.Marshal(fields)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal issue fields", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}
