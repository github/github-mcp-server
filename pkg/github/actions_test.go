package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/profiler"
	buffer "github.com/github/github-mcp-server/pkg/buffer"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListWorkflows(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListWorkflows(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_workflows", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsWorkflowsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						workflows := &github.Workflows{
							TotalCount: github.Ptr(2),
							Workflows: []*github.Workflow{
								{
									ID:        github.Ptr(int64(123)),
									Name:      github.Ptr("CI"),
									Path:      github.Ptr(".github/workflows/ci.yml"),
									State:     github.Ptr("active"),
									CreatedAt: &github.Timestamp{},
									UpdatedAt: &github.Timestamp{},
									URL:       github.Ptr("https://api.github.com/repos/owner/repo/actions/workflows/123"),
									HTMLURL:   github.Ptr("https://github.com/owner/repo/actions/workflows/ci.yml"),
									BadgeURL:  github.Ptr("https://github.com/owner/repo/workflows/CI/badge.svg"),
									NodeID:    github.Ptr("W_123"),
								},
								{
									ID:        github.Ptr(int64(456)),
									Name:      github.Ptr("Deploy"),
									Path:      github.Ptr(".github/workflows/deploy.yml"),
									State:     github.Ptr("active"),
									CreatedAt: &github.Timestamp{},
									UpdatedAt: &github.Timestamp{},
									URL:       github.Ptr("https://api.github.com/repos/owner/repo/actions/workflows/456"),
									HTMLURL:   github.Ptr("https://github.com/owner/repo/actions/workflows/deploy.yml"),
									BadgeURL:  github.Ptr("https://github.com/owner/repo/workflows/Deploy/badge.svg"),
									NodeID:    github.Ptr("W_456"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(workflows)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"repo": "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListWorkflows(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response github.Workflows
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.TotalCount)
			assert.Greater(t, *response.TotalCount, 0)
			assert.NotEmpty(t, response.Workflows)
		})
	}
}

func Test_RunWorkflow(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := RunWorkflow(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "run_workflow", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "workflow_id")
	assert.Contains(t, tool.InputSchema.Properties, "ref")
	assert.Contains(t, tool.InputSchema.Properties, "inputs")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "workflow_id", "ref"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
				"ref":         "main",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter workflow_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"ref":   "main",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: workflow_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := RunWorkflow(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Equal(t, "Workflow run has been queued", response["message"])
			assert.Contains(t, response, "workflow_type")
		})
	}
}

func Test_RunWorkflow_WithFilename(t *testing.T) {
	// Test the unified RunWorkflow function with filenames
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run by filename",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "ci.yml",
				"ref":         "main",
			},
			expectError: false,
		},
		{
			name: "successful workflow run by numeric ID as string",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
				"ref":         "main",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter workflow_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"ref":   "main",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: workflow_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := RunWorkflow(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Equal(t, "Workflow run has been queued", response["message"])
			assert.Contains(t, response, "workflow_type")
		})
	}
}

func Test_CancelWorkflowRun(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CancelWorkflowRun(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "cancel_workflow_run", tool.Name)
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
			name: "successful workflow run cancellation",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/actions/runs/12345/cancel",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusAccepted)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name: "conflict when cancelling a workflow run",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/actions/runs/12345/cancel",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusConflict)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError:    true,
			expectedErrMsg: "failed to cancel workflow run",
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CancelWorkflowRun(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Equal(t, "Workflow run has been cancelled", response["message"])
			assert.Equal(t, float64(12345), response["run_id"])
		})
	}
}

func Test_ListWorkflowRunArtifacts(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListWorkflowRunArtifacts(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_workflow_run_artifacts", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful artifacts listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsArtifactsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						artifacts := &github.ArtifactList{
							TotalCount: github.Ptr(int64(2)),
							Artifacts: []*github.Artifact{
								{
									ID:                 github.Ptr(int64(1)),
									NodeID:             github.Ptr("A_1"),
									Name:               github.Ptr("build-artifacts"),
									SizeInBytes:        github.Ptr(int64(1024)),
									URL:                github.Ptr("https://api.github.com/repos/owner/repo/actions/artifacts/1"),
									ArchiveDownloadURL: github.Ptr("https://api.github.com/repos/owner/repo/actions/artifacts/1/zip"),
									Expired:            github.Ptr(false),
									CreatedAt:          &github.Timestamp{},
									UpdatedAt:          &github.Timestamp{},
									ExpiresAt:          &github.Timestamp{},
									WorkflowRun: &github.ArtifactWorkflowRun{
										ID:               github.Ptr(int64(12345)),
										RepositoryID:     github.Ptr(int64(1)),
										HeadRepositoryID: github.Ptr(int64(1)),
										HeadBranch:       github.Ptr("main"),
										HeadSHA:          github.Ptr("abc123"),
									},
								},
								{
									ID:                 github.Ptr(int64(2)),
									NodeID:             github.Ptr("A_2"),
									Name:               github.Ptr("test-results"),
									SizeInBytes:        github.Ptr(int64(512)),
									URL:                github.Ptr("https://api.github.com/repos/owner/repo/actions/artifacts/2"),
									ArchiveDownloadURL: github.Ptr("https://api.github.com/repos/owner/repo/actions/artifacts/2/zip"),
									Expired:            github.Ptr(false),
									CreatedAt:          &github.Timestamp{},
									UpdatedAt:          &github.Timestamp{},
									ExpiresAt:          &github.Timestamp{},
									WorkflowRun: &github.ArtifactWorkflowRun{
										ID:               github.Ptr(int64(12345)),
										RepositoryID:     github.Ptr(int64(1)),
										HeadRepositoryID: github.Ptr(int64(1)),
										HeadBranch:       github.Ptr("main"),
										HeadSHA:          github.Ptr("abc123"),
									},
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(artifacts)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListWorkflowRunArtifacts(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response github.ArtifactList
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.TotalCount)
			assert.Greater(t, *response.TotalCount, int64(0))
			assert.NotEmpty(t, response.Artifacts)
		})
	}
}

func Test_DownloadWorkflowRunArtifact(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := DownloadWorkflowRunArtifact(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "download_workflow_run_artifact", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "artifact_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "artifact_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful artifact download URL",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/actions/artifacts/123/zip",
						Method:  "GET",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						// GitHub returns a 302 redirect to the download URL
						w.Header().Set("Location", "https://api.github.com/repos/owner/repo/actions/artifacts/123/download")
						w.WriteHeader(http.StatusFound)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"artifact_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter artifact_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: artifact_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DownloadWorkflowRunArtifact(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Contains(t, response, "download_url")
			assert.Contains(t, response, "message")
			assert.Equal(t, "Artifact is available for download", response["message"])
			assert.Equal(t, float64(123), response["artifact_id"])
		})
	}
}

func Test_DeleteWorkflowRunLogs(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := DeleteWorkflowRunLogs(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "delete_workflow_run_logs", tool.Name)
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
			name: "successful logs deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteReposActionsRunsLogsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteWorkflowRunLogs(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.Equal(t, "Workflow run logs have been deleted", response["message"])
			assert.Equal(t, float64(12345), response["run_id"])
		})
	}
}

func Test_GetWorkflowRunUsage(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetWorkflowRunUsage(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_workflow_run_usage", tool.Name)
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
			name: "successful workflow run usage",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsTimingByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						usage := &github.WorkflowRunUsage{
							Billable: &github.WorkflowRunBillMap{
								"UBUNTU": &github.WorkflowRunBill{
									TotalMS: github.Ptr(int64(120000)),
									Jobs:    github.Ptr(2),
									JobRuns: []*github.WorkflowRunJobRun{
										{
											JobID:      github.Ptr(1),
											DurationMS: github.Ptr(int64(60000)),
										},
										{
											JobID:      github.Ptr(2),
											DurationMS: github.Ptr(int64(60000)),
										},
									},
								},
							},
							RunDurationMS: github.Ptr(int64(120000)),
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(usage)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: run_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetWorkflowRunUsage(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			// Unmarshal and verify the result
			var response github.WorkflowRunUsage
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.RunDurationMS)
			assert.NotNil(t, response.Billable)
		})
	}
}

func Test_GetJobLogs(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetJobLogs(stubGetClientFn(mockClient), translations.NullTranslationHelper, 5000)

	assert.Equal(t, "get_job_logs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "job_id")
	assert.Contains(t, tool.InputSchema.Properties, "run_id")
	assert.Contains(t, tool.InputSchema.Properties, "failed_only")
	assert.Contains(t, tool.InputSchema.Properties, "return_content")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		checkResponse  func(t *testing.T, response map[string]any)
	}{
		{
			name: "successful single job logs with URL",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Location", "https://github.com/logs/job/123")
						w.WriteHeader(http.StatusFound)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"job_id": float64(123),
			},
			expectError: false,
			checkResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, float64(123), response["job_id"])
				assert.Contains(t, response, "logs_url")
				assert.Equal(t, "Job logs are available for download", response["message"])
				assert.Contains(t, response, "note")
			},
		},
		{
			name: "successful failed jobs logs",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsJobsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						jobs := &github.Jobs{
							TotalCount: github.Ptr(3),
							Jobs: []*github.WorkflowJob{
								{
									ID:         github.Ptr(int64(1)),
									Name:       github.Ptr("test-job-1"),
									Conclusion: github.Ptr("success"),
								},
								{
									ID:         github.Ptr(int64(2)),
									Name:       github.Ptr("test-job-2"),
									Conclusion: github.Ptr("failure"),
								},
								{
									ID:         github.Ptr(int64(3)),
									Name:       github.Ptr("test-job-3"),
									Conclusion: github.Ptr("failure"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(jobs)
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Location", "https://github.com/logs/job/"+r.URL.Path[len(r.URL.Path)-1:])
						w.WriteHeader(http.StatusFound)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"run_id":      float64(456),
				"failed_only": true,
			},
			expectError: false,
			checkResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, float64(456), response["run_id"])
				assert.Equal(t, float64(3), response["total_jobs"])
				assert.Equal(t, float64(2), response["failed_jobs"])
				assert.Contains(t, response, "logs")
				assert.Equal(t, "Retrieved logs for 2 failed jobs", response["message"])

				logs, ok := response["logs"].([]interface{})
				assert.True(t, ok)
				assert.Len(t, logs, 2)
			},
		},
		{
			name: "no failed jobs found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsJobsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						jobs := &github.Jobs{
							TotalCount: github.Ptr(2),
							Jobs: []*github.WorkflowJob{
								{
									ID:         github.Ptr(int64(1)),
									Name:       github.Ptr("test-job-1"),
									Conclusion: github.Ptr("success"),
								},
								{
									ID:         github.Ptr(int64(2)),
									Name:       github.Ptr("test-job-2"),
									Conclusion: github.Ptr("success"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(jobs)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"run_id":      float64(456),
				"failed_only": true,
			},
			expectError: false,
			checkResponse: func(t *testing.T, response map[string]any) {
				assert.Equal(t, "No failed jobs found in this workflow run", response["message"])
				assert.Equal(t, float64(456), response["run_id"])
				assert.Equal(t, float64(2), response["total_jobs"])
				assert.Equal(t, float64(0), response["failed_jobs"])
			},
		},
		{
			name:         "missing job_id when not using failed_only",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "job_id is required when failed_only is false",
		},
		{
			name:         "missing run_id when using failed_only",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"failed_only": true,
			},
			expectError:    true,
			expectedErrMsg: "run_id is required when failed_only is true",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"repo":   "repo",
				"job_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter repo",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":  "owner",
				"job_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name: "API error when getting single job logs",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]string{
							"message": "Not Found",
						})
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"job_id": float64(999),
			},
			expectError: true,
		},
		{
			name: "API error when listing workflow jobs for failed_only",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsJobsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]string{
							"message": "Not Found",
						})
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"run_id":      float64(999),
				"failed_only": true,
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetJobLogs(stubGetClientFn(client), translations.NullTranslationHelper, 5000)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			if tc.expectError {
				// For API errors, just verify we got an error
				assert.True(t, result.IsError)
				return
			}

			// Unmarshal and verify the result
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			if tc.checkResponse != nil {
				tc.checkResponse(t, response)
			}
		})
	}
}

func Test_GetJobLogs_WithContentReturn(t *testing.T) {
	// Test the return_content functionality with a mock HTTP server
	logContent := "2023-01-01T10:00:00.000Z Starting job...\n2023-01-01T10:00:01.000Z Running tests...\n2023-01-01T10:00:02.000Z Job completed successfully"

	// Create a test server to serve log content
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(logContent))
	}))
	defer testServer.Close()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", testServer.URL)
				w.WriteHeader(http.StatusFound)
			}),
		),
	)

	client := github.NewClient(mockedClient)
	_, handler := GetJobLogs(stubGetClientFn(client), translations.NullTranslationHelper, 5000)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(123), response["job_id"])
	assert.Equal(t, logContent, response["logs_content"])
	assert.Equal(t, "Job logs content retrieved successfully", response["message"])
	assert.NotContains(t, response, "logs_url") // Should not have URL when returning content
}

func Test_GetJobLogs_WithContentReturnAndTailLines(t *testing.T) {
	// Test the return_content functionality with a mock HTTP server
	logContent := "2023-01-01T10:00:00.000Z Starting job...\n2023-01-01T10:00:01.000Z Running tests...\n2023-01-01T10:00:02.000Z Job completed successfully"
	expectedLogContent := "2023-01-01T10:00:02.000Z Job completed successfully"

	// Create a test server to serve log content
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(logContent))
	}))
	defer testServer.Close()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", testServer.URL)
				w.WriteHeader(http.StatusFound)
			}),
		),
	)

	client := github.NewClient(mockedClient)
	_, handler := GetJobLogs(stubGetClientFn(client), translations.NullTranslationHelper, 5000)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
		"tail_lines":     float64(1), // Requesting last 1 line
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(123), response["job_id"])
	assert.Equal(t, float64(3), response["original_length"])
	assert.Equal(t, expectedLogContent, response["logs_content"])
	assert.Equal(t, "Job logs content retrieved successfully", response["message"])
	assert.NotContains(t, response, "logs_url") // Should not have URL when returning content
}

func Test_GetJobLogs_WithContentReturnAndLargeTailLines(t *testing.T) {
	logContent := "Line 1\nLine 2\nLine 3"
	expectedLogContent := "Line 1\nLine 2\nLine 3"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(logContent))
	}))
	defer testServer.Close()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", testServer.URL)
				w.WriteHeader(http.StatusFound)
			}),
		),
	)

	client := github.NewClient(mockedClient)
	_, handler := GetJobLogs(stubGetClientFn(client), translations.NullTranslationHelper, 5000)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
		"tail_lines":     float64(100),
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(123), response["job_id"])
	assert.Equal(t, float64(3), response["original_length"])
	assert.Equal(t, expectedLogContent, response["logs_content"])
	assert.Equal(t, "Job logs content retrieved successfully", response["message"])
	assert.NotContains(t, response, "logs_url")
}

func Test_MemoryUsage_SlidingWindow_vs_NoWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory profiling test in short mode")
	}

	const logLines = 100000
	const bufferSize = 5000
	largeLogContent := strings.Repeat("log line with some content\n", logLines-1) + "final log line"

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largeLogContent))
	}))
	defer testServer.Close()

	os.Setenv("GITHUB_MCP_PROFILING_ENABLED", "true")
	defer os.Unsetenv("GITHUB_MCP_PROFILING_ENABLED")

	profiler.InitFromEnv(nil)
	ctx := context.Background()

	debug.SetGCPercent(-1)
	defer debug.SetGCPercent(100)

	for i := 0; i < 3; i++ {
		runtime.GC()
	}

	var baselineStats runtime.MemStats
	runtime.ReadMemStats(&baselineStats)

	profile1, err1 := profiler.ProfileFuncWithMetrics(ctx, "sliding_window", func() (int, int64, error) {
		resp1, err := http.Get(testServer.URL)
		if err != nil {
			return 0, 0, err
		}
		defer resp1.Body.Close()                                                                  //nolint:bodyclose
		content, totalLines, _, err := buffer.ProcessResponseAsRingBufferToEnd(resp1, bufferSize) //nolint:bodyclose
		return totalLines, int64(len(content)), err
	})
	require.NoError(t, err1)

	for i := 0; i < 3; i++ {
		runtime.GC()
	}

	profile2, err2 := profiler.ProfileFuncWithMetrics(ctx, "no_window", func() (int, int64, error) {
		resp2, err := http.Get(testServer.URL)
		if err != nil {
			return 0, 0, err
		}
		defer resp2.Body.Close() //nolint:bodyclose

		allContent, err := io.ReadAll(resp2.Body)
		if err != nil {
			return 0, 0, err
		}

		allLines := strings.Split(string(allContent), "\n")
		var nonEmptyLines []string
		for _, line := range allLines {
			if line != "" {
				nonEmptyLines = append(nonEmptyLines, line)
			}
		}
		totalLines := len(nonEmptyLines)

		var resultLines []string
		if totalLines > bufferSize {
			resultLines = nonEmptyLines[totalLines-bufferSize:]
		} else {
			resultLines = nonEmptyLines
		}

		result := strings.Join(resultLines, "\n")
		return totalLines, int64(len(result)), nil
	})
	require.NoError(t, err2)

	assert.Greater(t, profile2.MemoryDelta, profile1.MemoryDelta,
		"Sliding window should use less memory than reading all into memory")

	assert.Equal(t, profile1.LinesCount, profile2.LinesCount,
		"Both approaches should count the same number of input lines")
	assert.InDelta(t, profile1.BytesCount, profile2.BytesCount, 100,
		"Both approaches should produce similar output sizes (within 100 bytes)")

	memoryReduction := float64(profile2.MemoryDelta-profile1.MemoryDelta) / float64(profile2.MemoryDelta) * 100
	t.Logf("Memory reduction: %.1f%% (%.2f MB vs %.2f MB)",
		memoryReduction,
		float64(profile2.MemoryDelta)/1024/1024,
		float64(profile1.MemoryDelta)/1024/1024)

	t.Logf("Baseline: %d bytes", baselineStats.Alloc)
	t.Logf("Sliding window: %s", profile1.String())
	t.Logf("No window: %s", profile2.String())
}
