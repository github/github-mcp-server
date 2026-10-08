package github

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestGranularBatchUpdateIssueLabelsTransport(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid batch", true: "invalid later operation"}[invalid], func(t *testing.T) {
			calls := 0
			httpClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepoByIssueNumberLabels: func(w http.ResponseWriter, r *http.Request) {
					calls++
					expectPath(t, "/repos/owner/repo/issues/1/labels").
						andThen(expectRequestBody(t, []any{"bug"}).
							andThen(mockResponse(t, http.StatusOK, []*gogithub.Label{{Name: "bug"}})))(w, r)
				},
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			tool := GranularBatchUpdateIssueLabels(translations.NullTranslationHelper)
			server := mcp.NewServer(&mcp.Implementation{Name: "batch-label-test"}, nil)
			server.AddTool(&tool.Tool, tool.Handler(deps))
			server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
				return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
					return next(ContextWithDeps(ctx, deps), method, request)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			st, ct := mcp.NewInMemoryTransports()
			ss, err := server.Connect(ctx, st, nil)
			require.NoError(t, err)
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "batch-label-client"}, nil).Connect(ctx, ct, nil)
			require.NoError(t, err)
			defer func() { _ = cs.Close() }()
			operations := []any{map[string]any{"issue_number": 1, "add": []string{"bug"}}}
			if invalid {
				operations = append(operations, map[string]any{"issue_number": 2, "add": []string{" "}})
			}
			result, err := cs.CallTool(ctx, &mcp.CallToolParams{
				Name:      tool.Tool.Name,
				Arguments: map[string]any{"owner": "owner", "repo": "repo", "operations": operations},
			})
			require.NoError(t, err)
			assert.Equal(t, invalid, result.IsError)
			if invalid {
				assert.Zero(t, calls, "validation must precede every mutation")
				assert.Contains(t, getErrorResult(t, result).Text, "operations[1]")
			} else {
				assert.Equal(t, 1, calls)
				assert.Contains(t, getTextResult(t, result).Text, `"applied":true`)
			}
		})
	}
}
