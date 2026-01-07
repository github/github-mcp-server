package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListGlobalSecurityAdvisories(t *testing.T) {
	toolDef := ListGlobalSecurityAdvisories(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_global_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "ecosystem")
	assert.Contains(t, schema.Properties, "severity")
	assert.Contains(t, schema.Properties, "ghsaId")
	assert.Empty(t, schema.Required)

	// Setup mock advisory for success case
	mockAdvisory := &github.GlobalSecurityAdvisory{
		SecurityAdvisory: github.SecurityAdvisory{
			GHSAID:      github.Ptr("GHSA-xxxx-xxxx-xxxx"),
			Summary:     github.Ptr("Test advisory"),
			Description: github.Ptr("This is a test advisory."),
			Severity:    github.Ptr("high"),
		},
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]interface{}
		expectError        bool
		expectedAdvisories []*github.GlobalSecurityAdvisory
		expectedErrMsg     string
	}{
		{
			name: "successful advisory fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisories: mockResponse(t, http.StatusOK, []*github.GlobalSecurityAdvisory{mockAdvisory}),
			}),
			requestArgs: map[string]interface{}{
				"type":      "reviewed",
				"ecosystem": "npm",
				"severity":  "high",
			},
			expectError:        false,
			expectedAdvisories: []*github.GlobalSecurityAdvisory{mockAdvisory},
		},
		{
			name: "invalid severity value",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisories: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Bad Request"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"type":     "reviewed",
				"severity": "extreme",
			},
			expectError:    true,
			expectedErrMsg: "failed to list global security advisories",
		},
		{
			name: "API error handling",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisories: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
				}),
			}),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "failed to list global security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedAdvisories []*github.GlobalSecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisories)
			assert.NoError(t, err)
			assert.Len(t, returnedAdvisories, len(tc.expectedAdvisories))
			for i, advisory := range returnedAdvisories {
				assert.Equal(t, *tc.expectedAdvisories[i].GHSAID, *advisory.GHSAID)
				assert.Equal(t, *tc.expectedAdvisories[i].Summary, *advisory.Summary)
				assert.Equal(t, *tc.expectedAdvisories[i].Description, *advisory.Description)
				assert.Equal(t, *tc.expectedAdvisories[i].Severity, *advisory.Severity)
			}
		})
	}
}

func Test_GetGlobalSecurityAdvisory(t *testing.T) {
	toolDef := GetGlobalSecurityAdvisory(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_global_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "ghsaId")
	assert.ElementsMatch(t, schema.Required, []string{"ghsaId"})

	// Setup mock advisory for success case
	mockAdvisory := &github.GlobalSecurityAdvisory{
		SecurityAdvisory: github.SecurityAdvisory{
			GHSAID:      github.Ptr("GHSA-xxxx-xxxx-xxxx"),
			Summary:     github.Ptr("Test advisory"),
			Description: github.Ptr("This is a test advisory."),
			Severity:    github.Ptr("high"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedAdvisory *github.GlobalSecurityAdvisory
		expectedErrMsg   string
	}{
		{
			name: "successful advisory fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisoriesByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "invalid ghsaId format",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisoriesByGhsaID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"message": "Bad Request"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"ghsaId": "invalid-ghsa-id",
			},
			expectError:    true,
			expectedErrMsg: "failed to get advisory",
		},
		{
			name: "advisory not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisoriesByGhsaID: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:    true,
			expectedErrMsg: "failed to get advisory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Verify the result
			assert.Contains(t, textContent.Text, *tc.expectedAdvisory.GHSAID)
			assert.Contains(t, textContent.Text, *tc.expectedAdvisory.Summary)
			assert.Contains(t, textContent.Text, *tc.expectedAdvisory.Description)
			assert.Contains(t, textContent.Text, *tc.expectedAdvisory.Severity)
		})
	}
}

func Test_ListRepositorySecurityAdvisories(t *testing.T) {
	// Verify tool definition once
	toolDef := ListRepositorySecurityAdvisories(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_repository_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "direction")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "state")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock advisories for success cases
	adv1 := &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-1111-1111-1111"),
		Summary:     github.Ptr("Repo advisory one"),
		Description: github.Ptr("First repo advisory."),
		Severity:    github.Ptr("high"),
	}
	adv2 := &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-2222-2222-2222"),
		Summary:     github.Ptr("Repo advisory two"),
		Description: github.Ptr("Second repo advisory."),
		Severity:    github.Ptr("medium"),
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]interface{}
		expectError        bool
		expectedAdvisories []*github.SecurityAdvisory
		expectedErrMsg     string
	}{
		{
			name: "successful advisories listing (no filters)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecurityAdvisoriesByOwnerByRepo: expect(t, expectations{
					path:        "/repos/owner/repo/security-advisories",
					queryParams: map[string]string{},
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1, adv2}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1, adv2},
		},
		{
			name: "successful advisories listing with filters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecurityAdvisoriesByOwnerByRepo: expect(t, expectations{
					path: "/repos/octo/hello-world/security-advisories",
					queryParams: map[string]string{
						"direction": "desc",
						"sort":      "updated",
						"state":     "published",
					},
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner":     "octo",
				"repo":      "hello-world",
				"direction": "desc",
				"sort":      "updated",
				"state":     "published",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1},
		},
		{
			name: "advisories listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecurityAdvisoriesByOwnerByRepo: expect(t, expectations{
					path:        "/repos/owner/repo/security-advisories",
					queryParams: map[string]string{},
				}).andThen(
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list repository security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			textContent := getTextResult(t, result)

			var returnedAdvisories []*github.SecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisories)
			assert.NoError(t, err)
			assert.Len(t, returnedAdvisories, len(tc.expectedAdvisories))
			for i, advisory := range returnedAdvisories {
				assert.Equal(t, *tc.expectedAdvisories[i].GHSAID, *advisory.GHSAID)
				assert.Equal(t, *tc.expectedAdvisories[i].Summary, *advisory.Summary)
				assert.Equal(t, *tc.expectedAdvisories[i].Description, *advisory.Description)
				assert.Equal(t, *tc.expectedAdvisories[i].Severity, *advisory.Severity)
			}
		})
	}
}

func Test_ListOrgRepositorySecurityAdvisories(t *testing.T) {
	// Verify tool definition once
	toolDef := ListOrgRepositorySecurityAdvisories(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_org_repository_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "org")
	assert.Contains(t, schema.Properties, "direction")
	assert.Contains(t, schema.Properties, "sort")
	assert.Contains(t, schema.Properties, "state")
	assert.ElementsMatch(t, schema.Required, []string{"org"})

	adv1 := &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-aaaa-bbbb-cccc"),
		Summary:     github.Ptr("Org repo advisory 1"),
		Description: github.Ptr("First advisory"),
		Severity:    github.Ptr("low"),
	}
	adv2 := &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-dddd-eeee-ffff"),
		Summary:     github.Ptr("Org repo advisory 2"),
		Description: github.Ptr("Second advisory"),
		Severity:    github.Ptr("critical"),
	}

	tests := []struct {
		name               string
		mockedClient       *http.Client
		requestArgs        map[string]interface{}
		expectError        bool
		expectedAdvisories []*github.SecurityAdvisory
		expectedErrMsg     string
	}{
		{
			name: "successful listing (no filters)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsSecurityAdvisoriesByOrg: expect(t, expectations{
					path:        "/orgs/octo/security-advisories",
					queryParams: map[string]string{},
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1, adv2}),
				),
			}),
			requestArgs: map[string]interface{}{
				"org": "octo",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1, adv2},
		},
		{
			name: "successful listing with filters",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsSecurityAdvisoriesByOrg: expect(t, expectations{
					path: "/orgs/octo/security-advisories",
					queryParams: map[string]string{
						"direction": "asc",
						"sort":      "created",
						"state":     "triage",
					},
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1}),
				),
			}),
			requestArgs: map[string]interface{}{
				"org":       "octo",
				"direction": "asc",
				"sort":      "created",
				"state":     "triage",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1},
		},
		{
			name: "listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsSecurityAdvisoriesByOrg: expect(t, expectations{
					path:        "/orgs/octo/security-advisories",
					queryParams: map[string]string{},
				}).andThen(
					mockResponse(t, http.StatusForbidden, map[string]string{"message": "Forbidden"}),
				),
			}),
			requestArgs: map[string]interface{}{
				"org": "octo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list organization repository security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)

			textContent := getTextResult(t, result)

			var returnedAdvisories []*github.SecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisories)
			assert.NoError(t, err)
			assert.Len(t, returnedAdvisories, len(tc.expectedAdvisories))
			for i, advisory := range returnedAdvisories {
				assert.Equal(t, *tc.expectedAdvisories[i].GHSAID, *advisory.GHSAID)
				assert.Equal(t, *tc.expectedAdvisories[i].Summary, *advisory.Summary)
				assert.Equal(t, *tc.expectedAdvisories[i].Description, *advisory.Description)
				assert.Equal(t, *tc.expectedAdvisories[i].Severity, *advisory.Severity)
			}
		})
	}
}

func Test_ReportSecurityVulnerability(t *testing.T) {
	toolDef := ReportSecurityVulnerability(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "report_security_vulnerability", tool.Name)
	assert.NotEmpty(t, tool.Description)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "summary")
	assert.Contains(t, schema.Properties, "description")
	assert.Contains(t, schema.Properties, "severity")
	assert.Contains(t, schema.Properties, "cvss_vector_string")
	assert.Contains(t, schema.Properties, "cwe_ids")
	assert.Contains(t, schema.Properties, "vulnerabilities")
	assert.Contains(t, schema.Properties, "start_private_fork")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "summary", "description"})

	// Setup mock advisory for success case
	mockAdvisory := &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-xxxx-yyyy-zzzz"),
		Summary:     github.Ptr("Newly reported vulnerability"),
		Description: github.Ptr("A detailed description of the vulnerability."),
		Severity:    github.Ptr("high"),
		State:       github.Ptr("triage"),
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedAdvisory *github.SecurityAdvisory
		expectedErrMsg   string
	}{
		{
			name: "successful vulnerability report",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"summary":     "Newly reported vulnerability",
				"description": "A detailed description of the vulnerability.",
				"severity":    "high",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful report with CWE IDs",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"summary":     "XSS vulnerability",
				"description": "Cross-site scripting issue in form validation.",
				"severity":    "medium",
				"cwe_ids":     []interface{}{"CWE-79", "CWE-20"},
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful report with vulnerabilities",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"summary":     "Package vulnerability",
				"description": "Security issue in npm package.",
				"severity":    "critical",
				"vulnerabilities": []interface{}{
					map[string]interface{}{
						"package": map[string]interface{}{
							"ecosystem": "npm",
							"name":      "vulnerable-package",
						},
						"vulnerable_version_range": "< 1.0.0",
						"patched_versions":         "1.0.0",
						"vulnerable_functions":     []interface{}{"validateInput", "processData"},
					},
				},
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful report with CVSS vector string",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"owner":              "owner",
				"repo":               "repo",
				"summary":            "Custom CVSS severity",
				"description":        "Vulnerability with custom CVSS scoring.",
				"cvss_vector_string": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful report with private fork",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]interface{}{
				"owner":              "owner",
				"repo":               "repo",
				"summary":            "Vulnerability requiring patch",
				"description":        "Issue that needs immediate fix.",
				"severity":           "high",
				"start_private_fork": true,
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name:         "error when both severity and cvss_vector_string provided",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"owner":              "owner",
				"repo":               "repo",
				"summary":            "Test",
				"description":        "Test description",
				"severity":           "high",
				"cvss_vector_string": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			},
			expectError:    true,
			expectedErrMsg: "cannot specify both severity and cvss_vector_string",
		},
		{
			name:         "missing required owner parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]interface{}{
				"repo":        "repo",
				"summary":     "Test",
				"description": "Test description",
			},
			expectError:    true,
			expectedErrMsg: "owner",
		},
		{
			name: "API error - forbidden",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"summary":     "Test",
				"description": "Test description",
				"severity":    "high",
			},
			expectError:    true,
			expectedErrMsg: "failed to report security vulnerability",
		},
		{
			name: "API error - not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "nonexistent",
				"summary":     "Test",
				"description": "Test description",
				"severity":    "medium",
			},
			expectError:    true,
			expectedErrMsg: "failed to report security vulnerability",
		},
		{
			name: "API error - validation failed",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/owner/repo/security-advisories/reports": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"summary":     "Test",
				"description": "Test description",
				"severity":    "invalid",
			},
			expectError:    true,
			expectedErrMsg: "failed to report security vulnerability",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				// For validation errors, err is nil but result.IsError is true
				// For API errors, err is not nil
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				} else {
					require.True(t, result.IsError)
					text := getTextResult(t, result).Text
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var returnedAdvisory github.SecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisory)
			assert.NoError(t, err)
			assert.Equal(t, *tc.expectedAdvisory.GHSAID, *returnedAdvisory.GHSAID)
			assert.Equal(t, *tc.expectedAdvisory.Summary, *returnedAdvisory.Summary)
			assert.Equal(t, *tc.expectedAdvisory.Description, *returnedAdvisory.Description)
			assert.Equal(t, *tc.expectedAdvisory.Severity, *returnedAdvisory.Severity)
			if tc.expectedAdvisory.State != nil {
				assert.Equal(t, *tc.expectedAdvisory.State, *returnedAdvisory.State)
			}
		})
	}
}
