package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"

	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetPullRequest(t *testing.T) {
	// Verify tool definition once
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "get",
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
				"method":     "get",
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
			gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
			deps := BaseDeps{
				Client:          client,
				GQLClient:       gqlClient,
				RepoAccessCache: stubRepoAccessCache(gqlClient, 5*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
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
	serverTool := UpdatePullRequest(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "draft")
	assert.Contains(t, schema.Properties, "title")
	assert.Contains(t, schema.Properties, "body")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "base")
	assert.Contains(t, schema.Properties, "maintainer_can_modify")
	assert.Contains(t, schema.Properties, "reviewers")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

	// Setup mock PR for success case
	mockUpdatedPR := &github.PullRequest{
		Number:              github.Ptr(42),
		Title:               github.Ptr("Updated Test PR Title"),
		State:               github.Ptr("open"),
		HTMLURL:             github.Ptr("https://github.com/owner/repo/pull/42"),
		Body:                github.Ptr("Updated test PR body."),
		MaintainerCanModify: github.Ptr(false),
		Draft:               github.Ptr(false),
		Base: &github.PullRequestBranch{
			Ref: github.Ptr("develop"),
		},
	}

	mockClosedPR := &github.PullRequest{
		Number: github.Ptr(42),
		Title:  github.Ptr("Test PR"),
		State:  github.Ptr("closed"), // State updated
	}

	// Mock PR for when there are no updates but we still need a response
	mockPRWithReviewers := &github.PullRequest{
		Number: github.Ptr(42),
		Title:  github.Ptr("Test PR"),
		State:  github.Ptr("open"),
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
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockUpdatedPR,
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
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockClosedPR,
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
			name: "successful PR update with reviewers",
			mockedClient: mock.NewMockedHTTPClient(
				// Mock for RequestReviewers call, returning the PR with reviewers
				mock.WithRequestMatch(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					mockPRWithReviewers,
				),
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockPRWithReviewers,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"reviewers":  []interface{}{"reviewer1", "reviewer2"},
			},
			expectError: false,
			expectedPR:  mockPRWithReviewers,
		},
		{
			name: "successful PR update (title only)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposPullsByOwnerByRepoByPullNumber,
					expectRequestBody(t, map[string]interface{}{
						"title": "Updated Test PR Title",
					}).andThen(
						mockResponse(t, http.StatusOK, mockUpdatedPR),
					),
				),
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockUpdatedPR,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"title":      "Updated Test PR Title",
			},
			expectError: false,
			expectedPR:  mockUpdatedPR,
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
		{
			name: "request reviewers fails",
			mockedClient: mock.NewMockedHTTPClient(
				// Then reviewer request fails
				mock.WithRequestMatchHandler(
					mock.PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnprocessableEntity)
						_, _ = w.Write([]byte(`{"message": "Invalid reviewers"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"reviewers":  []interface{}{"invalid-user"},
			},
			expectError:    true,
			expectedErrMsg: "failed to request reviewers",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			gqlClient := githubv4.NewClient(nil)
			deps := BaseDeps{
				Client:    client,
				GQLClient: gqlClient,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError || tc.expectedErrMsg != "" {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var updateResp MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &updateResp)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPR.GetHTMLURL(), updateResp.URL)
		})
	}
}

func Test_UpdatePullRequest_Draft(t *testing.T) {
	// Setup mock PR for success case
	mockUpdatedPR := &github.PullRequest{
		Number:              github.Ptr(42),
		Title:               github.Ptr("Test PR Title"),
		State:               github.Ptr("open"),
		HTMLURL:             github.Ptr("https://github.com/owner/repo/pull/42"),
		Body:                github.Ptr("Test PR body."),
		MaintainerCanModify: github.Ptr(false),
		Draft:               github.Ptr(false), // Updated to ready for review
		Base: &github.PullRequestBranch{
			Ref: github.Ptr("main"),
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
			name: "successful draft update to ready for review",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"id":      "PR_kwDOA0xdyM50BPaO",
								"isDraft": true, // Current state is draft
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						MarkPullRequestReadyForReview struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							}
						} `graphql:"markPullRequestReadyForReview(input: $input)"`
					}{},
					githubv4.MarkPullRequestReadyForReviewInput{
						PullRequestID: "PR_kwDOA0xdyM50BPaO",
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"markPullRequestReadyForReview": map[string]any{
							"pullRequest": map[string]any{
								"id":      "PR_kwDOA0xdyM50BPaO",
								"isDraft": false,
							},
						},
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"draft":      false,
			},
			expectError: false,
			expectedPR:  mockUpdatedPR,
		},
		{
			name: "successful convert pull request to draft",
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							} `graphql:"pullRequest(number: $prNum)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
						"prNum": githubv4.Int(42),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"id":      "PR_kwDOA0xdyM50BPaO",
								"isDraft": false, // Current state is draft
							},
						},
					}),
				),
				githubv4mock.NewMutationMatcher(
					struct {
						ConvertPullRequestToDraft struct {
							PullRequest struct {
								ID      githubv4.ID
								IsDraft githubv4.Boolean
							}
						} `graphql:"convertPullRequestToDraft(input: $input)"`
					}{},
					githubv4.ConvertPullRequestToDraftInput{
						PullRequestID: "PR_kwDOA0xdyM50BPaO",
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"convertPullRequestToDraft": map[string]any{
							"pullRequest": map[string]any{
								"id":      "PR_kwDOA0xdyM50BPaO",
								"isDraft": true,
							},
						},
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"draft":      true,
			},
			expectError: false,
			expectedPR:  mockUpdatedPR,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// For draft-only tests, we need to mock both GraphQL and the final REST GET call
			restClient := github.NewClient(mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					mockUpdatedPR,
				),
			))
			gqlClient := githubv4.NewClient(tc.mockedClient)

			serverTool := UpdatePullRequest(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:    restClient,
				GQLClient: gqlClient,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError || tc.expectedErrMsg != "" {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var updateResp MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &updateResp)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPR.GetHTMLURL(), updateResp.URL)
		})
	}
}

func Test_ListPullRequests(t *testing.T) {
	// Verify tool definition once
	serverTool := ListPullRequests(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_pull_requests", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "head")
	assert.Contains(t, schema.Properties, "base")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "direction")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "page")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

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
			serverTool := ListPullRequests(translations.NullTranslationHelper)
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
	serverTool := MergePullRequest(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "merge_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "commit_title")
	assert.Contains(t, schema.Properties, "commit_message")
	assert.Contains(t, schema.Properties, "merge_method")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

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
			serverTool := MergePullRequest(translations.NullTranslationHelper)
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
			var returnedResult github.PullRequestMergeResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedMergeResult.Merged, *returnedResult.Merged)
			assert.Equal(t, *tc.expectedMergeResult.Message, *returnedResult.Message)
			assert.Equal(t, *tc.expectedMergeResult.SHA, *returnedResult.SHA)
		})
	}
}

func Test_SearchPullRequests(t *testing.T) {
	serverTool := SearchPullRequests(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_pull_requests", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "order")
	assert.Contains(t, schema.Properties, "perPage")
	assert.Contains(t, schema.Properties, "page")
	assert.ElementsMatch(t, schema.Required, []string{"query"})

	mockSearchResult := &github.IssuesSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		Issues: []*github.Issue{
			{
				Number:   github.Ptr(42),
				Title:    github.Ptr("Test PR 1"),
				Body:     github.Ptr("Updated tests."),
				State:    github.Ptr("open"),
				HTMLURL:  github.Ptr("https://github.com/owner/repo/pull/1"),
				Comments: github.Ptr(5),
				User: &github.User{
					Login: github.Ptr("user1"),
				},
			},
			{
				Number:   github.Ptr(43),
				Title:    github.Ptr("Test PR 2"),
				Body:     github.Ptr("Updated build scripts."),
				State:    github.Ptr("open"),
				HTMLURL:  github.Ptr("https://github.com/owner/repo/pull/2"),
				Comments: github.Ptr(3),
				User: &github.User{
					Login: github.Ptr("user2"),
				},
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedResult *github.IssuesSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful pull request search with all parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr repo:owner/repo is:open",
							"sort":     "created",
							"order":    "desc",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query":   "repo:owner/repo is:open",
				"sort":    "created",
				"order":   "desc",
				"page":    float64(1),
				"perPage": float64(30),
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "pull request search with owner and repo parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "repo:test-owner/test-repo is:pr draft:false",
							"sort":     "updated",
							"order":    "asc",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "draft:false",
				"owner": "test-owner",
				"repo":  "test-repo",
				"sort":  "updated",
				"order": "asc",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "pull request search with only owner parameter (should ignore it)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr feature",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "feature",
				"owner": "test-owner",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "pull request search with only repo parameter (should ignore it)",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr review-required",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "review-required",
				"repo":  "test-repo",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "pull request search with minimal parameters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetSearchIssues,
					mockSearchResult,
				),
			),
			requestArgs: map[string]interface{}{
				"query": "is:pr repo:owner/repo is:open",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing is:pr filter - no duplication",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr repo:github/github-mcp-server is:open draft:false",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "is:pr repo:github/github-mcp-server is:open draft:false",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing repo: filter and conflicting owner/repo params - uses query filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr repo:github/github-mcp-server author:octocat",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "repo:github/github-mcp-server author:octocat",
				"owner": "different-owner",
				"repo":  "different-repo",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with existing is:pr filter and OR operators",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
					expectQueryParams(
						t,
						map[string]string{
							"q":        "is:pr repo:github/github-mcp-server (label:bug OR label:enhancement OR label:feature)",
							"page":     "1",
							"per_page": "30",
						},
					).andThen(
						mockResponse(t, http.StatusOK, mockSearchResult),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"query": "is:pr repo:github/github-mcp-server (label:bug OR label:enhancement OR label:feature)",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search pull requests fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetSearchIssues,
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
			expectedErrMsg: "failed to search pull requests",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			serverTool := SearchPullRequests(translations.NullTranslationHelper)
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
				require.NotNil(t, result)
				require.True(t, result.IsError)
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedResult github.IssuesSearchResult
			err = json.Unmarshal([]byte(textContent.Text), &returnedResult)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedResult.Total, *returnedResult.Total)
			assert.Equal(t, *tc.expectedResult.IncompleteResults, *returnedResult.IncompleteResults)
			assert.Len(t, returnedResult.Issues, len(tc.expectedResult.Issues))
			for i, issue := range returnedResult.Issues {
				assert.Equal(t, *tc.expectedResult.Issues[i].Number, *issue.Number)
				assert.Equal(t, *tc.expectedResult.Issues[i].Title, *issue.Title)
				assert.Equal(t, *tc.expectedResult.Issues[i].State, *issue.State)
				assert.Equal(t, *tc.expectedResult.Issues[i].HTMLURL, *issue.HTMLURL)
				assert.Equal(t, *tc.expectedResult.Issues[i].User.Login, *issue.User.Login)
			}
		})
	}

}

func Test_GetPullRequestFiles(t *testing.T) {
	// Verify tool definition once
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "get_files",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:   false,
			expectedFiles: mockFiles,
		},
		{
			name: "successful files fetch with pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsFilesByOwnerByRepoByPullNumber,
					mockFiles,
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "get_files",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
				"page":       float64(2),
				"perPage":    float64(10),
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
				"method":     "get_files",
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
			serverTool := PullRequestRead(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:          client,
				RepoAccessCache: stubRepoAccessCache(githubv4.NewClient(githubv4mock.NewMockedHTTPClient()), 5*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
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
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "get_status",
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
				"method":     "get_status",
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
				"method":     "get_status",
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
			serverTool := PullRequestRead(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:          client,
				RepoAccessCache: stubRepoAccessCache(githubv4.NewClient(nil), 5*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
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
	serverTool := UpdatePullRequestBranch(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_pull_request_branch", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "expectedHeadSha")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

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
			serverTool := UpdatePullRequestBranch(translations.NullTranslationHelper)
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

			assert.Contains(t, textContent.Text, "is in progress")
		})
	}
}

func Test_GetPullRequestComments(t *testing.T) {
	// Verify tool definition once
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

	tests := []struct {
		name            string
		gqlHTTPClient   *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedErrMsg  string
		lockdownEnabled bool
		validateResult  func(t *testing.T, textContent string)
	}{
		{
			name: "successful review threads fetch",
			gqlHTTPClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					reviewThreadsQuery{},
					map[string]interface{}{
						"owner":             githubv4.String("owner"),
						"repo":              githubv4.String("repo"),
						"prNum":             githubv4.Int(42),
						"first":             githubv4.Int(30),
						"commentsPerThread": githubv4.Int(100),
						"after":             (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"reviewThreads": map[string]any{
									"nodes": []map[string]any{
										{
											"id":          "RT_kwDOA0xdyM4AX1Yz",
											"isResolved":  false,
											"isOutdated":  false,
											"isCollapsed": false,
											"comments": map[string]any{
												"totalCount": 2,
												"nodes": []map[string]any{
													{
														"id":   "PRRC_kwDOA0xdyM4AX1Y0",
														"body": "This looks good",
														"path": "file1.go",
														"line": 5,
														"author": map[string]any{
															"login": "reviewer1",
														},
														"createdAt": "2024-01-01T12:00:00Z",
														"updatedAt": "2024-01-01T12:00:00Z",
														"url":       "https://github.com/owner/repo/pull/42#discussion_r101",
													},
													{
														"id":   "PRRC_kwDOA0xdyM4AX1Y1",
														"body": "Please fix this",
														"path": "file1.go",
														"line": 10,
														"author": map[string]any{
															"login": "reviewer2",
														},
														"createdAt": "2024-01-01T13:00:00Z",
														"updatedAt": "2024-01-01T13:00:00Z",
														"url":       "https://github.com/owner/repo/pull/42#discussion_r102",
													},
												},
											},
										},
									},
									"pageInfo": map[string]any{
										"hasNextPage":     false,
										"hasPreviousPage": false,
										"startCursor":     "cursor1",
										"endCursor":       "cursor2",
									},
									"totalCount": 1,
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "get_review_comments",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError: false,
			validateResult: func(t *testing.T, textContent string) {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(textContent), &result)
				require.NoError(t, err)

				// Validate response structure
				assert.Contains(t, result, "reviewThreads")
				assert.Contains(t, result, "pageInfo")
				assert.Contains(t, result, "totalCount")

				// Validate review threads
				threads := result["reviewThreads"].([]interface{})
				assert.Len(t, threads, 1)

				thread := threads[0].(map[string]interface{})
				assert.Equal(t, "RT_kwDOA0xdyM4AX1Yz", thread["ID"])
				assert.Equal(t, false, thread["IsResolved"])
				assert.Equal(t, false, thread["IsOutdated"])
				assert.Equal(t, false, thread["IsCollapsed"])

				// Validate comments within thread
				comments := thread["Comments"].(map[string]interface{})
				commentNodes := comments["Nodes"].([]interface{})
				assert.Len(t, commentNodes, 2)

				// Validate first comment
				comment1 := commentNodes[0].(map[string]interface{})
				assert.Equal(t, "PRRC_kwDOA0xdyM4AX1Y0", comment1["ID"])
				assert.Equal(t, "This looks good", comment1["Body"])
				assert.Equal(t, "file1.go", comment1["Path"])

				// Validate pagination info
				pageInfo := result["pageInfo"].(map[string]interface{})
				assert.Equal(t, false, pageInfo["hasNextPage"])
				assert.Equal(t, false, pageInfo["hasPreviousPage"])
				assert.Equal(t, "cursor1", pageInfo["startCursor"])
				assert.Equal(t, "cursor2", pageInfo["endCursor"])

				// Validate total count
				assert.Equal(t, float64(1), result["totalCount"])
			},
		},
		{
			name: "review threads fetch fails",
			gqlHTTPClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					reviewThreadsQuery{},
					map[string]interface{}{
						"owner":             githubv4.String("owner"),
						"repo":              githubv4.String("repo"),
						"prNum":             githubv4.Int(999),
						"first":             githubv4.Int(30),
						"commentsPerThread": githubv4.Int(100),
						"after":             (*githubv4.String)(nil),
					},
					githubv4mock.ErrorResponse("Could not resolve to a PullRequest with the number of 999."),
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "get_review_comments",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request review threads",
		},
		{
			name: "lockdown enabled filters review comments without push access",
			gqlHTTPClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					reviewThreadsQuery{},
					map[string]interface{}{
						"owner":             githubv4.String("owner"),
						"repo":              githubv4.String("repo"),
						"prNum":             githubv4.Int(42),
						"first":             githubv4.Int(30),
						"commentsPerThread": githubv4.Int(100),
						"after":             (*githubv4.String)(nil),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"pullRequest": map[string]any{
								"reviewThreads": map[string]any{
									"nodes": []map[string]any{
										{
											"id":          "RT_kwDOA0xdyM4AX1Yz",
											"isResolved":  false,
											"isOutdated":  false,
											"isCollapsed": false,
											"comments": map[string]any{
												"totalCount": 2,
												"nodes": []map[string]any{
													{
														"id":   "PRRC_kwDOA0xdyM4AX1Y0",
														"body": "Maintainer review comment",
														"path": "file1.go",
														"line": 5,
														"author": map[string]any{
															"login": "maintainer",
														},
														"createdAt": "2024-01-01T12:00:00Z",
														"updatedAt": "2024-01-01T12:00:00Z",
														"url":       "https://github.com/owner/repo/pull/42#discussion_r2010",
													},
													{
														"id":   "PRRC_kwDOA0xdyM4AX1Y1",
														"body": "External review comment",
														"path": "file1.go",
														"line": 10,
														"author": map[string]any{
															"login": "testuser",
														},
														"createdAt": "2024-01-01T13:00:00Z",
														"updatedAt": "2024-01-01T13:00:00Z",
														"url":       "https://github.com/owner/repo/pull/42#discussion_r2011",
													},
												},
											},
										},
									},
									"pageInfo": map[string]any{
										"hasNextPage":     false,
										"hasPreviousPage": false,
										"startCursor":     "cursor1",
										"endCursor":       "cursor2",
									},
									"totalCount": 1,
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "get_review_comments",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:     false,
			lockdownEnabled: true,
			validateResult: func(t *testing.T, textContent string) {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(textContent), &result)
				require.NoError(t, err)

				// Validate that only maintainer comment is returned
				threads := result["reviewThreads"].([]interface{})
				assert.Len(t, threads, 1)

				thread := threads[0].(map[string]interface{})
				comments := thread["Comments"].(map[string]interface{})

				// Should only have 1 comment (maintainer) after filtering
				assert.Equal(t, float64(1), comments["TotalCount"])

				commentNodes := comments["Nodes"].([]interface{})
				assert.Len(t, commentNodes, 1)

				comment := commentNodes[0].(map[string]interface{})
				author := comment["Author"].(map[string]interface{})
				assert.Equal(t, "maintainer", author["Login"])
				assert.Equal(t, "Maintainer review comment", comment["Body"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup GraphQL client with mock
			var gqlClient *githubv4.Client
			if tc.gqlHTTPClient != nil {
				gqlClient = githubv4.NewClient(tc.gqlHTTPClient)
			} else {
				gqlClient = githubv4.NewClient(nil)
			}

			// Setup cache for lockdown mode
			var cache *lockdown.RepoAccessCache
			if tc.lockdownEnabled {
				cache = stubRepoAccessCache(githubv4.NewClient(newRepoAccessHTTPClient()), 5*time.Minute)
			} else {
				cache = stubRepoAccessCache(gqlClient, 5*time.Minute)
			}

			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": tc.lockdownEnabled})
			serverTool := PullRequestRead(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:          github.NewClient(nil),
				GQLClient:       gqlClient,
				RepoAccessCache: cache,
				Flags:           flags,
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

			// Use custom validation if provided
			if tc.validateResult != nil {
				tc.validateResult(t, textContent.Text)
			}
		})
	}
}

func Test_GetPullRequestReviews(t *testing.T) {
	// Verify tool definition once
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
		gqlHTTPClient   *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedReviews []*github.PullRequestReview
		expectedErrMsg  string
		lockdownEnabled bool
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
				"method":     "get_reviews",
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
				"method":     "get_reviews",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request reviews",
		},
		{
			name: "lockdown enabled filters reviews without push access",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposPullsReviewsByOwnerByRepoByPullNumber,
					[]*github.PullRequestReview{
						{
							ID:    github.Ptr(int64(2030)),
							State: github.Ptr("APPROVED"),
							Body:  github.Ptr("Maintainer review"),
							User:  &github.User{Login: github.Ptr("maintainer")},
						},
						{
							ID:    github.Ptr(int64(2031)),
							State: github.Ptr("COMMENTED"),
							Body:  github.Ptr("External reviewer"),
							User:  &github.User{Login: github.Ptr("testuser")},
						},
					},
				),
			),
			gqlHTTPClient: newRepoAccessHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":     "get_reviews",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError: false,
			expectedReviews: []*github.PullRequestReview{
				{
					ID:    github.Ptr(int64(2030)),
					State: github.Ptr("APPROVED"),
					Body:  github.Ptr("Maintainer review"),
					User:  &github.User{Login: github.Ptr("maintainer")},
				},
			},
			lockdownEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			var gqlClient *githubv4.Client
			if tc.gqlHTTPClient != nil {
				gqlClient = githubv4.NewClient(tc.gqlHTTPClient)
			} else {
				gqlClient = githubv4.NewClient(nil)
			}
			cache := stubRepoAccessCache(gqlClient, 5*time.Minute)
			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": tc.lockdownEnabled})
			serverTool := PullRequestRead(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:          client,
				RepoAccessCache: cache,
				Flags:           flags,
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
			var returnedReviews []*github.PullRequestReview
			err = json.Unmarshal([]byte(textContent.Text), &returnedReviews)
			require.NoError(t, err)
			assert.Len(t, returnedReviews, len(tc.expectedReviews))
			for i, review := range returnedReviews {
				require.NotNil(t, tc.expectedReviews[i].User)
				require.NotNil(t, review.User)
				assert.Equal(t, tc.expectedReviews[i].GetID(), review.GetID())
				assert.Equal(t, tc.expectedReviews[i].GetState(), review.GetState())
				assert.Equal(t, tc.expectedReviews[i].GetBody(), review.GetBody())
				assert.Equal(t, tc.expectedReviews[i].GetUser().GetLogin(), review.GetUser().GetLogin())
				assert.Equal(t, tc.expectedReviews[i].GetHTMLURL(), review.GetHTMLURL())
			}
		})
	}
}

func Test_CreatePullRequest(t *testing.T) {
	// Verify tool definition once
	serverTool := CreatePullRequest(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "create_pull_request", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "title")
	assert.Contains(t, schema.Properties, "body")
	assert.Contains(t, schema.Properties, "head")
	assert.Contains(t, schema.Properties, "base")
	assert.Contains(t, schema.Properties, "draft")
	assert.Contains(t, schema.Properties, "maintainer_can_modify")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "title", "head", "base"})

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
			serverTool := CreatePullRequest(translations.NullTranslationHelper)
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

			// Unmarshal and verify the minimal result
			var returnedPR MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &returnedPR)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPR.GetHTMLURL(), returnedPR.URL)
		})
	}
}

func TestCreateAndSubmitPullRequestReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_review_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "body")
	assert.Contains(t, schema.Properties, "event")
	assert.Contains(t, schema.Properties, "commitID")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "create",
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
				"method":     "create",
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
				"method":     "create",
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
			serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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

	serverTool := RequestCopilotReview(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "request_copilot_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber"})

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
			serverTool := RequestCopilotReview(translations.NullTranslationHelper)
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
	serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_review_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "commitID")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "create",
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
				"method":     "create",
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
				"method":     "create",
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
			serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			textContent := getTextResult(t, result)

			if tc.expectToolError {
				require.True(t, result.IsError)
				assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				return
			}

			// Parse the result and get the text content if no error
			require.Equal(t, "pending pull request created", textContent.Text)
		})
	}
}

func TestAddPullRequestReviewCommentToPendingReview(t *testing.T) {
	t.Parallel()

	// Verify tool definition once
	serverTool := AddCommentToPendingReview(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "add_comment_to_pending_review", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "path")
	assert.Contains(t, schema.Properties, "body")
	assert.Contains(t, schema.Properties, "subjectType")
	assert.Contains(t, schema.Properties, "line")
	assert.Contains(t, schema.Properties, "side")
	assert.Contains(t, schema.Properties, "startLine")
	assert.Contains(t, schema.Properties, "startSide")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumber", "path", "body", "subjectType"})

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
					githubv4mock.DataResponse(map[string]any{
						"addPullRequestReviewThread": map[string]any{
							"thread": map[string]any{
								"id": "MDEyOlB1bGxSZXF1ZXN0UmV2aWV3VGhyZWFkMTIzNDU2",
							},
						},
					}),
				),
			),
		},
		{
			name: "thread ID is nil - invalid line number",
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"pullNumber":  float64(42),
				"path":        "file.go",
				"body":        "Comment on non-existent line",
				"subjectType": "LINE",
				"line":        float64(999),
				"side":        "RIGHT",
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
								ID githubv4.ID
							}
						} `graphql:"addPullRequestReviewThread(input: $input)"`
					}{},
					githubv4.AddPullRequestReviewThreadInput{
						Path:                githubv4.String("file.go"),
						Body:                githubv4.String("Comment on non-existent line"),
						SubjectType:         githubv4mock.Ptr(githubv4.PullRequestReviewThreadSubjectTypeLine),
						Line:                githubv4.NewInt(999),
						Side:                githubv4mock.Ptr(githubv4.DiffSideRight),
						StartLine:           nil,
						StartSide:           nil,
						PullRequestReviewID: githubv4.NewID("PR_kwDODKw3uc6WYN1T"),
					},
					nil,
					githubv4mock.DataResponse(map[string]any{
						"addPullRequestReviewThread": map[string]any{
							"thread": map[string]any{
								"id": nil,
							},
						},
					}),
				),
			),
			expectToolError:    true,
			expectedToolErrMsg: "Failed to add comment to pending review",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Setup client with mock
			client := githubv4.NewClient(tc.mockedClient)
			serverTool := AddCommentToPendingReview(translations.NullTranslationHelper)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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
	serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_review_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.Contains(t, schema.Properties, "event")
	assert.Contains(t, schema.Properties, "body")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "submit_pending",
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
			serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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
	serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_review_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "delete_pending",
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
			serverTool := PullRequestReviewWrite(translations.NullTranslationHelper)
			deps := BaseDeps{
				GQLClient: client,
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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
	serverTool := PullRequestRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "pull_request_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo", "pullNumber"})

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
				"method":     "get_diff",
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
			serverTool := PullRequestRead(translations.NullTranslationHelper)
			deps := BaseDeps{
				Client:          client,
				RepoAccessCache: stubRepoAccessCache(githubv4.NewClient(nil), 5*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
			}
			handler := serverTool.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
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
