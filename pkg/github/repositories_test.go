package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetFileContents(t *testing.T) {
	// Verify tool definition once
	serverTool := GetFileContents(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_file_contents", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "path")
	assert.Contains(t, schema.Properties, "ref")
	assert.Contains(t, schema.Properties, "sha")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Mock response for raw content
	mockRawContent := []byte("# Test Repository\n\nThis is a test repository.")

	// Setup mock directory content for success case
	mockDirContent := []*github.RepositoryContent{
		{
			Type:    github.Ptr("file"),
			Name:    github.Ptr("README.md"),
			Path:    github.Ptr("README.md"),
			SHA:     github.Ptr("abc123"),
			Size:    github.Ptr(42),
			HTMLURL: github.Ptr("https://github.com/owner/repo/blob/main/README.md"),
		},
		{
			Type:    github.Ptr("dir"),
			Name:    github.Ptr("src"),
			Path:    github.Ptr("src"),
			SHA:     github.Ptr("def456"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/tree/main/src"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult interface{}
		expectedErrMsg string
		expectStatus   int
		expectedMsg    string // optional: expected message text to verify in result
	}{
		{
			name: "successful text content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						fileContent := &github.RepositoryContent{
							Name: github.Ptr("README.md"),
							Path: github.Ptr("README.md"),
							SHA:  github.Ptr("abc123"),
							Type: github.Ptr("file"),
						}
						contentBytes, _ := json.Marshal(fileContent)
						_, _ = w.Write(contentBytes)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, _ = w.Write(mockRawContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "README.md",
				"ref":   "refs/heads/main",
			},
			expectError: false,
			expectedResult: mcp.ResourceContents{
				URI:      "repo://owner/repo/refs/heads/main/contents/README.md",
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
			},
		},
		{
			name: "successful file blob content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						fileContent := &github.RepositoryContent{
							Name: github.Ptr("test.png"),
							Path: github.Ptr("test.png"),
							SHA:  github.Ptr("def456"),
							Type: github.Ptr("file"),
						}
						contentBytes, _ := json.Marshal(fileContent)
						_, _ = w.Write(contentBytes)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "image/png")
						_, _ = w.Write(mockRawContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "test.png",
				"ref":   "refs/heads/main",
			},
			expectError: false,
			expectedResult: mcp.ResourceContents{
				URI:      "repo://owner/repo/refs/heads/main/contents/test.png",
				Blob:     mockRawContent,
				MIMEType: "image/png",
			},
		},
		{
			name: "successful PDF file content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						fileContent := &github.RepositoryContent{
							Name: github.Ptr("document.pdf"),
							Path: github.Ptr("document.pdf"),
							SHA:  github.Ptr("pdf123"),
							Type: github.Ptr("file"),
						}
						contentBytes, _ := json.Marshal(fileContent)
						_, _ = w.Write(contentBytes)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/pdf")
						_, _ = w.Write(mockRawContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "document.pdf",
				"ref":   "refs/heads/main",
			},
			expectError: false,
			expectedResult: mcp.ResourceContents{
				URI:      "repo://owner/repo/refs/heads/main/contents/document.pdf",
				Blob:     mockRawContent,
				MIMEType: "application/pdf",
			},
		},
		{
			name: "successful directory content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"name": "repo", "default_branch": "main"}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					expectQueryParams(t, map[string]string{}).andThen(
						mockResponse(t, http.StatusOK, mockDirContent),
					),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					expectQueryParams(t, map[string]string{
						"branch": "main",
					}).andThen(
						mockResponse(t, http.StatusNotFound, nil),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "src/",
			},
			expectError:    false,
			expectedResult: mockDirContent,
		},
		{
			name: "successful text content fetch with leading slash in path",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						fileContent := &github.RepositoryContent{
							Name: github.Ptr("README.md"),
							Path: github.Ptr("README.md"),
							SHA:  github.Ptr("abc123"),
							Type: github.Ptr("file"),
						}
						contentBytes, _ := json.Marshal(fileContent)
						_, _ = w.Write(contentBytes)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, _ = w.Write(mockRawContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "/README.md",
				"ref":   "refs/heads/main",
			},
			expectError: false,
			expectedResult: mcp.ResourceContents{
				URI:      "repo://owner/repo/refs/heads/main/contents/README.md",
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
			},
		},
		{
			name: "successful text content fetch with note when ref falls back to default branch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"name": "repo", "default_branch": "develop"}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Request for "refs/heads/main" -> 404 (doesn't exist)
						// Request for "refs/heads/develop" (default branch) -> 200
						switch {
						case strings.Contains(r.URL.Path, "heads/main"):
							w.WriteHeader(http.StatusNotFound)
							_, _ = w.Write([]byte(`{"message": "Not Found"}`))
						case strings.Contains(r.URL.Path, "heads/develop"):
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/heads/develop", "object": {"sha": "abc123def456"}}`))
						default:
							w.WriteHeader(http.StatusNotFound)
							_, _ = w.Write([]byte(`{"message": "Not Found"}`))
						}
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						fileContent := &github.RepositoryContent{
							Name: github.Ptr("README.md"),
							Path: github.Ptr("README.md"),
							SHA:  github.Ptr("abc123"),
							Type: github.Ptr("file"),
						}
						contentBytes, _ := json.Marshal(fileContent)
						_, _ = w.Write(contentBytes)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoBySHAByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, _ = w.Write(mockRawContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "README.md",
				"ref":   "main",
			},
			expectError: false,
			expectedResult: mcp.ResourceContents{
				URI:      "repo://owner/repo/abc123def456/contents/README.md",
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
			},
			expectedMsg: " Note: the provided ref 'main' does not exist, default branch 'refs/heads/develop' was used instead.",
		},
		{
			name: "content fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": ""}}`))
					}),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "nonexistent.md",
				"ref":   "refs/heads/main",
			},
			expectError:    false,
			expectedResult: utils.NewToolResultError("Failed to get file contents. The path does not point to a file or directory, or the file does not exist in the repository."),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			mockRawClient := raw.NewClient(client, &url.URL{Scheme: "https", Host: "raw.example.com", Path: "/"})
			deps := BaseDeps{
				Client:    client,
				RawClient: mockRawClient,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			// Use the correct result helper based on the expected type
			switch expected := tc.expectedResult.(type) {
			case mcp.ResourceContents:
				// Handle both text and blob resources
				resource := getResourceResult(t, result)
				assert.Equal(t, expected, *resource)

				// If expectedMsg is set, verify the message text
				if tc.expectedMsg != "" {
					require.Len(t, result.Content, 2)
					textContent, ok := result.Content[0].(*mcp.TextContent)
					require.True(t, ok, "expected Content[0] to be TextContent")
					assert.Contains(t, textContent.Text, tc.expectedMsg)
				}
			case []*github.RepositoryContent:
				// Directory content fetch returns a text result (JSON array)
				textContent := getTextResult(t, result)
				var returnedContents []*github.RepositoryContent
				err = json.Unmarshal([]byte(textContent.Text), &returnedContents)
				require.NoError(t, err, "Failed to unmarshal directory content result: %v", textContent.Text)
				assert.Len(t, returnedContents, len(expected))
				for i, content := range returnedContents {
					assert.Equal(t, *expected[i].Name, *content.Name)
					assert.Equal(t, *expected[i].Path, *content.Path)
					assert.Equal(t, *expected[i].Type, *content.Type)
				}
			case mcp.TextContent:
				textContent := getErrorResult(t, result)
				require.Equal(t, textContent, expected)
			}
		})
	}
}

func Test_ForkRepository(t *testing.T) {
	// Verify tool definition once
	serverTool := ForkRepository(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "fork_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "organization")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock forked repo for success case
	mockForkedRepo := &github.Repository{
		ID:       github.Ptr(int64(123456)),
		Name:     github.Ptr("repo"),
		FullName: github.Ptr("new-owner/repo"),
		Owner: &github.User{
			Login: github.Ptr("new-owner"),
		},
		HTMLURL:       github.Ptr("https://github.com/new-owner/repo"),
		DefaultBranch: github.Ptr("main"),
		Fork:          github.Ptr(true),
		ForksCount:    github.Ptr(0),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedRepo   *github.Repository
		expectedErrMsg string
	}{
		{
			name: "successful repository fork",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposForksByOwnerByRepo,
					mockResponse(t, http.StatusAccepted, mockForkedRepo),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:  false,
			expectedRepo: mockForkedRepo,
		},
		{
			name: "repository fork fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposForksByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to fork repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			assert.Contains(t, textContent.Text, "Fork is in progress")
		})
	}
}

func Test_CreateBranch(t *testing.T) {
	// Verify tool definition once
	serverTool := CreateBranch(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "create_branch", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "branch")
	assert.Contains(t, schema.Properties, "from_branch")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "branch"})

	// Setup mock repository for default branch test
	mockRepo := &github.Repository{
		DefaultBranch: github.Ptr("main"),
	}

	// Setup mock reference for from_branch tests
	mockSourceRef := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("abc123def456"),
		},
	}

	// Setup mock created reference
	mockCreatedRef := &github.Reference{
		Ref: github.Ptr("refs/heads/new-feature"),
		Object: &github.GitObject{
			SHA: github.Ptr("abc123def456"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedRef    *github.Reference
		expectedErrMsg string
	}{
		{
			name: "successful branch creation with from_branch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockSourceRef,
				),
				mock.WithRequestMatch(
					mock.PostReposGitRefsByOwnerByRepo,
					mockCreatedRef,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"branch":      "new-feature",
				"from_branch": "main",
			},
			expectError: false,
			expectedRef: mockCreatedRef,
		},
		{
			name: "successful branch creation with default branch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					mockRepo,
				),
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockSourceRef,
				),
				mock.WithRequestMatchHandler(
					mock.PostReposGitRefsByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"ref": "refs/heads/new-feature",
						"sha": "abc123def456",
					}).andThen(
						mockResponse(t, http.StatusCreated, mockCreatedRef),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "new-feature",
			},
			expectError: false,
			expectedRef: mockCreatedRef,
		},
		{
			name: "fail to get repository",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Repository not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "nonexistent-repo",
				"branch": "new-feature",
			},
			expectError:    true,
			expectedErrMsg: "failed to get repository",
		},
		{
			name: "fail to get reference",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Reference not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"branch":      "new-feature",
				"from_branch": "nonexistent-branch",
			},
			expectError:    true,
			expectedErrMsg: "failed to get reference",
		},
		{
			name: "fail to create branch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockSourceRef,
				),
				mock.WithRequestMatchHandler(
					mock.PostReposGitRefsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Reference already exists"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"branch":      "existing-branch",
				"from_branch": "main",
			},
			expectError:    true,
			expectedErrMsg: "failed to create branch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedRef github.Reference
			err = json.Unmarshal([]byte(textContent.Text), &returnedRef)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedRef.Ref, *returnedRef.Ref)
			assert.Equal(t, *tc.expectedRef.Object.SHA, *returnedRef.Object.SHA)
		})
	}
}

func Test_GetCommit(t *testing.T) {
	// Verify tool definition once
	serverTool := GetCommit(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_commit", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "sha")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "sha"})

	mockCommit := &github.RepositoryCommit{
		SHA: github.Ptr("abc123def456"),
		Commit: &github.Commit{
			Message: github.Ptr("First commit"),
			Author: &github.CommitAuthor{
				Name:  github.Ptr("Test User"),
				Email: github.Ptr("test@example.com"),
				Date:  &github.Timestamp{Time: time.Now().Add(-48 * time.Hour)},
			},
		},
		Author: &github.User{
			Login: github.Ptr("testuser"),
		},
		HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123def456"),
		Stats: &github.CommitStats{
			Additions: github.Ptr(10),
			Deletions: github.Ptr(2),
			Total:     github.Ptr(12),
		},
		Files: []*github.CommitFile{
			{
				Filename:  github.Ptr("file1.go"),
				Status:    github.Ptr("modified"),
				Additions: github.Ptr(10),
				Deletions: github.Ptr(2),
				Changes:   github.Ptr(12),
				Patch:     github.Ptr("@@ -1,2 +1,10 @@"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedCommit *github.RepositoryCommit
		expectedErrMsg string
	}{
		{
			name: "successful commit fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepoByRef,
					mockResponse(t, http.StatusOK, mockCommit),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"sha":   "abc123def456",
			},
			expectError:    false,
			expectedCommit: mockCommit,
		},
		{
			name: "commit fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"sha":   "nonexistent-sha",
			},
			expectError:    true,
			expectedErrMsg: "failed to get commit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedCommit github.RepositoryCommit
			err = json.Unmarshal([]byte(textContent.Text), &returnedCommit)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedCommit.SHA, *returnedCommit.SHA)
			assert.Equal(t, *tc.expectedCommit.Commit.Message, *returnedCommit.Commit.Message)
			assert.Equal(t, *tc.expectedCommit.Author.Login, *returnedCommit.Author.Login)
			assert.Equal(t, *tc.expectedCommit.HTMLURL, *returnedCommit.HTMLURL)
		})
	}
}

func Test_ListCommits(t *testing.T) {
	// Verify tool definition once
	serverTool := ListCommits(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_commits", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "sha")
	assert.Contains(t, schema.Properties, "author")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock commits for success case
	mockCommits := []*github.RepositoryCommit{
		{
			SHA: github.Ptr("abc123def456"),
			Commit: &github.Commit{
				Message: github.Ptr("First commit"),
				Author: &github.CommitAuthor{
					Name:  github.Ptr("Test User"),
					Email: github.Ptr("test@example.com"),
					Date:  &github.Timestamp{Time: time.Now().Add(-48 * time.Hour)},
				},
			},
			Author: &github.User{
				Login:     github.Ptr("testuser"),
				ID:        github.Ptr(int64(12345)),
				HTMLURL:   github.Ptr("https://github.com/testuser"),
				AvatarURL: github.Ptr("https://github.com/testuser.png"),
			},
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123def456"),
			Stats: &github.CommitStats{
				Additions: github.Ptr(10),
				Deletions: github.Ptr(5),
				Total:     github.Ptr(15),
			},
			Files: []*github.CommitFile{
				{
					Filename:  github.Ptr("src/main.go"),
					Status:    github.Ptr("modified"),
					Additions: github.Ptr(8),
					Deletions: github.Ptr(3),
					Changes:   github.Ptr(11),
				},
				{
					Filename:  github.Ptr("README.md"),
					Status:    github.Ptr("added"),
					Additions: github.Ptr(2),
					Deletions: github.Ptr(2),
					Changes:   github.Ptr(4),
				},
			},
		},
		{
			SHA: github.Ptr("def456abc789"),
			Commit: &github.Commit{
				Message: github.Ptr("Second commit"),
				Author: &github.CommitAuthor{
					Name:  github.Ptr("Another User"),
					Email: github.Ptr("another@example.com"),
					Date:  &github.Timestamp{Time: time.Now().Add(-24 * time.Hour)},
				},
			},
			Author: &github.User{
				Login:     github.Ptr("anotheruser"),
				ID:        github.Ptr(int64(67890)),
				HTMLURL:   github.Ptr("https://github.com/anotheruser"),
				AvatarURL: github.Ptr("https://github.com/anotheruser.png"),
			},
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/def456abc789"),
			Stats: &github.CommitStats{
				Additions: github.Ptr(20),
				Deletions: github.Ptr(10),
				Total:     github.Ptr(30),
			},
			Files: []*github.CommitFile{
				{
					Filename:  github.Ptr("src/utils.go"),
					Status:    github.Ptr("added"),
					Additions: github.Ptr(20),
					Deletions: github.Ptr(10),
					Changes:   github.Ptr(30),
				},
			},
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedCommits []*github.RepositoryCommit
		expectedErrMsg  string
	}{
		{
			name: "successful commits fetch with default params",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposCommitsByOwnerByRepo,
					mockCommits,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:     false,
			expectedCommits: mockCommits,
		},
		{
			name: "successful commits fetch with branch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"author":   "username",
						"sha":      "main",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockCommits),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"sha":    "main",
				"author": "username",
			},
			expectError:     false,
			expectedCommits: mockCommits,
		},
		{
			name: "successful commits fetch with pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockCommits),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:     false,
			expectedCommits: mockCommits,
		},
		{
			name: "commits fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "nonexistent-repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list commits",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedCommits []MinimalCommit
			err = json.Unmarshal([]byte(textContent.Text), &returnedCommits)
			require.NoError(t, err)
			assert.Len(t, returnedCommits, len(tc.expectedCommits))
			for i, commit := range returnedCommits {
				assert.Equal(t, tc.expectedCommits[i].GetSHA(), commit.SHA)
				assert.Equal(t, tc.expectedCommits[i].GetHTMLURL(), commit.HTMLURL)
				if tc.expectedCommits[i].Commit != nil {
					assert.Equal(t, tc.expectedCommits[i].Commit.GetMessage(), commit.Commit.Message)
				}
				if tc.expectedCommits[i].Author != nil {
					assert.Equal(t, tc.expectedCommits[i].Author.GetLogin(), commit.Author.Login)
				}

				// Files and stats are never included in list_commits
				assert.Nil(t, commit.Files)
				assert.Nil(t, commit.Stats)
			}
		})
	}
}

func Test_CreateOrUpdateFile(t *testing.T) {
	// Verify tool definition once
	serverTool := CreateOrUpdateFile(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "create_or_update_file", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "path")
	assert.Contains(t, schema.Properties, "content")
	assert.Contains(t, schema.Properties, "message")
	assert.Contains(t, schema.Properties, "branch")
	assert.Contains(t, schema.Properties, "sha")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "path", "content", "message", "branch"})

	// Setup mock file content response
	mockFileResponse := &github.RepositoryContentResponse{
		Content: &github.RepositoryContent{
			Name:        github.Ptr("example.md"),
			Path:        github.Ptr("docs/example.md"),
			SHA:         github.Ptr("abc123def456"),
			Size:        github.Ptr(42),
			HTMLURL:     github.Ptr("https://github.com/owner/repo/blob/main/docs/example.md"),
			DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/docs/example.md"),
		},
		Commit: github.Commit{
			SHA:     github.Ptr("def456abc789"),
			Message: github.Ptr("Add example file"),
			Author: &github.CommitAuthor{
				Name:  github.Ptr("Test User"),
				Email: github.Ptr("test@example.com"),
				Date:  &github.Timestamp{Time: time.Now()},
			},
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/def456abc789"),
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedContent *github.RepositoryContentResponse
		expectedErrMsg  string
	}{
		{
			name: "successful file creation",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Add example file",
						"content": "IyBFeGFtcGxlCgpUaGlzIGlzIGFuIGV4YW1wbGUgZmlsZS4=", // Base64 encoded content
						"branch":  "main",
					}).andThen(
						mockResponse(t, http.StatusOK, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# Example\n\nThis is an example file.",
				"message": "Add example file",
				"branch":  "main",
			},
			expectError:     false,
			expectedContent: mockFileResponse,
		},
		{
			name: "successful file update with SHA",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Update example file",
						"content": "IyBVcGRhdGVkIEV4YW1wbGUKClRoaXMgZmlsZSBoYXMgYmVlbiB1cGRhdGVkLg==", // Base64 encoded content
						"branch":  "main",
						"sha":     "abc123def456",
					}).andThen(
						mockResponse(t, http.StatusOK, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# Updated Example\n\nThis file has been updated.",
				"message": "Update example file",
				"branch":  "main",
				"sha":     "abc123def456",
			},
			expectError:     false,
			expectedContent: mockFileResponse,
		},
		{
			name: "file creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Invalid request"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "#Invalid Content",
				"message": "Invalid request",
				"branch":  "nonexistent-branch",
			},
			expectError:    true,
			expectedErrMsg: "failed to create/update file",
		},
		{
			name: "sha validation - current sha matches (304 Not Modified)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/contents/docs/example.md",
						Method:  "HEAD",
					},
					http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						// Verify If-None-Match header is set correctly
						ifNoneMatch := req.Header.Get("If-None-Match")
						if ifNoneMatch == `"abc123def456"` {
							w.WriteHeader(http.StatusNotModified)
						} else {
							w.WriteHeader(http.StatusOK)
							w.Header().Set("ETag", `"abc123def456"`)
						}
					}),
				),
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Update example file",
						"content": "IyBVcGRhdGVkIEV4YW1wbGUKClRoaXMgZmlsZSBoYXMgYmVlbiB1cGRhdGVkLg==",
						"branch":  "main",
						"sha":     "abc123def456",
					}).andThen(
						mockResponse(t, http.StatusOK, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# Updated Example\n\nThis file has been updated.",
				"message": "Update example file",
				"branch":  "main",
				"sha":     "abc123def456",
			},
			expectError:     false,
			expectedContent: mockFileResponse,
		},
		{
			name: "sha validation - stale sha detected (200 OK with different ETag)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/contents/docs/example.md",
						Method:  "HEAD",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						// SHA doesn't match - return 200 with current ETag
						w.Header().Set("ETag", `"newsha999888"`)
						w.WriteHeader(http.StatusOK)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# Updated Example\n\nThis file has been updated.",
				"message": "Update example file",
				"branch":  "main",
				"sha":     "oldsha123456",
			},
			expectError:    true,
			expectedErrMsg: "SHA mismatch: provided SHA oldsha123456 is stale. Current file SHA is newsha999888",
		},
		{
			name: "sha validation - file doesn't exist (404), proceed with create",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/contents/docs/example.md",
						Method:  "HEAD",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}),
				),
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Create new file",
						"content": "IyBOZXcgRmlsZQoKVGhpcyBpcyBhIG5ldyBmaWxlLg==",
						"branch":  "main",
						"sha":     "ignoredsha", // SHA is sent but GitHub API ignores it for new files
					}).andThen(
						mockResponse(t, http.StatusCreated, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# New File\n\nThis is a new file.",
				"message": "Create new file",
				"branch":  "main",
				"sha":     "ignoredsha",
			},
			expectError:     false,
			expectedContent: mockFileResponse,
		},
		{
			name: "no sha provided - file exists, returns warning",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/contents/docs/example.md",
						Method:  "HEAD",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("ETag", `"existing123"`)
						w.WriteHeader(http.StatusOK)
					}),
				),
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Update without SHA",
						"content": "IyBVcGRhdGVkCgpVcGRhdGVkIHdpdGhvdXQgU0hBLg==",
						"branch":  "main",
						"sha":     "existing123", // SHA is automatically added from ETag
					}).andThen(
						mockResponse(t, http.StatusOK, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# Updated\n\nUpdated without SHA.",
				"message": "Update without SHA",
				"branch":  "main",
			},
			expectError:    false,
			expectedErrMsg: "Warning: File updated without SHA validation. Previous file SHA was existing123",
		},
		{
			name: "no sha provided - file doesn't exist, no warning",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/repos/owner/repo/contents/docs/example.md",
						Method:  "HEAD",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}),
				),
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					expectRequestBody(t, map[string]interface{}{
						"message": "Create new file",
						"content": "IyBOZXcgRmlsZQoKQ3JlYXRlZCB3aXRob3V0IFNIQQ==",
						"branch":  "main",
					}).andThen(
						mockResponse(t, http.StatusCreated, mockFileResponse),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"content": "# New File\n\nCreated without SHA",
				"message": "Create new file",
				"branch":  "main",
			},
			expectError:     false,
			expectedContent: mockFileResponse,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// If expectedErrMsg is set (but expectError is false), this is a warning case
			if tc.expectedErrMsg != "" {
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Unmarshal and verify the result
			var returnedContent github.RepositoryContentResponse
			err = json.Unmarshal([]byte(textContent.Text), &returnedContent)
			require.NoError(t, err)

			// Verify content
			assert.Equal(t, *tc.expectedContent.Content.Name, *returnedContent.Content.Name)
			assert.Equal(t, *tc.expectedContent.Content.Path, *returnedContent.Content.Path)
			assert.Equal(t, *tc.expectedContent.Content.SHA, *returnedContent.Content.SHA)

			// Verify commit
			assert.Equal(t, *tc.expectedContent.Commit.SHA, *returnedContent.Commit.SHA)
			assert.Equal(t, *tc.expectedContent.Commit.Message, *returnedContent.Commit.Message)
		})
	}
}

func Test_CreateRepository(t *testing.T) {
	// Verify tool definition once
	serverTool := CreateRepository(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "create_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "name")
	assert.Contains(t, schema.Properties, "description")
	assert.Contains(t, schema.Properties, "organization")
	assert.Contains(t, schema.Properties, "private")
	assert.Contains(t, schema.Properties, "autoInit")
	assert.ElementsMatch(t, schema.Required, []string{"name"})

	// Setup mock repository response
	mockRepo := &github.Repository{
		Name:        github.Ptr("test-repo"),
		Description: github.Ptr("Test repository"),
		Private:     github.Ptr(true),
		HTMLURL:     github.Ptr("https://github.com/testuser/test-repo"),
		CreatedAt:   &github.Timestamp{Time: time.Now()},
		Owner: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedRepo   *github.Repository
		expectedErrMsg string
	}{
		{
			name: "successful repository creation with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/user/repos",
						Method:  "POST",
					},
					expectRequestBody(t, map[string]interface{}{
						"name":        "test-repo",
						"description": "Test repository",
						"private":     true,
						"auto_init":   true,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockRepo),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"name":        "test-repo",
				"description": "Test repository",
				"private":     true,
				"autoInit":    true,
			},
			expectError:  false,
			expectedRepo: mockRepo,
		},
		{
			name: "successful repository creation in organization",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/orgs/testorg/repos",
						Method:  "POST",
					},
					expectRequestBody(t, map[string]interface{}{
						"name":        "test-repo",
						"description": "Test repository",
						"private":     false,
						"auto_init":   true,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockRepo),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"name":         "test-repo",
				"description":  "Test repository",
				"organization": "testorg",
				"private":      false,
				"autoInit":     true,
			},
			expectError:  false,
			expectedRepo: mockRepo,
		},
		{
			name: "successful repository creation with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/user/repos",
						Method:  "POST",
					},
					expectRequestBody(t, map[string]interface{}{
						"name":        "test-repo",
						"auto_init":   false,
						"description": "",
						"private":     false,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockRepo),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"name": "test-repo",
			},
			expectError:  false,
			expectedRepo: mockRepo,
		},
		{
			name: "repository creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{
						Pattern: "/user/repos",
						Method:  "POST",
					},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Repository creation failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"name": "invalid-repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to create repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var returnedRepo MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &returnedRepo)
			assert.NoError(t, err)

			// Verify repository details
			assert.Equal(t, tc.expectedRepo.GetHTMLURL(), returnedRepo.URL)
		})
	}
}

func Test_PushFiles(t *testing.T) {
	// Verify tool definition once
	serverTool := PushFiles(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "push_files", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "branch")
	assert.Contains(t, schema.Properties, "files")
	assert.Contains(t, schema.Properties, "message")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "branch", "files", "message"})

	// Setup mock objects
	mockRef := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("abc123"),
			URL: github.Ptr("https://api.github.com/repos/owner/repo/git/trees/abc123"),
		},
	}

	mockCommit := &github.Commit{
		SHA: github.Ptr("abc123"),
		Tree: &github.Tree{
			SHA: github.Ptr("def456"),
		},
	}

	mockTree := &github.Tree{
		SHA: github.Ptr("ghi789"),
	}

	mockNewCommit := &github.Commit{
		SHA:     github.Ptr("jkl012"),
		Message: github.Ptr("Update multiple files"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/commit/jkl012"),
	}

	mockUpdatedRef := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("jkl012"),
			URL: github.Ptr("https://api.github.com/repos/owner/repo/git/trees/jkl012"),
		},
	}

	// Define test cases
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedRef    *github.Reference
		expectedErrMsg string
	}{
		{
			name: "successful push of multiple files",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Get commit
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
				// Create tree
				mock.WithRequestMatchHandler(
					mock.PostReposGitTreesByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"base_tree": "def456",
						"tree": []interface{}{
							map[string]interface{}{
								"path":    "README.md",
								"mode":    "100644",
								"type":    "blob",
								"content": "# Updated README\n\nThis is an updated README file.",
							},
							map[string]interface{}{
								"path":    "docs/example.md",
								"mode":    "100644",
								"type":    "blob",
								"content": "# Example\n\nThis is an example file.",
							},
						},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockTree),
					),
				),
				// Create commit
				mock.WithRequestMatchHandler(
					mock.PostReposGitCommitsByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"message": "Update multiple files",
						"tree":    "ghi789",
						"parents": []interface{}{"abc123"},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockNewCommit),
					),
				),
				// Update reference
				mock.WithRequestMatchHandler(
					mock.PatchReposGitRefsByOwnerByRepoByRef,
					expectRequestBody(t, map[string]interface{}{
						"sha":   "jkl012",
						"force": false,
					}).andThen(
						mockResponse(t, http.StatusOK, mockUpdatedRef),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# Updated README\n\nThis is an updated README file.",
					},
					map[string]interface{}{
						"path":    "docs/example.md",
						"content": "# Example\n\nThis is an example file.",
					},
				},
				"message": "Update multiple files",
			},
			expectError: false,
			expectedRef: mockUpdatedRef,
		},
		{
			name:         "fails when files parameter is invalid",
			mockedClient: mock.NewMockedHTTPClient(
			// No requests expected
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"branch":  "main",
				"files":   "invalid-files-parameter", // Not an array
				"message": "Update multiple files",
			},
			expectError:    false, // This returns a tool error, not a Go error
			expectedErrMsg: "files parameter must be an array",
		},
		{
			name: "fails when files contains object without path",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Get commit
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"content": "# Missing path",
					},
				},
				"message": "Update file",
			},
			expectError:    false, // This returns a tool error, not a Go error
			expectedErrMsg: "each file must have a path",
		},
		{
			name: "fails when files contains object without content",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Get commit
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path": "README.md",
						// Missing content
					},
				},
				"message": "Update file",
			},
			expectError:    false, // This returns a tool error, not a Go error
			expectedErrMsg: "each file must have content",
		},
		{
			name: "fails to get branch reference",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockResponse(t, http.StatusNotFound, nil),
				),
				// Mock Repositories.Get to fail when trying to create branch from default
				mock.WithRequestMatchHandler(
					mock.GetReposByOwnerByRepo,
					mockResponse(t, http.StatusNotFound, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "non-existent-branch",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
				},
				"message": "Update file",
			},
			expectError:    false,
			expectedErrMsg: "failed to create branch from default",
		},
		{
			name: "fails to get base commit",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Fail to get commit
				mock.WithRequestMatchHandler(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockResponse(t, http.StatusNotFound, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
				},
				"message": "Update file",
			},
			expectError:    true,
			expectedErrMsg: "failed to get base commit",
		},
		{
			name: "fails to create tree",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Get commit
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
				// Fail to create tree
				mock.WithRequestMatchHandler(
					mock.PostReposGitTreesByOwnerByRepo,
					mockResponse(t, http.StatusInternalServerError, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
				},
				"message": "Update file",
			},
			expectError:    true,
			expectedErrMsg: "failed to create tree",
		},
		{
			name: "successful push to empty repository",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference - first returns 409 for empty repo, second returns success after init
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					func() http.HandlerFunc {
						callCount := 0
						return func(w http.ResponseWriter, _ *http.Request) {
							w.Header().Set("Content-Type", "application/json")
							callCount++
							if callCount == 1 {
								// First call: empty repo
								w.WriteHeader(http.StatusConflict)
								response := map[string]interface{}{
									"message": "Git Repository is empty.",
								}
								_ = json.NewEncoder(w).Encode(response)
							} else {
								// Second call: return the created reference
								w.WriteHeader(http.StatusOK)
								_ = json.NewEncoder(w).Encode(mockRef)
							}
						}
					}(),
				),
				// Mock Repositories.Get to return default branch for initialization
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{
						DefaultBranch: github.Ptr("main"),
					},
				),
				// Create initial file using Contents API
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]interface{}
						err := json.NewDecoder(r.Body).Decode(&body)
						require.NoError(t, err)
						require.Equal(t, "Initial commit", body["message"])
						require.Equal(t, "main", body["branch"])
						w.WriteHeader(http.StatusCreated)
						response := &github.RepositoryContentResponse{
							Commit: github.Commit{SHA: github.Ptr("abc123")},
						}
						b, _ := json.Marshal(response)
						_, _ = w.Write(b)
					}),
				),
				// Get the commit after initialization
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
				// Create tree
				mock.WithRequestMatch(
					mock.PostReposGitTreesByOwnerByRepo,
					mockTree,
				),
				// Create commit
				mock.WithRequestMatch(
					mock.PostReposGitCommitsByOwnerByRepo,
					mockNewCommit,
				),
				// Update reference
				mock.WithRequestMatch(
					mock.PatchReposGitRefsByOwnerByRepoByRef,
					mockUpdatedRef,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# Initial README\n\nFirst commit to empty repository.",
					},
				},
				"message": "Initial commit",
			},
			expectError: false,
			expectedRef: mockUpdatedRef,
		},
		{
			name: "successful push multiple files to empty repository",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference - called twice: first for empty check, second after file creation
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					func() http.HandlerFunc {
						callCount := 0
						return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
							callCount++
							if callCount == 1 {
								// First call: returns 409 Conflict for empty repo
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(http.StatusConflict)
								response := map[string]interface{}{
									"message": "Git Repository is empty.",
								}
								_ = json.NewEncoder(w).Encode(response)
							} else {
								// Second call: returns the updated reference after first file creation
								w.WriteHeader(http.StatusOK)
								b, _ := json.Marshal(&github.Reference{
									Ref:    github.Ptr("refs/heads/main"),
									Object: &github.GitObject{SHA: github.Ptr("init456")},
								})
								_, _ = w.Write(b)
							}
						})
					}(),
				),
				// Mock Repositories.Get to return default branch for initialization
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{
						DefaultBranch: github.Ptr("main"),
					},
				),
				// Create initial empty README.md file using Contents API to initialize repo
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var body map[string]interface{}
						err := json.NewDecoder(r.Body).Decode(&body)
						require.NoError(t, err)
						require.Equal(t, "Initial commit", body["message"])
						require.Equal(t, "main", body["branch"])
						// Verify it's an empty file
						expectedContent := base64.StdEncoding.EncodeToString([]byte(""))
						require.Equal(t, expectedContent, body["content"])
						w.WriteHeader(http.StatusCreated)
						response := &github.RepositoryContentResponse{
							Content: &github.RepositoryContent{
								SHA: github.Ptr("readme123"),
							},
							Commit: github.Commit{
								SHA: github.Ptr("init456"),
								Tree: &github.Tree{
									SHA: github.Ptr("tree456"),
								},
							},
						}
						b, _ := json.Marshal(response)
						_, _ = w.Write(b)
					}),
				),
				// Get the commit to retrieve parent SHA
				mock.WithRequestMatchHandler(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						response := &github.Commit{
							SHA: github.Ptr("init456"),
							Tree: &github.Tree{
								SHA: github.Ptr("tree456"),
							},
						}
						b, _ := json.Marshal(response)
						_, _ = w.Write(b)
					}),
				),
				// Create tree with all user files
				mock.WithRequestMatchHandler(
					mock.PostReposGitTreesByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"base_tree": "tree456",
						"tree": []interface{}{
							map[string]interface{}{
								"path":    "README.md",
								"mode":    "100644",
								"type":    "blob",
								"content": "# Project\n\nProject README",
							},
							map[string]interface{}{
								"path":    ".gitignore",
								"mode":    "100644",
								"type":    "blob",
								"content": "node_modules/\n*.log\n",
							},
							map[string]interface{}{
								"path":    "src/main.js",
								"mode":    "100644",
								"type":    "blob",
								"content": "console.log('Hello World');\n",
							},
						},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockTree),
					),
				),
				// Create commit with all user files
				mock.WithRequestMatchHandler(
					mock.PostReposGitCommitsByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"message": "Initial project setup",
						"tree":    "ghi789",
						"parents": []interface{}{"init456"},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockNewCommit),
					),
				),
				// Update reference
				mock.WithRequestMatchHandler(
					mock.PatchReposGitRefsByOwnerByRepoByRef,
					expectRequestBody(t, map[string]interface{}{
						"sha":   "jkl012",
						"force": false,
					}).andThen(
						mockResponse(t, http.StatusOK, mockUpdatedRef),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# Project\n\nProject README",
					},
					map[string]interface{}{
						"path":    ".gitignore",
						"content": "node_modules/\n*.log\n",
					},
					map[string]interface{}{
						"path":    "src/main.js",
						"content": "console.log('Hello World');\n",
					},
				},
				"message": "Initial project setup",
			},
			expectError: false,
			expectedRef: mockUpdatedRef,
		},
		{
			name: "fails to create initial file in empty repository",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference returns 409 Conflict for empty repo
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusConflict)
						response := map[string]interface{}{
							"message": "Git Repository is empty.",
						}
						_ = json.NewEncoder(w).Encode(response)
					}),
				),
				// Mock Repositories.Get to return default branch
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{
						DefaultBranch: github.Ptr("main"),
					},
				),
				// Fail to create initial file using Contents API
				mock.WithRequestMatchHandler(
					mock.PutReposContentsByOwnerByRepoByPath,
					mockResponse(t, http.StatusInternalServerError, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
				},
				"message": "Initial commit",
			},
			expectError:    false,
			expectedErrMsg: "failed to initialize repository",
		},
		{
			name: "fails to get reference after creating initial file in empty repository",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference - called twice
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					func() http.HandlerFunc {
						callCount := 0
						return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
							callCount++
							if callCount == 1 {
								// First call: returns 409 Conflict for empty repo
								w.Header().Set("Content-Type", "application/json")
								w.WriteHeader(http.StatusConflict)
								response := map[string]interface{}{
									"message": "Git Repository is empty.",
								}
								_ = json.NewEncoder(w).Encode(response)
							} else {
								// Second call: fails
								w.WriteHeader(http.StatusInternalServerError)
							}
						})
					}(),
				),
				// Mock Repositories.Get to return default branch
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{
						DefaultBranch: github.Ptr("main"),
					},
				),
				// Create initial file using Contents API
				mock.WithRequestMatch(
					mock.PutReposContentsByOwnerByRepoByPath,
					&github.RepositoryContentResponse{
						Content: &github.RepositoryContent{SHA: github.Ptr("readme123")},
						Commit:  github.Commit{SHA: github.Ptr("init456")},
					},
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
				},
				"message": "Initial commit",
			},
			expectError:    false,
			expectedErrMsg: "failed to initialize repository",
		},
		{
			name: "fails to get commit in empty repository with multiple files",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference returns 409 Conflict for empty repo
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusConflict)
						response := map[string]interface{}{
							"message": "Git Repository is empty.",
						}
						_ = json.NewEncoder(w).Encode(response)
					}),
				),
				// Mock Repositories.Get to return default branch
				mock.WithRequestMatch(
					mock.GetReposByOwnerByRepo,
					&github.Repository{
						DefaultBranch: github.Ptr("main"),
					},
				),
				// Create initial file using Contents API
				mock.WithRequestMatch(
					mock.PutReposContentsByOwnerByRepoByPath,
					&github.RepositoryContentResponse{
						Content: &github.RepositoryContent{SHA: github.Ptr("readme123")},
						Commit:  github.Commit{SHA: github.Ptr("init456")},
					},
				),
				// Fail to get commit
				mock.WithRequestMatchHandler(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockResponse(t, http.StatusInternalServerError, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"branch": "main",
				"files": []interface{}{
					map[string]interface{}{
						"path":    "README.md",
						"content": "# README",
					},
					map[string]interface{}{
						"path":    "LICENSE",
						"content": "MIT",
					},
				},
				"message": "Initial commit",
			},
			expectError:    false,
			expectedErrMsg: "failed to initialize repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			if tc.expectedErrMsg != "" {
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedRef github.Reference
			err = json.Unmarshal([]byte(textContent.Text), &returnedRef)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedRef.Ref, *returnedRef.Ref)
			assert.Equal(t, *tc.expectedRef.Object.SHA, *returnedRef.Object.SHA)
		})
	}
}

func Test_ListBranches(t *testing.T) {
	// Verify tool definition once
	serverTool := ListBranches(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_branches", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock branches for success case
	mockBranches := []*github.Branch{
		{
			Name:   github.Ptr("main"),
			Commit: &github.RepositoryCommit{SHA: github.Ptr("abc123")},
		},
		{
			Name:   github.Ptr("develop"),
			Commit: &github.RepositoryCommit{SHA: github.Ptr("def456")},
		},
	}

	// Test cases
	tests := []struct {
		name          string
		args          map[string]interface{}
		mockResponses []mock.MockBackendOption
		wantErr       bool
		errContains   string
	}{
		{
			name: "success",
			args: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"page":  float64(2),
			},
			mockResponses: []mock.MockBackendOption{
				mock.WithRequestMatch(
					mock.GetReposBranchesByOwnerByRepo,
					mockBranches,
				),
			},
			wantErr: false,
		},
		{
			name: "missing owner",
			args: map[string]interface{}{
				"repo": "repo",
			},
			mockResponses: []mock.MockBackendOption{},
			wantErr:       false,
			errContains:   "missing required parameter: owner",
		},
		{
			name: "missing repo",
			args: map[string]interface{}{
				"owner": "owner",
			},
			mockResponses: []mock.MockBackendOption{},
			wantErr:       false,
			errContains:   "missing required parameter: repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := github.NewClient(mock.NewMockedHTTPClient(tt.mockResponses...))
			deps := BaseDeps{
				Client: mockClient,
			}
			handler := serverTool.Handler(deps)

			// Create request
			request := createMCPRequest(tt.args)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			if tt.wantErr {
				require.NoError(t, err)
				if tt.errContains != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.errContains != "" {
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tt.errContains)
				return
			}

			textContent := getTextResult(t, result)
			require.NotEmpty(t, textContent.Text)

			// Verify response
			var branches []*github.Branch
			err = json.Unmarshal([]byte(textContent.Text), &branches)
			require.NoError(t, err)
			assert.Len(t, branches, 2)
			assert.Equal(t, "main", *branches[0].Name)
			assert.Equal(t, "develop", *branches[1].Name)
		})
	}
}

func Test_DeleteFile(t *testing.T) {
	// Verify tool definition once
	serverTool := DeleteFile(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "delete_file", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "path")
	assert.Contains(t, schema.Properties, "message")
	assert.Contains(t, schema.Properties, "branch")
	// SHA is no longer required since we're using Git Data API
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "path", "message", "branch"})

	// Setup mock objects for Git Data API
	mockRef := &github.Reference{
		Ref: github.Ptr("refs/heads/main"),
		Object: &github.GitObject{
			SHA: github.Ptr("abc123"),
		},
	}

	mockCommit := &github.Commit{
		SHA: github.Ptr("abc123"),
		Tree: &github.Tree{
			SHA: github.Ptr("def456"),
		},
	}

	mockTree := &github.Tree{
		SHA: github.Ptr("ghi789"),
	}

	mockNewCommit := &github.Commit{
		SHA:     github.Ptr("jkl012"),
		Message: github.Ptr("Delete example file"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/commit/jkl012"),
	}

	tests := []struct {
		name              string
		mockedClient      *http.Client
		requestArgs       map[string]interface{}
		expectError       bool
		expectedCommitSHA string
		expectedErrMsg    string
	}{
		{
			name: "successful file deletion using Git Data API",
			mockedClient: mock.NewMockedHTTPClient(
				// Get branch reference
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockRef,
				),
				// Get commit
				mock.WithRequestMatch(
					mock.GetReposGitCommitsByOwnerByRepoByCommitSha,
					mockCommit,
				),
				// Create tree
				mock.WithRequestMatchHandler(
					mock.PostReposGitTreesByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"base_tree": "def456",
						"tree": []interface{}{
							map[string]interface{}{
								"path": "docs/example.md",
								"mode": "100644",
								"type": "blob",
								"sha":  nil,
							},
						},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockTree),
					),
				),
				// Create commit
				mock.WithRequestMatchHandler(
					mock.PostReposGitCommitsByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"message": "Delete example file",
						"tree":    "ghi789",
						"parents": []interface{}{"abc123"},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockNewCommit),
					),
				),
				// Update reference
				mock.WithRequestMatchHandler(
					mock.PatchReposGitRefsByOwnerByRepoByRef,
					expectRequestBody(t, map[string]interface{}{
						"sha":   "jkl012",
						"force": false,
					}).andThen(
						mockResponse(t, http.StatusOK, &github.Reference{
							Ref: github.Ptr("refs/heads/main"),
							Object: &github.GitObject{
								SHA: github.Ptr("jkl012"),
							},
						}),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/example.md",
				"message": "Delete example file",
				"branch":  "main",
			},
			expectError:       false,
			expectedCommitSHA: "jkl012",
		},
		{
			name: "file deletion fails - branch not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Reference not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "docs/nonexistent.md",
				"message": "Delete nonexistent file",
				"branch":  "nonexistent-branch",
			},
			expectError:    true,
			expectedErrMsg: "failed to get branch reference",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var response map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)

			// Verify the response contains the expected commit
			commit, ok := response["commit"].(map[string]interface{})
			require.True(t, ok)
			commitSHA, ok := commit["sha"].(string)
			require.True(t, ok)
			assert.Equal(t, tc.expectedCommitSHA, commitSHA)
		})
	}
}

func Test_ListTags(t *testing.T) {
	// Verify tool definition once
	serverTool := ListTags(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_tags", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock tags for success case
	mockTags := []*github.RepositoryTag{
		{
			Name: github.Ptr("v1.0.0"),
			Commit: &github.Commit{
				SHA: github.Ptr("v1.0.0-tag-sha"),
				URL: github.Ptr("https://api.github.com/repos/owner/repo/commits/abc123"),
			},
			ZipballURL: github.Ptr("https://github.com/owner/repo/zipball/v1.0.0"),
			TarballURL: github.Ptr("https://github.com/owner/repo/tarball/v1.0.0"),
		},
		{
			Name: github.Ptr("v0.9.0"),
			Commit: &github.Commit{
				SHA: github.Ptr("v0.9.0-tag-sha"),
				URL: github.Ptr("https://api.github.com/repos/owner/repo/commits/def456"),
			},
			ZipballURL: github.Ptr("https://github.com/owner/repo/zipball/v0.9.0"),
			TarballURL: github.Ptr("https://github.com/owner/repo/tarball/v0.9.0"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedTags   []*github.RepositoryTag
		expectedErrMsg string
	}{
		{
			name: "successful tags list",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposTagsByOwnerByRepo,
					expectPath(
						t,
						"/repos/owner/repo/tags",
					).andThen(
						mockResponse(t, http.StatusOK, mockTags),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:  false,
			expectedTags: mockTags,
		},
		{
			name: "list tags fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposTagsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list tags",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Parse and verify the result
			var returnedTags []*github.RepositoryTag
			err = json.Unmarshal([]byte(textContent.Text), &returnedTags)
			require.NoError(t, err)

			// Verify each tag
			require.Equal(t, len(tc.expectedTags), len(returnedTags))
			for i, expectedTag := range tc.expectedTags {
				assert.Equal(t, *expectedTag.Name, *returnedTags[i].Name)
				assert.Equal(t, *expectedTag.Commit.SHA, *returnedTags[i].Commit.SHA)
			}
		})
	}
}

func Test_GetTag(t *testing.T) {
	// Verify tool definition once
	serverTool := GetTag(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_tag", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "tag")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "tag"})

	mockTagRef := &github.Reference{
		Ref: github.Ptr("refs/tags/v1.0.0"),
		Object: &github.GitObject{
			SHA: github.Ptr("v1.0.0-tag-sha"),
		},
	}

	mockTagObj := &github.Tag{
		SHA:     github.Ptr("v1.0.0-tag-sha"),
		Tag:     github.Ptr("v1.0.0"),
		Message: github.Ptr("Release v1.0.0"),
		Object: &github.GitObject{
			Type: github.Ptr("commit"),
			SHA:  github.Ptr("abc123"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedTag    *github.Tag
		expectedErrMsg string
	}{
		{
			name: "successful tag retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					expectPath(
						t,
						"/repos/owner/repo/git/ref/tags/v1.0.0",
					).andThen(
						mockResponse(t, http.StatusOK, mockTagRef),
					),
				),
				mock.WithRequestMatchHandler(
					mock.GetReposGitTagsByOwnerByRepoByTagSha,
					expectPath(
						t,
						"/repos/owner/repo/git/tags/v1.0.0-tag-sha",
					).andThen(
						mockResponse(t, http.StatusOK, mockTagObj),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v1.0.0",
			},
			expectError: false,
			expectedTag: mockTagObj,
		},
		{
			name: "tag reference not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposGitRefByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Reference does not exist"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v1.0.0",
			},
			expectError:    true,
			expectedErrMsg: "failed to get tag reference",
		},
		{
			name: "tag object not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposGitRefByOwnerByRepoByRef,
					mockTagRef,
				),
				mock.WithRequestMatchHandler(
					mock.GetReposGitTagsByOwnerByRepoByTagSha,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Tag object does not exist"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v1.0.0",
			},
			expectError:    true,
			expectedErrMsg: "failed to get tag object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Parse and verify the result
			var returnedTag github.Tag
			err = json.Unmarshal([]byte(textContent.Text), &returnedTag)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedTag.SHA, *returnedTag.SHA)
			assert.Equal(t, *tc.expectedTag.Tag, *returnedTag.Tag)
			assert.Equal(t, *tc.expectedTag.Message, *returnedTag.Message)
			assert.Equal(t, *tc.expectedTag.Object.Type, *returnedTag.Object.Type)
			assert.Equal(t, *tc.expectedTag.Object.SHA, *returnedTag.Object.SHA)
		})
	}
}

func Test_ListReleases(t *testing.T) {
	serverTool := ListReleases(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_releases", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	mockReleases := []*github.RepositoryRelease{
		{
			ID:      github.Ptr(int64(1)),
			TagName: github.Ptr("v1.0.0"),
			Name:    github.Ptr("First Release"),
		},
		{
			ID:      github.Ptr(int64(2)),
			TagName: github.Ptr("v0.9.0"),
			Name:    github.Ptr("Beta Release"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult []*github.RepositoryRelease
		expectedErrMsg string
	}{
		{
			name: "successful releases list",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposReleasesByOwnerByRepo,
					mockReleases,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false,
			expectedResult: mockReleases,
		},
		{
			name: "releases list fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list releases",
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

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			var returnedReleases []*github.RepositoryRelease
			err = json.Unmarshal([]byte(textContent.Text), &returnedReleases)
			require.NoError(t, err)
			assert.Len(t, returnedReleases, len(tc.expectedResult))
			for i, rel := range returnedReleases {
				assert.Equal(t, *tc.expectedResult[i].TagName, *rel.TagName)
			}
		})
	}
}
func Test_GetLatestRelease(t *testing.T) {
	serverTool := GetLatestRelease(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_latest_release", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	mockRelease := &github.RepositoryRelease{
		ID:      github.Ptr(int64(1)),
		TagName: github.Ptr("v1.0.0"),
		Name:    github.Ptr("First Release"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.RepositoryRelease
		expectedErrMsg string
	}{
		{
			name: "successful latest release fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposReleasesLatestByOwnerByRepo,
					mockRelease,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false,
			expectedResult: mockRelease,
		},
		{
			name: "latest release fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesLatestByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to get latest release",
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

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			var returnedRelease github.RepositoryRelease
			err = json.Unmarshal([]byte(textContent.Text), &returnedRelease)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.TagName, *returnedRelease.TagName)
		})
	}
}

func Test_GetReleaseByTag(t *testing.T) {
	serverTool := GetReleaseByTag(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "get_release_by_tag", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "tag")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "tag"})

	mockRelease := &github.RepositoryRelease{
		ID:      github.Ptr(int64(1)),
		TagName: github.Ptr("v1.0.0"),
		Name:    github.Ptr("Release v1.0.0"),
		Body:    github.Ptr("This is the first stable release."),
		Assets: []*github.ReleaseAsset{
			{
				ID:   github.Ptr(int64(1)),
				Name: github.Ptr("release-v1.0.0.tar.gz"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.RepositoryRelease
		expectedErrMsg string
	}{
		{
			name: "successful release by tag fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposReleasesTagsByOwnerByRepoByTag,
					mockRelease,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v1.0.0",
			},
			expectError:    false,
			expectedResult: mockRelease,
		},
		{
			name:         "missing owner parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"repo": "repo",
				"tag":  "v1.0.0",
			},
			expectError:    false, // Returns tool error, not Go error
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing repo parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"tag":   "v1.0.0",
			},
			expectError:    false, // Returns tool error, not Go error
			expectedErrMsg: "missing required parameter: repo",
		},
		{
			name:         "missing tag parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false, // Returns tool error, not Go error
			expectedErrMsg: "missing required parameter: tag",
		},
		{
			name: "release by tag not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesTagsByOwnerByRepoByTag,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v999.0.0",
			},
			expectError:    false, // API errors return tool errors, not Go errors
			expectedErrMsg: "failed to get release by tag: v999.0.0",
		},
		{
			name: "server error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposReleasesTagsByOwnerByRepoByTag,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"tag":   "v1.0.0",
			},
			expectError:    false, // API errors return tool errors, not Go errors
			expectedErrMsg: "failed to get release by tag: v1.0.0",
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

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			if tc.expectedErrMsg != "" {
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)

			textContent := getTextResult(t, result)

			var returnedRelease github.RepositoryRelease
			err = json.Unmarshal([]byte(textContent.Text), &returnedRelease)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedResult.ID, *returnedRelease.ID)
			assert.Equal(t, *tc.expectedResult.TagName, *returnedRelease.TagName)
			assert.Equal(t, *tc.expectedResult.Name, *returnedRelease.Name)
			if tc.expectedResult.Body != nil {
				assert.Equal(t, *tc.expectedResult.Body, *returnedRelease.Body)
			}
			if len(tc.expectedResult.Assets) > 0 {
				require.Len(t, returnedRelease.Assets, len(tc.expectedResult.Assets))
				assert.Equal(t, *tc.expectedResult.Assets[0].Name, *returnedRelease.Assets[0].Name)
			}
		})
	}
}

func Test_looksLikeSHA(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "full 40-character SHA",
			input:    "abc123def456abc123def456abc123def456abc1",
			expected: true,
		},
		{
			name:     "too short",
			input:    "abc123def456abc123def45",
			expected: false,
		},
		{
			name:     "too long - 41 characters",
			input:    "abc123def456abc123def456abc123def456abc12",
			expected: false,
		},
		{
			name:     "contains invalid character - space",
			input:    "abc123def456abc123def456 bc123def456abc1",
			expected: false,
		},
		{
			name:     "contains invalid character - dash",
			input:    "abc123def456abc123d-f456abc123def456abc1",
			expected: false,
		},
		{
			name:     "contains invalid character - g",
			input:    "abc123def456gbc123def456abc123def456abc1",
			expected: false,
		},
		{
			name:     "branch name with slash",
			input:    "feature/branch",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "all zeros SHA",
			input:    "0000000000000000000000000000000000000000",
			expected: true,
		},
		{
			name:     "all f's SHA",
			input:    "ffffffffffffffffffffffffffffffffffffffff",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := looksLikeSHA(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_filterPaths(t *testing.T) {
	tests := []struct {
		name       string
		tree       []*github.TreeEntry
		path       string
		maxResults int
		expected   []string
	}{
		{
			name: "file name",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("folder/foo.txt"), Type: github.Ptr("blob")},
				{Path: github.Ptr("bar.txt"), Type: github.Ptr("blob")},
				{Path: github.Ptr("nested/folder/foo.txt"), Type: github.Ptr("blob")},
				{Path: github.Ptr("nested/folder/baz.txt"), Type: github.Ptr("blob")},
			},
			path:       "foo.txt",
			maxResults: -1,
			expected:   []string{"folder/foo.txt", "nested/folder/foo.txt"},
		},
		{
			name: "dir name",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("bar.txt"), Type: github.Ptr("blob")},
				{Path: github.Ptr("nested/folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/folder/baz.txt"), Type: github.Ptr("blob")},
			},
			path:       "folder/",
			maxResults: -1,
			expected:   []string{"folder/", "nested/folder/"},
		},
		{
			name: "dir and file match",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("name"), Type: github.Ptr("tree")},
				{Path: github.Ptr("name"), Type: github.Ptr("blob")},
			},
			path:       "name", // No trailing slash can match both files and directories
			maxResults: -1,
			expected:   []string{"name/", "name"},
		},
		{
			name: "dir only match",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("name"), Type: github.Ptr("tree")},
				{Path: github.Ptr("name"), Type: github.Ptr("blob")},
			},
			path:       "name/", // Trialing slash ensures only directories are matched
			maxResults: -1,
			expected:   []string{"name/"},
		},
		{
			name: "max results limit 2",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/nested/folder"), Type: github.Ptr("tree")},
			},
			path:       "folder/",
			maxResults: 2,
			expected:   []string{"folder/", "nested/folder/"},
		},
		{
			name: "max results limit 1",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/nested/folder"), Type: github.Ptr("tree")},
			},
			path:       "folder/",
			maxResults: 1,
			expected:   []string{"folder/"},
		},
		{
			name: "max results limit 0",
			tree: []*github.TreeEntry{
				{Path: github.Ptr("folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/folder"), Type: github.Ptr("tree")},
				{Path: github.Ptr("nested/nested/folder"), Type: github.Ptr("tree")},
			},
			path:       "folder/",
			maxResults: 0,
			expected:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterPaths(tc.tree, tc.path, tc.maxResults)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_resolveGitReference(t *testing.T) {
	ctx := context.Background()
	owner := "owner"
	repo := "repo"

	tests := []struct {
		name           string
		ref            string
		sha            string
		mockSetup      func() *http.Client
		expectedOutput *raw.ContentOpts
		expectError    bool
		errorContains  string
	}{
		{
			name: "sha takes precedence over ref",
			ref:  "refs/heads/main",
			sha:  "123sha456",
			mockSetup: func() *http.Client {
				// No API calls should be made when SHA is provided
				return mock.NewMockedHTTPClient()
			},
			expectedOutput: &raw.ContentOpts{
				SHA: "123sha456",
			},
			expectError: false,
		},
		{
			name: "use default branch if ref and sha both empty",
			ref:  "",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposByOwnerByRepo,
						http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"name": "repo", "default_branch": "main"}`))
						}),
					),
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							assert.Contains(t, r.URL.Path, "/git/ref/heads/main")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": "main-sha"}}`))
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/heads/main",
				SHA: "main-sha",
			},
			expectError: false,
		},
		{
			name: "fully qualified ref passed through unchanged",
			ref:  "refs/heads/feature-branch",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							assert.Contains(t, r.URL.Path, "/git/ref/heads/feature-branch")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/heads/feature-branch", "object": {"sha": "feature-sha"}}`))
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/heads/feature-branch",
				SHA: "feature-sha",
			},
			expectError: false,
		},
		{
			name: "short branch name resolves to refs/heads/",
			ref:  "main",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							if strings.Contains(r.URL.Path, "/git/ref/heads/main") {
								w.WriteHeader(http.StatusOK)
								_, _ = w.Write([]byte(`{"ref": "refs/heads/main", "object": {"sha": "main-sha"}}`))
							} else {
								t.Errorf("Unexpected path: %s", r.URL.Path)
								w.WriteHeader(http.StatusNotFound)
							}
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/heads/main",
				SHA: "main-sha",
			},
			expectError: false,
		},
		{
			name: "short tag name falls back to refs/tags/ when branch not found",
			ref:  "v1.0.0",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							switch {
							case strings.Contains(r.URL.Path, "/git/ref/heads/v1.0.0"):
								w.WriteHeader(http.StatusNotFound)
								_, _ = w.Write([]byte(`{"message": "Not Found"}`))
							case strings.Contains(r.URL.Path, "/git/ref/tags/v1.0.0"):
								w.WriteHeader(http.StatusOK)
								_, _ = w.Write([]byte(`{"ref": "refs/tags/v1.0.0", "object": {"sha": "tag-sha"}}`))
							default:
								t.Errorf("Unexpected path: %s", r.URL.Path)
								w.WriteHeader(http.StatusNotFound)
							}
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/tags/v1.0.0",
				SHA: "tag-sha",
			},
			expectError: false,
		},
		{
			name: "heads/ prefix gets refs/ prepended",
			ref:  "heads/feature-branch",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							assert.Contains(t, r.URL.Path, "/git/ref/heads/feature-branch")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/heads/feature-branch", "object": {"sha": "feature-sha"}}`))
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/heads/feature-branch",
				SHA: "feature-sha",
			},
			expectError: false,
		},
		{
			name: "tags/ prefix gets refs/ prepended",
			ref:  "tags/v1.0.0",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							assert.Contains(t, r.URL.Path, "/git/ref/tags/v1.0.0")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/tags/v1.0.0", "object": {"sha": "tag-sha"}}`))
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/tags/v1.0.0",
				SHA: "tag-sha",
			},
			expectError: false,
		},
		{
			name: "invalid short name that doesn't exist as branch or tag",
			ref:  "nonexistent",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
							// Both branch and tag attempts should return 404
							w.WriteHeader(http.StatusNotFound)
							_, _ = w.Write([]byte(`{"message": "Not Found"}`))
						}),
					),
				)
			},
			expectError:   true,
			errorContains: "could not resolve ref \"nonexistent\" as a branch or a tag",
		},
		{
			name: "fully qualified pull request ref",
			ref:  "refs/pull/123/head",
			sha:  "",
			mockSetup: func() *http.Client {
				return mock.NewMockedHTTPClient(
					mock.WithRequestMatchHandler(
						mock.GetReposGitRefByOwnerByRepoByRef,
						http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							assert.Contains(t, r.URL.Path, "/git/ref/pull/123/head")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write([]byte(`{"ref": "refs/pull/123/head", "object": {"sha": "pr-sha"}}`))
						}),
					),
				)
			},
			expectedOutput: &raw.ContentOpts{
				Ref: "refs/pull/123/head",
				SHA: "pr-sha",
			},
			expectError: false,
		},
		{
			name: "ref looks like full SHA with empty sha parameter",
			ref:  "abc123def456abc123def456abc123def456abc1",
			sha:  "",
			mockSetup: func() *http.Client {
				// No API calls should be made when ref looks like SHA
				return mock.NewMockedHTTPClient()
			},
			expectedOutput: &raw.ContentOpts{
				SHA: "abc123def456abc123def456abc123def456abc1",
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockSetup())
			opts, _, err := resolveGitReference(ctx, client, owner, repo, tc.ref, tc.sha)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, opts)

			if tc.expectedOutput.SHA != "" {
				assert.Equal(t, tc.expectedOutput.SHA, opts.SHA)
			}
			if tc.expectedOutput.Ref != "" {
				assert.Equal(t, tc.expectedOutput.Ref, opts.Ref)
			}
		})
	}
}

func Test_ListStarredRepositories(t *testing.T) {
	// Verify tool definition once
	serverTool := ListStarredRepositories(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "list_starred_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "username")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "direction")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Empty(t, schema.Required) // All parameters are optional

	// Setup mock starred repositories
	starredAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-2 * time.Hour)
	mockStarredRepos := []*github.StarredRepository{
		{
			StarredAt: &github.Timestamp{Time: starredAt},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(12345)),
				Name:            github.Ptr("awesome-repo"),
				FullName:        github.Ptr("owner/awesome-repo"),
				Description:     github.Ptr("An awesome repository"),
				HTMLURL:         github.Ptr("https://github.com/owner/awesome-repo"),
				Language:        github.Ptr("Go"),
				StargazersCount: github.Ptr(100),
				ForksCount:      github.Ptr(25),
				OpenIssuesCount: github.Ptr(5),
				UpdatedAt:       &github.Timestamp{Time: updatedAt},
				Private:         github.Ptr(false),
				Fork:            github.Ptr(false),
				Archived:        github.Ptr(false),
				DefaultBranch:   github.Ptr("main"),
			},
		},
		{
			StarredAt: &github.Timestamp{Time: starredAt.Add(-12 * time.Hour)},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(67890)),
				Name:            github.Ptr("cool-project"),
				FullName:        github.Ptr("user/cool-project"),
				Description:     github.Ptr("A very cool project"),
				HTMLURL:         github.Ptr("https://github.com/user/cool-project"),
				Language:        github.Ptr("Python"),
				StargazersCount: github.Ptr(500),
				ForksCount:      github.Ptr(75),
				OpenIssuesCount: github.Ptr(10),
				UpdatedAt:       &github.Timestamp{Time: updatedAt.Add(-1 * time.Hour)},
				Private:         github.Ptr(false),
				Fork:            github.Ptr(true),
				Archived:        github.Ptr(false),
				DefaultBranch:   github.Ptr("master"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		expectedCount  int
	}{
		{
			name: "successful list for authenticated user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockStarredRepos))
					}),
				),
			),
			requestArgs:   map[string]interface{}{},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "successful list for specific user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersStarredByUsername,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(mockStarredRepos))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "list fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "failed to list starred repositories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)

				// Unmarshal and verify the result
				var returnedRepos []MinimalRepository
				err = json.Unmarshal([]byte(textContent.Text), &returnedRepos)
				require.NoError(t, err)

				assert.Len(t, returnedRepos, tc.expectedCount)
				if tc.expectedCount > 0 {
					assert.Equal(t, "awesome-repo", returnedRepos[0].Name)
					assert.Equal(t, "owner/awesome-repo", returnedRepos[0].FullName)
				}
			}
		})
	}
}

func Test_StarRepository(t *testing.T) {
	// Verify tool definition once
	serverTool := StarRepository(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "star_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful star",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			expectError: false,
		},
		{
			name: "star fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to star repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, "Successfully starred repository")
			}
		})
	}
}

func Test_UnstarRepository(t *testing.T) {
	// Verify tool definition once
	serverTool := UnstarRepository(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Equal(t, "unstar_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful unstar",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "testrepo",
			},
			expectError: false,
		},
		{
			name: "unstar fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.DeleteUserStarredByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "testowner",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to unstar repository",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NotNil(t, result)
				textResult, ok := result.Content[0].(*mcp.TextContent)
				require.True(t, ok, "Expected text content")
				assert.Contains(t, textResult.Text, tc.expectedErrMsg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Parse the result and get the text content
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, "Successfully unstarred repository")
			}
		})
	}
}
