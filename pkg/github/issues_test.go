package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/http/headers"
	transportpkg "github.com/github/github-mcp-server/pkg/http/transport"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var defaultGQLClient *githubv4.Client = githubv4.NewClient(newRepoAccessHTTPClient())

type repoAccessKey struct {
	owner string
	repo  string
}

type repoAccessValue struct {
	isPrivate bool
}

type repoAccessMockTransport struct {
	responses map[repoAccessKey]repoAccessValue
}

func newRepoAccessHTTPClient() *http.Client {
	responses := map[repoAccessKey]repoAccessValue{
		{owner: "owner2", repo: "repo2"}: {isPrivate: true},
		{owner: "owner", repo: "repo"}:   {isPrivate: false},
	}

	return &http.Client{Transport: &repoAccessMockTransport{responses: responses}}
}

func (rt *repoAccessMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil {
		return nil, fmt.Errorf("missing request body")
	}

	var payload struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}

	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		return nil, err
	}
	_ = req.Body.Close()

	owner := toString(payload.Variables["owner"])
	repo := toString(payload.Variables["name"])

	value, ok := rt.responses[repoAccessKey{owner: owner, repo: repo}]
	if !ok {
		value = repoAccessValue{isPrivate: false}
	}

	data := map[string]any{}
	if strings.Contains(payload.Query, "viewer") {
		data["viewer"] = map[string]any{"login": "test-viewer"}
	}
	if strings.Contains(payload.Query, "repository") {
		data["repository"] = map[string]any{"isPrivate": value.isPrivate}
	}

	responseBody, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		return nil, err
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", value)
	}
}

func Test_GetIssue(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number"})

	// Setup mock issue for success case
	mockIssue := &github.Issue{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Test Issue"),
		Body:    github.Ptr("This is a test issue"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Repository: &github.Repository{
			Name: github.Ptr("repo"),
			Owner: &github.User{
				Login: github.Ptr("owner"),
			},
		},
	}
	mockIssue2 := &github.Issue{
		Number:  github.Ptr(422),
		Title:   github.Ptr("Test Issue 2"),
		Body:    github.Ptr("This is a test issue 2"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		User: &github.User{
			Login: github.Ptr("testuser2"),
		},
		Repository: &github.Repository{
			Name: github.Ptr("repo2"),
			Owner: &github.User{
				Login: github.Ptr("owner2"),
			},
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectHandlerError bool
		expectResultError  bool
		expectedIssue      *github.Issue
		expectedErrMsg     string
		lockdownEnabled    bool
		restPermission     string
	}{
		{
			name: "successful issue retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "get",
				"owner":        "owner2",
				"repo":         "repo2",
				"issue_number": float64(42),
			},
			expectedIssue: mockIssue,
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectHandlerError: true,
			expectedErrMsg:     "failed to get issue",
		},
		{
			name: "lockdown enabled - private repository",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue2),
			}),
			requestArgs: map[string]any{
				"method":       "get",
				"owner":        "owner2",
				"repo":         "repo2",
				"issue_number": float64(422),
			},
			expectedIssue:   mockIssue2,
			lockdownEnabled: true,
			restPermission:  "none",
		},
		{
			name: "lockdown enabled - user lacks push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "get",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectResultError: true,
			expectedErrMsg:    "access to issue details is restricted by lockdown mode",
			lockdownEnabled:   true,
			restPermission:    "read",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)

			var restClient *github.Client
			if tc.restPermission != "" {
				restClient = mockRESTPermissionServer(t, tc.restPermission, nil)
			}
			cache := stubRepoAccessCache(restClient, 15*time.Minute)

			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": tc.lockdownEnabled})
			deps := BaseDeps{
				Client:          client,
				GQLClient:       defaultGQLClient,
				RepoAccessCache: cache,
				Flags:           flags,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectHandlerError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectResultError {
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			textContent := getTextResult(t, result)

			var returnedIssue MinimalIssue
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedIssue.GetNumber(), returnedIssue.Number)
			assert.Equal(t, tc.expectedIssue.GetTitle(), returnedIssue.Title)
			assert.Equal(t, tc.expectedIssue.GetBody(), returnedIssue.Body)
			assert.Equal(t, tc.expectedIssue.GetState(), returnedIssue.State)
			assert.Equal(t, tc.expectedIssue.GetHTMLURL(), returnedIssue.HTMLURL)
			assert.Equal(t, tc.expectedIssue.GetUser().GetLogin(), returnedIssue.User.Login)
		})
	}
}

func Test_IssueRead_IFC_InsidersMode(t *testing.T) {
	t.Parallel()

	serverTool := IssueRead(translations.NullTranslationHelper)

	mockIssue := &github.Issue{
		Number:  github.Ptr(1),
		Title:   github.Ptr("Test"),
		Body:    github.Ptr("body"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/octocat/repo/issues/1"),
		User:    &github.User{Login: github.Ptr("u")},
	}

	mockComments := []*github.IssueComment{
		{Body: github.Ptr("hello"), User: &github.User{Login: github.Ptr("u")}},
	}

	makeMockClient := func(isPrivate bool, repoStatus int) *http.Client {
		handlers := map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber:         mockResponse(t, http.StatusOK, mockIssue),
			GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockComments),
		}
		if repoStatus != 0 && repoStatus != http.StatusOK {
			handlers[GetReposByOwnerByRepo] = mockResponse(t, repoStatus, "boom")
		} else {
			handlers[GetReposByOwnerByRepo] = mockResponse(t, http.StatusOK, map[string]any{
				"name":    "repo",
				"private": isPrivate,
			})
		}
		return MockHTTPClientWithHandlers(handlers)
	}

	getReq := map[string]any{
		"method":       "get",
		"owner":        "octocat",
		"repo":         "repo",
		"issue_number": float64(1),
	}
	commentsReq := map[string]any{
		"method":       "get_comments",
		"owner":        "octocat",
		"repo":         "repo",
		"issue_number": float64(1),
	}

	t.Run("insiders mode disabled omits ifc label", func(t *testing.T) {
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient(false, 0)),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(getReq)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Nil(t, result.Meta)
	})

	t.Run("insiders mode enabled on public repo emits public untrusted", func(t *testing.T) {
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(false, 0)),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(getReq)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
	})

	t.Run("insiders mode enabled on private repo with get_comments emits private trusted", func(t *testing.T) {
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(true, 0)),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(commentsReq)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcMap := unmarshalIFC(t, result.Meta["ifc"])
		assert.Equal(t, "trusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})

	t.Run("insiders mode skips ifc label when visibility lookup fails", func(t *testing.T) {
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(false, http.StatusInternalServerError)),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(getReq)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "tool call should still succeed when visibility lookup fails")

		if result.Meta != nil {
			_, hasIFC := result.Meta["ifc"]
			assert.False(t, hasIFC, "ifc label should be omitted when visibility lookup fails")
		}
	})
}

func Test_GetIssue_FieldValues(t *testing.T) {
	// The raw REST issue_field_values are always cleared. Enriched field_values are
	// only populated via GraphQL when the issue has a node ID; this issue has none,
	// so field_values stays empty.
	serverTool := IssueRead(translations.NullTranslationHelper)

	mockIssueWithFields := &github.Issue{
		Number:  github.Ptr(99),
		Title:   github.Ptr("Issue with field values"),
		Body:    github.Ptr("body"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/99"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		IssueFieldValues: []*github.IssueFieldValue{
			{
				IssueFieldID: 1001,
				NodeID:       "FV_node_1",
				DataType:     "single_select",
				Value:        "High",
				SingleSelectOption: &github.IssueFieldValueSingleSelectOption{
					ID:    42,
					Name:  "High",
					Color: "red",
				},
			},
			{
				IssueFieldID: 1002,
				NodeID:       "FV_node_2",
				DataType:     "text",
				Value:        "some text value",
			},
		},
	}

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssueWithFields),
	})

	cache := stubRepoAccessCache(nil, 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	deps := BaseDeps{
		Client:          mustNewGHClient(t, mockedClient),
		GQLClient:       defaultGQLClient,
		RepoAccessCache: cache,
		Flags:           flags,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":       "get",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(99),
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)

	textContent := getTextResult(t, result)

	var returnedIssue MinimalIssue
	err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
	require.NoError(t, err)

	// Raw REST IssueFieldValues must be cleared, and no enriched field_values are
	// present because this issue has no node ID.
	assert.Empty(t, returnedIssue.IssueFieldValues, "raw REST issue_field_values should not be exposed")
	assert.Empty(t, returnedIssue.FieldValues, "enriched field_values should not be present without a node ID")
}

func Test_GetIssue_FieldValues_Enriched(t *testing.T) {
	// Verify the enriched field_values are populated via GraphQL when the issue has
	// a node ID, and the raw REST issue_field_values stays cleared.
	serverTool := IssueRead(translations.NullTranslationHelper)

	mockIssueWithFields := &github.Issue{
		Number:  github.Ptr(99),
		NodeID:  github.Ptr("I_node_99"),
		Title:   github.Ptr("Issue with field values"),
		Body:    github.Ptr("body"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/99"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		IssueFieldValues: []*github.IssueFieldValue{
			{
				IssueFieldID: 1001,
				NodeID:       "FV_node_1",
				DataType:     "single_select",
				Value:        "High",
			},
		},
	}

	restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssueWithFields),
	})

	gqlVars := map[string]any{
		"ids": []any{"I_node_99"},
	}
	gqlResponse := githubv4mock.DataResponse(map[string]any{
		"nodes": []map[string]any{
			{
				"id": "I_node_99",
				"issueFieldValues": map[string]any{
					"nodes": []map[string]any{
						{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"name": "priority"},
							"value":      "P1",
						},
						{
							"__typename":  "IssueFieldNumberValue",
							"field":       map[string]any{"name": "estimate"},
							"valueNumber": 2.5,
						},
					},
				},
			},
		},
	})

	const nodesQueryString = "query($ids:[ID!]!){nodes(ids: $ids){... on Issue{id,issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}}}}"
	matcher := githubv4mock.NewQueryMatcher(nodesQueryString, gqlVars, gqlResponse)
	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher))

	cache := stubRepoAccessCache(nil, 15*time.Minute)
	deps := BaseDeps{
		Client:          mustNewGHClient(t, restClient),
		GQLClient:       gqlClient,
		RepoAccessCache: cache,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":       "get",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(99),
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "expected result to not be an error")

	textContent := getTextResult(t, result)

	var returnedIssue MinimalIssue
	err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
	require.NoError(t, err)

	// Raw REST IssueFieldValues is always cleared.
	assert.Empty(t, returnedIssue.IssueFieldValues, "raw REST issue_field_values should not be exposed")

	// Enriched FieldValues comes from the GraphQL nodes() round-trip.
	require.Len(t, returnedIssue.FieldValues, 2, "field_values should be populated from GraphQL")
	assert.Equal(t, "priority", returnedIssue.FieldValues[0].Field)
	assert.Equal(t, "P1", returnedIssue.FieldValues[0].Value)
	assert.Equal(t, "estimate", returnedIssue.FieldValues[1].Field)
	assert.Equal(t, "2.5", returnedIssue.FieldValues[1].Value)
}

func Test_AddIssueComment(t *testing.T) {
	// Verify tool definition once
	serverTool := AddIssueComment(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "add_issue_comment", tool.Name)
	assert.NotEmpty(t, tool.Description)

	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "body")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo", "issue_number", "body"})

	// Setup mock comment for success case
	mockComment := &github.IssueComment{
		ID:   github.Ptr(int64(123)),
		Body: github.Ptr("This is a test comment"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42#issuecomment-123"),
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]any
		expectError     bool
		expectedComment *github.IssueComment
		expectedErrMsg  string
	}{
		{
			name: "successful comment creation",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, mockComment),
			}),
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"body":         "This is a test comment",
			},
			expectError:     false,
			expectedComment: mockComment,
		},
		{
			name: "comment creation fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesCommentsByOwnerByRepoByIssueNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Invalid request"}`))
				}),
			}),
			requestArgs: map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"body":         "",
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: body",
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

			// Unmarshal and verify the result contains minimal response
			var minimalResponse MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &minimalResponse)
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%d", tc.expectedComment.GetID()), minimalResponse.ID)
			assert.Equal(t, tc.expectedComment.GetHTMLURL(), minimalResponse.URL)
		})
	}
}

func Test_SearchIssues(t *testing.T) {
	// Verify tool definition once
	serverTool := SearchIssues(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_issues", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "query")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "sort")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "order")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "perPage")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "page")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"query"})

	// Setup mock search results
	mockSearchResult := &github.IssuesSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		Issues: []*github.Issue{
			{
				Number:   github.Ptr(42),
				Title:    github.Ptr("Bug: Something is broken"),
				Body:     github.Ptr("This is a bug report"),
				State:    github.Ptr("open"),
				HTMLURL:  github.Ptr("https://github.com/owner/repo/issues/42"),
				Comments: github.Ptr(5),
				User: &github.User{
					Login: github.Ptr("user1"),
				},
			},
			{
				Number:   github.Ptr(43),
				Title:    github.Ptr("Feature: Add new functionality"),
				Body:     github.Ptr("This is a feature request"),
				State:    github.Ptr("open"),
				HTMLURL:  github.Ptr("https://github.com/owner/repo/issues/43"),
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
		requestArgs    map[string]any
		expectError    bool
		expectedResult *github.IssuesSearchResult
		expectedErrMsg string
	}{
		{
			name: "successful issues search with all parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue repo:owner/repo is:open",
						"sort":     "created",
						"order":    "desc",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
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
			name: "issues search with owner and repo parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "repo:test-owner/test-repo is:issue is:open",
						"sort":     "created",
						"order":    "asc",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "is:open",
				"owner": "test-owner",
				"repo":  "test-repo",
				"sort":  "created",
				"order": "asc",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "issues search with only owner parameter (should ignore it)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue bug",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "bug",
				"owner": "test-owner",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "issues search with only repo parameter (should ignore it)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue feature",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "feature",
				"repo":  "test-repo",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "issues search with minimal parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: mockResponse(t, http.StatusOK, mockSearchResult),
			}),
			requestArgs: map[string]any{
				"query": "is:issue repo:owner/repo is:open",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing is:issue filter - no duplication",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "repo:github/github-mcp-server is:issue is:open (label:critical OR label:urgent)",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "repo:github/github-mcp-server is:issue is:open (label:critical OR label:urgent)",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with existing repo: filter and conflicting owner/repo params - uses query filter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue repo:github/github-mcp-server critical",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "repo:github/github-mcp-server critical",
				"owner": "different-owner",
				"repo":  "different-repo",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with both is: and repo: filters already present",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue repo:octocat/Hello-World bug",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "is:issue repo:octocat/Hello-World bug",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "complex query with multiple OR operators and existing filters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "repo:github/github-mcp-server is:issue (label:critical OR label:urgent OR label:high-priority OR label:blocker)",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "repo:github/github-mcp-server is:issue (label:critical OR label:urgent OR label:high-priority OR label:blocker)",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query with field. qualifier enables advanced_search",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":               "is:issue field.priority:P1",
						"page":            "1",
						"per_page":        "30",
						"advanced_search": "true",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "field.priority:P1",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "query without field. qualifier does not set advanced_search",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: expectQueryParams(
					t,
					map[string]string{
						"q":        "is:issue is:open",
						"page":     "1",
						"per_page": "30",
					},
				).andThen(
					mockResponse(t, http.StatusOK, mockSearchResult),
				),
			}),
			requestArgs: map[string]any{
				"query": "is:open",
			},
			expectError:    false,
			expectedResult: mockSearchResult,
		},
		{
			name: "search issues fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetSearchIssues: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"query": "invalid:query",
			},
			expectError:    true,
			expectedErrMsg: "failed to search issues",
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
				require.NoError(t, err) // No Go error, but result should be an error
				require.NotNil(t, result)
				require.True(t, result.IsError, "expected result to be an error")
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError, "expected result to not be an error")

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

func Test_SearchIssues_IFC_InsidersMode(t *testing.T) {
	t.Parallel()

	serverTool := SearchIssues(translations.NullTranslationHelper)

	makeIssue := func(owner, repo string, number int) *github.Issue {
		return &github.Issue{
			Number:        github.Ptr(number),
			Title:         github.Ptr("issue"),
			State:         github.Ptr("open"),
			RepositoryURL: github.Ptr("https://api.github.com/repos/" + owner + "/" + repo),
			User:          &github.User{Login: github.Ptr("u")},
		}
	}

	type repoFixture struct {
		owner      string
		repo       string
		isPrivate  bool
		repoStatus int
	}

	repoHandlers := func(repos []repoFixture) map[string]http.HandlerFunc {
		repoByPath := map[string]repoFixture{}
		for _, r := range repos {
			repoByPath["/repos/"+r.owner+"/"+r.repo] = r
		}
		return map[string]http.HandlerFunc{
			GetReposByOwnerByRepo: func(w http.ResponseWriter, req *http.Request) {
				r, ok := repoByPath[req.URL.Path]
				if !ok {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.repoStatus != 0 && r.repoStatus != http.StatusOK {
					w.WriteHeader(r.repoStatus)
					return
				}
				body, _ := json.Marshal(map[string]any{
					"name":    r.repo,
					"private": r.isPrivate,
				})
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(body)
			},
		}
	}

	makeMockClient := func(searchResult *github.IssuesSearchResult, repos []repoFixture) *http.Client {
		handlers := repoHandlers(repos)
		handlers[GetSearchIssues] = mockResponse(t, http.StatusOK, searchResult)
		return MockHTTPClientWithHandlers(handlers)
	}

	reqParams := map[string]any{"query": "bug"}

	t.Run("insiders mode disabled omits ifc label", func(t *testing.T) {
		searchResult := &github.IssuesSearchResult{Issues: []*github.Issue{makeIssue("octocat", "public-repo", 1)}}
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient(searchResult, []repoFixture{{owner: "octocat", repo: "public-repo"}})),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Nil(t, result.Meta)
	})

	t.Run("insiders mode all public emits public untrusted", func(t *testing.T) {
		searchResult := &github.IssuesSearchResult{Issues: []*github.Issue{makeIssue("octocat", "public-repo", 1)}}
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(searchResult, []repoFixture{{owner: "octocat", repo: "public-repo"}})),
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
		searchResult := &github.IssuesSearchResult{Issues: []*github.Issue{
			makeIssue("octocat", "private-repo", 1),
			makeIssue("octocat", "public-repo", 2),
		}}
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient(searchResult, []repoFixture{
				{owner: "octocat", repo: "private-repo", isPrivate: true},
				{owner: "octocat", repo: "public-repo"},
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

	t.Run("insiders mode skips ifc label when visibility lookup fails", func(t *testing.T) {
		searchResult := &github.IssuesSearchResult{Issues: []*github.Issue{makeIssue("octocat", "broken", 1)}}
		deps := BaseDeps{
			Client: mustNewGHClient(t, makeMockClient(searchResult, []repoFixture{
				{owner: "octocat", repo: "broken", repoStatus: http.StatusInternalServerError},
			})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError, "tool call should still succeed when visibility lookup fails")

		if result.Meta != nil {
			_, hasIFC := result.Meta["ifc"]
			assert.False(t, hasIFC, "ifc label should be omitted when visibility lookup fails")
		}
	})

	t.Run("insiders mode empty results emits public untrusted", func(t *testing.T) {
		searchResult := &github.IssuesSearchResult{Issues: []*github.Issue{}}
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(searchResult, nil)),
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

func unmarshalIFC(t *testing.T, ifcLabel any) map[string]any {
	t.Helper()
	require.NotNil(t, ifcLabel, "ifc label should be present")
	ifcJSON, err := json.Marshal(ifcLabel)
	require.NoError(t, err)
	var ifcMap map[string]any
	require.NoError(t, json.Unmarshal(ifcJSON, &ifcMap))
	return ifcMap
}

func Test_SearchIssues_FieldValuesEnrichment(t *testing.T) {
	serverTool := SearchIssues(translations.NullTranslationHelper)

	mockSearchResult := &github.IssuesSearchResult{
		Total:             github.Ptr(2),
		IncompleteResults: github.Ptr(false),
		Issues: []*github.Issue{
			{
				Number:  github.Ptr(42),
				Title:   github.Ptr("Bug: Something is broken"),
				State:   github.Ptr("open"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
				NodeID:  github.Ptr("I_node_42"),
				User:    &github.User{Login: github.Ptr("user1")},
			},
			{
				Number:  github.Ptr(43),
				Title:   github.Ptr("Feature request"),
				State:   github.Ptr("open"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/issues/43"),
				NodeID:  github.Ptr("I_node_43"),
				User:    &github.User{Login: github.Ptr("user2")},
			},
		},
	}

	restClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetSearchIssues: mockResponse(t, http.StatusOK, mockSearchResult),
	})

	gqlVars := map[string]any{
		"ids": []any{"I_node_42", "I_node_43"},
	}
	gqlResponse := githubv4mock.DataResponse(map[string]any{
		"nodes": []map[string]any{
			{
				"id": "I_node_42",
				"issueFieldValues": map[string]any{
					"nodes": []map[string]any{
						{
							"__typename": "IssueFieldSingleSelectValue",
							"field":      map[string]any{"name": "priority"},
							"value":      "P1",
						},
						{
							"__typename":  "IssueFieldNumberValue",
							"field":       map[string]any{"name": "estimate"},
							"valueNumber": 2.5,
						},
					},
				},
			},
			{
				"id": "I_node_43",
				"issueFieldValues": map[string]any{
					"nodes": []map[string]any{},
				},
			},
		},
	})

	const nodesQueryString = "query($ids:[ID!]!){nodes(ids: $ids){... on Issue{id,issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}}}}"
	matcher := githubv4mock.NewQueryMatcher(nodesQueryString, gqlVars, gqlResponse)
	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher))

	deps := BaseDeps{
		Client:    mustNewGHClient(t, restClient),
		GQLClient: gqlClient,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"query": "repo:owner/repo is:open",
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, "expected result to not be an error")

	textContent := getTextResult(t, result)

	var response SearchIssuesResponse
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &response))
	require.Equal(t, 2, *response.Total)
	require.Len(t, response.Items, 2)
	assert.Equal(t, 42, *response.Items[0].Number)
	assert.Equal(t, []MinimalFieldValue{
		{Field: "priority", Value: "P1"},
		{Field: "estimate", Value: "2.5"},
	}, response.Items[0].FieldValues)
	assert.Equal(t, 43, *response.Items[1].Number)
	assert.Empty(t, response.Items[1].FieldValues)
}

func Test_CreateIssue(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Empty(t, serverTool.FeatureFlagEnable)

	assert.Equal(t, "issue_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "title")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "body")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "assignees")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "labels")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "milestone")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "type")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_fields")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo"})

	// Setup mock issue for success case
	mockIssue := &github.Issue{
		Number:    github.Ptr(123),
		Title:     github.Ptr("Test Issue"),
		Body:      github.Ptr("This is a test issue"),
		State:     github.Ptr("open"),
		HTMLURL:   github.Ptr("https://github.com/owner/repo/issues/123"),
		Assignees: []*github.User{{Login: github.Ptr("user1")}, {Login: github.Ptr("user2")}},
		Labels:    []*github.Label{{Name: github.Ptr("bug")}, {Name: github.Ptr("help wanted")}},
		Milestone: &github.Milestone{Number: github.Ptr(5)},
		Type:      &github.IssueType{Name: github.Ptr("Bug")},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		mockedGQLClient *http.Client
		requestArgs     map[string]any
		expectError     bool
		expectedIssue   *github.Issue
		expectedErrMsg  string
	}{
		{
			name: "successful issue creation with all fields",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"title":     "Test Issue",
					"body":      "This is a test issue",
					"labels":    []any{"bug", "help wanted"},
					"assignees": []any{"user1", "user2"},
					"milestone": float64(5),
					"type":      "Bug",
				}).andThen(
					mockResponse(t, http.StatusCreated, mockIssue),
				),
			}),
			requestArgs: map[string]any{
				"method":    "create",
				"owner":     "owner",
				"repo":      "repo",
				"title":     "Test Issue",
				"body":      "This is a test issue",
				"assignees": []any{"user1", "user2"},
				"labels":    []any{"bug", "help wanted"},
				"milestone": float64(5),
				"type":      "Bug",
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "successful issue creation with minimal fields",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepo: mockResponse(t, http.StatusCreated, &github.Issue{
					Number:  github.Ptr(124),
					Title:   github.Ptr("Minimal Issue"),
					HTMLURL: github.Ptr("https://github.com/owner/repo/issues/124"),
					State:   github.Ptr("open"),
				}),
			}),
			requestArgs: map[string]any{
				"method":    "create",
				"owner":     "owner",
				"repo":      "repo",
				"title":     "Minimal Issue",
				"assignees": nil, // Expect no failure with nil optional value.
			},
			expectError: false,
			expectedIssue: &github.Issue{
				Number:  github.Ptr(124),
				Title:   github.Ptr("Minimal Issue"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/issues/124"),
				State:   github.Ptr("open"),
			},
		},
		{
			name: "successful issue creation with issue fields reconciled by names",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"title":     "Issue with fields",
					"body":      "",
					"labels":    []any{},
					"assignees": []any{},
					"issue_field_values": []any{
						map[string]any{"field_id": float64(101), "value": "P1"},
						map[string]any{"field_id": float64(102), "value": "Acme"},
					},
				}).andThen(
					mockResponse(t, http.StatusCreated, &github.Issue{
						Number:  github.Ptr(125),
						Title:   github.Ptr("Issue with fields"),
						HTMLURL: github.Ptr("https://github.com/owner/repo/issues/125"),
						State:   github.Ptr("open"),
					}),
				),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					issueFieldWriteMetadataQuery{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"issueFields": map[string]any{
								"nodes": []any{
									map[string]any{
										"__typename":     "IssueFieldSingleSelect",
										"fullDatabaseId": "101",
										"name":           "Priority",
										"dataType":       "single_select",
										"options": []any{
											map[string]any{"fullDatabaseId": "9001", "name": "P1"},
										},
									},
									map[string]any{
										"__typename":     "IssueFieldText",
										"fullDatabaseId": "102",
										"name":           "Customer",
										"dataType":       "text",
									},
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "Issue with fields",
				"issue_fields": []any{
					map[string]any{"field_name": "Priority", "field_option_name": "P1"},
					map[string]any{"field_name": "Customer", "value": "Acme"},
				},
			},
			expectError: false,
			expectedIssue: &github.Issue{
				Number:  github.Ptr(125),
				Title:   github.Ptr("Issue with fields"),
				HTMLURL: github.Ptr("https://github.com/owner/repo/issues/125"),
				State:   github.Ptr("open"),
			},
		},
		{
			name: "issue creation fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Validation failed"}`))
				}),
			}),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "",
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: title",
		},
		{
			name:         "issue_fields rejects both value and field_option_name",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "create",
				"owner":  "owner",
				"repo":   "repo",
				"title":  "Invalid fields",
				"issue_fields": []any{
					map[string]any{"field_name": "Priority", "value": "P1", "field_option_name": "P1"},
				},
			},
			expectError:    false,
			expectedErrMsg: "cannot specify both value and field_option_name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
			gqlHTTPClient := tc.mockedGQLClient
			if gqlHTTPClient == nil {
				gqlHTTPClient = githubv4mock.NewMockedHTTPClient()
			}
			gqlClient := githubv4.NewClient(gqlHTTPClient)
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
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var returnedIssue MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedIssue.GetHTMLURL(), returnedIssue.URL)
		})
	}
}

// Test_IssueWrite_MCPAppsFeature_UIGate verifies the MCP Apps feature UI gate
// behavior: UI clients get a form message, non-UI clients execute directly.
func Test_IssueWrite_MCPAppsFeature_UIGate(t *testing.T) {
	t.Parallel()

	mockIssue := &github.Issue{
		Number:  github.Ptr(1),
		Title:   github.Ptr("Test"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/1"),
	}

	serverTool := IssueWrite(translations.NullTranslationHelper)

	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PostReposIssuesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockIssue),
	}))

	deps := BaseDeps{
		Client:         client,
		GQLClient:      githubv4.NewClient(nil),
		featureChecker: featureCheckerFor(MCPAppsFeatureFlag),
	}
	handler := serverTool.Handler(deps)

	t.Run("UI client without _ui_submitted returns form message", func(t *testing.T) {
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"title":  "Test",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "interactive form has been shown to the user for creating a new issue")
		assert.True(t, result.IsError, "form-routing stub should be marked IsError so agents don't claim success")
	})

	t.Run("UI client with _ui_submitted executes directly", func(t *testing.T) {
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":        "create",
			"owner":         "owner",
			"repo":          "repo",
			"title":         "Test",
			"_ui_submitted": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"tool should return the created issue URL")
	})

	t.Run("non-UI client executes directly without _ui_submitted", func(t *testing.T) {
		request := createMCPRequest(map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"title":  "Test",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"non-UI client should execute directly")
	})

	t.Run("UI client with state change routes through UI form", func(t *testing.T) {
		// state/state_reason/duplicate_of are form params (the issue-write view
		// renders close/reopen controls), so a call carrying them must go to
		// the form rather than execute directly.
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":       "update",
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"state":        "closed",
			"state_reason": "completed",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "interactive form has been shown to the user for editing issue #1",
			"state change should route through UI form")
		assert.True(t, result.IsError, "form-routing stub should be marked IsError so agents don't claim success")
	})

	t.Run("UI client update without state change returns form message", func(t *testing.T) {
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":       "update",
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(1),
			"title":        "New Title",
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "interactive form has been shown to the user for editing issue #1",
			"update without state should show UI form")
		assert.True(t, result.IsError, "form-routing stub should be marked IsError so agents don't claim success")
	})

	t.Run("UI client with issue_fields routes through UI form", func(t *testing.T) {
		// issue_fields is now a form param (the issue-write view renders a
		// per-field editor), so a call carrying it must go to the form rather
		// than execute directly.
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"title":  "Issue with fields",
			"issue_fields": []any{
				map[string]any{"field_name": "Priority", "field_option_name": "P1"},
			},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "interactive form has been shown to the user for creating a new issue",
			"issue_fields should route through UI form")
		assert.True(t, result.IsError, "form-routing stub should be marked IsError so agents don't claim success")
	})

	t.Run("UI client with labels skips form and executes directly", func(t *testing.T) {
		// The form does not collect labels, so a call carrying them must bypass
		// the form rather than silently drop them.
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method": "create",
			"owner":  "owner",
			"repo":   "repo",
			"title":  "Test",
			"labels": []any{"bug"},
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.NotContains(t, textContent.Text, "interactive form has been shown",
			"labels should skip UI form")
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"labels call should execute directly and return issue URL")
	})

	t.Run("UI client with show_ui=false skips form and executes directly", func(t *testing.T) {
		// show_ui=false is the explicit, model-facing way to opt out of the
		// form. It must bypass the form even when every other condition would
		// route the call there (UI capability, MCP Apps flag on, no
		// _ui_submitted, only form params present).
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":  "create",
			"owner":   "owner",
			"repo":    "repo",
			"title":   "Test",
			"show_ui": false,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.NotContains(t, textContent.Text, "interactive form has been shown",
			"show_ui=false should skip UI form")
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"show_ui=false call should execute directly and return issue URL")
	})

	t.Run("UI client with show_ui=true returns form message", func(t *testing.T) {
		// show_ui=true is the explicit, redundant-with-the-default way to ask
		// for the form. It must still route through the form and must not be
		// treated as a non-form parameter that would trigger the safety-net
		// bypass.
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":  "create",
			"owner":   "owner",
			"repo":    "repo",
			"title":   "Test",
			"show_ui": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "interactive form has been shown",
			"show_ui=true should still route through the form")
	})

	t.Run("UI client with show_ui=false and _ui_submitted=true executes directly", func(t *testing.T) {
		// _ui_submitted and show_ui=false are two ways to say "execute
		// directly". When both are set there must be no conflict — the call
		// still executes directly.
		request := createMCPRequestWithSession(t, ClientNameVSCodeInsiders, true, map[string]any{
			"method":        "create",
			"owner":         "owner",
			"repo":          "repo",
			"title":         "Test",
			"show_ui":       false,
			"_ui_submitted": true,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"show_ui=false + _ui_submitted should execute directly")
	})

	t.Run("non-UI client with show_ui=false executes directly (no regression)", func(t *testing.T) {
		// show_ui is irrelevant when the client does not support UI; the call
		// must execute directly exactly as it does today.
		request := createMCPRequest(map[string]any{
			"method":  "create",
			"owner":   "owner",
			"repo":    "repo",
			"title":   "Test",
			"show_ui": false,
		})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)

		textContent := getTextResult(t, result)
		assert.Contains(t, textContent.Text, "https://github.com/owner/repo/issues/1",
			"non-UI client should execute directly regardless of show_ui")
	})
}

func Test_issueWriteHasNonFormParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
		want bool
	}{
		{name: "no params", args: map[string]any{}, want: false},
		{name: "only form params", args: map[string]any{"method": "create", "owner": "o", "repo": "r", "title": "t", "body": "b", "issue_number": float64(1), "_ui_submitted": true}, want: false},
		{name: "show_ui true is a form param", args: map[string]any{"title": "t", "show_ui": true}, want: false},
		{name: "show_ui false is a form param", args: map[string]any{"title": "t", "show_ui": false}, want: false},
		{name: "labels present", args: map[string]any{"title": "t", "labels": []any{"bug"}}, want: true},
		{name: "assignees present", args: map[string]any{"title": "t", "assignees": []any{"octocat"}}, want: true},
		{name: "milestone present", args: map[string]any{"title": "t", "milestone": float64(2)}, want: true},
		{name: "type present", args: map[string]any{"title": "t", "type": "Bug"}, want: true},
		{name: "issue_fields present", args: map[string]any{"issue_fields": []any{map[string]any{"field_name": "Priority"}}}, want: false},
		{name: "state present", args: map[string]any{"state": "closed"}, want: false},
		{name: "state_reason present", args: map[string]any{"state_reason": "completed"}, want: false},
		{name: "duplicate_of present", args: map[string]any{"duplicate_of": float64(7)}, want: false},
		{name: "nil value is ignored", args: map[string]any{"issue_fields": nil}, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, issueWriteHasNonFormParams(tc.args))
		})
	}
}

// Test_issueWriteSchemaClassification fails when a schema property is added
// without classifying it as either form-resendable (issueWriteFormParams) or
// known-non-form (knownNonForm below). Without this guard, an unclassified
// property would silently flip UI gating: form-incompatible fields would
// stop tripping the safety-net bypass and the form would drop their values.
func Test_issueWriteSchemaClassification(t *testing.T) {
	t.Parallel()

	// Schema properties the MCP App form cannot represent — their presence
	// must trigger the safety-net bypass via issueWriteHasNonFormParams.
	knownNonForm := map[string]struct{}{
		"assignees": {},
		"labels":    {},
		"milestone": {},
		"type":      {},
	}

	cases := []struct {
		name string
		tool inventory.ServerTool
	}{
		{name: "IssueWrite", tool: IssueWrite(translations.NullTranslationHelper)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			schema, ok := tc.tool.Tool.InputSchema.(*jsonschema.Schema)
			require.True(t, ok, "InputSchema should be *jsonschema.Schema")

			for prop := range schema.Properties {
				_, isForm := issueWriteFormParams[prop]
				_, isNonForm := knownNonForm[prop]

				assert.Falsef(t, isForm && isNonForm,
					"property %q is classified as both form-resendable and non-form — pick one", prop)
				assert.Truef(t, isForm || isNonForm,
					"property %q in %s schema is unclassified — add it to issueWriteFormParams (pkg/github/issues.go) "+
						"if the MCP App form can carry it on submit, otherwise add it to the knownNonForm allowlist in this test",
					prop, tc.name)
			}
		})
	}
}

func Test_ListIssues(t *testing.T) {
	// Verify tool definition
	serverTool := ListIssues(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
	require.Empty(t, serverTool.FeatureFlagEnable)

	assert.Equal(t, "list_issues", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "state")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "labels")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "orderBy")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "direction")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "since")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "after")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner", "repo"})

	// Mock issues data
	mockIssuesAll := []map[string]any{
		{
			"number":     123,
			"title":      "First Issue",
			"body":       "This is the first test issue",
			"state":      "OPEN",
			"databaseId": 1001,
			"createdAt":  "2023-01-01T00:00:00Z",
			"updatedAt":  "2023-01-01T00:00:00Z",
			"author":     map[string]any{"login": "user1"},
			"labels": map[string]any{
				"nodes": []map[string]any{
					{"name": "bug", "id": "label1", "description": "Bug label"},
				},
			},
			"comments": map[string]any{
				"totalCount": 5,
			},
			"issueFieldValues": map[string]any{
				"nodes": []map[string]any{
					{
						"__typename": "IssueFieldSingleSelectValue",
						"field":      map[string]any{"name": "priority"},
						"value":      "P1",
					},
				},
			},
		},
		{
			"number":     456,
			"title":      "Second Issue",
			"body":       "This is the second test issue",
			"state":      "OPEN",
			"databaseId": 1002,
			"createdAt":  "2023-02-01T00:00:00Z",
			"updatedAt":  "2023-02-01T00:00:00Z",
			"author":     map[string]any{"login": "user2"},
			"labels": map[string]any{
				"nodes": []map[string]any{
					{"name": "enhancement", "id": "label2", "description": "Enhancement label"},
				},
			},
			"comments": map[string]any{
				"totalCount": 3,
			},
			"issueFieldValues": map[string]any{
				"nodes": []map[string]any{
					{
						"__typename": "IssueFieldDateValue",
						"field":      map[string]any{"name": "due"},
						"value":      "2026-06-01",
					},
					{
						"__typename":  "IssueFieldNumberValue",
						"field":       map[string]any{"name": "estimate"},
						"valueNumber": 2.5,
					},
					{
						"__typename": "IssueFieldTextValue",
						"field":      map[string]any{"name": "notes"},
						"value":      "needs triage",
					},
				},
			},
		},
	}

	mockIssuesOpen := []map[string]any{mockIssuesAll[0], mockIssuesAll[1]}
	mockIssuesClosed := []map[string]any{
		{
			"number":     789,
			"title":      "Closed Issue",
			"body":       "This is a closed issue",
			"state":      "CLOSED",
			"databaseId": 1003,
			"createdAt":  "2023-03-01T00:00:00Z",
			"updatedAt":  "2023-03-01T00:00:00Z",
			"author":     map[string]any{"login": "user3"},
			"labels": map[string]any{
				"nodes": []map[string]any{},
			},
			"comments": map[string]any{
				"totalCount": 1,
			},
			"issueFieldValues": map[string]any{
				"nodes": []map[string]any{},
			},
		},
	}

	// Mock responses
	mockResponseListAll := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes": mockIssuesAll,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 2,
			},
			"isPrivate": false,
		},
	})

	mockResponseOpenOnly := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes": mockIssuesOpen,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 2,
			},
			"isPrivate": false,
		},
	})

	mockResponseClosedOnly := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes": mockIssuesClosed,
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
					"startCursor":     "",
					"endCursor":       "",
				},
				"totalCount": 1,
			},
			"isPrivate": false,
		},
	})

	mockErrorRepoNotFound := githubv4mock.ErrorResponse("repository not found")

	// Variables matching what GraphQL receives after JSON marshaling/unmarshaling.
	// issueFieldValues is always sent as an (empty by default) list because the query
	// declares the variable unconditionally; the server treats an empty list as no filter.
	varsListAll := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"OPEN", "CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	varsOpenOnly := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"OPEN"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	varsClosedOnly := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	varsWithLabels := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"states":           []any{"OPEN", "CLOSED"},
		"labels":           []any{"bug", "enhancement"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	varsRepoNotFound := map[string]any{
		"owner":            "owner",
		"repo":             "nonexistent-repo",
		"states":           []any{"OPEN", "CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	tests := []struct {
		name          string
		reqParams     map[string]any
		expectError   bool
		errContains   string
		expectedCount int
	}{
		{
			name: "list all issues",
			reqParams: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "filter by open state",
			reqParams: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"state": "OPEN",
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "filter by open state - lc",
			reqParams: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"state": "open",
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "filter by closed state",
			reqParams: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"state": "CLOSED",
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name: "filter by labels",
			reqParams: map[string]any{
				"owner":  "owner",
				"repo":   "repo",
				"labels": []any{"bug", "enhancement"},
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name: "repository not found error",
			reqParams: map[string]any{
				"owner": "owner",
				"repo":  "nonexistent-repo",
			},
			expectError: true,
			errContains: "repository not found",
		},
	}

	// Define the actual query strings that match the implementation
	issueFieldValuesSelection := "issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}"
	qBasicNoLabels := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount}," + issueFieldValuesSelection + "},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"
	qWithLabels := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$labels:[String!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount}," + issueFieldValuesSelection + "},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var httpClient *http.Client

			switch tc.name {
			case "list all issues":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoLabels, varsListAll, mockResponseListAll)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by open state":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoLabels, varsOpenOnly, mockResponseOpenOnly)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by open state - lc":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoLabels, varsOpenOnly, mockResponseOpenOnly)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by closed state":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoLabels, varsClosedOnly, mockResponseClosedOnly)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "filter by labels":
				matcher := githubv4mock.NewQueryMatcher(qWithLabels, varsWithLabels, mockResponseListAll)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			case "repository not found error":
				matcher := githubv4mock.NewQueryMatcher(qBasicNoLabels, varsRepoNotFound, mockErrorRepoNotFound)
				httpClient = githubv4mock.NewMockedHTTPClient(matcher)
			}

			gqlClient := githubv4.NewClient(httpClient)
			deps := BaseDeps{
				GQLClient: gqlClient,
			}
			handler := serverTool.Handler(deps)

			req := createMCPRequest(tc.reqParams)
			res, err := handler(ContextWithDeps(context.Background(), deps), &req)
			text := getTextResult(t, res).Text

			if tc.expectError {
				require.True(t, res.IsError)
				assert.Contains(t, text, tc.errContains)
				return
			}
			require.NoError(t, err)

			// Parse the structured response with pagination info
			var response MinimalIssuesResponse
			err = json.Unmarshal([]byte(text), &response)
			require.NoError(t, err)

			assert.Len(t, response.Issues, tc.expectedCount, "Expected %d issues, got %d", tc.expectedCount, len(response.Issues))

			// Verify pagination metadata
			assert.Equal(t, tc.expectedCount, response.TotalCount)
			assert.False(t, response.PageInfo.HasNextPage)
			assert.False(t, response.PageInfo.HasPreviousPage)

			// Verify that returned issues have expected structure
			for _, issue := range response.Issues {
				assert.NotZero(t, issue.Number, "Issue should have number")
				assert.NotEmpty(t, issue.Title, "Issue should have title")
				assert.NotEmpty(t, issue.State, "Issue should have state")
				assert.NotEmpty(t, issue.CreatedAt, "Issue should have created_at")
				assert.NotEmpty(t, issue.UpdatedAt, "Issue should have updated_at")
				assert.NotNil(t, issue.User, "Issue should have user")
				assert.NotEmpty(t, issue.User.Login, "Issue user should have login")
				assert.Empty(t, issue.HTMLURL, "html_url should be empty (not populated by GraphQL fragment)")

				// Labels should be flattened to name strings
				for _, label := range issue.Labels {
					assert.NotEmpty(t, label, "Label should be a non-empty string")
				}

				// Field values should be flattened to {field, value} pairs. Issue #123 has a
				// SingleSelectValue; issue #456 exercises the Date/Number/Text branches
				// (including float formatting); #789 has no field values.
				switch issue.Number {
				case 123:
					assert.Equal(t, []MinimalFieldValue{{Field: "priority", Value: "P1"}}, issue.FieldValues)
				case 456:
					assert.Equal(t, []MinimalFieldValue{
						{Field: "due", Value: "2026-06-01"},
						{Field: "estimate", Value: "2.5"},
						{Field: "notes", Value: "needs triage"},
					}, issue.FieldValues)
				default:
					assert.Empty(t, issue.FieldValues)
				}
			}
		})
	}
}

func Test_ListIssues_FieldFilters(t *testing.T) {
	t.Parallel()

	serverTool := ListIssues(translations.NullTranslationHelper)

	mockIssues := []map[string]any{
		{
			"number":     1,
			"title":      "An issue",
			"body":       "body",
			"state":      "OPEN",
			"databaseId": 1,
			"createdAt":  "2026-01-01T00:00:00Z",
			"updatedAt":  "2026-01-01T00:00:00Z",
			"author":     map[string]any{"login": "user1"},
			"labels":     map[string]any{"nodes": []map[string]any{}},
			"comments":   map[string]any{"totalCount": 0},
		},
	}

	pageInfo := map[string]any{
		"hasNextPage":     false,
		"hasPreviousPage": false,
		"startCursor":     "",
		"endCursor":       "",
	}

	response := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issues": map[string]any{
				"nodes":      mockIssues,
				"pageInfo":   pageInfo,
				"totalCount": 1,
			},
			"isPrivate": false,
		},
	})

	// Field-lookup matcher used by every subtest that supplies field_filters.
	// The handler calls fetchIssueFields(owner, repo) before issuing the issues query.
	fieldsResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issueFields": map[string]any{
				"nodes": []any{
					map[string]any{
						"__typename": "IssueFieldSingleSelect",
						"id":         "IFSS_1",
						"name":       "Priority",
						"dataType":   "SINGLE_SELECT",
						"visibility": "ALL",
						"options": []any{
							map[string]any{"id": "OPT_P1", "name": "P1", "color": "red"},
							map[string]any{"id": "OPT_P2", "name": "P2", "color": "yellow"},
						},
					},
					map[string]any{
						"__typename": "IssueFieldText",
						"id":         "IFT_1",
						"name":       "Notes",
						"dataType":   "TEXT",
						"visibility": "ALL",
					},
					map[string]any{
						"__typename": "IssueFieldNumber",
						"id":         "IFN_1",
						"name":       "Estimate",
						"dataType":   "NUMBER",
						"visibility": "ALL",
					},
					map[string]any{
						"__typename": "IssueFieldDate",
						"id":         "IFD_1",
						"name":       "Due",
						"dataType":   "DATE",
						"visibility": "ALL",
					},
				},
			},
		},
	})
	fieldsMatcher := func() githubv4mock.Matcher {
		return githubv4mock.NewQueryMatcher(
			issueFieldsRepoQuery{},
			map[string]any{
				"owner": githubv4.String("owner"),
				"name":  githubv4.String("repo"),
			},
			fieldsResponse,
		)
	}

	qNoLabels := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount},issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"
	qWithLabels := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$labels:[String!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount},issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"

	baseVars := func() map[string]any {
		return map[string]any{
			"owner":     "owner",
			"repo":      "repo",
			"states":    []any{"OPEN", "CLOSED"},
			"orderBy":   "CREATED_AT",
			"direction": "DESC",
			"first":     float64(30),
			"after":     (*string)(nil),
		}
	}

	t.Run("single select field filter", func(t *testing.T) {
		vars := baseVars()
		vars["issueFieldValues"] = []any{
			map[string]any{"fieldName": "Priority", "singleSelectOptionValue": "P1"},
		}
		matcher := githubv4mock.NewQueryMatcher(qNoLabels, vars, response)
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Priority", "value": "P1"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
	})

	t.Run("text field filter combined with labels", func(t *testing.T) {
		vars := baseVars()
		vars["labels"] = []any{"bug"}
		vars["issueFieldValues"] = []any{
			map[string]any{"fieldName": "Notes", "textValue": "needs triage"},
		}
		matcher := githubv4mock.NewQueryMatcher(qWithLabels, vars, response)
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"labels": []any{"bug"},
			"field_filters": []any{
				map[string]any{"field_name": "Notes", "value": "needs triage"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
	})

	t.Run("number and date field filters", func(t *testing.T) {
		vars := baseVars()
		vars["issueFieldValues"] = []any{
			map[string]any{"fieldName": "Estimate", "numberValue": float64(2.5)},
			map[string]any{"fieldName": "Due", "dateValue": "2026-06-01"},
		}
		matcher := githubv4mock.NewQueryMatcher(qNoLabels, vars, response)
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Estimate", "value": "2.5"},
				map[string]any{"field_name": "Due", "value": "2026-06-01"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
	})

	t.Run("number field accepts zero values", func(t *testing.T) {
		for _, value := range []string{"0", "0.0"} {
			t.Run(value, func(t *testing.T) {
				vars := baseVars()
				vars["issueFieldValues"] = []any{
					map[string]any{"fieldName": "Estimate", "numberValue": float64(0)},
				}
				matcher := githubv4mock.NewQueryMatcher(qNoLabels, vars, response)
				gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
				deps := BaseDeps{GQLClient: gqlClient}
				handler := serverTool.Handler(deps)

				req := createMCPRequest(map[string]any{
					"owner": "owner",
					"repo":  "repo",
					"field_filters": []any{
						map[string]any{"field_name": "Estimate", "value": value},
					},
				})
				res, err := handler(ContextWithDeps(context.Background(), deps), &req)
				require.NoError(t, err)
				require.False(t, res.IsError, getTextResult(t, res).Text)
			})
		}
	})

	t.Run("validation error when value missing", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher("", nil, response)))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Priority"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		text := getTextResult(t, res).Text
		assert.Contains(t, text, "field_filters entry")
		assert.Contains(t, text, "Priority")
		assert.Contains(t, text, "value")
	})

	t.Run("validation error when field_name missing", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(githubv4mock.NewQueryMatcher("", nil, response)))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"value": "P1"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		text := getTextResult(t, res).Text
		assert.Contains(t, text, "field_filters entry")
		assert.Contains(t, text, "field_name")
	})

	t.Run("error when field is unknown", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher()))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "NotARealField", "value": "x"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		text := getTextResult(t, res).Text
		assert.Contains(t, text, "unknown field")
		assert.Contains(t, text, "Priority")
	})

	t.Run("error when single-select option is invalid", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher()))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Priority", "value": "P9"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		text := getTextResult(t, res).Text
		assert.Contains(t, text, "not a valid option")
		assert.Contains(t, text, "P1")
	})

	t.Run("error when number value is non-numeric", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher()))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Estimate", "value": "not-a-number"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		assert.Contains(t, getTextResult(t, res).Text, "not a valid number")
	})

	t.Run("error when date value is malformed", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher()))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"field_filters": []any{
				map[string]any{"field_name": "Due", "value": "06/01/2026"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.True(t, res.IsError)
		assert.Contains(t, getTextResult(t, res).Text, "not a valid date")
	})

	// Query string fragments for the `since` variants. Built by string concatenation
	// because they only differ from the base variants by the variable declaration and
	// the filterBy clause.
	qNoLabelsWithSince := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$since:DateTime!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since, issueFieldValues: $issueFieldValues})" + qNoLabels[len("query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues})"):]
	qLabelsWithSince := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$labels:[String!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$since:DateTime!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since, issueFieldValues: $issueFieldValues})" + qWithLabels[len("query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$labels:[String!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues})"):]

	t.Run("field filter with since", func(t *testing.T) {
		vars := baseVars()
		vars["since"] = "2026-01-01T00:00:00Z"
		vars["issueFieldValues"] = []any{
			map[string]any{"fieldName": "Priority", "singleSelectOptionValue": "P1"},
		}
		matcher := githubv4mock.NewQueryMatcher(qNoLabelsWithSince, vars, response)
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
			"since": "2026-01-01T00:00:00Z",
			"field_filters": []any{
				map[string]any{"field_name": "Priority", "value": "P1"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
	})

	t.Run("field filter with labels and since", func(t *testing.T) {
		vars := baseVars()
		vars["labels"] = []any{"bug"}
		vars["since"] = "2026-01-01T00:00:00Z"
		vars["issueFieldValues"] = []any{
			map[string]any{"fieldName": "Priority", "singleSelectOptionValue": "P1"},
		}
		matcher := githubv4mock.NewQueryMatcher(qLabelsWithSince, vars, response)
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(fieldsMatcher(), matcher))
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{
			"owner":  "owner",
			"repo":   "repo",
			"labels": []any{"bug"},
			"since":  "2026-01-01T00:00:00Z",
			"field_filters": []any{
				map[string]any{"field_name": "Priority", "value": "P1"},
			},
		})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
	})

	t.Run("sends GraphQL-Features: issue_fields, repo_issue_fields header", func(t *testing.T) {
		vars := baseVars()
		vars["issueFieldValues"] = []any{}
		matcher := githubv4mock.NewQueryMatcher(qNoLabels, vars, response)

		// Build a transport chain matching production: GraphQLFeaturesTransport
		// wraps a header-capturing spy, which forwards to the mock's RoundTripper.
		// This verifies the handler sets the issue_fields context value and the
		// transport translates it into the outgoing header.
		mockClient := githubv4mock.NewMockedHTTPClient(matcher)
		spy := &headerCaptureTransport{inner: mockClient.Transport}
		httpClient := &http.Client{
			Transport: &transportpkg.GraphQLFeaturesTransport{Transport: spy},
		}
		gqlClient := githubv4.NewClient(httpClient)
		deps := BaseDeps{GQLClient: gqlClient}
		handler := serverTool.Handler(deps)

		req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
		res, err := handler(ContextWithDeps(context.Background(), deps), &req)
		require.NoError(t, err)
		require.False(t, res.IsError, getTextResult(t, res).Text)
		assert.Equal(t, "issue_fields, repo_issue_fields", spy.captured.Get(headers.GraphQLFeaturesHeader))
	})
}

// headerCaptureTransport records the headers of the most recent request that passed
// through it before forwarding to the inner RoundTripper.
type headerCaptureTransport struct {
	inner    http.RoundTripper
	captured http.Header
}

func (t *headerCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.captured = req.Header.Clone()
	return t.inner.RoundTrip(req)
}

func Test_ListIssues_IFC_InsidersMode(t *testing.T) {
	t.Parallel()

	serverTool := ListIssues(translations.NullTranslationHelper)

	mockIssues := []map[string]any{
		{
			"number":     1,
			"title":      "An issue",
			"body":       "body",
			"state":      "OPEN",
			"databaseId": 1,
			"createdAt":  "2023-01-01T00:00:00Z",
			"updatedAt":  "2023-01-01T00:00:00Z",
			"author":     map[string]any{"login": "user1"},
			"labels":     map[string]any{"nodes": []map[string]any{}},
			"comments":   map[string]any{"totalCount": 0},
		},
	}

	pageInfo := map[string]any{
		"hasNextPage":     false,
		"hasPreviousPage": false,
		"startCursor":     "",
		"endCursor":       "",
	}

	makeResponse := func(isPrivate bool) githubv4mock.GQLResponse {
		return githubv4mock.DataResponse(map[string]any{
			"repository": map[string]any{
				"issues": map[string]any{
					"nodes":      mockIssues,
					"pageInfo":   pageInfo,
					"totalCount": 1,
				},
				"isPrivate": isPrivate,
			},
		})
	}

	query := "query($after:String$direction:OrderDirection!$first:Int!$issueFieldValues:[IssueFieldValueFilter!]!$orderBy:IssueOrderField!$owner:String!$repo:String!$states:[IssueState!]!){repository(owner: $owner, name: $repo){issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues}){nodes{number,title,body,state,databaseId,author{login},createdAt,updatedAt,labels(first: 100){nodes{name,id,description}},comments{totalCount},issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}},pageInfo{hasNextPage,hasPreviousPage,startCursor,endCursor},totalCount},isPrivate}}"

	vars := map[string]any{
		"owner":            "octocat",
		"repo":             "hello",
		"states":           []any{"OPEN", "CLOSED"},
		"orderBy":          "CREATED_AT",
		"direction":        "DESC",
		"first":            float64(30),
		"after":            (*string)(nil),
		"issueFieldValues": []any{},
	}

	reqParams := map[string]any{"owner": "octocat", "repo": "hello"}

	t.Run("insiders mode disabled omits ifc label from result meta", func(t *testing.T) {
		matcher := githubv4mock.NewQueryMatcher(query, vars, makeResponse(false))
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher))
		deps := BaseDeps{
			GQLClient: gqlClient,
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Nil(t, result.Meta, "result meta should be nil when insiders mode is disabled")
	})

	t.Run("insiders mode enabled on public repo emits public untrusted label", func(t *testing.T) {
		matcher := githubv4mock.NewQueryMatcher(query, vars, makeResponse(false))
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher))
		deps := BaseDeps{
			GQLClient:      gqlClient,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcLabel, ok := result.Meta["ifc"]
		require.True(t, ok, "result meta should contain ifc key")

		ifcJSON, err := json.Marshal(ifcLabel)
		require.NoError(t, err)
		var ifcMap map[string]any
		require.NoError(t, json.Unmarshal(ifcJSON, &ifcMap))

		assert.Equal(t, "untrusted", ifcMap["integrity"])
		assert.Equal(t, "public", ifcMap["confidentiality"])
	})

	t.Run("insiders mode enabled on private repo emits private trusted label", func(t *testing.T) {
		matcher := githubv4mock.NewQueryMatcher(query, vars, makeResponse(true))
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(matcher))
		deps := BaseDeps{
			GQLClient:      gqlClient,
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := serverTool.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta)
		ifcLabel, ok := result.Meta["ifc"]
		require.True(t, ok, "result meta should contain ifc key")

		ifcJSON, err := json.Marshal(ifcLabel)
		require.NoError(t, err)
		var ifcMap map[string]any
		require.NoError(t, json.Unmarshal(ifcJSON, &ifcMap))

		assert.Equal(t, "trusted", ifcMap["integrity"])
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})
}

func Test_UpdateIssue(t *testing.T) {
	// Verify tool definition
	serverTool := IssueWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "title")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "body")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "labels")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "assignees")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "milestone")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "type")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "state")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "state_reason")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "duplicate_of")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_fields")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo"})

	// Mock issues for reuse across test cases
	mockBaseIssue := &github.Issue{
		Number:    github.Ptr(123),
		Title:     github.Ptr("Title"),
		Body:      github.Ptr("Description"),
		State:     github.Ptr("open"),
		HTMLURL:   github.Ptr("https://github.com/owner/repo/issues/123"),
		Assignees: []*github.User{{Login: github.Ptr("assignee1")}, {Login: github.Ptr("assignee2")}},
		Labels:    []*github.Label{{Name: github.Ptr("bug")}, {Name: github.Ptr("priority")}},
		Milestone: &github.Milestone{Number: github.Ptr(5)},
		Type:      &github.IssueType{Name: github.Ptr("Bug")},
	}

	mockUpdatedIssue := &github.Issue{
		Number:      github.Ptr(123),
		Title:       github.Ptr("Updated Title"),
		Body:        github.Ptr("Updated Description"),
		State:       github.Ptr("closed"),
		StateReason: github.Ptr("duplicate"),
		HTMLURL:     github.Ptr("https://github.com/owner/repo/issues/123"),
		Assignees:   []*github.User{{Login: github.Ptr("assignee1")}, {Login: github.Ptr("assignee2")}},
		Labels:      []*github.Label{{Name: github.Ptr("bug")}, {Name: github.Ptr("priority")}},
		Milestone:   &github.Milestone{Number: github.Ptr(5)},
		Type:        &github.IssueType{Name: github.Ptr("Bug")},
	}

	mockReopenedIssue := &github.Issue{
		Number:      github.Ptr(123),
		Title:       github.Ptr("Title"),
		State:       github.Ptr("open"),
		StateReason: github.Ptr("reopened"),
		HTMLURL:     github.Ptr("https://github.com/owner/repo/issues/123"),
	}

	// Mock GraphQL responses for reuse across test cases
	issueIDQueryResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"id": "I_kwDOA0xdyM50BPaO",
			},
		},
	})

	duplicateIssueIDQueryResponse := githubv4mock.DataResponse(map[string]any{
		"repository": map[string]any{
			"issue": map[string]any{
				"id": "I_kwDOA0xdyM50BPaO",
			},
			"duplicateIssue": map[string]any{
				"id": "I_kwDOA0xdyM50BPbP",
			},
		},
	})

	closeSuccessResponse := githubv4mock.DataResponse(map[string]any{
		"closeIssue": map[string]any{
			"issue": map[string]any{
				"id":     "I_kwDOA0xdyM50BPaO",
				"number": 123,
				"url":    "https://github.com/owner/repo/issues/123",
				"state":  "CLOSED",
			},
		},
	})

	reopenSuccessResponse := githubv4mock.DataResponse(map[string]any{
		"reopenIssue": map[string]any{
			"issue": map[string]any{
				"id":     "I_kwDOA0xdyM50BPaO",
				"number": 123,
				"url":    "https://github.com/owner/repo/issues/123",
				"state":  "OPEN",
			},
		},
	})

	duplicateStateReason := IssueClosedStateReasonDuplicate

	tests := []struct {
		name             string
		mockedRESTClient *http.Client
		mockedGQLClient  *http.Client
		requestArgs      map[string]any
		expectError      bool
		expectedIssue    *github.Issue
		expectedErrMsg   string
	}{
		{
			name: "partial update of non-state fields only",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
					"title": "Updated Title",
					"body":  "Updated Description",
				}).andThen(
					mockResponse(t, http.StatusOK, mockUpdatedIssue),
				),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"title":        "Updated Title",
				"body":         "Updated Description",
			},
			expectError:   false,
			expectedIssue: mockUpdatedIssue,
		},
		{
			name: "partial update clears labels and assignees",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
					"labels":    []any{},
					"assignees": []any{},
				}).andThen(
					mockResponse(t, http.StatusOK, &github.Issue{
						Number:  github.Ptr(123),
						HTMLURL: github.Ptr("https://github.com/owner/repo/issues/123"),
					}),
				),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"labels":       []any{},
				"assignees":    []any{},
			},
			expectError: false,
			expectedIssue: &github.Issue{
				HTMLURL: github.Ptr("https://github.com/owner/repo/issues/123"),
			},
		},
		{
			name: "partial update with issue fields reconciled by names",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
					"issue_field_values": []any{
						map[string]any{"field_id": float64(101), "value": "P1"},
						map[string]any{"field_id": float64(102), "value": "Acme"},
					},
					"title": "Updated Title",
				}).andThen(
					mockResponse(t, http.StatusOK, mockUpdatedIssue),
				),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				// fetch-and-merge: returns no existing fields so the incoming values are used as-is
				githubv4mock.NewQueryMatcher(
					"query($number:Int!$owner:String!$repo:String!){repository(owner: $owner, name: $repo){issue(number: $number){issueFieldValues(first: 25){nodes{__typename,... on IssueFieldDateValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldNumberValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},valueNumber: value},... on IssueFieldSingleSelectValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value},... on IssueFieldTextValue{field{... on IssueFieldDate{name,fullDatabaseId},... on IssueFieldNumber{name,fullDatabaseId},... on IssueFieldSingleSelect{name,fullDatabaseId},... on IssueFieldText{name,fullDatabaseId}},value}}}}}}",
					map[string]any{
						"owner":  githubv4.String("owner"),
						"repo":   githubv4.String("repo"),
						"number": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"issue": map[string]any{
								"issueFieldValues": map[string]any{
									"nodes": []any{},
								},
							},
						},
					}),
				),
				githubv4mock.NewQueryMatcher(
					issueFieldWriteMetadataQuery{},
					map[string]any{
						"owner": githubv4.String("owner"),
						"repo":  githubv4.String("repo"),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"issueFields": map[string]any{
								"nodes": []any{
									map[string]any{
										"__typename":     "IssueFieldSingleSelect",
										"fullDatabaseId": "101",
										"name":           "Priority",
										"dataType":       "single_select",
										"options":        []any{map[string]any{"fullDatabaseId": "9001", "name": "P1"}},
									},
									map[string]any{
										"__typename":     "IssueFieldText",
										"fullDatabaseId": "102",
										"name":           "Customer",
										"dataType":       "text",
									},
								},
							},
						},
					}),
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"title":        "Updated Title",
				"issue_fields": []any{
					map[string]any{"field_name": "Priority", "field_option_name": "P1"},
					map[string]any{"field_name": "Customer", "value": "Acme"},
				},
			},
			expectError:   false,
			expectedIssue: mockUpdatedIssue,
		},
		{
			name: "issue not found when updating non-state fields only",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"title":        "Updated Title",
			},
			expectError:    true,
			expectedErrMsg: "failed to update issue",
		},
		{
			name: "close issue as duplicate",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockBaseIssue),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								ID githubv4.ID
							} `graphql:"issue(number: $issueNumber)"`
							DuplicateIssue struct {
								ID githubv4.ID
							} `graphql:"duplicateIssue: issue(number: $duplicateOf)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(123),
						"duplicateOf": githubv4.Int(456),
					},
					duplicateIssueIDQueryResponse,
				),
				githubv4mock.NewMutationMatcher(
					struct {
						CloseIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
								State  githubv4.String
							}
						} `graphql:"closeIssue(input: $input)"`
					}{},
					CloseIssueInput{
						IssueID:          "I_kwDOA0xdyM50BPaO",
						StateReason:      &duplicateStateReason,
						DuplicateIssueID: githubv4.NewID("I_kwDOA0xdyM50BPbP"),
					},
					nil,
					closeSuccessResponse,
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"state":        "closed",
				"state_reason": "duplicate",
				"duplicate_of": float64(456),
			},
			expectError:   false,
			expectedIssue: mockUpdatedIssue,
		},
		{
			name: "reopen issue",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockBaseIssue),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								ID githubv4.ID
							} `graphql:"issue(number: $issueNumber)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(123),
					},
					issueIDQueryResponse,
				),
				githubv4mock.NewMutationMatcher(
					struct {
						ReopenIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
								State  githubv4.String
							}
						} `graphql:"reopenIssue(input: $input)"`
					}{},
					githubv4.ReopenIssueInput{
						IssueID: "I_kwDOA0xdyM50BPaO",
					},
					nil,
					reopenSuccessResponse,
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"state":        "open",
			},
			expectError:   false,
			expectedIssue: mockReopenedIssue,
		},
		{
			name: "main issue not found when trying to close it",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockBaseIssue),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								ID githubv4.ID
							} `graphql:"issue(number: $issueNumber)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(999),
					},
					githubv4mock.ErrorResponse("Could not resolve to an Issue with the number of 999."),
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"state":        "closed",
				"state_reason": "not_planned",
			},
			expectError:    true,
			expectedErrMsg: "Failed to find issues",
		},
		{
			name: "duplicate issue not found when closing as duplicate",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockBaseIssue),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								ID githubv4.ID
							} `graphql:"issue(number: $issueNumber)"`
							DuplicateIssue struct {
								ID githubv4.ID
							} `graphql:"duplicateIssue: issue(number: $duplicateOf)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(123),
						"duplicateOf": githubv4.Int(999),
					},
					githubv4mock.ErrorResponse("Could not resolve to an Issue with the number of 999."),
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"state":        "closed",
				"state_reason": "duplicate",
				"duplicate_of": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "Failed to find issues",
		},
		{
			name: "close as duplicate with combined non-state updates",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
					"title":     "Updated Title",
					"body":      "Updated Description",
					"labels":    []any{"bug", "priority"},
					"assignees": []any{"assignee1", "assignee2"},
					"milestone": float64(5),
					"type":      "Bug",
				}).andThen(
					mockResponse(t, http.StatusOK, &github.Issue{
						Number:    github.Ptr(123),
						Title:     github.Ptr("Updated Title"),
						Body:      github.Ptr("Updated Description"),
						Labels:    []*github.Label{{Name: github.Ptr("bug")}, {Name: github.Ptr("priority")}},
						Assignees: []*github.User{{Login: github.Ptr("assignee1")}, {Login: github.Ptr("assignee2")}},
						Milestone: &github.Milestone{Number: github.Ptr(5)},
						Type:      &github.IssueType{Name: github.Ptr("Bug")},
						State:     github.Ptr("open"), // Still open after REST update
						HTMLURL:   github.Ptr("https://github.com/owner/repo/issues/123"),
					}),
				),
			}),
			mockedGQLClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								ID githubv4.ID
							} `graphql:"issue(number: $issueNumber)"`
							DuplicateIssue struct {
								ID githubv4.ID
							} `graphql:"duplicateIssue: issue(number: $duplicateOf)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(123),
						"duplicateOf": githubv4.Int(456),
					},
					duplicateIssueIDQueryResponse,
				),
				githubv4mock.NewMutationMatcher(
					struct {
						CloseIssue struct {
							Issue struct {
								ID     githubv4.ID
								Number githubv4.Int
								URL    githubv4.String
								State  githubv4.String
							}
						} `graphql:"closeIssue(input: $input)"`
					}{},
					CloseIssueInput{
						IssueID:          "I_kwDOA0xdyM50BPaO",
						StateReason:      &duplicateStateReason,
						DuplicateIssueID: githubv4.NewID("I_kwDOA0xdyM50BPbP"),
					},
					nil,
					closeSuccessResponse,
				),
			),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"title":        "Updated Title",
				"body":         "Updated Description",
				"labels":       []any{"bug", "priority"},
				"assignees":    []any{"assignee1", "assignee2"},
				"milestone":    float64(5),
				"type":         "Bug",
				"state":        "closed",
				"state_reason": "duplicate",
				"duplicate_of": float64(456),
			},
			expectError:   false,
			expectedIssue: mockUpdatedIssue,
		},
		{
			name:             "duplicate_of without duplicate state_reason should fail",
			mockedRESTClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			mockedGQLClient:  githubv4mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"method":       "update",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
				"state":        "closed",
				"state_reason": "completed",
				"duplicate_of": float64(456),
			},
			expectError:    true,
			expectedErrMsg: "duplicate_of can only be used when state_reason is 'duplicate'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup clients with mocks
			restClient := mustNewGHClient(t, tc.mockedRESTClient)
			gqlClient := githubv4.NewClient(tc.mockedGQLClient)
			deps := BaseDeps{
				Client:    restClient,
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
			if result.IsError {
				t.Fatalf("Unexpected error result: %s", getErrorResult(t, result).Text)
			}

			require.False(t, result.IsError)

			// Parse the result and get the text content
			textContent := getTextResult(t, result)

			// Unmarshal and verify the minimal result
			var updateResp MinimalResponse
			err = json.Unmarshal([]byte(textContent.Text), &updateResp)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedIssue.GetHTMLURL(), updateResp.URL)
		})
	}
}

func Test_UpdateIssueClearsLabelsAndAssignees(t *testing.T) {
	serverTool := IssueWrite(translations.NullTranslationHelper)
	updatedIssue := &github.Issue{
		Number:  github.Ptr(8),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/8"),
	}

	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		PatchReposIssuesByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
			"labels":    []any{},
			"assignees": []any{},
		}).andThen(mockResponse(t, http.StatusOK, updatedIssue)),
	}))
	gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient())
	deps := BaseDeps{
		Client:    client,
		GQLClient: gqlClient,
	}
	handler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"method":       "update",
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(8),
		"labels":       []any{},
		"assignees":    []any{},
	})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	if result.IsError {
		t.Fatalf("Unexpected error result: %s", getErrorResult(t, result).Text)
	}
	textContent := getTextResult(t, result)

	var updateResp MinimalResponse
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &updateResp))
	assert.Equal(t, updatedIssue.GetHTMLURL(), updateResp.URL)
}

func Test_ParseISOTimestamp(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedErr  bool
		expectedTime time.Time
	}{
		{
			name:         "valid RFC3339 format",
			input:        "2023-01-15T14:30:00Z",
			expectedErr:  false,
			expectedTime: time.Date(2023, 1, 15, 14, 30, 0, 0, time.UTC),
		},
		{
			name:         "valid date only format",
			input:        "2023-01-15",
			expectedErr:  false,
			expectedTime: time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "empty timestamp",
			input:       "",
			expectedErr: true,
		},
		{
			name:        "invalid format",
			input:       "15/01/2023",
			expectedErr: true,
		},
		{
			name:        "invalid date",
			input:       "2023-13-45",
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parsedTime, err := parseISOTimestamp(tc.input)

			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTime, parsedTime)
			}
		})
	}
}

func Test_GetIssueComments(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "page")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number"})

	// Setup mock comments for success case
	mockComments := []*github.IssueComment{
		{
			ID:   github.Ptr(int64(123)),
			Body: github.Ptr("This is the first comment"),
			User: &github.User{
				Login: github.Ptr("user1"),
			},
			CreatedAt: &github.Timestamp{Time: time.Now().Add(-time.Hour * 24)},
		},
		{
			ID:   github.Ptr(int64(456)),
			Body: github.Ptr("This is the second comment"),
			User: &github.User{
				Login: github.Ptr("user2"),
			},
			CreatedAt: &github.Timestamp{Time: time.Now().Add(-time.Hour)},
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]any
		expectError      bool
		expectedComments []*github.IssueComment
		expectedErrMsg   string
		lockdownEnabled  bool
	}{
		{
			name: "successful comments retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockComments),
			}),
			requestArgs: map[string]any{
				"method":       "get_comments",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:      false,
			expectedComments: mockComments,
		},
		{
			name: "successful comments retrieval with pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: expectQueryParams(t, map[string]string{
					"page":     "2",
					"per_page": "10",
				}).andThen(
					mockResponse(t, http.StatusOK, mockComments),
				),
			}),
			requestArgs: map[string]any{
				"method":       "get_comments",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectError:      false,
			expectedComments: mockComments,
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_comments",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get issue comments",
		},
		{
			name: "lockdown enabled filters comments without push access",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesCommentsByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*github.IssueComment{
					{
						ID:   github.Ptr(int64(789)),
						Body: github.Ptr("Maintainer comment"),
						User: &github.User{Login: github.Ptr("maintainer")},
					},
					{
						ID:   github.Ptr(int64(790)),
						Body: github.Ptr("External user comment"),
						User: &github.User{Login: github.Ptr("testuser")},
					},
				}),
			}),
			requestArgs: map[string]any{
				"method":       "get_comments",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError: false,
			expectedComments: []*github.IssueComment{
				{
					ID:   github.Ptr(int64(789)),
					Body: github.Ptr("Maintainer comment"),
					User: &github.User{Login: github.Ptr("maintainer")},
				},
			},
			lockdownEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
			var restClient *github.Client
			if tc.lockdownEnabled {
				restClient = mockRESTPermissionServer(t, "read", map[string]string{
					"maintainer": "write",
					"testuser":   "read",
				})
			}
			cache := stubRepoAccessCache(restClient, 15*time.Minute)
			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": tc.lockdownEnabled})
			deps := BaseDeps{
				Client:          client,
				GQLClient:       defaultGQLClient,
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
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedComments []MinimalIssueComment
			err = json.Unmarshal([]byte(textContent.Text), &returnedComments)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedComments), len(returnedComments))
			for i := range tc.expectedComments {
				require.NotNil(t, tc.expectedComments[i].User)
				require.NotNil(t, returnedComments[i].User)
				assert.Equal(t, tc.expectedComments[i].GetID(), returnedComments[i].ID)
				assert.Equal(t, tc.expectedComments[i].GetBody(), returnedComments[i].Body)
				assert.Equal(t, tc.expectedComments[i].GetUser().GetLogin(), returnedComments[i].User.Login)
			}
		})
	}
}

func Test_GetIssueLabels(t *testing.T) {
	t.Parallel()

	// Verify tool definition
	serverTool := IssueRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number"})

	tests := []struct {
		name               string
		requestArgs        map[string]any
		mockedClient       *http.Client
		expectToolError    bool
		expectedToolErrMsg string
	}{
		{
			name: "successful issue labels listing",
			requestArgs: map[string]any{
				"method":       "get_labels",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(123),
			},
			mockedClient: githubv4mock.NewMockedHTTPClient(
				githubv4mock.NewQueryMatcher(
					struct {
						Repository struct {
							Issue struct {
								Labels struct {
									Nodes []struct {
										ID          githubv4.ID
										Name        githubv4.String
										Color       githubv4.String
										Description githubv4.String
									}
									TotalCount githubv4.Int
								} `graphql:"labels(first: 100)"`
							} `graphql:"issue(number: $issueNumber)"`
						} `graphql:"repository(owner: $owner, name: $repo)"`
					}{},
					map[string]any{
						"owner":       githubv4.String("owner"),
						"repo":        githubv4.String("repo"),
						"issueNumber": githubv4.Int(123),
					},
					githubv4mock.DataResponse(map[string]any{
						"repository": map[string]any{
							"issue": map[string]any{
								"labels": map[string]any{
									"nodes": []any{
										map[string]any{
											"id":          githubv4.ID("label-1"),
											"name":        githubv4.String("bug"),
											"color":       githubv4.String("d73a4a"),
											"description": githubv4.String("Something isn't working"),
										},
									},
									"totalCount": githubv4.Int(1),
								},
							},
						},
					}),
				),
			),
			expectToolError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClient := githubv4.NewClient(tc.mockedClient)
			client := mustNewGHClient(t, nil)
			deps := BaseDeps{
				Client:          client,
				GQLClient:       gqlClient,
				RepoAccessCache: stubRepoAccessCache(nil, 15*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			assert.NotNil(t, result)

			if tc.expectToolError {
				assert.True(t, result.IsError)
				if tc.expectedToolErrMsg != "" {
					textContent := getErrorResult(t, result)
					assert.Contains(t, textContent.Text, tc.expectedToolErrMsg)
				}
			} else {
				assert.False(t, result.IsError)
			}
		})
	}
}

func Test_AddSubIssue(t *testing.T) {
	// Verify tool definition once
	serverTool := SubIssueWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "sub_issue_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "sub_issue_id")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "replace_parent")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number", "sub_issue_id"})

	// Setup mock issue for success case (matches GitHub API response format)
	mockIssue := &github.Issue{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Parent Issue"),
		Body:    github.Ptr("This is the parent issue with a sub-issue"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Labels: []*github.Label{
			{
				Name:        github.Ptr("enhancement"),
				Color:       github.Ptr("84b6eb"),
				Description: github.Ptr("New feature or request"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedIssue  *github.Issue
		expectedErrMsg string
	}{
		{
			name: "successful sub-issue addition with all parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":         "add",
				"owner":          "owner",
				"repo":           "repo",
				"issue_number":   float64(42),
				"sub_issue_id":   float64(123),
				"replace_parent": true,
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "successful sub-issue addition with minimal parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(456),
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "successful sub-issue addition with replace_parent false",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":         "add",
				"owner":          "owner",
				"repo":           "repo",
				"issue_number":   float64(42),
				"sub_issue_id":   float64(789),
				"replace_parent": false,
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "parent issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Parent issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to add sub-issue",
		},
		{
			name: "sub-issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Sub-issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to add sub-issue",
		},
		{
			name: "validation failed - sub-issue cannot be parent of itself",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusUnprocessableEntity, `{"message": "Validation failed", "errors": [{"message": "Sub-issue cannot be a parent of itself"}]}`),
			}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "failed to add sub-issue",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusForbidden, `{"message": "Must have write access to repository"}`),
			}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to add sub-issue",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "add",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter sub_issue_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: sub_issue_id",
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
			var returnedIssue github.Issue
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedIssue.Number, *returnedIssue.Number)
			assert.Equal(t, *tc.expectedIssue.Title, *returnedIssue.Title)
			assert.Equal(t, *tc.expectedIssue.Body, *returnedIssue.Body)
			assert.Equal(t, *tc.expectedIssue.State, *returnedIssue.State)
			assert.Equal(t, *tc.expectedIssue.HTMLURL, *returnedIssue.HTMLURL)
			assert.Equal(t, *tc.expectedIssue.User.Login, *returnedIssue.User.Login)
		})
	}
}

func Test_GetSubIssues(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "page")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "perPage")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number"})

	// Setup mock sub-issues for success case
	mockSubIssues := []*github.Issue{
		{
			Number:  github.Ptr(123),
			Title:   github.Ptr("Sub-issue 1"),
			Body:    github.Ptr("This is the first sub-issue"),
			State:   github.Ptr("open"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/issues/123"),
			User: &github.User{
				Login: github.Ptr("user1"),
			},
			Labels: []*github.Label{
				{
					Name:        github.Ptr("bug"),
					Color:       github.Ptr("d73a4a"),
					Description: github.Ptr("Something isn't working"),
				},
			},
		},
		{
			Number:  github.Ptr(124),
			Title:   github.Ptr("Sub-issue 2"),
			Body:    github.Ptr("This is the second sub-issue"),
			State:   github.Ptr("closed"),
			HTMLURL: github.Ptr("https://github.com/owner/repo/issues/124"),
			User: &github.User{
				Login: github.Ptr("user2"),
			},
			Assignees: []*github.User{
				{Login: github.Ptr("assignee1")},
			},
		},
	}

	tests := []struct {
		name              string
		mockedClient      *http.Client
		requestArgs       map[string]any
		expectError       bool
		expectedSubIssues []*github.Issue
		expectedErrMsg    string
	}{
		{
			name: "successful sub-issues listing with minimal parameters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockSubIssues),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:       false,
			expectedSubIssues: mockSubIssues,
		},
		{
			name: "successful sub-issues listing with pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: expectQueryParams(t, map[string]string{
					"page":     "2",
					"per_page": "10",
				}).andThen(
					mockResponse(t, http.StatusOK, mockSubIssues),
				),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectError:       false,
			expectedSubIssues: mockSubIssues,
		},
		{
			name: "successful sub-issues listing with empty result",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []*github.Issue{}),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:       false,
			expectedSubIssues: []*github.Issue{},
		},
		{
			name: "parent issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to list sub-issues",
		},
		{
			name: "repository not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "nonexistent",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "failed to list sub-issues",
		},
		{
			name: "sub-issues feature gone/deprecated",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusGone, `{"message": "This feature has been deprecated"}`),
			}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "failed to list sub-issues",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "get_sub_issues",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter issue_number",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method": "get_sub_issues",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: issue_number",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
			gqlClient := githubv4.NewClient(nil)
			deps := BaseDeps{
				Client:          client,
				GQLClient:       gqlClient,
				RepoAccessCache: stubRepoAccessCache(nil, 15*time.Minute),
				Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
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
			var returnedSubIssues []*github.Issue
			err = json.Unmarshal([]byte(textContent.Text), &returnedSubIssues)
			require.NoError(t, err)

			assert.Len(t, returnedSubIssues, len(tc.expectedSubIssues))
			for i, subIssue := range returnedSubIssues {
				if i < len(tc.expectedSubIssues) {
					assert.Equal(t, *tc.expectedSubIssues[i].Number, *subIssue.Number)
					assert.Equal(t, *tc.expectedSubIssues[i].Title, *subIssue.Title)
					assert.Equal(t, *tc.expectedSubIssues[i].State, *subIssue.State)
					assert.Equal(t, *tc.expectedSubIssues[i].HTMLURL, *subIssue.HTMLURL)
					assert.Equal(t, *tc.expectedSubIssues[i].User.Login, *subIssue.User.Login)

					if tc.expectedSubIssues[i].Body != nil {
						assert.Equal(t, *tc.expectedSubIssues[i].Body, *subIssue.Body)
					}
				}
			}
		})
	}
}

func Test_RemoveSubIssue(t *testing.T) {
	// Verify tool definition once
	serverTool := SubIssueWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "sub_issue_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "sub_issue_id")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number", "sub_issue_id"})

	// Setup mock issue for success case (matches GitHub API response format - the updated parent issue)
	mockIssue := &github.Issue{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Parent Issue"),
		Body:    github.Ptr("This is the parent issue after sub-issue removal"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Labels: []*github.Label{
			{
				Name:        github.Ptr("enhancement"),
				Color:       github.Ptr("84b6eb"),
				Description: github.Ptr("New feature or request"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedIssue  *github.Issue
		expectedErrMsg string
	}{
		{
			name: "successful sub-issue removal",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "parent issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove sub-issue",
		},
		{
			name: "sub-issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Sub-issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove sub-issue",
		},
		{
			name: "bad request - invalid sub_issue_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusBadRequest, `{"message": "Invalid sub_issue_id"}`),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(-1),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove sub-issue",
		},
		{
			name: "repository not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "nonexistent",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove sub-issue",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusForbidden, `{"message": "Must have write access to repository"}`),
			}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove sub-issue",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "remove",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter sub_issue_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "remove",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: sub_issue_id",
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
			var returnedIssue github.Issue
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedIssue.Number, *returnedIssue.Number)
			assert.Equal(t, *tc.expectedIssue.Title, *returnedIssue.Title)
			assert.Equal(t, *tc.expectedIssue.Body, *returnedIssue.Body)
			assert.Equal(t, *tc.expectedIssue.State, *returnedIssue.State)
			assert.Equal(t, *tc.expectedIssue.HTMLURL, *returnedIssue.HTMLURL)
			assert.Equal(t, *tc.expectedIssue.User.Login, *returnedIssue.User.Login)
		})
	}
}

func Test_ReprioritizeSubIssue(t *testing.T) {
	// Verify tool definition once
	serverTool := SubIssueWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "sub_issue_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "sub_issue_id")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "after_id")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "before_id")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number", "sub_issue_id"})

	// Setup mock issue for success case (matches GitHub API response format - the updated parent issue)
	mockIssue := &github.Issue{
		Number:  github.Ptr(42),
		Title:   github.Ptr("Parent Issue"),
		Body:    github.Ptr("This is the parent issue with reprioritized sub-issues"),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/issues/42"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Labels: []*github.Label{
			{
				Name:        github.Ptr("enhancement"),
				Color:       github.Ptr("84b6eb"),
				Description: github.Ptr("New feature or request"),
			},
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedIssue  *github.Issue
		expectedErrMsg string
	}{
		{
			name: "successful reprioritization with after_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"after_id":     float64(456),
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name: "successful reprioritization with before_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"before_id":    float64(789),
			},
			expectError:   false,
			expectedIssue: mockIssue,
		},
		{
			name:         "validation error - neither after_id nor before_id specified",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "either after_id or before_id must be specified",
		},
		{
			name:         "validation error - both after_id and before_id specified",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"after_id":     float64(456),
				"before_id":    float64(789),
			},
			expectError:    false,
			expectedErrMsg: "only one of after_id or before_id should be specified, not both",
		},
		{
			name: "parent issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
				"sub_issue_id": float64(123),
				"after_id":     float64(456),
			},
			expectError:    false,
			expectedErrMsg: "failed to reprioritize sub-issue",
		},
		{
			name: "sub-issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Sub-issue not found"}`),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(999),
				"after_id":     float64(456),
			},
			expectError:    false,
			expectedErrMsg: "failed to reprioritize sub-issue",
		},
		{
			name: "validation failed - positioning sub-issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusUnprocessableEntity, `{"message": "Validation failed", "errors": [{"message": "Positioning sub-issue not found"}]}`),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"after_id":     float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to reprioritize sub-issue",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusForbidden, `{"message": "Must have write access to repository"}`),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"after_id":     float64(456),
			},
			expectError:    false,
			expectedErrMsg: "failed to reprioritize sub-issue",
		},
		{
			name: "service unavailable",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusServiceUnavailable, `{"message": "Service Unavailable"}`),
			}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"before_id":    float64(456),
			},
			expectError:    false,
			expectedErrMsg: "failed to reprioritize sub-issue",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"repo":         "repo",
				"issue_number": float64(42),
				"sub_issue_id": float64(123),
				"after_id":     float64(456),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter sub_issue_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"method":       "reprioritize",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"after_id":     float64(456),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: sub_issue_id",
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
			var returnedIssue github.Issue
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssue)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedIssue.Number, *returnedIssue.Number)
			assert.Equal(t, *tc.expectedIssue.Title, *returnedIssue.Title)
			assert.Equal(t, *tc.expectedIssue.Body, *returnedIssue.Body)
			assert.Equal(t, *tc.expectedIssue.State, *returnedIssue.State)
			assert.Equal(t, *tc.expectedIssue.HTMLURL, *returnedIssue.HTMLURL)
			assert.Equal(t, *tc.expectedIssue.User.Login, *returnedIssue.User.Login)
		})
	}
}

func Test_ListIssueTypes(t *testing.T) {
	// Verify tool definition once
	serverTool := ListIssueTypes(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_issue_types", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"owner"})

	// Setup mock issue types for success case
	mockIssueTypes := []*github.IssueType{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("bug"),
			Description: github.Ptr("Something isn't working"),
			Color:       github.Ptr("d73a4a"),
		},
		{
			ID:          github.Ptr(int64(2)),
			Name:        github.Ptr("feature"),
			Description: github.Ptr("New feature or enhancement"),
			Color:       github.Ptr("a2eeef"),
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]any
		expectError        bool
		expectedIssueTypes []*github.IssueType
		expectedErrMsg     string
	}{
		{
			name: "successful issue types retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-types": mockResponse(t, http.StatusOK, mockIssueTypes),
			}),
			requestArgs: map[string]any{
				"owner": "testorg",
			},
			expectError:        false,
			expectedIssueTypes: mockIssueTypes,
		},
		{
			name: "organization not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/nonexistent/issue-types": mockResponse(t, http.StatusNotFound, `{"message": "Organization not found"}`),
			}),
			requestArgs: map[string]any{
				"owner": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to list issue types",
		},
		{
			name: "missing owner parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-types": mockResponse(t, http.StatusOK, mockIssueTypes),
			}),
			requestArgs:    map[string]any{},
			expectError:    false, // This should be handled by parameter validation, error returned in result
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name: "successful repo issue types retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/testorg/testrepo/issue-types": mockResponse(t, http.StatusOK, mockIssueTypes),
			}),
			requestArgs: map[string]any{
				"owner": "testorg",
				"repo":  "testrepo",
			},
			expectError:        false,
			expectedIssueTypes: mockIssueTypes,
		},
		{
			name: "repo not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /repos/testorg/nonexistent/issue-types": mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"owner": "testorg",
				"repo":  "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to list issue types",
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
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}
				// Check if error is returned as tool result error
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			// Check if it's a parameter validation error (returned as tool result error)
			if result != nil && result.IsError {
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" && strings.Contains(errorContent.Text, tc.expectedErrMsg) {
					return // This is expected for parameter validation errors
				}
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedIssueTypes []*github.IssueType
			err = json.Unmarshal([]byte(textContent.Text), &returnedIssueTypes)
			require.NoError(t, err)

			if tc.expectedIssueTypes != nil {
				require.Equal(t, len(tc.expectedIssueTypes), len(returnedIssueTypes))
				for i, expected := range tc.expectedIssueTypes {
					assert.Equal(t, *expected.Name, *returnedIssueTypes[i].Name)
					assert.Equal(t, *expected.Description, *returnedIssueTypes[i].Description)
					assert.Equal(t, *expected.Color, *returnedIssueTypes[i].Color)
					assert.Equal(t, *expected.ID, *returnedIssueTypes[i].ID)
				}
			}
		})
	}
}

func Test_GetBlockedBy(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueDependencyRead(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_dependency_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number"})

	// Setup mock dependencies for success case
	mockDependencies := []IssueDependency{
		{
			ID:      123,
			Number:  10,
			Title:   "Blocking Issue 1",
			State:   "open",
			HTMLURL: "https://github.com/owner/repo/issues/10",
		},
		{
			ID:      456,
			Number:  20,
			Title:   "Blocking Issue 2",
			State:   "closed",
			HTMLURL: "https://github.com/owner/repo/issues/20",
		},
	}

	tests := []struct {
		name                 string
		mockedClient         *http.Client
		requestArgs          map[string]interface{}
		expectError          bool
		expectedDependencies []IssueDependency
		expectedErrMsg       string
	}{
		{
			name: "successful get blocked by",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockDependencies),
			}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:          false,
			expectedDependencies: mockDependencies,
		},
		{
			name: "successful get blocked by with pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: expectQueryParams(t, map[string]string{
					"page":     "2",
					"per_page": "10",
				}).andThen(mockResponse(t, http.StatusOK, mockDependencies)),
			}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectError:          false,
			expectedDependencies: mockDependencies,
		},
		{
			name: "empty dependencies list",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, []IssueDependency{}),
			}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:          false,
			expectedDependencies: []IssueDependency{},
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to get issue dependencies",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusForbidden, `{"message": "Must have read access to repository"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "failed to get issue dependencies",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method":       "get_blocked_by",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter issue_number",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method": "get_blocked_by",
				"owner":  "owner",
				"repo":   "repo",
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: issue_number",
		},
		{
			name:         "unknown method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method":       "invalid_method",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "unknown method: invalid_method",
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
			var returnedDependencies []IssueDependency
			err = json.Unmarshal([]byte(textContent.Text), &returnedDependencies)
			require.NoError(t, err)
			require.Equal(t, len(tc.expectedDependencies), len(returnedDependencies))
			for i, expected := range tc.expectedDependencies {
				assert.Equal(t, expected.ID, returnedDependencies[i].ID)
				assert.Equal(t, expected.Number, returnedDependencies[i].Number)
				assert.Equal(t, expected.Title, returnedDependencies[i].Title)
				assert.Equal(t, expected.State, returnedDependencies[i].State)
				assert.Equal(t, expected.HTMLURL, returnedDependencies[i].HTMLURL)
			}
		})
	}
}

func Test_AddBlockedBy(t *testing.T) {
	// Verify tool definition once
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "issue_dependency_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "method")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "owner")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "repo")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "issue_number")
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "blocking_issue_id")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"method", "owner", "repo", "issue_number", "blocking_issue_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful add blocked by",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusCreated, `{"message": "Dependency relationship created successfully"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError: false,
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(999),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to add issue dependency",
		},
		{
			name: "blocking issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusNotFound, `{"message": "Blocking issue not found"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to add issue dependency",
		},
		{
			name: "validation failed - self blocking",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusUnprocessableEntity, `{"message": "Validation failed", "errors": [{"message": "Issue cannot block itself"}]}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "failed to add issue dependency",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusForbidden, `{"message": "Must have write access to repository"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to add issue dependency",
		},
		{
			name:         "missing required parameter owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method":            "add",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required parameter blocking_issue_id",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method":       "add",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			},
			expectError:    false,
			expectedErrMsg: "missing required parameter: blocking_issue_id",
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

			if tc.expectedErrMsg != "" {
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and verify success message
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, "Dependency relationship created successfully")
		})
	}
}

func Test_RemoveBlockedBy(t *testing.T) {
	// Verify tool definition once (using same tool as AddBlockedBy)
	serverTool := IssueDependencyWrite(translations.NullTranslationHelper)
	tool := serverTool.Tool

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful remove blocked by",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumberByIssueID: mockResponse(t, http.StatusNoContent, ""),
			}),
			requestArgs: map[string]interface{}{
				"method":            "remove",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError: false,
		},
		{
			name: "dependency not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumberByIssueID: mockResponse(t, http.StatusNotFound, `{"message": "Dependency not found"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "remove",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(999),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove issue dependency",
		},
		{
			name: "issue not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumberByIssueID: mockResponse(t, http.StatusNotFound, `{"message": "Issue not found"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "remove",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(999),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove issue dependency",
		},
		{
			name: "insufficient permissions",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				DeleteReposIssuesDependenciesBlockedByByOwnerByRepoByIssueNumberByIssueID: mockResponse(t, http.StatusForbidden, `{"message": "Must have write access to repository"}`),
			}),
			requestArgs: map[string]interface{}{
				"method":            "remove",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "failed to remove issue dependency",
		},
		{
			name:         "unknown method",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"method":            "invalid_method",
				"owner":             "owner",
				"repo":              "repo",
				"issue_number":      float64(42),
				"blocking_issue_id": float64(123),
			},
			expectError:    false,
			expectedErrMsg: "unknown method: invalid_method",
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

			if tc.expectedErrMsg != "" {
				require.NotNil(t, result)
				textContent := getTextResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and verify success message
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, "Dependency relationship removed successfully")
		})
	}

	// Verify that we're testing the same tool for both add and remove
	assert.Equal(t, "issue_dependency_write", tool.Name)
}
