package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetFileContents(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetFileContents(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_file_contents", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "path")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "path"})

	// Setup mock file content for success case
	mockFileContent := &github.RepositoryContent{
		Type:        github.Ptr("file"),
		Name:        github.Ptr("README.md"),
		Path:        github.Ptr("README.md"),
		Content:     github.Ptr("IyBUZXN0IFJlcG9zaXRvcnkKClRoaXMgaXMgYSB0ZXN0IHJlcG9zaXRvcnku"), // Base64 encoded "# Test Repository\n\nThis is a test repository."
		SHA:         github.Ptr("abc123"),
		Size:        github.Ptr(42),
		HTMLURL:     github.Ptr("https://github.com/owner/repo/blob/main/README.md"),
		DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/README.md"),
	}

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
	}{
		{
			name: "successful file content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					expectQueryParams(t, map[string]string{
						"ref": "main",
					}).andThen(
						mockResponse(t, http.StatusOK, mockFileContent),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"path":   "README.md",
				"branch": "main",
			},
			expectError:    false,
			expectedResult: mockFileContent,
		},
		{
			name: "successful directory content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					expectQueryParams(t, map[string]string{}).andThen(
						mockResponse(t, http.StatusOK, mockDirContent),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"path":  "src",
			},
			expectError:    false,
			expectedResult: mockDirContent,
		},
		{
			name: "content fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"path":   "nonexistent.md",
				"branch": "main",
			},
			expectError:    true,
			expectedErrMsg: "failed to get file contents",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetFileContents(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Verify based on expected type
			switch expected := tc.expectedResult.(type) {
			case *github.RepositoryContent:
				var returnedContent github.RepositoryContent
				err = json.Unmarshal([]byte(textContent.Text), &returnedContent)
				require.NoError(t, err)
				assert.Equal(t, *expected.Name, *returnedContent.Name)
				assert.Equal(t, *expected.Path, *returnedContent.Path)
				assert.Equal(t, *expected.Type, *returnedContent.Type)
			case []*github.RepositoryContent:
				var returnedContents []*github.RepositoryContent
				err = json.Unmarshal([]byte(textContent.Text), &returnedContents)
				require.NoError(t, err)
				assert.Len(t, returnedContents, len(expected))
				for i, content := range returnedContents {
					assert.Equal(t, *expected[i].Name, *content.Name)
					assert.Equal(t, *expected[i].Path, *content.Path)
					assert.Equal(t, *expected[i].Type, *content.Type)
				}
			}
		})
	}
}

func Test_ForkRepository(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ForkRepository(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "fork_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "organization")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

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
			_, handler := ForkRepository(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			assert.Contains(t, textContent.Text, "Fork is in progress")
		})
	}
}

func Test_CreateBranch(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreateBranch(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_branch", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	assert.Contains(t, tool.InputSchema.Properties, "from_branch")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "branch"})

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
			_, handler := CreateBranch(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := GetCommit(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_commit", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "sha")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "sha"})

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
			_, handler := GetCommit(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListCommits(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_commits", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "sha")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

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
				Login: github.Ptr("testuser"),
			},
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/abc123def456"),
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
				Login: github.Ptr("anotheruser"),
			},
			HTMLURL: github.Ptr("https://github.com/owner/repo/commit/def456abc789"),
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
						"sha":      "main",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockCommits),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"sha":   "main",
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
			_, handler := ListCommits(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
			var returnedCommits []*github.RepositoryCommit
			err = json.Unmarshal([]byte(textContent.Text), &returnedCommits)
			require.NoError(t, err)
			assert.Len(t, returnedCommits, len(tc.expectedCommits))
			for i, commit := range returnedCommits {
				assert.Equal(t, *tc.expectedCommits[i].SHA, *commit.SHA)
				assert.Equal(t, *tc.expectedCommits[i].Commit.Message, *commit.Commit.Message)
				assert.Equal(t, *tc.expectedCommits[i].Author.Login, *commit.Author.Login)
				assert.Equal(t, *tc.expectedCommits[i].HTMLURL, *commit.HTMLURL)
			}
		})
	}
}

func Test_CreateOrUpdateFile(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreateOrUpdateFile(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_or_update_file", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "path")
	assert.Contains(t, tool.InputSchema.Properties, "content")
	assert.Contains(t, tool.InputSchema.Properties, "message")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	assert.Contains(t, tool.InputSchema.Properties, "sha")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "path", "content", "message", "branch"})

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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreateOrUpdateFile(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := CreateRepository(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_repository", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.Contains(t, tool.InputSchema.Properties, "description")
	assert.Contains(t, tool.InputSchema.Properties, "private")
	assert.Contains(t, tool.InputSchema.Properties, "autoInit")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"name"})

	// Setup mock repository response
	mockRepo := &github.Repository{
		Name:        github.Ptr("test-repo"),
		Description: github.Ptr("Test repository"),
		Private:     github.Ptr(true),
		HTMLURL:     github.Ptr("https://github.com/testuser/test-repo"),
		CloneURL:    github.Ptr("https://github.com/testuser/test-repo.git"),
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
			_, handler := CreateRepository(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
			var returnedRepo github.Repository
			err = json.Unmarshal([]byte(textContent.Text), &returnedRepo)
			assert.NoError(t, err)

			// Verify repository details
			assert.Equal(t, *tc.expectedRepo.Name, *returnedRepo.Name)
			assert.Equal(t, *tc.expectedRepo.Description, *returnedRepo.Description)
			assert.Equal(t, *tc.expectedRepo.Private, *returnedRepo.Private)
			assert.Equal(t, *tc.expectedRepo.HTMLURL, *returnedRepo.HTMLURL)
			assert.Equal(t, *tc.expectedRepo.Owner.Login, *returnedRepo.Owner.Login)
		})
	}
}

func Test_PushFiles(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := PushFiles(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "push_files", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	assert.Contains(t, tool.InputSchema.Properties, "files")
	assert.Contains(t, tool.InputSchema.Properties, "message")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "branch", "files", "message"})

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
			expectError:    true,
			expectedErrMsg: "failed to get branch reference",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := PushFiles(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			if tc.expectedErrMsg != "" {
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListBranches(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_branches", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

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
			_, handler := ListBranches(stubGetClientFn(mockClient), translations.NullTranslationHelper)

			// Create request
			request := createMCPRequest(tt.args)

			// Call handler
			result, err := handler(context.Background(), request)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
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
	mockClient := github.NewClient(nil)
	tool, _ := DeleteFile(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "delete_file", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "path")
	assert.Contains(t, tool.InputSchema.Properties, "message")
	assert.Contains(t, tool.InputSchema.Properties, "branch")
	// SHA is no longer required since we're using Git Data API
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "path", "message", "branch"})

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
			_, handler := DeleteFile(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListTags(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_tags", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

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
			_, handler := ListTags(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

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
	mockClient := github.NewClient(nil)
	tool, _ := GetTag(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_tag", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "tag")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "tag"})

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
			_, handler := GetTag(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

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
