package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	gh "github.com/google/go-github/v76/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ProjectRead(t *testing.T) {
	mockClient := gh.NewClient(nil)
	tool, _ := ProjectRead(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "project_read", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "method")
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "owner_type")
	assert.Contains(t, tool.InputSchema.Properties, "project_number")
	assert.Contains(t, tool.InputSchema.Properties, "field_id")
	assert.Contains(t, tool.InputSchema.Properties, "item_id")
	assert.Contains(t, tool.InputSchema.Properties, "query")
	assert.Contains(t, tool.InputSchema.Properties, "per_page")
	assert.Contains(t, tool.InputSchema.Properties, "fields")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"method", "owner", "owner_type"})

	orgProjects := []map[string]any{{"id": 1, "title": "Org Project"}}
	orgProject := map[string]any{"id": 1, "title": "Org Project"}
	projectFields := []map[string]any{{"id": 100, "name": "Status"}}
	projectField := map[string]any{"id": 100, "name": "Status"}
	projectItems := []map[string]any{{"id": 1000, "title": "Item 1"}}
	projectItem := map[string]any{"id": 1000, "title": "Item 1"}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "list_projects success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgProjects),
				),
			),
			requestArgs: map[string]interface{}{
				"method":     "list_projects",
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError: false,
		},
		{
			name: "get_project success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, orgProject),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "get_project",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
			},
			expectError: false,
		},
		{
			name: "list_project_fields success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/fields", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, projectFields),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "list_project_fields",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
			},
			expectError: false,
		},
		{
			name: "get_project_field success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/fields/{field_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, projectField),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "get_project_field",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"field_id":       float64(100),
			},
			expectError: false,
		},
		{
			name: "list_project_items success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, projectItems),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "list_project_items",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
			},
			expectError: false,
		},
		{
			name: "get_project_item success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items/{item_id}", Method: http.MethodGet},
					mockResponse(t, http.StatusOK, projectItem),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "get_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1000),
			},
			expectError: false,
		},
		{
			name:         "missing method",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
		{
			name:         "missing owner",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":     "list_projects",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "invalid method",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":     "invalid_method",
				"owner":      "octo-org",
				"owner_type": "org",
			},
			expectError:    true,
			expectedErrMsg: "unknown method: invalid_method",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := gh.NewClient(tc.mockedClient)

			_, handler := ProjectRead(stubGetClientFn(mockClient), translations.NullTranslationHelper)
			result, err := handler(ctx, createMCPRequest(tc.requestArgs))
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				assert.Contains(t, text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
		})
	}
}

// Test_ProjectRead_FilterSpecialFieldTypes_ListProjectFields ensures that special intrinsic
// field data types (e.g., labels, repository, title) are filtered out of the response
// by list_project_fields. Only non-special field data types should remain.
func Test_ProjectRead_FilterSpecialFieldTypes_ListProjectFields(t *testing.T) {
	// Mock fields returned by the API including special types.
	projectFields := []map[string]any{
		{"id": 1, "name": "Title", "data_type": "title"},
		{"id": 2, "name": "Labels", "data_type": "labels"},
		{"id": 3, "name": "Status", "data_type": "single_select"},
		{"id": 4, "name": "Repository", "data_type": "repository"},
	}

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/fields", Method: http.MethodGet},
			mockResponse(t, http.StatusOK, projectFields),
		),
	)

	ctx := context.Background()
	client := gh.NewClient(mockedClient)
	_, handler := ProjectRead(stubGetClientFn(client), translations.NullTranslationHelper)
	result, err := handler(ctx, createMCPRequest(map[string]any{
		"method":         "list_project_fields",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	text := getTextResult(t, result).Text
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	fieldsAny, ok := parsed["fields"].([]any)
	require.True(t, ok, "expected fields array in response")

	// Only the non-special field (single_select) should remain.
	assert.Equal(t, 1, len(fieldsAny))
	fieldObj := fieldsAny[0].(map[string]any)
	assert.Equal(t, float64(3), fieldObj["id"], "expected Status field to remain")
	assert.Equal(t, "single_select", fieldObj["data_type"])
}

// helper returning a project item seeded with special intrinsic field types.
func testProjectItemWithSpecialFields() map[string]any {
	return map[string]any{
		"id":    1000,
		"title": "Item 1",
		"fields": []map[string]any{
			{"id": 1, "name": "Title", "data_type": "title", "value": map[string]any{"raw": "Item 1"}},
			{"id": 2, "name": "Labels", "data_type": "labels", "value": []string{"bug"}},
			{"id": 3, "name": "Status", "data_type": "single_select", "value": map[string]any{"name": map[string]any{"raw": "In Progress"}}},
		},
	}
}

// Test_ProjectRead_FilterSpecialFieldTypes_GetProjectItem validates filtering for get_project_item
// so that special field data types are removed from each item's field values slice.
func Test_ProjectRead_FilterSpecialFieldTypes_GetProjectItem(t *testing.T) {
	projectItem := testProjectItemWithSpecialFields()

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items/{item_id}", Method: http.MethodGet},
			mockResponse(t, http.StatusOK, projectItem),
		),
	)

	ctx := context.Background()
	client := gh.NewClient(mockedClient)
	_, handler := ProjectRead(stubGetClientFn(client), translations.NullTranslationHelper)

	resGet, err := handler(ctx, createMCPRequest(map[string]any{
		"method":         "get_project_item",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
		"item_id":        float64(1000),
	}))
	require.NoError(t, err)
	require.False(t, resGet.IsError)

	textGet := getTextResult(t, resGet).Text
	var parsedGet map[string]any
	require.NoError(t, json.Unmarshal([]byte(textGet), &parsedGet))
	fieldsAny, ok := parsedGet["fields"].([]any)
	require.True(t, ok)
	assert.Equal(t, 1, len(fieldsAny), "expected only non-special field to remain for get_project_item")
	fieldObj := fieldsAny[0].(map[string]any)
	assert.Equal(t, float64(3), fieldObj["id"])
	assert.Equal(t, "single_select", fieldObj["data_type"])
}

// Test_ProjectRead_FilterSpecialFieldTypes_ListProjectItems validates filtering for list_project_items
// so that special field data types are removed from each item's field values slice.
func Test_ProjectRead_FilterSpecialFieldTypes_ListProjectItems(t *testing.T) {
	projectItem := testProjectItemWithSpecialFields()
	projectItems := []map[string]any{projectItem}

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items", Method: http.MethodGet},
			mockResponse(t, http.StatusOK, projectItems),
		),
	)

	ctx := context.Background()
	client := gh.NewClient(mockedClient)
	_, handler := ProjectRead(stubGetClientFn(client), translations.NullTranslationHelper)

	resList, err := handler(ctx, createMCPRequest(map[string]any{
		"method":         "list_project_items",
		"owner":          "octo-org",
		"owner_type":     "org",
		"project_number": float64(1),
	}))
	require.NoError(t, err)
	require.False(t, resList.IsError)

	textList := getTextResult(t, resList).Text
	var parsedList map[string]any
	require.NoError(t, json.Unmarshal([]byte(textList), &parsedList))
	itemsAny, ok := parsedList["items"].([]any)
	require.True(t, ok)
	require.Equal(t, 1, len(itemsAny))
	itemObj := itemsAny[0].(map[string]any)
	itemFields, ok := itemObj["fields"].([]any)
	require.True(t, ok)
	assert.Equal(t, 1, len(itemFields), "expected only non-special field to remain for list_project_items")
	itemFieldObj := itemFields[0].(map[string]any)
	assert.Equal(t, float64(3), itemFieldObj["id"])
	assert.Equal(t, "single_select", itemFieldObj["data_type"])
}

func Test_ProjectWrite(t *testing.T) {
	mockClient := gh.NewClient(nil)
	tool, _ := ProjectWrite(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "project_write", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "method")
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "owner_type")
	assert.Contains(t, tool.InputSchema.Properties, "project_number")
	assert.Contains(t, tool.InputSchema.Properties, "item_id")
	assert.Contains(t, tool.InputSchema.Properties, "item_type")
	assert.Contains(t, tool.InputSchema.Properties, "updated_field")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"method", "owner", "owner_type", "project_number", "item_id"})

	addedItem := map[string]any{"id": 1000, "title": "Added Item"}
	updatedItem := map[string]any{"id": 1000, "title": "Updated Item"}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "add_project_item success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items", Method: http.MethodPost},
					mockResponse(t, http.StatusCreated, addedItem),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "add_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(123),
				"item_type":      "issue",
			},
			expectError: false,
		},
		{
			name: "update_project_item success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items/{item_id}", Method: http.MethodPatch},
					mockResponse(t, http.StatusOK, updatedItem),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "update_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1000),
				"updated_field":  map[string]any{"id": float64(100), "value": "New Value"},
			},
			expectError: false,
		},
		{
			name: "delete_project_item success",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.EndpointPattern{Pattern: "/orgs/{org}/projectsV2/{project_number}/items/{item_id}", Method: http.MethodDelete},
					mockResponse(t, http.StatusNoContent, nil),
				),
			),
			requestArgs: map[string]interface{}{
				"method":         "delete_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1000),
			},
			expectError: false,
		},
		{
			name:         "missing method",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: method",
		},
		{
			name:         "add_project_item missing item_type",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":         "add_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(123),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: item_type",
		},
		{
			name:         "update_project_item missing updated_field",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":         "update_project_item",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(1000),
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: updated_field",
		},
		{
			name:         "invalid method",
			mockedClient: mock.NewMockedHTTPClient(),
			requestArgs: map[string]interface{}{
				"method":         "invalid_method",
				"owner":          "octo-org",
				"owner_type":     "org",
				"project_number": float64(1),
				"item_id":        float64(123),
			},
			expectError:    true,
			expectedErrMsg: "unknown method: invalid_method",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := gh.NewClient(tc.mockedClient)

			_, handler := ProjectWrite(stubGetClientFn(mockClient), translations.NullTranslationHelper)
			result, err := handler(ctx, createMCPRequest(tc.requestArgs))
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				text := getTextResult(t, result).Text
				assert.Contains(t, text, tc.expectedErrMsg)
				return
			}

			require.False(t, result.IsError)
		})
	}
}
