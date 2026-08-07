package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		requestArgs        map[string]any
		expectError        bool
		expectedAdvisories []*github.GlobalSecurityAdvisory
		expectedErrMsg     string
	}{
		{
			name: "successful advisory fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisories: mockResponse(t, http.StatusOK, []*github.GlobalSecurityAdvisory{mockAdvisory}),
			}),
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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
			requestArgs:    map[string]any{},
			expectError:    true,
			expectedErrMsg: "failed to list global security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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
		requestArgs      map[string]any
		expectError      bool
		expectedAdvisory *github.GlobalSecurityAdvisory
		expectedErrMsg   string
	}{
		{
			name: "successful advisory fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetAdvisoriesByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:    true,
			expectedErrMsg: "failed to get advisory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := mustNewGHClient(t, tc.mockedClient)
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
		requestArgs        map[string]any
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
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list repository security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
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

// Test_ListRepositorySecurityAdvisories_IFC_FeatureFlag verifies the IFC label
// attached to list_repository_security_advisories. The label is only present
// when the ifc_labels feature flag is enabled, and — critically — confidentiality
// is public only when the repository is public AND every returned advisory is
// published. Draft/triage/closed advisories are not world-readable even on a
// public repo, so a result containing one must be labeled private. This guards
// against the under-classification raised in PR review.
func Test_ListRepositorySecurityAdvisories_IFC_FeatureFlag(t *testing.T) {
	t.Parallel()

	toolDef := ListRepositorySecurityAdvisories(translations.NullTranslationHelper)

	publishedAdvisory := &github.SecurityAdvisory{
		GHSAID:  github.Ptr("GHSA-1111-1111-1111"),
		Summary: github.Ptr("Published advisory"),
		State:   github.Ptr("published"),
	}
	draftAdvisory := &github.SecurityAdvisory{
		GHSAID:  github.Ptr("GHSA-2222-2222-2222"),
		Summary: github.Ptr("Draft advisory"),
		State:   github.Ptr("draft"),
	}

	makeMockClient := func(isPrivate bool, advisories []*github.SecurityAdvisory) *http.Client {
		return MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusOK, advisories),
			GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, map[string]any{
				"name":    "repo",
				"private": isPrivate,
			}),
		})
	}

	reqParams := map[string]any{
		"owner": "owner",
		"repo":  "repo",
	}

	readIFC := func(t *testing.T, result *mcp.CallToolResult) (map[string]any, bool) {
		t.Helper()
		if result.Meta == nil {
			return nil, false
		}
		label, ok := result.Meta["ifc"]
		if !ok {
			return nil, false
		}
		labelJSON, err := json.Marshal(label)
		require.NoError(t, err)
		var labelMap map[string]any
		require.NoError(t, json.Unmarshal(labelJSON, &labelMap))
		return labelMap, true
	}

	t.Run("feature flag disabled omits ifc label", func(t *testing.T) {
		t.Parallel()
		deps := BaseDeps{Client: mustNewGHClient(t, makeMockClient(false, []*github.SecurityAdvisory{publishedAdvisory}))}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Nil(t, result.Meta, "result meta should be nil when IFC labels are disabled")
	})

	t.Run("public repo with only published advisories is public", func(t *testing.T) {
		t.Parallel()
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(false, []*github.SecurityAdvisory{publishedAdvisory})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		label, ok := readIFC(t, result)
		require.True(t, ok, "result meta should contain ifc key")
		assert.Equal(t, "untrusted", label["integrity"])
		assert.Equal(t, "public", label["confidentiality"])
	})

	t.Run("public repo with a draft advisory is private", func(t *testing.T) {
		t.Parallel()
		// Reviewer scenario: a draft advisory on a public repo is not
		// world-readable, so the label must not be public.
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(false, []*github.SecurityAdvisory{publishedAdvisory, draftAdvisory})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		label, ok := readIFC(t, result)
		require.True(t, ok, "result meta should contain ifc key")
		assert.Equal(t, "untrusted", label["integrity"])
		assert.Equal(t, "private", label["confidentiality"], "draft advisory on public repo must be private")
	})

	t.Run("private repo is private", func(t *testing.T) {
		t.Parallel()
		deps := BaseDeps{
			Client:         mustNewGHClient(t, makeMockClient(true, []*github.SecurityAdvisory{publishedAdvisory})),
			featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
		}
		handler := toolDef.Handler(deps)

		request := createMCPRequest(reqParams)
		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		label, ok := readIFC(t, result)
		require.True(t, ok, "result meta should contain ifc key")
		assert.Equal(t, "untrusted", label["integrity"])
		assert.Equal(t, "private", label["confidentiality"])
	})
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
		requestArgs        map[string]any
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
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
				"org": "octo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list organization repository security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
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
func sampleAdvisoryVulnerabilities() []any {
	return []any{
		map[string]any{
			"package": map[string]any{
				"ecosystem": "npm",
				"name":      "example-package",
			},
			"vulnerable_version_range": "< 2.0.0",
			"patched_versions":         "2.0.0",
		},
	}
}

func mockRepositorySecurityAdvisory() *github.SecurityAdvisory {
	return &github.SecurityAdvisory{
		GHSAID:      github.Ptr("GHSA-xxxx-xxxx-xxxx"),
		Summary:     github.Ptr("Stored XSS in Core"),
		Description: github.Ptr("A stored XSS vulnerability in Core."),
		Severity:    github.Ptr("high"),
		State:       github.Ptr("draft"),
	}
}

func Test_CreateRepositorySecurityAdvisory(t *testing.T) {
	toolDef := CreateRepositorySecurityAdvisory(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "create_repository_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.OpenWorldHint)
	assert.True(t, *tool.Annotations.OpenWorldHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.True(t, *tool.Annotations.DestructiveHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "summary", "description", "vulnerabilities"})

	mockAdvisory := mockRepositorySecurityAdvisory()
	expectedRequestBody := map[string]any{
		"summary":     "Stored XSS in Core",
		"description": "A stored XSS vulnerability in Core.",
		"severity":    "high",
		"vulnerabilities": []any{
			map[string]any{
				"package": map[string]any{
					"ecosystem": "npm",
					"name":      "example-package",
				},
				"vulnerable_version_range": "< 2.0.0",
				"patched_versions":         "2.0.0",
			},
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]any
		expectError      bool
		expectedAdvisory *github.SecurityAdvisory
		expectedErrMsg   string
	}{
		{
			name: "successful advisory creation",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: expectRequestBody(t, expectedRequestBody).andThen(
					mockResponse(t, http.StatusCreated, mockAdvisory),
				),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful advisory creation with cvss only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"summary":            "Stored XSS in Core",
					"description":        "A stored XSS vulnerability in Core.",
					"cvss_vector_string": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
					"vulnerabilities": []any{
						map[string]any{
							"package": map[string]any{
								"ecosystem": "npm",
								"name":      "example-package",
							},
							"vulnerable_version_range": "< 2.0.0",
							"patched_versions":         "2.0.0",
						},
					},
				}).andThen(mockResponse(t, http.StatusCreated, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":            "octo",
				"repo":             "hello-world",
				"summary":          "Stored XSS in Core",
				"description":      "A stored XSS vulnerability in Core.",
				"cvssVectorString": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
				"vulnerabilities":  sampleAdvisoryVulnerabilities(),
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "successful advisory creation with startPrivateFork",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"summary":            "Stored XSS in Core",
					"description":        "A stored XSS vulnerability in Core.",
					"severity":           "high",
					"start_private_fork": true,
					"vulnerabilities": []any{
						map[string]any{
							"package": map[string]any{
								"ecosystem": "npm",
								"name":      "example-package",
							},
							"vulnerable_version_range": "< 2.0.0",
							"patched_versions":         "2.0.0",
						},
					},
				}).andThen(mockResponse(t, http.StatusCreated, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":            "octo",
				"repo":             "hello-world",
				"summary":          "Stored XSS in Core",
				"description":      "A stored XSS vulnerability in Core.",
				"severity":         "high",
				"startPrivateFork": true,
				"vulnerabilities":  sampleAdvisoryVulnerabilities(),
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "missing required summary",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"description":     "A stored XSS vulnerability in Core.",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: summary",
		},
		{
			name: "missing severity and cvss",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:    true,
			expectedErrMsg: "exactly one of severity or cvssVectorString must be provided",
		},
		{
			name: "both severity and cvss set",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":            "octo",
				"repo":             "hello-world",
				"summary":          "Stored XSS in Core",
				"description":      "A stored XSS vulnerability in Core.",
				"severity":         "high",
				"cvssVectorString": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
				"vulnerabilities":  sampleAdvisoryVulnerabilities(),
			},
			expectError:    true,
			expectedErrMsg: "severity and cvssVectorString cannot both be set",
		},
		{
			name: "invalid severity value",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "urgent",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:    true,
			expectedErrMsg: "severity must be one of: low, medium, high, critical",
		},
		{
			name: "successful advisory creation with credits and cweIds",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: expectRequestBody(t, map[string]any{
					"summary":     "Stored XSS in Core",
					"description": "A stored XSS vulnerability in Core.",
					"severity":    "high",
					"cve_id":      "CVE-2024-12345",
					"cwe_ids":     []any{"CWE-79"},
					"credits": []any{
						map[string]any{
							"login": "octocat",
							"type":  "finder",
						},
					},
					"vulnerabilities": []any{
						map[string]any{
							"package": map[string]any{
								"ecosystem": "npm",
								"name":      "example-package",
							},
							"vulnerable_version_range": "< 2.0.0",
							"patched_versions":         "2.0.0",
						},
					},
				}).andThen(mockResponse(t, http.StatusCreated, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"cveId":           "CVE-2024-12345",
				"cweIds":          []any{"CWE-79"},
				"credits": []any{
					map[string]any{
						"login": "octocat",
						"type":  "finder",
					},
				},
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "reject empty vulnerabilities array",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid vulnerabilities: at least one vulnerability must be provided",
		},
		{
			name: "reject null vulnerabilities",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid vulnerabilities: at least one vulnerability must be provided",
		},
		{
			name: "reject null credits",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
				"credits":         nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid credits: value must not be null",
		},
		{
			name: "reject null cweIds",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
				"cweIds":          nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid cweIds: value must not be null",
		},
		{
			name: "reject empty cweIds",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
				"cweIds":          []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid cweIds: at least one CWE ID must be provided when cweIds is specified",
		},
		{
			name: "reject empty credits",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: mockResponse(t, http.StatusCreated, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
				"credits":         []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid credits: at least one credit must be provided when credits is specified",
		},
		{
			name: "API error handling",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesByOwnerByRepo: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
				},
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"summary":         "Stored XSS in Core",
				"description":     "A stored XSS vulnerability in Core.",
				"severity":        "high",
				"vulnerabilities": sampleAdvisoryVulnerabilities(),
			},
			expectError:    true,
			expectedErrMsg: "failed to create repository security advisory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectedErrMsg != "" {
				if tc.expectError {
					if err != nil {
						assert.Contains(t, err.Error(), tc.expectedErrMsg)
						return
					}
					require.NotNil(t, result)
					assert.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, tc.expectedErrMsg)
					return
				}
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)

			var returnedAdvisory github.SecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisory)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedAdvisory.GHSAID, *returnedAdvisory.GHSAID)
			assert.Equal(t, *tc.expectedAdvisory.Summary, *returnedAdvisory.Summary)
			assert.Equal(t, *tc.expectedAdvisory.Description, *returnedAdvisory.Description)
			assert.Equal(t, *tc.expectedAdvisory.Severity, *returnedAdvisory.Severity)
		})
	}
}

func Test_UpdateRepositorySecurityAdvisory(t *testing.T) {
	toolDef := UpdateRepositorySecurityAdvisory(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_repository_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.OpenWorldHint)
	assert.True(t, *tool.Annotations.OpenWorldHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.True(t, *tool.Annotations.DestructiveHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "ghsaId"})

	mockAdvisory := mockRepositorySecurityAdvisory()
	mockAdvisory.State = github.Ptr("published")

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]any
		expectError      bool
		expectedAdvisory *github.SecurityAdvisory
		expectedErrMsg   string
	}{
		{
			name: "successful advisory update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: expect(t, expectations{
					path:        "/repos/octo/hello-world/security-advisories/GHSA-xxxx-xxxx-xxxx",
					requestBody: map[string]any{"state": "published", "severity": "high"},
				}).andThen(mockResponse(t, http.StatusOK, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":    "octo",
				"repo":     "hello-world",
				"ghsaId":   "GHSA-xxxx-xxxx-xxxx",
				"state":    "published",
				"severity": "high",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "lowercase ghsaId normalized in request path",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: expect(t, expectations{
					path:        "/repos/octo/hello-world/security-advisories/GHSA-abcd-1234-5678",
					requestBody: map[string]any{"state": "published"},
				}).andThen(mockResponse(t, http.StatusOK, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "ghsa-abcd-1234-5678",
				"state":  "published",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "invalid ghsaId format",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "invalid/../../path",
				"state":  "published",
			},
			expectError:    true,
			expectedErrMsg: "invalid ghsaId format: must match GHSA-xxxx-xxxx-xxxx",
		},
		{
			name: "missing required ghsaId",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner": "octo",
				"repo":  "hello-world",
				"state": "published",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: ghsaId",
		},
		{
			name: "no update fields provided",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:    true,
			expectedErrMsg: "at least one of summary, description, vulnerabilities, cveId, cweIds, severity, cvssVectorString, credits, or state must be provided for update",
		},
		{
			name: "both severity and cvss set on update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":            "octo",
				"repo":             "hello-world",
				"ghsaId":           "GHSA-xxxx-xxxx-xxxx",
				"severity":         "high",
				"cvssVectorString": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
			},
			expectError:    true,
			expectedErrMsg: "severity and cvssVectorString cannot both be set",
		},
		{
			name: "reject empty severity on update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":    "octo",
				"repo":     "hello-world",
				"ghsaId":   "GHSA-xxxx-xxxx-xxxx",
				"severity": "",
			},
			expectError:    true,
			expectedErrMsg: "severity must be one of: low, medium, high, critical",
		},
		{
			name: "reject empty state on update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"state":  "",
			},
			expectError:    true,
			expectedErrMsg: "state must be one of: draft, published, closed, triage",
		},
		{
			name: "reject empty cvssVectorString on update",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":            "octo",
				"repo":             "hello-world",
				"ghsaId":           "GHSA-xxxx-xxxx-xxxx",
				"cvssVectorString": "",
			},
			expectError:    true,
			expectedErrMsg: "cvssVectorString must not be empty",
		},
		{
			name: "clear summary with empty string",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: expect(t, expectations{
					path:        "/repos/octo/hello-world/security-advisories/GHSA-xxxx-xxxx-xxxx",
					requestBody: map[string]any{"summary": ""},
				}).andThen(mockResponse(t, http.StatusOK, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":   "octo",
				"repo":    "hello-world",
				"ghsaId":  "GHSA-xxxx-xxxx-xxxx",
				"summary": "",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "reject empty vulnerabilities array",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"ghsaId":          "GHSA-xxxx-xxxx-xxxx",
				"vulnerabilities": []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid vulnerabilities: at least one vulnerability must be provided",
		},
		{
			name: "reject null vulnerabilities",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":           "octo",
				"repo":            "hello-world",
				"ghsaId":          "GHSA-xxxx-xxxx-xxxx",
				"vulnerabilities": nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid vulnerabilities: at least one vulnerability must be provided",
		},
		{
			name: "reject null credits only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":   "octo",
				"repo":    "hello-world",
				"ghsaId":  "GHSA-xxxx-xxxx-xxxx",
				"credits": nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid credits: value must not be null",
		},
		{
			name: "reject null cweIds only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"cweIds": nil,
			},
			expectError:    true,
			expectedErrMsg: "invalid cweIds: value must not be null",
		},
		{
			name: "reject empty cweIds only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"cweIds": []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid cweIds: at least one CWE ID must be provided when cweIds is specified",
		},
		{
			name: "reject empty credits only",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":   "octo",
				"repo":    "hello-world",
				"ghsaId":  "GHSA-xxxx-xxxx-xxxx",
				"credits": []any{},
			},
			expectError:    true,
			expectedErrMsg: "invalid credits: at least one credit must be provided when credits is specified",
		},
		{
			name: "successful update with credits and cweIds",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: expect(t, expectations{
					path: "/repos/octo/hello-world/security-advisories/GHSA-xxxx-xxxx-xxxx",
					requestBody: map[string]any{
						"cve_id":  "CVE-2024-12345",
						"cwe_ids": []any{"CWE-79"},
						"credits": []any{
							map[string]any{
								"login": "octocat",
								"type":  "reporter",
							},
						},
					},
				}).andThen(mockResponse(t, http.StatusOK, mockAdvisory)),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"cveId":  "CVE-2024-12345",
				"cweIds": []any{"CWE-79"},
				"credits": []any{
					map[string]any{
						"login": "octocat",
						"type":  "reporter",
					},
				},
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "invalid state value",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, mockAdvisory),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"state":  "open",
			},
			expectError:    true,
			expectedErrMsg: "state must be one of: draft, published, closed, triage",
		},
		{
			name: "API error handling",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposSecurityAdvisoriesByOwnerByRepoByGhsaID: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message": "Validation Failed"}`))
				},
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
				"state":  "published",
			},
			expectError:    true,
			expectedErrMsg: "failed to update repository security advisory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectedErrMsg != "" {
				if tc.expectError {
					if err != nil {
						assert.Contains(t, err.Error(), tc.expectedErrMsg)
						return
					}
					require.NotNil(t, result)
					assert.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, tc.expectedErrMsg)
					return
				}
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)

			var returnedAdvisory github.SecurityAdvisory
			err = json.Unmarshal([]byte(textContent.Text), &returnedAdvisory)
			require.NoError(t, err)
			assert.Equal(t, *tc.expectedAdvisory.GHSAID, *returnedAdvisory.GHSAID)
			assert.Equal(t, *tc.expectedAdvisory.State, *returnedAdvisory.State)
		})
	}
}

func Test_RequestCVEForRepositorySecurityAdvisory(t *testing.T) {
	toolDef := RequestCVEForRepositorySecurityAdvisory(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "request_cve_for_repository_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint)
	require.NotNil(t, tool.Annotations.OpenWorldHint)
	assert.True(t, *tool.Annotations.OpenWorldHint)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	assert.True(t, *tool.Annotations.DestructiveHint)

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be of type *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "ghsaId"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedText   string
		expectedErrMsg string
	}{
		{
			name: "successful CVE request",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:  false,
			expectedText: "CVE request submitted successfully",
		},
		{
			name: "successful CVE request with accepted status",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusAccepted)
				},
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:  false,
			expectedText: "CVE request submitted successfully",
		},
		{
			name: "lowercase ghsaId normalized in CVE request path",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: expectPath(t, "/repos/octo/hello-world/security-advisories/GHSA-abcd-1234-5678/cve").andThen(
					mockResponse(t, http.StatusOK, nil),
				),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "ghsa-abcd-1234-5678",
			},
			expectError:  false,
			expectedText: "CVE request submitted successfully",
		},
		{
			name: "invalid ghsaId format",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "not-a-valid-ghsa",
			},
			expectError:    true,
			expectedErrMsg: "invalid ghsaId format: must match GHSA-xxxx-xxxx-xxxx",
		},
		{
			name: "missing required ghsaId",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: mockResponse(t, http.StatusOK, nil),
			}),
			requestArgs: map[string]any{
				"owner": "octo",
				"repo":  "hello-world",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: ghsaId",
		},
		{
			name: "API error handling",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PostReposSecurityAdvisoriesCveByOwnerByRepoByGhsaID: func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
				},
			}),
			requestArgs: map[string]any{
				"owner":  "octo",
				"repo":   "hello-world",
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:    true,
			expectedErrMsg: "failed to request CVE for repository security advisory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectedErrMsg != "" {
				if tc.expectError {
					if err != nil {
						assert.Contains(t, err.Error(), tc.expectedErrMsg)
						return
					}
					require.NotNil(t, result)
					assert.True(t, result.IsError)
					assert.Contains(t, getTextResult(t, result).Text, tc.expectedErrMsg)
					return
				}
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			assert.Equal(t, tc.expectedText, textContent.Text)
		})
	}
}

func Test_ParseAdvisoryVulnerabilities(t *testing.T) {
	t.Run("required missing parameter", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required parameter: vulnerabilities")
	})

	t.Run("invalid parameter type", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": "not-an-array",
		}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid vulnerabilities")
	})

	t.Run("valid vulnerabilities", func(t *testing.T) {
		vulns, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": sampleAdvisoryVulnerabilities(),
		}, "vulnerabilities", true)
		require.NoError(t, err)
		require.Len(t, vulns, 1)
		assert.Equal(t, "npm", vulns[0].Package.Ecosystem)
		require.NotNil(t, vulns[0].Package.Name)
		assert.Equal(t, "example-package", *vulns[0].Package.Name)
	})

	t.Run("optional empty vulnerabilities array rejected", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": []any{},
		}, "vulnerabilities", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid vulnerabilities: at least one vulnerability must be provided")
	})

	t.Run("optional null vulnerabilities rejected", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": nil,
		}, "vulnerabilities", false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid vulnerabilities: at least one vulnerability must be provided")
	})

	t.Run("required empty vulnerabilities array rejected", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": []any{},
		}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid vulnerabilities: at least one vulnerability must be provided")
	})

	t.Run("missing package ecosystem", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": []any{
				map[string]any{
					"package": map[string]any{
						"name": "example-package",
					},
				},
			},
		}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vulnerabilities[0].package.ecosystem is required")
	})

	t.Run("missing package name", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": []any{
				map[string]any{
					"package": map[string]any{
						"ecosystem": "npm",
					},
				},
			},
		}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vulnerabilities[0].package.name is required")
	})

	t.Run("invalid package ecosystem", func(t *testing.T) {
		_, err := parseAdvisoryVulnerabilities(map[string]any{
			"vulnerabilities": []any{
				map[string]any{
					"package": map[string]any{
						"ecosystem": "invalid-ecosystem",
						"name":      "example-package",
					},
				},
			},
		}, "vulnerabilities", true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `vulnerabilities[0].package.ecosystem "invalid-ecosystem" is invalid`)
	})
}

func Test_ParseAdvisoryCredits(t *testing.T) {
	t.Run("valid credits", func(t *testing.T) {
		credits, err := parseAdvisoryCredits(map[string]any{
			"credits": []any{
				map[string]any{
					"login": "octocat",
					"type":  "finder",
				},
			},
		}, "credits")
		require.NoError(t, err)
		require.Len(t, credits, 1)
		assert.Equal(t, "octocat", credits[0].Login)
		assert.Equal(t, "finder", credits[0].Type)
	})

	t.Run("missing login", func(t *testing.T) {
		_, err := parseAdvisoryCredits(map[string]any{
			"credits": []any{
				map[string]any{
					"type": "finder",
				},
			},
		}, "credits")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "credits[0].login is required")
	})

	t.Run("missing type", func(t *testing.T) {
		_, err := parseAdvisoryCredits(map[string]any{
			"credits": []any{
				map[string]any{
					"login": "octocat",
				},
			},
		}, "credits")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "credits[0].type is required")
	})

	t.Run("invalid type", func(t *testing.T) {
		_, err := parseAdvisoryCredits(map[string]any{
			"credits": []any{
				map[string]any{
					"login": "octocat",
					"type":  "invalid-type",
				},
			},
		}, "credits")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `credits[0].type "invalid-type" is invalid`)
	})

	t.Run("null credits rejected", func(t *testing.T) {
		_, err := parseAdvisoryCredits(map[string]any{
			"credits": nil,
		}, "credits")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid credits: value must not be null")
	})

	t.Run("empty credits rejected", func(t *testing.T) {
		_, err := parseAdvisoryCredits(map[string]any{
			"credits": []any{},
		}, "credits")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid credits: at least one credit must be provided when credits is specified")
	})
}

func Test_ParseAdvisoryCWEIDs(t *testing.T) {
	t.Run("valid cweIds", func(t *testing.T) {
		cweIDs, err := parseAdvisoryCWEIDs(map[string]any{
			"cweIds": []any{"CWE-79"},
		}, "cweIds")
		require.NoError(t, err)
		assert.Equal(t, []string{"CWE-79"}, cweIDs)
	})

	t.Run("null cweIds rejected", func(t *testing.T) {
		_, err := parseAdvisoryCWEIDs(map[string]any{
			"cweIds": nil,
		}, "cweIds")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cweIds: value must not be null")
	})

	t.Run("empty cweIds rejected", func(t *testing.T) {
		_, err := parseAdvisoryCWEIDs(map[string]any{
			"cweIds": []any{},
		}, "cweIds")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid cweIds: at least one CWE ID must be provided when cweIds is specified")
	})
}

func Test_validateSeverityOrCVSS(t *testing.T) {
	t.Run("create requires exactly one", func(t *testing.T) {
		assert.NoError(t, validateSeverityOrCVSS("high", "", true))
		assert.NoError(t, validateSeverityOrCVSS("", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", true))
		assert.Error(t, validateSeverityOrCVSS("", "", true))
		assert.Error(t, validateSeverityOrCVSS("high", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", true))
	})

	t.Run("update rejects both but allows neither", func(t *testing.T) {
		assert.NoError(t, validateSeverityOrCVSS("", "", false))
		assert.NoError(t, validateSeverityOrCVSS("high", "", false))
		assert.NoError(t, validateSeverityOrCVSS("", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", false))
		assert.Error(t, validateSeverityOrCVSS("high", "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H", false))
	})

	t.Run("rejects invalid severity", func(t *testing.T) {
		assert.Error(t, validateAdvisorySeverity("urgent"))
		assert.NoError(t, validateAdvisorySeverity("critical"))
		assert.NoError(t, validateAdvisorySeverity(""))
	})

	t.Run("rejects invalid state", func(t *testing.T) {
		assert.Error(t, validateAdvisoryState("open"))
		assert.NoError(t, validateAdvisoryState("published"))
		assert.NoError(t, validateAdvisoryState(""))
	})

	t.Run("rejects empty present enum values on update", func(t *testing.T) {
		assert.Error(t, validatePresentAdvisorySeverity(""))
		assert.NoError(t, validatePresentAdvisorySeverity("high"))
		assert.Error(t, validatePresentAdvisoryState(""))
		assert.NoError(t, validatePresentAdvisoryState("draft"))
		assert.Error(t, validatePresentCVSSVectorString(""))
		assert.NoError(t, validatePresentCVSSVectorString("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H"))
	})
}

func Test_validateGHSAID(t *testing.T) {
	assert.NoError(t, validateGHSAID("GHSA-xxxx-xxxx-xxxx"))
	assert.NoError(t, validateGHSAID("ghsa-abcd-1234-5678"))
	assert.Error(t, validateGHSAID("invalid-ghsa-id"))
	assert.Error(t, validateGHSAID("GHSA-xxxx-xxxx-xxxx/extra"))
	assert.Error(t, validateGHSAID("../etc/passwd"))
}

func Test_normalizeGHSAID(t *testing.T) {
	normalized, err := normalizeGHSAID("ghsa-abcd-1234-5678")
	assert.NoError(t, err)
	assert.Equal(t, "GHSA-abcd-1234-5678", normalized)

	normalized, err = normalizeGHSAID("GHSA-xxxx-xxxx-xxxx")
	assert.NoError(t, err)
	assert.Equal(t, "GHSA-xxxx-xxxx-xxxx", normalized)

	_, err = normalizeGHSAID("invalid-ghsa-id")
	assert.Error(t, err)
}

func TestSecurityAdvisoryWriteToolsRegistered(t *testing.T) {
	expected := map[string]struct {
		readOnly     bool
		destructive  bool
		openWorld    bool
	}{
		"create_repository_security_advisory": {
			readOnly:    false,
			destructive: true,
			openWorld:   true,
		},
		"update_repository_security_advisory": {
			readOnly:    false,
			destructive: true,
			openWorld:   true,
		},
		"request_cve_for_repository_security_advisory": {
			readOnly:    false,
			destructive: true,
			openWorld:   true,
		},
	}

	for _, tool := range AllTools(translations.NullTranslationHelper) {
		want, ok := expected[tool.Tool.Name]
		if !ok {
			continue
		}
		assert.Equal(t, ToolsetMetadataSecurityAdvisories.ID, tool.Toolset.ID)
		require.NotNil(t, tool.Tool.Annotations)
		assert.Equal(t, want.readOnly, tool.Tool.Annotations.ReadOnlyHint)
		require.NotNil(t, tool.Tool.Annotations.OpenWorldHint)
		assert.Equal(t, want.openWorld, *tool.Tool.Annotations.OpenWorldHint)
		if want.destructive {
			require.NotNil(t, tool.Tool.Annotations.DestructiveHint)
			assert.True(t, *tool.Tool.Annotations.DestructiveHint)
		} else {
			assert.Nil(t, tool.Tool.Annotations.DestructiveHint)
		}
		delete(expected, tool.Tool.Name)
	}

	assert.Empty(t, expected, "missing security advisory write tools: %v", expected)
}
