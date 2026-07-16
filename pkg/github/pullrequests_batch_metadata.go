package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/github-mcp-server/pkg/scopes"
)

const maxPullRequestMetadataBatchSize = 25

type batchPullRequestMetadataError struct {
	PullNumber int    `json:"pull_number"`
	Message    string `json:"message"`
}

type batchPullRequestMetadataResponse struct {
	PullRequests []MinimalPullRequest            `json:"pull_requests"`
	Errors       []batchPullRequestMetadataError `json:"errors,omitempty"`
}

func GetPullRequestMetadataBatch(t translations.TranslationHelperFunc) inventory.ServerTool {
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
			"pullNumbers": {
				Type:        "array",
				Description: fmt.Sprintf("Explicit pull request numbers to hydrate. Accepts up to %d items.", maxPullRequestMetadataBatchSize),
				MinItems:    jsonschema.Ptr(1),
				MaxItems:    jsonschema.Ptr(maxPullRequestMetadataBatchSize),
				Items: &jsonschema.Schema{
					Type:    "integer",
					Minimum: jsonschema.Ptr(1.0),
				},
			},
		},
		Required: []string{"owner", "repo", "pullNumbers"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "get_pull_request_metadata_batch",
			Description: t("TOOL_GET_PULL_REQUEST_METADATA_BATCH_DESCRIPTION", "Get metadata for an explicit list of pull requests in a GitHub repository. Returns partial success with per-PR errors when some requested pull requests cannot be hydrated."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PULL_REQUEST_METADATA_BATCH_USER_TITLE", "Get batch pull request metadata"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			pullNumbers, err := requiredPullNumberBatchParam(args, "pullNumbers", maxPullRequestMetadataBatchSize)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				return attachRepoVisibilityIFCLabel(ctx, deps, client, owner, repo, r, ifc.LabelListIssues)
			}

			result := batchPullRequestMetadataResponse{
				PullRequests: make([]MinimalPullRequest, 0, len(pullNumbers)),
				Errors:       make([]batchPullRequestMetadataError, 0),
			}

			for _, pullNumber := range pullNumbers {
				pr, err := fetchMinimalPullRequest(ctx, client, deps, owner, repo, pullNumber)
				if err != nil {
					result.Errors = append(result.Errors, batchPullRequestMetadataError{
						PullNumber: pullNumber,
						Message:    err.Error(),
					})
					continue
				}

				result.PullRequests = append(result.PullRequests, pr)
			}

			return attachIFC(MarshalledTextResult(result)), nil, nil
		},
	)
}

func requiredPullNumberBatchParam(args map[string]any, key string, maxItems int) ([]int, error) {
	raw, ok := args[key]
	if !ok {
		return nil, fmt.Errorf("missing required parameter: %s", key)
	}

	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("parameter %s could not be coerced to []int, is %T", key, raw)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("parameter %s must contain at least one pull request number", key)
	}
	if len(values) > maxItems {
		return nil, fmt.Errorf("parameter %s exceeds the maximum batch size of %d", key, maxItems)
	}

	pullNumbers := make([]int, 0, len(values))
	seen := make(map[int]struct{}, len(values))
	for i, value := range values {
		number, ok := value.(float64)
		if !ok {
			return nil, fmt.Errorf("parameter %s element %d is not a number, is %T", key, i, value)
		}
		if number < 1 || number != float64(int(number)) {
			return nil, fmt.Errorf("parameter %s element %d must be a positive integer", key, i)
		}
		intNumber := int(number)
		if _, ok := seen[intNumber]; ok {
			continue
		}
		seen[intNumber] = struct{}{}
		pullNumbers = append(pullNumbers, intNumber)
	}

	return pullNumbers, nil
}

func fetchMinimalPullRequest(ctx context.Context, client *github.Client, deps ToolDependencies, owner, repo string, pullNumber int) (MinimalPullRequest, error) {
	minimalPR, toolErr, err := getMinimalPullRequest(ctx, client, deps, owner, repo, pullNumber)
	if toolErr != nil {
		return MinimalPullRequest{}, fmt.Errorf("%s", getErrorResultText(toolErr))
	}
	if err != nil {
		return MinimalPullRequest{}, err
	}
	return minimalPR, nil
}

func getErrorResultText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return "failed to get pull request"
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return "failed to get pull request"
	}
	return text.Text
}
