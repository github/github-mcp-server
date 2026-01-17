package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetRepositoryTree(t *testing.T) {
	// Verify tool definition once
	toolDef := GetRepositoryTree(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_repository_tree", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// Type assert the InputSchema to access its properties
	inputSchema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "expected InputSchema to be *jsonschema.Schema")
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "tree_sha")
	assert.Contains(t, inputSchema.Properties, "recursive")
	assert.Contains(t, inputSchema.Properties, "path_filter")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo"})

	// Setup mock data
	mockRepo := &github.Repository{
		DefaultBranch: github.Ptr("main"),
	}
	mockTree := &github.Tree{
		SHA:       github.Ptr("abc123"),
		Truncated: github.Ptr(false),
		Entries: []*github.TreeEntry{
			{
				Path: github.Ptr("README.md"),
				Mode: github.Ptr("100644"),
				Type: github.Ptr("blob"),
				SHA:  github.Ptr("file1sha"),
				Size: github.Ptr(123),
				URL:  github.Ptr("https://api.github.com/repos/owner/repo/git/blobs/file1sha"),
			},
			{
				Path: github.Ptr("src/main.go"),
				Mode: github.Ptr("100644"),
				Type: github.Ptr("blob"),
				SHA:  github.Ptr("file2sha"),
				Size: github.Ptr(456),
				URL:  github.Ptr("https://api.github.com/repos/owner/repo/git/blobs/file2sha"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successfully get repository tree",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo:               mockResponse(t, http.StatusOK, mockRepo),
				GetReposGitTreesByOwnerByRepoByTree: mockResponse(t, http.StatusOK, mockTree),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
		},
		{
			name: "successfully get repository tree with path filter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo:               mockResponse(t, http.StatusOK, mockRepo),
				GetReposGitTreesByOwnerByRepoByTree: mockResponse(t, http.StatusOK, mockTree),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"path_filter": "src/",
			},
		},
		{
			name: "repository not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get repository info",
		},
		{
			name: "tree not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, mockRepo),
				GetReposGitTreesByOwnerByRepoByTree: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to get repository tree",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create the tool request
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.False(t, result.IsError)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)

				// Parse the JSON response
				var treeResponse map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &treeResponse)
				require.NoError(t, err)

				// Verify response structure
				assert.Equal(t, "owner", treeResponse["owner"])
				assert.Equal(t, "repo", treeResponse["repo"])
				assert.Contains(t, treeResponse, "tree")
				assert.Contains(t, treeResponse, "count")
				assert.Contains(t, treeResponse, "sha")
				assert.Contains(t, treeResponse, "truncated")

				// Check filtering if path_filter was provided
				if pathFilter, exists := tc.requestArgs["path_filter"]; exists {
					tree := treeResponse["tree"].([]interface{})
					for _, entry := range tree {
						entryMap := entry.(map[string]interface{})
						path := entryMap["path"].(string)
						assert.True(t, strings.HasPrefix(path, pathFilter.(string)),
							"Path %s should start with filter %s", path, pathFilter)
					}
				}
			}
		})
	}
}
