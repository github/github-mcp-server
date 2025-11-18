package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListWorkflowRuns(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListWorkflowRuns(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_workflow_runs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "workflow_id")
	assert.Contains(t, tool.InputSchema.Properties, "actor")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	assert.Contains(t, tool.InputSchema.Properties, "event")
	assert.Contains(t, tool.InputSchema.Properties, "status")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "workflow_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow runs listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowId,
					github.WorkflowRuns{
						TotalCount: github.Ptr(2),
						WorkflowRuns: []*github.WorkflowRun{
							{
								ID:     github.Ptr(int64(123)),
								Name:   github.Ptr("CI Run 1"),
								Status: github.Ptr("completed"),
							},
							{
								ID:     github.Ptr(int64(456)),
								Name:   github.Ptr("CI Run 2"),
								Status: github.Ptr("in_progress"),
							},
						},
					},
				),
			),
			requestArgs: map[string]any{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"workflow_id": "ci.yml",
			},
			expectError: false,
		},
		{
			name: "with optional filters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowId,
					github.WorkflowRuns{
						TotalCount:   github.Ptr(1),
						WorkflowRuns: []*github.WorkflowRun{},
					},
				),
			),
			requestArgs: map[string]any{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"workflow_id": "ci.yml",
				"actor":       "testuser",
				"branch":      "main",
				"event":       "push",
				"status":      "completed",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter workflow_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: workflow_id",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"repo":        "test-repo",
				"workflow_id": "ci.yml",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := ListWorkflowRuns(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response github.WorkflowRuns
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.TotalCount)
		})
	}
}

func Test_GetWorkflowRun(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := GetWorkflowRun(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_workflow_run", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123",
						Method:  "GET",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						workflowRun := &github.WorkflowRun{
							ID:         github.Ptr(int64(123)),
							Name:       github.Ptr("CI Run"),
							Status:     github.Ptr("completed"),
							Conclusion: github.Ptr("success"),
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(workflowRun)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
		{
			name:         "missing required parameter repo",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"run_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetWorkflowRun(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response github.WorkflowRun
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.ID)
		})
	}
}

func Test_GetWorkflowRunLogs(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := GetWorkflowRunLogs(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_workflow_run_logs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run logs retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123/logs",
						Method:  "GET",
					},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Location", "https://example.com/logs.zip")
						w.WriteHeader(http.StatusFound)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetWorkflowRunLogs(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "logs_url")
			assert.Contains(t, response, "message")
		})
	}
}

func Test_ListWorkflowJobs(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := ListWorkflowJobs(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_workflow_jobs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.Contains(t, tool.InputSchema.Properties, "filter")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow jobs listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123/jobs",
						Method:  "GET",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						jobs := &github.Jobs{
							TotalCount: github.Ptr(2),
							Jobs: []*github.WorkflowJob{
								{
									ID:         github.Ptr(int64(789)),
									Name:       github.Ptr("build"),
									Status:     github.Ptr("completed"),
									Conclusion: github.Ptr("success"),
								},
								{
									ID:         github.Ptr(int64(790)),
									Name:       github.Ptr("test"),
									Status:     github.Ptr("in_progress"),
									Conclusion: github.Ptr(""),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(jobs)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError: false,
		},
		{
			name: "with filter parameter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123/jobs",
						Method:  "GET",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						jobs := &github.Jobs{
							TotalCount: github.Ptr(1),
							Jobs:       []*github.WorkflowJob{},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(jobs)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
				"filter": "latest",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := ListWorkflowJobs(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "jobs")
		})
	}
}

func Test_RerunWorkflowRun(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := RerunWorkflowRun(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "rerun_workflow_run", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run rerun",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123/rerun",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusCreated)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := RerunWorkflowRun(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "message")
			assert.Contains(t, response, "run_id")
		})
	}
}

func Test_RerunFailedJobs(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := RerunFailedJobs(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "rerun_failed_jobs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful failed jobs rerun",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/test-owner/test-repo/actions/runs/123/rerun-failed-jobs",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusCreated)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"repo":   "test-repo",
				"run_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "test-owner",
				"repo":  "test-repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
		{
			name:         "missing required parameter repo",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":  "test-owner",
				"run_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := RerunFailedJobs(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "message")
			assert.Contains(t, response, "run_id")
		})
	}
}
