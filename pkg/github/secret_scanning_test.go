package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetSecretScanningAlertHideSecret(t *testing.T) {
	testSecretScanningHideSecret(t, GetSecretScanningAlert(translations.NullTranslationHelper),
		GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber,
		map[string]any{"owner": "owner", "repo": "repo", "alertNumber": 42},
		map[string]string{},
		func(alert string) string { return alert })
}

func Test_ListSecretScanningAlertsHideSecret(t *testing.T) {
	testSecretScanningHideSecret(t, ListSecretScanningAlerts(translations.NullTranslationHelper),
		GetReposSecretScanningAlertsByOwnerByRepo,
		map[string]any{
			"owner": "owner", "repo": "repo", "page": 2, "perPage": 50,
			"state": "resolved", "secret_type": "test_secret", "resolution": "revoked",
		},
		map[string]string{
			"page": "2", "per_page": "50", "state": "resolved",
			"secret_type": "test_secret", "resolution": "revoked",
		},
		func(alert string) string { return "[" + alert + "]" })
}

func testSecretScanningHideSecret(t *testing.T, tool inventory.ServerTool, endpoint string, args map[string]any, query map[string]string, wrap func(string) string) {
	t.Helper()
	const visible = `{"number":42,"state":"resolved","secret_type":"test_secret","secret":"synthetic-test-secret"}`
	const hidden = `{"number":42,"state":"resolved","secret_type":"test_secret"}`
	for _, tc := range []struct {
		name     string
		option   *bool
		response string
	}{
		{name: "absent", response: visible},
		{name: "false", option: new(false), response: visible},
		{name: "true", option: new(true), response: hidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, protocol := range []string{"direct", "2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
				t.Run(protocol, func(t *testing.T) {
					requestArgs := make(map[string]any, len(args)+1)
					maps.Copy(requestArgs, args)
					expectedQuery := make(map[string][]string, len(query)+1)
					for key, value := range query {
						expectedQuery[key] = []string{value}
					}
					if tc.option != nil {
						requestArgs["hide_secret"] = *tc.option
						expectedQuery["hide_secret"] = []string{tc.name}
					}
					response := wrap(tc.response)
					calls := 0
					deps := BaseDeps{
						Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
							endpoint: func(w http.ResponseWriter, r *http.Request) {
								calls++
								assert.Equal(t, http.MethodGet, r.Method)
								assert.EqualValues(t, expectedQuery, r.URL.Query())
								mockResponse(t, http.StatusOK, json.RawMessage(response))(w, r)
							},
						})),
						featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
					}
					ctx := ContextWithDeps(context.Background(), deps)
					var result *mcp.CallToolResult
					var err error
					if protocol == "direct" {
						request := createMCPRequest(requestArgs)
						result, err = tool.Handler(deps)(ctx, &request)
					} else {
						server := mcp.NewServer(&mcp.Implementation{Name: "secret-hiding-test", Version: "v1"}, nil)
						server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
						tool.RegisterFunc(server, deps)
						session := connectCommentVisibilityClient(t, server, protocol)
						result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: requestArgs})
					}
					require.NoError(t, err)
					require.False(t, result.IsError)
					assert.Equal(t, 1, calls, "must make only the requested REST call")
					assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelSecurityAlert()), mustMarshalJSON(t, result.Meta["ifc"]))
					assert.JSONEq(t, response, getTextResult(t, result).Text)
					if protocol != inventory.ProtocolVersionMultiRoundTrip {
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.NotNil(t, result.StructuredContent)
					structuredJSON := mustMarshalJSON(t, result.StructuredContent)
					assert.JSONEq(t, response, structuredJSON)
					var structured any
					require.NoError(t, json.Unmarshal([]byte(structuredJSON), &structured))
					resolved, err := securityOutputSchema(t, tool.Tool.Name).Resolve(nil)
					require.NoError(t, err)
					require.NoError(t, resolved.Validate(structured))
				})
			}
		})
	}
}

func Test_GetSecretScanningAlert(t *testing.T) {
	toolDef := GetSecretScanningAlert(translations.NullTranslationHelper)

	testSecurityToolSnapshot(t, toolDef.Tool)

	assert.Equal(t, "get_secret_scanning_alert", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// Verify InputSchema structure
	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "alertNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "alertNumber"})

	// Setup mock alert for success case
	mockAlert := &github.SecretScanningAlert{
		Number:  new(42),
		State:   new("open"),
		HTMLURL: new("https://github.com/owner/private-repo/security/secret-scanning/42"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedAlert  *github.SecretScanningAlert
		expectedErrMsg string
	}{
		{
			name: "successful alert fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber: mockResponse(t, http.StatusOK, mockAlert),
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
				GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
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
			var returnedAlert github.Alert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlert)
			assert.NoError(t, err)
			assert.Equal(t, *tc.expectedAlert.Number, *returnedAlert.Number)
			assert.Equal(t, *tc.expectedAlert.State, *returnedAlert.State)
			assert.Equal(t, *tc.expectedAlert.HTMLURL, *returnedAlert.HTMLURL)

		})
	}
}

func Test_ListSecretScanningAlerts(t *testing.T) {
	// Verify tool definition once
	toolDef := ListSecretScanningAlerts(translations.NullTranslationHelper)

	testSecurityToolSnapshot(t, toolDef.Tool)

	assert.Equal(t, "list_secret_scanning_alerts", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)

	// Verify InputSchema structure
	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "state")
	assert.Contains(t, schema.Properties, "secret_type")
	assert.Contains(t, schema.Properties, "resolution")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	// Setup mock alerts for success case
	resolvedAlert := github.SecretScanningAlert{
		Number:     new(2),
		HTMLURL:    new("https://github.com/owner/private-repo/security/secret-scanning/2"),
		State:      new("resolved"),
		Resolution: new("false_positive"),
		SecretType: new("adafruit_io_key"),
	}
	openAlert := github.SecretScanningAlert{
		Number:     new(2),
		HTMLURL:    new("https://github.com/owner/private-repo/security/secret-scanning/3"),
		State:      new("open"),
		Resolution: new("false_positive"),
		SecretType: new("adafruit_io_key"),
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedAlerts []*github.SecretScanningAlert
		expectedErrMsg string
	}{
		{
			name: "successful resolved alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"state":    "resolved",
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{&resolvedAlert}),
				),
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"state": "resolved",
			},
			expectError:    false,
			expectedAlerts: []*github.SecretScanningAlert{&resolvedAlert},
		},
		{
			name: "successful alerts listing",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"page":     "1",
					"per_page": "30",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{&resolvedAlert, &openAlert}),
				),
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    false,
			expectedAlerts: []*github.SecretScanningAlert{&resolvedAlert, &openAlert},
		},
		{
			name: "successful alerts listing with custom pagination",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{
					"page":     "2",
					"per_page": "50",
				}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{&openAlert}),
				),
			}),
			requestArgs: map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"page":    float64(2),
				"perPage": float64(50),
			},
			expectError:    false,
			expectedAlerts: []*github.SecretScanningAlert{&openAlert},
		},
		{
			name: "direct caller zero pagination uses defaults",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepo: expectQueryParams(t, map[string]string{"page": "1", "per_page": "30"}).andThen(
					mockResponse(t, http.StatusOK, []*github.SecretScanningAlert{&openAlert}),
				),
			}),
			requestArgs: map[string]any{
				"owner": "owner", "repo": "repo", "page": 0, "perPage": 0,
			},
			expectedAlerts: []*github.SecretScanningAlert{&openAlert},
		},
		{
			name: "alerts listing fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposSecretScanningAlertsByOwnerByRepo: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			client := mustNewGHClient(t, tc.mockedClient)
			deps := BaseDeps{
				Client: client,
			}
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
			var returnedAlerts []*github.SecretScanningAlert
			err = json.Unmarshal([]byte(textContent.Text), &returnedAlerts)
			assert.NoError(t, err)
			assert.Len(t, returnedAlerts, len(tc.expectedAlerts))
			for i, alert := range returnedAlerts {
				assert.Equal(t, *tc.expectedAlerts[i].Number, *alert.Number)
				assert.Equal(t, *tc.expectedAlerts[i].HTMLURL, *alert.HTMLURL)
				assert.Equal(t, *tc.expectedAlerts[i].State, *alert.State)
				assert.Equal(t, *tc.expectedAlerts[i].Resolution, *alert.Resolution)
				assert.Equal(t, *tc.expectedAlerts[i].SecretType, *alert.SecretType)
			}
		})
	}
}
