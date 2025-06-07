package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SearchRepositories(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchRepositories(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "search_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"query"})

	// Setup mock search results
	mockSearchResult := &github.RepositoriesSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		Repositories: []*github.Repository{
			{
				ID:              github.Ptr(int64(12345)),
				Name:            github.Ptr("repo-1"),
				FullName:        github.Ptr("owner/repo-1"),
				HTMLURL:         github.Ptr("https://github.com/owner/repo-1"),
				Description:     github.Ptr("Test repository 1"),
				StargazersCount: github.Ptr(100),
			},
			{
				ID:              github.Ptr(int64(67890)),
				Name:            github.Ptr("repo-2"),
				FullName:        github.Ptr("owner/repo-2"),
				HTMLURL:         github.Ptr("https://github.com/owner/repo-2"),
				Description:     github.Ptr("Test repository 2"),
				StargazersCount: github.Ptr(50),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.RepositoriesSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful repository search",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchRepositories,
					expectQueryParams(t, map[string]string{
						"q":        "golang test",
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query":   "golang test",
				"page":    float64(2),
				"perPage": float64(10),
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "repository search with default pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchRepositories,
					expectQueryParams(t, map[string]string{
						"q":        "golang test",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "golang test",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchRepositories,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Invalid query"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search repositories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := SearchRepositories(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult github.RepositoriesSearchResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, *returnedResult.Total)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, *returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Repositories, len(tc.expectedResult.Repositories))
			for i, repo := range returnedResult.Repositories {
				assert.Equal(t, *tc.expectedResult.Repositories[i].ID, *repo.ID)
				assert.Equal(t, *tc.expectedResult.Repositories[i].Name, *repo.Name)
				assert.Equal(t, *tc.expectedResult.Repositories[i].FullName, *repo.FullName)
				assert.Equal(t, *tc.expectedResult.Repositories[i].HTMLURL, *repo.HTMLURL)
			}

		})
	}
}

func Test_SearchCode(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchCode(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "search_code", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "q")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"q"})

	// Setup mock search results
	mockSearchResult := &github.CodeSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		CodeResults: []*github.CodeResult{
			{
				Name:       github.Ptr("file1.go"),
				Path:       github.Ptr("path/to/file1.go"),
				SHA:        github.Ptr("abc123def456"),
				HTMLURL:    github.Ptr("https://github.com/owner/repo/blob/main/path/to/file1.go"),
				Repository: &github.Repository{Name: github.Ptr("repo"), FullName: github.Ptr("owner/repo")},
			},
			{
				Name:       github.Ptr("file2.go"),
				Path:       github.Ptr("path/to/file2.go"),
				SHA:        github.Ptr("def456abc123"),
				HTMLURL:    github.Ptr("https://github.com/owner/repo/blob/main/path/to/file2.go"),
				Repository: &github.Repository{Name: github.Ptr("repo"), FullName: github.Ptr("owner/repo")},
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.CodeSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful code search with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchCode,
					expectQueryParams(t, map[string]string{
						"q":        "fmt.Println language:go",
						"sort":     "indexed",
						"order":    "desc",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"q":       "fmt.Println language:go",
				"sort":    "indexed",
				"order":   "desc",
				"page":    float64(1),
				"perPage": float64(30),
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "code search with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchCode,
					expectQueryParams(t, map[string]string{
						"q":        "fmt.Println language:go",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"q": "fmt.Println language:go",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search code fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchCode,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"q": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search code",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := SearchCode(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult github.CodeSearchResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, *returnedResult.Total)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, *returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.CodeResults, len(tc.expectedResult.CodeResults))
			for i, code := range returnedResult.CodeResults {
				assert.Equal(t, *tc.expectedResult.CodeResults[i].Name, *code.Name)
				assert.Equal(t, *tc.expectedResult.CodeResults[i].Path, *code.Path)
				assert.Equal(t, *tc.expectedResult.CodeResults[i].SHA, *code.SHA)
				assert.Equal(t, *tc.expectedResult.CodeResults[i].HTMLURL, *code.HTMLURL)
				assert.Equal(t, *tc.expectedResult.CodeResults[i].Repository.FullName, *code.Repository.FullName)
			}
		})
	}
}

func Test_ListStarredRepositories(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := ListStarredRepositories(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_starred_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "direction")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Empty(t, tool.InputSchema.Required) // No required parameters

	// Setup mock starred repositories results
	mockStarredRepos := []*github.StarredRepository{
		{
			StarredAt: &github.Timestamp{},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(12345)),
				Name:            github.Ptr("repo-1"),
				FullName:        github.Ptr("owner/repo-1"),
				HTMLURL:         github.Ptr("https://github.com/owner/repo-1"),
				Description:     github.Ptr("Test repository 1"),
				StargazersCount: github.Ptr(100),
				Language:        github.Ptr("Go"),
				Fork:            github.Ptr(false),
			},
		},
		{
			StarredAt: &github.Timestamp{},
			Repository: &github.Repository{
				ID:              github.Ptr(int64(67890)),
				Name:            github.Ptr("repo-2"),
				FullName:        github.Ptr("owner/repo-2"),
				HTMLURL:         github.Ptr("https://github.com/owner/repo-2"),
				Description:     github.Ptr("Test repository 2"),
				StargazersCount: github.Ptr(50),
				Language:        github.Ptr("JavaScript"),
				Fork:            github.Ptr(true),
			},
		},
	}

	type SimplifiedRepo struct {
		FullName        string `json:"full_name,omitempty"`
		HTMLURL         string `json:"html_url,omitempty"`
		Description     string `json:"description,omitempty"`
		StargazersCount int    `json:"stargazers_count,omitempty"`
		Language        string `json:"language,omitempty"`
	}

	type SimplifiedStarredRepo struct {
		StarredAt  *github.Timestamp `json:"starred_at,omitempty"`
		Repository SimplifiedRepo    `json:"repository,omitempty"`
	}

	expectedFilteredRepos := []SimplifiedStarredRepo{
		{
			StarredAt: &github.Timestamp{},
			Repository: SimplifiedRepo{
				FullName:        "owner/repo-1",
				HTMLURL:         "https://github.com/owner/repo-1",
				Description:     "Test repository 1",
				StargazersCount: 100,
				Language:        "Go",
			},
		},
		{
			StarredAt: &github.Timestamp{},
			Repository: SimplifiedRepo{
				FullName:        "owner/repo-2",
				HTMLURL:         "https://github.com/owner/repo-2",
				Description:     "Test repository 2",
				StargazersCount: 50,
				Language:        "JavaScript",
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult []SimplifiedStarredRepo
		expectedErrMsg string
	}{
		{
			name: "successful starred repositories list with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					expectQueryParams(t, map[string]string{
						"sort":      "created",
						"direction": "desc",
						"page":      "2",
						"per_page":  "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"sort":      "created",
				"direction": "desc",
				"page":      float64(2),
				"perPage":   float64(10),
			},
			expectError:    false,
			expectedResult: expectedFilteredRepos,
		},
		{
			name: "list starred repositories with default parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					expectQueryParams(t, map[string]string{
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    false,
			expectedResult: expectedFilteredRepos,
		},
		{
			name: "list starred repositories with sort only",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					expectQueryParams(t, map[string]string{
						"sort":     "updated",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockStarredRepos),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"sort": "updated",
			},
			expectError:    false,
			expectedResult: expectedFilteredRepos,
		},
		{
			name: "list starred repositories fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUserStarred,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Requires authentication"}`))
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
			_, handler := ListStarredRepositories(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult []SimplifiedStarredRepo
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Len(t, returnedResult, len(tc.expectedResult))

			for i, repo := range returnedResult {
				assert.Equal(t, tc.expectedResult[i].Repository.FullName, repo.Repository.FullName)
				assert.Equal(t, tc.expectedResult[i].Repository.HTMLURL, repo.Repository.HTMLURL)
				assert.Equal(t, tc.expectedResult[i].Repository.Description, repo.Repository.Description)
				assert.Equal(t, tc.expectedResult[i].Repository.StargazersCount, repo.Repository.StargazersCount)
				assert.Equal(t, tc.expectedResult[i].Repository.Language, repo.Repository.Language)
			}
		})
	}
}

func Test_SearchUsers(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchUsers(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "search_users", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "q")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"q"})

	// Setup mock search results
	mockSearchResult := &github.UsersSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		Users: []*github.User{
			{
				Login:     github.Ptr("user1"),
				ID:        github.Ptr(int64(1001)),
				HTMLURL:   github.Ptr("https://github.com/user1"),
				AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/1001"),
			},
			{
				Login:     github.Ptr("user2"),
				ID:        github.Ptr(int64(1002)),
				HTMLURL:   github.Ptr("https://github.com/user2"),
				AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/1002"),
				Type:      github.Ptr("User"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.UsersSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful users search with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:user location:finland language:go",
						"sort":     "followers",
						"order":    "desc",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"q":       "location:finland language:go",
				"sort":    "followers",
				"order":   "desc",
				"page":    float64(1),
				"perPage": float64(30),
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "users search with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:user location:finland language:go",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"q": "location:finland language:go",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search users fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"q": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search users",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := SearchUsers(stubGetClientFn(client), translations.NullTranslationHelper)

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
			require.NotNil(t, result)

			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedResult MinimalSearchUsersResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, returnedResult.TotalCount)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Items, len(tc.expectedResult.Users))
			for i, user := range returnedResult.Items {
				assert.Equal(t, *tc.expectedResult.Users[i].Login, user.Login)
				assert.Equal(t, *tc.expectedResult.Users[i].ID, user.ID)
				assert.Equal(t, *tc.expectedResult.Users[i].HTMLURL, user.ProfileURL)
				assert.Equal(t, *tc.expectedResult.Users[i].AvatarURL, user.AvatarURL)
			}
		})
	}
}
