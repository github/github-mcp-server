package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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
	WithPagination(schema)

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

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			opts := &github.ListOptions{Page: pagination.Page, PerPage: pagination.PerPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			switch method {
			case "get_blocked_by":
				result, err := GetIssueBlockedBy(ctx, client, owner, repo, issueNumber, opts)
				return result, nil, err
			case "get_blocking":
				result, err := GetIssueBlocking(ctx, client, owner, repo, issueNumber, opts)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
	st.FeatureFlagEnable = FeatureFlagIssueDependencies
	return st
}

// GetIssueBlockedBy lists the issues that block the given issue.
func GetIssueBlockedBy(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, error) {
	issues, resp, err := client.Issues.ListBlockedBy(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list blocked-by issues", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list blocked-by issues", resp, body), nil
	}
	return dependencyReadResult(issues, resp), nil
}

// GetIssueBlocking lists the issues that the given issue blocks.
func GetIssueBlocking(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, error) {
	issues, resp, err := client.Issues.ListBlocking(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list blocking issues", resp, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list blocking issues", resp, body), nil
	}
	return dependencyReadResult(issues, resp), nil
}

// dependencyReadResult projects a list of related issues into the minimal
// dependency shape and attaches page-based pagination info.
func dependencyReadResult(issues []*github.Issue, resp *github.Response) *mcp.CallToolResult {
	refs := make([]MinimalIssueRef, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		refs = append(refs, issueToDependencyRef(issue))
	}
	return MarshalledTextResult(map[string]any{
		"issues": refs,
		"pageInfo": map[string]any{
			"hasNextPage": resp.NextPage != 0,
			"nextPage":    resp.NextPage,
		},
	})
}

// issueToDependencyRef converts a REST issue into the compact reference used by
// the dependency tools, deriving the "owner/repo" name from the issue's
// repository URL.
func issueToDependencyRef(issue *github.Issue) MinimalIssueRef {
	ref := MinimalIssueRef{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		State:  issue.GetState(),
		URL:    issue.GetHTMLURL(),
	}
	if owner, repo, ok := parseRepositoryURL(issue.GetRepositoryURL()); ok {
		ref.Repository = owner + "/" + repo
	}
	return ref
}

// IssueDependencyWrite creates a tool to add or remove an issue dependency
// (blocked-by / blocking) relationship. The REST dependency endpoints are always
// expressed as "the blocked issue is blocked_by the blocking issue", so both
// directions are served by the same endpoint pair with the two issues swapped.
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

			if owner == relatedOwner && repo == relatedRepo && issueNumber == relatedIssueNumber {
				return utils.NewToolResultError("an issue cannot block or depend on itself"), nil, nil
			}

			// Map the subject/related pair onto the blocked/blocking roles the REST
			// endpoints expect. For type 'blocked_by' the subject is the blocked
			// issue; for 'blocking' the subject blocks the related issue, so the
			// roles swap.
			blocked := issueCoordinate{owner: owner, repo: repo, number: issueNumber}
			blocking := issueCoordinate{owner: relatedOwner, repo: relatedRepo, number: relatedIssueNumber}
			if relationshipType == "blocking" {
				blocked, blocking = blocking, blocked
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			result, err := writeIssueDependency(ctx, client, method, blocked, blocking)
			return result, nil, err
		})
	st.FeatureFlagEnable = FeatureFlagIssueDependencies
	return st
}

// issueCoordinate identifies an issue by repository and number.
type issueCoordinate struct {
	owner  string
	repo   string
	number int
}

// writeIssueDependency resolves the blocking issue to its global database ID and
// then adds or removes the blocked-by relationship on the blocked issue.
func writeIssueDependency(ctx context.Context, client *github.Client, method string, blocked, blocking issueCoordinate) (*mcp.CallToolResult, error) {
	// The REST API identifies the blocking issue by its global database ID
	// (not its number), so resolve the number to an ID first.
	blockingIssue, resp, err := client.Issues.Get(ctx, blocking.owner, blocking.repo, blocking.number)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to resolve blocking issue", resp, err), nil
	}
	_ = resp.Body.Close()
	blockingID := blockingIssue.GetID()

	switch method {
	case "add":
		blockedIssue, opResp, err := client.Issues.AddBlockedBy(ctx, blocked.owner, blocked.repo, int64(blocked.number), github.IssueDependencyRequest{IssueID: blockingID})
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add issue dependency", opResp, err), nil
		}
		defer func() { _ = opResp.Body.Close() }()
		if opResp.StatusCode != http.StatusCreated {
			body, readErr := io.ReadAll(opResp.Body)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read response body: %w", readErr)
			}
			return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to add issue dependency", opResp, body), nil
		}
		return dependencyWriteResult("dependency added", blockedIssue, blockingIssue, blocked, blocking), nil
	case "remove":
		blockedIssue, opResp, err := client.Issues.RemoveBlockedBy(ctx, blocked.owner, blocked.repo, int64(blocked.number), blockingID)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to remove issue dependency", opResp, err), nil
		}
		defer func() { _ = opResp.Body.Close() }()
		if opResp.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(opResp.Body)
			if readErr != nil {
				return nil, fmt.Errorf("failed to read response body: %w", readErr)
			}
			return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to remove issue dependency", opResp, body), nil
		}
		return dependencyWriteResult("dependency removed", blockedIssue, blockingIssue, blocked, blocking), nil
	default:
		return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil
	}
}

// dependencyWriteResult builds the minimal description of the affected issues.
// The blocked issue comes from the mutation response and the blocking issue from
// the earlier resolve; each falls back to its known coordinate when the API
// response omits the repository URL.
func dependencyWriteResult(message string, blockedIssue, blockingIssue *github.Issue, blocked, blocking issueCoordinate) *mcp.CallToolResult {
	blockedRef := issueToDependencyRef(blockedIssue)
	if blockedRef.Repository == "" {
		blockedRef.Repository = blocked.owner + "/" + blocked.repo
	}
	blockingRef := issueToDependencyRef(blockingIssue)
	if blockingRef.Repository == "" {
		blockingRef.Repository = blocking.owner + "/" + blocking.repo
	}
	return MarshalledTextResult(map[string]any{
		"message":        message,
		"blocked_issue":  blockedRef,
		"blocking_issue": blockingRef,
	})
}
