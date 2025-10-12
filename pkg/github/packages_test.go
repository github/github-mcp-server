package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verifyDeletionSuccess is a helper function to verify deletion operation success.
// It checks that the result is not an error, parses the JSON response, and verifies
// the success status and message content.
func verifyDeletionSuccess(t *testing.T, result *mcp.CallToolResult, err error) {
	t.Helper()

	require.NoError(t, err)
	require.False(t, result.IsError)

	// Parse the success result
	textContent := getTextResult(t, result)
	var response map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.Contains(t, response["message"].(string), "deleted successfully")
}

func Test_ListOrgPackages(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := ListOrgPackages(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_org_packages", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "visibility")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org"})

	// Setup mock packages for success case
	mockPackages := []*github.Package{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("github-mcp-server"),
			PackageType: github.Ptr("container"),
			HTMLURL:     github.Ptr("https://github.com/orgs/github/packages/container/package/github-mcp-server"),
			Visibility:  github.Ptr("public"),
		},
		{
			ID:          github.Ptr(int64(2)),
			Name:        github.Ptr("test-package"),
			PackageType: github.Ptr("npm"),
			HTMLURL:     github.Ptr("https://github.com/orgs/github/packages/npm/package/test-package"),
			Visibility:  github.Ptr("private"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedPackages []*github.Package
		expectedErrMsg   string
	}{
		{
			name: "successful packages listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"org": "github",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name: "successful packages listing with filters",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"visibility":   "public",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name: "organization not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to list packages",
		},
		{
			name:         "missing required parameter org",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs:  map[string]interface{}{},
			expectError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListOrgPackages(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result
			textContent := getTextResult(t, result)
			var returnedPackages []*github.Package
			err = json.Unmarshal([]byte(textContent.Text), &returnedPackages)
			require.NoError(t, err)

			assert.Equal(t, len(tc.expectedPackages), len(returnedPackages))
			for i, pkg := range returnedPackages {
				assert.Equal(t, tc.expectedPackages[i].GetID(), pkg.GetID())
				assert.Equal(t, tc.expectedPackages[i].GetName(), pkg.GetName())
			}
		})
	}
}

func Test_GetOrgPackage(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := GetOrgPackage(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_org_package", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org", "package_type", "package_name"})

	// Setup mock package for success case
	mockPackage := &github.Package{
		ID:          github.Ptr(int64(1)),
		Name:        github.Ptr("github-mcp-server"),
		PackageType: github.Ptr("container"),
		HTMLURL:     github.Ptr("https://github.com/orgs/github/packages/container/package/github-mcp-server"),
		Visibility:  github.Ptr("public"),
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedPackage *github.Package
		expectedErrMsg  string
	}{
		{
			name: "successful package retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "GET"},
					mockPackage,
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
			},
			expectError:     false,
			expectedPackage: mockPackage,
		},
		{
			name: "package not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "GET"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Package not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get package",
		},
		{
			name:         "missing required parameters",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"org": "github",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetOrgPackage(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result
			textContent := getTextResult(t, result)
			var returnedPackage github.Package
			err = json.Unmarshal([]byte(textContent.Text), &returnedPackage)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedPackage.GetID(), returnedPackage.GetID())
			assert.Equal(t, tc.expectedPackage.GetName(), returnedPackage.GetName())
			assert.Equal(t, tc.expectedPackage.GetPackageType(), returnedPackage.GetPackageType())
		})
	}
}

func Test_ListPackageVersions(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := ListPackageVersions(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_package_versions", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org", "package_type", "package_name"})

	// Setup mock package versions for success case
	mockVersions := []*github.PackageVersion{
		{
			ID:   github.Ptr(int64(123)),
			Name: github.Ptr("v1.0.0"),
		},
		{
			ID:   github.Ptr(int64(124)),
			Name: github.Ptr("v1.0.1"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedVersions []*github.PackageVersion
		expectedErrMsg   string
	}{
		{
			name: "successful versions listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions", Method: "GET"},
					mockVersions,
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
			},
			expectError:      false,
			expectedVersions: mockVersions,
		},
		{
			name: "successful versions listing with state filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions", Method: "GET"},
					mockVersions,
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
				"state":        "active",
			},
			expectError:      false,
			expectedVersions: mockVersions,
		},
		{
			name: "package not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions", Method: "GET"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to list package versions",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListPackageVersions(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result
			textContent := getTextResult(t, result)
			var returnedVersions []*github.PackageVersion
			err = json.Unmarshal([]byte(textContent.Text), &returnedVersions)
			require.NoError(t, err)

			assert.Equal(t, len(tc.expectedVersions), len(returnedVersions))
		})
	}
}

func Test_GetPackageVersion(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := GetPackageVersion(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_package_version", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.Contains(t, tool.InputSchema.Properties, "package_version_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org", "package_type", "package_name", "package_version_id"})

	// Setup mock package version for success case
	mockVersion := &github.PackageVersion{
		ID:   github.Ptr(int64(123)),
		Name: github.Ptr("v1.0.0"),
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedVersion *github.PackageVersion
		expectedErrMsg  string
	}{
		{
			name: "successful version retrieval",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "GET"},
					mockVersion,
				),
			),
			requestArgs: map[string]interface{}{
				"org":                "github",
				"package_type":       "container",
				"package_name":       "github-mcp-server",
				"package_version_id": float64(123),
			},
			expectError:     false,
			expectedVersion: mockVersion,
		},
		{
			name: "version not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "GET"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":                "github",
				"package_type":       "container",
				"package_name":       "github-mcp-server",
				"package_version_id": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get package version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPackageVersion(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result
			textContent := getTextResult(t, result)
			var returnedVersion github.PackageVersion
			err = json.Unmarshal([]byte(textContent.Text), &returnedVersion)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedVersion.GetID(), returnedVersion.GetID())
			assert.Equal(t, tc.expectedVersion.GetName(), returnedVersion.GetName())
		})
	}
}

func Test_ListUserPackages(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := ListUserPackages(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_user_packages", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "username")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "visibility")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"username"})

	// Setup mock packages for success case
	mockPackages := []*github.Package{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("my-package"),
			PackageType: github.Ptr("npm"),
			Visibility:  github.Ptr("public"),
		},
	}

	tests := []struct {
		name             string
		mockedClient     *http.Client
		requestArgs      map[string]interface{}
		expectError      bool
		expectedPackages []*github.Package
		expectedErrMsg   string
	}{
		{
			name: "successful user packages listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/users/{username}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"username": "octocat",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name: "user not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{username}/packages", Method: "GET"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"username": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to list packages",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := ListUserPackages(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result
			textContent := getTextResult(t, result)
			var returnedPackages []*github.Package
			err = json.Unmarshal([]byte(textContent.Text), &returnedPackages)
			require.NoError(t, err)

			assert.Equal(t, len(tc.expectedPackages), len(returnedPackages))
		})
	}
}

func Test_DeleteOrgPackage(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := DeleteOrgPackage(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_org_package", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org", "package_type", "package_name"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful package deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "test-package",
			},
			expectError: false,
		},
		{
			name: "package not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Package not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to delete package",
		},
		{
			name: "insufficient permissions",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message": "Forbidden - requires delete:packages scope"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":          "github",
				"package_type": "container",
				"package_name": "test-package",
			},
			expectError:    true,
			expectedErrMsg: "failed to delete package",
		},
		{
			name:         "missing required parameters",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"org": "github",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteOrgPackage(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			verifyDeletionSuccess(t, result, err)
		})
	}
}

func Test_DeleteOrgPackageVersion(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := DeleteOrgPackageVersion(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_org_package_version", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.Contains(t, tool.InputSchema.Properties, "package_version_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org", "package_type", "package_name", "package_version_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful version deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":                "github",
				"package_type":       "container",
				"package_name":       "test-package",
				"package_version_id": float64(123),
			},
			expectError: false,
		},
		{
			name: "version not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Version not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":                "github",
				"package_type":       "container",
				"package_name":       "test-package",
				"package_version_id": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to delete package version",
		},
		{
			name: "insufficient permissions",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message": "Forbidden - requires delete:packages scope"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org":                "github",
				"package_type":       "container",
				"package_name":       "test-package",
				"package_version_id": float64(123),
			},
			expectError:    true,
			expectedErrMsg: "failed to delete package version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteOrgPackageVersion(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			verifyDeletionSuccess(t, result, err)
		})
	}
}

func Test_DeleteUserPackage(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := DeleteUserPackage(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_user_package", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"package_type", "package_name"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful user package deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"package_type": "npm",
				"package_name": "my-package",
			},
			expectError: false,
		},
		{
			name: "package not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Package not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"package_type": "npm",
				"package_name": "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to delete package",
		},
		{
			name:         "missing required parameters",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"package_type": "npm",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteUserPackage(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			verifyDeletionSuccess(t, result, err)
		})
	}
}

func Test_DeleteUserPackageVersion(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := DeleteUserPackageVersion(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_user_package_version", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "package_type")
	assert.Contains(t, tool.InputSchema.Properties, "package_name")
	assert.Contains(t, tool.InputSchema.Properties, "package_version_id")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"package_type", "package_name", "package_version_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "successful user version deletion",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"package_type":       "npm",
				"package_name":       "my-package",
				"package_version_id": float64(123),
			},
			expectError: false,
		},
		{
			name: "version not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Version not found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"package_type":       "npm",
				"package_name":       "my-package",
				"package_version_id": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to delete version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			_, handler := DeleteUserPackageVersion(stubGetClientFn(client), translations.NullTranslationHelper)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
			result, err := handler(context.Background(), request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				if tc.expectedErrMsg != "" {
					errorContent := getErrorResult(t, result)
					assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				}
				return
			}

			verifyDeletionSuccess(t, result, err)
		})
	}
}
