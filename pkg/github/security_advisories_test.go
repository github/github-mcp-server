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
