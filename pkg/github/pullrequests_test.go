package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/shurcooL/githubv4"

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

func TestCreateAndSubmitPullRequestReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := githubv4.NewClient(nil)
	tool, _ := CreateAndSubmitPullRequestReview(stubGetGQLClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_and_submit_pull_request_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "event")
	assert.Contains(t, tool.InputSchema.Properties, "commitID")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber", "body", "event"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful review creation",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(
						map[string]any{
							"repository": map[string]any{
								"pullRequest": map[string]any{
									"id": "PR_kwDODKw3uc6WYN1T",
								},
							},
						},
					),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						AddPullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"addPullRequestReview(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewInput{
						PullRequestID: githubv4.ID("PR_kwDODKw3uc6WYN1T"),
						Body:          githubv4.NewString("This is a test review"),
						Event:         githubv4mock.Ptr(githubv4.PullRequestReviewEventComment),
						CommitOID:     githubv4.NewGitObjectID("abcd1234"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{}),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "This is a test review",
				"event":      "COMMENT",
				"commitID":   "abcd1234",
			},
			expectToolError: false,
		},
		{
			name: "failure to get pull request",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.ErrorResponse("expected test failure"),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "This is a test review",
				"event":      "COMMENT",
				"commitID":   "abcd1234",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
		{
			name: "failure to submit review",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(
						map[string]any{
							"repository": map[string]any{
								"pullRequest": map[string]any{
									"id": "PR_kwDODKw3uc6WYN1T",
								},
							},
						},
					),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						AddPullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"addPullRequestReview(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewInput{
						PullRequestID: githubv4.ID("PR_kwDODKw3uc6WYN1T"),
						Body:          githubv4.NewString("This is a test review"),
						Event:         githubv4mock.Ptr(githubv4.PullRequestReviewEventComment),
						CommitOID:     githubv4.NewGitObjectID("abcd1234"),
					},
					nil,
					githubv4mock.ErrorResponse("expected test failure"),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"body":       "This is a test review",
				"event":      "COMMENT",
				"commitID":   "abcd1234",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			_, handler := CreateAndSubmitPullRequestReview(stubGetGQLClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, textContent.Text, "pull request review submitted successfully")
		})
	}
}

func Test_RequestCopilotReview(t *testing.T) {
	t.Parallel()

	mockClient := github.NewClient(nil)
	tool, _ := RequestCopilotReview(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "request_copilot_review", tool.Name)
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
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful request",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					expect(t, expectations{
						path: "/repos/owner/repo/pulls/1/requested_reviewers",
						requestBody: map[string]any{
							"reviewers": []any{"copilot-pull-request-reviewer[bot]"},
						},
					}).andThen(
						mockResponse(t, http.StatusCreated, mockPR),
					),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(1),
			},
			expectError: false,
		},
		{
			name: "request fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to request copilot review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := github.NewClient(tc.mockedClient)
			_, handler := RequestCopilotReview(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.Len(t, result.Content, 1)

			textContent := getTextResult(t, result)
			require.Equal(t, "", textContent.Text)
		})
	}
}

func TestCreatePendingPullRequestReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := githubv4.NewClient(nil)
	tool, _ := CreatePendingPullRequestReview(stubGetGQLClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "create_pending_pull_request_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "commitID")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful review creation",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(
						map[string]any{
							"repository": map[string]any{
								"pullRequest": map[string]any{
									"id": "PR_kwDODKw3uc6WYN1T",
								},
							},
						},
					),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						AddPullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"addPullRequestReview(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewInput{
						PullRequestID: githubv4.ID("PR_kwDODKw3uc6WYN1T"),
						CommitOID:     githubv4.NewGitObjectID("abcd1234"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{}),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"commitID":   "abcd1234",
			},
			expectToolError: false,
		},
		{
			name: "failure to get pull request",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.ErrorResponse("expected test failure"),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"commitID":   "abcd1234",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
		{
			name: "failure to create pending review",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID githubv4.ID
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(
						map[string]any{
							"repository": map[string]any{
								"pullRequest": map[string]any{
									"id": "PR_kwDODKw3uc6WYN1T",
								},
							},
						},
					),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						AddPullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"addPullRequestReview(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewInput{
						PullRequestID: githubv4.ID("PR_kwDODKw3uc6WYN1T"),
						CommitOID:     githubv4.NewGitObjectID("abcd1234"),
					},
					nil,
					githubv4mock.ErrorResponse("expected test failure"),
				),
			),
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"commitID":   "abcd1234",
			},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			_, handler := CreatePendingPullRequestReview(stubGetGQLClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, textContent.Text, "pending pull request created")
		})
	}
}

func TestAddPullRequestReviewCommentToPendingReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := githubv4.NewClient(nil)
	tool, _ := AddPullRequestReviewCommentToPendingReview(stubGetGQLClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "add_pull_request_review_comment_to_pending_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "path")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.Contains(t, tool.InputSchema.Properties, "subjectType")
	assert.Contains(t, tool.InputSchema.Properties, "line")
	assert.Contains(t, tool.InputSchema.Properties, "side")
	assert.Contains(t, tool.InputSchema.Properties, "startLine")
	assert.Contains(t, tool.InputSchema.Properties, "startSide")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber", "path", "body", "subjectType"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful line comment addition",
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumber":  float64(42),
				"path":        "file.go",
				"body":        "This is a test comment",
				"subjectType": "LINE",
				"line":        float64(10),
				"side":        "RIGHT",
				"startLine":   float64(5),
				"startSide":   "RIGHT",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				viewerQuery("williammartin"),
				getLatestPendingReviewQuery(getLatestPendingReviewQueryParams{
					author: "williammartin",
					owner:  "owner",
					repo:   "repo",
					prNum:  42,

					reviews: []getLatestPendingReviewQueryReview{
						{
							id:    "PR_kwDODKw3uc6WYN1T",
							state: "PENDING",
							url:   "https://github.com/owner/repo/pull/42",
						},
					},
				}),
				githubv4mock.NewMutationMatcher(
					struct {
						AddPullRequestReviewThread struct {
							Thread struct {
								ID githubv4.String // We don't need this, but a selector is required or GQL complains.
							}
						} `graphql:"addPullRequestReviewThread(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewThreadInput{
						Path:                githubv4.String("file.go"),
						Body:                githubv4.String("This is a test comment"),
						SubjectType:         githubv4mock.Ptr(githubv4.PullRequestReviewThreadSubjectTypeLine),
						Line:                githubv4.NewInt(10),
						Side:                githubv4mock.Ptr(githubv4.DiffSideRight),
						StartLine:           githubv4.NewInt(5),
						StartSide:           githubv4mock.Ptr(githubv4.DiffSideRight),
						PullRequestReviewID: githubv4.NewID("PR_kwDODKw3uc6WYN1T"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{}),
				),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			_, handler := AddPullRequestReviewCommentToPendingReview(stubGetGQLClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, textContent.Text, "pull request review comment successfully added to pending review")
		})
	}
}

func TestSubmitPendingPullRequestReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := githubv4.NewClient(nil)
	tool, _ := SubmitPendingPullRequestReview(stubGetGQLClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "submit_pending_pull_request_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.Contains(t, tool.InputSchema.Properties, "event")
	assert.Contains(t, tool.InputSchema.Properties, "body")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber", "event"})

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful review submission",
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"event":      "COMMENT",
				"body":       "This is a test review",
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				viewerQuery("williammartin"),
				getLatestPendingReviewQuery(getLatestPendingReviewQueryParams{
					author: "williammartin",
					owner:  "owner",
					repo:   "repo",
					prNum:  42,

					reviews: []getLatestPendingReviewQueryReview{
						{
							id:    "PR_kwDODKw3uc6WYN1T",
							state: "PENDING",
							url:   "https://github.com/owner/repo/pull/42",
						},
					},
				}),
				githubv4mock.NewMutationMatcher(
					struct {
						SubmitPullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"submitPullRequestReview(input: $input)"`
					}{},
					githubv4.SubmitPullRequestReviewInput{
						PullRequestReviewID: githubv4.NewID("PR_kwDODKw3uc6WYN1T"),
						Event:               githubv4.PullRequestReviewEventComment,
						Body:                githubv4.NewString("This is a test review"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{}),
				),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			_, handler := SubmitPendingPullRequestReview(stubGetGQLClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, "pending pull request review successfully submitted", textContent.Text)
		})
	}
}

func TestDeletePendingPullRequestReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := githubv4.NewClient(nil)
	tool, _ := DeletePendingPullRequestReview(stubGetGQLClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "delete_pending_pull_request_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful review deletion",
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				viewerQuery("williammartin"),
				getLatestPendingReviewQuery(getLatestPendingReviewQueryParams{
					author: "williammartin",
					owner:  "owner",
					repo:   "repo",
					prNum:  42,

					reviews: []getLatestPendingReviewQueryReview{
						{
							id:    "PR_kwDODKw3uc6WYN1T",
							state: "PENDING",
							url:   "https://github.com/owner/repo/pull/42",
						},
					},
				}),
				githubv4mock.NewMutationMatcher(
					struct {
						DeletePullRequestReview struct {
							PullRequestReview struct {
								ID githubv4.ID
							}
						} `graphql:"deletePullRequestReview(input: $input)"`
					}{},
					githubv4.DeletePullRequestReviewInput{
						PullRequestReviewID: githubv4.NewID("PR_kwDODKw3uc6WYN1T"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{}),
				),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			_, handler := DeletePendingPullRequestReview(stubGetGQLClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, "pending pull request review successfully deleted", textContent.Text)
		})
	}
}

func TestGetPullRequestDiff(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestDiff(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_diff", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	stubbedDiff := `diff --git a/README.md b/README.md
index 5d6e7b2..8a4f5c3 100644
--- a/README.md
+++ b/README.md
@@ -1,4 +1,6 @@
 # Hello-World

 Hello World project for GitHub

+## New Section
+
+This is a new section added in the pull request.`

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful diff retrieval",
			requestArgs: map[string]any{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					// Should also expect Accept header to be application/vnd.github.v3.diff
					expectPath(t, "/repos/owner/repo/pulls/42").andThen(
						mockResponse(t, http.StatusOK, stubbedDiff),
					),
				),
			),
			expectToolError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestDiff(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, stubbedDiff, textContent.Text)
		})
	}
}

func viewerQuery(login string) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			Viewer struct {
				Login githubv4.String
			} `graphql:"viewer"`
		}{},
		map[string]any{},
		githubv4mock.DataResponse(map[string]any{
			"viewer": map[string]any{
				"login": login,
			},
		}),
	)
}

type getLatestPendingReviewQueryReview struct {
	id    string
	state string
	url   string
}

type getLatestPendingReviewQueryParams struct {
	author string
	owner  string
	repo   string
	prNum  int32

	reviews []getLatestPendingReviewQueryReview
}

func getLatestPendingReviewQuery(p getLatestPendingReviewQueryParams) githubv4mock.Matcher {
	return githubv4mock.NewQueryMatcher(
		struct {
			Repository struct {
				PullRequest struct {
					Reviews struct {
						Nodes []struct {
							ID    githubv4.ID
							State githubv4.PullRequestReviewState
							URL   githubv4.URI
						}
					} `graphql:"reviews(first: 1, author: $author)"`
				} `graphql:"pullRequest(number: $prNum)"`
			} `graphql:"repository(owner: $owner, name: $name)"`
		}{},
		map[string]any{
			"author": githubv4.String(p.author),
			"owner":  githubv4.String(p.owner),
			"name":   githubv4.String(p.repo),
			"prNum":  githubv4.Int(p.prNum),
		},
		githubv4mock.DataResponse(
			map[string]any{
				"repository": map[string]any{
					"pullRequest": map[string]any{
						"reviews": map[string]any{
							"nodes": []any{
								map[string]any{
									"id":    p.reviews[0].id,
									"state": p.reviews[0].state,
									"url":   p.reviews[0].url,
								},
							},
						},
					},
				},
			},
		),
	)
}
