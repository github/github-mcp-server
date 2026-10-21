package github

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetPullRequestReviewThreadsBatch(t *testing.T) {
	serverTool := GetPullRequestReviewThreadsBatch(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_pull_request_review_threads_batch", tool.Name)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumbers")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "afterByPullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumbers"})

	tests := []struct {
		name           string
		gqlHTTPClient  *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		validateResult func(t *testing.T, textContent string)
	}{
		{
			name: "successful batch review thread fetch with per-pr cursor forwarding",
			gqlHTTPClient: newBatchReviewThreadsHTTPClient(t,
				map[int]MinimalReviewThreadsResponse{
					42: {
						ReviewThreads: []MinimalReviewThread{{ID: "RT-42", TotalCount: 1, Comments: []MinimalReviewComment{{Body: "Looks good", Path: "file1.go", Author: "reviewer1", HTMLURL: "https://github.com/owner/repo/pull/42#discussion_r42"}}}},
						TotalCount:    1,
						PageInfo:      MinimalPageInfo{HasNextPage: false, EndCursor: "cursor-42-next"},
					},
					18: {
						ReviewThreads: []MinimalReviewThread{{ID: "RT-18", TotalCount: 1, Comments: []MinimalReviewComment{{Body: "Needs update", Path: "file2.go", Author: "reviewer2", HTMLURL: "https://github.com/owner/repo/pull/18#discussion_r18"}}}},
						TotalCount:    1,
						PageInfo:      MinimalPageInfo{HasNextPage: true, EndCursor: "cursor-18-next"},
					},
				},
				map[int]string{42: "", 18: "cursor-18-prev"},
				nil,
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(18)},
				"afterByPullNumber": map[string]any{
					"18": "cursor-18-prev",
				},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestReviewThreadsResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.Results, 2)
				assert.Empty(t, result.Errors)
				assert.Equal(t, 42, result.Results[0].PullNumber)
				assert.Equal(t, "RT-42", result.Results[0].ReviewThreads.ReviewThreads[0].ID)
				assert.Equal(t, 18, result.Results[1].PullNumber)
				assert.Equal(t, "cursor-18-next", result.Results[1].ReviewThreads.PageInfo.EndCursor)
			},
		},
		{
			name: "partial GraphQL failures become per-pr errors",
			gqlHTTPClient: newBatchReviewThreadsHTTPClient(t,
				map[int]MinimalReviewThreadsResponse{
					42: {
						ReviewThreads: []MinimalReviewThread{{ID: "RT-42", TotalCount: 1, Comments: []MinimalReviewComment{{Body: "Looks good", Path: "file1.go", Author: "reviewer1", HTMLURL: "https://github.com/owner/repo/pull/42#discussion_r42"}}}},
						TotalCount:    1,
						PageInfo:      MinimalPageInfo{},
					},
				},
				map[int]string{42: "", 999: ""},
				map[int]string{999: "Could not resolve to a PullRequest with the number of 999."},
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(999)},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestReviewThreadsResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.Results, 1)
				assert.Equal(t, 42, result.Results[0].PullNumber)
				assert.Len(t, result.Errors, 1)
				assert.Equal(t, 999, result.Errors[0].PullNumber)
				assert.Contains(t, result.Errors[0].Message, "failed to get pull request review threads")
			},
		},
		{
			name: "duplicate pull numbers are deduplicated before hydration",
			gqlHTTPClient: newBatchReviewThreadsHTTPClient(t,
				map[int]MinimalReviewThreadsResponse{
					42: {
						ReviewThreads: []MinimalReviewThread{{ID: "RT-42", TotalCount: 1, Comments: []MinimalReviewComment{{Body: "Looks good", Path: "file1.go", Author: "reviewer1", HTMLURL: "https://github.com/owner/repo/pull/42#discussion_r42"}}}},
						TotalCount:    1,
						PageInfo:      MinimalPageInfo{},
					},
				},
				map[int]string{42: ""},
				nil,
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{float64(42), float64(42), float64(42)},
			},
			validateResult: func(t *testing.T, textContent string) {
				var result batchPullRequestReviewThreadsResponse
				require.NoError(t, json.Unmarshal([]byte(textContent), &result))
				assert.Len(t, result.Results, 1)
				assert.Empty(t, result.Errors)
				assert.Equal(t, 42, result.Results[0].PullNumber)
			},
		},
		{
			name:          "empty pullNumbers fails validation",
			gqlHTTPClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": []any{},
			},
			expectError:    true,
			expectedErrMsg: "must contain at least one pull request number",
		},
		{
			name:          "oversized pullNumbers fails validation",
			gqlHTTPClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumbers": oversizedReviewThreadArgs(maxPullRequestReviewThreadsBatchSize + 1),
			},
			expectError:    true,
			expectedErrMsg: "exceeds the maximum batch size",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4.NewClient(tc.gqlHTTPClient)
			deps := BaseDeps{
				GQLClient: gqlClient,
				Client:    mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})),
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getErrorResult(t, result)
				assert.Contains(t, text.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result)
			tc.validateResult(t, text.Text)
		})
	}
}

func oversizedReviewThreadArgs(count int) []any {
	values := make([]any, 0, count)
	for i := range count {
		values = append(values, float64(i+1))
	}
	return values
}

type batchReviewThreadsRoundTripper struct {
	t             *testing.T
	responses     map[int]MinimalReviewThreadsResponse
	expectedAfter map[int]string
	errorMessages map[int]string
}

func newBatchReviewThreadsHTTPClient(t *testing.T, responses map[int]MinimalReviewThreadsResponse, expectedAfter map[int]string, errorMessages map[int]string) *http.Client {
	return &http.Client{Transport: &batchReviewThreadsRoundTripper{t: t, responses: responses, expectedAfter: expectedAfter, errorMessages: errorMessages}}
}

func (rt *batchReviewThreadsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.t.Helper()
	body, err := io.ReadAll(req.Body)
	require.NoError(rt.t, err)
	_ = req.Body.Close()

	var gqlReq struct {
		Variables map[string]any `json:"variables"`
	}
	require.NoError(rt.t, json.Unmarshal(body, &gqlReq))

	prNum := int(gqlReq.Variables["prNum"].(float64))
	var actualAfter string
	if afterValue, ok := gqlReq.Variables["after"]; ok && afterValue != nil {
		actualAfter = afterValue.(string)
	}
	assert.Equal(rt.t, rt.expectedAfter[prNum], actualAfter)

	var payload map[string]any
	if errMsg, ok := rt.errorMessages[prNum]; ok {
		payload = map[string]any{"errors": []map[string]any{{"message": errMsg}}}
	} else {
		payload = map[string]any{
			"data": map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviewThreads": batchReviewThreadsPayload(rt.responses[prNum]),
					},
				},
			},
		}
	}

	jsonBody, err := json.Marshal(payload)
	require.NoError(rt.t, err)

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(jsonBody)),
	}, nil
}

func batchReviewThreadsPayload(resp MinimalReviewThreadsResponse) map[string]any {
	nodes := make([]map[string]any, 0, len(resp.ReviewThreads))
	for _, thread := range resp.ReviewThreads {
		comments := make([]map[string]any, 0, len(thread.Comments))
		for _, comment := range thread.Comments {
			commentNode := map[string]any{
				"id":   comment.HTMLURL,
				"body": comment.Body,
				"path": comment.Path,
				"url":  comment.HTMLURL,
				"author": map[string]any{
					"login": comment.Author,
				},
			}
			if comment.Line != nil {
				commentNode["line"] = *comment.Line
			}
			if comment.CreatedAt != "" {
				commentNode["createdAt"] = comment.CreatedAt
				commentNode["updatedAt"] = comment.CreatedAt
			}
			comments = append(comments, commentNode)
		}

		nodes = append(nodes, map[string]any{
			"id":          thread.ID,
			"isResolved":  thread.IsResolved,
			"isOutdated":  thread.IsOutdated,
			"isCollapsed": thread.IsCollapsed,
			"comments": map[string]any{
				"totalCount": thread.TotalCount,
				"nodes":      comments,
			},
		})
	}

	return map[string]any{
		"nodes": nodes,
		"pageInfo": map[string]any{
			"hasNextPage":     resp.PageInfo.HasNextPage,
			"hasPreviousPage": resp.PageInfo.HasPreviousPage,
			"startCursor":     resp.PageInfo.StartCursor,
			"endCursor":       resp.PageInfo.EndCursor,
		},
		"totalCount": resp.TotalCount,
	}
}
