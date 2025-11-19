package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListGlobalSecurityAdvisories(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := ListGlobalSecurityAdvisories(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_global_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "ecosystem")
	assert.Contains(t, tool.InputSchema.Properties, "severity")
	assert.Contains(t, tool.InputSchema.Properties, "ghsaId")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{})

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetAdvisories,
					[]*github.GlobalSecurityAdvisory{mockAdvisory},
				),
			),
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetAdvisories,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Bad Request"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"type":     "reviewed",
				"severity": "extreme",
			},
			expectError:    true,
			expectedErrMsg: "failed to list global security advisories",
		},
		{
			name: "API error handling",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetAdvisories,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal Server Error"}`))
					}),
				),
			),
			requestArgs:    map[string]interface{}{},
			expectError:    true,
			expectedErrMsg: "failed to list global security advisories",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListGlobalSecurityAdvisories(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := GetGlobalSecurityAdvisory(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_global_security_advisory", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "ghsaId")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"ghsaId"})

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetAdvisoriesByGhsaId,
					mockAdvisory,
				),
			),
			requestArgs: map[string]interface{}{
				"ghsaId": "GHSA-xxxx-xxxx-xxxx",
			},
			expectError:      false,
			expectedAdvisory: mockAdvisory,
		},
		{
			name: "invalid ghsaId format",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetAdvisoriesByGhsaId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Bad Request"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"ghsaId": "invalid-ghsa-id",
			},
			expectError:    true,
			expectedErrMsg: "failed to get advisory",
		},
		{
			name: "advisory not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetAdvisoriesByGhsaId,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
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
			_, handler := GetGlobalSecurityAdvisory(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListRepositorySecurityAdvisories(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_repository_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "direction")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

	// Local endpoint pattern for repository security advisories
	var GetReposSecurityAdvisoriesByOwnerByRepo = mock.EndpointPattern{
		Pattern: "/repos/{owner}/{repo}/security-advisories",
		Method:  "GET",
	}

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposSecurityAdvisoriesByOwnerByRepo,
					expect(t, expectations{
						path:        "/repos/owner/repo/security-advisories",
						queryParams: map[string]string{},
					}).andThen(
						mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1, adv2}),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1, adv2},
		},
		{
			name: "successful advisories listing with filters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposSecurityAdvisoriesByOwnerByRepo,
					expect(t, expectations{
						path: "/repos/octo/hello-world/security-advisories",
						queryParams: map[string]string{
							"direction": "desc",
							"sort":      "updated",
							"state":     "published",
						},
					}).andThen(
						mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1}),
					),
				),
			),
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposSecurityAdvisoriesByOwnerByRepo,
					expect(t, expectations{
						path:        "/repos/owner/repo/security-advisories",
						queryParams: map[string]string{},
					}).andThen(
						mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"}),
					),
				),
			),
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
			_, handler := ListRepositorySecurityAdvisories(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListOrgRepositorySecurityAdvisories(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_org_repository_security_advisories", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "direction")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org"})

	// Endpoint pattern for org repository security advisories
	var GetOrgsSecurityAdvisoriesByOrg = mock.EndpointPattern{
		Pattern: "/orgs/{org}/security-advisories",
		Method:  "GET",
	}

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetOrgsSecurityAdvisoriesByOrg,
					expect(t, expectations{
						path:        "/orgs/octo/security-advisories",
						queryParams: map[string]string{},
					}).andThen(
						mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1, adv2}),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"org": "octo",
			},
			expectError:        false,
			expectedAdvisories: []*github.SecurityAdvisory{adv1, adv2},
		},
		{
			name: "successful listing with filters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetOrgsSecurityAdvisoriesByOrg,
					expect(t, expectations{
						path: "/orgs/octo/security-advisories",
						queryParams: map[string]string{
							"direction": "asc",
							"sort":      "created",
							"state":     "triage",
						},
					}).andThen(
						mockResponse(t, http.StatusOK, []*github.SecurityAdvisory{adv1}),
					),
				),
			),
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetOrgsSecurityAdvisoriesByOrg,
					expect(t, expectations{
						path:        "/orgs/octo/security-advisories",
						queryParams: map[string]string{},
					}).andThen(
						mockResponse(t, http.StatusForbidden, map[string]string{"message": "Forbidden"}),
					),
				),
			),
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
			_, handler := ListOrgRepositorySecurityAdvisories(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)

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
