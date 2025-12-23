package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SearchRepositories(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchRepositories(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

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
						"sort":     "stars",
						"order":    "desc",
						"page":     "2",
						"per_page": "10",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query":   "golang test",
				"sort":    "stars",
				"order":   "desc",
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
			var returnedResult MinimalSearchRepositoriesResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, returnedResult.TotalCount)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Items, len(tc.expectedResult.Repositories))
			for i, repo := range returnedResult.Items {
				assert.Equal(t, *tc.expectedResult.Repositories[i].ID, repo.ID)
				assert.Equal(t, *tc.expectedResult.Repositories[i].Name, repo.Name)
				assert.Equal(t, *tc.expectedResult.Repositories[i].FullName, repo.FullName)
				assert.Equal(t, *tc.expectedResult.Repositories[i].HTMLURL, repo.HTMLURL)
			}

		})
	}
}

func Test_SearchRepositories_FullOutput(t *testing.T) {
	mockSearchResult := &github.RepositoriesSearchResult{
		Total:             github.Ptr(1),
		IncompleteResults: github.Ptr(false),
		Repositories: []*github.Repository{
			{
				ID:              github.Ptr(int64(12345)),
				Name:            github.Ptr("test-repo"),
				FullName:        github.Ptr("owner/test-repo"),
				HTMLURL:         github.Ptr("https://github.com/owner/test-repo"),
				Description:     github.Ptr("Test repository"),
				StargazersCount: github.Ptr(100),
			},
		},
	}

	mockedClient := mock.NewMockedHTTPClient(
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
	)

	client := github.NewClient(mockedClient)
	serverTool := SearchRepositories(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client: client,
	}
	handler := serverTool.Handler(deps)

	args := map[string]interface{}{
		"query":          "golang test",
		"minimal_output": false,
	}

	request := createMCPRequest(args)

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// Unmarshal as full GitHub API response
	var returnedResult github.RepositoriesSearchResult
	err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
	require.NoError(t, err)

	// Verify it's the full API response, not minimal
	assert.Equal(t, *mockSearchResult.Total, *returnedResult.Total)
	assert.Equal(t, *mockSearchResult.IncompleteResults, *returnedResult.IncompleteResults)
	assert.Len(t, returnedResult.Repositories, 1)
	assert.Equal(t, *mockSearchResult.Repositories[0].ID, *returnedResult.Repositories[0].ID)
	assert.Equal(t, *mockSearchResult.Repositories[0].Name, *returnedResult.Repositories[0].Name)
}

func Test_SearchCode(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchCode(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_code", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "page")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

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
				"query":   "fmt.Println language:go",
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
				"query": "fmt.Println language:go",
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
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search code",
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

func Test_SearchUsers(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchUsers(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_users", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "page")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

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
				"query":   "location:finland language:go",
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
				"query": "location:finland language:go",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing type:user filter - no duplication",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:user location:seattle followers:>100",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "type:user location:seattle followers:>100",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with existing type:user filter and OR operators",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:user (location:seattle OR location:california) followers:>50",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "type:user (location:seattle OR location:california) followers:>50",
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
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search users",
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

func Test_SearchOrgs(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchOrgs(translations.NullTranslationHelper)
	tool := serverTool.Tool

	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_orgs", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "page")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

	// Setup mock search results
	mockSearchResult := &github.UsersSearchResult{
		Total:             github.Ptr(int(2)),
		IncompleteResults: github.Ptr(false),
		Users: []*github.User{
			{
				Login:     github.Ptr("org-1"),
				ID:        github.Ptr(int64(111)),
				HTMLURL:   github.Ptr("https://github.com/org-1"),
				AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/111?v=4"),
			},
			{
				Login:     github.Ptr("org-2"),
				ID:        github.Ptr(int64(222)),
				HTMLURL:   github.Ptr("https://github.com/org-2"),
				AvatarURL: github.Ptr("https://avatars.githubusercontent.com/u/222?v=4"),
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
			name: "successful org search",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:org github",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "github",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing type:org filter - no duplication",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:org location:california followers:>1000",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "type:org location:california followers:>1000",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with existing type:org filter and OR operators",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchUsers,
					expectQueryParams(t, map[string]string{
						"q":        "type:org (location:seattle OR location:california OR location:newyork) repos:>10",
						"page":     "1",
						"per_page": "30",
					}).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "type:org (location:seattle OR location:california OR location:newyork) repos:>10",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "org search fails",
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
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search orgs",
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
			require.NotNil(t, result)

			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedResult MinimalSearchUsersResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, returnedResult.TotalCount)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Items, len(tc.expectedResult.Users))
			for i, org := range returnedResult.Items {
				assert.Equal(t, *tc.expectedResult.Users[i].Login, org.Login)
				assert.Equal(t, *tc.expectedResult.Users[i].ID, org.ID)
				assert.Equal(t, *tc.expectedResult.Users[i].HTMLURL, org.ProfileURL)
				assert.Equal(t, *tc.expectedResult.Users[i].AvatarURL, org.AvatarURL)
			}
		})
	}
}
