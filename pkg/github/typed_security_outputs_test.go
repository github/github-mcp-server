package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedSecurityToolOutputs(t *testing.T) {
	codeQualityFinding := map[string]any{
		"number": 42,
		"state":  "open",
		"rule": map[string]any{
			"id":          "test-rule",
			"description": "Test rule",
		},
		"unknown_legacy_field": "preserved in text",
	}
	codeScanningAlert := &github.Alert{
		Number:  new(42),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/security/code-scanning/42"),
		Rule: &github.Rule{
			ID:              new("test-rule"),
			Description:     new("Test rule"),
			FullDescription: new("Full rule description"),
		},
	}
	secretScanningAlert := &github.SecretScanningAlert{
		Number:                             new(43),
		State:                              new("open"),
		HTMLURL:                            new("https://github.com/owner/repo/security/secret-scanning/43"),
		SecretType:                         new("test_secret"),
		PushProtectionBypassRequestComment: new("legacy response detail"),
	}
	dependabotAlert := &github.DependabotAlert{
		Number:  new(44),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/security/dependabot/44"),
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			GHSAID:         new("GHSA-aaaa-bbbb-cccc"),
			Classification: new("malware"),
		},
	}
	advisory := &github.SecurityAdvisory{
		GHSAID:      new("GHSA-aaaa-bbbb-cccc"),
		Summary:     new("Test advisory"),
		Description: new("Test advisory description"),
		Severity:    new("high"),
		State:       new("published"),
	}
	globalAdvisory := &github.GlobalSecurityAdvisory{
		SecurityAdvisory: *advisory,
	}

	deps := BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber: mockResponse(t, http.StatusOK, codeQualityFinding),
			GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber:    mockResponse(t, http.StatusOK, codeScanningAlert),
			GetReposCodeScanningAlertsByOwnerByRepo:                 mockResponse(t, http.StatusOK, []*github.Alert{codeScanningAlert}),
			GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber:  mockResponse(t, http.StatusOK, secretScanningAlert),
			GetReposSecretScanningAlertsByOwnerByRepo:               mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{secretScanningAlert}),
			GetReposDependabotAlertsByOwnerByRepoByAlertNumber:      mockResponse(t, http.StatusOK, dependabotAlert),
			GetReposDependabotAlertsByOwnerByRepo:                   mockResponse(t, http.StatusOK, []*github.DependabotAlert{dependabotAlert}),
			GetAdvisories:                                           mockResponse(t, http.StatusOK, []*github.GlobalSecurityAdvisory{globalAdvisory}),
			GetAdvisoriesByGhsaID:                                   mockResponse(t, http.StatusOK, globalAdvisory),
			GetReposSecurityAdvisoriesByOwnerByRepo:                 mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{advisory}),
			GetOrgsSecurityAdvisoriesByOrg:                          mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{advisory}),
		})),
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
	}
	tools := []inventory.ServerTool{
		GetCodeQualityFinding(translations.NullTranslationHelper),
		GetCodeScanningAlert(translations.NullTranslationHelper),
		ListCodeScanningAlerts(translations.NullTranslationHelper),
		GetSecretScanningAlert(translations.NullTranslationHelper),
		ListSecretScanningAlerts(translations.NullTranslationHelper),
		GetDependabotAlert(translations.NullTranslationHelper),
		ListDependabotAlerts(translations.NullTranslationHelper),
		ListGlobalSecurityAdvisories(translations.NullTranslationHelper),
		GetGlobalSecurityAdvisory(translations.NullTranslationHelper),
		ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
		ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
	}
	calls := []mcp.CallToolParams{
		{Name: "get_code_quality_finding", Arguments: map[string]any{"owner": "owner", "repo": "repo", "findingNumber": "42"}},
		{Name: "get_code_scanning_alert", Arguments: map[string]any{"owner": "owner", "repo": "repo", "alertNumber": "42"}},
		{Name: "list_code_scanning_alerts", Arguments: map[string]any{"owner": "owner", "repo": "repo", "page": "1", "perPage": "30"}},
		{Name: "get_secret_scanning_alert", Arguments: map[string]any{"owner": "owner", "repo": "repo", "alertNumber": "43"}},
		{Name: "list_secret_scanning_alerts", Arguments: map[string]any{"owner": "owner", "repo": "repo", "page": "1", "perPage": "30"}},
		{Name: "get_dependabot_alert", Arguments: map[string]any{"owner": "owner", "repo": "repo", "alertNumber": "44"}},
		{Name: "list_dependabot_alerts", Arguments: map[string]any{"owner": "owner", "repo": "repo", "perPage": "30"}},
		{Name: "list_global_security_advisories", Arguments: map[string]any{"type": "reviewed", "cwes": []string{"79"}, "unknown_legacy_field": true}},
		{Name: "get_global_security_advisory", Arguments: map[string]any{"ghsaId": "GHSA-aaaa-bbbb-cccc"}},
		{Name: "list_repository_security_advisories", Arguments: map[string]any{"owner": "owner", "repo": "repo"}},
		{Name: "list_org_repository_security_advisories", Arguments: map[string]any{"org": "owner"}},
	}

	for _, protocolVersion := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocolVersion, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-security-output-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range tools {
				tool.RegisterFunc(server, deps)
			}

			session := connectCommentVisibilityClient(t, server, protocolVersion)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, len(tools))
			toolsByName := make(map[string]*mcp.Tool, len(list.Tools))
			for _, tool := range list.Tools {
				toolsByName[tool.Name] = tool
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, tool.OutputSchema, "legacy clients must not see outputSchema for %s", tool.Name)
					continue
				}
				require.NotNil(t, tool.OutputSchema, "%s must publish an output schema", tool.Name)
			}

			for _, call := range calls {
				result, err := session.CallTool(context.Background(), &call)
				require.NoError(t, err, call.Name)
				require.False(t, result.IsError, "%s: %s", call.Name, result)
				text := getTextResult(t, result).Text
				switch call.Name {
				case "get_code_quality_finding":
					assert.Contains(t, text, "unknown_legacy_field", "the legacy text response must preserve unknown API fields")
				case "get_code_scanning_alert":
					assert.Contains(t, text, "full_description", "the legacy text response must retain full API details")
				case "get_secret_scanning_alert":
					assert.Contains(t, text, "legacy response detail", "the legacy text response must retain full API details")
				case "list_dependabot_alerts":
					assert.Contains(t, text, "classification", "the legacy text response must retain full API details")
				}
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, result.StructuredContent, "%s must remain text-only for legacy clients", call.Name)
					continue
				}

				require.NotNil(t, result.StructuredContent, "%s must return structured content", call.Name)
				schemaJSON, err := json.Marshal(toolsByName[call.Name].OutputSchema)
				require.NoError(t, err)
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal(schemaJSON, &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				structuredJSON, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				var structured any
				require.NoError(t, json.Unmarshal(structuredJSON, &structured))
				require.NoError(t, resolved.Validate(structured), "%s output must conform to its schema", call.Name)

				if call.Name == "get_code_quality_finding" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "unknown_legacy_field")
				}
				if call.Name == "get_code_scanning_alert" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "full_description")
				}
				if call.Name == "get_secret_scanning_alert" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "legacy response detail")
				}
				if call.Name == "list_dependabot_alerts" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "classification")
				}
				if call.Name == "get_code_scanning_alert" {
					assert.NotNil(t, result.Meta["ifc"], "typed output must retain the security alert IFC label")
				}
			}
		})
	}
}
