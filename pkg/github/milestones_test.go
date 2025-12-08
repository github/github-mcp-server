package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMilestoneTestClient(t *testing.T, handler http.Handler) *github.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := github.NewClient(server.Client())
	baseURL, err := url.Parse(server.URL + "/")
	require.NoError(t, err)
	client.BaseURL = baseURL

	return client
}

func TestListMilestones_ToolDefinition(t *testing.T) {
	t.Parallel()

	mockClient := github.NewClient(nil)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	tool, _ := ListMilestones(stubGetClientFn(mockClient), cache, translations.NullTranslationHelper, stubFeatureFlags(map[string]bool{"lockdown-mode": false}))
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_milestones", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "direction")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})
}

func TestListMilestones_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectError  bool
		errContains  string
	}{
		{
			name: "success with filters and pagination",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
					expectQueryParams(t, map[string]string{
						"state":     "all",
						"sort":      "due_on",
						"direction": "desc",
						"page":      "2",
						"per_page":  "50",
					}).andThen(
						mockResponse(t, http.StatusOK, []map[string]any{
							{
								"id":            1,
								"number":        10,
								"title":         "v1",
								"state":         "open",
								"description":   "first",
								"due_on":        "2024-01-02T00:00:00Z",
								"open_issues":   3,
								"closed_issues": 1,
								"html_url":      "https://example.com/1",
							},
							{
								"id":            2,
								"number":        11,
								"title":         "v2",
								"state":         "closed",
								"description":   "second",
								"open_issues":   0,
								"closed_issues": 4,
								"html_url":      "https://example.com/2",
							},
						}),
					),
				),
			),
			args: map[string]any{
				"owner":     "owner",
				"repo":      "repo",
				"state":     "all",
				"sort":      "due_on",
				"direction": "desc",
				"per_page":  float64(50),
				"page":      float64(2),
			},
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			args: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError: true,
			errContains: "failed to list milestones",
		},
		{
			name: "validation error",
			args: map[string]any{
				"owner": "o",
				"repo":  "r",
				"state": "invalid",
			},
			expectError: true,
			errContains: "state must be",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
			_, handler := ListMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

			request := createMCPRequest(tc.args)
			result, _, err := handler(context.Background(), &request, tc.args)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getErrorResult(t, result)
				assert.Contains(t, text.Text, tc.errContains)
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result)
			var resp map[string]any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
			assert.Equal(t, float64(2), resp["count"])
		})
	}
}

func TestListMilestones_LockdownFiltersUnsafeCreators(t *testing.T) {
	t.Parallel()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
			mockResponse(t, http.StatusOK, []map[string]any{
				{
					"id":          1,
					"number":      10,
					"title":       "unsafe",
					"description": "from reader",
					"creator": map[string]any{
						"login": "testuser",
					},
					"html_url": "https://example.com/1",
				},
				{
					"id":          2,
					"number":      11,
					"title":       "safe",
					"description": "from writer",
					"creator": map[string]any{
						"login": "testuser2",
					},
					"html_url": "https://example.com/2",
				},
			}),
		),
	)

	client := github.NewClient(mockedClient)
	gqlClient := githubv4.NewClient(newRepoAccessHTTPClient())
	cache := stubRepoAccessCache(gqlClient, 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": true})
	_, handler := ListMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner": "owner",
		"repo":  "repo",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))

	milestones, ok := resp["milestones"].([]any)
	require.True(t, ok)
	assert.Len(t, milestones, 1)

	first, ok := milestones[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "safe", first["title"])
}

func TestListMilestones_ValidationError(t *testing.T) {
	t.Parallel()

	client := github.NewClient(nil)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	_, handler := ListMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner": "o",
		"repo":  "r",
		"state": "invalid",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)
	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "state must be")
}

func TestSearchMilestones_ToolDefinition(t *testing.T) {
	t.Parallel()

	mockClient := github.NewClient(nil)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	tool, _ := SearchMilestones(stubGetClientFn(mockClient), cache, translations.NullTranslationHelper, stubFeatureFlags(map[string]bool{"lockdown-mode": false}))
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "search_milestones", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "state")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "query"})
}

func TestSearchMilestones_Success(t *testing.T) {
	t.Parallel()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
			expectQueryParams(t, map[string]string{
				"state":    "open",
				"page":     "1",
				"per_page": "25",
			}).andThen(
				mockResponse(t, http.StatusOK, []map[string]any{
					{
						"id":            1,
						"number":        10,
						"title":         "Alpha release",
						"state":         "open",
						"description":   "first milestone",
						"html_url":      "https://example.com/1",
						"open_issues":   3,
						"closed_issues": 1,
					},
					{
						"id":            2,
						"number":        11,
						"title":         "Beta",
						"state":         "open",
						"description":   "stability work",
						"html_url":      "https://example.com/2",
						"open_issues":   0,
						"closed_issues": 4,
					},
				}),
			),
		),
	)

	client := github.NewClient(mockedClient)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	_, handler := SearchMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner":    "owner",
		"repo":     "repo",
		"query":    "alpha",
		"per_page": float64(25),
		"page":     float64(1),
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))

	assert.Equal(t, float64(1), resp["count"])
	milestones, ok := resp["milestones"].([]any)
	require.True(t, ok)
	require.Len(t, milestones, 1)
	first, ok := milestones[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Alpha release", first["title"])
}

func TestSearchMilestones_Validation(t *testing.T) {
	t.Parallel()

	client := github.NewClient(nil)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	_, handler := SearchMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner": "o",
		"repo":  "r",
		"query": "q",
		"state": "invalid",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)
	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "state must be")
}

func TestSearchMilestones_Lockdown(t *testing.T) {
	t.Parallel()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
			mockResponse(t, http.StatusOK, []map[string]any{
				{
					"id":          1,
					"number":      10,
					"title":       "Unsafe alpha",
					"description": "match me",
					"creator": map[string]any{
						"login": "testuser",
					},
					"html_url": "https://example.com/1",
				},
				{
					"id":          2,
					"number":      11,
					"title":       "Safe alpha",
					"description": "match me too",
					"creator": map[string]any{
						"login": "testuser2",
					},
					"html_url": "https://example.com/2",
				},
			}),
		),
	)

	client := github.NewClient(mockedClient)
	gqlClient := githubv4.NewClient(newRepoAccessHTTPClient())
	cache := stubRepoAccessCache(gqlClient, 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": true})
	_, handler := SearchMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner": "owner",
		"repo":  "repo",
		"query": "alpha",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))

	milestones, ok := resp["milestones"].([]any)
	require.True(t, ok)
	assert.Len(t, milestones, 1)

	first, ok := milestones[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Safe alpha", first["title"])
}

func TestSearchMilestones_ApiError(t *testing.T) {
	t.Parallel()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones", Method: http.MethodGet},
			mockResponse(t, http.StatusInternalServerError, map[string]any{"message": "boom"}),
		),
	)

	client := github.NewClient(mockedClient)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	_, handler := SearchMilestones(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner": "owner",
		"repo":  "repo",
		"query": "alpha",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)

	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "failed to search milestones")
}

func TestGetMilestone_ToolDefinition(t *testing.T) {
	t.Parallel()

	mockClient := github.NewClient(nil)
	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	tool, _ := GetMilestone(stubGetClientFn(mockClient), cache, translations.NullTranslationHelper, stubFeatureFlags(map[string]bool{"lockdown-mode": false}))
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_milestone", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.Contains(t, schema.Properties, "milestone_number")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "milestone_number"})
}

func TestGetMilestone_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectError  bool
		errContains  string
	}{
		{
			name: "success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones/{milestone_number}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, map[string]any{
						"id":            55,
						"number":        5,
						"title":         "v1",
						"state":         "open",
						"description":   "first",
						"due_on":        "2024-01-02T00:00:00Z",
						"open_issues":   3,
						"closed_issues": 1,
						"html_url":      "https://example.com/1",
					}),
				),
			),
			args: map[string]any{
				"owner":            "owner",
				"repo":             "repo",
				"milestone_number": float64(5),
			},
		},
		{
			name: "not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones/{milestone_number}", Method: http.MethodGet},
					mockResponse(t, http.StatusNotFound, map[string]string{"message": "not found"}),
				),
			),
			args: map[string]any{
				"owner":            "owner",
				"repo":             "repo",
				"milestone_number": float64(99),
			},
			expectError: true,
			errContains: "failed to get milestone",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
			_, handler := GetMilestone(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

			request := createMCPRequest(tc.args)
			result, _, err := handler(context.Background(), &request, tc.args)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getErrorResult(t, result)
				assert.Contains(t, text.Text, tc.errContains)
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result)
			var resp map[string]any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
			assert.Equal(t, float64(5), resp["number"])
			assert.Equal(t, "v1", resp["title"])
		})
	}
}

func TestGetMilestone_LockdownEnforced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mockedClient *http.Client
		args         map[string]any
		expectError  bool
		errContains  string
	}{
		{
			name: "blocked when creator lacks push access",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones/{milestone_number}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, map[string]any{
						"id":          55,
						"number":      5,
						"title":       "v1",
						"state":       "open",
						"description": "first",
						"html_url":    "https://example.com/1",
						"creator": map[string]any{
							"login": "testuser",
						},
					}),
				),
			),
			args: map[string]any{
				"owner":            "owner",
				"repo":             "repo",
				"milestone_number": float64(5),
			},
			expectError: true,
			errContains: "access to milestone is restricted by lockdown mode",
		},
		{
			name: "allowed for private repo creator",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/repos/{owner}/{repo}/milestones/{milestone_number}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, map[string]any{
						"id":          56,
						"number":      6,
						"title":       "v2",
						"state":       "open",
						"description": "second",
						"html_url":    "https://example.com/2",
						"creator": map[string]any{
							"login": "testuser2",
						},
					}),
				),
			),
			args: map[string]any{
				"owner":            "owner2",
				"repo":             "repo2",
				"milestone_number": float64(6),
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			gqlClient := githubv4.NewClient(newRepoAccessHTTPClient())
			cache := stubRepoAccessCache(gqlClient, 15*time.Minute)
			flags := stubFeatureFlags(map[string]bool{"lockdown-mode": true})
			_, handler := GetMilestone(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

			request := createMCPRequest(tc.args)
			result, _, err := handler(context.Background(), &request, tc.args)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectError {
				require.True(t, result.IsError)
				errText := getErrorResult(t, result)
				assert.Contains(t, errText.Text, tc.errContains)
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result)
			var resp map[string]any
			require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
			assert.Equal(t, tc.args["milestone_number"], resp["number"])
		})
	}
}

func TestGetMilestone_NotFound(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "Not Found",
		})
	}))

	cache := stubRepoAccessCache(githubv4.NewClient(nil), 15*time.Minute)
	flags := stubFeatureFlags(map[string]bool{"lockdown-mode": false})
	_, handler := GetMilestone(stubGetClientFn(client), cache, translations.NullTranslationHelper, flags)

	args := map[string]any{
		"owner":            "owner",
		"repo":             "repo",
		"milestone_number": float64(1),
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)
	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "failed to get milestone")
}

func TestMilestoneWrite_ToolDefinition(t *testing.T) {
	t.Parallel()

	mockClient := github.NewClient(nil)
	tool, _ := MilestoneWrite(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "milestone_write", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")

	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "method")
	assert.Contains(t, schema.Properties, "title")
	assert.Contains(t, schema.Properties, "due_on")
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})
}

func TestMilestoneWrite_CreateSuccess(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/repos/owner/repo/milestones", r.URL.Path)

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         101,
			"number":     5,
			"html_url":   "https://example.com/milestones/5",
			"created_at": time.Now(),
		})
	}))

	tool, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)
	require.Equal(t, "milestone_write", tool.Name)

	args := map[string]any{
		"method":      "create",
		"owner":       "owner",
		"repo":        "repo",
		"title":       "v1.0",
		"description": "Initial release",
		"state":       "open",
		"due_on":      "2024-01-02",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	assert.Equal(t, "101", resp["id"])
	assert.Equal(t, float64(5), resp["number"])
	assert.Equal(t, "https://example.com/milestones/5", resp["url"])
}

func TestMilestoneWrite_UpdateSuccess(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/repos/owner/repo/milestones/7", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":       202,
			"number":   7,
			"html_url": "https://example.com/milestones/7",
		})
	}))

	_, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)

	args := map[string]any{
		"method":           "update",
		"owner":            "owner",
		"repo":             "repo",
		"milestone_number": float64(7),
		"title":            "v1.1",
		"state":            "closed",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	assert.Equal(t, "202", resp["id"])
	assert.Equal(t, float64(7), resp["number"])
	assert.Equal(t, "https://example.com/milestones/7", resp["url"])
}

func TestMilestoneWrite_DeleteSuccess(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/repos/owner/repo/milestones/3", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))

	_, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)

	args := map[string]any{
		"method":           "delete",
		"owner":            "owner",
		"repo":             "repo",
		"milestone_number": float64(3),
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	assert.Contains(t, text.Text, "milestone 3 deleted")
}

func TestMilestoneWrite_DeleteApiError(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/repos/owner/repo/milestones/3", r.URL.Path)

		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": "delete failed",
		})
	}))

	_, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)

	args := map[string]any{
		"method":           "delete",
		"owner":            "owner",
		"repo":             "repo",
		"milestone_number": float64(3),
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)

	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "failed to delete milestone")
}

func TestMilestoneWrite_ValidationErrors(t *testing.T) {
	t.Parallel()

	client := github.NewClient(nil)
	_, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)

	tests := []struct {
		name      string
		args      map[string]any
		errSubstr string
	}{
		{
			name: "missing title on create",
			args: map[string]any{
				"method": "create",
				"owner":  "o",
				"repo":   "r",
			},
			errSubstr: "missing required parameter: title",
		},
		{
			name: "invalid state",
			args: map[string]any{
				"method": "create",
				"owner":  "o",
				"repo":   "r",
				"title":  "x",
				"state":  "pending",
			},
			errSubstr: "state must be 'open' or 'closed'",
		},
		{
			name: "invalid due_on",
			args: map[string]any{
				"method": "create",
				"owner":  "o",
				"repo":   "r",
				"title":  "x",
				"due_on": "not-a-date",
			},
			errSubstr: "invalid due_on format",
		},
		{
			name: "update without fields",
			args: map[string]any{
				"method":           "update",
				"owner":            "o",
				"repo":             "r",
				"milestone_number": float64(1),
			},
			errSubstr: "at least one of title",
		},
		{
			name: "invalid method",
			args: map[string]any{
				"method": "noop",
				"owner":  "o",
				"repo":   "r",
			},
			errSubstr: "invalid method",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			request := createMCPRequest(tc.args)
			result, _, err := handler(context.Background(), &request, tc.args)

			require.NoError(t, err)
			require.True(t, result.IsError)
			text := getErrorResult(t, result)
			assert.Contains(t, text.Text, tc.errSubstr)
		})
	}
}

func TestMilestoneWrite_ApiError(t *testing.T) {
	t.Parallel()

	client := newMilestoneTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(github.ErrorResponse{
			Message: "bad request",
		})
	}))

	_, handler := MilestoneWrite(stubGetClientFn(client), translations.NullTranslationHelper)

	args := map[string]any{
		"method": "create",
		"owner":  "o",
		"repo":   "r",
		"title":  "t",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)

	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "failed to create milestone")
}

func TestParseDueOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		expectNil bool
	}{
		{"empty", "", true},
		{"date only", "2024-01-02", false},
		{"rfc3339", "2024-01-02T15:04:05Z", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ts, err := parseDueOn(tc.value)
			require.NoError(t, err)
			if tc.expectNil {
				assert.Nil(t, ts)
				return
			}
			require.NotNil(t, ts)
		})
	}

	_, err := parseDueOn("bad-date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid due_on format")
}

func TestMarshalMilestoneResponse(t *testing.T) {
	t.Parallel()

	milestone := &github.Milestone{
		ID:      github.Ptr(int64(5)),
		Number:  github.Ptr(3),
		HTMLURL: github.Ptr("https://example.com/m/3"),
	}

	result, _, err := marshalMilestoneResponse(milestone)
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &out))
	assert.Equal(t, "5", out["id"])
	assert.Equal(t, float64(3), out["number"])
	assert.Equal(t, "https://example.com/m/3", out["url"])
}

func TestMilestoneWrite_ClientError(t *testing.T) {
	t.Parallel()

	getClientErr := func(_ context.Context) (*github.Client, error) {
		return nil, fmt.Errorf("boom")
	}

	_, handler := MilestoneWrite(getClientErr, translations.NullTranslationHelper)

	args := map[string]any{
		"method": "create",
		"owner":  "o",
		"repo":   "r",
		"title":  "t",
	}

	request := createMCPRequest(args)
	result, _, err := handler(context.Background(), &request, args)

	require.NoError(t, err)
	require.True(t, result.IsError)

	text := getErrorResult(t, result)
	assert.Contains(t, text.Text, "failed to get GitHub client")
}
