package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const pullRequestStacksAPIVersion = "2026-03-10"

// PullRequestStackRepository identifies a repository referenced by a stack layer.
type PullRequestStackRepository struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// PullRequestStackRef identifies a branch and commit referenced by a stack.
type PullRequestStackRef struct {
	Ref  string                      `json:"ref"`
	SHA  string                      `json:"sha,omitempty"`
	Repo *PullRequestStackRepository `json:"repo,omitempty"`
}

// PullRequestStackPullRequest is the compact pull request representation returned
// by the stack tools.
type PullRequestStackPullRequest struct {
	ID       int64                `json:"id,omitempty"`
	NodeID   string               `json:"node_id,omitempty"`
	Number   int                  `json:"number"`
	URL      string               `json:"url,omitempty"`
	HTMLURL  string               `json:"html_url,omitempty"`
	State    string               `json:"state"`
	MergedAt *string              `json:"merged_at"`
	Draft    bool                 `json:"draft"`
	Head     PullRequestStackRef  `json:"head"`
	Base     *PullRequestStackRef `json:"base,omitempty"`
}

// PullRequestStack is a compact representation of a native GitHub pull request
// stack. PullRequests are ordered from the bottom of the stack to the top.
type PullRequestStack struct {
	ID           int64                         `json:"id"`
	Number       int                           `json:"number"`
	NodeID       string                        `json:"node_id"`
	URL          string                        `json:"url"`
	Base         PullRequestStackRef           `json:"base"`
	Open         bool                          `json:"open"`
	CreatedAt    string                        `json:"created_at"`
	PullRequests []PullRequestStackPullRequest `json:"pull_requests"`
}

type pullRequestStackInput struct {
	PullRequests []int `json:"pull_requests"`
}

type pullRequestStackListResult struct {
	Stacks   []PullRequestStack `json:"stacks"`
	PageInfo map[string]any     `json:"pageInfo"`
}

type pullRequestStackUnstackResult struct {
	Dissolved   bool              `json:"dissolved"`
	StackNumber int               `json:"stack_number"`
	Stack       *PullRequestStack `json:"stack,omitempty"`
}

// PullRequestStackRead creates a tool for reading native pull request stacks.
func PullRequestStackRead(t translations.TranslationHelperFunc, opts ...ToolOption) inventory.ServerTool {
	cfg := newToolConfig(opts)
	schema := WithPagination(&jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type:        "string",
				Description: "The read operation: `get` retrieves one stack by stackNumber; `list` lists repository stacks and can filter by pullNumber.",
				Enum:        []any{"get", "list"},
			},
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"stackNumber": {
				Type:        "number",
				Description: "Stack number. Required when method is `get`.",
				Minimum:     jsonschema.Ptr(1.0),
			},
			"pullNumber": {
				Type:        "number",
				Description: "Filter listed stacks to the stack containing this repository pull request number. Used only when method is `list`.",
				Minimum:     jsonschema.Ptr(1.0),
			},
		},
		Required: []string{"method", "owner", "repo"},
	})

	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "pull_request_stack_read",
			Description: t("TOOL_PULL_REQUEST_STACK_READ_DESCRIPTION", "Read native GitHub pull request stacks. Use `get` for a stack number or `list` to enumerate stacks and optionally resolve the stack containing a pull request."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_PULL_REQUEST_STACK_READ_USER_TITLE", "Read pull request stacks"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		scopes.PublicRead(scopes.Repo),
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
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var result *mcp.CallToolResult
			switch method {
			case "get":
				stackNumber, err := requiredPullRequestStackNumber(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				stack, resp, err := GetPullRequestStack(ctx, client, owner, repo, stackNumber)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request stack", resp, err), nil, nil
				}
				result = MarshalledTextResult(stack)
			case "list":
				pullNumber, err := OptionalIntParam(args, "pullNumber")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if _, provided := args["pullNumber"]; provided && pullNumber < 1 {
					return utils.NewToolResultError("parameter pullNumber must be greater than zero"), nil, nil
				}
				pagination, err := OptionalPaginationParams(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				stacks, resp, err := ListPullRequestStacks(ctx, client, owner, repo, pullNumber, pagination)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list pull request stacks", resp, err), nil, nil
				}
				result = MarshalledTextResult(pullRequestStackListResult{
					Stacks: stacks,
					PageInfo: map[string]any{
						"hasNextPage": resp.NextPage != 0,
						"nextPage":    resp.NextPage,
					},
				})
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}

			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelRepoUserContent)
			return result, nil, nil
		},
	)
	if cfg.hostType == utils.HostTypeGHES {
		st.Enabled = func(context.Context) (bool, error) { return false, nil }
	}
	return st
}

// PullRequestStackWrite creates a tool for creating, extending, or unstacking
// native pull request stacks.
func PullRequestStackWrite(t translations.TranslationHelperFunc, opts ...ToolOption) inventory.ServerTool {
	cfg := newToolConfig(opts)
	st := NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name: "pull_request_stack_write",
			Description: t("TOOL_PULL_REQUEST_STACK_WRITE_DESCRIPTION",
				"Create, extend, or unstack a native GitHub pull request stack. "+
					"`create` accepts 2-100 pullNumbers ordered bottom-to-top. "+
					"`add` accepts 1-100 pullNumbers to append above the current top. "+
					"`unstack` removes every removable unmerged pull request and may leave locked or queued pull requests in the stack. "+
					"All pull requests must belong to the target repository, use branches in that repository, and form a linear base/head chain. "+
					"These operations manage stack metadata only; they do not create pull requests, retarget bases, rebase commits, push branches, or merge."),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_PULL_REQUEST_STACK_WRITE_USER_TITLE", "Manage pull request stack"),
				ReadOnlyHint:    false,
				DestructiveHint: jsonschema.Ptr(true),
				OpenWorldHint:   jsonschema.Ptr(true),
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type:        "string",
						Description: "The write operation: `create`, `add`, or `unstack`.",
						Enum:        []any{"create", "add", "unstack"},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"stackNumber": {
						Type:        "number",
						Description: "Stack number. Required for `add` and `unstack`.",
						Minimum:     jsonschema.Ptr(1.0),
					},
					"pullNumbers": {
						Type:        "array",
						Description: "Repository pull request numbers in bottom-to-top order. Required for `create` and `add`.",
						Items: &jsonschema.Schema{
							Type:    "number",
							Minimum: jsonschema.Ptr(1.0),
						},
						MinItems: jsonschema.Ptr(1),
						MaxItems: jsonschema.Ptr(100),
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		publicRepositoryWriteScopeAccess(),
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
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var result *mcp.CallToolResult
			switch method {
			case "create":
				pullNumbers, err := parsePullRequestStackNumbers(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if len(pullNumbers) < 2 {
					return utils.NewToolResultError("method create requires at least two pullNumbers"), nil, nil
				}
				stack, resp, err := CreatePullRequestStack(ctx, client, owner, repo, pullNumbers)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to create pull request stack", resp, err), nil, nil
				}
				result = MarshalledTextResult(stack)
			case "add":
				stackNumber, err := requiredPullRequestStackNumber(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				pullNumbers, err := parsePullRequestStackNumbers(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				if len(pullNumbers) == 0 {
					return utils.NewToolResultError("method add requires at least one pullNumber"), nil, nil
				}
				stack, resp, err := AddPullRequestsToStack(ctx, client, owner, repo, stackNumber, pullNumbers)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add pull requests to stack", resp, err), nil, nil
				}
				result = MarshalledTextResult(stack)
			case "unstack":
				stackNumber, err := requiredPullRequestStackNumber(args)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				stack, resp, err := UnstackPullRequests(ctx, client, owner, repo, stackNumber)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to unstack pull requests", resp, err), nil, nil
				}
				result = MarshalledTextResult(pullRequestStackUnstackResult{
					Dissolved:   stack == nil,
					StackNumber: stackNumber,
					Stack:       stack,
				})
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}

			result = attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, result, ifc.LabelRepoUserContent)
			return result, nil, nil
		},
	)
	if cfg.hostType == utils.HostTypeGHES {
		st.Enabled = func(context.Context) (bool, error) { return false, nil }
	}
	return st
}

// GetPullRequestStack gets a native pull request stack by number.
func GetPullRequestStack(ctx context.Context, client *github.Client, owner, repo string, stackNumber int) (*PullRequestStack, *github.Response, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/stacks/%d", owner, repo, stackNumber)
	req, err := newPullRequestStackRequest(ctx, client, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, nil, err
	}

	var stack PullRequestStack
	resp, err := client.Do(req, &stack)
	return &stack, resp, err
}

// ListPullRequestStacks lists native pull request stacks, optionally filtering
// to the stack containing pullNumber.
func ListPullRequestStacks(ctx context.Context, client *github.Client, owner, repo string, pullNumber int, pagination PaginationParams) ([]PullRequestStack, *github.Response, error) {
	query := url.Values{
		"page":     {strconv.Itoa(pagination.Page)},
		"per_page": {strconv.Itoa(pagination.PerPage)},
	}
	if pullNumber > 0 {
		query.Set("pull_request", strconv.Itoa(pullNumber))
	}
	apiURL := fmt.Sprintf("repos/%s/%s/stacks?%s", owner, repo, query.Encode())
	req, err := newPullRequestStackRequest(ctx, client, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, nil, err
	}

	var stacks []PullRequestStack
	resp, err := client.Do(req, &stacks)
	return stacks, resp, err
}

// CreatePullRequestStack creates a native stack from pull request numbers
// ordered from bottom to top.
func CreatePullRequestStack(ctx context.Context, client *github.Client, owner, repo string, pullNumbers []int) (*PullRequestStack, *github.Response, error) {
	return mutatePullRequestStack(ctx, client, owner, repo, "", pullNumbers)
}

// AddPullRequestsToStack appends pull request numbers above the current stack top.
func AddPullRequestsToStack(ctx context.Context, client *github.Client, owner, repo string, stackNumber int, pullNumbers []int) (*PullRequestStack, *github.Response, error) {
	return mutatePullRequestStack(ctx, client, owner, repo, fmt.Sprintf("%d/add", stackNumber), pullNumbers)
}

// UnstackPullRequests removes every removable unmerged pull request from a
// native stack. A nil stack means the stack was dissolved.
func UnstackPullRequests(ctx context.Context, client *github.Client, owner, repo string, stackNumber int) (*PullRequestStack, *github.Response, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/stacks/%d/unstack", owner, repo, stackNumber)
	req, err := newPullRequestStackRequest(ctx, client, http.MethodPost, apiURL, nil)
	if err != nil {
		return nil, nil, err
	}

	var stack PullRequestStack
	resp, err := client.Do(req, &stack)
	if err != nil {
		return nil, resp, err
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, resp, nil
	}
	return &stack, resp, nil
}

func mutatePullRequestStack(ctx context.Context, client *github.Client, owner, repo, suffix string, pullNumbers []int) (*PullRequestStack, *github.Response, error) {
	apiURL := fmt.Sprintf("repos/%s/%s/stacks", owner, repo)
	if suffix != "" {
		apiURL += "/" + suffix
	}
	req, err := newPullRequestStackRequest(ctx, client, http.MethodPost, apiURL, pullRequestStackInput{PullRequests: pullNumbers})
	if err != nil {
		return nil, nil, err
	}

	var stack PullRequestStack
	resp, err := client.Do(req, &stack)
	return &stack, resp, err
}

func newPullRequestStackRequest(ctx context.Context, client *github.Client, method, apiURL string, body any) (*http.Request, error) {
	return client.NewRequest(ctx, method, apiURL, body, github.WithVersion(pullRequestStacksAPIVersion))
}

func requiredPullRequestStackNumber(args map[string]any) (int, error) {
	stackNumber, err := RequiredInt(args, "stackNumber")
	if err != nil {
		return 0, err
	}
	if stackNumber < 1 {
		return 0, fmt.Errorf("parameter stackNumber must be greater than zero")
	}
	return stackNumber, nil
}

func parsePullRequestStackNumbers(args map[string]any) ([]int, error) {
	value, ok := args["pullNumbers"]
	if !ok {
		return nil, nil
	}

	var pullNumbers []int
	switch values := value.(type) {
	case []int:
		pullNumbers = append([]int(nil), values...)
	case []any:
		pullNumbers = make([]int, len(values))
		for i, value := range values {
			if number, ok := value.(int); ok {
				pullNumbers[i] = number
				continue
			}
			number, err := toInt(value)
			if err != nil {
				return nil, fmt.Errorf("pullNumbers[%d] is invalid: %w", i, err)
			}
			pullNumbers[i] = number
		}
	default:
		return nil, fmt.Errorf("parameter pullNumbers is not an array, is %T", value)
	}

	if len(pullNumbers) > 100 {
		return nil, fmt.Errorf("pullNumbers must contain at most 100 items")
	}
	seen := make(map[int]struct{}, len(pullNumbers))
	for i, number := range pullNumbers {
		if number < 1 {
			return nil, fmt.Errorf("pullNumbers[%d] must be greater than zero", i)
		}
		if _, exists := seen[number]; exists {
			return nil, fmt.Errorf("pullNumbers[%d] duplicates pull request %d", i, number)
		}
		seen[number] = struct{}{}
	}
	return pullNumbers, nil
}
