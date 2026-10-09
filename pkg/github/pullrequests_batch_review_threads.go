package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/github/github-mcp-server/pkg/scopes"
)

const maxPullRequestReviewThreadsBatchSize = 20

type batchPullRequestReviewThreadsItem struct {
	PullNumber    int                          `json:"pull_number"`
	ReviewThreads MinimalReviewThreadsResponse `json:"review_threads"`
}

type batchPullRequestReviewThreadsError struct {
	PullNumber int    `json:"pull_number"`
	Message    string `json:"message"`
}

type batchPullRequestReviewThreadsResponse struct {
	Results []batchPullRequestReviewThreadsItem  `json:"results"`
	Errors  []batchPullRequestReviewThreadsError `json:"errors,omitempty"`
}

func GetPullRequestReviewThreadsBatch(t translations.TranslationHelperFunc) inventory.ServerTool {
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
				Description: fmt.Sprintf("Explicit pull request numbers to hydrate. Accepts up to %d items.", maxPullRequestReviewThreadsBatchSize),
				MinItems:    jsonschema.Ptr(1),
				MaxItems:    jsonschema.Ptr(maxPullRequestReviewThreadsBatchSize),
				Items: &jsonschema.Schema{
					Type:    "integer",
					Minimum: jsonschema.Ptr(1.0),
				},
			},
			"perPage": {
				Type:        "integer",
				Description: "Review threads per pull request page (min 1, max 100)",
				Minimum:     jsonschema.Ptr(1.0),
				Maximum:     jsonschema.Ptr(100.0),
			},
			"afterByPullNumber": {
				Type:                 "object",
				Description:          "Optional per-PR cursor map keyed by stringified pull request number. Each value should be the endCursor returned for that pull request in a previous batch response.",
				AdditionalProperties: &jsonschema.Schema{Type: "string"},
			},
		},
		Required: []string{"owner", "repo", "pullNumbers"},
	}

	return NewTool(
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        "get_pull_request_review_threads_batch",
			Description: t("TOOL_GET_PULL_REQUEST_REVIEW_THREADS_BATCH_DESCRIPTION", "Get review threads for an explicit list of pull requests in a GitHub repository. Returns partial success with per-PR errors and supports per-PR cursors via afterByPullNumber."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_PULL_REQUEST_REVIEW_THREADS_BATCH_USER_TITLE", "Get batch pull request review threads"),
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
			pullNumbers, err := requiredReviewThreadBatchPullNumbers(args, "pullNumbers", maxPullRequestReviewThreadsBatchSize)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			perPage, err := OptionalIntParamWithDefault(args, "perPage", 30)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			basePagination := CursorPaginationParams{PerPage: perPage}
			afterByPullNumber, err := optionalAfterByPullNumberParam(args, "afterByPullNumber")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub GQL client", err), nil, nil
			}

			result := batchPullRequestReviewThreadsResponse{
				Results: make([]batchPullRequestReviewThreadsItem, 0, len(pullNumbers)),
				Errors:  make([]batchPullRequestReviewThreadsError, 0),
			}

			for _, pullNumber := range pullNumbers {
				pagination := basePagination
				if cursor, ok := afterByPullNumber[pullNumber]; ok {
					pagination.After = cursor
				}

				toolResult, err := GetPullRequestReviewComments(ctx, gqlClient, deps, owner, repo, pullNumber, pagination)
				if err != nil {
					return utils.NewToolResultErrorFromErr(fmt.Sprintf("failed to get review threads for pull request %d", pullNumber), err), nil, nil
				}
				if toolResult == nil {
					result.Errors = append(result.Errors, batchPullRequestReviewThreadsError{PullNumber: pullNumber, Message: "failed to get pull request review threads"})
					continue
				}
				if toolResult.IsError {
					result.Errors = append(result.Errors, batchPullRequestReviewThreadsError{PullNumber: pullNumber, Message: getCallToolText(toolResult)})
					continue
				}

				var reviewThreads MinimalReviewThreadsResponse
				if err := json.Unmarshal([]byte(getCallToolText(toolResult)), &reviewThreads); err != nil {
					result.Errors = append(result.Errors, batchPullRequestReviewThreadsError{PullNumber: pullNumber, Message: fmt.Sprintf("failed to decode review thread response: %v", err)})
					continue
				}

				result.Results = append(result.Results, batchPullRequestReviewThreadsItem{
					PullNumber:    pullNumber,
					ReviewThreads: reviewThreads,
				})
			}

			return attachRepoVisibilityIFCLabelLazy(ctx, deps, owner, repo, MarshalledTextResult(result), ifc.LabelListIssues), nil, nil
		},
	)
}

func requiredReviewThreadBatchPullNumbers(args map[string]any, key string, maxItems int) ([]int, error) {
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

func optionalAfterByPullNumberParam(args map[string]any, key string) (map[int]string, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return map[int]string{}, nil
	}

	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parameter %s could not be coerced to map[string]string, is %T", key, raw)
	}

	result := make(map[int]string, len(values))
	for pullNumber, cursorValue := range values {
		cursor, ok := cursorValue.(string)
		if !ok {
			return nil, fmt.Errorf("parameter %s[%s] is not a string, is %T", key, pullNumber, cursorValue)
		}
		parsedPullNumber, err := strconv.Atoi(pullNumber)
		if err != nil || parsedPullNumber < 1 {
			return nil, fmt.Errorf("parameter %s contains invalid pull request key %q", key, pullNumber)
		}
		result[parsedPullNumber] = cursor
	}

	return result, nil
}

func getCallToolText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return text.Text
}
