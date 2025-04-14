package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetPullRequest(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequest(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR for success case
	mockPR := &github.PullRequest{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test PR"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		Head: &github.PullRequestBranch{
			SHA: github.Ptr("abcd1234"),
			Ref: github.Ptr("feature-branch"),
		},
		Base: &github.PullRequestBranch{
			Ref: github.Ptr("main"),
		},
		Body: github.Ptr("This is a test PR"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedPR     *github.PullRequest
		expectedErrMsg string
	}{
		{
			name: "successful PR fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockPR,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError: false,
			expectedPR:  mockPR,
		},
		{
			name: "PR fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequest(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedPR github.PullRequest
			err = json.Unmarshal([]byte(textContent.Text), &returnedPR)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedPR.Number, *returnedPR.Number)
			assert.Equal(t, *tc.expectedPR.Title, *returnedPR.Title)
			assert.Equal(t, *tc.expectedPR.State, *returnedPR.State)
			assert.Equal(t, *tc.expectedPR.HTMLURL, *returnedPR.HTMLURL)
		})
	}
}

func Test_UpdatePullRequest(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := UpdatePullRequest(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "update_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "title")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.Contains(t, tool.InputSchema.Properties, "base")
	assert.Contains(t, tool.InputSchema.Properties, "maintainer_can_modify")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR for success case
	mockUpdatedPR := &github.PullRequest{
		Number:              github.Ptr(42),
		Title:               github.Ptr("Updated Test PR Title"),
		State:               github.Ptr("open"),
		HTMLURL:             github.Ptr("https://github.com/owner/repo/pull/42"),
		Body:                github.Ptr("Updated test PR body."),
		MaintainerCanModify: github.Ptr(false),
		Base: &github.PullRequestBranch{
			Ref: github.Ptr("develop"),
		},
	}

	mockClosedPR := &github.PullRequest{
		Number: github.Ptr(42),
		Title:  github.Ptr("Test PR"),
		State:  github.Ptr("closed"), // State updated
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedPR     *github.PullRequest
		expectedErrMsg string
	}{
		{
			name: "successful PR update (title, body, base, maintainer_can_modify)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposPullsByOwnerByRepoByPullNumber,
					// Expect the flat string based on previous test failure output and API docs
					expectRequestBody(t, map[string]interface{}{
						"title":                 "Updated Test PR Title",
						"body":                  "Updated test PR body.",
						"base":                  "develop",
						"maintainer_can_modify": false,
					}).andThen(
						mockResponse(t, http.StatusOK, mockUpdatedPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":                 "owner",
				"repo":                  "repo",
				"pullNumber":            float64(42),
				"title":                 "Updated Test PR Title",
				"body":                  "Updated test PR body.",
				"base":                  "develop",
				"maintainer_can_modify": false,
			},
			expectError: false,
			expectedPR:  mockUpdatedPR,
		},
		{
			name: "successful PR update (state)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposPullsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"state": "closed",
					}).andThen(
						mockResponse(t, http.StatusOK, mockClosedPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"state":      "closed",
			},
			expectError: false,
			expectedPR:  mockClosedPR,
		},
		{
			name:         "no update parameters provided",
			mockedClient: mock.NewMockedHTTPClient(), // No API call expected
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				// No update fields
			},
			expectError:    false, // Error is returned in the result, not as Go error
			expectedErrMsg: "No update parameters provided",
		},
		{
			name: "PR update fails (API error)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposPullsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"title":      "Invalid Title Causing Error",
			},
			expectError:    true,
			expectedErrMsg: "failed to update pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := UpdatePullRequest(stubGetClientFn(client), translations.NullTranslationHelper)

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

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Check for expected error message within the result text
			if tc.expectedErrMsg != "" {
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Unmarshal and verify the successful result
			var returnedPR github.PullRequest
			err = json.Unmarshal([]byte(textContent.Text), &returnedPR)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedPR.Number, *returnedPR.Number)
			if tc.expectedPR.Title != nil {
				assert.Equal(t, *tc.expectedPR.Title, *returnedPR.Title)
			}
			if tc.expectedPR.Body != nil {
				assert.Equal(t, *tc.expectedPR.Body, *returnedPR.Body)
			}
			if tc.expectedPR.State != nil {
				assert.Equal(t, *tc.expectedPR.State, *returnedPR.State)
			}
			if tc.expectedPR.Base != nil && tc.expectedPR.Base.Ref != nil {
				assert.NotNil(t, returnedPR.Base)
				assert.Equal(t, *tc.expectedPR.Base.Ref, *returnedPR.Base.Ref)
			}
			if tc.expectedPR.MaintainerCanModify != nil {
				assert.Equal(t, *tc.expectedPR.MaintainerCanModify, *returnedPR.MaintainerCanModify)
			}
		})
	}
}

func Test_ListPullRequests(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := ListPullRequests(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_pull_requests", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.Contains(t, tool.InputSchema.Properties, "head")
	assert.Contains(t, tool.InputSchema.Properties, "base")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "direction")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	// Setup mock PRs for success case
	mockPRs := []*github.PullRequest{
		{
			Number:  github.Ptr(42),
			Title:   github.Ptr("First PR"),
			State:   github.Ptr("open"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		},
		{
			Number:  github.Ptr(43),
			Title:   github.Ptr("Second PR"),
			State:   github.Ptr("closed"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/43"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedPRs    []*github.PullRequest
		expectedErrMsg string
	}{
		{
			name: "successful PRs listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"state":     "all",
						"sort":      "created",
						"direction": "desc",
						"per_page":  "30",
						"page":      "1",
					}).andThen(
						mockResponse(t, http.StatusOK, mockPRs),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"state":     "all",
				"sort":      "created",
				"direction": "desc",
				"perPage":   float64(30),
				"page":      float64(1),
			},
			expectError: false,
			expectedPRs: mockPRs,
		},
		{
			name: "PRs listing fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Invalid request"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"state": "invalid",
			},
			expectError:    true,
			expectedErrMsg: "failed to list pull requests",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListPullRequests(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedPRs []*github.PullRequest
			err = json.Unmarshal([]byte(textContent.Text), &returnedPRs)
			require.NoError(t, err)
			assert.Len(t, returnedPRs, 2)
			assert.Equal(t, *tc.expectedPRs[0].Number, *returnedPRs[0].Number)
			assert.Equal(t, *tc.expectedPRs[0].Title, *returnedPRs[0].Title)
			assert.Equal(t, *tc.expectedPRs[0].State, *returnedPRs[0].State)
			assert.Equal(t, *tc.expectedPRs[1].Number, *returnedPRs[1].Number)
			assert.Equal(t, *tc.expectedPRs[1].Title, *returnedPRs[1].Title)
			assert.Equal(t, *tc.expectedPRs[1].State, *returnedPRs[1].State)
		})
	}
}

func Test_MergePullRequest(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := MergePullRequest(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "merge_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "commit_title")
	assert.Contains(t, tool.InputSchema.Properties, "commit_message")
	assert.Contains(t, tool.InputSchema.Properties, "merge_method")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock merge result for success case
	mockMergeResult := &github.PullRequestMergeResult{
		Merged:  github.Ptr(true),
		Message: github.Ptr("Pull Request successfully merged"),
		SHA:     github.Ptr("abcd1234efgh5678"),
	}

	tests := []struct {
		name                string
		mockedClient        *http.Client
		requestArgs         map[string]interface{}
		expectError         bool
		expectedMergeResult *github.PullRequestMergeResult
		expectedErrMsg      string
	}{
		{
			name: "successful merge",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposPullsMergeByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"commit_title":   "Merge PR #42",
						"commit_message": "Merging awesome feature",
						"merge_method":   "squash",
					}).andThen(
						mockResponse(t, http.StatusOK, mockMergeResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "owner",
				"repo":           "repo",
				"pullNumber":     float64(42),
				"commit_title":   "Merge PR #42",
				"commit_message": "Merging awesome feature",
				"merge_method":   "squash",
			},
			expectError:         false,
			expectedMergeResult: mockMergeResult,
		},
		{
			name: "merge fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposPullsMergeByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusMethodNotAllowed)
						_, _ = w.Write([]byte(`{"message": "Pull request cannot be merged"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to merge pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := MergePullRequest(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedResult github.PullRequestMergeResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedMergeResult.Merged, *returnedResult.Merged)
			assert.Equal(t, *tc.expectedMergeResult.Message, *returnedResult.Message)
			assert.Equal(t, *tc.expectedMergeResult.SHA, *returnedResult.SHA)
		})
	}
}

func Test_GetPullRequestFiles(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestFiles(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_files", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR files for success case
	mockFiles := []*github.CommitFile{
		{
			Filename:  github.Ptr("file1.go"),
			Status:    github.Ptr("modified"),
			Additions: github.Ptr(10),
			Deletions: github.Ptr(5),
			Changes:   github.Ptr(15),
			Patch:     github.Ptr("@@ -1,5 +1,10 @@"),
		},
		{
			Filename:  github.Ptr("file2.go"),
			Status:    github.Ptr("added"),
			Additions: github.Ptr(20),
			Deletions: github.Ptr(0),
			Changes:   github.Ptr(20),
			Patch:     github.Ptr("@@ -0,0 +1,20 @@"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedFiles  []*github.CommitFile
		expectedErrMsg string
	}{
		{
			name: "successful files fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsFilesByOwnerByRepoByPullNumber,
					mockFiles,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:   false,
			expectedFiles: mockFiles,
		},
		{
			name: "files fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsFilesByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestFiles(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedFiles []*github.CommitFile
			err = json.Unmarshal([]byte(textContent.Text), &returnedFiles)
			require.NoError(t, err)
			assert.Len(t, returnedFiles, len(tc.expectedFiles))
			for i, file := range returnedFiles {
				assert.Equal(t, *tc.expectedFiles[i].Filename, *file.Filename)
				assert.Equal(t, *tc.expectedFiles[i].Status, *file.Status)
				assert.Equal(t, *tc.expectedFiles[i].Additions, *file.Additions)
				assert.Equal(t, *tc.expectedFiles[i].Deletions, *file.Deletions)
			}
		})
	}
}

func Test_GetPullRequestStatus(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestStatus(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_status", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR for successful PR fetch
	mockPR := &github.PullRequest{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test PR"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		Head: &github.PullRequestBranch{
			SHA: github.Ptr("abcd1234"),
			Ref: github.Ptr("feature-branch"),
		},
	}

	// Setup mock status for success case
	mockStatus := &github.CombinedStatus{
		State:      github.Ptr("success"),
		TotalCount: github.Ptr(3),
		Statuses: []*github.RepoStatus{
			{
				State:       github.Ptr("success"),
				Context:     github.Ptr("continuous-integration/travis-ci"),
				Description: github.Ptr("Build succeeded"),
				TargetURL:   github.Ptr("https://travis-ci.org/owner/repo/builds/123"),
			},
			{
				State:       github.Ptr("success"),
				Context:     github.Ptr("codecov/patch"),
				Description: github.Ptr("Coverage increased"),
				TargetURL:   github.Ptr("https://codecov.io/gh/owner/repo/pull/42"),
			},
			{
				State:       github.Ptr("success"),
				Context:     github.Ptr("lint/golangci-lint"),
				Description: github.Ptr("No issues found"),
				TargetURL:   github.Ptr("https://golangci.com/r/owner/repo/pull/42"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedStatus *github.CombinedStatus
		expectedErrMsg string
	}{
		{
			name: "successful status fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockPR,
				),
				mock.WithRequestMatch(
					mock.GetReposCommitsStatusByOwnerByRepoByRef,
					mockStatus,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:    false,
			expectedStatus: mockStatus,
		},
		{
			name: "PR fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request",
		},
		{
			name: "status fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockPR,
				),
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsStatusesByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to get combined status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestStatus(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedStatus github.CombinedStatus
			err = json.Unmarshal([]byte(textContent.Text), &returnedStatus)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedStatus.State, *returnedStatus.State)
			assert.Equal(t, *tc.expectedStatus.TotalCount, *returnedStatus.TotalCount)
			assert.Len(t, returnedStatus.Statuses, len(tc.expectedStatus.Statuses))
			for i, status := range returnedStatus.Statuses {
				assert.Equal(t, *tc.expectedStatus.Statuses[i].State, *status.State)
				assert.Equal(t, *tc.expectedStatus.Statuses[i].Context, *status.Context)
				assert.Equal(t, *tc.expectedStatus.Statuses[i].Description, *status.Description)
			}
		})
	}
}

func Test_UpdatePullRequestBranch(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := UpdatePullRequestBranch(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "update_pull_request_branch", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "expectedHeadSha")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock update result for success case
	mockUpdateResult := &github.PullRequestBranchUpdateResponse{
		Message: github.Ptr("Branch was updated successfully"),
		URL:     github.Ptr("https://api.github.com/repos/owner/repo/pulls/42"),
	}

	tests := []struct {
		name                 string
		mockedClient         *http.Client
		requestArgs          map[string]interface{}
		expectError          bool
		expectedUpdateResult *github.PullRequestBranchUpdateResponse
		expectedErrMsg       string
	}{
		{
			name: "successful branch update",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposPullsUpdateBranchByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"expected_head_sha": "abcd1234",
					}).andThen(
						mockResponse(t, http.StatusAccepted, mockUpdateResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":           "owner",
				"repo":            "repo",
				"pullNumber":      float64(42),
				"expectedHeadSha": "abcd1234",
			},
			expectError:          false,
			expectedUpdateResult: mockUpdateResult,
		},
		{
			name: "branch update without expected SHA",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposPullsUpdateBranchByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{}).andThen(
						mockResponse(t, http.StatusAccepted, mockUpdateResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:          false,
			expectedUpdateResult: mockUpdateResult,
		},
		{
			name: "branch update fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PutReposPullsUpdateBranchByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusConflict)
						_, _ = w.Write([]byte(`{"message": "Merge conflict"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:    true,
			expectedErrMsg: "failed to update pull request branch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := UpdatePullRequestBranch(stubGetClientFn(client), translations.NullTranslationHelper)

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

			assert.Contains(t, textContent.Text, "is in progress")
		})
	}
}

func Test_GetPullRequestComments(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestComments(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_comments", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR comments for success case
	mockComments := []*github.PullRequestComment{
		{
			ID:      github.Ptr(int64(101)),
			Body:    github.Ptr("This looks good"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42#discussion_r101"),
			User: &github.User{
				Login: github.Ptr("reviewer1"),
			},
			Path:      github.Ptr("file1.go"),
			Position:  github.Ptr(5),
			CommitID:  github.Ptr("abcdef123456"),
			CreatedAt: &github.Timestamp{Time: time.Now().Add(-24 * time.Hour)},
			UpdatedAt: &github.Timestamp{Time: time.Now().Add(-24 * time.Hour)},
		},
		{
			ID:      github.Ptr(int64(102)),
			Body:    github.Ptr("Please fix this"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42#discussion_r102"),
			User: &github.User{
				Login: github.Ptr("reviewer2"),
			},
			Path:      github.Ptr("file2.go"),
			Position:  github.Ptr(10),
			CommitID:  github.Ptr("abcdef123456"),
			CreatedAt: &github.Timestamp{Time: time.Now().Add(-12 * time.Hour)},
			UpdatedAt: &github.Timestamp{Time: time.Now().Add(-12 * time.Hour)},
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedComments []*github.PullRequestComment
		expectedErrMsg   string
	}{
		{
			name: "successful comments fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsCommentsByOwnerByRepoByPullNumber,
					mockComments,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:      false,
			expectedComments: mockComments,
		},
		{
			name: "comments fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsCommentsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request comments",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestComments(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedComments []*github.PullRequestComment
			err = json.Unmarshal([]byte(textContent.Text), &returnedComments)
			require.NoError(t, err)
			assert.Len(t, returnedComments, len(tc.expectedComments))
			for i, comment := range returnedComments {
				assert.Equal(t, *tc.expectedComments[i].ID, *comment.ID)
				assert.Equal(t, *tc.expectedComments[i].Body, *comment.Body)
				assert.Equal(t, *tc.expectedComments[i].User.Login, *comment.User.Login)
				assert.Equal(t, *tc.expectedComments[i].Path, *comment.Path)
				assert.Equal(t, *tc.expectedComments[i].HTMLURL, *comment.HTMLURL)
			}
		})
	}
}

func Test_GetPullRequestReviews(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestReviews(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_reviews", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR reviews for success case
	mockReviews := []*github.PullRequestReview{
		{
			ID:      github.Ptr(int64(201)),
			State:   github.Ptr("APPROVED"),
			Body:    github.Ptr("LGTM"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42#pullrequestreview-201"),
			User: &github.User{
				Login: github.Ptr("approver"),
			},
			CommitID:    github.Ptr("abcdef123456"),
			SubmittedAt: &github.Timestamp{Time: time.Now().Add(-24 * time.Hour)},
		},
		{
			ID:      github.Ptr(int64(202)),
			State:   github.Ptr("CHANGES_REQUESTED"),
			Body:    github.Ptr("Please address the following issues"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42#pullrequestreview-202"),
			User: &github.User{
				Login: github.Ptr("reviewer"),
			},
			CommitID:    github.Ptr("abcdef123456"),
			SubmittedAt: &github.Timestamp{Time: time.Now().Add(-12 * time.Hour)},
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedReviews []*github.PullRequestReview
		expectedErrMsg  string
	}{
		{
			name: "successful reviews fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsReviewsByOwnerByRepoByPullNumber,
					mockReviews,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:     false,
			expectedReviews: mockReviews,
		},
		{
			name: "reviews fetch fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsReviewsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request reviews",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestReviews(stubGetClientFn(client), translations.NullTranslationHelper)

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
			var returnedReviews []*github.PullRequestReview
			err = json.Unmarshal([]byte(textContent.Text), &returnedReviews)
			require.NoError(t, err)
			assert.Len(t, returnedReviews, len(tc.expectedReviews))
			for i, review := range returnedReviews {
				assert.Equal(t, *tc.expectedReviews[i].ID, *review.ID)
				assert.Equal(t, *tc.expectedReviews[i].State, *review.State)
				assert.Equal(t, *tc.expectedReviews[i].Body, *review.Body)
				assert.Equal(t, *tc.expectedReviews[i].User.Login, *review.User.Login)
				assert.Equal(t, *tc.expectedReviews[i].HTMLURL, *review.HTMLURL)
			}
		})
	}
}

func Test_CreatePullRequestReview(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreatePullRequestReview(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_pull_request_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "event")
	assert.Contains(t, tool.InputSchema.Properties, "commitId")
	assert.Contains(t, tool.InputSchema.Properties, "comments")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber", "event"})

	// Setup mock review for success case
	mockReview := &github.PullRequestReview{
		ID:      github.Ptr(int64(301)),
		State:   github.Ptr("APPROVED"),
		Body:    github.Ptr("Looks good!"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42#pullrequestreview-301"),
		User: &github.User{
			Login: github.Ptr("reviewer"),
		},
		CommitID:    github.Ptr("abcdef123456"),
		SubmittedAt: &github.Timestamp{Time: time.Now()},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedReview *github.PullRequestReview
		expectedErrMsg string
	}{
		{
			name: "successful review creation with body only",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"body":  "Looks good!",
						"event": "APPROVE",
					}).andThen(
						mockResponse(t, http.StatusOK, mockReview),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Looks good!",
				"event":      "APPROVE",
			},
			expectError:    false,
			expectedReview: mockReview,
		},
		{
			name: "successful review creation with commitId",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"body":      "Looks good!",
						"event":     "APPROVE",
						"commit_id": "abcdef123456",
					}).andThen(
						mockResponse(t, http.StatusOK, mockReview),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Looks good!",
				"event":      "APPROVE",
				"commitId":   "abcdef123456",
			},
			expectError:    false,
			expectedReview: mockReview,
		},
		{
			name: "successful review creation with comments",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"body":  "Some issues to fix",
						"event": "REQUEST_CHANGES",
						"comments": []interface{}{
							map[string]interface{}{
								"path":     "file1.go",
								"position": float64(10),
								"body":     "This needs to be fixed",
							},
							map[string]interface{}{
								"path":     "file2.go",
								"position": float64(20),
								"body":     "Consider a different approach here",
							},
						},
					}).andThen(
						mockResponse(t, http.StatusOK, mockReview),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Some issues to fix",
				"event":      "REQUEST_CHANGES",
				"comments": []interface{}{
					map[string]interface{}{
						"path":     "file1.go",
						"position": float64(10),
						"body":     "This needs to be fixed",
					},
					map[string]interface{}{
						"path":     "file2.go",
						"position": float64(20),
						"body":     "Consider a different approach here",
					},
				},
			},
			expectError:    false,
			expectedReview: mockReview,
		},
		{
			name: "invalid comment format",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Invalid comment format"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"event":      "REQUEST_CHANGES",
				"comments": []interface{}{
					map[string]interface{}{
						"path": "file1.go",
						// missing position
						"body": "This needs to be fixed",
					},
				},
			},
			expectError:    false,
			expectedErrMsg: "each comment must have either position or line",
		},
		{
			name: "successful review creation with line parameter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"body":  "Code review comments",
						"event": "COMMENT",
						"comments": []interface{}{
							map[string]interface{}{
								"path": "main.go",
								"line": float64(42),
								"body": "Consider adding a comment here",
							},
						},
					}).andThen(
						mockResponse(t, http.StatusOK, mockReview),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Code review comments",
				"event":      "COMMENT",
				"comments": []interface{}{
					map[string]interface{}{
						"path": "main.go",
						"line": float64(42),
						"body": "Consider adding a comment here",
					},
				},
			},
			expectError:    false,
			expectedReview: mockReview,
		},
		{
			name: "successful review creation with multi-line comment",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"body":  "Multi-line comment review",
						"event": "COMMENT",
						"comments": []interface{}{
							map[string]interface{}{
								"path":       "main.go",
								"start_line": float64(10),
								"line":       float64(15),
								"side":       "RIGHT",
								"body":       "This entire block needs refactoring",
							},
						},
					}).andThen(
						mockResponse(t, http.StatusOK, mockReview),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Multi-line comment review",
				"event":      "COMMENT",
				"comments": []interface{}{
					map[string]interface{}{
						"path":       "main.go",
						"start_line": float64(10),
						"line":       float64(15),
						"side":       "RIGHT",
						"body":       "This entire block needs refactoring",
					},
				},
			},
			expectError:    false,
			expectedReview: mockReview,
		},
		{
			name:         "invalid multi-line comment - missing line parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"event":      "COMMENT",
				"comments": []interface{}{
					map[string]interface{}{
						"path":       "main.go",
						"start_line": float64(10),
						// missing line parameter
						"body": "Invalid multi-line comment",
					},
				},
			},
			expectError:    false,
			expectedErrMsg: "each comment must have either position or line", // Updated error message
		},
		{
			name: "invalid comment - mixing position with line parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					mockReview,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"event":      "COMMENT",
				"comments": []interface{}{
					map[string]interface{}{
						"path":     "main.go",
						"position": float64(5),
						"line":     float64(42),
						"body":     "Invalid parameter combination",
					},
				},
			},
			expectError:    false,
			expectedErrMsg: "position cannot be combined with line, side, start_line, or start_side",
		},
		{
			name:         "invalid multi-line comment - missing side parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"event":      "COMMENT",
				"comments": []interface{}{
					map[string]interface{}{
						"path":       "main.go",
						"start_line": float64(10),
						"line":       float64(15),
						"start_side": "LEFT",
						// missing side parameter
						"body": "Invalid multi-line comment",
					},
				},
			},
			expectError:    false,
			expectedErrMsg: "if start_side is provided, side must also be provided",
		},
		{
			name: "review creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsReviewsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Invalid comment format"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "Looks good!",
				"event":      "APPROVE",
			},
			expectError:    true,
			expectedErrMsg: "failed to create pull request review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreatePullRequestReview(stubGetClientFn(client), translations.NullTranslationHelper)

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

			// For error messages in the result
			if tc.expectedErrMsg != "" {
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedReview github.PullRequestReview
			err = json.Unmarshal([]byte(textContent.Text), &returnedReview)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedReview.ID, *returnedReview.ID)
			assert.Equal(t, *tc.expectedReview.State, *returnedReview.State)
			assert.Equal(t, *tc.expectedReview.Body, *returnedReview.Body)
			assert.Equal(t, *tc.expectedReview.User.Login, *returnedReview.User.Login)
			assert.Equal(t, *tc.expectedReview.HTMLURL, *returnedReview.HTMLURL)
		})
	}
}

func Test_CreatePullRequest(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CreatePullRequest(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "title")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "head")
	assert.Contains(t, tool.InputSchema.Properties, "base")
	assert.Contains(t, tool.InputSchema.Properties, "draft")
	assert.Contains(t, tool.InputSchema.Properties, "maintainer_can_modify")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "title", "head", "base"})

	// Setup mock PR for success case
	mockPR := &github.PullRequest{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test PR"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		Head: &github.PullRequestBranch{
			SHA: github.Ptr("abcd1234"),
			Ref: github.Ptr("feature-branch"),
		},
		Base: &github.PullRequestBranch{
			SHA: github.Ptr("efgh5678"),
			Ref: github.Ptr("main"),
		},
		Body:                github.Ptr("This is a test PR"),
		Draft:               github.Ptr(false),
		MaintainerCanModify: github.Ptr(true),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedPR     *github.PullRequest
		expectedErrMsg string
	}{
		{
			name: "successful PR creation",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsByOwnerByRepo,
					expectRequestBody(t, map[string]interface{}{
						"title":                 "Test PR",
						"body":                  "This is a test PR",
						"head":                  "feature-branch",
						"base":                  "main",
						"draft":                 false,
						"maintainer_can_modify": true,
					}).andThen(
						mockResponse(t, http.StatusCreated, mockPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":                 "owner",
				"repo":                  "repo",
				"title":                 "Test PR",
				"body":                  "This is a test PR",
				"head":                  "feature-branch",
				"base":                  "main",
				"draft":                 false,
				"maintainer_can_modify": true,
			},
			expectError: false,
			expectedPR:  mockPR,
		},
		{
			name:         "missing required parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				// missing title, head, base
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: title",
		},
		{
			name: "PR creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message":"Validation failed","errors":[{"resource":"PullRequest","code":"invalid"}]}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"title": "Test PR",
				"head":  "feature-branch",
				"base":  "main",
			},
			expectError:    true,
			expectedErrMsg: "failed to create pull request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := CreatePullRequest(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}

				// If no error returned but in the result
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedPR github.PullRequest
			err = json.Unmarshal([]byte(textContent.Text), &returnedPR)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedPR.Number, *returnedPR.Number)
			assert.Equal(t, *tc.expectedPR.Title, *returnedPR.Title)
			assert.Equal(t, *tc.expectedPR.State, *returnedPR.State)
			assert.Equal(t, *tc.expectedPR.HTMLURL, *returnedPR.HTMLURL)
			assert.Equal(t, *tc.expectedPR.Head.SHA, *returnedPR.Head.SHA)
			assert.Equal(t, *tc.expectedPR.Base.Ref, *returnedPR.Base.Ref)
			assert.Equal(t, *tc.expectedPR.Body, *returnedPR.Body)
			assert.Equal(t, *tc.expectedPR.User.Login, *returnedPR.User.Login)
		})
	}
}

func Test_AddPullRequestReviewComment(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := AddPullRequestReviewComment(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "add_pull_request_review_comment", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pull_number")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "commit_id")
	assert.Contains(t, tool.InputSchema.Properties, "path")
	// Since we've updated commit_id and path to be optional when using in_reply_to
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pull_number", "body"})

	mockComment := &github.PullRequestComment{
		ID:   github.Ptr(int64(123)),
		Body: github.Ptr("Great stuff!"),
		Path: github.Ptr("file1.txt"),
		Line: github.Ptr(2),
		Side: github.Ptr("RIGHT"),
	}

	mockReply := &github.PullRequestComment{
		ID:   github.Ptr(int64(456)),
		Body: github.Ptr("Good point, will fix!"),
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedComment *github.PullRequestComment
		expectedErrMsg  string
	}{
		{
			name: "successful line comment creation",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsCommentsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusCreated)
						err := json.NewEncoder(w).Encode(mockComment)
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(1),
				"body":        "Great stuff!",
				"commit_id":   "6dcb09b5b57875f334f61aebed695e2e4193db5e",
				"path":        "file1.txt",
				"line":        float64(2),
				"side":        "RIGHT",
			},
			expectError:     false,
			expectedComment: mockComment,
		},
		{
			name: "successful reply using in_reply_to",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsCommentsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusCreated)
						err := json.NewEncoder(w).Encode(mockReply)
						if err != nil {
							http.Error(w, err.Error(), http.StatusInternalServerError)
							return
						}
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(1),
				"body":        "Good point, will fix!",
				"in_reply_to": float64(123),
			},
			expectError:     false,
			expectedComment: mockReply,
		},
		{
			name: "comment creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsCommentsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(1),
				"body":        "Great stuff!",
				"commit_id":   "6dcb09b5b57875f334f61aebed695e2e4193db5e",
				"path":        "file1.txt",
				"line":        float64(2),
			},
			expectError:    true,
			expectedErrMsg: "failed to create pull request comment",
		},
		{
			name: "reply creation fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsCommentsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"message": "Comment not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(1),
				"body":        "Good point, will fix!",
				"in_reply_to": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to reply to pull request comment",
		},
		{
			name:         "missing required parameters for comment",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(1),
				"body":        "Great stuff!",
				// missing commit_id and path
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: commit_id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := github.NewClient(tc.mockedClient)

			_, handler := AddPullRequestReviewComment(stubGetClientFn(mockClient), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			require.Len(t, result.Content, 1)

			textContent := getTextResult(t, result)
			if tc.expectedErrMsg != "" {
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			var returnedComment github.PullRequestComment
			err = json.Unmarshal([]byte(getTextResult(t, result).Text), &returnedComment)
			require.NoError(t, err)

			assert.Equal(t, *tc.expectedComment.ID, *returnedComment.ID)
			assert.Equal(t, *tc.expectedComment.Body, *returnedComment.Body)

			// Only check Path, Line, and Side if they exist in the expected comment
			if tc.expectedComment.Path != nil {
				assert.Equal(t, *tc.expectedComment.Path, *returnedComment.Path)
			}
			if tc.expectedComment.Line != nil {
				assert.Equal(t, *tc.expectedComment.Line, *returnedComment.Line)
			}
			if tc.expectedComment.Side != nil {
				assert.Equal(t, *tc.expectedComment.Side, *returnedComment.Side)
			}
		})
	}
}

func Test_RequestPullRequestReviewers(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := RequestPullRequestReviewers(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "request_pull_request_reviewers", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pull_number")
	assert.Contains(t, tool.InputSchema.Properties, "reviewers")
	assert.Contains(t, tool.InputSchema.Properties, "team_reviewers")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pull_number"})

	// Setup mock PR for success case
	mockPR := &github.PullRequest{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test PR"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/pull/42"),
		RequestedReviewers: []*github.User{
			{Login: github.Ptr("reviewer1")},
			{Login: github.Ptr("reviewer2")},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedPR     *github.PullRequest
		expectedErrMsg string
	}{
		{
			name: "successful reviewers request with individual reviewers",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"reviewers": []interface{}{"reviewer1", "reviewer2"},
					}).andThen(
						mockResponse(t, http.StatusOK, mockPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(42),
				"reviewers":   []interface{}{"reviewer1", "reviewer2"},
			},
			expectError: false,
			expectedPR:  mockPR,
		},
		{
			name: "successful reviewers request with team reviewers",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"team_reviewers": []interface{}{"core-team", "design-team"},
					}).andThen(
						mockResponse(t, http.StatusOK, mockPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "owner",
				"repo":           "repo",
				"pull_number":    float64(42),
				"team_reviewers": []interface{}{"core-team", "design-team"},
			},
			expectError: false,
			expectedPR:  mockPR,
		},
		{
			name: "successful reviewers request with both individual and team reviewers",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"reviewers":      []interface{}{"reviewer1"},
						"team_reviewers": []interface{}{"core-team"},
					}).andThen(
						mockResponse(t, http.StatusOK, mockPR),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "owner",
				"repo":           "repo",
				"pull_number":    float64(42),
				"reviewers":      []interface{}{"reviewer1"},
				"team_reviewers": []interface{}{"core-team"},
			},
			expectError: false,
			expectedPR:  mockPR,
		},
		{
			name:         "no reviewers specified",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(42),
				// No reviewers or team_reviewers provided
			},
			expectError:    false,
			expectedErrMsg: "At least one reviewer or team reviewer must be specified",
		},
		{
			name: "request reviewers fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Validation failed"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"pull_number": float64(999),
				"reviewers":   []interface{}{"reviewer1"},
			},
			expectError:    true,
			expectedErrMsg: "failed to request reviewers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := RequestPullRequestReviewers(stubGetClientFn(client), translations.NullTranslationHelper)

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

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Check for expected error message within the result text
			if tc.expectedErrMsg != "" {
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			// Unmarshal and verify the successful result
			var returnedPR github.PullRequest
			err = json.Unmarshal([]byte(textContent.Text), &returnedPR)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedPR.Number, *returnedPR.Number)
			assert.Equal(t, *tc.expectedPR.Title, *returnedPR.Title)
			assert.Equal(t, *tc.expectedPR.State, *returnedPR.State)

			// Verify requested reviewers if available
			if tc.expectedPR.RequestedReviewers != nil {
				require.NotNil(t, returnedPR.RequestedReviewers)
				assert.Len(t, returnedPR.RequestedReviewers, len(tc.expectedPR.RequestedReviewers))
				for i, reviewer := range returnedPR.RequestedReviewers {
					assert.Equal(t, *tc.expectedPR.RequestedReviewers[i].Login, *reviewer.Login)
				}
			}
		})
	}
}
