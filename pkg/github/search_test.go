package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v76/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SearchRepositories(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchRepositories(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_repositories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "cursor")
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
						"sort":     "stars",
						"order":    "desc",
						"page":     "2",
						"per_page": "11",
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
						"per_page": "11",
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

			// Unmarshal paginated search result (includes totalCount, items, moreData, cursor)
			var returnedResult map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			
			// Check totalCount
			if totalCount, ok := returnedResult["totalCount"].(float64); ok {
				assert.Equal(t, float64(*tc.expectedResult.Total), totalCount)
			}
			if incompleteResults, ok := returnedResult["incompleteResults"].(bool); ok {
				assert.Equal(t, *tc.expectedResult.IncompleteResults, incompleteResults)
			}
			
			// Extract items from paginated response
			items, ok := returnedResult["items"].([]interface{})
			require.True(t, ok, "items should be present in response")
			assert.Len(t, items, len(tc.expectedResult.Repositories))
			
			// Convert items to MinimalRepository for comparison
			for i, item := range items {
				if i >= len(tc.expectedResult.Repositories) {
					break
				}
				itemMap, ok := item.(map[string]interface{})
				require.True(t, ok)
				if id, ok := itemMap["id"].(float64); ok {
					assert.Equal(t, float64(*tc.expectedResult.Repositories[i].ID), id)
				}
				if name, ok := itemMap["name"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Repositories[i].Name, name)
				}
				if fullName, ok := itemMap["fullName"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Repositories[i].FullName, fullName)
				}
				if htmlURL, ok := itemMap["htmlUrl"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Repositories[i].HTMLURL, htmlURL)
				}
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
				"per_page": "11",
			}).andThen(
				mockResponse(t, http.StatusOK, mockSearchResult),
			),
		),
	)

	client := github.NewClient(mockedClient)
	_, handlerTest := SearchRepositories(stubGetClientFn(client), translations.NullTranslationHelper)

	request := createMCPRequest(map[string]interface{}{
		"query":          "golang test",
		"minimal_output": false,
	})

	result, err := handlerTest(context.Background(), request)

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// Unmarshal paginated search result (full output still includes pagination metadata)
	var returnedResult map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
	require.NoError(t, err)
	
	// Verify it's the full API response, not minimal
	// Note: returnedResult is now a map with pagination metadata
	if totalCount, ok := returnedResult["totalCount"].(float64); ok {
		assert.Equal(t, float64(*mockSearchResult.Total), totalCount)
	}
	if incompleteResults, ok := returnedResult["incompleteResults"].(bool); ok {
		assert.Equal(t, *mockSearchResult.IncompleteResults, incompleteResults)
	}
	
	// Extract repositories from items array
	items, ok := returnedResult["items"].([]interface{})
	require.True(t, ok, "items should be present in response")
	assert.Len(t, items, 1)
	
	// Convert first item to map and verify
	repoMap, ok := items[0].(map[string]interface{})
	require.True(t, ok)
	if id, ok := repoMap["id"].(float64); ok {
		assert.Equal(t, float64(*mockSearchResult.Repositories[0].ID), id)
	}
	if name, ok := repoMap["name"].(string); ok {
		assert.Equal(t, *mockSearchResult.Repositories[0].Name, name)
	}
}

func Test_SearchCode(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchCode(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_code", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "cursor")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"query"})

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
						"per_page": "11",
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
						"per_page": "11",
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
			_, handler := SearchCode(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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

			// Unmarshal paginated search result
			var returnedResult map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			
			// Check totalCount and incompleteResults
			if totalCount, ok := returnedResult["totalCount"].(float64); ok {
				assert.Equal(t, float64(*tc.expectedResult.Total), totalCount)
			}
			if incompleteResults, ok := returnedResult["incompleteResults"].(bool); ok {
				assert.Equal(t, *tc.expectedResult.IncompleteResults, incompleteResults)
			}
			
			// Extract items (CodeResults array)
			items, ok := returnedResult["items"].([]interface{})
			require.True(t, ok)
			assert.Len(t, items, len(tc.expectedResult.CodeResults))
			
			// Convert items array to CodeResult slice for comparison
			itemsBytes, err := json.Marshal(items)
			require.NoError(t, err)
			var codeResults []*github.CodeResult
			err = json.Unmarshal(itemsBytes, &codeResults)
			require.NoError(t, err)
			
			for i, code := range codeResults {
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
	mockClient := github.NewClient(nil)
	tool, _ := SearchUsers(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_users", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "cursor")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"query"})

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
						"per_page": "11",
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
						"per_page": "11",
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
						"per_page": "11",
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
						"per_page": "11",
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
			_, handler := SearchUsers(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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

			// Unmarshal paginated search result
			var returnedResult map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			
			// Check totalCount and incompleteResults
			if totalCount, ok := returnedResult["totalCount"].(float64); ok {
				assert.Equal(t, float64(*tc.expectedResult.Total), totalCount)
			}
			if incompleteResults, ok := returnedResult["incompleteResults"].(bool); ok {
				assert.Equal(t, *tc.expectedResult.IncompleteResults, incompleteResults)
			}
			
			// Extract items
			items, ok := returnedResult["items"].([]interface{})
			require.True(t, ok)
			assert.Len(t, items, len(tc.expectedResult.Users))
			
			// Convert items to MinimalUser for comparison
			for i, item := range items {
				if i >= len(tc.expectedResult.Users) {
					break
				}
				itemMap, ok := item.(map[string]interface{})
				require.True(t, ok)
				if login, ok := itemMap["login"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].Login, login)
				}
				if id, ok := itemMap["id"].(float64); ok {
					assert.Equal(t, float64(*tc.expectedResult.Users[i].ID), id)
				}
				if profileURL, ok := itemMap["profileURL"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].HTMLURL, profileURL)
				}
				if avatarURL, ok := itemMap["avatarURL"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].AvatarURL, avatarURL)
				}
			}
		})
	}
}

func Test_SearchOrgs(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := SearchOrgs(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "search_orgs", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "order")
	assert.Contains(t, tool.InputSchema.Properties, "cursor")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"query"})

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
						"per_page": "11",
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
						"per_page": "11",
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
						"per_page": "11",
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
			_, handler := SearchOrgs(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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

			// Unmarshal paginated search result
			var returnedResult map[string]interface{}
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			
			// Check totalCount and incompleteResults
			if totalCount, ok := returnedResult["totalCount"].(float64); ok {
				assert.Equal(t, float64(*tc.expectedResult.Total), totalCount)
			}
			if incompleteResults, ok := returnedResult["incompleteResults"].(bool); ok {
				assert.Equal(t, *tc.expectedResult.IncompleteResults, incompleteResults)
			}
			
			// Extract items
			items, ok := returnedResult["items"].([]interface{})
			require.True(t, ok)
			assert.Len(t, items, len(tc.expectedResult.Users))
			
			// Convert items to MinimalUser for comparison
			for i, item := range items {
				if i >= len(tc.expectedResult.Users) {
					break
				}
				itemMap, ok := item.(map[string]interface{})
				require.True(t, ok)
				if login, ok := itemMap["login"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].Login, login)
				}
				if id, ok := itemMap["id"].(float64); ok {
					assert.Equal(t, float64(*tc.expectedResult.Users[i].ID), id)
				}
				if profileURL, ok := itemMap["profileURL"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].HTMLURL, profileURL)
				}
				if avatarURL, ok := itemMap["avatarURL"].(string); ok {
					assert.Equal(t, *tc.expectedResult.Users[i].AvatarURL, avatarURL)
				}
			}
		})
	}
}
