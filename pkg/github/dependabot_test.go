package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetDependabotAlert(t *testing.T) {
	// Verify tool definition
	toolDef := GetDependabotAlert(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Validate tool schema
	assert.Equal(t, "get_dependabot_alert", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "get_dependabot_alert tool should be read-only")

	// Setup mock alert for success case
	mockAlert := &github.DependabotAlert{
		Number:  github.Ptr(42),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/security/dependabot/42"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedAlert  *github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successful alert fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepoByAlertNumber: mockResponse(t, http.StatusOK, mockAlert),
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
				GetReposDependabotAlertsByOwnerByRepoByAlertNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			// Create call request
			request := createMCPRequest(tc.requestArgs)

			// Call handler
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
			var returnedAlert github.DependabotAlert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlert)
			assert.NoError(t, err)
			assert.Equal(t, *tc.expectedAlert.Number, *returnedAlert.Number)
			assert.Equal(t, *tc.expectedAlert.State, *returnedAlert.State)
			assert.Equal(t, *tc.expectedAlert.HTMLURL, *returnedAlert.HTMLURL)
		})
	}
}

func Test_ListDependabotAlerts(t *testing.T) {
	// Verify tool definition once
	toolDef := ListDependabotAlerts(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_dependabot_alerts", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_dependabot_alerts tool should be read-only")

	// Setup mock alerts for success case
	criticalAlert := github.DependabotAlert{
		Number:  github.Ptr(1),
		HTMLURL: github.Ptr("https://github.com/owner/repo/security/dependabot/1"),
		State:   github.Ptr("open"),
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			Severity: github.Ptr("critical"),
		},
	}
	highSeverityAlert := github.DependabotAlert{
		Number:  github.Ptr(2),
		HTMLURL: github.Ptr("https://github.com/owner/repo/security/dependabot/2"),
		State:   github.Ptr("fixed"),
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			Severity: github.Ptr("high"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedAlerts []*github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successful open alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"state": "open",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"state": "open",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&criticalAlert},
		},
		{
			name: "successful severity filtered listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"severity": "high",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&highSeverityAlert}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner":    "owner",
				"repo":     "repo",
				"severity": "high",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&highSeverityAlert},
		},
		{
			name: "successful all alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert, &highSeverityAlert}),
				),
			}),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&criticalAlert, &highSeverityAlert},
		},
		{
			name: "alerts listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			client := github.NewClient(tc.mockedClient)
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				require.True(t, result.IsError)
				errorContent := getErrorResult(t, result)
				assert.Contains(t, errorContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)

			textContent := getTextResult(t, result)

			// Unmarshal and verify the result
			var returnedAlerts []*github.DependabotAlert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlerts)
			assert.NoError(t, err)
			assert.Len(t, returnedAlerts, len(tc.expectedAlerts))
			for i, alert := range returnedAlerts {
				assert.Equal(t, *tc.expectedAlerts[i].Number, *alert.Number)
				assert.Equal(t, *tc.expectedAlerts[i].HTMLURL, *alert.HTMLURL)
				assert.Equal(t, *tc.expectedAlerts[i].State, *alert.State)
				if tc.expectedAlerts[i].SecurityAdvisory != nil && tc.expectedAlerts[i].SecurityAdvisory.Severity != nil &&
					alert.SecurityAdvisory != nil && alert.SecurityAdvisory.Severity != nil {
					assert.Equal(t, *tc.expectedAlerts[i].SecurityAdvisory.Severity, *alert.SecurityAdvisory.Severity)
				}
			}
		})
	}
}
