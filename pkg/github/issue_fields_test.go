package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListOrgIssueFields(t *testing.T) {
	// Verify tool definition
	serverTool := ListOrgIssueFields(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_org_issue_fields", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	assert.Contains(t, tool.InputSchema.(*jsonschema.Schema).Properties, "org")
	assert.ElementsMatch(t, tool.InputSchema.(*jsonschema.Schema).Required, []string{"org"})

	mockIssueFields := []*IssueField{
		{
			ID:          1,
			NodeID:      "IFT_kwDNAd3NAZo",
			Name:        "DRI",
			Description: "Directly responsible individual",
			DataType:    "text",
			CreatedAt:   "2024-12-11T14:39:09Z",
			UpdatedAt:   "2024-12-11T14:39:09Z",
		},
		{
			ID:          2,
			NodeID:      "IFSS_kwDNAd3NAZs",
			Name:        "Priority",
			Description: "Level of importance",
			DataType:    "single_select",
			Options: []IssueFieldOption{
				{ID: 1, Name: "High"},
				{ID: 2, Name: "Medium"},
				{ID: 3, Name: "Low"},
			},
			CreatedAt: "2024-12-11T14:39:09Z",
			UpdatedAt: "2024-12-11T14:39:09Z",
		},
	}

	tests := []struct {
		name                string
		mockedClient        *http.Client
		requestArgs         map[string]any
		expectError         bool
		expectedIssueFields []*IssueField
		expectedErrMsg      string
	}{
		{
			name: "successful issue fields retrieval",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-fields": mockResponse(t, http.StatusOK, mockIssueFields),
			}),
			requestArgs: map[string]any{
				"org": "testorg",
			},
			expectError:         false,
			expectedIssueFields: mockIssueFields,
		},
		{
			name: "issue fields not enabled returns empty list",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-fields": mockResponse(t, http.StatusNotFound, `{"message": "Not Found"}`),
			}),
			requestArgs: map[string]any{
				"org": "testorg",
			},
			expectError:         false,
			expectedIssueFields: []*IssueField{},
		},
		{
			name: "missing org parameter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-fields": mockResponse(t, http.StatusOK, mockIssueFields),
			}),
			requestArgs:    map[string]any{},
			expectError:    false,
			expectedErrMsg: "missing required parameter: org",
		},
		{
			name: "forbidden returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-fields": mockResponse(t, http.StatusForbidden, `{"message": "Forbidden"}`),
			}),
			requestArgs: map[string]any{
				"org": "testorg",
			},
			expectError:    false,
			expectedErrMsg: "failed to list issue fields",
		},
		{
			name: "internal server error returns error",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /orgs/testorg/issue-fields": mockResponse(t, http.StatusInternalServerError, `{"message": "Internal Server Error"}`),
			}),
			requestArgs: map[string]any{
				"org": "testorg",
			},
			expectError:    false,
			expectedErrMsg: "failed to list issue fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := serverTool.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				if err != nil {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
					return
				}
				require.NotNil(t, result)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			if result != nil && result.IsError {
				errorContent := getErrorResult(t, result)
				if tc.expectedErrMsg != "" && strings.Contains(errorContent.Text, tc.expectedErrMsg) {
					return
				}
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			var returnedFields []*IssueField
			err = json.Unmarshal([]byte(textContent.Text), &returnedFields)
			require.NoError(t, err)

			require.Equal(t, len(tc.expectedIssueFields), len(returnedFields))
			for i, expected := range tc.expectedIssueFields {
				assert.Equal(t, expected.ID, returnedFields[i].ID)
				assert.Equal(t, expected.Name, returnedFields[i].Name)
				assert.Equal(t, expected.DataType, returnedFields[i].DataType)
				if expected.Options != nil {
					require.Equal(t, len(expected.Options), len(returnedFields[i].Options))
					for j, opt := range expected.Options {
						assert.Equal(t, opt.Name, returnedFields[i].Options[j].Name)
					}
				}
			}
		})
	}
}
