package github

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGranularBatchUpdateIssueLabels(t *testing.T) {
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectToolErr  bool
		expectedErrMsg string
	}{
		{
			name: "add and remove across multiple issues",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepoByIssueNumberLabels: expectRequestBody(t, []any{"bug", "priority/high"}).
					andThen(mockResponse(t, http.StatusOK, []*gogithub.Label{
						{Name: "bug"},
						{Name: "priority/high"},
					})),
				DeleteReposIssuesByOwnerByRepoByIssueNumberLabel: mockResponse(t, http.StatusNoContent, nil),
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"operations": []any{
					map[string]any{
						"issue_number": float64(1),
						"add":          []any{"bug", "priority/high"},
					},
					map[string]any{
						"issue_number": float64(2),
						"remove":       []any{"wontfix"},
					},
				},
			},
			expectToolErr: false,
		},
		{
			name:         "empty operations array rejected",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"operations": []any{},
			},
			expectToolErr:  true,
			expectedErrMsg: "operations must contain at least one entry",
		},
		{
			name:         "missing operations parameter rejected",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectToolErr:  true,
			expectedErrMsg: "missing required parameter: operations",
		},
		{
			name:         "operation without add or remove rejected",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"operations": []any{
					map[string]any{"issue_number": float64(1)},
				},
			},
			expectToolErr:  true,
			expectedErrMsg: "at least one non-empty of add or remove is required",
		},
		{
			name:         "duplicate issue numbers rejected",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"operations": []any{
					map[string]any{"issue_number": float64(1), "add": []any{"bug"}},
					map[string]any{"issue_number": float64(1), "remove": []any{"wontfix"}},
				},
			},
			expectToolErr:  true,
			expectedErrMsg: "duplicate issue_number 1",
		},
		{
			name:         "empty label name in add rejected",
			mockedClient: MockHTTPClientWithHandlers(nil),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"operations": []any{
					map[string]any{"issue_number": float64(1), "add": []any{""}},
				},
			},
			expectToolErr:  true,
			expectedErrMsg: "add contains an empty label name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, tc.mockedClient)}
			serverTool := GranularBatchUpdateIssueLabels(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			if tc.expectToolErr {
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}
			assert.False(t, result.IsError)
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, `"issue_number":1`)
			assert.Contains(t, textContent.Text, `"applied":true`)
		})
	}
}

func TestGranularBatchUpdateIssueLabelsPartialFailure(t *testing.T) {
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PostReposIssuesByOwnerByRepoByIssueNumberLabels: func(_ http.ResponseWriter, _ *http.Request) {},
	}))
	deps := BaseDeps{Client: client}
	serverTool := GranularBatchUpdateIssueLabels(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner": "owner",
		"repo":  "repo",
		"operations": []any{
			map[string]any{"issue_number": float64(1), "add": []any{"bug"}},
		},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	// A per-issue API failure must surface as a tool error with a JSON body
	// identifying the failing issue.
	assert.True(t, result.IsError, "expected IsError on API failure")
	textContent := getTextResult(t, result)
	assert.True(t, strings.Contains(textContent.Text, "add failed"))
}
