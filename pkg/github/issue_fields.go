package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// IssueField represents 一个仓库 议题 field definition.
type IssueField struct {
	ID          string                         `json:"id"`
	DatabaseID  int64                          `json:"full_database_id,omitempty"`
	Name        string                         `json:"name"`
	Description string                         `json:"description,omitempty"`
	DataType    string                         `json:"data_type"`
	Visibility  string                         `json:"visibility"`
	Options     []IssueSingleSelectFieldOption `json:"options,omitempty"`
}

// IssueSingleSelectFieldOption represents 一个option f或一个单个_select 议题 field.
type IssueSingleSelectFieldOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color"`
	Priority    *int   `json:"priority,omitempty"`
}

// 议题FieldNode is GraphQL fragment f或一个单个 议题 field 在IssueFields union.
// 仅fragment matching __typename is populated; 读取 来自matching fragment.
// fullDatabaseId (BigInt scalar, 返回ed as string) is fetched on 每个concrete type because
// shurcooL/githubv4 does 不support interface fragments at top level of 一个union.
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

// 议题FieldsRepoQuery is GraphQL query f或列出ing 议题 fields on 一个仓库.
type issueFieldsRepoQuery struct {
	Repository struct {
		IssueFields struct {
			Nodes []issueFieldNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"repository(owner: $owner, name: $name)"`
}

// 议题FieldsOrgQuery is GraphQL query f或列出ing 议题 fields on 一个organization.
type issueFieldsOrgQuery struct {
	Organization struct {
		IssueFields struct {
			Nodes []issueFieldNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"organization(login: $login)"`
}

// ListIssueFields 创建一个工具以 列出 议题 field definitions f或一个仓库 或organization.
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

			result := utils.NewToolResultText(string(r))
			// Issue field definitions are repo/org structural 元数据
			// (受信任). When scoped to 一个specific repo, confidentiality
			// follows that repo's visibility; f或一个org-level lookup (no
			// repo) it is conservatively treated as 私有.
			if repo == "" {
				result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelRepoMetadata(true))
			} else {
				result = attachRepoVisibilityIFCLabelLazy(ctx, deps, owner, repo, result, ifc.LabelRepoMetadata)
			}
			return result, nil, nil
		})
	return st
}

// fetchIssueFields 返回 议题 field definitions 用于given owner.
// If repo is provided, fields are scoped to that 仓库 (inherited from its
// organization); otherwise fields are 返回ed directly 来自organization.
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

// 议题FieldsFromNodes converts GraphQL 议题 field union nodes into IssueField 值.
// Read 来自fragment matching __typename; other fragments are zero-值d.
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

// parseFullDatabaseID converts 一个BigInt scalar string (e.g. "12345") to int64.
// Returns 0 如果string is 空 或can不be parsed.
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
