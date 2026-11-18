package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func codeScanningUpdateSession(t *testing.T, handler http.HandlerFunc, protocol string, labels bool) (*mcp.ClientSession, *BaseDeps) {
	t.Helper()
	deps := &BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandler(handler))}
	if labels {
		deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "code-scanning-update-test", Version: "1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool := UpdateCodeScanningAlert(translations.NullTranslationHelper)
	tool.RegisterFunc(server, deps)
	return connectCommentVisibilityClient(t, server, protocol), deps
}

func TestUpdateCodeScanningAlert(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tc := range []struct {
			name, patch, response, expected string
		}{
			{"dismiss", `{"state":"dismissed","dismissed_reason":"false positive","dismissed_comment":"sanitized"}`,
				`{"state":"dismissed","dismissed_reason":"false positive","rule":{"help":"do not echo"},"most_recent_instance":{"message":{"text":"untrusted code"}}}`,
				`{"state":"dismissed","dismissed_reason":"false positive"}`},
			{"reopen with nulls", `{"state":"open","dismissed_reason":null,"dismissed_comment":null,"create_request":false}`,
				`{"state":"open","dismissed_reason":null,"assignees":[{"login":"octocat","email":"not returned"}]}`,
				`{"state":"open","assignees":[{"login":"octocat"}]}`},
			{"assignment only", `{"assignees":["octocat","copilot-swe-agent[bot]"]}`,
				`{"state":"open","assignees":[{"login":"copilot-swe-agent[bot]"}]}`,
				`{"state":"open","assignees":[{"login":"copilot-swe-agent[bot]"}]}`},
			{"clear", `{"assignees":[]}`, `{"state":"open","assignees":[]}`, `{"state":"open","assignees":[]}`},
			{"dismissal request does not dismiss", `{"state":"dismissed","dismissed_reason":"mitigated","create_request":true}`,
				`{"state":"open"}`, `{"state":"open"}`},
			{"request attempt can return dismissed", `{"state":"dismissed","dismissed_reason":"won't fix","create_request":true}`,
				`{"state":"dismissed"}`, `{"state":"dismissed"}`},
			{"nullable actual state", `{"assignees":["octocat"]}`, `{"state":null}`, `{"state":null}`},
			{"combined", `{"state":"dismissed","dismissed_reason":"used in tests","assignees":["octocat"]}`,
				`{"state":"dismissed","assignees":[{"login":"octocat"}]}`, `{"state":"dismissed","assignees":[{"login":"octocat"}]}`},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				var calls int
				session, deps := codeScanningUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "PATCH /repos/octo/widgets/code-scanning/alerts/42", r.Method+" "+r.URL.Path)
					assert.Empty(t, r.URL.RawQuery)
					assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, tc.patch, string(body), "preserve omitted, null and empty PATCH fields")
					_, _ = fmt.Fprintf(w, `{"number":42,"html_url":"https://github.com/octo/widgets/security/code-scanning/42",%s}`, tc.response[1:len(tc.response)-1])
				}, protocol, true)
				args := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":42,` + tc.patch[1:])
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
				assert.Equal(t, 1, calls, "one PATCH, no fallback or policy preflights")
				expected := `{"number":"42","html_url":"https://github.com/octo/widgets/security/code-scanning/42",` + tc.expected[1:]
				text := getTextResult(t, result).Text
				assert.JSONEq(t, expected, text)
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, expected, mustMarshalJSON(t, result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
				assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelSecurityAlert()), mustMarshalJSON(t, result.Meta["ifc"]))
				request, err := deps.Client.NewRequest(t.Context(), http.MethodGet, "repos/octo/widgets", nil)
				require.NoError(t, err)
				assert.Equal(t, "2022-11-28", request.Header.Get("X-GitHub-Api-Version"), "do not mutate the shared client's API version")
			})
		}
	}
}

func TestUpdateCodeScanningAlertConfiguredClient(t *testing.T) {
	t.Parallel()
	for _, enterprise := range []bool{false, true} {
		t.Run(fmt.Sprint("enterprise=", enterprise), func(t *testing.T) {
			t.Parallel()
			prefix := ""
			if enterprise {
				prefix = "/api/v3"
			}
			var calls int
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, "PATCH", r.Method)
				assert.Equal(t, prefix+"/repos/octo/widgets/code-scanning/alerts/42", r.URL.Path)
				assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
				assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
				assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
				_, _ = io.WriteString(w, `{"number":42,"state":"open","html_url":"https://github.com/octo/widgets/security/code-scanning/42"}`)
				if !enterprise {
					assert.Equal(t, "api.github.com", r.URL.Host)
				}
			})
			opts := []github.ClientOptionsFunc{
				github.WithHTTPClient(MockHTTPClientWithHandler(handler)),
				github.WithAuthToken("test-token"),
			}
			if enterprise {
				api := httptest.NewServer(handler)
				t.Cleanup(api.Close)
				opts = append(opts, github.WithHTTPClient(api.Client()), github.WithEnterpriseURLs(api.URL+"/api/v3/", api.URL+"/api/uploads/"))
			}
			client, err := github.NewClient(opts...)
			require.NoError(t, err)
			baseURL := client.BaseURL()
			deps := &BaseDeps{Client: client}
			for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
				server := mcp.NewServer(&mcp.Implementation{Name: "configured-client-test", Version: "1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				tool := UpdateCodeScanningAlert(translations.NullTranslationHelper)
				tool.RegisterFunc(server, deps)
				session := connectCommentVisibilityClient(t, server, protocol)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open",
				}})
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
			}
			assert.Equal(t, 2, calls)
			assert.Equal(t, baseURL, client.BaseURL())
		})
	}
}

func TestUpdateCodeScanningAlertReasonsAndComments(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, reason := range []string{"false positive", "won't fix", "used in tests", "mitigated"} {
			t.Run(protocol+"/"+reason, func(t *testing.T) {
				t.Parallel()
				session, _ := codeScanningUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, reason, body["dismissed_reason"])
					assert.Contains(t, body, "dismissed_comment")
					_, _ = io.WriteString(w, `{"number":42,"state":"dismissed","html_url":"https://github.com/octo/widgets/security/code-scanning/42"}`)
				}, protocol, false)
				for _, comment := range []any{nil, "", strings.Repeat("a", 280), strings.Repeat("\u00e9", 280)} {
					result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: map[string]any{
						"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "dismissed", "dismissed_reason": reason, "dismissed_comment": comment,
					}})
					require.NoError(t, err)
					require.False(t, result.IsError, "%+v", result.Content)
					assert.NotContains(t, result.Meta, "ifc")
				}
			})
		}
	}
}

func TestUpdateCodeScanningAlertRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	base := map[string]any{"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open"}
	var inputs []json.RawMessage
	for _, tc := range []struct {
		field string
		value any
	}{
		{"owner", ""}, {"repo", ""}, {"owner", "../octo"}, {"repo", ".."}, {"repo", "."},
		{"owner", "octo/widgets"}, {"repo", "widgets/../../issues"}, {"repo", `widgets\repo`},
		{"owner", " octo"}, {"repo", "widgets\n"}, {"owner", "octo?x=y"}, {"repo", "widgets#fragment"}, {"repo", "%2e%2e"},
		{"owner", nil}, {"repo", nil}, {"alertNumber", 0}, {"alertNumber", -1}, {"alertNumber", 1.5},
		{"alertNumber", "42"}, {"alertNumber", 1e30}, {"alertNumber", nil},
		{"state", "fixed"}, {"state", ""}, {"state", nil}, {"state", "dismissed"},
		{"dismissed_reason", "fix_started"}, {"dismissed_reason", false}, {"dismissed_comment", 7},
		{"dismissed_comment", strings.Repeat("x", 281)}, {"dismissed_comment", strings.Repeat("\u00e9", 281)},
		{"create_request", true}, {"create_request", "true"}, {"create_request", nil},
		{"assignees", nil}, {"assignees", "octocat"}, {"assignees", []any{nil}}, {"assignees", []any{1}},
		{"agent_assignment", map[string]any{"model": "unsupported"}}, {"confirm", true}, {"unexpected", true}, {"STATE", "dismissed"},
	} {
		args := maps.Clone(base)
		args[tc.field] = tc.value
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		inputs = append(inputs, raw)
	}
	for _, patch := range []string{
		``, `,"dismissed_comment":"comment only"`, `,"dismissed_reason":"mitigated"`, `,"create_request":false`,
		`,"state":"dismissed"`, `,"state":"dismissed","dismissed_reason":null`,
		`,"assignees":[],"create_request":true`,
	} {
		inputs = append(inputs, json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":42`+patch+`}`))
	}
	for _, field := range []string{"owner", "repo", "alertNumber"} {
		args := maps.Clone(base)
		delete(args, field)
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		inputs = append(inputs, raw)
	}
	inputs = append(inputs, json.RawMessage(`null`), json.RawMessage(`[]`))
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			t.Parallel()
			session, deps := codeScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid input reached GitHub")
				w.WriteHeader(http.StatusInternalServerError)
			}, protocol, false)
			tool := UpdateCodeScanningAlert(translations.NullTranslationHelper)
			for _, raw := range inputs {
				t.Run(string(raw), func(t *testing.T) {
					result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: raw})
					require.NoError(t, err)
					require.True(t, result.IsError, "%+v", result.Content)
					assert.Nil(t, result.StructuredContent)
					result, err = tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &mcp.CallToolRequest{
						Params: &mcp.CallToolParamsRaw{Name: tool.Tool.Name, Arguments: raw},
					})
					require.NoError(t, err)
					require.True(t, result.IsError, "%+v", result.Content)
				})
			}
		})
	}
}

func TestUpdateCodeScanningAlertInvalidDismissalFieldTypes(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			t.Parallel()
			session, deps := codeScanningUpdateSession(t, func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("invalid input reached GitHub")
			}, protocol, false)
			tool := UpdateCodeScanningAlert(translations.NullTranslationHelper)
			for _, field := range []string{"dismissed_reason", "dismissed_comment"} {
				for _, value := range []string{`false`, `7`, `[]`, `{}`} {
					t.Run(field+"/"+value, func(t *testing.T) {
						raw := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":42,"state":"open","` + field + `":` + value + `}`)
						result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: raw})
						require.NoError(t, err)
						require.True(t, result.IsError)
						assert.Contains(t, getTextResult(t, result).Text, "invalid "+field+":")
						result, err = tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &mcp.CallToolRequest{
							Params: &mcp.CallToolParamsRaw{Name: tool.Tool.Name, Arguments: raw},
						})
						require.NoError(t, err)
						require.True(t, result.IsError)
						assert.Contains(t, getTextResult(t, result).Text, "invalid "+field+":")
					})
				}
			}
		})
	}
}

func TestUpdateCodeScanningAlertErrors(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, status := range []int{400, 403, 404, 409, 410, 422, 500, 503} {
			t.Run(protocol+"/"+fmt.Sprint(status), func(t *testing.T) {
				t.Parallel()
				var calls int
				session, _ := codeScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"message":"upstream policy denied this update"}`)
				}, protocol, true)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "dismissed", "dismissed_reason": "mitigated", "create_request": true,
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "upstream policy denied this update")
				assert.Equal(t, 1, calls, "no fallback writes")
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
			})
		}
		for _, response := range []string{
			`{`, `{}`, ``, `{"number":"42"}`, `{"number":42,"state":"open"}`,
			`{"number":42,"html_url":"https://github.com/octo/widgets/security/code-scanning/42"}`,
			`{"number":42,"state":true,"html_url":"https://github.com/octo/widgets/security/code-scanning/42"}`,
			`{"number":43,"state":"open","html_url":"https://github.com/octo/widgets/security/code-scanning/43"}`,
		} {
			t.Run(protocol+"/"+response, func(t *testing.T) {
				t.Parallel()
				var calls int
				session, _ := codeScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					_, _ = io.WriteString(w, response)
				}, protocol, true)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open",
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
				assert.Contains(t, getTextResult(t, result).Text, "may already have been applied")
				assert.Contains(t, getTextResult(t, result).Text, "verify the alert with get_code_scanning_alert before retrying")
				assert.Equal(t, 1, calls, "do not retry uncertain writes")
			})
		}
		for _, status := range []int{http.StatusAccepted, http.StatusNoContent} {
			t.Run(protocol+"/"+fmt.Sprint(status), func(t *testing.T) {
				t.Parallel()
				var calls int
				session, _ := codeScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(status)
				}, protocol, false)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open",
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent)
				assert.Contains(t, getTextResult(t, result).Text, "may already have been applied")
				assert.Contains(t, getTextResult(t, result).Text, "verify the alert with get_code_scanning_alert before retrying")
				assert.Equal(t, 1, calls, "do not retry uncertain writes")
			})
		}
	}
}

func TestUpdateCodeScanningAlertExactNumbers(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, number := range []string{"9007199254740993", "9223372036854775807", "9223372036854775808", "1.0000000000000000001", "9007199254740993.1", "42.000000000000000001", "1e-500"} {
			t.Run(protocol+"/"+number, func(t *testing.T) {
				t.Parallel()
				valid := number == "9007199254740993" || number == "9223372036854775807"
				var calls int
				session, _ := codeScanningUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.True(t, valid, "invalid input must not reach GitHub")
					assert.Equal(t, "/repos/octo/widgets/code-scanning/alerts/"+number, r.URL.Path)
					_, _ = fmt.Fprintf(w, `{"number":%s,"state":"open","html_url":"https://github.com/octo/widgets/security/code-scanning/%s"}`, number, number)
				}, protocol, false)
				args := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":` + number + `,"state":"open"}`)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_code_scanning_alert", Arguments: args})
				require.NoError(t, err)
				if !valid {
					require.True(t, result.IsError)
					assert.Zero(t, calls)
					return
				}
				require.False(t, result.IsError, "%+v", result.Content)
				assert.Equal(t, 1, calls)
				var output struct {
					Number string `json:"number"`
				}
				require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
				assert.Equal(t, number, output.Number)
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					assert.Equal(t, number, output.Number)
				}
			})
		}
	}
}

func TestUpdateCodeScanningAlertInventoryAndSchema(t *testing.T) {
	t.Parallel()
	tool := UpdateCodeScanningAlert(translations.NullTranslationHelper)
	testSecurityToolSnapshot(t, tool.Tool)
	assert.Equal(t, ToolsetMetadataCodeSecurity, tool.Toolset)
	assert.Equal(t, scopes.RequireAll(scopes.SecurityEvents).Scopes, tool.ScopeAccess.Scopes)
	assert.NotEmpty(t, tool.ScopeAccess.Challenge(map[string]any{}, nil))
	assert.False(t, tool.IsReadOnly())
	assert.True(t, *tool.Tool.Annotations.DestructiveHint)
	assert.False(t, tool.Tool.Annotations.IdempotentHint)
	for _, tc := range []struct {
		name     string
		toolsets []string
		readOnly bool
		scopes   []string
		want     bool
	}{
		{"opt-in", []string{"code_security"}, false, []string{"security_events"}, true},
		{"default excludes", []string{"default"}, false, []string{"security_events"}, false},
		{"read-only excludes", []string{"code_security"}, true, []string{"security_events"}, false},
		{"missing scope excludes", []string{"code_security"}, false, []string{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := NewInventory(translations.NullTranslationHelper).
				WithToolsets(tc.toolsets).WithReadOnly(tc.readOnly).
				WithFilter(CreateToolScopeFilter(tc.scopes)).Build()
			require.NoError(t, err)
			found := false
			for _, candidate := range inv.AvailableTools(context.Background()) {
				found = found || candidate.Tool.Name == tool.Tool.Name
			}
			assert.Equal(t, tc.want, found)
		})
	}
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		session, _ := codeScanningUpdateSession(t, func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("listing tools must not call GitHub")
		}, protocol, false)
		listed, err := session.ListTools(t.Context(), nil)
		require.NoError(t, err)
		require.Len(t, listed.Tools, 1)
		encoded := mustMarshalJSON(t, listed.Tools[0].InputSchema)
		var schema map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(encoded), &schema))
		for _, keyword := range []string{"anyOf", "oneOf", "allOf", "if", "then", "else"} {
			assert.NotContains(t, schema, keyword)
		}
		assert.NotContains(t, encoded, `"default"`)
		if protocol == inventory.ProtocolVersionMultiRoundTrip {
			assert.JSONEq(t, mustMarshalJSON(t, securityOutputSchema(t, tool.Tool.Name)), mustMarshalJSON(t, listed.Tools[0].OutputSchema))
		} else {
			assert.Nil(t, listed.Tools[0].OutputSchema)
		}
	}
}
