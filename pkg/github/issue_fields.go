package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
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
	DatabaseID  int64                          `json:"database_id,omitempty"`
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
// Only the fragment matching __typename is populated; read from the matching fragment.
// fullDatabaseId (BigInt scalar, returned as string) is fetched on each concrete type because
// shurcooL/githubv4 does not support interface fragments at the top level of a union.
type issueFieldNode struct {
	TypeName       githubv4.String `graphql:"__typename"`
	IssueFieldText struct {
		ID             githubv4.ID
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		Description    githubv4.String
		DataType       githubv4.String
		Visibility     githubv4.String
	} `graphql:"... on IssueFieldText"`
	IssueFieldNumber struct {
		ID             githubv4.ID
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		Description    githubv4.String
		DataType       githubv4.String
		Visibility     githubv4.String
	} `graphql:"... on IssueFieldNumber"`
	IssueFieldDate struct {
		ID             githubv4.ID
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		Description    githubv4.String
		DataType       githubv4.String
		Visibility     githubv4.String
	} `graphql:"... on IssueFieldDate"`
	IssueFieldSingleSelect struct {
		ID             githubv4.ID
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		Description    githubv4.String
		DataType       githubv4.String
		Visibility     githubv4.String
		Options        []struct {
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
// Gated by FeatureFlagIssueFields: the tool is only registered when the flag is on.
func ListIssueFields(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
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

			fields, err := fetchIssueFields(ctx, gqlClient, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to list issue fields", err), nil, nil
			}

			r, err := json.Marshal(fields)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal issue fields", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
	st.FeatureFlagEnable = FeatureFlagIssueFields
	return st
}

// fetchIssueFields returns the issue field definitions for the given owner.
// If repo is provided, fields are scoped to that repository (inherited from its
// organization); otherwise fields are returned directly from the organization.
func fetchIssueFields(ctx context.Context, gqlClient *githubv4.Client, owner, repo string) ([]IssueField, error) {
	ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")
	if repo != "" {
		var query issueFieldsRepoQuery
		vars := map[string]any{
			"owner": githubv4.String(owner),
			"name":  githubv4.String(repo),
		}
		if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
			return nil, err
		}
		return issueFieldsFromNodes(query.Repository.IssueFields.Nodes), nil
	}

	var query issueFieldsOrgQuery
	vars := map[string]any{
		"login": githubv4.String(owner),
	}
	if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
		return nil, err
	}
	return issueFieldsFromNodes(query.Organization.IssueFields.Nodes), nil
}

// issueFieldsFromNodes converts GraphQL issue field union nodes into IssueField values.
// Read from the fragment matching __typename; the other fragments are zero-valued.
func issueFieldsFromNodes(nodes []issueFieldNode) []IssueField {
	fields := make([]IssueField, 0, len(nodes))
	for _, node := range nodes {
		var f IssueField
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
				DatabaseID:  parseFullDatabaseID(string(node.IssueFieldSingleSelect.FullDatabaseID)),
				Name:        string(node.IssueFieldSingleSelect.Name),
				Description: string(node.IssueFieldSingleSelect.Description),
				DataType:    string(node.IssueFieldSingleSelect.DataType),
				Visibility:  string(node.IssueFieldSingleSelect.Visibility),
				Options:     opts,
			}
		case "IssueFieldText":
			f = IssueField{
				ID:          fmt.Sprintf("%v", node.IssueFieldText.ID),
				DatabaseID:  parseFullDatabaseID(string(node.IssueFieldText.FullDatabaseID)),
				Name:        string(node.IssueFieldText.Name),
				Description: string(node.IssueFieldText.Description),
				DataType:    string(node.IssueFieldText.DataType),
				Visibility:  string(node.IssueFieldText.Visibility),
			}
		case "IssueFieldNumber":
			f = IssueField{
				ID:          fmt.Sprintf("%v", node.IssueFieldNumber.ID),
				DatabaseID:  parseFullDatabaseID(string(node.IssueFieldNumber.FullDatabaseID)),
				Name:        string(node.IssueFieldNumber.Name),
				Description: string(node.IssueFieldNumber.Description),
				DataType:    string(node.IssueFieldNumber.DataType),
				Visibility:  string(node.IssueFieldNumber.Visibility),
			}
		case "IssueFieldDate":
			f = IssueField{
				ID:          fmt.Sprintf("%v", node.IssueFieldDate.ID),
				DatabaseID:  parseFullDatabaseID(string(node.IssueFieldDate.FullDatabaseID)),
				Name:        string(node.IssueFieldDate.Name),
				Description: string(node.IssueFieldDate.Description),
				DataType:    string(node.IssueFieldDate.DataType),
				Visibility:  string(node.IssueFieldDate.Visibility),
			}
		default:
			continue
		}
		fields = append(fields, f)
	}
	return fields
}

// parseFullDatabaseID converts a BigInt scalar string (e.g. "12345") to int64.
// Returns 0 if the string is empty or cannot be parsed.
func parseFullDatabaseID(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
