package github

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
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
		var response github.WorkflowRuns
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.TotalCount)
	})

	t.Run("list all workflow runs without resource_id", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	assert.Contains(t, inputSchema.Properties, "run_id")
	assert.Contains(t, inputSchema.Properties, "artifact_name")
	assert.Contains(t, inputSchema.Properties, "path")
	assert.Contains(t, inputSchema.Properties, "max_bytes")
	assert.ElementsMatch(t, inputSchema.Required, []string{"method", "owner", "repo"})
}

func Test_ActionsGet_GetWorkflow(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	t.Run("successful workflow get", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsWorkflowsByOwnerByRepoByWorkflowID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				workflow := &github.Workflow{
					ID:    github.Ptr(int64(1)),
					Name:  github.Ptr("CI"),
					Path:  github.Ptr(".github/workflows/ci.yml"),
					State: github.Ptr("active"),
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
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposActionsRunsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				run := &github.WorkflowRun{
					ID:         github.Ptr(int64(12345)),
					Name:       github.Ptr("CI"),
					Status:     github.Ptr("completed"),
					Conclusion: github.Ptr("success"),
				}
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
		var response github.WorkflowRun
		err = json.Unmarshal([]byte(textContent.Text), &response)
		require.NoError(t, err)
		assert.NotNil(t, response.ID)
		assert.Equal(t, int64(12345), *response.ID)
	})
}

func TestActionsGet_DownloadWorkflowArtifact_LegacyURL(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	mockedClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/artifacts/456/zip" {
			w.Header().Set("Location", "https://example.com/artifacts/456.zip")
			w.WriteHeader(http.StatusFound)
			return
		}

		http.NotFound(w, r)
	})

	client := mustNewGHClient(t, mockedClient)
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":      "download_workflow_run_artifact",
		"owner":       "owner",
		"repo":        "repo",
		"resource_id": "456",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response map[string]any
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/artifacts/456.zip", response["download_url"])
	assert.Equal(t, float64(456), response["artifact_id"])
}

func TestActionsGet_DownloadWorkflowArtifact_ByRunAndName(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)
	archiveBytes := mustCreateArtifactZip(t, map[string][]byte{
		"usage.jsonl": []byte("{\"count\":123}\n"),
		"trace.bin":   {0x00, 0x01, 0x02},
	})
	archiveServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archiveBytes)
	}))
	defer archiveServer.Close()

	mockedClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/runs/123/artifacts":
			artifacts := &github.ArtifactList{
				TotalCount: github.Ptr(int64(2)),
				Artifacts: []*github.Artifact{
					{
						ID:          github.Ptr(int64(456)),
						Name:        github.Ptr("agent-artifacts"),
						Expired:     github.Ptr(false),
						SizeInBytes: github.Ptr(int64(len(archiveBytes))),
					},
					{
						ID:   github.Ptr(int64(789)),
						Name: github.Ptr("other-artifact"),
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(artifacts)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/artifacts/456/zip":
			w.Header().Set("Location", archiveServer.URL+"/download.zip")
			w.WriteHeader(http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})

	client := mustNewGHClient(t, mockedClient)
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":        "download_workflow_run_artifact",
		"owner":         "owner",
		"repo":          "repo",
		"run_id":        float64(123),
		"artifact_name": "agent-artifacts",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response struct {
		ArtifactID   int64  `json:"artifact_id"`
		ArtifactName string `json:"artifact_name"`
		Expired      bool   `json:"expired"`
		SizeInBytes  int64  `json:"size_in_bytes"`
		MaxBytes     int    `json:"max_bytes"`
		Files        []struct {
			Path                 string `json:"path"`
			Size                 int64  `json:"size"`
			Truncated            bool   `json:"truncated"`
			Binary               bool   `json:"binary"`
			Content              string `json:"content"`
			ContentOmittedReason string `json:"content_omitted_reason"`
		} `json:"files"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)
	assert.Equal(t, int64(456), response.ArtifactID)
	assert.Equal(t, "agent-artifacts", response.ArtifactName)
	assert.False(t, response.Expired)
	assert.Len(t, response.Files, 2)

	filesByPath := map[string]struct {
		Path                 string `json:"path"`
		Size                 int64  `json:"size"`
		Truncated            bool   `json:"truncated"`
		Binary               bool   `json:"binary"`
		Content              string `json:"content"`
		ContentOmittedReason string `json:"content_omitted_reason"`
	}{}
	for _, file := range response.Files {
		filesByPath[file.Path] = file
	}

	usageFile := filesByPath["usage.jsonl"]
	assert.Equal(t, "{\"count\":123}\n", usageFile.Content)
	assert.False(t, usageFile.Truncated)

	binaryFile := filesByPath["trace.bin"]
	assert.True(t, binaryFile.Binary)
	assert.Empty(t, binaryFile.Content)
}

func TestActionsGet_DownloadWorkflowArtifact_PathFilteringAndTruncation(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)
	archiveBytes := mustCreateArtifactZip(t, map[string][]byte{
		"nested/result.txt": []byte("abcdefghij"),
		"other.txt":         []byte("unused"),
	})
	archiveServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(archiveBytes)
	}))
	defer archiveServer.Close()

	mockedClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/runs/123/artifacts":
			artifacts := &github.ArtifactList{
				TotalCount: github.Ptr(int64(1)),
				Artifacts: []*github.Artifact{
					{
						ID:          github.Ptr(int64(456)),
						Name:        github.Ptr("agent-artifacts"),
						Expired:     github.Ptr(false),
						SizeInBytes: github.Ptr(int64(len(archiveBytes))),
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(artifacts)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/artifacts/456/zip":
			w.Header().Set("Location", archiveServer.URL+"/download.zip")
			w.WriteHeader(http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})

	client := mustNewGHClient(t, mockedClient)
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":        "download_workflow_run_artifact",
		"owner":         "owner",
		"repo":          "repo",
		"run_id":        float64(123),
		"artifact_name": "agent-artifacts",
		"path":          "nested/result.txt",
		"max_bytes":     float64(4),
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response struct {
		Files []struct {
			Path                 string `json:"path"`
			Truncated            bool   `json:"truncated"`
			Content              string `json:"content"`
			ContentOmittedReason string `json:"content_omitted_reason"`
		} `json:"files"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)
	require.Len(t, response.Files, 1)
	assert.Equal(t, "nested/result.txt", response.Files[0].Path)
	assert.Equal(t, "abcd", response.Files[0].Content)
	assert.True(t, response.Files[0].Truncated)
	assert.Contains(t, response.Files[0].ContentOmittedReason, "truncated")
}

func TestActionsGet_DownloadWorkflowArtifact_MissingArtifact(t *testing.T) {
	toolDef := ActionsGet(translations.NullTranslationHelper)

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposActionsRunsArtifactsByOwnerByRepoByRunID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			artifacts := &github.ArtifactList{
				TotalCount: github.Ptr(int64(1)),
				Artifacts: []*github.Artifact{
					{
						ID:   github.Ptr(int64(456)),
						Name: github.Ptr("other-artifact"),
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(artifacts)
		}),
	})

	client := mustNewGHClient(t, mockedClient)
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":        "download_workflow_run_artifact",
		"owner":         "owner",
		"repo":          "repo",
		"run_id":        float64(123),
		"artifact_name": "agent-artifacts",
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getTextResult(t, result).Text, "artifact \"agent-artifacts\" was not found")
}

func TestDownloadArtifactArchiveRejectsOversizedArchive(t *testing.T) {
	archiveServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("12345"))
	}))
	defer archiveServer.Close()

	archiveURL, err := url.Parse(archiveServer.URL + "/download.zip")
	require.NoError(t, err)

	_, resp, err := downloadArtifactArchive(context.Background(), archiveURL, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "artifact archive exceeds maximum supported size")
	require.NotNil(t, resp)
	require.NoError(t, resp.Body.Close())
}

func TestReadWorkflowArtifactFileTruncatesAtUTF8Boundary(t *testing.T) {
	archiveBytes := mustCreateArtifactZip(t, map[string][]byte{
		"unicode.txt": []byte("abé"),
	})
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	require.NoError(t, err)
	require.Len(t, reader.File, 1)

	result, err := readWorkflowArtifactFile(reader.File[0], 2)
	require.NoError(t, err)
	assert.Equal(t, "ab", result.Content)
	assert.True(t, result.Truncated)
}

func mustCreateArtifactZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for path, content := range files {
		entry, err := writer.Create(path)
		require.NoError(t, err)
		_, err = entry.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	return archive.Bytes()
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

func Test_ActionsGetJobLogs_FailedJobs(t *testing.T) {
	toolDef := ActionsGetJobLogs(translations.NullTranslationHelper)

	t.Run("successful failed jobs logs", func(t *testing.T) {
		mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
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
