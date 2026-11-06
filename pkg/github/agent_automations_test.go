package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/requeststate"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func agentAutomationToolsForTest() []inventory.ServerTool {
	t := translations.NullTranslationHelper
	return []inventory.ServerTool{
		ListAutomations(t),
		GetAutomation(t),
		ListAutomationRevisions(t),
		GetAutomationRevision(t),
		ListAvailableAutomationTools(t),
		ListAvailableAutomationTriggers(t),
		GetAutomationsConfigurationForRepository(t),
		CreateAutomation(t),
		UpdateAutomation(t),
		DeleteAutomation(t),
	}
}

func TestAgentAutomationReadToolSnapshots(t *testing.T) {
	t.Parallel()

	for _, tool := range agentAutomationToolsForTest() {
		require.NoError(t, toolsnaps.Test(tool.Tool.Name, tool.Tool))
	}
}

func TestAgentAutomationFeatureFlagAvailability(t *testing.T) {
	t.Parallel()

	automationTools := agentAutomationToolsForTest()
	var automationNames []string
	for _, tool := range automationTools {
		automationNames = append(automationNames, tool.Tool.Name)
		assert.Equal(t, []inventory.FeatureFlag{FeatureFlagAgentAutomations}, tool.FeatureRule.Features())
	}
	assert.Contains(t, AllowedFeatureFlags, FeatureFlagAgentAutomations)
	assert.NotContains(t, InsidersFeatureFlags, FeatureFlagAgentAutomations)

	for _, tc := range []struct {
		name            string
		enabledFeatures []string
		insidersMode    bool
		readOnly        bool
		wantAvailable   bool
	}{
		{name: "default with all toolsets"},
		{name: "insiders without explicit flag", insidersMode: true},
		{
			name:            "explicit opt-in",
			enabledFeatures: []string{FeatureFlagAgentAutomations},
			wantAvailable:   true,
		},
		{
			name:            "explicit opt-in in read-only mode",
			enabledFeatures: []string{FeatureFlagAgentAutomations},
			readOnly:        true,
			wantAvailable:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enabled := ResolveFeatureFlags(tc.enabledFeatures, tc.insidersMode)[FeatureFlagAgentAutomations]
			var flags []inventory.FeatureFlag
			if enabled {
				flags = append(flags, FeatureFlagAgentAutomations)
			}
			inv, err := inventory.NewBuilder().
				SetTools(AllTools(translations.NullTranslationHelper)).
				WithToolsets([]string{"all"}).
				WithReadOnly(tc.readOnly).
				WithFeatureChecker(featureCheckerFor(flags...)).
				Build()
			require.NoError(t, err)

			var availableNames []string
			for _, tool := range inv.AvailableTools(context.Background()) {
				for _, name := range automationNames {
					if tool.Tool.Name == name {
						availableNames = append(availableNames, name)
					}
				}
			}
			if tc.wantAvailable {
				expectedNames := automationNames
				if tc.readOnly {
					expectedNames = nil
					for _, tool := range automationTools {
						if tool.IsReadOnly() {
							expectedNames = append(expectedNames, tool.Tool.Name)
						}
					}
				}
				assert.ElementsMatch(t, expectedNames, availableNames)
				return
			}
			assert.Empty(t, availableNames)

			server := mcp.NewServer(&mcp.Implementation{Name: "agent-automations-test", Version: "v1"}, nil)
			inv.RegisterAll(context.Background(), server, BaseDeps{})
			session := connectCommentVisibilityClient(t, server, inventory.ProtocolVersionMultiRoundTrip)
			for _, name := range automationNames {
				_, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
				require.Error(t, err, "direct call to unavailable tool %q must fail", name)
			}
		})
	}
}

func TestAgentAutomationReadTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tool     func(translations.TranslationHelperFunc) inventory.ServerTool
		args     map[string]any
		endpoint string
		query    string
		response string
	}{
		{
			name: "list automations",
			tool: ListAutomations,
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "page": 2, "perPage": 20, "type": "shared",
			},
			endpoint: "/agents/repos/octo-org/octo-repo/automations",
			query:    "page=2&per_page=20&type=shared",
			response: `{"automations":[],"total_count":0}`,
		},
		{
			name:     "get automation",
			tool:     GetAutomation,
			args:     map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id"},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/automation-id",
			response: `{"id":"automation-id","name":"Issue triage"}`,
		},
		{
			name: "list automation revisions",
			tool: ListAutomationRevisions,
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id",
				"cursor": "next-cursor", "perPage": 10,
			},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/automation-id/revisions",
			query:    "cursor=next-cursor&per_page=10",
			response: `{"revisions":[]}`,
		},
		{
			name: "get automation revision",
			tool: GetAutomationRevision,
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id", "revision_id": "revision-id",
			},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/automation-id/revisions/revision-id",
			response: `{"id":"revision-id","revision_number":1}`,
		},
		{
			name:     "list available automation tools",
			tool:     ListAvailableAutomationTools,
			args:     map[string]any{"owner": "octo-org", "repo": "octo-repo"},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/available-tools",
			response: `{"tool_groups":[],"mcp_servers":[]}`,
		},
		{
			name:     "list available automation triggers",
			tool:     ListAvailableAutomationTriggers,
			args:     map[string]any{"owner": "octo-org", "repo": "octo-repo"},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/available-triggers",
			response: `[]`,
		},
		{
			name:     "get repository automation configuration",
			tool:     GetAutomationsConfigurationForRepository,
			args:     map[string]any{"owner": "octo-org", "repo": "octo-repo"},
			endpoint: "/agents/repos/octo-org/octo-repo/automations/configuration",
			response: `{"automations_enabled":true,"can_manage_automations":false,"allow_read_access_triggers":true,"require_trigger_scoped_writes":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool := tt.tool(translations.NullTranslationHelper)
			httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.endpoint, r.URL.Path)
				assert.Equal(t, tt.query, r.URL.RawQuery)
				assert.Equal(t, agentAutomationAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			request := createMCPRequest(tt.args)
			result, err := tool.Handler(nil)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.JSONEq(t, tt.response, getTextResult(t, result).Text)
		})
	}
}

func TestAgentAutomationReadToolsSanitizeNames(t *testing.T) {
	t.Parallel()

	response := `{"automations":[{"id":"automation-id","name":"<b>Issue</b>\u202E triage","prompt":"<b>preserved prompt</b>"}],"next_cursor":"cursor"}`
	expected := `{"automations":[{"id":"automation-id","name":"Issue triage","prompt":"<b>preserved prompt</b>"}],"next_cursor":"cursor"}`
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	})
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
	tool := ListAutomations(translations.NullTranslationHelper)
	inv, err := inventory.NewBuilder().SetTools([]inventory.ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-automations-test", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	session := connectCommentVisibilityClient(t, server, inventory.ProtocolVersionMultiRoundTrip)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      tool.Tool.Name,
		Arguments: map[string]any{"owner": "octo-org", "repo": "octo-repo"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.JSONEq(t, expected, getTextResult(t, result).Text)
	structuredContent, err := json.Marshal(result.StructuredContent)
	require.NoError(t, err)
	assert.JSONEq(t, expected, string(structuredContent))
}

func TestAgentAutomationReadToolsRejectUnsafeIDsAndPathSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tool func(translations.TranslationHelperFunc) inventory.ServerTool
		args map[string]any
	}{
		{
			name: "dot owner",
			tool: ListAutomations,
			args: map[string]any{"owner": ".", "repo": "octo-repo"},
		},
		{
			name: "dot-dot repo",
			tool: ListAutomations,
			args: map[string]any{"owner": "octo-org", "repo": ".."},
		},
	}
	for _, automationID := range []string{".", "..", "available-tools", "available-triggers", "configuration", "revisions"} {
		tests = append(tests, struct {
			name string
			tool func(translations.TranslationHelperFunc) inventory.ServerTool
			args map[string]any
		}{
			name: "reserved or dot automation ID " + automationID,
			tool: GetAutomation,
			args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": automationID},
		})
	}
	tests = append(tests, struct {
		name string
		tool func(translations.TranslationHelperFunc) inventory.ServerTool
		args map[string]any
	}{
		name: "dot revision ID",
		tool: GetAutomationRevision,
		args: map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id", "revision_id": ".."},
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			httpClient := MockHTTPClientWithHandler(func(http.ResponseWriter, *http.Request) {
				t.Fatal("invalid path segment must not make a request")
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			request := createMCPRequest(tt.args)
			tool := tt.tool(translations.NullTranslationHelper)
			result, err := tool.Handler(nil)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestAutomationCatalogToolsRequireRepository(t *testing.T) {
	t.Parallel()

	for _, tool := range []struct {
		name string
		def  func(translations.TranslationHelperFunc) inventory.ServerTool
	}{
		{name: "available tools", def: ListAvailableAutomationTools},
		{name: "available triggers", def: ListAvailableAutomationTriggers},
		{name: "repository configuration", def: GetAutomationsConfigurationForRepository},
	} {
		t.Run(tool.name, func(t *testing.T) {
			definition := tool.def(translations.NullTranslationHelper)
			schema := definition.Tool.InputSchema.(*jsonschema.Schema)
			assert.ElementsMatch(t, []string{"owner", "repo"}, schema.Required)
			assert.Contains(t, schema.Properties, "owner")
			assert.Contains(t, schema.Properties, "repo")
		})
	}
}

func TestGetAutomationsConfigurationForRepositoryOutputSchema(t *testing.T) {
	t.Parallel()

	tool := GetAutomationsConfigurationForRepository(translations.NullTranslationHelper)
	var schema struct {
		Type       string `json:"type"`
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	schemaJSON, err := json.Marshal(tool.Tool.OutputSchema)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(schemaJSON, &schema))
	require.Equal(t, "object", schema.Type)
	assert.ElementsMatch(t, []string{
		"automations_enabled",
		"can_manage_automations",
		"allow_read_access_triggers",
		"require_trigger_scoped_writes",
	}, schema.Required)
	for _, field := range schema.Required {
		assert.Equal(t, "boolean", schema.Properties[field].Type, field)
	}
}

func TestGetAutomationsConfigurationForRepositoryReturnsAPIError(t *testing.T) {
	t.Parallel()

	tool := GetAutomationsConfigurationForRepository(translations.NullTranslationHelper)
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/agents/repos/octo-org/octo-repo/automations/configuration", r.URL.Path)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Resource not accessible by integration"}`))
	})
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
	request := createMCPRequest(map[string]any{"owner": "octo-org", "repo": "octo-repo"})
	result, err := tool.Handler(nil)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.NotEmpty(t, getErrorResult(t, result).Text)
}

func TestGetAutomationsConfigurationForRepositoryDoesNotCacheResponses(t *testing.T) {
	t.Parallel()

	tool := GetAutomationsConfigurationForRepository(translations.NullTranslationHelper)
	responses := []string{
		`{"automations_enabled":true,"can_manage_automations":true,"allow_read_access_triggers":false,"require_trigger_scoped_writes":false}`,
		`{"automations_enabled":true,"can_manage_automations":false,"allow_read_access_triggers":true,"require_trigger_scoped_writes":true}`,
	}
	calls := 0
	httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/agents/repos/octo-org/octo-repo/automations/configuration", r.URL.Path)
		_, _ = w.Write([]byte(responses[calls]))
		calls++
	})
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
	for _, response := range responses {
		request := createMCPRequest(map[string]any{"owner": "octo-org", "repo": "octo-repo"})
		result, err := tool.Handler(nil)(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.JSONEq(t, response, getTextResult(t, result).Text)
	}
	assert.Equal(t, len(responses), calls)
}

func TestAgentAutomationWriteTools(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		tool            func(translations.TranslationHelperFunc) inventory.ServerTool
		args            map[string]any
		method          string
		endpoint        string
		status          int
		body            string
		response        string
		confirm         bool
		confirmID       string
		confirmValue    string
		messageContents []string
	}{
		{
			name: "create automation",
			tool: CreateAutomation,
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "name": "Issue triage", "prompt": "Triage new issues.",
				"type": "shared", "enabled": false, "tools": []string{"github/issue_read"},
				"triggers": map[string]any{"issue_opened": map[string]any{"query": "is:open"}},
			},
			method:          http.MethodPost,
			endpoint:        "/agents/repos/octo-org/octo-repo/automations",
			status:          http.StatusCreated,
			body:            `{"name":"Issue triage","prompt":"Triage new issues.","type":"shared","enabled":false,"tools":["github/issue_read"],"triggers":{"issue_opened":{"query":"is:open"}}}`,
			response:        `{"id":"automation-id","name":"Issue triage"}`,
			confirm:         true,
			confirmID:       agentAutomationConfirmationIDPrefix + "create",
			confirmValue:    "CREATE octo-org/octo-repo",
			messageContents: []string{`"name": "Issue triage"`, `"prompt": "Triage new issues."`},
		},
		{
			name: "update automation",
			tool: UpdateAutomation,
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id",
				"enabled": false, "prompt": "Review new issues.", "tools": []string{},
			},
			method:          http.MethodPatch,
			endpoint:        "/agents/repos/octo-org/octo-repo/automations/automation-id",
			status:          http.StatusOK,
			body:            `{"enabled":false,"prompt":"Review new issues.","tools":[]}`,
			response:        `{"id":"automation-id","prompt":"Review new issues."}`,
			confirm:         true,
			confirmID:       agentAutomationConfirmationIDPrefix + "update",
			confirmValue:    "UPDATE octo-org/octo-repo/automation-id",
			messageContents: []string{`"enabled": false`, `"prompt": "Review new issues."`},
		},
		{
			name:         "delete automation",
			tool:         DeleteAutomation,
			args:         map[string]any{"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id"},
			method:       http.MethodDelete,
			endpoint:     "/agents/repos/octo-org/octo-repo/automations/automation-id",
			status:       http.StatusNoContent,
			response:     `{"message":"Automation deleted."}`,
			confirm:      true,
			confirmID:    agentAutomationConfirmationIDPrefix + "delete",
			confirmValue: "DELETE octo-org/octo-repo/automation-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tool := tt.tool(translations.NullTranslationHelper)
			httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.method, r.Method)
				assert.Equal(t, tt.endpoint, r.URL.Path)
				assert.Equal(t, agentAutomationAPIVersion, r.Header.Get("X-GitHub-Api-Version"))
				if tt.body == "" {
					assert.Empty(t, r.Body)
				} else {
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, tt.body, string(body))
				}
				w.WriteHeader(tt.status)
				if tt.status != http.StatusNoContent {
					_, _ = w.Write([]byte(tt.response))
				}
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			var result *mcp.CallToolResult
			if tt.confirm {
				sealer, err := requeststate.NewRandom()
				require.NoError(t, err)
				deps.StateSealer = sealer
				result = invokeConfirmedAutomation(t, tool, deps, tt.args, tt.confirmID, tt.confirmValue, tt.messageContents...)
			} else {
				request := createMCPRequest(tt.args)
				var err error
				result, err = tool.Handler(nil)(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
			}
			require.False(t, result.IsError)
			assert.JSONEq(t, tt.response, getTextResult(t, result).Text)
		})
	}
}

func TestAutomationCreateAndUpdateRequireRequestStateProtection(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		tool inventory.ServerTool
		args map[string]any
	}{
		{
			name: "create",
			tool: CreateAutomation(translations.NullTranslationHelper),
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "name": "Issue triage", "prompt": "Triage new issues.",
			},
		},
		{
			name: "update",
			tool: UpdateAutomation(translations.NullTranslationHelper),
			args: map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id", "enabled": false,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpClient := MockHTTPClientWithHandler(func(http.ResponseWriter, *http.Request) {
				t.Fatal("mutation should not be sent without request-state protection")
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			request := createMCPRequest(tc.args)
			result, err := tc.tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, "request-state protection is not configured")
		})
	}
}

func TestDeleteAutomationRequiresRequestStateProtection(t *testing.T) {
	t.Parallel()

	tool := DeleteAutomation(translations.NullTranslationHelper)
	httpClient := MockHTTPClientWithHandler(func(http.ResponseWriter, *http.Request) {
		t.Fatal("delete should not be sent without request-state protection")
	})
	deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
	request := createMCPRequest(map[string]any{
		"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id",
	})
	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, "request-state protection is not configured")
}

func TestDeleteAutomationRequiresMatchingConfirmation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name               string
		action             string
		confirmation       string
		changeAutomationID bool
		wantError          string
	}{
		{
			name:         "declined",
			action:       "decline",
			confirmation: "DELETE octo-org/octo-repo/automation-id",
			wantError:    "was not confirmed",
		},
		{
			name:         "incorrect confirmation",
			action:       "accept",
			confirmation: "DELETE octo-org/octo-repo/another-automation",
			wantError:    "did not match",
		},
		{
			name:               "changed target",
			action:             "accept",
			confirmation:       "DELETE octo-org/octo-repo/automation-id",
			changeAutomationID: true,
			wantError:          "changed after confirmation",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := DeleteAutomation(translations.NullTranslationHelper)
			requestCount := 0
			httpClient := MockHTTPClientWithHandler(func(http.ResponseWriter, *http.Request) {
				requestCount++
			})
			deps := BaseDeps{Client: mustNewGHClient(t, httpClient)}
			sealer, err := requeststate.NewRandom()
			require.NoError(t, err)
			deps.StateSealer = sealer

			args := map[string]any{
				"owner": "octo-org", "repo": "octo-repo", "automation_id": "automation-id",
			}
			handler := tool.Handler(deps)
			request := createMCPRequest(args)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.changeAutomationID {
				args["automation_id"] = "another-automation"
			}
			retry := createMCPRequest(args)
			retry.Params.RequestState = result.RequestState
			retry.Params.InputResponses = mcp.InputResponseMap{
				agentAutomationConfirmationIDPrefix + "delete": &mcp.ElicitResult{
					Action: tc.action,
					Content: map[string]any{
						agentAutomationConfirmationField: tc.confirmation,
					},
				},
			}
			result, err = handler(ContextWithDeps(context.Background(), deps), &retry)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, tc.wantError)
			assert.Zero(t, requestCount)
		})
	}
}

func TestAutomationMutationsRequireFormElicitation(t *testing.T) {
	t.Parallel()

	for _, tool := range []inventory.ServerTool{
		CreateAutomation(translations.NullTranslationHelper),
		UpdateAutomation(translations.NullTranslationHelper),
		DeleteAutomation(translations.NullTranslationHelper),
	} {
		assert.Equal(t, inventory.ProtocolVersionMultiRoundTrip, tool.MinimumProtocolVersion)
		assert.Equal(t, inventory.ElicitationModeForm, tool.RequiredElicitationMode)
	}
}

func invokeConfirmedAutomation(
	t *testing.T,
	tool inventory.ServerTool,
	deps BaseDeps,
	args map[string]any,
	confirmationID, confirmationValue string,
	messageContents ...string,
) *mcp.CallToolResult {
	t.Helper()

	handler := tool.Handler(deps)
	request := createMCPRequest(args)
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)
	params, ok := result.InputRequests[confirmationID].(*mcp.ElicitParams)
	require.True(t, ok)
	assert.Equal(t, "form", params.Mode)
	assert.Contains(t, params.Message, confirmationValue)
	for _, content := range messageContents {
		assert.Contains(t, params.Message, content)
	}

	retry := createMCPRequest(args)
	retry.Params.RequestState = result.RequestState
	retry.Params.InputResponses = mcp.InputResponseMap{
		confirmationID: &mcp.ElicitResult{
			Action: "accept",
			Content: map[string]any{
				agentAutomationConfirmationField: confirmationValue,
			},
		},
	}
	result, err = handler(ContextWithDeps(context.Background(), deps), &retry)
	require.NoError(t, err)
	return result
}
