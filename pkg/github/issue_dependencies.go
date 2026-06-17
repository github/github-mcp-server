package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// AddBlockedByInput represents the input for the addBlockedBy GraphQL mutation.
// The pinned githubv4 library predates the dependency mutations, so the input
// type is declared here. The Go type name must match the GraphQL input type name.
type AddBlockedByInput struct {
	IssueID          githubv4.ID      `json:"issueId"`
	BlockingIssueID  githubv4.ID      `json:"blockingIssueId"`
	ClientMutationID *githubv4.String `json:"clientMutationId,omitempty"`
}

// RemoveBlockedByInput represents the input for the removeBlockedBy GraphQL mutation.
type RemoveBlockedByInput struct {
	IssueID          githubv4.ID      `json:"issueId"`
	BlockingIssueID  githubv4.ID      `json:"blockingIssueId"`
	ClientMutationID *githubv4.String `json:"clientMutationId,omitempty"`
}

// dependencyIssueNode is the minimal projection returned for each related issue
// in a blocked-by / blocking listing.
type dependencyIssueNode struct {
	Number     githubv4.Int
	Title      githubv4.String
	State      githubv4.String
	URL        githubv4.String
	Repository struct {
		NameWithOwner githubv4.String
	}
}

// dependencyConnection mirrors the shape of an IssueConnection returned by the
// blockedBy / blocking fields.
type dependencyConnection struct {
	TotalCount githubv4.Int
	PageInfo   struct {
		HasNextPage githubv4.Boolean
		EndCursor   githubv4.String
	}
	Nodes []dependencyIssueNode
}

// minimalDependencyIssue is the JSON-serialised form of a related issue.
type minimalDependencyIssue struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	State      string `json:"state"`
	URL        string `json:"url"`
	Repository string `json:"repository"`
}

// IssueDependencyRead creates a tool to read an issue's blocked-by and blocking
// relationships. It is a separate, feature-flagged tool (rather than a method on
// the default issue_read) so the whole dependency capability can be gated as a
// unit without enlarging the default issue tool surface.
func IssueDependencyRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `The read operation to perform on a single issue's dependencies.
Options are:
1. get_blocked_by - List the issues that block this issue (this issue is blocked by them).
2. get_blocking - List the issues that this issue blocks.
`,
				Enum: []any{"get_blocked_by", "get_blocking"},
			},
			"owner": {
				Type:        "string",
				Description: "The owner of the repository",
			},
			"repo": {
				Type:        "string",
				Description: "The name of the repository",
			},
			"issue_number": {
				Type:        "number",
				Description: "The number of the issue",
			},
		},
		Required: []string{"method", "owner", "repo", "issue_number"},
	}
	WithCursorPagination(schema)

	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_dependency_read",
			Description: t("TOOL_ISSUE_DEPENDENCY_READ_DESCRIPTION", "Read an issue's dependency relationships in a GitHub repository: the issues that block it (blocked_by) or the issues it blocks (blocking)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_DEPENDENCY_READ_USER_TITLE", "Read issue dependencies"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if _, pageProvided := args["page"]; pageProvided {
				return utils.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil, nil
			}
			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			gqlPagination, err := pagination.ToGraphQLParams()
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			switch method {
			case "get_blocked_by":
				result, err := GetIssueBlockedBy(ctx, gqlClient, owner, repo, issueNumber, gqlPagination)
				return result, nil, err
			case "get_blocking":
				result, err := GetIssueBlocking(ctx, gqlClient, owner, repo, issueNumber, gqlPagination)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
	st.FeatureFlagEnable = FeatureFlagIssueDependencies
	return st
}

func dependencyQueryVars(owner, repo string, issueNumber int, pagination *GraphQLPaginationParams) map[string]any {
	vars := map[string]any{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
		"first":       githubv4.Int(*pagination.First),
	}
	if pagination.After != nil {
		vars["after"] = githubv4.String(*pagination.After)
	} else {
		vars["after"] = (*githubv4.String)(nil)
	}
	return vars
}

func dependencyResult(conn dependencyConnection) *mcp.CallToolResult {
	issues := make([]minimalDependencyIssue, 0, len(conn.Nodes))
	for _, node := range conn.Nodes {
		issues = append(issues, minimalDependencyIssue{
			Number:     int(node.Number),
			Title:      string(node.Title),
			State:      string(node.State),
			URL:        string(node.URL),
			Repository: string(node.Repository.NameWithOwner),
		})
	}
	return MarshalledTextResult(map[string]any{
		"issues":     issues,
		"totalCount": int(conn.TotalCount),
		"pageInfo": map[string]any{
			"hasNextPage": bool(conn.PageInfo.HasNextPage),
			"endCursor":   string(conn.PageInfo.EndCursor),
		},
	})
}

// GetIssueBlockedBy lists the issues that block the given issue.
func GetIssueBlockedBy(ctx context.Context, client *githubv4.Client, owner, repo string, issueNumber int, pagination *GraphQLPaginationParams) (*mcp.CallToolResult, error) {
	var query struct {
		Repository struct {
			Issue struct {
				BlockedBy dependencyConnection `graphql:"blockedBy(first: $first, after: $after)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	if err := client.Query(ctx, &query, dependencyQueryVars(owner, repo, issueNumber, pagination)); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get blocked-by issues", err), nil
	}
	return dependencyResult(query.Repository.Issue.BlockedBy), nil
}

// GetIssueBlocking lists the issues that the given issue blocks.
func GetIssueBlocking(ctx context.Context, client *githubv4.Client, owner, repo string, issueNumber int, pagination *GraphQLPaginationParams) (*mcp.CallToolResult, error) {
	var query struct {
		Repository struct {
			Issue struct {
				Blocking dependencyConnection `graphql:"blocking(first: $first, after: $after)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	if err := client.Query(ctx, &query, dependencyQueryVars(owner, repo, issueNumber, pagination)); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get blocking issues", err), nil
	}
	return dependencyResult(query.Repository.Issue.Blocking), nil
}

// IssueDependencyWrite creates a tool to add or remove an issue dependency
// (blocked-by / blocking) relationship. It accepts issue numbers and resolves
// them to GraphQL node IDs before calling the addBlockedBy / removeBlockedBy
// mutations. "blocking" is the inverse of "blocked_by", so both directions are
// served by the same mutation pair with the issue arguments swapped.
func IssueDependencyWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name: "issue_dependency_write",
			Description: t("TOOL_ISSUE_DEPENDENCY_WRITE_DESCRIPTION",
				"Add or remove an issue dependency relationship in a GitHub repository. "+
					"Use type 'blocked_by' to record that the subject issue is blocked by a related issue, "+
					"or type 'blocking' to record that the subject issue blocks a related issue. "+
					"The related issue defaults to the same repository as the subject unless related_owner/related_repo are provided."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_DEPENDENCY_WRITE_USER_TITLE", "Change issue dependency"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `The action to perform.
Options are:
- 'add' - create the dependency relationship.
- 'remove' - delete the dependency relationship.`,
						Enum: []any{"add", "remove"},
					},
					"type": {
						Type: "string",
						Description: `The relationship direction relative to the subject issue.
Options are:
- 'blocked_by' - the subject issue is blocked by the related issue.
- 'blocking' - the subject issue blocks the related issue.`,
						Enum: []any{"blocked_by", "blocking"},
					},
					"owner": {
						Type:        "string",
						Description: "The owner of the subject issue's repository",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the subject issue's repository",
					},
					"issue_number": {
						Type:        "number",
						Description: "The number of the subject issue",
					},
					"related_issue_number": {
						Type:        "number",
						Description: "The number of the related issue to link or unlink",
					},
					"related_owner": {
						Type:        "string",
						Description: "The owner of the related issue's repository. Defaults to 'owner' when omitted.",
					},
					"related_repo": {
						Type:        "string",
						Description: "The name of the related issue's repository. Defaults to 'repo' when omitted.",
					},
				},
				Required: []string{"method", "type", "owner", "repo", "issue_number", "related_issue_number"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			relationshipType, err := RequiredParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			relatedIssueNumber, err := RequiredInt(args, "related_issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			relatedOwner, err := OptionalParam[string](args, "related_owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			relatedRepo, err := OptionalParam[string](args, "related_repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if relatedOwner == "" {
				relatedOwner = owner
			}
			if relatedRepo == "" {
				relatedRepo = repo
			}

			method = strings.ToLower(method)
			relationshipType = strings.ToLower(relationshipType)
			if method != "add" && method != "remove" {
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
			if relationshipType != "blocked_by" && relationshipType != "blocking" {
				return utils.NewToolResultError(fmt.Sprintf("unknown type: %s", relationshipType)), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), nil, nil
			}

			subjectID, relatedID, err := resolveIssueNodeIDs(ctx, gqlClient, owner, repo, issueNumber, relatedOwner, relatedRepo, relatedIssueNumber)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to resolve issues", err), nil, nil
			}

			// "A blocks B" is recorded as addBlockedBy(issueId: B, blockingIssueId: A).
			// For type 'blocked_by' the subject is blocked by the related issue.
			// For type 'blocking' the subject blocks the related issue, so the roles swap.
			blockedID, blockingID := subjectID, relatedID
			if relationshipType == "blocking" {
				blockedID, blockingID = relatedID, subjectID
			}

			result, err := writeBlockedByRelationship(ctx, gqlClient, method, blockedID, blockingID)
			return result, nil, err
		})
	st.FeatureFlagEnable = FeatureFlagIssueDependencies
	return st
}

// resolveIssueNodeIDs resolves the subject and related issue numbers to their
// GraphQL node IDs in a single aliased query.
func resolveIssueNodeIDs(ctx context.Context, client *githubv4.Client, owner, repo string, issueNumber int, relatedOwner, relatedRepo string, relatedIssueNumber int) (githubv4.ID, githubv4.ID, error) {
	var query struct {
		Subject struct {
			Issue struct {
				ID githubv4.ID
			} `graphql:"issue(number: $subjectNumber)"`
		} `graphql:"subject: repository(owner: $subjectOwner, name: $subjectRepo)"`
		Related struct {
			Issue struct {
				ID githubv4.ID
			} `graphql:"issue(number: $relatedNumber)"`
		} `graphql:"related: repository(owner: $relatedOwner, name: $relatedRepo)"`
	}
	vars := map[string]any{
		"subjectOwner":  githubv4.String(owner),
		"subjectRepo":   githubv4.String(repo),
		"subjectNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
		"relatedOwner":  githubv4.String(relatedOwner),
		"relatedRepo":   githubv4.String(relatedRepo),
		"relatedNumber": githubv4.Int(relatedIssueNumber), // #nosec G115 - issue numbers are always small positive integers
	}
	if err := client.Query(ctx, &query, vars); err != nil {
		return "", "", err
	}
	return query.Subject.Issue.ID, query.Related.Issue.ID, nil
}

// writeBlockedByRelationship runs the addBlockedBy / removeBlockedBy mutation and
// returns a minimal description of the affected issues.
func writeBlockedByRelationship(ctx context.Context, client *githubv4.Client, method string, blockedID, blockingID githubv4.ID) (*mcp.CallToolResult, error) {
	type mutationIssue struct {
		Number githubv4.Int
		URL    githubv4.String
	}

	switch method {
	case "add":
		var mutation struct {
			AddBlockedBy struct {
				Issue         mutationIssue
				BlockingIssue mutationIssue
			} `graphql:"addBlockedBy(input: $input)"`
		}
		input := AddBlockedByInput{IssueID: blockedID, BlockingIssueID: blockingID}
		if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to add issue dependency", err), nil
		}
		return MarshalledTextResult(map[string]any{
			"message":        "dependency added",
			"blocked_issue":  map[string]any{"number": int(mutation.AddBlockedBy.Issue.Number), "url": string(mutation.AddBlockedBy.Issue.URL)},
			"blocking_issue": map[string]any{"number": int(mutation.AddBlockedBy.BlockingIssue.Number), "url": string(mutation.AddBlockedBy.BlockingIssue.URL)},
		}), nil
	case "remove":
		var mutation struct {
			RemoveBlockedBy struct {
				Issue         mutationIssue
				BlockingIssue mutationIssue
			} `graphql:"removeBlockedBy(input: $input)"`
		}
		input := RemoveBlockedByInput{IssueID: blockedID, BlockingIssueID: blockingID}
		if err := client.Mutate(ctx, &mutation, input, nil); err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to remove issue dependency", err), nil
		}
		return MarshalledTextResult(map[string]any{
			"message":        "dependency removed",
			"blocked_issue":  map[string]any{"number": int(mutation.RemoveBlockedBy.Issue.Number), "url": string(mutation.RemoveBlockedBy.Issue.URL)},
			"blocking_issue": map[string]any{"number": int(mutation.RemoveBlockedBy.BlockingIssue.Number), "url": string(mutation.RemoveBlockedBy.BlockingIssue.URL)},
		}), nil
	default:
		return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil
	}
}
