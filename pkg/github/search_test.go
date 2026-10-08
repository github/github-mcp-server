package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SearchRepositories(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchRepositories(translations.NullTranslationHelper)
	tool := serverTool.Tool

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
		Total:             new(2),
		IncompleteResults: new(false),
		Repositories: []*github.Repository{
			{
				ID:              new(int64(12345)),
				Name:            new("repo-1"),
				FullName:        new("owner/repo-1"),
				HTMLURL:         new("https://github.com/owner/repo-1"),
				Description:     new("Test repository 1"),
				StargazersCount: new(100),
			},
			{
				ID:              new(int64(67890)),
				Name:            new("repo-2"),
				FullName:        new("owner/repo-2"),
				HTMLURL:         new("https://github.com/owner/repo-2"),
				Description:     new("Test repository 2"),
				StargazersCount: new(50),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.RepositoriesSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful repository search",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchRepositories: expectQueryParams(t, map[string]string{
					"q":        "golang test",
					"sort":     "stars",
					"order":    "desc",
					"page":     "2",
					"per_page": "10",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchRepositories: expectQueryParams(t, map[string]string{
					"q":        "golang test",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "golang test",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchRepositories: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Invalid query"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search repositories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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

func Test_SearchRepositories_IFC_InsidersMode(t *testing.T) {
	t.Parallel()

	serverTool := SearchRepositories(translations.NullTranslationHelper)

	type repoFixture struct {
		owner     string
		name      string
		isPrivate bool
	}

	makeRepo := func(r repoFixture) *github.Repository {
		return &github.Repository{
			ID:       new(int64(1)),
			Name:     new(r.name),
			FullName: new(r.owner + "/" + r.name),
			Private:  new(r.isPrivate),
			Owner:    &github.User{Login: new(r.owner)},
		}
	}

	makeMockClient := func(repos []repoFixture) *http.Client {
		searchResult := &github.RepositoriesSearchResult{
			Total:             new(len(repos)),
			IncompleteResults: new(false),
		}
		for _, r := range repos {
			searchResult.Repositories = append(searchResult.Repositories, makeRepo(r))
		}
		return MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetSearchRepositories: mockResponse(t, http.StatusOK, searchResult),
		})
	}

	reqParams := map[string]any{"query": "octocat"}

	t.Run("insiders mode disabled omits ifc label", func(t *testing.T) {
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient([]repoFixture{{owner: "octocat", name: "public-repo"}})),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Nil(t, result.Meta)
	})

	t.Run("insiders mode all public emits public untrusted", func(t *testing.T) {
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient([]repoFixture{
				{owner: "octocat", name: "public-a"},
				{owner: "octocat", name: "public-b"},
			})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
	})

	t.Run("insiders mode mixed public and private emits private untrusted", func(t *testing.T) {
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient([]repoFixture{
				{owner: "octocat", name: "private-repo", isPrivate: true},
				{owner: "octocat", name: "public-repo"},
			})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("insiders mode empty results emits public untrusted", func(t *testing.T) {
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(nil)),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
	})
}

func Test_SearchRepositories_FullOutput(t *testing.T) {
	mockSearchResult := &github.RepositoriesSearchResult{
		Total:             new(1),
		IncompleteResults: new(false),
		Repositories: []*github.Repository{
			{
				ID:              new(int64(12345)),
				Name:            new("test-repo"),
				FullName:        new("owner/test-repo"),
				HTMLURL:         new("https://github.com/owner/test-repo"),
				Description:     new("Test repository"),
				StargazersCount: new(100),
			},
		},
	}

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchRepositories: expectQueryParams(t, map[string]string{
			"q":        "golang test",
			"page":     "1",
			"per_page": "30",
		}).andThen(
			mockResponse(t, http.StatusOK, mockSearchResult),
		),
	})

	client := mustNewGHClient(t, mockedClient)
	serverTool := SearchRepositories(translations.NullTranslationHelper)
	deps := BaseDeps{
		Client: client,
	}
	handler := serverTool.Handler(deps)

	args := map[string]any{
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
	assert.Contains(t, schema.Properties, "fields")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

	// Setup mock search results
	mockSearchResult := &github.CodeSearchResult{
		Total:             new(2),
		IncompleteResults: new(false),
		CodeResults: []*github.CodeResult{
			{
				Name: new("file1.go"),
				Path: new("path/to/file1.go"),
				SHA:  new("abc123def456"),
				Repository: &github.Repository{
					Name:     new("repo"),
					FullName: new("owner/repo"),
				},
				TextMatches: []*github.TextMatch{
					{
						Fragment: new("func main() { fmt.Println(\"hello\") }"),
					},
				},
			},
			{
				Name: new("file2.go"),
				Path: new("path/to/file2.go"),
				SHA:  new("def456abc123"),
				Repository: &github.Repository{
					Name:     new("repo"),
					FullName: new("owner/repo"),
				},
			},
		},
	}

	textMatchAcceptHeader := map[string]string{
		"Accept": "text-match",
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.CodeSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful code search with all parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchCode: expectQueryParams(t, map[string]string{
					"q":        "fmt.Println language:go",
					"sort":     "indexed",
					"order":    "desc",
					"page":     "1",
					"per_page": "30",
				}).withHeaders(textMatchAcceptHeader).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchCode: expectQueryParams(t, map[string]string{
					"q":        "fmt.Println language:go",
					"page":     "1",
					"per_page": "30",
				}).withHeaders(textMatchAcceptHeader).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "fmt.Println language:go",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search code fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchCode: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search code",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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

			textContent := getTextResult(t, result)

			var returnedResult MinimalCodeSearchResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, returnedResult.TotalCount)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Items, len(tc.expectedResult.CodeResults))
			for i, code := range returnedResult.Items {
				assert.Equal(t, tc.expectedResult.CodeResults[i].GetName(), code.Name)
				assert.Equal(t, tc.expectedResult.CodeResults[i].GetPath(), code.Path)
				assert.Equal(t, tc.expectedResult.CodeResults[i].GetSHA(), code.SHA)
				assert.Equal(t, tc.expectedResult.CodeResults[i].Repository.GetFullName(), code.Repository)
			}

			// Verify text matches are included when present
			if len(tc.expectedResult.CodeResults[0].TextMatches) > 0 {
				require.NotEmpty(t, returnedResult.Items[0].TextMatches)
				assert.Equal(t,
					tc.expectedResult.CodeResults[0].TextMatches[0].GetFragment(),
					returnedResult.Items[0].TextMatches[0].GetFragment(),
				)
			}
		})
	}
}

func Test_SearchCode_FieldFiltering(t *testing.T) {
	mockSearchResult := &github.CodeSearchResult{
		Total:             new(1),
		IncompleteResults: new(false),
		CodeResults: []*github.CodeResult{
			{
				Name: new("file1.go"),
				Path: new("path/to/file1.go"),
				SHA:  new("abc123def456"),
				Repository: &github.Repository{
					Name:     new("repo"),
					FullName: new("owner/repo"),
				},
				TextMatches: []*github.TextMatch{
					{Fragment: new("func main() {}")},
				},
			},
		},
	}

	serverTool := SearchCode(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchCode: mockResponse(t, http.StatusOK, mockSearchResult),
	}))
	deps := BaseDeps{Client: client}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"query":  "fmt.Println language:go",
		"fields": []any{"name", "path"},
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)

	// The wrapper metadata is preserved while each item is reduced to the
	// requested fields only; the heavier repository and text_matches data is
	// dropped.
	var returned struct {
		TotalCount        int              `json:"total_count"`
		IncompleteResults bool             `json:"incomplete_results"`
		Items             []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returned))
	assert.Equal(t, 1, returned.TotalCount)
	require.Len(t, returned.Items, 1)
	require.Len(t, returned.Items[0], 2)
	assert.Contains(t, returned.Items[0], "name")
	assert.Contains(t, returned.Items[0], "path")

	assert.NotContains(t, textContent.Text, "repository")
	assert.NotContains(t, textContent.Text, "text_matches")
}

func Test_SearchCode_FieldsTelemetry(t *testing.T) {
	mockSearchResult := &github.CodeSearchResult{
		Total:             new(1),
		IncompleteResults: new(false),
		CodeResults: []*github.CodeResult{
			{
				Name: new("file1.go"),
				Path: new("path/to/file1.go"),
				SHA:  new("abc123def456"),
				Repository: &github.Repository{
					Name:     new("repo"),
					FullName: new("owner/repo"),
				},
				TextMatches: []*github.TextMatch{
					{Fragment: new("func main() {}")},
				},
			},
		},
	}

	serverTool := SearchCode(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchCode: mockResponse(t, http.StatusOK, mockSearchResult),
	}))

	t.Run("filtered call records savings", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{Client: client})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"query":  "fmt.Println language:go",
			"fields": []any{"name", "path"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		call, ok := rec.increment(metricFieldsToolCall)
		require.True(t, ok)
		assert.Equal(t, "search_code", call.tags["tool"])
		assert.Equal(t, "true", call.tags["filtered"])

		full, ok := rec.counter(metricFieldsBytesFull)
		require.True(t, ok)
		sent, ok := rec.counter(metricFieldsBytesSent)
		require.True(t, ok)
		assert.Greater(t, full.value, sent.value, "filtering should remove bytes")
	})

	t.Run("unfiltered call records adoption only", func(t *testing.T) {
		deps, rec := depsWithRecordingMetrics(t, BaseDeps{Client: client})
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{
			"query": "fmt.Println language:go",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		call, ok := rec.increment(metricFieldsToolCall)
		require.True(t, ok)
		assert.Equal(t, "false", call.tags["filtered"])

		_, ok = rec.counter(metricFieldsBytesFull)
		assert.False(t, ok, "no byte counters when not filtered")
	})
}

func Test_SearchUsers(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchUsers(translations.NullTranslationHelper)
	tool := serverTool.Tool

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
		Total:             new(2),
		IncompleteResults: new(false),
		Users: []*github.User{
			{
				Login:     new("user1"),
				ID:        new(int64(1001)),
				HTMLURL:   new("https://github.com/user1"),
				AvatarURL: new("https://avatars.githubusercontent.com/u/1001"),
			},
			{
				Login:     new("user2"),
				ID:        new(int64(1002)),
				HTMLURL:   new("https://github.com/user2"),
				AvatarURL: new("https://avatars.githubusercontent.com/u/1002"),
				Type:      new("User"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.UsersSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful users search with all parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:user location:finland language:go",
					"sort":     "followers",
					"order":    "desc",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
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
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:user location:finland language:go",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "location:finland language:go",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing type:user filter - no duplication",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:user location:seattle followers:>100",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "type:user location:seattle followers:>100",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with existing type:user filter and OR operators",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:user (location:seattle OR location:california) followers:>50",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "type:user (location:seattle OR location:california) followers:>50",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search users fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search users",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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
		Total:             new(int(2)),
		IncompleteResults: new(false),
		Users: []*github.User{
			{
				Login:     new("org-1"),
				ID:        new(int64(111)),
				HTMLURL:   new("https://github.com/org-1"),
				AvatarURL: new("https://avatars.githubusercontent.com/u/111?v=4"),
			},
			{
				Login:     new("org-2"),
				ID:        new(int64(222)),
				HTMLURL:   new("https://github.com/org-2"),
				AvatarURL: new("https://avatars.githubusercontent.com/u/222?v=4"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.UsersSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful org search",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:org github",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "github",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing type:org filter - no duplication",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:org location:california followers:>1000",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "type:org location:california followers:>1000",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with existing type:org filter and OR operators",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: expectQueryParams(t, map[string]string{
					"q":        "type:org (location:seattle OR location:california OR location:newyork) repos:>10",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "type:org (location:seattle OR location:california OR location:newyork) repos:>10",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "org search fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchUsers: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search orgs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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

func Test_SearchCommits(t *testing.T) {
	serverTool := SearchCommits(translations.NullTranslationHelper)
	tool := serverTool.Tool

	assert.Equal(t, "search_commits", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

	now := time.Now().Truncate(time.Second)
	mockSearchResult := &github.CommitsSearchResult{
		Total:             new(2),
		IncompleteResults: new(false),
		Commits: []*github.CommitResult{
			{
				SHA:     new("abc123commit"),
				HTMLURL: new("https://github.com/owner/repo/commit/abc123commit"),
				Commit: &github.Commit{
					Message: new("Initial commit"),
					Author: &github.CommitAuthor{
						Name:  new("Author Name"),
						Email: new("author@example.com"),
						Date:  &github.Timestamp{Time: now},
					},
				},
				Author: &github.User{
					Login:   new("author"),
					ID:      new(int64(1)),
					HTMLURL: new("https://github.com/author"),
				},
				Repository: &github.Repository{
					FullName: new("owner/repo"),
					HTMLURL:  new("https://github.com/owner/repo"),
					Private:  new(false),
				},
			},
			{
				// Commit with no resolved GitHub user for author or committer
				// (common when the commit email isn't linked to an account).
				SHA:     new("def456commit"),
				HTMLURL: new("https://github.com/owner/repo/commit/def456commit"),
				Commit: &github.Commit{
					Message: new("Unlinked author"),
				},
				Repository: &github.Repository{
					FullName: new("owner/repo"),
				},
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.CommitsSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful commit search",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchCommits: expectQueryParams(t, map[string]string{
					"q":        "fix bug in:message repo:owner/repo",
					"sort":     "author-date",
					"order":    "desc",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "fix bug in:message repo:owner/repo",
				"sort":  "author-date",
				"order": "desc",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchCommits: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:syntax",
			},
			expectError:    true,
			expectedErrMsg: "failed to search commits",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			textContent := getTextResult(t, result)
			var returnedResult MinimalSearchCommitsResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedResult.GetTotal(), returnedResult.TotalCount)
			assert.Len(t, returnedResult.Items, len(tc.expectedResult.Commits))
			assert.Equal(t, *tc.expectedResult.Commits[0].SHA, returnedResult.Items[0].SHA)
			assert.Equal(t, *tc.expectedResult.Commits[0].Commit.Message, returnedResult.Items[0].Commit.Message)
			assert.Equal(t, *tc.expectedResult.Commits[0].Commit.Author.Name, returnedResult.Items[0].Commit.Author.Name)
			assert.Equal(t, now.Format(time.RFC3339), returnedResult.Items[0].Commit.Author.Date)
			assert.Equal(t, *tc.expectedResult.Commits[0].Author.Login, returnedResult.Items[0].Author.Login)

			// Repository info is required so callers can identify which repo
			// each cross-repo search result belongs to.
			require.NotNil(t, returnedResult.Items[0].Repository)
			assert.Equal(t, "owner/repo", returnedResult.Items[0].Repository.FullName)
			assert.Equal(t, "https://github.com/owner/repo", returnedResult.Items[0].Repository.HTMLURL)

			// Second commit has no resolved GitHub user for author/committer
			// and no commit-level author block — the handler must not panic
			// and must omit those fields cleanly.
			require.Len(t, returnedResult.Items, 2)
			assert.Equal(t, "def456commit", returnedResult.Items[1].SHA)
			assert.Nil(t, returnedResult.Items[1].Author)
			assert.Nil(t, returnedResult.Items[1].Committer)
			require.NotNil(t, returnedResult.Items[1].Commit)
			assert.Nil(t, returnedResult.Items[1].Commit.Author)
			assert.Nil(t, returnedResult.Items[1].Commit.Committer)
		})
	}
}
