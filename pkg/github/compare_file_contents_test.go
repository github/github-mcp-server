package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CompareFileContents(t *testing.T) {
	serverTool := CompareFileContents(translations.NullTranslationHelper)
	tool := serverTool.Tool

	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "compare_file_contents", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "compare_file_contents should be read-only")
	assert.Equal(t, FeatureFlagCompareFileContents, serverTool.FeatureFlagEnable)

	// Helper to create a mock handler that returns file content for a specific ref
	mockContentsForRef := func(contentsByRef map[string]string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ref := r.URL.Query().Get("ref")
			content, ok := contentsByRef[ref]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				return
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(content))
			fileContent := &github.RepositoryContent{
				Name:     github.Ptr("config.json"),
				Path:     github.Ptr("config.json"),
				SHA:      github.Ptr("abc123"),
				Type:     github.Ptr("file"),
				Encoding: github.Ptr("base64"),
				Content:  github.Ptr(encoded),
			}
			w.WriteHeader(http.StatusOK)
			data, _ := json.Marshal(fileContent)
			_, _ = w.Write(data)
		}
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		expectFormat   string
		expectDiff     string
	}{
		{
			name: "JSON semantic diff - value change",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main":    `{"theme": "light", "version": "1.0"}`,
					"feature": `{"theme": "dark", "version": "1.0"}`,
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "json",
			expectDiff:   `theme: "light" → "dark"`,
		},
		{
			name: "JSON semantic diff - no changes after reformatting",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main":    `{"key":"value","num":42}`,
					"feature": "{\n  \"key\": \"value\",\n  \"num\": 42\n}",
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "json",
			expectDiff:   "no changes detected",
		},
		{
			name: "YAML semantic diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"v1": "host: localhost\nport: 5432\n",
					"v2": "host: production.db\nport: 5432\n",
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.yaml",
				"base":  "v1",
				"head":  "v2",
			},
			expectFormat: "yaml",
			expectDiff:   `host: "localhost" → "production.db"`,
		},
		{
			name: "unsupported format falls back to unified diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main":    "func main() {}\n",
					"feature": "func main() {\n\tfmt.Println(\"hello\")\n}\n",
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "main.go",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "unified",
			expectDiff:   "--- a/main.go",
		},
		{
			name:         "missing required parameter - owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"repo": "repo",
				"path": "config.json",
				"base": "main",
				"head": "feature",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter - base",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"head":  "feature",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: base",
		},
		{
			name: "new file - base not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"feature": `{"key": "value"}`,
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "json",
			expectDiff:   "file added",
		},
		{
			name: "deleted file - head not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main": `{"key": "value"}`,
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "json",
			expectDiff:   "file deleted",
		},
		{
			name: "both refs not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "main",
				"head":  "feature",
			},
			expectError:    true,
			expectedErrMsg: "failed to get file at both refs",
		},
		{
			name: "CSV semantic diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main":    "name,status\nAlice,active\nBob,pending\n",
					"feature": "name,status\nAlice,active\nBob,shipped\n",
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "data.csv",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "csv",
			expectDiff:   `row 2.status: "pending" → "shipped"`,
		},
		{
			name: "TOML semantic diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: mockContentsForRef(map[string]string{
					"main":    "[database]\nhost = \"localhost\"\n",
					"feature": "[database]\nhost = \"production.db\"\n",
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.toml",
				"base":  "main",
				"head":  "feature",
			},
			expectFormat: "toml",
			expectDiff:   `database.host: "localhost" → "production.db"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var diffResult SemanticDiffResult
			err = json.Unmarshal([]byte(textContent.Text), &diffResult)
			require.NoError(t, err)

			assert.Equal(t, DiffFormat(tc.expectFormat), diffResult.Format)
			assert.Contains(t, diffResult.Diff, tc.expectDiff)
		})
	}
}
