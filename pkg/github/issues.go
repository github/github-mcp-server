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
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// CloseIssueInput represents the input for closing an issue via the GraphQL API.
// Used to extend the functionality of the githubv4 library to support closing issues as duplicates.
type CloseIssueInput struct {
	IssueID          githubv4.ID             `json:"issueId"`
	ClientMutationID *githubv4.String        `json:"clientMutationId,omitempty"`
	StateReason      *IssueClosedStateReason `json:"stateReason,omitempty"`
	DuplicateIssueID *githubv4.ID            `json:"duplicateIssueId,omitempty"`
}

// IssueClosedStateReason represents the reason an issue was closed.
// Used to extend the functionality of the githubv4 library to support closing issues as duplicates.
type IssueClosedStateReason string

const (
	IssueClosedStateReasonCompleted  IssueClosedStateReason = "COMPLETED"
	IssueClosedStateReasonDuplicate  IssueClosedStateReason = "DUPLICATE"
	IssueClosedStateReasonNotPlanned IssueClosedStateReason = "NOT_PLANNED"
)

// fetchIssueIDs retrieves issue IDs via the GraphQL API.
// When duplicateOf is 0, it fetches only the main issue ID.
// When duplicateOf is non-zero, it fetches both the main issue and duplicate issue IDs in a single query.
func fetchIssueIDs(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueNumber int, duplicateOf int) (githubv4.ID, githubv4.ID, error) {
	// Build query variables common to both cases
	vars := map[string]interface{}{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if duplicateOf == 0 {
		// Only fetch the main issue ID
		var query struct {
			Repository struct {
				Issue struct {
					ID githubv4.ID
				} `graphql:"issue(number: $issueNumber)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}

		if err := gqlClient.Query(ctx, &query, vars); err != nil {
			return "", "", fmt.Errorf("failed to get issue ID")
		}

		return query.Repository.Issue.ID, "", nil
	}

	// Fetch both issue IDs in a single query
	var query struct {
		Repository struct {
			Issue struct {
				ID githubv4.ID
			} `graphql:"issue(number: $issueNumber)"`
			DuplicateIssue struct {
				ID githubv4.ID
			} `graphql:"duplicateIssue: issue(number: $duplicateOf)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Add duplicate issue number to variables
	vars["duplicateOf"] = githubv4.Int(duplicateOf) // #nosec G115 - issue numbers are always small positive integers

	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return "", "", fmt.Errorf("failed to get issue ID")
	}

	return query.Repository.Issue.ID, query.Repository.DuplicateIssue.ID, nil
}

// getCloseStateReason converts a string state reason to the appropriate enum value
func getCloseStateReason(stateReason string) IssueClosedStateReason {
	switch stateReason {
	case "not_planned":
		return IssueClosedStateReasonNotPlanned
	case "duplicate":
		return IssueClosedStateReasonDuplicate
	default: // Default to "completed" for empty or "completed" values
		return IssueClosedStateReasonCompleted
	}
}

// IssueFragment represents a fragment of an issue node in the GraphQL API.
type IssueFragment struct {
	Number     githubv4.Int
	Title      githubv4.String
	Body       githubv4.String
	State      githubv4.String
	DatabaseID int64

	Author struct {
		Login githubv4.String
	}
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	Labels    struct {
		Nodes []struct {
			Name        githubv4.String
			ID          githubv4.String
			Description githubv4.String
		}
	} `graphql:"labels(first: 100)"`
	Comments struct {
		TotalCount githubv4.Int
	} `graphql:"comments"`
}

// Common interface for all issue query types
type IssueQueryResult interface {
	GetIssueFragment() IssueQueryFragment
}

type IssueQueryFragment struct {
	Nodes    []IssueFragment `graphql:"nodes"`
	PageInfo struct {
		HasNextPage     githubv4.Boolean
		HasPreviousPage githubv4.Boolean
		StartCursor     githubv4.String
		EndCursor       githubv4.String
	}
	TotalCount int
}

// ListIssuesQuery is the root query structure for fetching issues with optional label filtering.
type ListIssuesQuery struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabels is the query structure for fetching issues with optional label filtering.
type ListIssuesQueryTypeWithLabels struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryWithSince is the query structure for fetching issues without label filtering but with since filtering.
type ListIssuesQueryWithSince struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabelsWithSince is the query structure for fetching issues with both label and since filtering.
type ListIssuesQueryTypeWithLabelsWithSince struct {
	Repository struct {
		Issues IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// Implement the interface for all query types
func (q *ListIssuesQueryTypeWithLabels) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQuery) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryTypeWithLabelsWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func getIssueQueryType(hasLabels bool, hasSince bool) any {
	switch {
	case hasLabels && hasSince:
		return &ListIssuesQueryTypeWithLabelsWithSince{}
	case hasLabels:
		return &ListIssuesQueryTypeWithLabels{}
	case hasSince:
		return &ListIssuesQueryWithSince{}
	default:
		return &ListIssuesQuery{}
	}
}

func fragmentToIssue(fragment IssueFragment) *github.Issue {
	// Convert GraphQL labels to GitHub API labels format
	var foundLabels []*github.Label
	for _, labelNode := range fragment.Labels.Nodes {
		foundLabels = append(foundLabels, &github.Label{
			Name:        github.Ptr(string(labelNode.Name)),
			NodeID:      github.Ptr(string(labelNode.ID)),
			Description: github.Ptr(string(labelNode.Description)),
		})
	}

	return &github.Issue{
		Number:    github.Ptr(int(fragment.Number)),
		Title:     github.Ptr(sanitize.Sanitize(string(fragment.Title))),
		CreatedAt: &github.Timestamp{Time: fragment.CreatedAt.Time},
		UpdatedAt: &github.Timestamp{Time: fragment.UpdatedAt.Time},
		User: &github.User{
			Login: github.Ptr(string(fragment.Author.Login)),
		},
		State:    github.Ptr(string(fragment.State)),
		ID:       github.Ptr(fragment.DatabaseID),
		Body:     github.Ptr(sanitize.Sanitize(string(fragment.Body))),
		Labels:   foundLabels,
		Comments: github.Ptr(int(fragment.Comments.TotalCount)),
	}
}

// IssueRead creates a tool to get details of a specific issue in a GitHub repository.
func IssueRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `The read operation to perform on a single issue.
Options are:
1. get - Get details of a specific issue.
2. get_comments - Get issue comments.
3. get_sub_issues - Get sub-issues of the issue.
4. get_labels - Get labels assigned to the issue.
`,
				Enum: []any{"get", "get_comments", "get_sub_issues", "get_labels"},
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
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_read",
			Description: t("TOOL_ISSUE_READ_DESCRIPTION", "Get information about a specific issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_READ_USER_TITLE", "Get issue details"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
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

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub graphql client", err), nil, nil
			}

			switch method {
			case "get":
				result, err := GetIssue(ctx, client, deps.GetRepoAccessCache(), owner, repo, issueNumber, deps.GetFlags())
				return result, nil, err
			case "get_comments":
				result, err := GetIssueComments(ctx, client, deps.GetRepoAccessCache(), owner, repo, issueNumber, pagination, deps.GetFlags())
				return result, nil, err
			case "get_sub_issues":
				result, err := GetSubIssues(ctx, client, deps.GetRepoAccessCache(), owner, repo, issueNumber, pagination, deps.GetFlags())
				return result, nil, err
			case "get_labels":
				result, err := GetIssueLabels(ctx, gqlClient, owner, repo, issueNumber)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
}

func GetIssue(ctx context.Context, client *github.Client, cache *lockdown.RepoAccessCache, owner string, repo string, issueNumber int, flags FeatureFlags) (*mcp.CallToolResult, error) {
	issue, resp, err := client.Issues.Get(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get issue", resp, body), nil
	}

	if flags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		login := issue.GetUser().GetLogin()
		if login != "" {
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if !isSafeContent {
				return utils.NewToolResultError("access to issue details is restricted by lockdown mode"), nil
			}
		}
	}

	// Sanitize title/body on response
	if issue != nil {
		if issue.Title != nil {
			issue.Title = github.Ptr(sanitize.Sanitize(*issue.Title))
		}
		if issue.Body != nil {
			issue.Body = github.Ptr(sanitize.Sanitize(*issue.Body))
		}
	}

	r, err := json.Marshal(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal issue: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetIssueComments(ctx context.Context, client *github.Client, cache *lockdown.RepoAccessCache, owner string, repo string, issueNumber int, pagination PaginationParams, flags FeatureFlags) (*mcp.CallToolResult, error) {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	comments, resp, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue comments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get issue comments", resp, body), nil
	}
	if flags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredComments := make([]*github.IssueComment, 0, len(comments))
		for _, comment := range comments {
			user := comment.User
			if user == nil {
				continue
			}
			login := user.GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if isSafeContent {
				filteredComments = append(filteredComments, comment)
			}
		}
		comments = filteredComments
	}

	r, err := json.Marshal(comments)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetSubIssues(ctx context.Context, client *github.Client, cache *lockdown.RepoAccessCache, owner string, repo string, issueNumber int, pagination PaginationParams, featureFlags FeatureFlags) (*mcp.CallToolResult, error) {
	opts := &github.IssueListOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	subIssues, resp, err := client.SubIssue.ListByIssue(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to list sub-issues",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list sub-issues", resp, body), nil
	}

	if featureFlags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredSubIssues := make([]*github.SubIssue, 0, len(subIssues))
		for _, subIssue := range subIssues {
			user := subIssue.User
			if user == nil {
				continue
			}
			login := user.GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if isSafeContent {
				filteredSubIssues = append(filteredSubIssues, subIssue)
			}
		}
		subIssues = filteredSubIssues
	}

	r, err := json.Marshal(subIssues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetIssueLabels(ctx context.Context, client *githubv4.Client, owner string, repo string, issueNumber int) (*mcp.CallToolResult, error) {
	// Get current labels on the issue using GraphQL
	var query struct {
		Repository struct {
			Issue struct {
				Labels struct {
					Nodes []struct {
						ID          githubv4.ID
						Name        githubv4.String
						Color       githubv4.String
						Description githubv4.String
					}
					TotalCount githubv4.Int
				} `graphql:"labels(first: 100)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]any{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if err := client.Query(ctx, &query, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to get issue labels", err), nil
	}

	// Extract label information
	issueLabels := make([]map[string]any, len(query.Repository.Issue.Labels.Nodes))
	for i, label := range query.Repository.Issue.Labels.Nodes {
		issueLabels[i] = map[string]any{
			"id":          fmt.Sprintf("%v", label.ID),
			"name":        string(label.Name),
			"color":       string(label.Color),
			"description": string(label.Description),
		}
	}

	response := map[string]any{
		"labels":     issueLabels,
		"totalCount": int(query.Repository.Issue.Labels.TotalCount),
	}

	out, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(out)), nil

}

// ListIssueTypes creates a tool to list defined issue types for an organization. This can be used to understand supported issue type values for creating or updating issues.
func ListIssueTypes(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issue_types",
			Description: t("TOOL_LIST_ISSUE_TYPES_FOR_ORG", "List supported issue types for repository owner (organization)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUE_TYPES_USER_TITLE", "List available issue types"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The organization owner of the repository",
					},
				},
				Required: []string{"owner"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			issueTypes, resp, err := client.Organizations.ListIssueTypes(ctx, owner)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to list issue types", err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list issue types", resp, body), nil, nil
			}

			r, err := json.Marshal(issueTypes)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal issue types", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// AddIssueComment creates a tool to add a comment to an issue.
func AddIssueComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "add_issue_comment",
			Description: t("TOOL_ADD_ISSUE_COMMENT_DESCRIPTION", "Add a comment to a specific issue in a GitHub repository. Use this tool to add comments to pull requests as well (in this case pass pull request number as issue_number), but only if user is not asking specifically to add review comments."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_ISSUE_COMMENT_USER_TITLE", "Add comment to issue"),
				ReadOnlyHint: false,
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number to comment on",
					},
					"body": {
						Type:        "string",
						Description: "Comment content",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "body"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			comment := &github.IssueComment{
				Body: github.Ptr(body),
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			createdComment, resp, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create comment", err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create comment", resp, body), nil, nil
			}

			r, err := json.Marshal(createdComment)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// SubIssueWrite creates a tool to add a sub-issue to a parent issue.
func SubIssueWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "sub_issue_write",
			Description: t("TOOL_SUB_ISSUE_WRITE_DESCRIPTION", "Add a sub-issue to a parent issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SUB_ISSUE_WRITE_USER_TITLE", "Change sub-issue"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `The action to perform on a single sub-issue
Options are:
- 'add' - add a sub-issue to a parent issue in a GitHub repository.
- 'remove' - remove a sub-issue from a parent issue in a GitHub repository.
- 'reprioritize' - change the order of sub-issues within a parent issue in a GitHub repository. Use either 'after_id' or 'before_id' to specify the new position.
				`,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "The number of the parent issue",
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to add. ID is not the same as issue number",
					},
					"replace_parent": {
						Type:        "boolean",
						Description: "When true, replaces the sub-issue's current parent issue. Use with 'add' method only.",
					},
					"after_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified)",
					},
					"before_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified)",
					},
				},
				Required: []string{"method", "owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
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
			subIssueID, err := RequiredInt(args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			replaceParent, err := OptionalParam[bool](args, "replace_parent")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			afterID, err := OptionalIntParam(args, "after_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			beforeID, err := OptionalIntParam(args, "before_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			switch strings.ToLower(method) {
			case "add":
				result, err := AddSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, replaceParent)
				return result, nil, err
			case "remove":
				// Call the remove sub-issue function
				result, err := RemoveSubIssue(ctx, client, owner, repo, issueNumber, subIssueID)
				return result, nil, err
			case "reprioritize":
				// Call the reprioritize sub-issue function
				result, err := ReprioritizeSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, afterID, beforeID)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
}

func AddSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int, replaceParent bool) (*mcp.CallToolResult, error) {
	subIssueRequest := github.SubIssueRequest{
		SubIssueID:    int64(subIssueID),
		ReplaceParent: github.Ptr(replaceParent),
	}

	subIssue, resp, err := client.SubIssue.Add(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to add sub-issue",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to add sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil

}

func RemoveSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int) (*mcp.CallToolResult, error) {
	subIssueRequest := github.SubIssueRequest{
		SubIssueID: int64(subIssueID),
	}

	subIssue, resp, err := client.SubIssue.Remove(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to remove sub-issue",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to remove sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func ReprioritizeSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int, afterID int, beforeID int) (*mcp.CallToolResult, error) {
	// Validate that either after_id or before_id is specified, but not both
	if afterID == 0 && beforeID == 0 {
		return utils.NewToolResultError("either after_id or before_id must be specified"), nil
	}
	if afterID != 0 && beforeID != 0 {
		return utils.NewToolResultError("only one of after_id or before_id should be specified, not both"), nil
	}

	subIssueRequest := github.SubIssueRequest{
		SubIssueID: int64(subIssueID),
	}

	if afterID != 0 {
		afterIDInt64 := int64(afterID)
		subIssueRequest.AfterID = &afterIDInt64
	}
	if beforeID != 0 {
		beforeIDInt64 := int64(beforeID)
		subIssueRequest.BeforeID = &beforeIDInt64
	}

	subIssue, resp, err := client.SubIssue.Reprioritize(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to reprioritize sub-issue",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to reprioritize sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

// SearchIssues creates a tool to search for issues.
func SearchIssues(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query using GitHub issues search syntax",
			},
			"owner": {
				Type:        "string",
				Description: "Optional repository owner. If provided with repo, only issues for this repository are listed.",
			},
			"repo": {
				Type:        "string",
				Description: "Optional repository name. If provided with owner, only issues for this repository are listed.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field by number of matches of categories, defaults to best match",
				Enum: []any{
					"comments",
					"reactions",
					"reactions-+1",
					"reactions--1",
					"reactions-smile",
					"reactions-thinking_face",
					"reactions-heart",
					"reactions-tada",
					"interactions",
					"created",
					"updated",
				},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "search_issues",
			Description: t("TOOL_SEARCH_ISSUES_DESCRIPTION", "Search for issues in GitHub repositories using issues search syntax already scoped to is:issue"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_ISSUES_USER_TITLE", "Search issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			result, err := searchHandler(ctx, deps.GetClient, args, "issue", "failed to search issues")
			return result, nil, err
		})
}

// IssueWrite creates a tool to create a new or update an existing issue in a GitHub repository.
func IssueWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_write",
			Description: t("TOOL_ISSUE_WRITE_DESCRIPTION", "Create a new or update an existing issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_WRITE_USER_TITLE", "Create or update issue."),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `Write operation to perform on a single issue.
Options are:
- 'create' - creates a new issue.
- 'update' - updates an existing issue.
`,
						Enum: []any{"create", "update"},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "Issue number to update",
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body content",
					},
					"assignees": {
						Type:        "array",
						Description: "Usernames to assign to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"labels": {
						Type:        "array",
						Description: "Labels to apply to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Milestone number",
					},
					"type": {
						Type:        "string",
						Description: "Type of this issue. Only use if the repository has issue types configured. Use list_issue_types tool to get valid type values for the organization. If the repository doesn't support issue types, omit this parameter.",
					},
					"state": {
						Type:        "string",
						Description: "New state",
						Enum:        []any{"open", "closed"},
					},
					"state_reason": {
						Type:        "string",
						Description: "Reason for the state change. Ignored unless state is changed.",
						Enum:        []any{"completed", "not_planned", "duplicate"},
					},
					"duplicate_of": {
						Type:        "number",
						Description: "Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'.",
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
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
			title, err := OptionalParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Optional parameters
			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get assignees
			assignees, err := OptionalStringArrayParam(args, "assignees")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get labels
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional milestone
			milestone, err := OptionalIntParam(args, "milestone")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var milestoneNum int
			if milestone != 0 {
				milestoneNum = milestone
			}

			// Get optional type
			issueType, err := OptionalParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Handle state, state_reason and duplicateOf parameters
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			stateReason, err := OptionalParam[string](args, "state_reason")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			duplicateOf, err := OptionalIntParam(args, "duplicate_of")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if duplicateOf != 0 && stateReason != "duplicate" {
				return utils.NewToolResultError("duplicate_of can only be used when state_reason is 'duplicate'"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GraphQL client", err), nil, nil
			}

			switch method {
			case "create":
				result, err := CreateIssue(ctx, client, owner, repo, title, body, assignees, labels, milestoneNum, issueType)
				return result, nil, err
			case "update":
				issueNumber, err := RequiredInt(args, "issue_number")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, err := UpdateIssue(ctx, client, gqlClient, owner, repo, issueNumber, title, body, assignees, labels, milestoneNum, issueType, state, stateReason, duplicateOf)
				return result, nil, err
			default:
				return utils.NewToolResultError("invalid method, must be either 'create' or 'update'"), nil, nil
			}
		})
}

func CreateIssue(ctx context.Context, client *github.Client, owner string, repo string, title string, body string, assignees []string, labels []string, milestoneNum int, issueType string) (*mcp.CallToolResult, error) {
	if title == "" {
		return utils.NewToolResultError("missing required parameter: title"), nil
	}

	// Create the issue request
	issueRequest := &github.IssueRequest{
		Title:     github.Ptr(title),
		Body:      github.Ptr(body),
		Assignees: &assignees,
		Labels:    &labels,
	}

	if milestoneNum != 0 {
		issueRequest.Milestone = &milestoneNum
	}

	if issueType != "" {
		issueRequest.Type = github.Ptr(issueType)
	}

	issue, resp, err := client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to create issue",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create issue", resp, body), nil
	}

	// Return minimal response with just essential information
	minimalResponse := MinimalResponse{
		ID:  fmt.Sprintf("%d", issue.GetID()),
		URL: issue.GetHTMLURL(),
	}

	r, err := json.Marshal(minimalResponse)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil
	}

	return utils.NewToolResultText(string(r)), nil
}

func UpdateIssue(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner string, repo string, issueNumber int, title string, body string, assignees []string, labels []string, milestoneNum int, issueType string, state string, stateReason string, duplicateOf int) (*mcp.CallToolResult, error) {
	// Create the issue request with only provided fields
	issueRequest := &github.IssueRequest{}

	// Set optional parameters if provided
	if title != "" {
		issueRequest.Title = github.Ptr(title)
	}

	if body != "" {
		issueRequest.Body = github.Ptr(body)
	}

	if len(labels) > 0 {
		issueRequest.Labels = &labels
	}

	if len(assignees) > 0 {
		issueRequest.Assignees = &assignees
	}

	if milestoneNum != 0 {
		issueRequest.Milestone = &milestoneNum
	}

	if issueType != "" {
		issueRequest.Type = github.Ptr(issueType)
	}

	updatedIssue, resp, err := client.Issues.Edit(ctx, owner, repo, issueNumber, issueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to update issue",
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
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update issue", resp, body), nil
	}

	// Use GraphQL API for state updates
	if state != "" {
		// Mandate specifying duplicateOf when trying to close as duplicate
		if state == "closed" && stateReason == "duplicate" && duplicateOf == 0 {
			return utils.NewToolResultError("duplicate_of must be provided when state_reason is 'duplicate'"), nil
		}

		// Get target issue ID (and duplicate issue ID if needed)
		issueID, duplicateIssueID, err := fetchIssueIDs(ctx, gqlClient, owner, repo, issueNumber, duplicateOf)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find issues", err), nil
		}

		switch state {
		case "open":
			// Use ReopenIssue mutation for opening
			var mutation struct {
				ReopenIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
						State  githubv4.String
					}
				} `graphql:"reopenIssue(input: $input)"`
			}

			err = gqlClient.Mutate(ctx, &mutation, githubv4.ReopenIssueInput{
				IssueID: issueID,
			}, nil)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to reopen issue", err), nil
			}
		case "closed":
			// Use CloseIssue mutation for closing
			var mutation struct {
				CloseIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
						State  githubv4.String
					}
				} `graphql:"closeIssue(input: $input)"`
			}

			stateReasonValue := getCloseStateReason(stateReason)
			closeInput := CloseIssueInput{
				IssueID:     issueID,
				StateReason: &stateReasonValue,
			}

			// Set duplicate issue ID if needed
			if stateReason == "duplicate" {
				closeInput.DuplicateIssueID = &duplicateIssueID
			}

			err = gqlClient.Mutate(ctx, &mutation, closeInput, nil)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to close issue", err), nil
			}
		}
	}

	// Return minimal response with just essential information
	minimalResponse := MinimalResponse{
		ID:  fmt.Sprintf("%d", updatedIssue.GetID()),
		URL: updatedIssue.GetHTMLURL(),
	}

	r, err := json.Marshal(minimalResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

// ListIssues creates a tool to list and filter repository issues
func ListIssues(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"state": {
				Type:        "string",
				Description: "Filter by state, by default both open and closed issues are returned when not provided",
				Enum:        []any{"OPEN", "CLOSED"},
			},
			"labels": {
				Type:        "array",
				Description: "Filter by labels",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"orderBy": {
				Type:        "string",
				Description: "Order issues by field. If provided, the 'direction' also needs to be provided.",
				Enum:        []any{"CREATED_AT", "UPDATED_AT", "COMMENTS"},
			},
			"direction": {
				Type:        "string",
				Description: "Order direction. If provided, the 'orderBy' also needs to be provided.",
				Enum:        []any{"ASC", "DESC"},
			},
			"since": {
				Type:        "string",
				Description: "Filter by date (ISO 8601 timestamp)",
			},
		},
		Required: []string{"owner", "repo"},
	}
	WithCursorPagination(schema)

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issues",
			Description: t("TOOL_LIST_ISSUES_DESCRIPTION", "List issues in a GitHub repository. For pagination, use the 'endCursor' from the previous response's 'pageInfo' in the 'after' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUES_USER_TITLE", "List issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set optional parameters if provided
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Normalize and filter by state
			state = strings.ToUpper(state)
			var states []githubv4.IssueState

			switch state {
			case "OPEN", "CLOSED":
				states = []githubv4.IssueState{githubv4.IssueState(state)}
			default:
				states = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
			}

			// Get labels
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			orderBy, err := OptionalParam[string](args, "orderBy")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Normalize and validate orderBy
			orderBy = strings.ToUpper(orderBy)
			switch orderBy {
			case "CREATED_AT", "UPDATED_AT", "COMMENTS":
				// Valid, keep as is
			default:
				orderBy = "CREATED_AT"
			}

			// Normalize and validate direction
			direction = strings.ToUpper(direction)
			switch direction {
			case "ASC", "DESC":
				// Valid, keep as is
			default:
				direction = "DESC"
			}

			since, err := OptionalParam[string](args, "since")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// There are two optional parameters: since and labels.
			var sinceTime time.Time
			var hasSince bool
			if since != "" {
				sinceTime, err = parseISOTimestamp(since)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to list issues: %s", err.Error())), nil, nil
				}
				hasSince = true
			}
			hasLabels := len(labels) > 0

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return nil, nil, err
			}

			// Check if someone tried to use page-based pagination instead of cursor-based
			if _, pageProvided := args["page"]; pageProvided {
				return utils.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil, nil
			}

			// Check if pagination parameters were explicitly provided
			_, perPageProvided := args["perPage"]
			paginationExplicit := perPageProvided

			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, nil, err
			}

			// Use default of 30 if pagination was not explicitly provided
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			vars := map[string]interface{}{
				"owner":     githubv4.String(owner),
				"repo":      githubv4.String(repo),
				"states":    states,
				"orderBy":   githubv4.IssueOrderField(orderBy),
				"direction": githubv4.OrderDirection(direction),
				"first":     githubv4.Int(*paginationParams.First),
			}

			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				// Used within query, therefore must be set to nil and provided as $after
				vars["after"] = (*githubv4.String)(nil)
			}

			// Ensure optional parameters are set
			if hasLabels {
				// Use query with labels filtering - convert string labels to githubv4.String slice
				labelStrings := make([]githubv4.String, len(labels))
				for i, label := range labels {
					labelStrings[i] = githubv4.String(label)
				}
				vars["labels"] = labelStrings
			}

			if hasSince {
				vars["since"] = githubv4.DateTime{Time: sinceTime}
			}

			issueQuery := getIssueQueryType(hasLabels, hasSince)
			if err := client.Query(ctx, issueQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(
					ctx,
					"failed to list issues",
					err,
				), nil, nil
			}

			// Extract and convert all issue nodes using the common interface
			var issues []*github.Issue
			var pageInfo struct {
				HasNextPage     githubv4.Boolean
				HasPreviousPage githubv4.Boolean
				StartCursor     githubv4.String
				EndCursor       githubv4.String
			}
			var totalCount int

			if queryResult, ok := issueQuery.(IssueQueryResult); ok {
				fragment := queryResult.GetIssueFragment()
				for _, issue := range fragment.Nodes {
					issues = append(issues, fragmentToIssue(issue))
				}
				pageInfo = fragment.PageInfo
				totalCount = fragment.TotalCount
			}

			// Create response with issues
			response := map[string]interface{}{
				"issues": issues,
				"pageInfo": map[string]interface{}{
					"hasNextPage":     pageInfo.HasNextPage,
					"hasPreviousPage": pageInfo.HasPreviousPage,
					"startCursor":     string(pageInfo.StartCursor),
					"endCursor":       string(pageInfo.EndCursor),
				},
				"totalCount": totalCount,
			}
			out, err := json.Marshal(response)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal issues: %w", err)
			}
			return utils.NewToolResultText(string(out)), nil, nil
		})
}

// mvpDescription is an MVP idea for generating tool descriptions from structured data in a shared format.
// It is not intended for widespread usage and is not a complete implementation.
type mvpDescription struct {
	summary        string
	outcomes       []string
	referenceLinks []string
}

func (d *mvpDescription) String() string {
	var sb strings.Builder
	sb.WriteString(d.summary)
	if len(d.outcomes) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString("This tool can help with the following outcomes:\n")
		for _, outcome := range d.outcomes {
			sb.WriteString(fmt.Sprintf("- %s\n", outcome))
		}
	}

	if len(d.referenceLinks) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString("More information can be found at:\n")
		for _, link := range d.referenceLinks {
			sb.WriteString(fmt.Sprintf("- %s\n", link))
		}
	}

	return sb.String()
}

func AssignCopilotToIssue(t translations.TranslationHelperFunc) inventory.ServerTool {
	description := mvpDescription{
		summary: "Assign Copilot to a specific issue in a GitHub repository.",
		outcomes: []string{
			"a Pull Request created with source code changes to resolve the issue",
		},
		referenceLinks: []string{
			"https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot",
		},
	}

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "assign_copilot_to_issue",
			Description: t("TOOL_ASSIGN_COPILOT_TO_ISSUE_DESCRIPTION", description.String()),
			Icons:       octicons.Icons("copilot"),
			Annotations: &mcp.ToolAnnotations{
				Title:          t("TOOL_ASSIGN_COPILOT_TO_ISSUE_USER_TITLE", "Assign Copilot to issue"),
				ReadOnlyHint:   false,
				IdempotentHint: true,
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number",
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
			},
		},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var params struct {
				Owner       string `mapstructure:"owner"`
				Repo        string `mapstructure:"repo"`
				IssueNumber int32  `mapstructure:"issue_number"`
			}
			if err := mapstructure.Decode(args, &params); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Firstly, we try to find the copilot bot in the suggested actors for the repository.
			// Although as I write this, we would expect copilot to be at the top of the list, in future, maybe
			// it will not be on the first page of responses, thus we will keep paginating until we find it.
			type botAssignee struct {
				ID       githubv4.ID
				Login    string
				TypeName string `graphql:"__typename"`
			}

			type suggestedActorsQuery struct {
				Repository struct {
					SuggestedActors struct {
						Nodes []struct {
							Bot botAssignee `graphql:"... on Bot"`
						}
						PageInfo struct {
							HasNextPage bool
							EndCursor   string
						}
					} `graphql:"suggestedActors(first: 100, after: $endCursor, capabilities: CAN_BE_ASSIGNED)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			variables := map[string]any{
				"owner":     githubv4.String(params.Owner),
				"name":      githubv4.String(params.Repo),
				"endCursor": (*githubv4.String)(nil),
			}

			var copilotAssignee *botAssignee
			for {
				var query suggestedActorsQuery
				err := client.Query(ctx, &query, variables)
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get suggested actors", err), nil, nil
				}

				// Iterate all the returned nodes looking for the copilot bot, which is supposed to have the
				// same name on each host. We need this in order to get the ID for later assignment.
				for _, node := range query.Repository.SuggestedActors.Nodes {
					if node.Bot.Login == "copilot-swe-agent" {
						copilotAssignee = &node.Bot
						break
					}
				}

				if !query.Repository.SuggestedActors.PageInfo.HasNextPage {
					break
				}
				variables["endCursor"] = githubv4.String(query.Repository.SuggestedActors.PageInfo.EndCursor)
			}

			// If we didn't find the copilot bot, we can't proceed any further.
			if copilotAssignee == nil {
				// The e2e tests depend upon this specific message to skip the test.
				return utils.NewToolResultError("copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot for more information."), nil, nil
			}

			// Next let's get the GQL Node ID and current assignees for this issue because the only way to
			// assign copilot is to use replaceActorsForAssignable which requires the full list.
			var getIssueQuery struct {
				Repository struct {
					Issue struct {
						ID        githubv4.ID
						Assignees struct {
							Nodes []struct {
								ID githubv4.ID
							}
						} `graphql:"assignees(first: 100)"`
					} `graphql:"issue(number: $number)"`
				} `graphql:"repository(owner: $owner, name: $name)"`
			}

			variables = map[string]any{
				"owner":  githubv4.String(params.Owner),
				"name":   githubv4.String(params.Repo),
				"number": githubv4.Int(params.IssueNumber),
			}

			if err := client.Query(ctx, &getIssueQuery, variables); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to get issue ID", err), nil, nil
			}

			// Finally, do the assignment. Just for reference, assigning copilot to an issue that it is already
			// assigned to seems to have no impact (which is a good thing).
			var assignCopilotMutation struct {
				ReplaceActorsForAssignable struct {
					Typename string `graphql:"__typename"` // Not required but we need a selector or GQL errors
				} `graphql:"replaceActorsForAssignable(input: $input)"`
			}

			actorIDs := make([]githubv4.ID, len(getIssueQuery.Repository.Issue.Assignees.Nodes)+1)
			for i, node := range getIssueQuery.Repository.Issue.Assignees.Nodes {
				actorIDs[i] = node.ID
			}
			actorIDs[len(getIssueQuery.Repository.Issue.Assignees.Nodes)] = copilotAssignee.ID

			if err := client.Mutate(
				ctx,
				&assignCopilotMutation,
				ReplaceActorsForAssignableInput{
					AssignableID: getIssueQuery.Repository.Issue.ID,
					ActorIDs:     actorIDs,
				},
				nil,
			); err != nil {
				return nil, nil, fmt.Errorf("failed to replace actors for assignable: %w", err)
			}

			return utils.NewToolResultText("successfully assigned copilot to issue"), nil, nil
		})
}

type ReplaceActorsForAssignableInput struct {
	AssignableID githubv4.ID   `json:"assignableId"`
	ActorIDs     []githubv4.ID `json:"actorIds"`
}

// parseISOTimestamp parses an ISO 8601 timestamp string into a time.Time object.
// Returns the parsed time or an error if parsing fails.
// Example formats supported: "2023-01-15T14:30:00Z", "2023-01-15"
func parseISOTimestamp(timestamp string) (time.Time, error) {
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Try RFC3339 format (standard ISO 8601 with time)
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t, nil
	}

	// Try simple date format (YYYY-MM-DD)
	t, err = time.Parse("2006-01-02", timestamp)
	if err == nil {
		return t, nil
	}

	// Return error with supported formats
	return time.Time{}, fmt.Errorf("invalid ISO 8601 timestamp: %s (supported formats: YYYY-MM-DDThh:mm:ssZ or YYYY-MM-DD)", timestamp)
}

func AssignCodingAgentPrompt(t translations.TranslationHelperFunc) inventory.ServerPrompt {
	return inventory.NewServerPrompt(
		ToolsetMetadataIssues,
		mcp.Prompt{
			Name:        "AssignCodingAgent",
			Description: t("PROMPT_ASSIGN_CODING_AGENT_DESCRIPTION", "Assign GitHub Coding Agent to multiple tasks in a GitHub repository."),
			Arguments: []*mcp.PromptArgument{
				{
					Name:        "repo",
					Description: "The repository to assign tasks in (owner/repo).",
					Required:    true,
				},
			},
		},
		func(_ context.Context, request *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			repo := request.Params.Arguments["repo"]

			messages := []*mcp.PromptMessage{
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "You are a personal assistant for GitHub the Copilot GitHub Coding Agent. Your task is to help the user assign tasks to the Coding Agent based on their open GitHub issues. You can use `assign_copilot_to_issue` tool to assign the Coding Agent to issues that are suitable for autonomous work, and `search_issues` tool to find issues that match the user's criteria. You can also use `list_issues` to get a list of issues in the repository.",
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Please go and get a list of the most recent 10 issues from the %s GitHub repository", repo),
					},
				},
				{
					Role: "assistant",
					Content: &mcp.TextContent{
						Text: fmt.Sprintf("Sure! I will get a list of the 10 most recent issues for the repo %s.", repo),
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "For each issue, please check if it is a clearly defined coding task with acceptance criteria and a low to medium complexity to identify issues that are suitable for an AI Coding Agent to work on. Then assign each of the identified issues to Copilot.",
					},
				},
				{
					Role: "assistant",
					Content: &mcp.TextContent{
						Text: "Certainly! Let me carefully check which ones are clearly scoped issues that are good to assign to the coding agent, and I will summarize and assign them now.",
					},
				},
				{
					Role: "user",
					Content: &mcp.TextContent{
						Text: "Great, if you are unsure if an issue is good to assign, ask me first, rather than assigning copilot. If you are certain the issue is clear and suitable you can assign it to Copilot without asking.",
					},
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		},
	)
}
