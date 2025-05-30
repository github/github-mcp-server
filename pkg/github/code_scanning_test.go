package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetCodeScanningAlert(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetCodeScanningAlert(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_code_scanning_alert", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "alertNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "alertNumber"})

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
					mockAlert,
				),
			),
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
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
			_, handler := GetCodeScanningAlert(stubGetClientFn(client), translations.NullTranslationHelper)

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
	mockClient := github.NewClient(nil)
	tool, _ := ListCodeScanningAlerts(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_code_scanning_alerts", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "ref")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.Contains(t, tool.InputSchema.Properties, "severity")
	assert.Contains(t, tool.InputSchema.Properties, "tool_name")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo"})

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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCodeScanningAlertsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"ref":       "main",
						"state":     "open",
						"severity":  "high",
						"tool_name": "codeql",
					}).andThen(
						mockResponse(t, http.StatusOK, mockAlerts),
					),
				),
			),
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCodeScanningAlertsByOwnerByRepo,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusUnauthorized)
						_, _ = w.Write([]byte(`{"message": "Unauthorized access"}`))
					}),
				),
			),
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
			_, handler := ListCodeScanningAlerts(stubGetClientFn(client), translations.NullTranslationHelper)

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
