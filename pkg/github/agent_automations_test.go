package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

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
		wantAvailable   bool
	}{
		{name: "default with all toolsets"},
		{name: "insiders without explicit flag", insidersMode: true},
		{
			name:            "explicit opt-in",
			enabledFeatures: []string{FeatureFlagAgentAutomations},
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
				assert.ElementsMatch(t, automationNames, availableNames)
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
