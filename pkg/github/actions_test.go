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
	"github.com/github/github-mcp-server/internal/toolsnaps"
	buffer "github.com/github/github-mcp-server/pkg/buffer"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListWorkflows(t *testing.T) {
	// Verify tool definition once
	toolDef := ListWorkflows(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "list_workflows", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "perPage")
	assert.Contains(t, inputSchema.Properties, "page")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsWorkflowsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := RunWorkflow(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "run_workflow", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "workflow_id")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "ref")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "inputs")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "workflow_id", "ref"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := RunWorkflow(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run by filename",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := CancelWorkflowRun(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "cancel_workflow_run", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "run_id")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run cancellation",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/actions/runs/12345/cancel": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusAccepted)
				}),
			}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name: "conflict when cancelling a workflow run",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/actions/runs/12345/cancel": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusConflict)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := ListWorkflowRunArtifacts(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "list_workflow_run_artifacts", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "run_id")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "perPage")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "page")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful artifacts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsArtifactsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := DownloadWorkflowRunArtifact(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "download_workflow_run_artifact", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "artifact_id")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "artifact_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful artifact download URL",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/owner/repo/actions/artifacts/123/zip": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					// GitHub returns a 302 redirect to the download URL
					w.Header().Set("Location", "https://api.github.com/repos/owner/repo/actions/artifacts/123/download")
					w.WriteHeader(http.StatusFound)
				}),
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"artifact_id": float64(123),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter artifact_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := DeleteWorkflowRunLogs(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "delete_workflow_run_logs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "run_id")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful logs deletion",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposActionsRunsLogsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := GetWorkflowRunUsage(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_workflow_run_usage", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "run_id")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow run usage",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsTimingByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"run_id": float64(12345),
			},
			expectError: false,
		},
		{
			name:         "missing required parameter run_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
	toolDef := GetJobLogs(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_job_logs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "job_id")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "run_id")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "failed_only")
	assert.Contains(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Properties, "return_content")
	assert.ElementsMatch(t, toolDef.Tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo"})

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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Location", "https://github.com/logs/job/123")
					w.WriteHeader(http.StatusFound)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsJobsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
				GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Location", "https://github.com/logs/job/"+r.URL.Path[len(r.URL.Path)-1:])
					w.WriteHeader(http.StatusFound)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsJobsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "job_id is required when failed_only is false",
		},
		{
			name:         "missing run_id when using failed_only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"repo":   "repo",
				"job_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"job_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name: "API error when getting single job logs",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"message": "Not Found",
					})
				}),
			}),
			requestArgs: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"job_id": float64(999),
			},
			expectError: true,
		},
		{
			name: "API error when listing workflow jobs for failed_only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsRunsJobsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"message": "Not Found",
					})
				}),
			}),
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
			deps := BaseDeps{
				Client:            client,
				ContentWindowSize: 5000,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", testServer.URL)
			w.WriteHeader(http.StatusFound)
		}),
	})

	client := github.NewClient(mockedClient)
	toolDef := GetJobLogs(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client:            client,
		ContentWindowSize: 5000,
	}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", testServer.URL)
			w.WriteHeader(http.StatusFound)
		}),
	})

	client := github.NewClient(mockedClient)
	toolDef := GetJobLogs(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client:            client,
		ContentWindowSize: 5000,
	}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
		"tail_lines":     float64(1), // Requesting last 1 line
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", testServer.URL)
			w.WriteHeader(http.StatusFound)
		}),
	})

	client := github.NewClient(mockedClient)
	toolDef := GetJobLogs(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client:            client,
		ContentWindowSize: 5000,
	}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":          "owner",
		"repo":           "repo",
		"job_id":         float64(123),
		"return_content": true,
		"tail_lines":     float64(100),
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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

func Test_ListWorkflowRuns(t *testing.T) {
	// Verify tool definition once
	toolDef := ListWorkflowRuns(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "list_workflow_runs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "workflow_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "workflow_id"})
}

func Test_GetWorkflowRun(t *testing.T) {
	// Verify tool definition once
	toolDef := GetWorkflowRun(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_workflow_run", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "run_id"})
}

func Test_GetWorkflowRunLogs(t *testing.T) {
	// Verify tool definition once
	toolDef := GetWorkflowRunLogs(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_workflow_run_logs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "run_id"})
}

func Test_ListWorkflowJobs(t *testing.T) {
	// Verify tool definition once
	toolDef := ListWorkflowJobs(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "list_workflow_jobs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "run_id"})
}

func Test_RerunWorkflowRun(t *testing.T) {
	// Verify tool definition once
	toolDef := RerunWorkflowRun(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "rerun_workflow_run", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "run_id"})
}

func Test_RerunFailedJobs(t *testing.T) {
	// Verify tool definition once
	toolDef := RerunFailedJobs(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "rerun_failed_jobs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "run_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful rerun of failed jobs",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/actions/runs/12345/rerun-failed-jobs",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusCreated)
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Equal(t, "Failed jobs have been queued for re-run", response["message"])
			assert.Equal(t, float64(12345), response["run_id"])
		})
	}
}

func Test_RerunWorkflowRun_Behavioral(t *testing.T) {
	toolDef := RerunWorkflowRun(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful rerun of workflow run",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/actions/runs/12345/rerun",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusCreated)
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Equal(t, "Workflow run has been queued for re-run", response["message"])
			assert.Equal(t, float64(12345), response["run_id"])
		})
	}
}

func Test_ListWorkflowRuns_Behavioral(t *testing.T) {
	toolDef := ListWorkflowRuns(translations.NullTranslationHelper)

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
				mock.WithRequestMatchHandler(
					mock.GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						runs := &github.WorkflowRuns{
							TotalCount: github.Ptr(2),
							WorkflowRuns: []*github.WorkflowRun{
								{
									ID:         github.Ptr(int64(123)),
									Name:       github.Ptr("CI"),
									Status:     github.Ptr("completed"),
									Conclusion: github.Ptr("success"),
								},
								{
									ID:         github.Ptr(int64(456)),
									Name:       github.Ptr("CI"),
									Status:     github.Ptr("completed"),
									Conclusion: github.Ptr("failure"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(runs)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "ci.yml",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter workflow_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: workflow_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Greater(t, *response.TotalCount, 0)
		})
	}
}

func Test_GetWorkflowRun_Behavioral(t *testing.T) {
	toolDef := GetWorkflowRun(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful get workflow run",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						run := &github.WorkflowRun{
							ID:         github.Ptr(int64(12345)),
							Name:       github.Ptr("CI"),
							Status:     github.Ptr("completed"),
							Conclusion: github.Ptr("success"),
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(run)
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Equal(t, int64(12345), *response.ID)
		})
	}
}

func Test_GetWorkflowRunLogs_Behavioral(t *testing.T) {
	toolDef := GetWorkflowRunLogs(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful get workflow run logs",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsLogsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Location", "https://github.com/logs/run/12345")
						w.WriteHeader(http.StatusFound)
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Equal(t, "Workflow run logs are available for download", response["message"])
		})
	}
}

func Test_ListWorkflowJobs_Behavioral(t *testing.T) {
	toolDef := ListWorkflowJobs(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful list workflow jobs",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsRunsJobsByOwnerByRepoByRunId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						jobs := &github.Jobs{
							TotalCount: github.Ptr(2),
							Jobs: []*github.WorkflowJob{
								{
									ID:         github.Ptr(int64(1)),
									Name:       github.Ptr("build"),
									Status:     github.Ptr("completed"),
									Conclusion: github.Ptr("success"),
								},
								{
									ID:         github.Ptr(int64(2)),
									Name:       github.Ptr("test"),
									Status:     github.Ptr("completed"),
									Conclusion: github.Ptr("failure"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(jobs)
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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

// Tests for consolidated actions tools

func Test_ActionsList(t *testing.T) {
	// Verify tool definition once
	toolDef := ActionsList(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "actions_list", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "repo"})
}

func Test_ActionsList_ListWorkflows(t *testing.T) {
	toolDef := ActionsList(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful workflow list",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposActionsWorkflowsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						workflows := &github.Workflows{
							TotalCount: github.Ptr(2),
							Workflows: []*github.Workflow{
								{
									ID:    github.Ptr(int64(1)),
									Name:  github.Ptr("CI"),
									Path:  github.Ptr(".github/workflows/ci.yml"),
									State: github.Ptr("active"),
								},
								{
									ID:    github.Ptr(int64(2)),
									Name:  github.Ptr("Deploy"),
									Path:  github.Ptr(".github/workflows/deploy.yml"),
									State: github.Ptr("active"),
								},
							},
						}
						w.WriteHeader(http.StatusOK)
						_ = json.NewEncoder(w).Encode(workflows)
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "list_workflows",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter method",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			require.Equal(t, tc.expectError, result.IsError)

			textContent := getTextResult(t, result)

			if tc.expectedErrMsg != "" {
				assert.Equal(t, tc.expectedErrMsg, textContent.Text)
				return
			}

			var response github.Workflows
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			assert.NotNil(t, response.TotalCount)
			assert.Greater(t, *response.TotalCount, 0)
		})
	}
}

func Test_ActionsList_ListWorkflowRuns(t *testing.T) {
	toolDef := ActionsList(translations.NullTranslationHelper)

	t.Run("successful workflow runs list", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowId,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					runs := &github.WorkflowRuns{
						TotalCount: github.Ptr(1),
						WorkflowRuns: []*github.WorkflowRun{
							{
								ID:         github.Ptr(int64(123)),
								Name:       github.Ptr("CI"),
								Status:     github.Ptr("completed"),
								Conclusion: github.Ptr("success"),
							},
						},
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(runs)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "list_workflow_runs",
			"owner":       "owner",
			"repo":        "repo",
			"resource_id": "ci.yml",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response github.WorkflowRuns
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.TotalCount)
	})

	t.Run("list all workflow runs without resource_id", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposActionsRunsByOwnerByRepo,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					runs := &github.WorkflowRuns{
						TotalCount: github.Ptr(2),
						WorkflowRuns: []*github.WorkflowRun{
							{
								ID:         github.Ptr(int64(123)),
								Name:       github.Ptr("CI"),
								Status:     github.Ptr("completed"),
								Conclusion: github.Ptr("success"),
							},
							{
								ID:         github.Ptr(int64(456)),
								Name:       github.Ptr("Deploy"),
								Status:     github.Ptr("in_progress"),
								Conclusion: nil,
							},
						},
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(runs)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "list_workflow_runs",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response github.WorkflowRuns
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, 2, *response.TotalCount)
	})
}

func Test_ActionsGet(t *testing.T) {
	// Verify tool definition once
	toolDef := ActionsGet(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "actions_get", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "resource_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "repo", "resource_id"})
}

func Test_ActionsGet_GetWorkflow(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	t.Run("successful workflow get", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposActionsWorkflowsByOwnerByRepoByWorkflowId,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					workflow := &github.Workflow{
						ID:    github.Ptr(int64(1)),
						Name:  github.Ptr("CI"),
						Path:  github.Ptr(".github/workflows/ci.yml"),
						State: github.Ptr("active"),
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(workflow)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "get_workflow",
			"owner":       "owner",
			"repo":        "repo",
			"resource_id": "ci.yml",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response github.Workflow
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.ID)
		assert.Equal(t, "CI", *response.Name)
	})
}

func Test_ActionsGet_GetWorkflowRun(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	t.Run("successful workflow run get", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposActionsRunsByOwnerByRepoByRunId,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					run := &github.WorkflowRun{
						ID:         github.Ptr(int64(12345)),
						Name:       github.Ptr("CI"),
						Status:     github.Ptr("completed"),
						Conclusion: github.Ptr("success"),
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(run)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method":      "get_workflow_run",
			"owner":       "owner",
			"repo":        "repo",
			"resource_id": "12345",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response github.WorkflowRun
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.ID)
		assert.Equal(t, int64(12345), *response.ID)
	})
}

func Test_ActionsRunTrigger(t *testing.T) {
	// Verify tool definition once
	toolDef := ActionsRunTrigger(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "actions_run_trigger", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "method")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "workflow_id")
	assert.Contains(t, inputSchema.Properties, "ref")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "repo"})
}

func Test_ActionsRunTrigger_RunWorkflow(t *testing.T) {
	toolDef := ActionsRunTrigger(translations.NullTranslationHelper)

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
				"method":      "run_workflow",
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
				"method": "run_workflow",
				"owner":  "owner",
				"repo":   "repo",
				"ref":    "main",
			},
			expectError:    true,
			expectedErrMsg: "workflow_id is required for run_workflow action",
		},
		{
			name:         "missing required parameter ref",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method":      "run_workflow",
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
			},
			expectError:    true,
			expectedErrMsg: "ref is required for run_workflow action",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

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
			assert.Equal(t, "Workflow run has been queued", response["message"])
		})
	}
}

func Test_ActionsRunTrigger_CancelWorkflowRun(t *testing.T) {
	toolDef := ActionsRunTrigger(translations.NullTranslationHelper)

	t.Run("successful workflow run cancellation", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.EndpointPattern{
					Pattern: "/repos/owner/repo/actions/runs/12345/cancel",
					Method:  "POST",
				},
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusAccepted)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "cancel_workflow_run",
			"owner":  "owner",
			"repo":   "repo",
			"run_id": float64(12345),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "Workflow run has been cancelled", response["message"])
	})

	t.Run("conflict when cancelling a workflow run", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.EndpointPattern{
					Pattern: "/repos/owner/repo/actions/runs/12345/cancel",
					Method:  "POST",
				},
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusConflict)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "cancel_workflow_run",
			"owner":  "owner",
			"repo":   "repo",
			"run_id": float64(12345),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "failed to cancel workflow run")
	})

	t.Run("missing run_id for non-run_workflow methods", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient()

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client: client,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"method": "cancel_workflow_run",
			"owner":  "owner",
			"repo":   "repo",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.True(t, result.IsError)

		textContent := getTextResult(t, result)
		assert.Equal(t, "missing required parameter: run_id", textContent.Text)
	})
}

func Test_ActionsGetJobLogs(t *testing.T) {
	// Verify tool definition once
	toolDef := ActionsGetJobLogs(translations.NullTranslationHelper)

	// Note: consolidated ActionsGetJobLogs has same tool name "get_job_logs" as the individual tool
	// but with different descriptions. We skip toolsnap validation here since the individual
	// tool's toolsnap already exists and is tested in Test_GetJobLogs.
	// The consolidated tool has FeatureFlagEnable set, so only one will be active at a time.
	assert.Equal(t, "get_job_logs", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	inputSchema := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "job_id")
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.Contains(t, inputSchema.Properties, "failed_only")
	assert.Contains(t, inputSchema.Properties, "return_content")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo"})
}

func Test_ActionsGetJobLogs_SingleJob(t *testing.T) {
	toolDef := ActionsGetJobLogs(translations.NullTranslationHelper)

	t.Run("successful single job logs with URL", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
			mock.WithRequestMatchHandler(
				mock.GetReposActionsJobsLogsByOwnerByRepoByJobId,
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Location", "https://github.com/logs/job/123")
					w.WriteHeader(http.StatusFound)
				}),
			),
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client:            client,
			ContentWindowSize: 5000,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"job_id": float64(123),
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(123), response["job_id"])
		assert.Contains(t, response, "logs_url")
		assert.Equal(t, "Job logs are available for download", response["message"])
	})
}

func Test_ActionsGetJobLogs_FailedJobs(t *testing.T) {
	toolDef := ActionsGetJobLogs(translations.NullTranslationHelper)

	t.Run("successful failed jobs logs", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
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
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client:            client,
			ContentWindowSize: 5000,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":       "owner",
			"repo":        "repo",
			"run_id":      float64(456),
			"failed_only": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(456), response["run_id"])
		assert.Contains(t, response, "logs")
		assert.Contains(t, response["message"], "Retrieved logs for")
	})

	t.Run("no failed jobs found", func(t *testing.T) {
		mockedClient := mock.NewMockedHTTPClient(
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
		)

		client := github.NewClient(mockedClient)
		deps := BaseDeps{
			Client:            client,
			ContentWindowSize: 5000,
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(map[string]any{
			"owner":       "owner",
			"repo":        "repo",
			"run_id":      float64(456),
			"failed_only": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)

		require.NoError(t, err)
		require.False(t, result.IsError)

		textContent := getTextResult(t, result)
		var response map[string]any
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, "No failed jobs found in this workflow run", response["message"])
	})
}
