package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposActionsWorkflowsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					workflows := &github.Workflows{
						TotalCount: new(2),
						Workflows: []*github.Workflow{
							{
								ID:    new(int64(1)),
								Name:  new("CI"),
								Path:  new(".github/workflows/ci.yml"),
								State: new("active"),
							},
							{
								ID:    new(int64(2)),
								Name:  new("Deploy"),
								Path:  new(".github/workflows/deploy.yml"),
								State: new("active"),
							},
						},
					}
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(workflows)
				}),
			}),
			requestArgs: map[string]any{
				"method": "list_workflows",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError: false,
		},
		{
			name:         "missing required parameter method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			client := mustNewGHClient(t, tc.mockedClient)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				runs := &github.WorkflowRuns{
					TotalCount: new(1),
					WorkflowRuns: []*github.WorkflowRun{
						{
							ID:         new(int64(123)),
							Name:       new("CI"),
							Status:     new("completed"),
							Conclusion: new("success"),
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(runs)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		var response MinimalWorkflowRunsResult
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, 1, response.TotalCount)
		require.Len(t, response.WorkflowRuns, 1)
		assert.Equal(t, int64(123), response.WorkflowRuns[0].ID)
	})

	t.Run("list all workflow runs without resource_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				runs := &github.WorkflowRuns{
					TotalCount: new(2),
					WorkflowRuns: []*github.WorkflowRun{
						{
							ID:         new(int64(123)),
							Name:       new("CI"),
							Status:     new("completed"),
							Conclusion: new("success"),
						},
						{
							ID:         new(int64(456)),
							Name:       new("Deploy"),
							Status:     new("in_progress"),
							Conclusion: nil,
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(runs)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		var response MinimalWorkflowRunsResult
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.Equal(t, 2, response.TotalCount)
		assert.Len(t, response.WorkflowRuns, 2)
	})
}

func Test_ActionsList_ListWorkflowJobs(t *testing.T) {
	toolDef := ActionsList(translations.NullTranslationHelper)
	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsRunsJobsByOwnerByRepoByRunID: mockResponse(t, http.StatusOK, &github.Jobs{
			TotalCount: new(1),
			Jobs:       []*github.WorkflowJob{actionsTestWorkflowJob()},
		}),
	})

	client := mustNewGHClient(t, mockedClient)
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
		"method":      "list_workflow_jobs",
		"owner":       "owner",
		"repo":        "repo",
		"resource_id": "30433642",
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var response struct {
		Jobs MinimalWorkflowJobsResult `json:"jobs"`
	}
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
	assert.Equal(t, 1, response.Jobs.TotalCount)
	require.Len(t, response.Jobs.Jobs, 1)
	assert.Equal(t, int64(399444496), response.Jobs.Jobs[0].ID)
	assert.Len(t, response.Jobs.Jobs[0].Steps, 2)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsWorkflowsByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				workflow := &github.Workflow{
					ID:    new(int64(1)),
					Name:  new("CI"),
					Path:  new(".github/workflows/ci.yml"),
					State: new("active"),
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(workflow)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		run := actionsTestWorkflowRun()
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(run)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		var response MinimalWorkflowRun
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)

		expected := convertToMinimalWorkflowRun(run)
		assert.Equal(t, expected, response)

		var payload map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &payload))
		assert.Equal(t, marshalActionsObject(t, expected), payload)
		assert.NotContains(t, payload, "node_id")
		assert.NotContains(t, payload, "repository")
		assert.NotContains(t, payload, "head_repository")
		assert.NotContains(t, payload, "url")
		assert.NotContains(t, payload, "jobs_url")
	})
}

// workflowRunSequence serves the given status/conclusion pairs in order, repeating the last one.
// It returns the handler and a function that reports how many times the run was fetched.
func workflowRunSequence(states ...[2]string) (http.HandlerFunc, func() int) {
	var calls atomic.Int32
	handler := func(w http.ResponseWriter, _ *http.Request) {
		i := int(calls.Add(1)) - 1
		state := states[min(i, len(states)-1)]
		run := actionsTestWorkflowRun()
		run.Status = new(state[0])
		run.Conclusion = new(state[1])
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(run)
	}
	return handler, func() int { return int(calls.Load()) }
}

func watchedRun(status, conclusion string) MinimalWorkflowRun {
	run := actionsTestWorkflowRun()
	run.Status = new(status)
	run.Conclusion = new(conclusion)
	return convertToMinimalWorkflowRun(run)
}

func Test_ActionsGet_WatchWorkflowRun(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	failedRunJobs := &github.Jobs{
		TotalCount: new(3),
		Jobs: []*github.WorkflowJob{
			{ID: new(int64(1)), Name: new("lint"), Status: new("completed"), Conclusion: new("success")},
			{ID: new(int64(2)), Name: new("test"), Status: new("completed"), Conclusion: new("failure"), HTMLURL: new("https://github.com/octo-org/octo-repo/actions/runs/30433642/job/2")},
			{ID: new(int64(3)), Name: new("deploy"), Status: new("completed"), Conclusion: new("cancelled")},
		},
	}

	tests := []struct {
		name          string
		states        [][2]string
		jobs          *github.Jobs
		timeout       int
		expectedCalls int
		expected      MinimalWorkflowRunWatchResult
	}{
		{
			name:          "returns at once when the run already completed",
			states:        [][2]string{{"completed", "success"}},
			expectedCalls: 1,
			expected: MinimalWorkflowRunWatchResult{
				WorkflowRun: watchedRun("completed", "success"),
				Completed:   true,
			},
		},
		{
			name:          "waits until the run completes",
			states:        [][2]string{{"queued", ""}, {"in_progress", ""}, {"completed", "success"}},
			expectedCalls: 3,
			expected: MinimalWorkflowRunWatchResult{
				WorkflowRun: watchedRun("completed", "success"),
				Completed:   true,
			},
		},
		{
			name:          "lists the jobs that did not succeed when the run fails",
			states:        [][2]string{{"in_progress", ""}, {"completed", "failure"}},
			jobs:          failedRunJobs,
			expectedCalls: 2,
			expected: MinimalWorkflowRunWatchResult{
				WorkflowRun: watchedRun("completed", "failure"),
				Completed:   true,
				FailedJobs: []MinimalFailedWorkflowJob{
					{ID: 2, Name: "test", Conclusion: "failure", HTMLURL: "https://github.com/octo-org/octo-repo/actions/runs/30433642/job/2"},
					{ID: 3, Name: "deploy", Conclusion: "cancelled"},
				},
				NextStep: "Use get_job_logs with run_id 30433642 and failed_only true to read the logs of the failed jobs.",
			},
		},
		{
			name:    "returns the current status when the timeout elapses",
			states:  [][2]string{{"in_progress", ""}},
			timeout: 1,
			expected: MinimalWorkflowRunWatchResult{
				WorkflowRun:   watchedRun("in_progress", ""),
				Completed:     false,
				WaitedSeconds: 1,
				NextStep:      "The workflow run is still in_progress. Call watch_workflow_run again to keep waiting.",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runHandler, calls := workflowRunSequence(tc.states...)
			handlers := map[string]http.HandlerFunc{
				GetReposActionsRunsByOwnerByRepoByRunID: runHandler,
			}
			if tc.jobs != nil {
				handlers[GetReposActionsRunsJobsByOwnerByRepoByRunID] = mockResponse(t, http.StatusOK, tc.jobs)
			}
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(handlers))}
			handler := toolDef.Handler(deps)

			args := map[string]any{
				"method":      "watch_workflow_run",
				"owner":       "owner",
				"repo":        "repo",
				"resource_id": "30433642",
			}
			if tc.timeout > 0 {
				args["timeout_seconds"] = tc.timeout
			}
			request := createMCPRequest(args)
			ctx := ContextWithWatchConfig(ContextWithDeps(context.Background(), deps), WatchConfig{PollInterval: 10 * time.Millisecond})
			result, err := handler(ctx, &request)
			require.NoError(t, err)
			require.False(t, result.IsError, getTextResult(t, result).Text)

			var response MinimalWorkflowRunWatchResult
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
			assert.Equal(t, tc.expected, response)
			if tc.expectedCalls > 0 {
				assert.Equal(t, tc.expectedCalls, calls())
			}
		})
	}
}

func Test_ActionsGet_WatchWorkflowRun_Errors(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	tests := []struct {
		name           string
		handler        http.HandlerFunc
		args           map[string]any
		expectedErrMsg string
	}{
		{
			name:           "timeout above the maximum",
			args:           map[string]any{"timeout_seconds": 601},
			expectedErrMsg: "timeout_seconds must be between 1 and 600",
		},
		{
			name:           "negative timeout",
			args:           map[string]any{"timeout_seconds": -5},
			expectedErrMsg: "timeout_seconds must be between 1 and 600",
		},
		{
			name:           "run ID is not a number",
			args:           map[string]any{"resource_id": "ci.yml"},
			expectedErrMsg: "invalid resource_id, must be an integer for method watch_workflow_run",
		},
		{
			name:           "run not found",
			handler:        mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}),
			expectedErrMsg: "failed to get workflow run",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handlers := map[string]http.HandlerFunc{}
			if tc.handler != nil {
				handlers[GetReposActionsRunsByOwnerByRepoByRunID] = tc.handler
			}
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(handlers))}
			handler := toolDef.Handler(deps)

			args := map[string]any{
				"method":      "watch_workflow_run",
				"owner":       "owner",
				"repo":        "repo",
				"resource_id": "30433642",
			}
			maps.Copy(args, tc.args)
			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
		})
	}
}

func Test_ActionsGet_WatchWorkflowRun_StopsWhenCancelled(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runHandler, calls := workflowRunSequence([2]string{"in_progress", ""})
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsRunsByOwnerByRepoByRunID: func(w http.ResponseWriter, r *http.Request) {
			runHandler(w, r)
			cancel()
		},
	}))}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":          "watch_workflow_run",
		"owner":           "owner",
		"repo":            "repo",
		"resource_id":     "30433642",
		"timeout_seconds": 600,
	})
	ctx = ContextWithWatchConfig(ContextWithDeps(ctx, deps), WatchConfig{PollInterval: time.Hour})
	_, err := handler(ctx, &request)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, calls())
}

func Test_ActionsGet_WatchWorkflowRun_SendsProgress(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)
	runHandler, _ := workflowRunSequence([2]string{"queued", ""}, [2]string{"in_progress", ""}, [2]string{"completed", "success"})
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsRunsByOwnerByRepoByRunID: runHandler,
	}))}
	handler := toolDef.Handler(deps)

	var mu sync.Mutex
	var messages []string
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, req.Params.Message)
		},
	})
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })

	argsJSON, err := json.Marshal(map[string]any{
		"method":      "watch_workflow_run",
		"owner":       "owner",
		"repo":        "repo",
		"resource_id": "30433642",
	})
	require.NoError(t, err)
	request := mcp.CallToolRequest{
		Session: serverSession,
		Params: &mcp.CallToolParamsRaw{
			Meta:      mcp.Meta{"progressToken": "watch-1"},
			Arguments: argsJSON,
		},
	}
	ctx := ContextWithWatchConfig(ContextWithDeps(context.Background(), deps), WatchConfig{PollInterval: 10 * time.Millisecond})
	result, err := handler(ctx, &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	expected := []string{
		"Workflow run 30433642 is queued (waited 0s)",
		"Workflow run 30433642 is in_progress (waited 0s)",
	}
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		mu.Lock()
		defer mu.Unlock()
		assert.Equal(c, expected, messages)
	}, time.Second, 10*time.Millisecond)
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":      "run_workflow",
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
			},
			expectError:    true,
			expectedErrMsg: "ref is required for run_workflow action",
		},
		{
			name: "successful workflow run with inputs",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				}),
			}),
			requestArgs: map[string]any{
				"method":      "run_workflow",
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
				"ref":         "main",
				"inputs":      map[string]any{"FIELD1": "value1", "FIELD2": "value2"},
			},
			expectError: false,
		},
		{
			name:         "invalid inputs type returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":      "run_workflow",
				"owner":       "owner",
				"repo":        "repo",
				"workflow_id": "12345",
				"ref":         "main",
				"inputs":      "not a map",
			},
			expectError:    true,
			expectedErrMsg: "parameter inputs is not of type map[string]interface {}, is string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposActionsRunsCancelByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposActionsRunsCancelByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusConflict)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{})

		client := mustNewGHClient(t, mockedClient)
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
	// The functional feature rules ensure only one variant is active at a time.
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsJobsLogsByOwnerByRepoByJobID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Location", "https://github.com/logs/job/123")
				w.WriteHeader(http.StatusFound)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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

func TestGetJobLogData_DownloadTransportErrorReturnsNilResponse(t *testing.T) {
	logServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	logURL := logServer.URL
	logServer.Close()

	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsJobsLogsByOwnerByRepoByJobID: func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", logURL)
			w.WriteHeader(http.StatusFound)
		},
	}))

	_, resp, err := getJobLogData(t.Context(), client, "owner", "repo", 123, "", true, 100, 5000)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to download log content for job 123")
}

func Test_ActionsGetJobLogs_FailedJobs(t *testing.T) {
	toolDef := ActionsGetJobLogs(translations.NullTranslationHelper)

	t.Run("successful failed jobs logs", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsJobsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				jobs := &github.Jobs{
					TotalCount: new(3),
					Jobs: []*github.WorkflowJob{
						{
							ID:         new(int64(1)),
							Name:       new("test-job-1"),
							Conclusion: new("success"),
						},
						{
							ID:         new(int64(2)),
							Name:       new("test-job-2"),
							Conclusion: new("failure"),
						},
						{
							ID:         new(int64(3)),
							Name:       new("test-job-3"),
							Conclusion: new("failure"),
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
		})

		client := mustNewGHClient(t, mockedClient)
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsJobsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				jobs := &github.Jobs{
					TotalCount: new(2),
					Jobs: []*github.WorkflowJob{
						{
							ID:         new(int64(1)),
							Name:       new("test-job-1"),
							Conclusion: new("success"),
						},
						{
							ID:         new(int64(2)),
							Name:       new("test-job-2"),
							Conclusion: new("success"),
						},
					},
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(jobs)
			}),
		})

		client := mustNewGHClient(t, mockedClient)
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
