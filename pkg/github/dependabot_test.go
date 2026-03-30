package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
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
		requestArgs    map[string]any
		expectError    bool
		expectedAlert  *github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successful alert fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepoByAlertNumber: mockResponse(t, http.StatusOK, mockAlert),
			}),
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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

func Test_UpdateDependabotAlert(t *testing.T) {
	// Verify tool definition
	toolDef := UpdateDependabotAlert(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_dependabot_alert", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.False(t, tool.Annotations.ReadOnlyHint, "update_dependabot_alert tool should not be read-only")

	mockDismissedAlert := &github.DependabotAlert{
		Number:           github.Ptr(42),
		State:            github.Ptr("dismissed"),
		DismissedReason:  github.Ptr("tolerable_risk"),
		DismissedComment: github.Ptr(""),
		HTMLURL:          github.Ptr("https://github.com/owner/repo/security/dependabot/42"),
	}
	mockOpenAlert := &github.DependabotAlert{
		Number:  github.Ptr(42),
		State:   github.Ptr("open"),
		HTMLURL: github.Ptr("https://github.com/owner/repo/security/dependabot/42"),
	}
	mockDismissedAlertWithComment := &github.DependabotAlert{
		Number:           github.Ptr(42),
		State:            github.Ptr("dismissed"),
		DismissedReason:  github.Ptr("tolerable_risk"),
		DismissedComment: github.Ptr("Acceptable risk in this context"),
		HTMLURL:          github.Ptr("https://github.com/owner/repo/security/dependabot/42"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedAlert  *github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successfully dismiss alert",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposDependabotAlertsByOwnerByRepoByAlertNumber: expectRequestBody(t, map[string]any{
					"state":            "dismissed",
					"dismissed_reason": "tolerable_risk",
				}).andThen(mockResponse(t, http.StatusOK, mockDismissedAlert)),
			}),
			requestArgs: map[string]any{
				"owner":           "owner",
				"repo":            "repo",
				"alertNumber":     float64(42),
				"state":           "dismissed",
				"dismissedReason": "tolerable_risk",
			},
			expectError:   false,
			expectedAlert: mockDismissedAlert,
		},
		{
			name: "successfully reopen alert",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposDependabotAlertsByOwnerByRepoByAlertNumber: expectRequestBody(t, map[string]any{
					"state": "open",
				}).andThen(mockResponse(t, http.StatusOK, mockOpenAlert)),
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(42),
				"state":       "open",
			},
			expectError:   false,
			expectedAlert: mockOpenAlert,
		},
		{
			name: "dismiss alert with comment",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposDependabotAlertsByOwnerByRepoByAlertNumber: expectRequestBody(t, map[string]any{
					"state":             "dismissed",
					"dismissed_reason":  "tolerable_risk",
					"dismissed_comment": "Acceptable risk in this context",
				}).andThen(mockResponse(t, http.StatusOK, mockDismissedAlertWithComment)),
			}),
			requestArgs: map[string]any{
				"owner":            "owner",
				"repo":             "repo",
				"alertNumber":      float64(42),
				"state":            "dismissed",
				"dismissedReason":  "tolerable_risk",
				"dismissedComment": "Acceptable risk in this context",
			},
			expectError:   false,
			expectedAlert: mockDismissedAlertWithComment,
		},
		{
			name: "failed request",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				PatchReposDependabotAlertsByOwnerByRepoByAlertNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					_, _ = w.Write([]byte(`{"message": "Forbidden"}`))
				}),
			}),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"alertNumber": float64(42),
				"state":       "dismissed",
			},
			expectError:    true,
			expectedErrMsg: "failed to update alert",
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
		requestArgs    map[string]any
		expectError    bool
		expectedAlerts []*github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successful open alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposDependabotAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"state":    "open",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert}),
				),
			}),
			requestArgs: map[string]any{
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
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&highSeverityAlert}),
				),
			}),
			requestArgs: map[string]any{
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
				GetReposDependabotAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert, &highSeverityAlert}),
				),
			}),
			requestArgs: map[string]any{
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
			requestArgs: map[string]any{
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

func Test_ListOrgDependabotAlerts(t *testing.T) {
	// Verify tool definition once
	toolDef := ListOrgDependabotAlerts(translations.NullTranslationHelper)
	tool := toolDef.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_org_dependabot_alerts", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint, "list_org_dependabot_alerts tool should be read-only")

	// Setup mock alerts for success case
	criticalAlert := github.DependabotAlert{
		Number:  github.Ptr(1),
		HTMLURL: github.Ptr("https://github.com/myorg/repo1/security/dependabot/1"),
		State:   github.Ptr("open"),
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			Severity: github.Ptr("critical"),
		},
	}
	highSeverityAlert := github.DependabotAlert{
		Number:  github.Ptr(2),
		HTMLURL: github.Ptr("https://github.com/myorg/repo2/security/dependabot/2"),
		State:   github.Ptr("open"),
		SecurityAdvisory: &github.DependabotSecurityAdvisory{
			Severity: github.Ptr("high"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedAlerts []*github.DependabotAlert
		expectedErrMsg string
	}{
		{
			name: "successful listing with state filter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsDependabotAlertsByOrg: expectQueryParams(t, map[string]string{
					"state":    "open",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert, &highSeverityAlert}),
				),
			}),
			requestArgs: map[string]any{
				"org":   "myorg",
				"state": "open",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&criticalAlert, &highSeverityAlert},
		},
		{
			name: "successful listing with severity filter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsDependabotAlertsByOrg: expectQueryParams(t, map[string]string{
					"severity": "critical",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&criticalAlert}),
				),
			}),
			requestArgs: map[string]any{
				"org":      "myorg",
				"severity": "critical",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&criticalAlert},
		},
		{
			name: "successful listing with ecosystem filter",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsDependabotAlertsByOrg: expectQueryParams(t, map[string]string{
					"ecosystem": "npm",
					"page":      "1",
					"per_page":  "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.DependabotAlert{&highSeverityAlert}),
				),
			}),
			requestArgs: map[string]any{
				"org":       "myorg",
				"ecosystem": "npm",
			},
			expectError:    false,
			expectedAlerts: []*github.DependabotAlert{&highSeverityAlert},
		},
		{
			name: "alerts listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetOrgsDependabotAlertsByOrg: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"message": "Unauthorized access"}`))
				}),
			}),
			requestArgs: map[string]any{
				"org": "myorg",
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
