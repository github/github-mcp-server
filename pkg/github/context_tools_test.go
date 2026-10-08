package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/githubv4mock"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
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
		CreatedAt: &github.Timestamp{Time: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)},
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
	emptyOrganizationsMatcher := githubv4mock.NewQueryMatcher(
		"query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}",
		map[string]any{"login": "no-orgs"},
		githubv4mock.DataResponse(map[string]any{
			"user": map[string]any{"organizations": map[string]any{"nodes": []any{}}},
		}),
	)
	emptyTeamsMatcher := githubv4mock.NewQueryMatcher(
		"query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}",
		map[string]any{"login": "no-teams"},
		githubv4mock.DataResponse(map[string]any{
			"user": map[string]any{"organizations": map[string]any{"nodes": []any{
				map[string]any{"login": "testorg", "teams": map[string]any{"nodes": []any{}}},
			}}},
		}),
	)
	emptyMembersMatcher := githubv4mock.NewQueryMatcher(
		"query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}",
		map[string]any{"org": "testorg", "teamSlug": "emptyteam"},
		githubv4mock.DataResponse(map[string]any{
			"organization": map[string]any{"team": map[string]any{"members": map[string]any{"nodes": []any{}}}},
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
	getTeamsTool := GetTeams(translations.NullTranslationHelper)
	teamMembersTool := GetTeamMembers(translations.NullTranslationHelper)
	inv, err := inventory.NewBuilder().
		SetTools([]inventory.ServerTool{getMeTool, getTeamsTool, teamMembersTool}).
		WithToolsets([]string{"all"}).
		Build()
	require.NoError(t, err)
	inv.RegisterTools(context.Background(), server, deps)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	clientSession, err := client.Connect(context.Background(), clientTransport, &mcp.ClientSessionOptions{
		ProtocolVersion: inventory.ProtocolVersionMultiRoundTrip,
	})
	require.NoError(t, err)
	require.Equal(t, inventory.ProtocolVersionMultiRoundTrip, clientSession.InitializeResult().ProtocolVersion)
	t.Cleanup(func() { _ = clientSession.Close() })

	protocolMeta := mcp.Meta{mcp.MetaKeyProtocolVersion: inventory.ProtocolVersionMultiRoundTrip}
	list, err := clientSession.ListTools(context.Background(), &mcp.ListToolsParams{Meta: protocolMeta})
	require.NoError(t, err)
	require.Len(t, list.Tools, 3)
	outputSchemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		assert.NotNil(t, tool.InputSchema)
		assert.NotNil(t, tool.OutputSchema)
		outputJSON, err := json.Marshal(tool.OutputSchema)
		require.NoError(t, err)
		var outputSchema jsonschema.Schema
		require.NoError(t, json.Unmarshal(outputJSON, &outputSchema))
		outputSchemas[tool.Name], err = outputSchema.Resolve(nil)
		require.NoError(t, err)
		schemaJSON, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
			Required   []string                   `json:"required"`
			AnyOf      []json.RawMessage          `json:"anyOf"`
		}
		require.NoError(t, json.Unmarshal(schemaJSON, &schema))
		assert.Equal(t, "object", schema.Type, "input roots must remain non-nullable objects")
		assert.Empty(t, schema.AnyOf)
		var schemaMetadata struct {
			AdditionalProperties *bool `json:"additionalProperties"`
		}
		require.NoError(t, json.Unmarshal(schemaJSON, &schemaMetadata))
		assert.Nil(t, schemaMetadata.AdditionalProperties, "advertised input schema must match the legacy snapshot bytes")
		for propertyName, property := range schema.Properties {
			var propertySchema jsonschema.Schema
			require.NoError(t, json.Unmarshal(property, &propertySchema))
			assert.Empty(t, propertySchema.Enum, "input enums must match the legacy schema")
			assert.Nil(t, propertySchema.Minimum)
			assert.Nil(t, propertySchema.Maximum)
			assert.Nil(t, propertySchema.MinLength, "input bounds must match the legacy schema")
			assert.Nil(t, propertySchema.MaxLength)
			assert.Nil(t, propertySchema.Default)
			switch tool.Name + "." + propertyName {
			case "get_teams.user":
				assert.Equal(t, "Username to get teams for. If not provided, uses the authenticated user.", propertySchema.Description)
			case "get_team_members.org":
				assert.Equal(t, "Organization login (owner) that contains the team.", propertySchema.Description)
			case "get_team_members.team_slug":
				assert.Equal(t, "Team slug", propertySchema.Description)
			default:
				t.Fatalf("unexpected input property %q on tool %q", propertyName, tool.Name)
			}
		}
		switch tool.Name {
		case "get_me":
			assert.NotNil(t, schema.Properties, "empty input schemas must retain properties")
			assert.Empty(t, schema.Properties)
			assert.Empty(t, schema.Required)
		case "get_teams":
			assert.Contains(t, schema.Properties, "user")
			assert.Empty(t, schema.Required, "user must remain optional")
		case "get_team_members":
			assert.ElementsMatch(t, []string{"org", "team_slug"}, schema.Required)
		default:
			t.Fatalf("unexpected context tool %q", tool.Name)
		}
	}

	result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_me",
		Arguments: map[string]any{"legacy_ignored_argument": true},
		Meta:      protocolMeta,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, result.StructuredContent)

	structuredJSON, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, getTextResult(t, result).Text, string(structuredJSON))
	var returnedUser MinimalUser
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returnedUser))
	legacyText, err := json.Marshal(returnedUser)
	require.NoError(t, err)
	assert.JSONEq(t, string(legacyText), getTextResult(t, result).Text)
	request := createMCPRequest(map[string]any{})
	legacyResult, err := getMeTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.Equal(t, `{"login":"testuser","profile_url":"https://github.com/testuser","details":{"public_repos":0,"public_gists":0,"followers":0,"following":0,"created_at":"2020-01-02T03:04:05Z","updated_at":"0001-01-01T00:00:00Z"}}`, getTextResult(t, legacyResult).Text)
	require.NoError(t, outputSchemas["get_me"].Validate(result.StructuredContent))

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_teams",
		Arguments: map[string]any{"user": "specificuser", "legacy_ignored_argument": true},
		Meta:      protocolMeta,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	structuredJSON, err = json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"org":"testorg","teams":[{"name":"team1","slug":"team1","description":"Team 1"}]}]`, string(structuredJSON))
	assert.JSONEq(t, string(structuredJSON), getTextResult(t, result).Text)
	request = createMCPRequest(map[string]any{"user": "specificuser"})
	legacyResult, err = getTeamsTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.Equal(t, `[{"org":"testorg","teams":[{"name":"team1","slug":"team1","description":"Team 1"}]}]`, getTextResult(t, legacyResult).Text)
	require.NoError(t, outputSchemas["get_teams"].Validate(result.StructuredContent))
	assert.Equal(t, 2, graphQLCalls)

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_team_members",
		Arguments: map[string]any{"org": "testorg", "team_slug": "testteam", "legacy_ignored_argument": true},
		Meta:      protocolMeta,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	structuredJSON, err = json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, `["user1","user2"]`, string(structuredJSON))
	assert.JSONEq(t, string(structuredJSON), getTextResult(t, result).Text)
	request = createMCPRequest(map[string]any{"org": "testorg", "team_slug": "testteam"})
	legacyResult, err = teamMembersTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	assert.Equal(t, `["user1","user2"]`, getTextResult(t, legacyResult).Text)
	require.NoError(t, outputSchemas["get_team_members"].Validate(result.StructuredContent))
	assert.Equal(t, 4, graphQLCalls)

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_team_members",
		Arguments: map[string]any{},
		Meta:      protocolMeta,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "missing required arguments should be rejected by the inferred input schema")
	assert.Equal(t, 4, graphQLCalls, "schema validation must happen before invoking the handler")

	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_team_members",
		Arguments: map[string]any{"org": "", "team_slug": "testteam"},
		Meta:      protocolMeta,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError, "empty required strings should remain invalid")
	assert.Equal(t, 4, graphQLCalls, "handler validation must reject empty identifiers before acquiring GraphQL")

	for _, tc := range []struct {
		name       string
		args       map[string]any
		text       string
		structured string
		matcher    githubv4mock.Matcher
	}{
		{"get_teams", map[string]any{"user": "no-orgs"}, "null", "[]", emptyOrganizationsMatcher},
		{"get_teams", map[string]any{"user": "no-teams"}, `[{"org":"testorg","teams":[]}]`, `[{"org":"testorg","teams":[]}]`, emptyTeamsMatcher},
		{"get_team_members", map[string]any{"org": "testorg", "team_slug": "emptyteam"}, "null", "[]", emptyMembersMatcher},
	} {
		mockedGQLClient = githubv4.NewClient(githubv4mock.NewMockedHTTPClient(tc.matcher))
		serverTool := getTeamsTool
		if tc.name == "get_team_members" {
			serverTool = teamMembersTool
		}
		request := createMCPRequest(tc.args)
		legacyResult, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, legacyResult.IsError)
		assert.Equal(t, tc.text, getTextResult(t, legacyResult).Text)
		result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args, Meta: protocolMeta})
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.NotNil(t, result.StructuredContent, "empty successful collections must have structured content on the wire")
		require.Len(t, result.Content, 1, "SDK fallback must not duplicate the legacy text")
		structuredJSON, err := json.Marshal(result.StructuredContent)
		require.NoError(t, err)
		assert.JSONEq(t, tc.structured, string(structuredJSON))
		assert.JSONEq(t, string(structuredJSON), getTextResult(t, result).Text)
		require.NoError(t, outputSchemas[tc.name].Validate(result.StructuredContent))
	}

	failGetMe = true
	result, err = clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_me", Meta: protocolMeta})
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
			name: "empty user uses authenticated user",
			makeDeps: func() ToolDependencies {
				return BaseDeps{
					Client:    mustNewGHClient(t, httpClientWithUser()),
					GQLClient: gqlClientForTestuser(),
				}
			},
			requestArgs:        map[string]any{"user": ""},
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

func Test_GetTeams_NullUserForDirectHandler(t *testing.T) {
	t.Parallel()

	clientCalls := 0
	gqlClientCalls := 0
	deps := stubDeps{
		clientFn: func(context.Context) (*github.Client, error) {
			clientCalls++
			return nil, nil
		},
		gqlClientFn: func(context.Context) (*githubv4.Client, error) {
			gqlClientCalls++
			return nil, nil
		},
		obsv: stubExporters(),
	}
	serverTool := GetTeams(translations.NullTranslationHelper)
	request := createMCPRequest(map[string]any{"user": nil})
	result, err := serverTool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)

	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Equal(t, "parameter user is not of type string, is <nil>", getErrorResult(t, result).Text)
	assert.Nil(t, result.StructuredContent)
	assert.Zero(t, clientCalls)
	assert.Zero(t, gqlClientCalls)
}

func TestNormalizeGetTeamsInput(t *testing.T) {
	t.Parallel()

	for _, arguments := range []string{`{}`, `{"user":""}`, `{"user":"octocat"}`, `{"legacy_ignored_argument":true}`} {
		t.Run(arguments, func(t *testing.T) {
			normalized, err := normalizeGetTeamsInput(json.RawMessage(arguments))
			require.NoError(t, err)
			assert.Equal(t, arguments, string(normalized))
		})
	}
	for _, arguments := range []string{`{"user":null}`, `{"user": null }`} {
		t.Run(arguments, func(t *testing.T) {
			_, err := normalizeGetTeamsInput(json.RawMessage(arguments))
			var inputError *inventory.ToolInputError
			require.ErrorAs(t, err, &inputError)
			assert.Equal(t, "parameter user is not of type string, is <nil>", inputError.Message)
		})
	}
}

func TestContextToolsExactRoutingKeys(t *testing.T) {
	for _, mode := range []string{"direct", "legacy", "modern", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			var clientCalls, gqlCalls int
			deps := stubDeps{
				clientFn: func(ctx context.Context) (*github.Client, error) {
					clientCalls++
					return stubClientFnFromHTTP(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						GetUser: mockResponse(t, http.StatusOK, &github.User{Login: new("authenticated")}),
					}))(ctx)
				},
				gqlClientFn: func(context.Context) (*githubv4.Client, error) {
					gqlCalls++
					return githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
						githubv4mock.NewQueryMatcher(
							"query($login:String!){user(login: $login){organizations(first: 100){nodes{login,teams(first: 100, userLogins: [$login]){nodes{name,slug,description}}}}}}",
							map[string]any{"login": "authenticated"},
							githubv4mock.DataResponse(map[string]any{
								"user": map[string]any{"organizations": map[string]any{"nodes": []any{}}},
							}),
						),
						githubv4mock.NewQueryMatcher(
							"query($org:String!$teamSlug:String!){organization(login: $org){team(slug: $teamSlug){members(first: 100){nodes{login}}}}}",
							map[string]any{"org": "testorg", "teamSlug": "testteam"},
							githubv4mock.DataResponse(map[string]any{
								"organization": map[string]any{"team": map[string]any{"members": map[string]any{"nodes": []any{}}}},
							}),
						),
					)), nil
				},
				obsv: stubExporters(),
			}
			tools := []inventory.ServerTool{
				GetTeams(translations.NullTranslationHelper),
				GetTeamMembers(translations.NullTranslationHelper),
			}
			call := func(name string, args map[string]any) *mcp.CallToolResult {
				for _, tool := range tools {
					if tool.Tool.Name == name {
						request := createMCPRequest(args)
						if mode == "unknown" {
							request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: "2099-01-01"}
						}
						result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
						require.NoError(t, err)
						return result
					}
				}
				t.Fatalf("unknown tool %s", name)
				return nil
			}
			if mode == "legacy" || mode == "modern" {
				server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
				require.NoError(t, err)
				inv.RegisterTools(context.Background(), server, deps)
				serverTransport, clientTransport := mcp.NewInMemoryTransports()
				serverSession, err := server.Connect(context.Background(), serverTransport, nil)
				require.NoError(t, err)
				t.Cleanup(func() { _ = serverSession.Close() })
				client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
				version := "2025-03-26"
				if mode == "modern" {
					version = inventory.ProtocolVersionMultiRoundTrip
				}
				session, err := client.Connect(context.Background(), clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: version})
				require.NoError(t, err)
				t.Cleanup(func() { _ = session.Close() })
				meta := mcp.Meta{}
				if mode == "modern" {
					meta[mcp.MetaKeyProtocolVersion] = inventory.ProtocolVersionMultiRoundTrip
				}
				call = func(name string, args map[string]any) *mcp.CallToolResult {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
						Name: name, Arguments: args, Meta: meta,
					})
					require.NoError(t, err)
					return result
				}
			}
			for _, args := range []map[string]any{
				{"USER": "wrong-user", "unrelated": true},
				{"UsEr": nil},
				{"user": "", "USER": 42},
				{"user": "authenticated", "USER": "wrong-user"},
			} {
				beforeREST := clientCalls
				beforeGQL := gqlCalls
				result := call("get_teams", args)
				require.False(t, result.IsError)
				assert.Equal(t, beforeGQL+1, gqlCalls)
				if args["user"] == "authenticated" {
					assert.Equal(t, beforeREST, clientCalls)
				} else {
					assert.Equal(t, beforeREST+1, clientCalls)
				}
				if mode == "modern" {
					assert.Equal(t, "[]", getTextResult(t, result).Text)
					assert.NotNil(t, result.StructuredContent)
				} else {
					assert.Equal(t, "null", getTextResult(t, result).Text)
					assert.Nil(t, result.StructuredContent)
				}
			}
			beforeREST, beforeGQL := clientCalls, gqlCalls
			result := call("get_teams", map[string]any{"user": nil, "USER": "authenticated"})
			require.True(t, result.IsError)
			assert.Equal(t, "parameter user is not of type string, is <nil>", getErrorResult(t, result).Text)
			assert.Equal(t, beforeREST, clientCalls)
			assert.Equal(t, beforeGQL, gqlCalls)
			for _, args := range []map[string]any{
				{"ORG": "testorg", "team_slug": "testteam"},
				{"org": "testorg", "TEAM_SLUG": "testteam"},
				{"ORG": "testorg", "TEAM_SLUG": "testteam"},
			} {
				result := call("get_team_members", args)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				assert.Equal(t, beforeGQL, gqlCalls)
			}
			result = call("get_team_members", map[string]any{
				"org": "testorg", "team_slug": "testteam", "ORG": 42, "TEAM_SLUG": nil, "unrelated": true,
			})
			require.False(t, result.IsError)
			assert.Equal(t, beforeGQL+1, gqlCalls)
			if mode == "modern" {
				assert.Equal(t, "[]", getTextResult(t, result).Text)
			} else {
				assert.Equal(t, "null", getTextResult(t, result).Text)
			}
		})
	}
}

func TestNormalizeContextRoutingKeys(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		normalize inventory.InputNormalizer
		args      string
		want      string
	}{
		{"teams", normalizeGetTeamsInput, `{"user":"exact","USER":42,"UsEr":null,"unknown":{"USER":"retained"}}`, `{"user":"exact","unknown":{"USER":"retained"}}`},
		{"members", normalizeGetTeamMembersInput, `{"org":"exact","ORG":false,"team_slug":"slug","Team_Slug":null,"unknown":true}`, `{"org":"exact","team_slug":"slug","unknown":true}`},
		{"canonical null retained", normalizeGetTeamMembersInput, `{"org":null,"team_slug":null,"ORG":"ignored"}`, `{"org":null,"team_slug":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.normalize(json.RawMessage(tc.args))
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(result))
		})
	}
}

func Test_GetTeamMembers_RequiredIdentifiersForDirectHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          map[string]any
		expectedError string
	}{
		{
			name:          "missing organization",
			args:          map[string]any{"team_slug": "testteam"},
			expectedError: "missing required parameter: org",
		},
		{
			name:          "empty organization",
			args:          map[string]any{"org": "", "team_slug": "testteam"},
			expectedError: "missing required parameter: org",
		},
		{
			name:          "missing team slug",
			args:          map[string]any{"org": "testorg"},
			expectedError: "missing required parameter: team_slug",
		},
		{
			name:          "empty team slug",
			args:          map[string]any{"org": "testorg", "team_slug": ""},
			expectedError: "missing required parameter: team_slug",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gqlClientCalls := 0
			deps := stubDeps{
				gqlClientFn: func(context.Context) (*githubv4.Client, error) {
					gqlClientCalls++
					return nil, nil
				},
				obsv: stubExporters(),
			}
			serverTool := GetTeamMembers(translations.NullTranslationHelper)
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.args)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, tc.expectedError)
			assert.Zero(t, gqlClientCalls, "invalid identifiers must fail before acquiring the GraphQL client")
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
