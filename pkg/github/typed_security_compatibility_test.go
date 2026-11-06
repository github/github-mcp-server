package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityRegisteredFilters(t *testing.T) {
	tests := []struct {
		name      string
		tool      inventory.ServerTool
		route     string
		arguments map[string]any
		wantQuery url.Values
	}{
		{
			name: "dependabot omitted state",
			tool: ListDependabotAlerts(translations.NullTranslationHelper), route: GetReposDependabotAlertsByOwnerByRepo,
			arguments: map[string]any{"owner": "owner", "repo": "repo"},
			wantQuery: url.Values{"per_page": {"30"}},
		},
		{
			name: "dependabot explicit state",
			tool: ListDependabotAlerts(translations.NullTranslationHelper), route: GetReposDependabotAlertsByOwnerByRepo,
			arguments: map[string]any{"owner": "owner", "repo": "repo", "state": "fixed"},
			wantQuery: url.Values{"per_page": {"30"}, "state": {"fixed"}},
		},
		{
			name:      "dependabot omitted state with harness page size",
			tool:      ListDependabotAlerts(translations.NullTranslationHelper),
			route:     GetReposDependabotAlertsByOwnerByRepo,
			arguments: map[string]any{"owner": "owner", "repo": "repo", "perPage": 2},
			wantQuery: url.Values{"per_page": {"2"}},
		},
		{
			name: "code scanning omitted state",
			tool: ListCodeScanningAlerts(translations.NullTranslationHelper), route: GetReposCodeScanningAlertsByOwnerByRepo,
			arguments: map[string]any{"owner": "owner", "repo": "repo"},
			wantQuery: url.Values{"page": {"1"}, "per_page": {"30"}},
		},
		{
			name: "advisories omitted type",
			tool: ListGlobalSecurityAdvisories(translations.NullTranslationHelper), route: GetAdvisories,
			arguments: map[string]any{},
			wantQuery: url.Values{},
		},
		{
			name: "advisories null cwes",
			tool: ListGlobalSecurityAdvisories(translations.NullTranslationHelper), route: GetAdvisories,
			arguments: map[string]any{"type": "reviewed", "cwes": nil},
			wantQuery: url.Values{"type": {"reviewed"}},
		},
		{
			name: "advisories explicit cwes",
			tool: ListGlobalSecurityAdvisories(translations.NullTranslationHelper), route: GetAdvisories,
			arguments: map[string]any{"type": "reviewed", "cwes": []string{"79", "284"}},
			wantQuery: url.Values{"type": {"reviewed"}, "cwes": {"79", "284"}},
		},
	}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tt := range tests {
			t.Run(protocol+"/"+tt.name, func(t *testing.T) {
				called := false
				deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					tt.route: func(w http.ResponseWriter, r *http.Request) {
						called = true
						assert.Equal(t, tt.wantQuery, r.URL.Query(), "SDK validation must not insert advertised defaults into omitted filters")
						mockResponse(t, http.StatusOK, []any{})(w, r)
					},
				}))}
				server := mcp.NewServer(&mcp.Implementation{Name: "security-filters-test", Version: "v1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				tt.tool.RegisterFunc(server, deps)
				session := connectCommentVisibilityClient(t, server, protocol)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool.Tool.Name, Arguments: tt.arguments})
				require.NoError(t, err)
				require.False(t, result.IsError, "%s", getTextResult(t, result).Text)
				assert.True(t, called, "the API must receive the compatible filter")
			})
		}
	}
}

func TestSecurityEmptyAndErrorOutputs(t *testing.T) {
	tests := []struct {
		tool      inventory.ServerTool
		route     string
		arguments map[string]any
		list      bool
	}{
		{GetCodeQualityFinding(translations.NullTranslationHelper), GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber, map[string]any{"owner": "owner", "repo": "repo", "findingNumber": 42}, false},
		{GetCodeScanningAlert(translations.NullTranslationHelper), GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber, map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 42}, false},
		{ListCodeScanningAlerts(translations.NullTranslationHelper), GetReposCodeScanningAlertsByOwnerByRepo, map[string]any{"owner": "owner", "repo": "repo"}, true},
		{GetSecretScanningAlert(translations.NullTranslationHelper), GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber, map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 42}, false},
		{ListSecretScanningAlerts(translations.NullTranslationHelper), GetReposSecretScanningAlertsByOwnerByRepo, map[string]any{"owner": "owner", "repo": "repo"}, true},
		{GetDependabotAlert(translations.NullTranslationHelper), GetReposDependabotAlertsByOwnerByRepoByAlertNumber, map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 42}, false},
		{ListDependabotAlerts(translations.NullTranslationHelper), GetReposDependabotAlertsByOwnerByRepo, map[string]any{"owner": "owner", "repo": "repo"}, true},
		{GetGlobalSecurityAdvisory(translations.NullTranslationHelper), GetAdvisoriesByGhsaID, map[string]any{"ghsaId": "GHSA-aaaa-bbbb-cccc"}, false},
		{ListGlobalSecurityAdvisories(translations.NullTranslationHelper), GetAdvisories, map[string]any{}, true},
		{ListRepositorySecurityAdvisories(translations.NullTranslationHelper), GetReposSecurityAdvisoriesByOwnerByRepo, map[string]any{"owner": "owner", "repo": "repo"}, true},
		{ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper), GetOrgsSecurityAdvisoriesByOrg, map[string]any{"org": "owner"}, true},
	}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tt := range tests {
			for _, body := range []string{"null", "[]", "[null]", `{"message":"access denied"}`} {
				isError := body == `{"message":"access denied"}`
				if !tt.list && !isError && body != "null" {
					continue
				}
				t.Run(protocol+"/"+tt.tool.Tool.Name+"/"+body, func(t *testing.T) {
					status := http.StatusOK
					if isError {
						status = http.StatusForbidden
					}
					deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
						tt.route: mockResponse(t, status, json.RawMessage(body)),
					}))}
					server := mcp.NewServer(&mcp.Implementation{Name: "security-empty-test", Version: "v1"}, nil)
					var serializedResult json.RawMessage
					server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
						return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
							result, err := next(ctx, method, request)
							if method == "tools/call" && err == nil {
								serializedResult, err = json.Marshal(result)
							}
							return result, err
						}
					})
					server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
					tt.tool.RegisterFunc(server, deps)
					session := connectCommentVisibilityClient(t, server, protocol)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tt.tool.Tool.Name, Arguments: tt.arguments})
					require.NoError(t, err)
					require.Equal(t, isError, result.IsError, "%s", getTextResult(t, result).Text)
					if isError {
						assert.Nil(t, result.StructuredContent, "errors must not produce success-shaped structured output")
						assert.Contains(t, getTextResult(t, result).Text, "access denied")
						return
					}
					if protocol == "2025-11-25" {
						if tt.tool.Tool.Name == "list_dependabot_alerts" {
							assert.JSONEq(t, `{"alerts":`+body+`,"pageInfo":{"hasNextPage":false,"hasPreviousPage":false}}`, getTextResult(t, result).Text)
						} else {
							assert.Equal(t, body, getTextResult(t, result).Text)
						}
						assert.Nil(t, result.StructuredContent)
						return
					}
					schema, err := securityOutputSchema(t, tt.tool.Tool.Name).Resolve(nil)
					require.NoError(t, err)
					var wireFields map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(serializedResult, &wireFields))
					wireStructured, present := wireFields["structuredContent"]
					require.True(t, present, "successful schema-bearing tools must serialize structuredContent, including explicit JSON null")
					var structured any
					require.NoError(t, json.Unmarshal(wireStructured, &structured))
					require.NoError(t, schema.Validate(structured))
					if !tt.list {
						expected := "null"
						if tt.tool.Tool.Name == "get_code_quality_finding" {
							expected = "{}"
						}
						assert.JSONEq(t, expected, string(wireStructured))
						assert.JSONEq(t, string(wireStructured), getTextResult(t, result).Text)
					}
					if tt.list {
						assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text, "modern lists expose the same DTO through both channels")
						if tt.tool.Tool.Name == "list_dependabot_alerts" {
							assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
						} else {
							assert.Equal(t, body, mustMarshalJSON(t, result.StructuredContent))
						}
					}
				})
			}
		}
	}
}

func TestLegacyCodeQualityFindingFormatting(t *testing.T) {
	raw := json.RawMessage(`{"z":1.0,"number":42,"rule":{"z":"last","a":"first"},"unknown":{"n":1e3},"a":null}`)
	formatted, err := marshalLegacyCodeQualityFinding(raw)
	require.NoError(t, err)
	assert.Equal(t, `{"a":null,"number":42,"rule":{"a":"first","z":"last"},"unknown":{"n":1000},"z":1}`, string(formatted))
}

func TestSecurityInputSchemasPreserveEnums(t *testing.T) {
	for _, tool := range []inventory.ServerTool{
		ListCodeScanningAlerts(translations.NullTranslationHelper),
		ListDependabotAlerts(translations.NullTranslationHelper),
		ListGlobalSecurityAdvisories(translations.NullTranslationHelper),
	} {
		schema := tool.Tool.InputSchema.(*jsonschema.Schema)
		property := "state"
		expectedDefault := `"open"`
		if tool.Tool.Name == "list_global_security_advisories" {
			property, expectedDefault = "type", `"reviewed"`
		}
		require.NotEmpty(t, schema.Properties[property].Enum)
		assert.Equal(t, expectedDefault, string(schema.Properties[property].Default))
		assert.NotEmpty(t, schema.Properties[property].Description)
	}
}
