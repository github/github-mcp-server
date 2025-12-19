package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListProjects(t *testing.T) {
	serverTool := ListProjects(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_projects", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "per_page")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "owner_type"})

	// API returns full ProjectV2 objects; we only need minimal fields for decoding.
	orgProjects := []map[string]any{{"id": 1, "node_id": "NODE1", "title": "Org Project"}}
	userProjects := []map[string]any{{"id": 2, "node_id": "NODE2", "title": "User Project"}}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLength int
		expectedErrMsg string
	}{
		{
			name: "success organization",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(orgProjects))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "success user",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{username}/projectsV2", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(userProjects))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octocat",
				"owner_type": "user",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "success organization with pagination & query",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						if q.Get("per_page") == "50" && q.Get("q") == "roadmap" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(orgProjects))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
				"per_page":   float64(50),
				"query":      "roadmap",
			},
			expectError:    false,
			expectedLength: 1,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "failed to list projects",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner_type": "org",
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner": "octo-org",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			projects, ok := response["projects"].([]interface{})
			require.True(t, ok)
			assert.Equal(t, tc.expectedLength, len(projects))
			// pageInfo should exist
			_, hasPageInfo := response["pageInfo"].(map[string]interface{})
			assert.True(t, hasPageInfo)
		})
	}
}

func Test_GetProject(t *testing.T) {
	serverTool := GetProject(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_project", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.ElementsMatch(t, schema.Required, []string{"project_number", "owner", "owner_type"})

	project := map[string]any{"id": 123, "title": "Project Title"}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "success organization project fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/123", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, project),
				),
			),
			requestArgs: map[string]interface{}{
				"project_number": float64(123),
				"owner":          "octo-org",
				"owner_type":     "org",
			},
			expectError: false,
		},
		{
			name: "success user project fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{username}/projectsV2/456", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, project),
				),
			),
			requestArgs: map[string]interface{}{
				"project_number": float64(456),
				"owner":          "octocat",
				"owner_type":     "user",
			},
			expectError: false,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/999", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]interface{}{
				"project_number": float64(999),
				"owner":          "octo-org",
				"owner_type":     "org",
			},
			expectError:    true,
			expectedErrMsg: "failed to get project",
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError: true,
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"project_number": float64(123),
				"owner_type":     "org",
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"project_number": float64(123),
				"owner":          "octo-org",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing project_number" {
					assert.Contains(t, text, "missing required parameter: project_number")
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var arr map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &arr)
			require.NoError(t, err)
		})
	}
}

func Test_ListProjectFields(t *testing.T) {
	serverTool := ListProjectFields(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_project_fields", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "per_page")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number"})

	orgFields := []map[string]any{{"id": 101, "name": "Status", "data_type": "single_select"}}
	userFields := []map[string]any{{"id": 201, "name": "Priority", "data_type": "single_select"}}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLength int
		expectedErrMsg string
	}{
		{
			name: "success organization fields",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/fields", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(orgFields))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
			},
			expectedLength: 1,
		},
		{
			name: "success user fields with per_page override",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/fields", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						if q.Get("per_page") == "50" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(userFields))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(456),
				"per_page":       float64(50),
			},
			expectedLength: 1,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/fields", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(789),
			},
			expectError:    true,
			expectedErrMsg: "failed to list project fields",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner_type":     "org",
				"project_number": 10,
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"project_number": 10,
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				if tc.name == "missing project_number" {
					assert.Contains(t, text, "missing required parameter: project_number")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			fields, ok := response["fields"].([]interface{})
			require.True(t, ok)
			assert.Equal(t, tc.expectedLength, len(fields))
			_, hasPageInfo := response["pageInfo"].(map[string]interface{})
			assert.True(t, hasPageInfo)
		})
	}
}

func Test_GetProjectField(t *testing.T) {
	serverTool := GetProjectField(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_project_field", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "field_id")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number", "field_id"})

	orgField := map[string]any{"id": 101, "name": "Status", "dataType": "single_select"}
	userField := map[string]any{"id": 202, "name": "Priority", "dataType": "single_select"}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectedID     int
	}{
		{
			name: "success organization field",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/fields/{field_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgField),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"field_id":       float64(101),
			},
			expectedID: 101,
		},
		{
			name: "success user field",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/fields/{field_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, userField),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(456),
				"field_id":       float64(202),
			},
			expectedID: 202,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/fields/{field_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(789),
				"field_id":       float64(303),
			},
			expectError:    true,
			expectedErrMsg: "failed to get project field",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner_type":     "org",
				"project_number": float64(10),
				"field_id":       float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"project_number": float64(10),
				"field_id":       float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
				"field_id":   float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing field_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(10),
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				if tc.name == "missing project_number" {
					assert.Contains(t, text, "missing required parameter: project_number")
				}
				if tc.name == "missing field_id" {
					assert.Contains(t, text, "missing required parameter: field_id")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var field map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &field)
			require.NoError(t, err)
			if tc.expectedID != 0 {
				assert.Equal(t, float64(tc.expectedID), field["id"])
			}
		})
	}
}

func Test_ListProjectItems(t *testing.T) {
	serverTool := ListProjectItems(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_project_items", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "query")
	assert.Contains(t, schema.Properties, "per_page")
	assert.Contains(t, schema.Properties, "fields")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number"})

	orgItems := []map[string]any{
		{"id": 301, "content_type": "Issue", "project_node_id": "PR_1", "fields": []map[string]any{
			{"id": 123, "name": "Status", "data_type": "single_select", "value": "value1"},
			{"id": 456, "name": "Priority", "data_type": "single_select", "value": "value2"},
		}},
	}
	userItems := []map[string]any{
		{"id": 401, "content_type": "PullRequest", "project_node_id": "PR_2"},
		{"id": 402, "content_type": "DraftIssue", "project_node_id": "PR_3"},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedLength int
		expectedErrMsg string
	}{
		{
			name: "success organization items",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgItems),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
			},
			expectedLength: 1,
		},
		{
			name: "success organization items with fields",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						if q.Get("fields") == "123,456,789" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(orgItems))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"fields":         []interface{}{"123", "456", "789"},
			},
			expectedLength: 1,
		},
		{
			name: "success user items",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/items", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, userItems),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(456),
			},
			expectedLength: 2,
		},
		{
			name: "success with pagination and query",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						if q.Get("per_page") == "50" && q.Get("q") == "bug" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(orgItems))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"per_page":       float64(50),
				"query":          "bug",
			},
			expectedLength: 1,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(789),
			},
			expectError:    true,
			expectedErrMsg: ProjectListFailedError,
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner_type":     "org",
				"project_number": float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"project_number": float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				if tc.name == "missing project_number" {
					assert.Contains(t, text, "missing required parameter: project_number")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var response map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &response)
			require.NoError(t, err)
			items, ok := response["items"].([]interface{})
			require.True(t, ok)
			assert.Equal(t, tc.expectedLength, len(items))
			_, hasPageInfo := response["pageInfo"].(map[string]interface{})
			assert.True(t, hasPageInfo)
		})
	}
}

func Test_GetProjectItem(t *testing.T) {
	serverTool := GetProjectItem(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_project_item", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "item_id")
	assert.Contains(t, schema.Properties, "fields")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number", "item_id"})

	orgItem := map[string]any{
		"id":              301,
		"content_type":    "Issue",
		"project_node_id": "PR_1",
		"creator":         map[string]any{"login": "octocat"},
	}
	userItem := map[string]any{
		"id":              501,
		"content_type":    "PullRequest",
		"project_node_id": "PR_2",
		"creator":         map[string]any{"login": "jane"},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectedID     int
	}{
		{
			name: "success organization item",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgItem),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"item_id":        float64(301),
			},
			expectedID: 301,
		},
		{
			name: "success organization item with fields",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodGet},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						q := r.URL.Query()
						if q.Get("fields") == "123,456" {
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(mock.MustMarshal(orgItem))
							return
						}
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message":"unexpected query params"}`))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"item_id":        float64(301),
				"fields":         []interface{}{"123", "456"},
			},
			expectedID: 301,
		},
		{
			name: "success user item",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/items/{item_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, userItem),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(456),
				"item_id":        float64(501),
			},
			expectedID: 501,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(789),
				"item_id":        float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get project item",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner_type":     "org",
				"project_number": float64(10),
				"item_id":        float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"project_number": float64(10),
				"item_id":        float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
				"item_id":    float64(1),
			},
			expectError: true,
		},
		{
			name:         "missing item_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(10),
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				if tc.name == "missing owner" {
					assert.Contains(t, text, "missing required parameter: owner")
				}
				if tc.name == "missing owner_type" {
					assert.Contains(t, text, "missing required parameter: owner_type")
				}
				if tc.name == "missing project_number" {
					assert.Contains(t, text, "missing required parameter: project_number")
				}
				if tc.name == "missing item_id" {
					assert.Contains(t, text, "missing required parameter: item_id")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var item map[string]any
			err = json.Unmarshal([]byte(textContent.Text), &item)
			require.NoError(t, err)
			if tc.expectedID != 0 {
				assert.Equal(t, float64(tc.expectedID), item["id"])
			}
		})
	}
}

func Test_AddProjectItem(t *testing.T) {
	serverTool := AddProjectItem(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "add_project_item", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "item_type")
	assert.Contains(t, schema.Properties, "item_id")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number", "item_type", "item_id"})

	orgItem := map[string]any{
		"id":           601,
		"content_type": "Issue",
		"creator": map[string]any{
			"login":      "octocat",
			"id":         1,
			"html_url":   "https://github.com/octocat",
			"avatar_url": "https://avatars.githubusercontent.com/u/1?v=4",
		},
	}

	userItem := map[string]any{
		"id":           701,
		"content_type": "PullRequest",
		"creator": map[string]any{
			"login":      "hubot",
			"id":         2,
			"html_url":   "https://github.com/hubot",
			"avatar_url": "https://avatars.githubusercontent.com/u/2?v=4",
		},
	}

	tests := []struct {
		name                 string
		mockedClient         *http.Client
		requestArgs          map[string]any
		expectError          bool
		expectedErrMsg       string
		expectedID           int
		expectedContentType  string
		expectedCreatorLogin string
	}{
		{
			name: "success organization issue",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodPost},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						var payload struct {
							Type string `json:"type"`
							ID   int    `json:"id"`
						}
						assert.NoError(t, json.Unmarshal(body, &payload))
						assert.Equal(t, "Issue", payload.Type)
						assert.Equal(t, 9876, payload.ID)
						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write(mock.MustMarshal(orgItem))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(321),
				"item_type":      "issue",
				"item_id":        float64(9876),
			},
			expectedID:           601,
			expectedContentType:  "Issue",
			expectedCreatorLogin: "octocat",
		},
		{
			name: "success user pull request",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/items", Method: http.MethodPost},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						var payload struct {
							Type string `json:"type"`
							ID   int    `json:"id"`
						}
						assert.NoError(t, json.Unmarshal(body, &payload))
						assert.Equal(t, "PullRequest", payload.Type)
						assert.Equal(t, 7654, payload.ID)
						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write(mock.MustMarshal(userItem))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(222),
				"item_type":      "pull_request",
				"item_id":        float64(7654),
			},
			expectedID:           701,
			expectedContentType:  "PullRequest",
			expectedCreatorLogin: "hubot",
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items", Method: http.MethodPost},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(999),
				"item_type":      "issue",
				"item_id":        float64(8888),
			},
			expectError:    true,
			expectedErrMsg: ProjectAddFailedError,
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner_type":     "org",
				"project_number": float64(1),
				"item_type":      "Issue",
				"item_id":        float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"project_number": float64(1),
				"item_type":      "Issue",
				"item_id":        float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
				"item_type":  "Issue",
				"item_id":    float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing item_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing item_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_type":      "Issue",
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				switch tc.name {
				case "missing owner":
					assert.Contains(t, text, "missing required parameter: owner")
				case "missing owner_type":
					assert.Contains(t, text, "missing required parameter: owner_type")
				case "missing project_number":
					assert.Contains(t, text, "missing required parameter: project_number")
				case "missing item_type":
					assert.Contains(t, text, "missing required parameter: item_type")
				case "missing item_id":
					assert.Contains(t, text, "missing required parameter: item_id")
					// case "api error":
					// 	assert.Contains(t, text, ProjectAddFailedError)
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var item map[string]any
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &item))
			if tc.expectedID != 0 {
				assert.Equal(t, float64(tc.expectedID), item["id"])
			}
			if tc.expectedContentType != "" {
				assert.Equal(t, tc.expectedContentType, item["content_type"])
			}
			if tc.expectedCreatorLogin != "" {
				creator, ok := item["creator"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, tc.expectedCreatorLogin, creator["login"])
			}
		})
	}
}

func Test_UpdateProjectItem(t *testing.T) {
	serverTool := UpdateProjectItem(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_project_item", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "item_id")
	assert.Contains(t, schema.Properties, "updated_field")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number", "item_id", "updated_field"})

	orgUpdatedItem := map[string]any{
		"id":           801,
		"content_type": "Issue",
	}
	userUpdatedItem := map[string]any{
		"id":           802,
		"content_type": "PullRequest",
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectedID     int
	}{
		{
			name: "success organization update",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodPatch},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						var payload struct {
							Fields []struct {
								ID    int         `json:"id"`
								Value interface{} `json:"value"`
							} `json:"fields"`
						}
						assert.NoError(t, json.Unmarshal(body, &payload))
						require.Len(t, payload.Fields, 1)
						assert.Equal(t, 101, payload.Fields[0].ID)
						assert.Equal(t, "Done", payload.Fields[0].Value)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(orgUpdatedItem))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1001),
				"item_id":        float64(5555),
				"updated_field": map[string]any{
					"id":    float64(101),
					"value": "Done",
				},
			},
			expectedID: 801,
		},
		{
			name: "success user update",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/items/{item_id}", Method: http.MethodPatch},
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						body, err := io.ReadAll(r.Body)
						assert.NoError(t, err)
						var payload struct {
							Fields []struct {
								ID    int         `json:"id"`
								Value interface{} `json:"value"`
							} `json:"fields"`
						}
						assert.NoError(t, json.Unmarshal(body, &payload))
						require.Len(t, payload.Fields, 1)
						assert.Equal(t, 202, payload.Fields[0].ID)
						assert.Equal(t, 42.0, payload.Fields[0].Value)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write(mock.MustMarshal(userUpdatedItem))
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(2002),
				"item_id":        float64(6666),
				"updated_field": map[string]any{
					"id":    float64(202),
					"value": float64(42),
				},
			},
			expectedID: 802,
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodPatch},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(3003),
				"item_id":        float64(7777),
				"updated_field": map[string]any{
					"id":    float64(303),
					"value": "In Progress",
				},
			},
			expectError:    true,
			expectedErrMsg: "failed to update a project item",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(2),
				"updated_field": map[string]any{
					"id":    float64(1),
					"value": "X",
				},
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"project_number": float64(1),
				"item_id":        float64(2),
				"updated_field": map[string]any{
					"id":    float64(1),
					"value": "X",
				},
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
				"item_id":    float64(2),
				"updated_field": map[string]any{
					"id":    float64(1),
					"value": "X",
				},
			},
			expectError: true,
		},
		{
			name:         "missing item_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"updated_field": map[string]any{
					"id":    float64(1),
					"value": "X",
				},
			},
			expectError: true,
		},
		{
			name:         "missing updated_field",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(2),
			},
			expectError: true,
		},
		{
			name:         "updated_field not object",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(2),
				"updated_field":  "not-an-object",
			},
			expectError: true,
		},
		{
			name:         "updated_field missing id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(2),
				"updated_field":  map[string]any{},
			},
			expectError: true,
		},
		{
			name:         "updated_field missing value",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(2),
				"updated_field": map[string]any{
					"id": float64(9),
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				switch tc.name {
				case "missing owner":
					assert.Contains(t, text, "missing required parameter: owner")
				case "missing owner_type":
					assert.Contains(t, text, "missing required parameter: owner_type")
				case "missing project_number":
					assert.Contains(t, text, "missing required parameter: project_number")
				case "missing item_id":
					assert.Contains(t, text, "missing required parameter: item_id")
				case "missing updated_field":
					assert.Contains(t, text, "missing required parameter: updated_field")
				case "updated_field not object":
					assert.Contains(t, text, "field_value must be an object")
				case "updated_field missing id":
					assert.Contains(t, text, "updated_field.id is required")
				case "updated_field missing value":
					assert.Contains(t, text, "updated_field.value is required")
				}
				return
			}

			require.False(t, result.IsError)
			textContent := getTextResult(t, result)
			var item map[string]any
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &item))
			if tc.expectedID != 0 {
				assert.Equal(t, float64(tc.expectedID), item["id"])
			}
		})
	}
}

func Test_DeleteProjectItem(t *testing.T) {
	serverTool := DeleteProjectItem(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "delete_project_item", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be a *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner_type")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "project_number")
	assert.Contains(t, schema.Properties, "item_id")
	assert.ElementsMatch(t, schema.Required, []string{"owner_type", "owner", "project_number", "item_id"})

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectedText   string
	}{
		{
			name: "success organization delete",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodDelete},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(123),
				"item_id":        float64(555),
			},
			expectedText: "project item successfully deleted",
		},
		{
			name: "success user delete",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/users/{user}/projectsV2/{project}/items/{item_id}", Method: http.MethodDelete},
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNoContent)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octocat",
				"owner_type":     "user",
				"project_number": float64(456),
				"item_id":        float64(777),
			},
			expectedText: "project item successfully deleted",
		},
		{
			name: "api error",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project}/items/{item_id}", Method: http.MethodDelete},
					mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "boom"}),
				),
			),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(321),
				"item_id":        float64(999),
			},
			expectError:    true,
			expectedErrMsg: ProjectDeleteFailedError,
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing owner_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"project_number": float64(1),
				"item_id":        float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing project_number",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":      "octo-org",
				"owner_type": "org",
				"item_id":    float64(10),
			},
			expectError: true,
		},
		{
			name:         "missing item_id",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gh.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			require.NoError(t, err)
			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				if tc.expectedErrMsg != "" {
					assert.Contains(t, text, tc.expectedErrMsg)
				}
				switch tc.name {
				case "missing owner":
					assert.Contains(t, text, "missing required parameter: owner")
				case "missing owner_type":
					assert.Contains(t, text, "missing required parameter: owner_type")
				case "missing project_number":
					assert.Contains(t, text, "missing required parameter: project_number")
				case "missing item_id":
					assert.Contains(t, text, "missing required parameter: item_id")
				}
				return
			}

			require.False(t, result.IsError)
			text := getTextResult(t, result).Text
			assert.Contains(t, text, tc.expectedText)
		})
	}
}
