package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_PackagesRead(t *testing.T) {
	toolDef := PackagesRead(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "packages_read", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"method"})
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")

	mockPackages := []*github.Package{{
		ID:          new(int64(1)),
		Name:        new("github-mcp-server"),
		PackageType: new("container"),
		Visibility:  new("public"),
	}}
	mockPackage := mockPackages[0]
	mockVersions := []*github.PackageVersion{
		{ID: new(int64(101)), Name: new("sha256:abc")},
		{ID: new(int64(102)), Name: new("sha256:def")},
	}
	mockVersion := mockVersions[0]

	expectQuery := func(t *testing.T, expected map[string]string, body any) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			for k, v := range expected {
				assert.Equal(t, v, r.URL.Query().Get(k), "query parameter %q", k)
			}
			mockResponse(t, http.StatusOK, body)(w, r)
		}
	}

	tests := []struct {
		name           string
		handlers       map[string]http.HandlerFunc
		args           map[string]any
		expectError    bool
		expectedErrMsg string
		expectedResult any
	}{
		{
			name: "list_org_packages with filters and pagination",
			handlers: map[string]http.HandlerFunc{
				"GET /orgs/{org}/packages": expectQuery(t, map[string]string{
					"package_type": "container",
					"visibility":   "public",
					"page":         "2",
					"per_page":     "10",
				}, mockPackages),
			},
			args: map[string]any{
				"method":       "list_org_packages",
				"org":          "github",
				"package_type": "container",
				"visibility":   "public",
				"page":         float64(2),
				"perPage":      float64(10),
			},
			expectedResult: mockPackages,
		},
		{
			name:           "list_org_packages requires org",
			handlers:       map[string]http.HandlerFunc{},
			args:           map[string]any{"method": "list_org_packages"},
			expectError:    true,
			expectedErrMsg: "missing required parameter: org",
		},
		{
			name: "list_org_packages API error",
			handlers: map[string]http.HandlerFunc{
				"GET /orgs/{org}/packages": mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			},
			args:           map[string]any{"method": "list_org_packages", "org": "missing"},
			expectError:    true,
			expectedErrMsg: "failed to list packages for organization 'missing'",
		},
		{
			name: "list_user_packages for authenticated user",
			handlers: map[string]http.HandlerFunc{
				"GET /user/packages": mockResponse(t, http.StatusOK, mockPackages),
			},
			args:           map[string]any{"method": "list_user_packages"},
			expectedResult: mockPackages,
		},
		{
			name: "list_user_packages for named user",
			handlers: map[string]http.HandlerFunc{
				"GET /users/{username}/packages": expectQuery(t, map[string]string{"package_type": "npm"}, mockPackages),
			},
			args:           map[string]any{"method": "list_user_packages", "username": "octocat", "package_type": "npm"},
			expectedResult: mockPackages,
		},
		{
			name: "get_org_package",
			handlers: map[string]http.HandlerFunc{
				"GET /orgs/{org}/packages/{package_type}/{package_name}": mockResponse(t, http.StatusOK, mockPackage),
			},
			args: map[string]any{
				"method":       "get_org_package",
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
			},
			expectedResult: mockPackage,
		},
		{
			name:           "get_org_package requires package_name",
			handlers:       map[string]http.HandlerFunc{},
			args:           map[string]any{"method": "get_org_package", "org": "github", "package_type": "container"},
			expectError:    true,
			expectedErrMsg: "missing required parameter: package_name",
		},
		{
			name: "get_user_package for named user",
			handlers: map[string]http.HandlerFunc{
				"GET /users/{username}/packages/{package_type}/{package_name}": mockResponse(t, http.StatusOK, mockPackage),
			},
			args: map[string]any{
				"method":       "get_user_package",
				"username":     "octocat",
				"package_type": "container",
				"package_name": "github-mcp-server",
			},
			expectedResult: mockPackage,
		},
		{
			name: "get_user_package API error",
			handlers: map[string]http.HandlerFunc{
				"GET /user/packages/{package_type}/{package_name}": mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			},
			args: map[string]any{
				"method":       "get_user_package",
				"package_type": "npm",
				"package_name": "missing",
			},
			expectError:    true,
			expectedErrMsg: "failed to get npm package 'missing' for the authenticated user",
		},
		{
			name: "list_org_package_versions with state",
			handlers: map[string]http.HandlerFunc{
				"GET /orgs/{org}/packages/{package_type}/{package_name}/versions": expectQuery(t, map[string]string{"state": "deleted"}, mockVersions),
			},
			args: map[string]any{
				"method":       "list_org_package_versions",
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
				"state":        "deleted",
			},
			expectedResult: mockVersions,
		},
		{
			name: "list_user_package_versions for authenticated user",
			handlers: map[string]http.HandlerFunc{
				"GET /user/packages/{package_type}/{package_name}/versions": expectQuery(t, map[string]string{"state": "active"}, mockVersions),
			},
			args: map[string]any{
				"method":       "list_user_package_versions",
				"package_type": "container",
				"package_name": "github-mcp-server",
				"state":        "active",
			},
			expectedResult: mockVersions,
		},
		{
			name: "list_user_package_versions for named user",
			handlers: map[string]http.HandlerFunc{
				"GET /users/{username}/packages/{package_type}/{package_name}/versions": expectQuery(t, map[string]string{
					"state":    "active",
					"page":     "2",
					"per_page": "5",
				}, mockVersions),
			},
			args: map[string]any{
				"method":       "list_user_package_versions",
				"username":     "octocat",
				"package_type": "container",
				"package_name": "github-mcp-server",
				"state":        "active",
				"page":         float64(2),
				"perPage":      float64(5),
			},
			expectedResult: mockVersions,
		},
		{
			name: "get_org_package_version",
			handlers: map[string]http.HandlerFunc{
				"GET /orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}": mockResponse(t, http.StatusOK, mockVersion),
			},
			args: map[string]any{
				"method":             "get_org_package_version",
				"org":                "github",
				"package_type":       "container",
				"package_name":       "github-mcp-server",
				"package_version_id": float64(101),
			},
			expectedResult: mockVersion,
		},
		{
			name:     "get_org_package_version requires package_version_id",
			handlers: map[string]http.HandlerFunc{},
			args: map[string]any{
				"method":       "get_org_package_version",
				"org":          "github",
				"package_type": "container",
				"package_name": "github-mcp-server",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: package_version_id",
		},
		{
			name: "get_user_package_version for authenticated user",
			handlers: map[string]http.HandlerFunc{
				"GET /user/packages/{package_type}/{package_name}/versions/{package_version_id}": mockResponse(t, http.StatusOK, mockVersion),
			},
			args: map[string]any{
				"method":             "get_user_package_version",
				"package_type":       "container",
				"package_name":       "github-mcp-server",
				"package_version_id": float64(101),
			},
			expectedResult: mockVersion,
		},
		{
			name:           "unknown method",
			handlers:       map[string]http.HandlerFunc{},
			args:           map[string]any{"method": "invalid"},
			expectError:    true,
			expectedErrMsg: "unknown method: invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tc.handlers))}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.args)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError, getTextResult(t, result).Text)
			expected, err := json.Marshal(tc.expectedResult)
			require.NoError(t, err)
			assert.JSONEq(t, string(expected), getTextResult(t, result).Text)
		})
	}
}

func Test_PackagesWrite(t *testing.T) {
	toolDef := PackagesWrite(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "packages_write", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)
	require.NotNil(t, toolDef.Tool.Annotations.DestructiveHint)
	assert.True(t, *toolDef.Tool.Annotations.DestructiveHint)
	assert.ElementsMatch(t, toolDef.ScopeAccess.Scopes, []string{"read:packages", "delete:packages"})

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"method", "package_type", "package_name"})

	noContent := mockResponse(t, http.StatusNoContent, "")

	tests := []struct {
		name            string
		handlers        map[string]http.HandlerFunc
		args            map[string]any
		expectError     bool
		expectedErrMsg  string
		expectedMessage string
	}{
		{
			name: "delete_org_package",
			handlers: map[string]http.HandlerFunc{
				"DELETE /orgs/{org}/packages/{package_type}/{package_name}": noContent,
			},
			args: map[string]any{
				"method":       "delete_org_package",
				"org":          "github",
				"package_type": "container",
				"package_name": "old-image",
			},
			expectedMessage: "Deleted container package 'old-image' for organization 'github'",
		},
		{
			name:     "delete_org_package requires org",
			handlers: map[string]http.HandlerFunc{},
			args: map[string]any{
				"method":       "delete_org_package",
				"package_type": "container",
				"package_name": "old-image",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: org",
		},
		{
			name: "delete_org_package API error",
			handlers: map[string]http.HandlerFunc{
				"DELETE /orgs/{org}/packages/{package_type}/{package_name}": mockResponse(t, http.StatusForbidden, `{"message": "Forbidden"}`),
			},
			args: map[string]any{
				"method":       "delete_org_package",
				"org":          "github",
				"package_type": "container",
				"package_name": "old-image",
			},
			expectError:    true,
			expectedErrMsg: "failed to delete container package 'old-image' for organization 'github'",
		},
		{
			name: "delete_org_package unexpected status",
			handlers: map[string]http.HandlerFunc{
				"DELETE /orgs/{org}/packages/{package_type}/{package_name}": mockResponse(t, http.StatusOK, `{"message": "unexpected"}`),
			},
			args: map[string]any{
				"method":       "delete_org_package",
				"org":          "github",
				"package_type": "container",
				"package_name": "old-image",
			},
			expectError:    true,
			expectedErrMsg: "unexpected status 200",
		},
		{
			name: "delete_org_package_version",
			handlers: map[string]http.HandlerFunc{
				"DELETE /orgs/{org}/packages/{package_type}/{package_name}/versions/{package_version_id}": noContent,
			},
			args: map[string]any{
				"method":             "delete_org_package_version",
				"org":                "github",
				"package_type":       "container",
				"package_name":       "old-image",
				"package_version_id": float64(101),
			},
			expectedMessage: "Deleted version 101 of container package 'old-image' for organization 'github'",
		},
		{
			name:     "delete_org_package_version requires package_version_id",
			handlers: map[string]http.HandlerFunc{},
			args: map[string]any{
				"method":       "delete_org_package_version",
				"org":          "github",
				"package_type": "container",
				"package_name": "old-image",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: package_version_id",
		},
		{
			name: "delete_user_package for authenticated user",
			handlers: map[string]http.HandlerFunc{
				"DELETE /user/packages/{package_type}/{package_name}": noContent,
			},
			args: map[string]any{
				"method":       "delete_user_package",
				"package_type": "npm",
				"package_name": "old-lib",
			},
			expectedMessage: "Deleted npm package 'old-lib' for the authenticated user",
		},
		{
			name: "delete_user_package for named user",
			handlers: map[string]http.HandlerFunc{
				"DELETE /users/{username}/packages/{package_type}/{package_name}": noContent,
			},
			args: map[string]any{
				"method":       "delete_user_package",
				"username":     "octocat",
				"package_type": "npm",
				"package_name": "old-lib",
			},
			expectedMessage: "Deleted npm package 'old-lib' for user 'octocat'",
		},
		{
			name: "delete_user_package_version",
			handlers: map[string]http.HandlerFunc{
				"DELETE /user/packages/{package_type}/{package_name}/versions/{package_version_id}": noContent,
			},
			args: map[string]any{
				"method":             "delete_user_package_version",
				"package_type":       "npm",
				"package_name":       "old-lib",
				"package_version_id": float64(7),
			},
			expectedMessage: "Deleted version 7 of npm package 'old-lib' for the authenticated user",
		},
		{
			name:     "unknown method",
			handlers: map[string]http.HandlerFunc{},
			args: map[string]any{
				"method":       "invalid",
				"package_type": "npm",
				"package_name": "old-lib",
			},
			expectError:    true,
			expectedErrMsg: "unknown method: invalid",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(tc.handlers))}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(tc.args)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError, getTextResult(t, result).Text)
			var response map[string]any
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &response))
			assert.Equal(t, true, response["success"])
			assert.Equal(t, tc.expectedMessage, response["message"])
		})
	}
}
