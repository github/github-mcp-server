package github

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func securityOutputSchema(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	var schema *jsonschema.Schema
	var err error
	switch name {
	case "get_code_quality_finding":
		schema, err = jsonschema.For[*CodeQualityFindingOutput](nil)
	case "get_code_scanning_alert":
		schema, err = jsonschema.For[*CodeScanningAlertOutput](nil)
	case "list_code_scanning_alerts":
		schema, err = jsonschema.For[[]*CodeScanningAlertOutput](nil)
	case "get_secret_scanning_alert":
		schema, err = jsonschema.For[*SecretScanningAlertOutput](nil)
	case "list_secret_scanning_alerts":
		schema, err = jsonschema.For[[]*SecretScanningAlertOutput](nil)
	case "get_dependabot_alert":
		schema, err = jsonschema.For[*DependabotAlertOutput](nil)
	case "list_dependabot_alerts":
		schema, err = jsonschema.For[DependabotAlertsOutput](nil)
	case "get_global_security_advisory":
		schema, err = jsonschema.For[*GlobalSecurityAdvisoryOutput](nil)
	case "list_global_security_advisories":
		schema, err = jsonschema.For[[]*GlobalSecurityAdvisoryOutput](nil)
	case "list_repository_security_advisories", "list_org_repository_security_advisories":
		schema, err = jsonschema.For[[]*SecurityAdvisoryOutput](nil)
	default:
		t.Fatalf("unexpected security tool %s", name)
	}
	require.NoError(t, err)
	return schema
}

func testSecurityToolSnapshot(t *testing.T, tool mcp.Tool) {
	t.Helper()
	tool.OutputSchema = securityOutputSchema(t, tool.Name)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))
}

func TestTypedSecurityToolOutputs(t *testing.T) {
	codeQualityFinding := map[string]any{
		"number": 42,
		"state":  "open",
		"rule": map[string]any{
			"id":          "test-rule",
			"description": "Test rule",
		},
		"unknown_legacy_field": "preserved in text",
		"url":                  "https://api.github.com/repos/owner/repo/code-quality/findings/42",
	}
	codeScanningAlert := &github.Alert{
		Number:  new(42),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/repo/security/code-scanning/42"),
		URL:     new("https://api.github.com/repos/owner/repo/code-scanning/alerts/42"),
		Rule: &github.Rule{
			ID:                    new("test-rule"),
			Description:           new("Test rule"),
			FullDescription:       new("Full rule description"),
			Help:                  new("Use parameterized queries"),
			SecuritySeverityLevel: new("high"),
		},
	}
	secretScanningAlert := &github.SecretScanningAlert{
		Number:                 new(43),
		State:                  new("open"),
		HTMLURL:                new("https://github.com/owner/repo/security/secret-scanning/43"),
		SecretType:             new("test_secret"),
		Secret:                 new("test-secret"),
		URL:                    new("https://api.github.com/repos/owner/repo/secret-scanning/alerts/43"),
		Validity:               new("active"),
		ResolutionComment:      new("Rotate this credential"),
		PushProtectionBypassed: new(true),
		IsBase64Encoded:        new(true),
		HasMoreLocations:       new(true),
		FirstLocationDetected: &github.SecretScanningAlertLocationDetails{
			Path: new("config.env"), CommitSHA: new("abc123"), StartColumn: new(5),
			CommitURL: new("https://api.github.com/repos/owner/repo/commits/abc123"),
		},
		PushProtectionBypassRequestComment: new("legacy response detail"),
	}
	dependabotAlert := &github.DependabotAlert{
		Number:          new(44),
		State:           new("open"),
		HTMLURL:         new("https://github.com/owner/repo/security/dependabot/44"),
		URL:             new("https://api.github.com/repos/owner/repo/dependabot/alerts/44"),
		DismissedReason: new("tolerable_risk"),
		SecurityVulnerability: &github.AdvisoryVulnerability{
			VulnerableVersionRange: new("< 2.0"),
			FirstPatchedVersion:    &github.FirstPatchedVersion{Identifier: new("2.0")},
		},
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			GHSAID:          new("GHSA-aaaa-bbbb-cccc"),
			Classification:  new("malware"),
			Vulnerabilities: []*github.AdvisoryVulnerability{{VulnerableVersionRange: new("< 2.0")}},
		},
	}
	advisory := &github.SecurityAdvisory{
		GHSAID:      new("GHSA-aaaa-bbbb-cccc"),
		Summary:     new("Test advisory"),
		Description: new("Test advisory description"),
		Severity:    new("high"),
		State:       new("published"),
		URL:         new("https://api.github.com/repos/owner/repo/security-advisories/GHSA-aaaa-bbbb-cccc"),
		Vulnerabilities: []*github.AdvisoryVulnerability{{
			PatchedVersions: new("2.0"), VulnerableVersionRange: new("< 2.0"),
			VulnerableFunctions: []string{"unsafeQuery"},
		}},
	}
	globalAdvisory := &github.GlobalSecurityAdvisory{
		SecurityAdvisory:      *advisory,
		RepositoryAdvisoryURL: new("https://api.github.com/repos/owner/repo/security-advisories/GHSA-aaaa-bbbb-cccc"),
		Vulnerabilities: []*github.GlobalSecurityVulnerability{{
			FirstPatchedVersion: new("2.0"), VulnerableVersionRange: new("< 2.0"),
			VulnerableFunctions: []string{"unsafeQuery"},
		}},
	}
	globalAdvisory.CWEs = []*github.AdvisoryCWEs{{CWEID: new("CWE-79"), Name: new("Cross-site scripting")}}
	codeQualityJSON := json.RawMessage(`{"unknown_legacy_field":"preserved in text","url":"https://api.github.com/repos/owner/repo/code-quality/findings/42","state":"open","rule":{"id":"test-rule","description":"Test rule"},"number":42}`)
	legacyOutputs := map[string]any{
		"get_code_quality_finding":    codeQualityFinding,
		"get_code_scanning_alert":     codeScanningAlert,
		"list_code_scanning_alerts":   []*github.Alert{codeScanningAlert},
		"get_secret_scanning_alert":   secretScanningAlert,
		"list_secret_scanning_alerts": []*github.SecretScanningAlert{secretScanningAlert},
		"get_dependabot_alert":        dependabotAlert,
		"list_dependabot_alerts": struct {
			Alerts   []*github.DependabotAlert `json:"alerts"`
			PageInfo pageInfo                  `json:"pageInfo"`
		}{Alerts: []*github.DependabotAlert{dependabotAlert}},
		"get_global_security_advisory":            globalAdvisory,
		"list_global_security_advisories":         []*github.GlobalSecurityAdvisory{globalAdvisory},
		"list_repository_security_advisories":     []*github.SecurityAdvisory{advisory},
		"list_org_repository_security_advisories": []*github.SecurityAdvisory{advisory},
	}

	deps := BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber: mockResponse(t, http.StatusOK, codeQualityJSON),
			GetReposByOwnerByRepo:                                  mockResponse(t, http.StatusOK, &github.Repository{Private: new(true)}),
			GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber:   mockResponse(t, http.StatusOK, codeScanningAlert),
			GetReposCodeScanningAlertsByOwnerByRepo:                mockResponse(t, http.StatusOK, []*github.Alert{codeScanningAlert}),
			GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber: mockResponse(t, http.StatusOK, secretScanningAlert),
			GetReposSecretScanningAlertsByOwnerByRepo:              mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{secretScanningAlert}),
			GetReposDependabotAlertsByOwnerByRepoByAlertNumber:     mockResponse(t, http.StatusOK, dependabotAlert),
			GetReposDependabotAlertsByOwnerByRepo:                  mockResponse(t, http.StatusOK, []*github.DependabotAlert{dependabotAlert}),
			GetAdvisories:                                          mockResponse(t, http.StatusOK, []*github.GlobalSecurityAdvisory{globalAdvisory}),
			GetAdvisoriesByGhsaID:                                  mockResponse(t, http.StatusOK, globalAdvisory),
			GetReposSecurityAdvisoriesByOwnerByRepo:                mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{advisory}),
			GetOrgsSecurityAdvisoriesByOrg:                         mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{advisory}),
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
	legacyIFC := make(map[string]string)
	t.Run("unknown direct caller", func(t *testing.T) {
		for i, tool := range tools {
			call := calls[i]
			arguments, err := json.Marshal(call.Arguments)
			require.NoError(t, err)
			result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{Name: call.Name, Arguments: arguments},
			})
			require.NoError(t, err, call.Name)
			require.False(t, result.IsError, call.Name)
			assert.Nil(t, result.StructuredContent, "unknown callers must remain text-only")
			assert.Equal(t, mustMarshalJSON(t, legacyOutputs[call.Name]), getTextResult(t, result).Text, call.Name)
		}
	})

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
				schemaBytes, err := json.Marshal(tool.OutputSchema)
				require.NoError(t, err)
				t.Logf("%s output schema: %d bytes", tool.Name, len(schemaBytes))
				assert.JSONEq(t, mustMarshalJSON(t, securityOutputSchema(t, tool.Name)), string(schemaBytes), "canonical snapshots must describe the modern wire schema")
			}

			for _, call := range calls {
				result, err := session.CallTool(context.Background(), &call)
				require.NoError(t, err, call.Name)
				require.False(t, result.IsError, "%s: %s", call.Name, result)
				text := getTextResult(t, result).Text
				if protocolVersion == "2025-11-25" {
					assert.Equal(t, mustMarshalJSON(t, legacyOutputs[call.Name]), text, "%s must retain byte-exact main serialization for legacy clients", call.Name)
					legacyIFC[call.Name] = mustMarshalJSON(t, result.Meta["ifc"])
					assert.Nil(t, result.StructuredContent, "%s must remain text-only for legacy clients", call.Name)
					continue
				}

				require.NotNil(t, result.StructuredContent, "%s must return structured content", call.Name)
				assert.Equal(t, legacyIFC[call.Name], mustMarshalJSON(t, result.Meta["ifc"]), "IFC labels must protect text and structured content identically")
				if call.Name != "get_code_quality_finding" {
					assert.NotNil(t, result.Meta["ifc"], "%s must retain its IFC label", call.Name)
				}
				schemaJSON, err := json.Marshal(toolsByName[call.Name].OutputSchema)
				require.NoError(t, err)
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal(schemaJSON, &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				structuredJSON, err := json.Marshal(result.StructuredContent)
				require.NoError(t, err)
				assert.JSONEq(t, string(structuredJSON), text, "modern JSON text must expose the same compact DTO as structuredContent")
				var structured any
				require.NoError(t, json.Unmarshal(structuredJSON, &structured))
				require.NoError(t, resolved.Validate(structured), "%s output must conform to its schema", call.Name)
				assert.NotContains(t, string(structuredJSON), "https://api.github.com", "API/hypermedia URLs must remain text-only")
				switch call.Name {
				case "get_code_scanning_alert", "list_code_scanning_alerts":
					assert.Contains(t, string(structuredJSON), "Use parameterized queries")
					assert.Contains(t, string(structuredJSON), `"security_severity_level":"high"`)
				case "get_secret_scanning_alert", "list_secret_scanning_alerts":
					assert.Contains(t, string(structuredJSON), `"is_base64_encoded":true`)
					assert.Contains(t, string(structuredJSON), `"has_more_locations":true`)
					assert.Contains(t, text, `"secret":"test-secret"`)
					assert.Contains(t, string(structuredJSON), `"secret":"test-secret"`, "preserve the same intentionally scoped secret as text")
					assert.Contains(t, string(structuredJSON), `"validity":"active"`)
					assert.Contains(t, string(structuredJSON), "Rotate this credential")
					assert.Contains(t, string(structuredJSON), `"start_column":5`)
				case "get_dependabot_alert", "list_dependabot_alerts":
					assert.Contains(t, string(structuredJSON), `"classification":"malware"`)
					assert.Contains(t, string(structuredJSON), `"first_patched_version":"2.0"`)
					assert.Contains(t, string(structuredJSON), `"dismissed_reason":"tolerable_risk"`)
					assert.NotContains(t, string(structuredJSON), `"vulnerabilities"`, "use the alert's affected package instead of repeating advisory-wide packages")
				case "get_global_security_advisory", "list_global_security_advisories":
					assert.Contains(t, string(structuredJSON), `"cwe_ids":["CWE-79"]`)
					assert.Contains(t, string(structuredJSON), `"first_patched_version":"2.0"`)
					assert.Contains(t, string(structuredJSON), "unsafeQuery")
				case "list_repository_security_advisories", "list_org_repository_security_advisories":
					assert.Contains(t, string(structuredJSON), `"patched_versions":"2.0"`)
					assert.Contains(t, string(structuredJSON), "unsafeQuery")
				}

				if call.Name == "get_code_quality_finding" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "unknown_legacy_field")
				}

				if call.Name == "get_code_scanning_alert" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "full_description")
				}
				if call.Name == "get_secret_scanning_alert" {
					assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "legacy response detail")
				}
				if call.Name == "get_code_scanning_alert" {
					assert.NotNil(t, result.Meta["ifc"], "typed output must retain the security alert IFC label")
				}
			}
		})
	}
}

func TestGlobalSecurityAdvisoryCWEIDs(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cweIDs   []string
		cwes     []*github.AdvisoryCWEs
		expected []string
	}{
		{name: "absent"},
		{name: "global CWEs", cwes: []*github.AdvisoryCWEs{{CWEID: new("CWE-79")}, {CWEID: new("CWE-89")}}, expected: []string{"CWE-79", "CWE-89"}},
		{name: "missing CWE ID", cwes: []*github.AdvisoryCWEs{nil, {Name: new("No ID")}, {CWEID: new("CWE-79")}}, expected: []string{"CWE-79"}},
		{name: "explicit IDs", cweIDs: []string{"CWE-89"}, cwes: []*github.AdvisoryCWEs{{CWEID: new("CWE-79")}}, expected: []string{"CWE-89"}},
		{name: "empty IDs", cweIDs: []string{}, cwes: []*github.AdvisoryCWEs{{CWEID: new("CWE-79")}}, expected: []string{"CWE-79"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			advisory := github.GlobalSecurityAdvisory{
				SecurityAdvisory: github.SecurityAdvisory{CWEIDs: tc.cweIDs, CWEs: tc.cwes},
			}
			assert.Equal(t, tc.expected, globalSecurityAdvisoryOutput(&advisory).CWEIDs)
			assert.Equal(t, tc.cweIDs, advisory.CWEIDs, "projection must not mutate the API model")
		})
	}
}

func TestSecretScanningOutputQualifiers(t *testing.T) {
	for _, tc := range []struct {
		name     string
		value    *bool
		expected string
	}{
		{"absent", nil, `{}`},
		{"false", new(false), `{"is_base64_encoded":false,"has_more_locations":false}`},
		{"true", new(true), `{"is_base64_encoded":true,"has_more_locations":true}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := secretScanningAlertOutput(&github.SecretScanningAlert{
				IsBase64Encoded: tc.value, HasMoreLocations: tc.value,
			})
			assert.JSONEq(t, tc.expected, mustMarshalJSON(t, output))
			resolved, err := securityOutputSchema(t, "get_secret_scanning_alert").Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(tc.expected), &value))
			require.NoError(t, resolved.Validate(value))
		})
	}
}

func TestSecurityOutputDTOs(t *testing.T) {
	seen := make(map[reflect.Type]bool)
	var check func(reflect.Type)
	check = func(typ reflect.Type) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		assert.NotEqual(t, "github.com/google/go-github/v92/github", typ.PkgPath(), "%s must not embed raw API models", typ)
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			check(typ.Elem())
		case reflect.Struct:
			for field := range typ.Fields() {
				if field.IsExported() {
					check(field.Type)
				}
			}
		case reflect.Map, reflect.Interface:
			t.Errorf("%s must not use open-ended maps or interfaces", typ)
		}
	}
	for _, typ := range []reflect.Type{
		reflect.TypeFor[CodeQualityFindingOutput](),
		reflect.TypeFor[CodeScanningAlertOutput](),
		reflect.TypeFor[SecretScanningAlertOutput](),
		reflect.TypeFor[DependabotAlertsOutput](),
		reflect.TypeFor[GlobalSecurityAdvisoryOutput](),
	} {
		check(typ)
	}
}
