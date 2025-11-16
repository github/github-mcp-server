package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListGists(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := ListGists(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_gists", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "username")
	assert.Contains(t, tool.InputSchema.Properties, "since")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Empty(t, tool.InputSchema.Required)

	// Setup mock gists for success case
	mockGists := []*github.Gist{
		{
			ID:          github.Ptr("gist1"),
			Description: github.Ptr("First Gist"),
			HTMLURL:     github.Ptr("https://gist.github.com/user/gist1"),
			Public:      github.Ptr(true),
			CreatedAt:   &github.Timestamp{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
			Owner:       &github.User{Login: github.Ptr("user")},
			Files: map[github.GistFilename]github.GistFile{
				"file1.txt": {
					Filename: github.Ptr("file1.txt"),
					Content:  github.Ptr("content of file 1"),
				},
			},
		},
		{
			ID:          github.Ptr("gist2"),
			Description: github.Ptr("Second Gist"),
			HTMLURL:     github.Ptr("https://gist.github.com/testuser/gist2"),
			Public:      github.Ptr(false),
			CreatedAt:   &github.Timestamp{Time: time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC)},
			Owner:       &github.User{Login: github.Ptr("testuser")},
			Files: map[github.GistFilename]github.GistFile{
				"file2.js": {
					Filename: github.Ptr("file2.js"),
					Content:  github.Ptr("console.log('hello');"),
				},
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedGists  []*github.Gist
		expectedErrMsg string
	}{
		{
			name: "list authenticated user's gists",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetGists,
					mockGists,
				),
			),
			requestArgs:   map[string]interface{}{},
			expectError:   false,
			expectedGists: mockGists,
		},
		{
			name: "list specific user's gists",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetUsersGistsByUsername,
					mockResponse(t, http.StatusOK, mockGists),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "testuser",
			},
			expectError:   false,
			expectedGists: mockGists,
		},
		{
			name: "list gists with pagination and since parameter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetGists,
					expectQueryParams(t, map[string]string{
						"since":    "2023-01-01T00:00:00Z",
						"page":     "2",
						"per_page": "5",
					}).andThen(
						mockResponse(t, http.StatusOK, mockGists),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"since":   "2023-01-01T00:00:00Z",
				"page":    float64(2),
				"perPage": float64(5),
			},
			expectError:   false,
			expectedGists: mockGists,
		},
		{
			name: "invalid since parameter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetGists,
					mockGists,
				),
			),
			requestArgs: map[string]interface{}{
				"since": "invalid-date",
			},
			expectError:    true,
			expectedErrMsg: "invalid since timestamp",
		},
		{
			name: "list gists fails with error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetGists,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Requires authentication"}`))
					}),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "failed to list gists",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListGists(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					// For errors returned as part of the result, not as an error
					assert.NotNil(t, result)
					textContent := getTextResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedGists []*github.Gist
			err = json.Unmarshal([]byte(textContent.Text), &returnedGists)
			require.NoError(t, err)

			assert.Len(t, returnedGists, len(tc.expectedGists))
			for i, gist := range returnedGists {
				assert.Equal(t, *tc.expectedGists[i].ID, *gist.ID)
				assert.Equal(t, *tc.expectedGists[i].Description, *gist.Description)
				assert.Equal(t, *tc.expectedGists[i].HTMLURL, *gist.HTMLURL)
				assert.Equal(t, *tc.expectedGists[i].Public, *gist.Public)
			}
		})
	}
}

func Test_GetGist(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := GetGist(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_gist", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "gist_id")

	assert.Contains(t, tool.InputSchema.Required, "gist_id")

	// Setup mock gist for success case
	mockGist := github.Gist{
		ID:          github.Ptr("gist1"),
		Description: github.Ptr("First Gist"),
		HTMLURL:     github.Ptr("https://gist.github.com/user/gist1"),
		Public:      github.Ptr(true),
		CreatedAt:   &github.Timestamp{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)},
		Owner:       &github.User{Login: github.Ptr("user")},
		Files: map[github.GistFilename]github.GistFile{
			github.GistFilename("file1.txt"): {
				Filename: github.Ptr("file1.txt"),
				Content:  github.Ptr("content of file 1"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedGists  github.Gist
		expectedErrMsg string
	}{
		{
			name: "Successful fetching different gist",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetGistsByGistId,
					mockResponse(t, http.StatusOK, mockGist),
				),
			),
			requestArgs: map[string]interface{}{
				"gist_id": "gist1",
			},
			expectError:   false,
			expectedGists: mockGist,
		},
		{
			name: "gist_id parameter missing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetGistsByGistId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Invalid Request"}`))
					}),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "missing required parameter: gist_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetGist(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					// For errors returned as part of the result, not as an error
					assert.NotNil(t, result)
					textContent := getTextResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedGists github.Gist
			err = json.Unmarshal([]byte(textContent.Text), &returnedGists)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedGists.ID, *returnedGists.ID)
			assert.Equal(t, *tc.expectedGists.Description, *returnedGists.Description)
			assert.Equal(t, *tc.expectedGists.HTMLURL, *returnedGists.HTMLURL)
			assert.Equal(t, *tc.expectedGists.Public, *returnedGists.Public)
		})
	}
}

func Test_CreateGist(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := CreateGist(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_gist", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "description")
	assert.Contains(t, tool.InputSchema.Properties, "filename")
	assert.Contains(t, tool.InputSchema.Properties, "content")
	assert.Contains(t, tool.InputSchema.Properties, "public")

	// Verify required parameters
	assert.Contains(t, tool.InputSchema.Required, "filename")
	assert.Contains(t, tool.InputSchema.Required, "content")

	// Setup mock data for test cases
	createdGist := &github.Gist{
		ID:          github.Ptr("new-gist-id"),
		Description: github.Ptr("Test Gist"),
		HTMLURL:     github.Ptr("https://gist.github.com/user/new-gist-id"),
		Public:      github.Ptr(false),
		CreatedAt:   &github.Timestamp{Time: time.Now()},
		Owner:       &github.User{Login: github.Ptr("user")},
		Files: map[github.GistFilename]github.GistFile{
			"test.go": {
				Filename: github.Ptr("test.go"),
				Content:  github.Ptr("package main\n\nfunc main() {\n\tfmt.Println(\"Hello, Gist!\")\n}"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		expectedGist   *github.Gist
	}{
		{
			name: "create gist successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostGists,
					mockResponse(t, http.StatusCreated, createdGist),
				),
			),
			requestArgs: map[string]interface{}{
				"filename":    "test.go",
				"content":     "package main\n\nfunc main() {\n\tfmt.Println(\"Hello, Gist!\")\n}",
				"description": "Test Gist",
				"public":      false,
			},
			expectError:  false,
			expectedGist: createdGist,
		},
		{
			name:         "missing required filename",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"content":     "test content",
				"description": "Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: filename",
		},
		{
			name:         "missing required content",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"filename":    "test.go",
				"description": "Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: content",
		},
		{
			name: "api returns error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostGists,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Requires authentication"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"filename":    "test.go",
				"content":     "package main",
				"description": "Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "failed to create gist",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreateGist(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					// For errors returned as part of the result, not as an error
					assert.NotNil(t, result)
					textContent := getTextResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var gist MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &gist)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedGist.GetHTMLURL(), gist.URL)
		})
	}
}

func Test_UpdateGist(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := UpdateGist(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "update_gist", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "gist_id")
	assert.Contains(t, tool.InputSchema.Properties, "description")
	assert.Contains(t, tool.InputSchema.Properties, "filename")
	assert.Contains(t, tool.InputSchema.Properties, "content")

	// Verify required parameters
	assert.Contains(t, tool.InputSchema.Required, "gist_id")
	assert.Contains(t, tool.InputSchema.Required, "filename")
	assert.Contains(t, tool.InputSchema.Required, "content")

	// Setup mock data for test cases
	updatedGist := &github.Gist{
		ID:          github.Ptr("existing-gist-id"),
		Description: github.Ptr("Updated Test Gist"),
		HTMLURL:     github.Ptr("https://gist.github.com/user/existing-gist-id"),
		Public:      github.Ptr(true),
		UpdatedAt:   &github.Timestamp{Time: time.Now()},
		Owner:       &github.User{Login: github.Ptr("user")},
		Files: map[github.GistFilename]github.GistFile{
			"updated.go": {
				Filename: github.Ptr("updated.go"),
				Content:  github.Ptr("package main\n\nfunc main() {\n\tfmt.Println(\"Updated Gist!\")\n}"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
		expectedGist   *github.Gist
	}{
		{
			name: "update gist successfully",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchGistsByGistId,
					mockResponse(t, http.StatusOK, updatedGist),
				),
			),
			requestArgs: map[string]interface{}{
				"gist_id":     "existing-gist-id",
				"filename":    "updated.go",
				"content":     "package main\n\nfunc main() {\n\tfmt.Println(\"Updated Gist!\")\n}",
				"description": "Updated Test Gist",
			},
			expectError:  false,
			expectedGist: updatedGist,
		},
		{
			name:         "missing required gist_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"filename":    "updated.go",
				"content":     "updated content",
				"description": "Updated Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: gist_id",
		},
		{
			name:         "missing required filename",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"gist_id":     "existing-gist-id",
				"content":     "updated content",
				"description": "Updated Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: filename",
		},
		{
			name:         "missing required content",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"gist_id":     "existing-gist-id",
				"filename":    "updated.go",
				"description": "Updated Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: content",
		},
		{
			name: "api returns error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchGistsByGistId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"gist_id":     "nonexistent-gist-id",
				"filename":    "updated.go",
				"content":     "package main",
				"description": "Updated Test Gist",
			},
			expectError:    true,
			expectedErrMsg: "failed to update gist",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := UpdateGist(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					// For errors returned as part of the result, not as an error
					assert.NotNil(t, result)
					textContent := getTextResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var updateResp MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &updateResp)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedGist.GetHTMLURL(), updateResp.URL)
		})
	}
}
