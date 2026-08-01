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

// IssueDependencyRead 创建一个工具以 读取 一个议题's blocked-by 和blocking
// relationships. It is 一个separate, feature-flagged 工具 (rather than 一个method on
// 默认议题_读取) so whole dependency 能力 可以 gated as a
// unit without enlarging 默认议题 工具 surface.
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

// GetIssueBlockedBy 列出s 议题 that block given 议题.
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

// GetIssueBlocking 列出s 议题 that given 议题 blocks.
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

// dependencyReadResult projects 一个列出 of related 议题 in到minimal
// dependency shape 和attaches 页-based pagination info.
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

// 议题ToDependencyRef converts 一个REST 议题 in到compact reference 由以下内容使用：
// dependency 工具, deriving "owner/repo" name 来自议题's
// 仓库 URL. state is upper-cased so it matches GraphQL-sourced
// state (e.g. "OPEN"/"CLOSED") that MinimalIssueRef carries 用于other 议题
// 工具 such as 获取_parent, keeping field consistent across 工具.
func issueToDependencyRef(issue *github.Issue) MinimalIssueRef {
	if issue == nil {
		return MinimalIssueRef{}
	}
	ref := MinimalIssueRef{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		State:  strings.ToUpper(issue.GetState()),
		URL:    issue.GetHTMLURL(),
	}
	if owner, repo, ok := parseRepositoryURL(issue.GetRepositoryURL()); ok {
		ref.Repository = owner + "/" + repo
	}
	return ref
}

// IssueDependencyWrite 创建一个工具以 add 或remove 一个议题 dependency
// (blocked-by / blocking) relationship. REST dependency endpoints are always
// expressed as "the blocked 议题 is blocked_by blocking 议题", so both
// directions are served 由相同 endpoint pair 使用two 议题 swapped.
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

			// Map subject/related pair on到blocked/blocking roles REST
			// endpoints expect. F或type 'blocked_by' subject is blocked
			// 议题; f或'blocking' subject blocks related 议题, so the
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

// 议题Coordinate identifies 一个议题 by 仓库 和number.
type issueCoordinate struct {
	owner  string
	repo   string
	number int
}

// 写入IssueDependency resolves blocking 议题 to its global 数据base ID and
// 然后adds 或removes blocked-by relationship 在blocked 议题.
func writeIssueDependency(ctx context.Context, client *github.Client, method string, blocked, blocking issueCoordinate) (*mcp.CallToolResult, error) {
	// REST API identifies blocking 议题 by its global 数据base ID
	// (不its number), so resolve number to 一个ID 第一个.
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

// dependencyWriteResult builds minimal description 的affected 议题.
// blocked 议题 comes 来自mutation 响应 以及blocking 议题 from
// earlier resolve; 每个falls back to its known coordinate 当API
// 响应 omits 仓库 URL.
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
