package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentAutomationReadToolSnapshots(t *testing.T) {
	t.Parallel()

	for _, tool := range []inventory.ServerTool{
		ListAutomations(translations.NullTranslationHelper),
		GetAutomation(translations.NullTranslationHelper),
		ListAutomationRevisions(translations.NullTranslationHelper),
		GetAutomationRevision(translations.NullTranslationHelper),
		ListAvailableAutomationTools(translations.NullTranslationHelper),
		ListAvailableAutomationTriggers(translations.NullTranslationHelper),
	} {
		require.NoError(t, toolsnaps.Test(tool.Tool.Name, tool.Tool))
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
