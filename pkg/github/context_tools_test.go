package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetMe(t *testing.T) {
	t.Parallel()

	serverTool := GetMe(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify some basic very important properties
	assert.Equal(t, "get_me", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_me tool should be read-only")

	// Setup mock user response
	mockUser := &github.User{
		Login:           github.Ptr("testuser"),
		Name:            github.Ptr("Test User"),
		Email:           github.Ptr("test@example.com"),
		Bio:             github.Ptr("GitHub user for testing"),
		Company:         github.Ptr("Test Company"),
		Location:        github.Ptr("Test Location"),
		HTMLURL:         github.Ptr("https://github.com/testuser"),
		CreatedAt:       &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		Type:            github.Ptr("User"),
		Hireable:        github.Ptr(true),
		TwitterUsername: github.Ptr("testuser_twitter"),
		Plan: &github.Plan{
			Name: github.Ptr("pro"),
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		clientErr          string // if set, GetClient returns this error
		requestArgs        map[string]any
		expectToolError    bool
		expectedUser       *github.User
		expectedToolErrMsg string
	}{
		{
			name: "successful get user",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetUser: mockResponse(t, http.StatusOK, mockUser),
			}),
			requestArgs:     map[string]any{},
			expectToolError: false,
			expectedUser:    mockUser,
		},
		{
			name: "successful get user with reason",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetUser: mockResponse(t, http.StatusOK, mockUser),
			}),
			requestArgs: map[string]any{
				"reason": "Testing API",
			},
			expectToolError: false,
			expectedUser:    mockUser,
		},
		{
			name:               "getting client fails",
			clientErr:          "expected test error",
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "get user fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetUser: badRequestHandler("expected test failure"),
			}),
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var deps ToolDependencies
			if tc.clientErr != "" {
				deps = stubDeps{clientFn: stubClientFnErr(tc.clientErr)}
			} else {
				deps = BaseDeps{Client: github.NewClient(tc.mockedClient)}
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectToolError {
				require.True(t, result.IsError, "expected tool call result to be an error")
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedUser MinimalUser
			err = json.Unmarshal([]byte(textContent.Text), &returnedUser)
			require.NoError(t, err)

			// Verify minimal user details
			assert.Equal(t, *tc.expectedUser.Login, returnedUser.Login)
			assert.Equal(t, *tc.expectedUser.HTMLURL, returnedUser.ProfileURL)

			// Verify user details
			require.NotNil(t, returnedUser.Details)
			assert.Equal(t, *tc.expectedUser.Name, returnedUser.Details.Name)
			assert.Equal(t, *tc.expectedUser.Email, returnedUser.Details.Email)
			assert.Equal(t, *tc.expectedUser.Bio, returnedUser.Details.Bio)
			assert.Equal(t, *tc.expectedUser.Company, returnedUser.Details.Company)
			assert.Equal(t, *tc.expectedUser.Location, returnedUser.Details.Location)
			assert.Equal(t, *tc.expectedUser.Hireable, returnedUser.Details.Hireable)
			assert.Equal(t, *tc.expectedUser.TwitterUsername, returnedUser.Details.TwitterUsername)
		})
	}
}

func Test_GetTeams(t *testing.T) {
	t.Parallel()

	serverTool := GetTeams(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_teams", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_teams tool should be read-only")

	mockUser := &github.User{
		Login:           github.Ptr("testuser"),
		Name:            github.Ptr("Test User"),
		Email:           github.Ptr("test@example.com"),
		Bio:             github.Ptr("GitHub user for testing"),
		Company:         github.Ptr("Test Company"),
		Location:        github.Ptr("Test Location"),
		HTMLURL:         github.Ptr("https://github.com/testuser"),
		CreatedAt:       &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		Type:            github.Ptr("User"),
		Hireable:        github.Ptr(true),
		TwitterUsername: github.Ptr("testuser_twitter"),
		Plan: &github.Plan{
			Name: github.Ptr("pro"),
		},
	}

	mockTeamsResponse := githubv4mock.DataResponse(map[string]any{
		"user": map[string]any{
			"organizations": map[string]any{
				"nodes": []map[string]any{
					{
						"login": "testorg1",
						"teams": map[string]any{
							"nodes": []map[string]any{
								{
									"name":        "team1",
									"slug":        "team1",
									"description": "Team 1",
								},
								{
									"name":        "team2",
									"slug":        "team2",
									"description": "Team 2",
								},
							},
						},
					},
					{
						"login": "testorg2",
						"teams": map[string]any{
							"nodes": []map[string]any{
								{
									"name":        "team3",
									"slug":        "team3",
									"description": "Team 3",
								},
							},
						},
					},
				},
			},
		},
	})

	mockNoTeamsResponse := githubv4mock.DataResponse(map[string]any{
		"user": map[string]any{
			"organizations": map[string]any{
				"nodes": []map[string]any{},
			},
		},
	})

	// Create GQL clients for different test scenarios - these are factory functions
	// to ensure each test gets a fresh client
	gqlClientForTestuser := func() *githubv4.Client {
		queryStr := "query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}"
		vars := map[string]interface{}{
			"login": "testuser",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamsResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientForSpecificuser := func() *githubv4.Client {
		queryStr := "query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}"
		vars := map[string]interface{}{
			"login": "specificuser",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamsResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientNoTeams := func() *githubv4.Client {
		queryStr := "query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}"
		vars := map[string]interface{}{
			"login": "testuser",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockNoTeamsResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	// Factory function for mock HTTP clients with user response
	httpClientWithUser := func() *http.Client {
		return MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUser: mockResponse(t, http.StatusOK, mockUser),
		})
	}

	httpClientUserFails := func() *http.Client {
		return MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUser: badRequestHandler("expected test failure"),
		})
	}

	tests := []struct {
		name               string
		makeDeps           func() ToolDependencies
		requestArgs        map[string]any
		expectToolError    bool
		expectedToolErrMsg string
		expectedTeamsCount int
	}{
		{
			name: "successful get teams",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					Client:    github.NewClient(httpClientWithUser()),
					GQLClient: gqlClientForTestuser(),
				}
			},
			requestArgs:        map[string]any{},
			expectToolError:    false,
			expectedTeamsCount: 2,
		},
		{
			name: "successful get teams for specific user",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					GQLClient: gqlClientForSpecificuser(),
				}
			},
			requestArgs: map[string]any{
				"user": "specificuser",
			},
			expectToolError:    false,
			expectedTeamsCount: 2,
		},
		{
			name: "no teams found",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					Client:    github.NewClient(httpClientWithUser()),
					GQLClient: gqlClientNoTeams(),
				}
			},
			requestArgs:        map[string]any{},
			expectToolError:    false,
			expectedTeamsCount: 0,
		},
		{
			name: "getting client fails",
			makeDeps: func() ToolDependencies {
				return stubDeps{clientFn: stubClientFnErr("expected test error")}
			},
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "get user fails",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					Client: github.NewClient(httpClientUserFails()),
				}
			},
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "expected test failure",
		},
		{
			name: "getting GraphQL client fails",
			makeDeps: func() ToolDependencies {
				return stubDeps{
					clientFn:    stubClientFnFromHTTP(httpClientWithUser()),
					gqlClientFn: stubGQLClientFnErr("GraphQL client error"),
				}
			},
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub GQL client: GraphQL client error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := tc.makeDeps()
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectToolError {
				require.True(t, result.IsError, "expected tool call result to be an error")
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var organizations []OrganizationTeams
			err = json.Unmarshal([]byte(textContent.Text), &organizations)
			require.NoError(t, err)

			assert.Len(t, organizations, tc.expectedTeamsCount)

			if tc.expectedTeamsCount > 0 {
				assert.Equal(t, "testorg1", organizations[0].Org)
				assert.Len(t, organizations[0].Teams, 2)
				assert.Equal(t, "team1", organizations[0].Teams[0].Name)
				assert.Equal(t, "team1", organizations[0].Teams[0].Slug)
				assert.Equal(t, "Team 1", organizations[0].Teams[0].Description)

				if tc.expectedTeamsCount > 1 {
					assert.Equal(t, "testorg2", organizations[1].Org)
					assert.Len(t, organizations[1].Teams, 1)
					assert.Equal(t, "team3", organizations[1].Teams[0].Name)
					assert.Equal(t, "team3", organizations[1].Teams[0].Slug)
					assert.Equal(t, "Team 3", organizations[1].Teams[0].Description)
				}
			}
		})
	}
}

func Test_GetTeamMembers(t *testing.T) {
	t.Parallel()

	serverTool := GetTeamMembers(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_team_members", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_team_members tool should be read-only")

	mockTeamMembersResponse := githubv4mock.DataResponse(map[string]any{
		"organization": map[string]any{
			"team": map[string]any{
				"members": map[string]any{
					"nodes": []map[string]any{
						{
							"login": "user1",
						},
						{
							"login": "user2",
						},
					},
				},
			},
		},
	})

	mockNoMembersResponse := githubv4mock.DataResponse(map[string]any{
		"organization": map[string]any{
			"team": map[string]any{
				"members": map[string]any{
					"nodes": []map[string]any{},
				},
			},
		},
	})

	// Create GQL clients for different test scenarios
	gqlClientWithMembers := func() *githubv4.Client {
		queryStr := "query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}"
		vars := map[string]interface{}{
			"org":      "testorg",
			"teamSlug": "testteam",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamMembersResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientNoMembers := func() *githubv4.Client {
		queryStr := "query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}"
		vars := map[string]interface{}{
			"org":      "testorg",
			"teamSlug": "emptyteam",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockNoMembersResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	tests := []struct {
		name                 string
		deps                 ToolDependencies
		requestArgs          map[string]any
		expectToolError      bool
		expectedToolErrMsg   string
		expectedMembersCount int
	}{
		{
			name: "successful get team members",
			deps: BaseDeps{GQLClient: gqlClientWithMembers()},
			requestArgs: map[string]any{
				"org":       "testorg",
				"team_slug": "testteam",
			},
			expectToolError:      false,
			expectedMembersCount: 2,
		},
		{
			name: "team with no members",
			deps: BaseDeps{GQLClient: gqlClientNoMembers()},
			requestArgs: map[string]any{
				"org":       "testorg",
				"team_slug": "emptyteam",
			},
			expectToolError:      false,
			expectedMembersCount: 0,
		},
		{
			name: "getting GraphQL client fails",
			deps: stubDeps{gqlClientFn: stubGQLClientFnErr("GraphQL client error")},
			requestArgs: map[string]any{
				"org":       "testorg",
				"team_slug": "testteam",
			},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub GQL client: GraphQL client error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := serverTool.Handler(tc.deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), tc.deps), &request)
			require.NoError(t, err)

			if tc.expectToolError {
				require.True(t, result.IsError, "expected tool call result to be an error")
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedToolErrMsg)
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var members []string
			err = json.Unmarshal([]byte(textContent.Text), &members)
			require.NoError(t, err)

			assert.Len(t, members, tc.expectedMembersCount)

			if tc.expectedMembersCount > 0 {
				assert.Equal(t, "user1", members[0])

				if tc.expectedMembersCount > 1 {
					assert.Equal(t, "user2", members[1])
				}
			}
		})
	}
}
