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
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// verifyDeletionSuccess is a helper function to verify deletion operation success.
func verifyDeletionSuccess(t *testing.T, result *mcp.CallToolResult, err error) {
	t.Helper()

	require.NoError(t, err)
	require.False(t, result.IsError)

	textContent := getTextResult(t, result)
	var response map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.True(t, response["success"].(bool))
	assert.Contains(t, response["message"].(string), "deleted successfully")
}

func Test_PackagesRead(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := PackagesRead(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "packages_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "PackagesRead tool should be read-only")
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "method")
	assert.ElementsMatch(t, schema.Required, []string{"method"})
}

func Test_PackagesRead_ListOrgPackages(t *testing.T) {
	mockPackages := []*github.Package{
		{
			ID:          github.Ptr(int64(1)),
			Name:        github.Ptr("github-mcp-server"),
			PackageType: github.Ptr("container"),
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
			name: "successful list",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"method": "list_org_packages",
				"org":    "github",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name: "successful list with package_type filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"method":       "list_org_packages",
				"org":          "github",
				"package_type": "container",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name: "successful list with visibility filter",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.EndpointPattern{Pattern: "/orgs/{org}/packages", Method: "GET"},
					mockPackages,
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "list_org_packages",
				"org":        "github",
				"visibility": "public",
			},
			expectError:      false,
			expectedPackages: mockPackages,
		},
		{
			name:         "missing org parameter",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method": "list_org_packages",
			},
			expectError: true,
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
				"method": "list_org_packages",
				"org":    "nonexistent-org",
			},
			expectError:    true,
			expectedErrMsg: "failed to list packages",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := PackagesRead(stubGetClientFn(client), translations.NullTranslationHelper)
			result, _, err := handler(context.Background(), nil, tc.requestArgs)

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

			// Parse and verify the result
			textContent := getTextResult(t, result)
			var returnedPackages []*github.Package
			err = json.Unmarshal([]byte(textContent.Text), &returnedPackages)
			require.NoError(t, err)

			assert.Len(t, returnedPackages, len(tc.expectedPackages))
			for i, pkg := range returnedPackages {
				assert.Equal(t, *tc.expectedPackages[i].ID, *pkg.ID)
				assert.Equal(t, *tc.expectedPackages[i].Name, *pkg.Name)
				assert.Equal(t, *tc.expectedPackages[i].PackageType, *pkg.PackageType)
				assert.Equal(t, *tc.expectedPackages[i].Visibility, *pkg.Visibility)
			}
		})
	}
}

func Test_PackagesRead_GetOrgPackage(t *testing.T) {
	mockPackage := &github.Package{
		ID:   github.Ptr(int64(1)),
		Name: github.Ptr("test-package"),
	}

	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "GET"},
			mockPackage,
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesRead(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":       "get_org_package",
		"org":          "github",
		"package_type": "container",
		"package_name": "test-package",
	})

	require.NoError(t, err)
	require.False(t, result.IsError)
}

func Test_PackagesRead_ListPackageVersions(t *testing.T) {
	mockVersions := []*github.PackageVersion{
		{ID: github.Ptr(int64(123)), Name: github.Ptr("v1.0.0")},
	}

	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions", Method: "GET"},
			mockVersions,
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesRead(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":       "list_package_versions",
		"org":          "github",
		"package_type": "container",
		"package_name": "test-package",
	})

	require.NoError(t, err)
	require.False(t, result.IsError)
}

func Test_PackagesRead_GetPackageVersion(t *testing.T) {
	mockVersion := &github.PackageVersion{
		ID:   github.Ptr(int64(123)),
		Name: github.Ptr("v1.0.0"),
	}

	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "GET"},
			mockVersion,
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesRead(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":             "get_package_version",
		"org":                "github",
		"package_type":       "container",
		"package_name":       "test-package",
		"package_version_id": float64(123),
	})

	require.NoError(t, err)
	require.False(t, result.IsError)
}

func Test_PackagesRead_ListUserPackages(t *testing.T) {
	mockPackages := []*github.Package{
		{ID: github.Ptr(int64(1)), Name: github.Ptr("user-package")},
	}

	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.EndpointPattern{Pattern: "/users/{username}/packages", Method: "GET"},
			mockPackages,
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesRead(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":   "list_user_packages",
		"username": "testuser",
	})

	require.NoError(t, err)
	require.False(t, result.IsError)
}

func Test_PackagesWrite(t *testing.T) {
	mockClient := github.NewClient(nil)
	tool, _ := PackagesWrite(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "packages_write", tool.Name)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.ElementsMatch(t, schema.Required, []string{"method", "package_type", "package_name"})
	assert.False(t, tool.Annotations.ReadOnlyHint, "PackagesWrite tool should not be read-only")
}

func Test_PackagesWrite_DeleteOrgPackage(t *testing.T) {
	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}", Method: "DELETE"},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesWrite(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":       "delete_org_package",
		"org":          "github",
		"package_type": "container",
		"package_name": "test-package",
	})

	verifyDeletionSuccess(t, result, err)
}

func Test_PackagesWrite_DeleteOrgPackageVersion(t *testing.T) {
	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesWrite(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":             "delete_org_package_version",
		"org":                "github",
		"package_type":       "container",
		"package_name":       "test-package",
		"package_version_id": float64(123),
	})

	verifyDeletionSuccess(t, result, err)
}

func Test_PackagesWrite_DeleteUserPackage(t *testing.T) {
	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}", Method: "DELETE"},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesWrite(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":       "delete_user_package",
		"package_type": "container",
		"package_name": "test-package",
	})

	verifyDeletionSuccess(t, result, err)
}

func Test_PackagesWrite_DeleteUserPackageVersion(t *testing.T) {
	client := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/user/packages/{package_type}/{package_name}/versions/{package_version_id}", Method: "DELETE"},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		),
	)

	ghClient := github.NewClient(client)
	_, handler := PackagesWrite(stubGetClientFn(ghClient), translations.NullTranslationHelper)

	result, _, err := handler(context.Background(), nil, map[string]interface{}{
		"method":             "delete_user_package_version",
		"package_type":       "container",
		"package_name":       "test-package",
		"package_version_id": float64(123),
	})

	verifyDeletionSuccess(t, result, err)
}
