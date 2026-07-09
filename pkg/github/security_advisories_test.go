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
