package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v73/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetCodeScanningAlert(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetCodeScanningAlert(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

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
	mockClient := github.NewClient(nil)
	tool, _ := ListCodeScanningAlerts(stubGetClientFn(mockClient), translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

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

func Test_UpdateCodeScanningAlert(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := UpdateCodeScanningAlert(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "update_code_scanning_alert", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "alertNumber")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "alertNumber", "state"})

	// Mock alert for success
	mockAlert := &github.Alert{
		Number:  github.Ptr(42),
		State:   github.Ptr("open"),
		Rule:    &github.Rule{ID: github.Ptr("rule-id"), Description: github.Ptr("desc")},
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
			name: "successful alert update",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.PatchReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
					mockAlert,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(42),
				"state":       "open",
			},
			expectError:   false,
			expectedAlert: mockAlert,
		},
		{
			name: "update fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.PatchReposCodeScanningAlertsByOwnerByRepoByAlertNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"message": "Invalid request"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(9999),
				"state":       "open",
			},
			expectError:    true,
			expectedErrMsg: "failed to update alert",
		},
		{
			name:         "error when dismissed_reason not provided",
			mockedClient: nil, // early exit happens before any HTTP call
			requestArgs: map[string]interface{}{
				"owner":            "owner",
				"repo":             "repo",
				"alertNumber":      float64(42),
				"state":            "dismissed",
				"dismissed_reason": "",
			},
			expectError:    true,
			expectedErrMsg: "dismissed_reason required for 'dismissed' state",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := UpdateCodeScanningAlert(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			text := getTextResult(t, result)
			var got github.Alert
			require.NoError(t, json.Unmarshal([]byte(text.Text), &got))

			assert.Equal(t, *tc.expectedAlert.Number, *got.Number)
			assert.Equal(t, *tc.expectedAlert.State, *got.State)
			assert.Equal(t, *tc.expectedAlert.Rule.ID, *got.Rule.ID)
			assert.Equal(t, *tc.expectedAlert.HTMLURL, *got.HTMLURL)
		})
	}
}

func Test_ListOrgCodeScanningAlerts(t *testing.T) {
	// Verify tool definition
	mockClient := github.NewClient(nil)
	tool, _ := ListOrgCodeScanningAlerts(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "list_org_code_scanning_alerts", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "org")
	assert.Contains(t, tool.InputSchema.Properties, "sort")
	assert.Contains(t, tool.InputSchema.Properties, "severity")
	assert.Contains(t, tool.InputSchema.Properties, "tool_name")
	assert.Contains(t, tool.InputSchema.Properties, "state")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"org"})

	// Mock alerts for success
	mockAlerts := []*github.Alert{
		{
			Number:  github.Ptr(100),
			State:   github.Ptr("open"),
			Rule:    &github.Rule{ID: github.Ptr("org-rule-1"), Description: github.Ptr("desc1")},
			HTMLURL: github.Ptr("https://github.com/org/repo/security/code-scanning/100"),
		},
		{
			Number:  github.Ptr(101),
			State:   github.Ptr("dismissed"),
			Rule:    &github.Rule{ID: github.Ptr("org-rule-2"), Description: github.Ptr("desc2")},
			HTMLURL: github.Ptr("https://github.com/org/repo/security/code-scanning/101"),
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
			name: "successful org alerts listing",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetOrgsCodeScanningAlertsByOrg,
					expectQueryParams(t, map[string]string{
						"state":     "open",
						"severity":  "high",
						"tool_name": "codeql",
						"sort":      "created",
					}).andThen(
						mockResponse(t, http.StatusOK, mockAlerts),
					),
				),
			),
			requestArgs: map[string]interface{}{
				"org":       "org",
				"state":     "open",
				"severity":  "high",
				"tool_name": "codeql",
				"sort":      "created",
			},
			expectError:    false,
			expectedAlerts: mockAlerts,
		},
		{
			name: "org alerts listing fails",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetOrgsCodeScanningAlertsByOrg,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusForbidden)
						_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"org": "org",
			},
			expectError:    true,
			expectedErrMsg: "failed to list organization alerts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := ListOrgCodeScanningAlerts(stubGetClientFn(client), translations.NullTranslationHelper)
			request := createMCPRequest(tc.requestArgs)

			result, err := handler(context.Background(), request)
			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			text := getTextResult(t, result)

			var got []*github.Alert
			require.NoError(t, json.Unmarshal([]byte(text.Text), &got))
			assert.Len(t, got, len(tc.expectedAlerts))

			for i := range got {
				assert.Equal(t, *tc.expectedAlerts[i].Number, *got[i].Number)
				assert.Equal(t, *tc.expectedAlerts[i].State, *got[i].State)
				assert.Equal(t, *tc.expectedAlerts[i].Rule.ID, *got[i].Rule.ID)
				assert.Equal(t, *tc.expectedAlerts[i].HTMLURL, *got[i].HTMLURL)
			}
		})
	}
}
