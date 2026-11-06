package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
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

	st := NewToolWithSchemaOptions[IssueDependencyReadInput, *IssueDependencyReadOutput](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_dependency_read",
			Description: t("TOOL_ISSUE_DEPENDENCY_READ_DESCRIPTION", "Read an issue's dependency relationships in a GitHub repository: the issues that block it (blocked_by) or the issues it blocks (blocking)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_DEPENDENCY_READ_USER_TITLE", "Read issue dependencies"),
				ReadOnlyHint: true,
			},
			InputSchema:  schema,
			OutputSchema: issueDependencyReadOutputSchema(),
		},
		scopes.PublicRead(scopes.Repo),
		inventory.TypedSchemaOptions{ValidationInputSchema: issuePaginationValidationSchema(schema), PreserveHandlerContent: true},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input IssueDependencyReadInput) (*mcp.CallToolResult, *IssueDependencyReadOutput, error) {
			method, owner, repo, issueNumber := input.Method, input.Owner, input.Repo, input.IssueNumber
			if method == "" {
				return utils.NewToolResultError("missing required parameter: method"), nil, nil
			}
			if err := validateIssueCoordinate(owner, repo, issueNumber); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			page, perPage := input.Page, input.PerPage
			if page == 0 {
				page = 1
			}
			if perPage == 0 {
				perPage = 30
			}
			opts := &github.ListOptions{Page: page, PerPage: perPage}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			switch method {
			case "get_blocked_by":
				return getIssueBlockedByOutput(ctx, client, owner, repo, issueNumber, opts)
			case "get_blocking":
				return getIssueBlockingOutput(ctx, client, owner, repo, issueNumber, opts)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		}, normalizeIssueStrings([]string{"method", "owner", "repo"}, nil),
		normalizeIssueIntegers([]string{"issue_number"}, []string{"page", "perPage"}),
		normalizeTypedReadArguments(nil, false))
	st.FeatureRule = featureEnabledRule(FeatureFlagIssueDependencies)
	return st
}

// GetIssueBlockedBy lists the issues that block the given issue.
func GetIssueBlockedBy(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, error) {
	result, _, err := getIssueBlockedByOutput(ctx, client, owner, repo, issueNumber, opts)
	return result, err
}

func getIssueBlockedByOutput(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, *IssueDependencyReadOutput, error) {
	issues, resp, err := client.Issues.ListBlockedBy(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list blocked-by issues", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list blocked-by issues", resp, body), nil, nil
	}
	output := dependencyReadOutput(issues, resp)
	return dependencyReadTextResult(issues, resp), output, nil
}

// GetIssueBlocking lists the issues that the given issue blocks.
func GetIssueBlocking(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, error) {
	result, _, err := getIssueBlockingOutput(ctx, client, owner, repo, issueNumber, opts)
	return result, err
}

func getIssueBlockingOutput(ctx context.Context, client *github.Client, owner, repo string, issueNumber int, opts *github.ListOptions) (*mcp.CallToolResult, *IssueDependencyReadOutput, error) {
	issues, resp, err := client.Issues.ListBlocking(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list blocking issues", resp, err), nil, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list blocking issues", resp, body), nil, nil
	}
	output := dependencyReadOutput(issues, resp)
	return dependencyReadTextResult(issues, resp), output, nil
}

// dependencyReadOutput projects a list of related issues into the minimal
// dependency shape and attaches page-based pagination info.
func dependencyReadOutput(issues []*github.Issue, resp *github.Response) *IssueDependencyReadOutput {
	refs := make([]MinimalIssueDependencyRef, 0, len(issues))
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		refs = append(refs, dependencyRefOutput(issueToDependencyRef(issue)))
	}
	return &IssueDependencyReadOutput{
		Issues: refs,
		PageInfo: IssueDependencyPageInfo{
			HasNextPage: resp.NextPage != 0,
			NextPage:    resp.NextPage,
		},
	}
}

func dependencyReadTextResult(issues []*github.Issue, resp *github.Response) *mcp.CallToolResult {
	refs := make([]MinimalIssueRef, 0, len(issues))
	for _, issue := range issues {
		if issue != nil {
			refs = append(refs, issueToDependencyRef(issue))
		}
	}
	return MarshalledTextResult(IssueDependencyReadLegacyOutput{
		Issues: refs,
		PageInfo: IssueDependencyPageInfo{
			HasNextPage: resp.NextPage != 0,
			NextPage:    resp.NextPage,
		},
	})
}

// issueToDependencyRef converts a REST issue into the compact reference used by
// the dependency tools, deriving the "owner/repo" name from the issue's
// repository URL. The state is upper-cased so it matches the GraphQL-sourced
// state (e.g. "OPEN"/"CLOSED") that MinimalIssueRef carries for the other issue
// tools such as get_parent, keeping the field consistent across tools.
func issueToDependencyRef(issue *github.Issue) MinimalIssueRef {
	if issue == nil {
		return MinimalIssueRef{}
	}
	var repository string
	if owner, repo, ok := parseRepositoryURL(issue.GetRepositoryURL()); ok {
		repository = owner + "/" + repo
	}
	return newMinimalIssueRef(
		issue.GetNumber(),
		issue.GetTitle(),
		strings.ToUpper(issue.GetState()),
		issue.GetHTMLURL(),
		repository,
	)
}

// IssueDependencyWrite creates a tool to add or remove an issue dependency
// (blocked-by / blocking) relationship. The REST dependency endpoints are always
// expressed as "the blocked issue is blocked_by the blocking issue", so both
// directions are served by the same endpoint pair with the two issues swapped.
func IssueDependencyWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewToolWithSchemaOptions[IssueDependencyWriteInput, *IssueDependencyWriteOutput](
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
			OutputSchema: issueDependencyWriteOutputSchema(),
		},
		scopes.RequireAll(scopes.Repo),
		inventory.TypedSchemaOptions{PreserveHandlerContent: true},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input IssueDependencyWriteInput) (*mcp.CallToolResult, *IssueDependencyWriteOutput, error) {
			method, relationshipType := input.Method, input.Type
			if method == "" {
				return utils.NewToolResultError("missing required parameter: method"), nil, nil
			}
			if relationshipType == "" {
				return utils.NewToolResultError("missing required parameter: type"), nil, nil
			}
			owner, repo, issueNumber := input.Owner, input.Repo, input.IssueNumber
			if err := validateIssueCoordinate(owner, repo, issueNumber); err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			relatedIssueNumber := input.RelatedIssueNumber
			if relatedIssueNumber == 0 {
				return utils.NewToolResultError("missing required parameter: related_issue_number"), nil, nil
			}
			relatedOwner, relatedRepo := input.RelatedOwner, input.RelatedRepo
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

			return writeIssueDependency(ctx, client, method, blocked, blocking)
		}, normalizeIssueStrings([]string{"method", "type", "owner", "repo"}, []string{"related_owner", "related_repo"}),
		normalizeIssueIntegers([]string{"issue_number", "related_issue_number"}, nil), normalizeIssueDependencyWriteArguments)
	st.FeatureRule = featureEnabledRule(FeatureFlagIssueDependencies)
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
func writeIssueDependency(ctx context.Context, client *github.Client, method string, blocked, blocking issueCoordinate) (*mcp.CallToolResult, *IssueDependencyWriteOutput, error) {
	// The REST API identifies the blocking issue by its global database ID
	// (not its number), so resolve the number to an ID first.
	blockingIssue, resp, err := client.Issues.Get(ctx, blocking.owner, blocking.repo, blocking.number)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to resolve blocking issue", resp, err), nil, nil
	}
	_ = resp.Body.Close()
	blockingID := blockingIssue.GetID()

	switch method {
	case "add":
		blockedIssue, opResp, err := client.Issues.AddBlockedBy(ctx, blocked.owner, blocked.repo, int64(blocked.number), github.IssueDependencyRequest{IssueID: blockingID})
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add issue dependency", opResp, err), nil, nil
		}
		defer func() { _ = opResp.Body.Close() }()
		if opResp.StatusCode != http.StatusCreated {
			body, readErr := io.ReadAll(opResp.Body)
			if readErr != nil {
				return nil, nil, fmt.Errorf("failed to read response body: %w", readErr)
			}
			return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to add issue dependency", opResp, body), nil, nil
		}
		output := dependencyWriteOutput("dependency added", blockedIssue, blockingIssue, blocked, blocking)
		return dependencyWriteTextResult("dependency added", blockedIssue, blockingIssue, blocked, blocking), output, nil
	case "remove":
		blockedIssue, opResp, err := client.Issues.RemoveBlockedBy(ctx, blocked.owner, blocked.repo, int64(blocked.number), blockingID)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to remove issue dependency", opResp, err), nil, nil
		}
		defer func() { _ = opResp.Body.Close() }()
		if opResp.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(opResp.Body)
			if readErr != nil {
				return nil, nil, fmt.Errorf("failed to read response body: %w", readErr)
			}
			return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to remove issue dependency", opResp, body), nil, nil
		}
		output := dependencyWriteOutput("dependency removed", blockedIssue, blockingIssue, blocked, blocking)
		return dependencyWriteTextResult("dependency removed", blockedIssue, blockingIssue, blocked, blocking), output, nil
	default:
		return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
	}
}

// dependencyWriteOutput builds the minimal description of the affected issues.
// The blocked issue comes from the mutation response and the blocking issue from
// the earlier resolve; each falls back to its known coordinate when the API
// response omits the repository URL.
func dependencyWriteOutput(message string, blockedIssue, blockingIssue *github.Issue, blocked, blocking issueCoordinate) *IssueDependencyWriteOutput {
	blockedRef := issueToDependencyRef(blockedIssue)
	if blockedRef.Repository == "" {
		blockedRef.Repository = blocked.owner + "/" + blocked.repo
	}
	blockingRef := issueToDependencyRef(blockingIssue)
	if blockingRef.Repository == "" {
		blockingRef.Repository = blocking.owner + "/" + blocking.repo
	}
	return &IssueDependencyWriteOutput{
		BlockedIssue: dependencyRefOutput(blockedRef), BlockingIssue: dependencyRefOutput(blockingRef), Message: message,
	}
}

func dependencyWriteTextResult(message string, blockedIssue, blockingIssue *github.Issue, blocked, blocking issueCoordinate) *mcp.CallToolResult {
	return MarshalledTextResult(IssueDependencyWriteLegacyOutput{
		BlockedIssue:  issueToLegacyDependencyRef(blockedIssue, blocked),
		BlockingIssue: issueToLegacyDependencyRef(blockingIssue, blocking),
		Message:       message,
	})
}

func issueToLegacyDependencyRef(issue *github.Issue, coordinate issueCoordinate) MinimalIssueRef {
	ref := issueToDependencyRef(issue)
	if ref.Repository == "" {
		ref.Repository = coordinate.owner + "/" + coordinate.repo
	}
	return ref
}
