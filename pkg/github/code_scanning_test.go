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

func Test_GetCodeScanningAlert(t *testing.T) {
	// Verify tool definition once
	toolDef := GetCodeScanningAlert(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_code_scanning_alert", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// InputSchema is of type any, need to cast to *jsonschema.Schema
	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "alertNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "alertNumber"})

	// Setup mock alert for success case
	mockAlert := &github.Alert{
		Number:  github.Ptr(42),
		State:   github.Ptr("open"),
		Rule:    &github.Rule{ID: github.Ptr("test-rule"), Description: github.Ptr("Test Rule Description")},
		HTMLURL: github.Ptr("https://github.com/owner/repo/security/code-scanning/42"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedAlert  *github.Alert
		expectedErrMsg string
	}{
		{
			name: "successful alert fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber: mockResponse(t, http.StatusOK, mockAlert),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(42),
			},
			expectError:   false,
			expectedAlert: mockAlert,
		},
		{
			name: "alert fetch fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(9999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get alert",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler with new signature
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedAlert github.Alert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlert)
			assert.NoError(t, err)
			assert.Equal(t, *tc.expectedAlert.Number, *returnedAlert.Number)
			assert.Equal(t, *tc.expectedAlert.State, *returnedAlert.State)
			assert.Equal(t, *tc.expectedAlert.Rule.ID, *returnedAlert.Rule.ID)
			assert.Equal(t, *tc.expectedAlert.HTMLURL, *returnedAlert.HTMLURL)

		})
	}
}

func Test_ListCodeScanningAlerts(t *testing.T) {
	// Verify tool definition once
	toolDef := ListCodeScanningAlerts(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "list_code_scanning_alerts", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// InputSchema is of type any, need to cast to *jsonschema.Schema
	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "ref")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "severity")
	assert.Contains(t, schema.Properties, "tool_name")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock alerts for success case
	mockAlerts := []*github.Alert{
		{
			Number:  github.Ptr(42),
			State:   github.Ptr("open"),
			Rule:    &github.Rule{ID: github.Ptr("test-rule-1"), Description: github.Ptr("Test Rule 1")},
			HTMLURL: github.Ptr("https://github.com/owner/repo/security/code-scanning/42"),
		},
		{
			Number:  github.Ptr(43),
			State:   github.Ptr("fixed"),
			Rule:    &github.Rule{ID: github.Ptr("test-rule-2"), Description: github.Ptr("Test Rule 2")},
			HTMLURL: github.Ptr("https://github.com/owner/repo/security/code-scanning/43"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedAlerts []*github.Alert
		expectedErrMsg string
	}{
		{
			name: "successful alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeScanningAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"ref":       "main",
					"state":     "open",
					"severity":  "high",
					"tool_name": "codeql",
				}).andThen(
					mockResponse(t, http.StatusOK, mockAlerts),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner":     "owner",
				"repo":      "repo",
				"ref":       "main",
				"state":     "open",
				"severity":  "high",
				"tool_name": "codeql",
			},
			expectError:    false,
			expectedAlerts: mockAlerts,
		},
		{
			name: "alerts listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposCodeScanningAlertsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"message": "Unauthorized access"}`))
				}),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "failed to list alerts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup client with mock
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler with new signature
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			// Verify results
			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			// Parse the result and get the text content if no error
			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedAlerts []*github.Alert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlerts)
			assert.NoError(t, err)
			assert.Len(t, returnedAlerts, len(tc.expectedAlerts))
			for i, alert := range returnedAlerts {
				assert.Equal(t, *tc.expectedAlerts[i].Number, *alert.Number)
				assert.Equal(t, *tc.expectedAlerts[i].State, *alert.State)
				assert.Equal(t, *tc.expectedAlerts[i].Rule.ID, *alert.Rule.ID)
				assert.Equal(t, *tc.expectedAlerts[i].HTMLURL, *alert.HTMLURL)
			}
		})
	}
}
