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
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetMe(t *testing.T) {
	t.Parallel()

	serverTool := GetMe(translations.NullTranslationHelper)
	tool := serverTool.Tool
	testContextToolSnapshot[GetMeInput, MinimalUser](t, tool)

	// Verify some basic very important properties
	assert.Equal(t, "get_me", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_me tool should be read-only")

	// Setup mock user response
	mockUser := &github.User{
		Login:           new("testuser"),
		Name:            new("Test User"),
		Email:           new("test@example.com"),
		Bio:             new("GitHub user for testing"),
		Company:         new("Test Company"),
		Location:        new("Test Location"),
		HTMLURL:         new("https://github.com/testuser"),
		CreatedAt:       &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		Type:            new("User"),
		Hireable:        new(true),
		TwitterUsername: new("testuser_twitter"),
		Plan: &github.Plan{
			Name: new("pro"),
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
				deps = stubDeps{clientFn: stubClientFnErr(tc.clientErr), obsv: stubExporters()}
			} else {
				obs := stubExporters()
				deps = BaseDeps{Client: mustNewGHClient(t, tc.mockedClient), Obsv: obs}
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

func Test_GetMe_OmittedArguments(t *testing.T) {
	t.Parallel()

	mockUser := &github.User{
		Login:     new("testuser"),
		HTMLURL:   new("https://github.com/testuser"),
		CreatedAt: &github.Timestamp{Time: time.Now()},
	}
	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetUser: mockResponse(t, http.StatusOK, mockUser),
	})
	deps := BaseDeps{Client: mustNewGHClient(t, mockedClient), Obsv: stubExporters()}
	serverTool := GetMe(translations.NullTranslationHelper)
	handler := serverTool.Handler(deps)
	request := mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "get_me"},
	}

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.False(t, result.IsError)
	textContent := getTextResult(t, result)
	var returnedUser MinimalUser
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &returnedUser))
	assert.Equal(t, mockUser.GetLogin(), returnedUser.Login)
	assert.Equal(t, mockUser.GetHTMLURL(), returnedUser.ProfileURL)
}

func TestContextToolsTypedRegistration(t *testing.T) {
	mockUser := &github.User{
		Login:     new("testuser"),
		HTMLURL:   new("https://github.com/testuser"),
		CreatedAt: &github.Timestamp{Time: time.Now()},
	}
	teamsMatcher := githubv4mock.NewQueryMatcher(
		"query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}",
		map[string]any{"login": "specificuser"},
		githubv4mock.DataResponse(map[string]any{
			"user": map[string]any{
				"organizations": map[string]any{
					"nodes": []map[string]any{{
						"login": "testorg",
						"teams": map[string]any{
							"nodes": []map[string]any{{
								"name":        "team1",
								"slug":        "team1",
								"description": "Team 1",
							}},
						},
					}},
				},
			},
		}),
	)
	membersMatcher := githubv4mock.NewQueryMatcher(
		"query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}",
		map[string]any{"org": "testorg", "teamSlug": "testteam"},
		githubv4mock.DataResponse(map[string]any{
			"organization": map[string]any{
				"team": map[string]any{
					"members": map[string]any{
						"nodes": []map[string]any{{"login": "user1"}, {"login": "user2"}},
					},
				},
			},
		}),
	)
	mockedGQLClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(teamsMatcher, membersMatcher))
	failGetMe := false
	var graphQLCalls int
	deps := stubDeps{
		clientFn: stubClientFnFromHTTP(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetUser: func(w http.ResponseWriter, r *http.Request) {
				if failGetMe {
					badRequestHandler("expected typed output test failure")(w, r)
					return
				}
				mockResponse(t, http.StatusOK, mockUser)(w, r)
			},
		})),
		obsv: stubExporters(),
		gqlClientFn: func(context.Context) (*githubv4.Client, error) {
			graphQLCalls++
			return mockedGQLClient, nil
		},
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	getMeTool := GetMe(translations.NullTranslationHelper)
	getMeTool.RegisterFunc(server, nil)
	getTeamsTool := GetTeams(translations.NullTranslationHelper)
	getTeamsTool.RegisterFunc(server, nil)
	teamMembersTool := GetTeamMembers(translations.NullTranslationHelper)
	teamMembersTool.RegisterFunc(server, nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = clientSession.Close() })

	list, err := clientSession.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 3)
	for _, tool := range list.Tools {
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.OutputSchema)
	}

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_me"})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, result.StructuredContent)

	structuredJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, getTextResult(t, result).Text, string(structuredJSON))

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_teams",
		Arguments: map[string]any{"user": "specificuser"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	structuredJSON, err = json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"org":"testorg","teams":[{"name":"team1","slug":"team1","description":"Team 1"}]}]`, string(structuredJSON))
	assert.Equal(t, 1, graphQLCalls)

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_team_members",
		Arguments: map[string]any{"org": "testorg", "team_slug": "testteam"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	structuredJSON, err = json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, `["user1","user2"]`, string(structuredJSON))
	assert.Equal(t, 2, graphQLCalls)

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_team_members",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "missing required arguments should be rejected by the inferred input schema")
	assert.Equal(t, 2, graphQLCalls, "schema validation must happen before invoking the handler")

	failGetMe = true
	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_me"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Nil(t, result.StructuredContent, "handler errors must not expose a successful typed output")
}

func Test_GetMe_IFC_FeatureFlag(t *testing.T) {
	t.Parallel()

	serverTool := GetMe(translations.NullTranslationHelper)

	mockUser := &github.User{
		Login:     new("testuser"),
		HTMLURL:   new("https://github.com/testuser"),
		CreatedAt: &github.Timestamp{Time: time.Now()},
	}
	mockedHTTPClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetUser: mockResponse(t, http.StatusOK, mockUser),
	})

	depsWithIFCFeature := func(enabled bool) *BaseDeps {
		return NewBaseDeps(
			mustNewGHClient(t, mockedHTTPClient), nil, nil, nil,
			translations.NullTranslationHelper,
			FeatureFlags{},
			0,
			func(_ context.Context, flagName string) (bool, error) {
				return flagName == FeatureFlagIFCLabels && enabled, nil
			},
			stubExporters(),
		)
	}

	t.Run("feature disabled omits ifc label from result meta", func(t *testing.T) {
		deps := depsWithIFCFeature(false)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Nil(t, result.Meta, "result meta should be nil when IFC labels are disabled")
	})

	t.Run("feature enabled includes ifc label in result meta", func(t *testing.T) {
		deps := depsWithIFCFeature(true)
		handler := serverTool.Handler(deps)

		request := createMCPRequest(map[string]any{})
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.NotNil(t, result.Meta, "result meta should be set when IFC labels are enabled")
		ifcLabel, ok := result.Meta["ifc"]
		require.True(t, ok, "result meta should contain ifc key")

		ifcJSON, err := json.Marshal(ifcLabel)
		require.NoError(t, err)

		var ifcMap map[string]any
		err = json.Unmarshal(ifcJSON, &ifcMap)
		require.NoError(t, err)

		assert.Equal(t, "trusted", ifcMap["integrity"])
		// get_me returns the caller's private repo/gist counts, which are not
		// part of the public profile, so confidentiality is private.
		assert.Equal(t, "private", ifcMap["confidentiality"])
	})
}

func Test_GetTeams(t *testing.T) {
	t.Parallel()

	serverTool := GetTeams(translations.NullTranslationHelper)
	tool := serverTool.Tool
	testContextToolSnapshot[GetTeamsInput, []OrganizationTeams](t, tool)

	assert.Equal(t, "get_teams", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_teams tool should be read-only")

	mockUser := &github.User{
		Login:           new("testuser"),
		Name:            new("Test User"),
		Email:           new("test@example.com"),
		Bio:             new("GitHub user for testing"),
		Company:         new("Test Company"),
		Location:        new("Test Location"),
		HTMLURL:         new("https://github.com/testuser"),
		CreatedAt:       &github.Timestamp{Time: time.Now().Add(-365 * 24 * time.Hour)},
		Type:            new("User"),
		Hireable:        new(true),
		TwitterUsername: new("testuser_twitter"),
		Plan: &github.Plan{
			Name: new("pro"),
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
		vars := map[string]any{
			"login": "testuser",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamsResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientForSpecificuser := func() *githubv4.Client {
		queryStr := "query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}"
		vars := map[string]any{
			"login": "specificuser",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamsResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientNoTeams := func() *githubv4.Client {
		queryStr := "query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}"
		vars := map[string]any{
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
					Client:    mustNewGHClient(t, httpClientWithUser()),
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
					Client:    mustNewGHClient(t, httpClientWithUser()),
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
				return stubDeps{clientFn: stubClientFnErr("expected test error"), obsv: stubExporters()}
			},
			requestArgs:        map[string]any{},
			expectToolError:    true,
			expectedToolErrMsg: "failed to get GitHub client: expected test error",
		},
		{
			name: "get user fails",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					Client: mustNewGHClient(t, httpClientUserFails()),
					Obsv:   stubExporters(),
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
					clientFn:    stubClientFnFromHTTP(t, httpClientWithUser()),
					gqlClientFn: stubGQLClientFnErr("GraphQL client error"),
					obsv:        stubExporters(),
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
	testContextToolSnapshot[GetTeamMembersInput, []string](t, tool)

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
		vars := map[string]any{
			"org":      "testorg",
			"teamSlug": "testteam",
		}
		matcher := githubv4mock.NewQueryMatcher(queryStr, vars, mockTeamMembersResponse)
		httpClient := githubv4mock.NewMockedHTTPClient(matcher)
		return githubv4.NewClient(httpClient)
	}

	gqlClientNoMembers := func() *githubv4.Client {
		queryStr := "query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}"
		vars := map[string]any{
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
			deps: stubDeps{gqlClientFn: stubGQLClientFnErr("GraphQL client error"), obsv: stubExporters()},
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

func testContextToolSnapshot[In, Out any](t *testing.T, tool mcp.Tool) {
	t.Helper()

	if tool.InputSchema == nil {
		inputSchema, err := jsonschema.For[In](nil)
		require.NoError(t, err)
		tool.InputSchema = inputSchema
	}
	if tool.OutputSchema == nil {
		outputSchema, err := jsonschema.For[Out](nil)
		require.NoError(t, err)
		tool.OutputSchema = outputSchema
	}
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
}
