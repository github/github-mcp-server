package github

import (
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type securityOutputContractCase struct {
	name     string
	tool     inventory.ServerTool
	endpoint string
	args     map[string]any
	response any
	expected any
}

func TestSecurityOutputsMatchDeclaredSchemas(t *testing.T) {
	codeQuality := codeQualityOutputFixture()
	codeScanning := codeScanningOutputFixture()
	dependabot := dependabotOutputFixture()
	secretScanning := secretScanningOutputFixture()
	advisory := repositoryAdvisoryOutputFixture()
	codeQualityResponse := withSecurityOutputFields(codeQuality, map[string]any{
		"repository": map[string]any{"id": 1.0, "full_name": "owner/repo"},
	})
	codeScanningResponse := withSecurityOutputFields(codeScanning, map[string]any{
		"repository": map[string]any{"id": 1.0, "full_name": "owner/repo"},
	})
	dependabotResponse := withSecurityOutputFields(dependabot, map[string]any{
		"repository": map[string]any{"id": 1.0, "full_name": "owner/repo"},
	})
	secretScanningResponse := withSecurityOutputFields(secretScanning, map[string]any{
		"repository": map[string]any{"id": 1.0, "full_name": "owner/repo"},
	})
	advisoryResponse := withSecurityOutputFields(advisory, map[string]any{
		"repository":          map[string]any{"id": 1.0, "full_name": "owner/repo"},
		"collaborating_users": []any{},
		"collaborating_teams": []any{},
	})

	tests := []securityOutputContractCase{
		{
			name:     "get_code_quality_finding",
			tool:     GetCodeQualityFinding(translations.NullTranslationHelper),
			endpoint: GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "findingNumber": 1},
			response: codeQualityResponse,
			expected: codeQuality,
		},
		{
			name:     "get_code_scanning_alert",
			tool:     GetCodeScanningAlert(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
			response: codeScanningResponse,
			expected: codeScanning,
		},
		{
			name:     "get_dependabot_alert",
			tool:     GetDependabotAlert(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
			response: dependabotResponse,
			expected: dependabot,
		},
		{
			name:     "get_secret_scanning_alert",
			tool:     GetSecretScanningAlert(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
			response: secretScanningResponse,
			expected: secretScanning,
		},
		{
			name:     "list_code_scanning_alerts",
			tool:     ListCodeScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{codeScanningResponse},
			expected: []any{codeScanning},
		},
		{
			name:     "list_dependabot_alerts",
			tool:     ListDependabotAlerts(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{dependabotResponse},
			expected: map[string]any{
				"alerts": []any{dependabot},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
				},
			},
		},
		{
			name:     "list_secret_scanning_alerts",
			tool:     ListSecretScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{secretScanningResponse},
			expected: []any{secretScanning},
		},
		{
			name:     "list_repository_security_advisories",
			tool:     ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetReposSecurityAdvisoriesByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{advisoryResponse},
			expected: []any{advisory},
		},
		{
			name:     "list_org_repository_security_advisories",
			tool:     ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetOrgsSecurityAdvisoriesByOrg,
			args:     map[string]any{"org": "org"},
			response: []any{advisoryResponse},
			expected: []any{advisory},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := securityOutputDeps(t, tc.endpoint, mockResponse(t, http.StatusOK, tc.response))
			result := callRegisteredTool(t, tc.tool, deps, tc.args)

			requireStructuredJSONAgreement(t, tc.tool, result)
			assert.Equal(t, tc.expected, result.StructuredContent)
		})
	}
}

func TestSecurityListOutputsValidateEmptyResults(t *testing.T) {
	tests := []securityOutputContractCase{
		{
			name:     "list_code_scanning_alerts",
			tool:     ListCodeScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{},
			expected: []any{},
		},
		{
			name:     "list_dependabot_alerts",
			tool:     ListDependabotAlerts(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{},
			expected: map[string]any{
				"alerts": []any{},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
				},
			},
		},
		{
			name:     "list_secret_scanning_alerts",
			tool:     ListSecretScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{},
			expected: []any{},
		},
		{
			name:     "list_repository_security_advisories",
			tool:     ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetReposSecurityAdvisoriesByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{},
			expected: []any{},
		},
		{
			name:     "list_org_repository_security_advisories",
			tool:     ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetOrgsSecurityAdvisoriesByOrg,
			args:     map[string]any{"org": "org"},
			response: []any{},
			expected: []any{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := securityOutputDeps(t, tc.endpoint, mockResponse(t, http.StatusOK, tc.response))
			result := callRegisteredTool(t, tc.tool, deps, tc.args)

			requireStructuredJSONAgreement(t, tc.tool, result)
			assert.Equal(t, tc.expected, result.StructuredContent)
		})
	}
}

func TestSecurityOutputsPreserveNullability(t *testing.T) {
	t.Run("code quality known fields", func(t *testing.T) {
		tool := GetCodeQualityFinding(translations.NullTranslationHelper)
		response := map[string]any{
			"number": 0.0, "state": nil, "url": "",
			"rule":       map[string]any{"id": nil, "title": "", "internal": "omit"},
			"location":   nil,
			"message":    map[string]any{"text": nil},
			"created_at": nil,
			"repository": map[string]any{"id": 1.0},
		}
		expected := map[string]any{
			"number": 0.0, "state": nil, "url": "",
			"rule":       map[string]any{"id": nil, "title": ""},
			"location":   nil,
			"message":    map[string]any{"text": nil},
			"created_at": nil,
		}
		deps := securityOutputDeps(t, GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber,
			mockResponse(t, http.StatusOK, response))
		result := callRegisteredTool(t, tool, deps,
			map[string]any{"owner": "owner", "repo": "repo", "findingNumber": 1})

		requireStructuredJSONAgreement(t, tool, result)
		assert.Equal(t, expected, result.StructuredContent)
	})

	tests := []securityOutputContractCase{
		{
			name:     "list_code_scanning_alerts",
			tool:     ListCodeScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{nil},
			expected: []any{nil},
		},
		{
			name:     "list_dependabot_alerts",
			tool:     ListDependabotAlerts(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{nil},
			expected: map[string]any{
				"alerts": []any{nil},
				"pageInfo": map[string]any{
					"hasNextPage":     false,
					"hasPreviousPage": false,
				},
			},
		},
		{
			name:     "list_secret_scanning_alerts",
			tool:     ListSecretScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{nil},
			expected: []any{nil},
		},
		{
			name:     "list_repository_security_advisories",
			tool:     ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetReposSecurityAdvisoriesByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
			response: []any{nil},
			expected: []any{nil},
		},
		{
			name:     "list_org_repository_security_advisories",
			tool:     ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetOrgsSecurityAdvisoriesByOrg,
			args:     map[string]any{"org": "org"},
			response: []any{nil},
			expected: []any{nil},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := securityOutputDeps(t, tc.endpoint, mockResponse(t, http.StatusOK, tc.response))
			result := callRegisteredTool(t, tc.tool, deps, tc.args)

			requireStructuredJSONAgreement(t, tc.tool, result)
			assert.Equal(t, tc.expected, result.StructuredContent)
		})
	}
}

func TestSecurityListOutputsPreserveFiltersAndPagination(t *testing.T) {
	tests := []struct {
		name     string
		tool     inventory.ServerTool
		endpoint string
		args     map[string]any
		handler  http.HandlerFunc
		check    func(*testing.T, any)
	}{
		{
			name:     "code scanning",
			tool:     ListCodeScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepo,
			args: map[string]any{
				"owner": "owner", "repo": "repo", "ref": "refs/heads/main",
				"state": "open", "severity": "high", "tool_name": "CodeQL",
				"page": 2, "perPage": 50,
			},
			handler: expectQueryParams(t, map[string]string{
				"ref": "refs/heads/main", "state": "open", "severity": "high",
				"tool_name": "CodeQL", "page": "2", "per_page": "50",
			}).andThen(mockResponse(t, http.StatusOK, []any{})),
		},
		{
			name:     "dependabot",
			tool:     ListDependabotAlerts(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepo,
			args: map[string]any{
				"owner": "owner", "repo": "repo", "state": "open",
				"severity": "critical", "after": "current", "perPage": 100,
			},
			handler: expectQueryParams(t, map[string]string{
				"state": "open", "severity": "critical", "after": "current", "per_page": "100",
			}).andThen(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/dependabot/alerts?after=next&per_page=100>; rel="next"`)
				mockResponse(t, http.StatusOK, []any{})(w, r)
			}),
			check: func(t *testing.T, structured any) {
				output := structured.(map[string]any)
				assert.Equal(t, map[string]any{
					"hasNextPage":     true,
					"hasPreviousPage": false,
					"nextCursor":      "next",
				}, output["pageInfo"])
			},
		},
		{
			name:     "secret scanning",
			tool:     ListSecretScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepo,
			args: map[string]any{
				"owner": "owner", "repo": "repo", "state": "resolved",
				"secret_type": "github_token", "resolution": "revoked",
				"page": 3, "perPage": 25,
			},
			handler: expectQueryParams(t, map[string]string{
				"state": "resolved", "secret_type": "github_token", "resolution": "revoked",
				"page": "3", "per_page": "25",
			}).andThen(mockResponse(t, http.StatusOK, []any{})),
		},
		{
			name:     "repository advisories",
			tool:     ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetReposSecurityAdvisoriesByOwnerByRepo,
			args: map[string]any{
				"owner": "owner", "repo": "repo", "direction": "asc",
				"sort": "published", "state": "draft",
			},
			handler: expectQueryParams(t, map[string]string{
				"direction": "asc", "sort": "published", "state": "draft",
			}).andThen(mockResponse(t, http.StatusOK, []any{})),
		},
		{
			name:     "organization advisories",
			tool:     ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetOrgsSecurityAdvisoriesByOrg,
			args: map[string]any{
				"org": "org", "direction": "desc", "sort": "updated", "state": "published",
			},
			handler: expectQueryParams(t, map[string]string{
				"direction": "desc", "sort": "updated", "state": "published",
			}).andThen(mockResponse(t, http.StatusOK, []any{})),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := securityOutputDeps(t, tc.endpoint, tc.handler)
			result := callRegisteredTool(t, tc.tool, deps, tc.args)

			requireStructuredJSONAgreement(t, tc.tool, result)
			if tc.check != nil {
				tc.check(t, result.StructuredContent)
			}
		})
	}
}

func TestSecurityOutputErrorsHaveNoStructuredContent(t *testing.T) {
	tests := []struct {
		name     string
		tool     inventory.ServerTool
		endpoint string
		args     map[string]any
	}{
		{
			name: "get_code_quality_finding", tool: GetCodeQualityFinding(translations.NullTranslationHelper),
			endpoint: GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "findingNumber": 1},
		},
		{
			name: "get_code_scanning_alert", tool: GetCodeScanningAlert(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
		},
		{
			name: "get_dependabot_alert", tool: GetDependabotAlert(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
		},
		{
			name: "get_secret_scanning_alert", tool: GetSecretScanningAlert(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber,
			args:     map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 1},
		},
		{
			name: "list_code_scanning_alerts", tool: ListCodeScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposCodeScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "list_dependabot_alerts", tool: ListDependabotAlerts(translations.NullTranslationHelper),
			endpoint: GetReposDependabotAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "list_secret_scanning_alerts", tool: ListSecretScanningAlerts(translations.NullTranslationHelper),
			endpoint: GetReposSecretScanningAlertsByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "list_repository_security_advisories", tool: ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetReposSecurityAdvisoriesByOwnerByRepo,
			args:     map[string]any{"owner": "owner", "repo": "repo"},
		},
		{
			name: "list_org_repository_security_advisories", tool: ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
			endpoint: GetOrgsSecurityAdvisoriesByOrg,
			args:     map[string]any{"org": "org"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := securityOutputDeps(t, tc.endpoint,
				mockResponse(t, http.StatusInternalServerError, map[string]any{"message": "boom"}))
			result := callRegisteredTool(t, tc.tool, deps, tc.args)

			require.True(t, result.IsError)
			assert.Nil(t, result.StructuredContent)
		})
	}
}

func TestSecurityListOutputsAreNotCSVEligible(t *testing.T) {
	tools := []inventory.ServerTool{
		ListCodeScanningAlerts(translations.NullTranslationHelper),
		ListDependabotAlerts(translations.NullTranslationHelper),
		ListSecretScanningAlerts(translations.NullTranslationHelper),
		ListRepositorySecurityAdvisories(translations.NullTranslationHelper),
		ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper),
	}
	for _, tool := range tools {
		t.Run(tool.Tool.Name, func(t *testing.T) {
			assert.False(t, isCSVOutputTool(tool))
		})
	}
}

func securityOutputDeps(t *testing.T, endpoint string, handler http.HandlerFunc) ToolDependencies {
	t.Helper()
	return BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			endpoint: handler,
		})),
		Obsv: stubExporters(),
	}
}

func withSecurityOutputFields(value map[string]any, fields map[string]any) map[string]any {
	result := maps.Clone(value)
	maps.Copy(result, fields)
	return result
}

func codeQualityOutputFixture() map[string]any {
	return map[string]any{
		"number": 0.0, "state": "", "url": "",
		"rule": map[string]any{
			"id": "", "title": "", "description": "", "help": "", "severity": "", "category": "",
		},
		"location": map[string]any{
			"path": "", "start_line": 0.0, "start_column": 0.0, "end_line": 0.0, "end_column": 0.0,
		},
		"message":    map[string]any{"text": "", "markdown": ""},
		"created_at": "2025-01-02T03:04:05Z",
	}
}

func codeScanningOutputFixture() map[string]any {
	return map[string]any{
		"number": 0.0, "rule_id": "", "rule_severity": "", "rule_description": "",
		"rule": map[string]any{
			"id": "", "severity": "", "description": "", "name": "",
			"security_severity_level": "", "full_description": "", "tags": []any{}, "help": "",
		},
		"tool":       map[string]any{"name": "", "guid": "", "version": ""},
		"created_at": "2025-01-02T03:04:05Z", "updated_at": "2025-01-02T03:04:05Z",
		"fixed_at": "2025-01-02T03:04:05Z", "state": "",
		"closed_by": map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"closed_at": "2025-01-02T03:04:05Z", "url": "", "html_url": "",
		"most_recent_instance": map[string]any{
			"ref": "", "analysis_key": "", "category": "", "environment": "", "state": "",
			"commit_sha": "", "message": map[string]any{"text": ""},
			"location": map[string]any{
				"path": "", "start_line": 0.0, "start_column": 0.0, "end_line": 0.0, "end_column": 0.0,
			},
			"html_url": "", "classifications": []any{},
		},
		"instances":    []any{},
		"dismissed_by": map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"dismissed_at": "2025-01-02T03:04:05Z", "dismissed_reason": "", "dismissed_comment": "",
		"instances_url": "",
	}
}

func dependabotOutputFixture() map[string]any {
	vulnerability := map[string]any{
		"package":  map[string]any{"ecosystem": "", "name": ""},
		"severity": "", "vulnerable_version_range": "",
		"first_patched_version": map[string]any{"identifier": ""},
		"patched_versions":      "", "vulnerable_functions": []any{},
	}
	cvss := map[string]any{"score": 0.0, "vector_string": ""}
	return map[string]any{
		"number": 0.0, "state": "",
		"dependency":             map[string]any{"package": map[string]any{"ecosystem": "", "name": ""}, "manifest_path": "", "scope": ""},
		"security_vulnerability": vulnerability,
		"security_advisory": map[string]any{
			"ghsa_id": "", "cve_id": "", "summary": "", "description": "",
			"vulnerabilities": []any{vulnerability}, "severity": "", "classification": "",
			"cvss": cvss, "cvss_severities": map[string]any{"cvss_v3": cvss, "cvss_v4": cvss},
			"cwes": []any{}, "epss": map[string]any{"percentage": 0.0, "percentile": 0.0},
			"identifiers": []any{}, "references": []any{},
			"published_at": "2025-01-02T03:04:05Z", "updated_at": "2025-01-02T03:04:05Z",
			"withdrawn_at": "2025-01-02T03:04:05Z",
		},
		"url": "", "html_url": "", "created_at": "2025-01-02T03:04:05Z",
		"updated_at": "2025-01-02T03:04:05Z", "dismissed_at": "2025-01-02T03:04:05Z",
		"dismissed_by":     map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"dismissed_reason": "", "dismissed_comment": "",
		"fixed_at": "2025-01-02T03:04:05Z", "auto_dismissed_at": "2025-01-02T03:04:05Z",
	}
}

func secretScanningOutputFixture() map[string]any {
	return map[string]any{
		"number": 0.0, "created_at": "2025-01-02T03:04:05Z",
		"url": "", "html_url": "", "locations_url": "",
		"first_location_detected": map[string]any{
			"path": "", "start_line": 0.0, "end_line": 0.0, "start_column": 0.0, "end_column": 0.0,
			"blob_sha": "", "blob_url": "", "commit_sha": "", "commit_url": "", "pull_request_comment_url": "",
		},
		"has_more_locations": false, "state": "", "resolution": "",
		"resolved_at": "2025-01-02T03:04:05Z",
		"resolved_by": map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"secret_type": "", "secret_type_display_name": "", "secret": "",
		"updated_at":        "2025-01-02T03:04:05Z",
		"is_base64_encoded": false, "multi_repo": false, "publicly_leaked": false,
		"push_protection_bypassed":    false,
		"push_protection_bypassed_by": map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"push_protection_bypassed_at": "2025-01-02T03:04:05Z",
		"resolution_comment":          "", "push_protection_bypass_request_comment": "",
		"push_protection_bypass_request_html_url":         "",
		"push_protection_bypass_request_reviewer":         map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"push_protection_bypass_request_reviewer_comment": "", "validity": "",
	}
}

func repositoryAdvisoryOutputFixture() map[string]any {
	cvss := map[string]any{"score": 0.0, "vector_string": ""}
	return map[string]any{
		"cvss": cvss, "cvss_severities": map[string]any{"cvss_v3": cvss, "cvss_v4": cvss},
		"cwes": []any{}, "ghsa_id": "", "summary": "", "description": "", "severity": "",
		"identifiers": []any{}, "references": []any{},
		"published_at": "2025-01-02T03:04:05Z", "updated_at": "2025-01-02T03:04:05Z",
		"withdrawn_at": "2025-01-02T03:04:05Z",
		"vulnerabilities": []any{map[string]any{
			"package":  map[string]any{"ecosystem": "", "name": ""},
			"severity": "", "vulnerable_version_range": "",
			"first_patched_version": map[string]any{"identifier": ""},
			"patched_versions":      "", "vulnerable_functions": []any{},
		}},
		"cve_id": "", "url": "", "html_url": "",
		"author":    map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"publisher": map[string]any{"login": "", "id": 0.0, "html_url": ""},
		"state":     "", "created_at": "2025-01-02T03:04:05Z", "closed_at": "2025-01-02T03:04:05Z",
		"submission": map[string]any{"accepted": false},
		"cwe_ids":    []any{}, "credits": []any{}, "credits_detailed": []any{},
	}
}
