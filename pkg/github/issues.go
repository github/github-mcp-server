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
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
		Title:     github.Ptr(string(fragment.Title)),
		CreatedAt: &github.Timestamp{Time: fragment.CreatedAt.Time},
		UpdatedAt: &github.Timestamp{Time: fragment.UpdatedAt.Time},
		User: &github.User{
			Login: github.Ptr(string(fragment.Author.Login)),
		},
		State:    github.Ptr(string(fragment.State)),
		ID:       github.Ptr(fragment.DatabaseID),
		Body:     github.Ptr(string(fragment.Body)),
		Labels:   foundLabels,
		Comments: github.Ptr(int(fragment.Comments.TotalCount)),
	}
}

// GetIssue creates a tool to get details of a specific issue in a GitHub repository.
func GetIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_issue",
			mcp.WithDescription(t("TOOL_GET_ISSUE_DESCRIPTION", "Get details of a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ISSUE_USER_TITLE", "Get issue details"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The owner of the repository"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("The name of the repository"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the issue"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get issue: %s", string(body))), nil
			}

			r, err := json.Marshal(issue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal issue: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListIssueTypes creates a tool to list defined issue types for an organization. This can be used to understand supported issue type values for creating or updating issues.
func ListIssueTypes(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {

	return mcp.NewTool("list_issue_types",
			mcp.WithDescription(t("TOOL_LIST_ISSUE_TYPES_FOR_ORG", "List supported issue types for repository owner (organization).")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ISSUE_TYPES_USER_TITLE", "List available issue types"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("The organization owner of the repository"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			issueTypes, resp, err := client.Organizations.ListIssueTypes(ctx, owner)
			if err != nil {
				return nil, fmt.Errorf("failed to list issue types: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to list issue types: %s", string(body))), nil
			}

			r, err := json.Marshal(issueTypes)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal issue types: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddIssueComment creates a tool to add a comment to an issue.
func AddIssueComment(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_issue_comment",
			mcp.WithDescription(t("TOOL_ADD_ISSUE_COMMENT_DESCRIPTION", "Add a comment to a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_ISSUE_COMMENT_USER_TITLE", "Add comment to issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number to comment on"),
			),
			mcp.WithString("body",
				mcp.Required(),
				mcp.Description("Comment content"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			body, err := RequiredParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			comment := &github.IssueComment{
				Body: github.Ptr(body),
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			createdComment, resp, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
			if err != nil {
				return nil, fmt.Errorf("failed to create comment: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create comment: %s", string(body))), nil
			}

			r, err := json.Marshal(createdComment)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// AddSubIssue creates a tool to add a sub-issue to a parent issue.
func AddSubIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("add_sub_issue",
			mcp.WithDescription(t("TOOL_ADD_SUB_ISSUE_DESCRIPTION", "Add a sub-issue to a parent issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ADD_SUB_ISSUE_USER_TITLE", "Add sub-issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the parent issue"),
			),
			mcp.WithNumber("sub_issue_id",
				mcp.Required(),
				mcp.Description("The ID of the sub-issue to add. ID is not the same as issue number"),
			),
			mcp.WithBoolean("replace_parent",
				mcp.Description("When true, replaces the sub-issue's current parent issue"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			subIssueID, err := RequiredInt(request, "sub_issue_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			replaceParent, err := OptionalParam[bool](request, "replace_parent")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			subIssueRequest := github.SubIssueRequest{
				SubIssueID:    int64(subIssueID),
				ReplaceParent: ToBoolPtr(replaceParent),
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to add sub-issue: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListSubIssues creates a tool to list sub-issues for a GitHub issue.
func ListSubIssues(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_sub_issues",
			mcp.WithDescription(t("TOOL_LIST_SUB_ISSUES_DESCRIPTION", "List sub-issues for a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_SUB_ISSUES_USER_TITLE", "List sub-issues"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination (default: 1)"),
			),
			mcp.WithNumber("per_page",
				mcp.Description("Number of results per page (max 100, default: 30)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			page, err := OptionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := OptionalIntParamWithDefault(request, "per_page", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.IssueListOptions{
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: perPage,
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to list sub-issues: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssues)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}

}

// RemoveSubIssue creates a tool to remove a sub-issue from a parent issue.
// Unlike other sub-issue tools, this currently uses a direct HTTP DELETE request
// because of a bug in the go-github library.
// Once the fix is released, this can be updated to use the library method.
// See: https://github.com/google/go-github/pull/3613
func RemoveSubIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("remove_sub_issue",
			mcp.WithDescription(t("TOOL_REMOVE_SUB_ISSUE_DESCRIPTION", "Remove a sub-issue from a parent issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_REMOVE_SUB_ISSUE_USER_TITLE", "Remove sub-issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the parent issue"),
			),
			mcp.WithNumber("sub_issue_id",
				mcp.Required(),
				mcp.Description("The ID of the sub-issue to remove. ID is not the same as issue number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			subIssueID, err := RequiredInt(request, "sub_issue_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

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
				return mcp.NewToolResultError(fmt.Sprintf("failed to remove sub-issue: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ReprioritizeSubIssue creates a tool to reprioritize a sub-issue to a different position in the parent list.
func ReprioritizeSubIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("reprioritize_sub_issue",
			mcp.WithDescription(t("TOOL_REPRIORITIZE_SUB_ISSUE_DESCRIPTION", "Reprioritize a sub-issue to a different position in the parent issue's sub-issue list.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_REPRIORITIZE_SUB_ISSUE_USER_TITLE", "Reprioritize sub-issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("The number of the parent issue"),
			),
			mcp.WithNumber("sub_issue_id",
				mcp.Required(),
				mcp.Description("The ID of the sub-issue to reprioritize. ID is not the same as issue number"),
			),
			mcp.WithNumber("after_id",
				mcp.Description("The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified)"),
			),
			mcp.WithNumber("before_id",
				mcp.Description("The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			subIssueID, err := RequiredInt(request, "sub_issue_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Handle optional positioning parameters
			afterID, err := OptionalIntParam(request, "after_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			beforeID, err := OptionalIntParam(request, "before_id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Validate that either after_id or before_id is specified, but not both
			if afterID == 0 && beforeID == 0 {
				return mcp.NewToolResultError("either after_id or before_id must be specified"), nil
			}
			if afterID != 0 && beforeID != 0 {
				return mcp.NewToolResultError("only one of after_id or before_id should be specified, not both"), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to reprioritize sub-issue: %s", string(body))), nil
			}

			r, err := json.Marshal(subIssue)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// SearchIssues creates a tool to search for issues.
func SearchIssues(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("search_issues",
			mcp.WithDescription(t("TOOL_SEARCH_ISSUES_DESCRIPTION", "Search for issues in GitHub repositories using issues search syntax already scoped to is:issue")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_SEARCH_ISSUES_USER_TITLE", "Search issues"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("Search query using GitHub issues search syntax"),
			),
			mcp.WithString("owner",
				mcp.Description("Optional repository owner. If provided with repo, only issues for this repository are listed."),
			),
			mcp.WithString("repo",
				mcp.Description("Optional repository name. If provided with owner, only issues for this repository are listed."),
			),
			mcp.WithString("sort",
				mcp.Description("Sort field by number of matches of categories, defaults to best match"),
				mcp.Enum(
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
				),
			),
			mcp.WithString("order",
				mcp.Description("Sort order"),
				mcp.Enum("asc", "desc"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return searchHandler(ctx, getClient, request, "issue", "failed to search issues")
		}
}

// CreateIssue creates a tool to create a new issue in a GitHub repository.
func CreateIssue(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("create_issue",
			mcp.WithDescription(t("TOOL_CREATE_ISSUE_DESCRIPTION", "Create a new issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_CREATE_ISSUE_USER_TITLE", "Open new issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Issue title"),
			),
			mcp.WithString("body",
				mcp.Description("Issue body content"),
			),
			mcp.WithArray("assignees",
				mcp.Description("Usernames to assign to this issue"),
				mcp.Items(
					map[string]any{
						"type": "string",
					},
				),
			),
			mcp.WithArray("labels",
				mcp.Description("Labels to apply to this issue"),
				mcp.Items(
					map[string]any{
						"type": "string",
					},
				),
			),
			mcp.WithNumber("milestone",
				mcp.Description("Milestone number"),
			),
			mcp.WithString("type",
				mcp.Description("Type of this issue"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			title, err := RequiredParam[string](request, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Optional parameters
			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get assignees
			assignees, err := OptionalStringArrayParam(request, "assignees")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get labels
			labels, err := OptionalStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Get optional milestone
			milestone, err := OptionalIntParam(request, "milestone")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			var milestoneNum *int
			if milestone != 0 {
				milestoneNum = &milestone
			}

			// Get optional type
			issueType, err := OptionalParam[string](request, "type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Create the issue request
			issueRequest := &github.IssueRequest{
				Title:     github.Ptr(title),
				Body:      github.Ptr(body),
				Assignees: &assignees,
				Labels:    &labels,
				Milestone: milestoneNum,
			}

			if issueType != "" {
				issueRequest.Type = github.Ptr(issueType)
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}
			issue, resp, err := client.Issues.Create(ctx, owner, repo, issueRequest)
			if err != nil {
				return nil, fmt.Errorf("failed to create issue: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to create issue: %s", string(body))), nil
			}

			// Return minimal response with just essential information
			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", issue.GetID()),
				URL: issue.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}

// ListIssues creates a tool to list and filter repository issues
func ListIssues(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("list_issues",
			mcp.WithDescription(t("TOOL_LIST_ISSUES_DESCRIPTION", "List issues in a GitHub repository. For pagination, use the 'endCursor' from the previous response's 'pageInfo' in the 'after' parameter.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_LIST_ISSUES_USER_TITLE", "List issues"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("state",
				mcp.Description("Filter by state, by default both open and closed issues are returned when not provided"),
				mcp.Enum("OPEN", "CLOSED"),
			),
			mcp.WithArray("labels",
				mcp.Description("Filter by labels"),
				mcp.Items(
					map[string]interface{}{
						"type": "string",
					},
				),
			),
			mcp.WithString("orderBy",
				mcp.Description("Order issues by field. If provided, the 'direction' also needs to be provided."),
				mcp.Enum("CREATED_AT", "UPDATED_AT", "COMMENTS"),
			),
			mcp.WithString("direction",
				mcp.Description("Order direction. If provided, the 'orderBy' also needs to be provided."),
				mcp.Enum("ASC", "DESC"),
			),
			mcp.WithString("since",
				mcp.Description("Filter by date (ISO 8601 timestamp)"),
			),
			WithCursorPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Set optional parameters if provided
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// If the state has a value, cast into an array of strings
			var states []githubv4.IssueState
			if state != "" {
				states = append(states, githubv4.IssueState(state))
			} else {
				states = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
			}

			// Get labels
			labels, err := OptionalStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			orderBy, err := OptionalParam[string](request, "orderBy")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			direction, err := OptionalParam[string](request, "direction")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// These variables are required for the GraphQL query to be set by default
			// If orderBy is empty, default to CREATED_AT
			if orderBy == "" {
				orderBy = "CREATED_AT"
			}
			// If direction is empty, default to DESC
			if direction == "" {
				direction = "DESC"
			}

			since, err := OptionalParam[string](request, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// There are two optional parameters: since and labels.
			var sinceTime time.Time
			var hasSince bool
			if since != "" {
				sinceTime, err = parseISOTimestamp(since)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("failed to list issues: %s", err.Error())), nil
				}
				hasSince = true
			}
			hasLabels := len(labels) > 0

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(request)
			if err != nil {
				return nil, err
			}

			// Check if someone tried to use page-based pagination instead of cursor-based
			if _, pageProvided := request.GetArguments()["page"]; pageProvided {
				return mcp.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil
			}

			// Check if pagination parameters were explicitly provided
			_, perPageProvided := request.GetArguments()["perPage"]
			paginationExplicit := perPageProvided

			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, err
			}

			// Use default of 30 if pagination was not explicitly provided
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil
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
				return mcp.NewToolResultError(err.Error()), nil
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
				return nil, fmt.Errorf("failed to marshal issues: %w", err)
			}
			return mcp.NewToolResultText(string(out)), nil
		}
}

// UpdateIssue creates a tool to update an existing issue in a GitHub repository.
func UpdateIssue(getClient GetClientFn, getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("update_issue",
			mcp.WithDescription(t("TOOL_UPDATE_ISSUE_DESCRIPTION", "Update an existing issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_UPDATE_ISSUE_USER_TITLE", "Edit issue"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number to update"),
			),
			mcp.WithString("title",
				mcp.Description("New title"),
			),
			mcp.WithString("body",
				mcp.Description("New description"),
			),
			mcp.WithArray("labels",
				mcp.Description("New labels"),
				mcp.Items(
					map[string]interface{}{
						"type": "string",
					},
				),
			),
			mcp.WithArray("assignees",
				mcp.Description("New assignees"),
				mcp.Items(
					map[string]interface{}{
						"type": "string",
					},
				),
			),
			mcp.WithNumber("milestone",
				mcp.Description("New milestone number"),
			),
			mcp.WithString("type",
				mcp.Description("New issue type"),
			),
			mcp.WithString("state",
				mcp.Description("New state"),
				mcp.Enum("open", "closed"),
			),
			mcp.WithString("state_reason",
				mcp.Description("Reason for the state change. Ignored unless state is changed."),
				mcp.Enum("completed", "not_planned", "duplicate"),
			),
			mcp.WithNumber("duplicate_of",
				mcp.Description("Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'."),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			// Create the issue request with only provided fields
			issueRequest := &github.IssueRequest{}

			// Set optional parameters if provided
			title, err := OptionalParam[string](request, "title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if title != "" {
				issueRequest.Title = github.Ptr(title)
			}

			body, err := OptionalParam[string](request, "body")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if body != "" {
				issueRequest.Body = github.Ptr(body)
			}

			// Get labels
			labels, err := OptionalStringArrayParam(request, "labels")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(labels) > 0 {
				issueRequest.Labels = &labels
			}

			// Get assignees
			assignees, err := OptionalStringArrayParam(request, "assignees")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if len(assignees) > 0 {
				issueRequest.Assignees = &assignees
			}

			milestone, err := OptionalIntParam(request, "milestone")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if milestone != 0 {
				milestoneNum := milestone
				issueRequest.Milestone = &milestoneNum
			}

			// Get issue type
			issueType, err := OptionalParam[string](request, "type")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if issueType != "" {
				issueRequest.Type = github.Ptr(issueType)
			}

			// Handle state, state_reason and duplicateOf parameters
			state, err := OptionalParam[string](request, "state")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			stateReason, err := OptionalParam[string](request, "state_reason")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			duplicateOf, err := OptionalIntParam(request, "duplicate_of")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if duplicateOf != 0 && stateReason != "duplicate" {
				return mcp.NewToolResultError("duplicate_of can only be used when state_reason is 'duplicate'"), nil
			}

			// Use REST API for non-state updates
			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to update issue: %s", string(body))), nil
			}

			// Use GraphQL API for state updates
			if state != "" {
				gqlClient, err := getGQLClient(ctx)
				if err != nil {
					return nil, fmt.Errorf("failed to get GraphQL client: %w", err)
				}

				// Mandate specifying duplicateOf when trying to close as duplicate
				if state == "closed" && stateReason == "duplicate" && duplicateOf == 0 {
					return mcp.NewToolResultError("duplicate_of must be provided when state_reason is 'duplicate'"), nil
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

			return mcp.NewToolResultText(string(r)), nil
		}
}

// GetIssueComments creates a tool to get comments for a GitHub issue.
func GetIssueComments(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_issue_comments",
			mcp.WithDescription(t("TOOL_GET_ISSUE_COMMENTS_DESCRIPTION", "Get comments for a specific issue in a GitHub repository.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_ISSUE_COMMENTS_USER_TITLE", "Get issue comments"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
			WithPagination(),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pagination, err := OptionalPaginationParams(request)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			opts := &github.IssueListCommentsOptions{
				ListOptions: github.ListOptions{
					Page:    pagination.Page,
					PerPage: pagination.PerPage,
				},
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get issue comments: %s", string(body))), nil
			}

			r, err := json.Marshal(comments)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
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

func AssignCopilotToIssue(getGQLClient GetGQLClientFn, t translations.TranslationHelperFunc) (mcp.Tool, server.ToolHandlerFunc) {
	description := mvpDescription{
		summary: "Assign Copilot to a specific issue in a GitHub repository.",
		outcomes: []string{
			"a Pull Request created with source code changes to resolve the issue",
		},
		referenceLinks: []string{
			"https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot",
		},
	}

	return mcp.NewTool("assign_copilot_to_issue",
			mcp.WithDescription(t("TOOL_ASSIGN_COPILOT_TO_ISSUE_DESCRIPTION", description.String())),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:          t("TOOL_ASSIGN_COPILOT_TO_ISSUE_USER_TITLE", "Assign Copilot to issue"),
				ReadOnlyHint:   ToBoolPtr(false),
				IdempotentHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issueNumber",
				mcp.Required(),
				mcp.Description("Issue number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var params struct {
				Owner       string
				Repo        string
				IssueNumber int32
			}
			if err := mapstructure.Decode(request.Params.Arguments, &params); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getGQLClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
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
					return nil, err
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
				return mcp.NewToolResultError("copilot isn't available as an assignee for this issue. Please inform the user to visit https://docs.github.com/en/copilot/using-github-copilot/using-copilot-coding-agent-to-work-on-tasks/about-assigning-tasks-to-copilot for more information."), nil
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
				return mcp.NewToolResultError(fmt.Sprintf("failed to get issue ID: %v", err)), nil
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
				return nil, fmt.Errorf("failed to replace actors for assignable: %w", err)
			}

			return mcp.NewToolResultText("successfully assigned copilot to issue"), nil
		}
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

func AssignCodingAgentPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("AssignCodingAgent",
			mcp.WithPromptDescription(t("PROMPT_ASSIGN_CODING_AGENT_DESCRIPTION", "Assign GitHub Coding Agent to multiple tasks in a GitHub repository.")),
			mcp.WithArgument("repo", mcp.ArgumentDescription("The repository to assign tasks in (owner/repo)."), mcp.RequiredArgument()),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			repo := request.Params.Arguments["repo"]

			messages := []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent("You are a personal assistant for GitHub the Copilot GitHub Coding Agent. Your task is to help the user assign tasks to the Coding Agent based on their open GitHub issues. You can use `assign_copilot_to_issue` tool to assign the Coding Agent to issues that are suitable for autonomous work, and `search_issues` tool to find issues that match the user's criteria. You can also use `list_issues` to get a list of issues in the repository."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(fmt.Sprintf("Please go and get a list of the most recent 10 issues from the %s GitHub repository", repo)),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf("Sure! I will get a list of the 10 most recent issues for the repo %s.", repo)),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("For each issue, please check if it is a clearly defined coding task with acceptance criteria and a low to medium complexity to identify issues that are suitable for an AI Coding Agent to work on. Then assign each of the identified issues to Copilot."),
				},
				{
					Role:    "assistant",
					Content: mcp.NewTextContent("Certainly! Let me carefully check which ones are clearly scoped issues that are good to assign to the coding agent, and I will summarize and assign them now."),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent("Great, if you are unsure if an issue is good to assign, ask me first, rather than assigning copilot. If you are certain the issue is clear and suitable you can assign it to Copilot without asking."),
				},
			}
			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
