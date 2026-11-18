package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/http/transport"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretScanningUpdateSession(t *testing.T, handler http.HandlerFunc, protocol string, labels bool) (*mcp.ClientSession, *BaseDeps) {
	t.Helper()
	client := MockHTTPClientWithHandler(handler)
	client.Transport = &transport.BearerAuthTransport{Transport: client.Transport, Token: "test-token"}
	deps := &BaseDeps{Client: mustNewGHClient(t, client)}
	if labels {
		deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "secret-scanning-update-test", Version: "1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool := UpdateSecretScanningAlert(translations.NullTranslationHelper)
	tool.RegisterFunc(server, deps)
	return connectCommentVisibilityClient(t, server, protocol), deps
}

func TestUpdateSecretScanningAlert(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tc := range []struct {
			name, patch string
		}{
			{"resolve false positive", `{"state":"resolved","resolution":"false_positive"}`},
			{"resolve wont fix", `{"state":"resolved","resolution":"wont_fix"}`},
			{"resolve revoked", `{"state":"resolved","resolution":"revoked"}`},
			{"resolve test secret", `{"state":"resolved","resolution":"used_in_tests"}`},
			{"reopen", `{"state":"open"}`},
			{"reopen with nulls", `{"state":"open","resolution":null,"resolution_comment":null}`},
			{"assign only", `{"assignee":"octocat"}`},
			{"unassign only", `{"assignee":null}`},
			{"active only", `{"validity":"active"}`},
			{"inactive only", `{"validity":"inactive"}`},
			{"clear override only", `{"validity":null}`},
			{"combined", `{"state":"resolved","resolution":"revoked","assignee":"octocat","validity":"inactive","resolution_comment":"rotated externally"}`},
			{"long comment", `{"state":"open","resolution_comment":"` + strings.Repeat("x", 281) + `"}`},
			{"empty comment", `{"state":"open","resolution_comment":""}`},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				var calls int
				session, deps := secretScanningUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "PATCH /repos/Octo/Widgets.name/secret-scanning/alerts/42", r.Method+" "+r.URL.Path)
					assert.Empty(t, r.URL.RawQuery)
					assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
					assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, tc.patch, string(body), "preserve omitted and null PATCH fields")
					_, _ = io.WriteString(w, `{"number":42,"state":"open","resolution":null,
						"assigned_to":{"login":"actual-owner","email":"not returned"},
						"validity":"unknown","secret":"must-not-appear","resolution_comment":"also not returned",
						"metadata":[{"key":"sensitive","value":"not returned"}]}`)
				}, protocol, true)
				args := json.RawMessage(`{"owner":"Octo","repo":"Widgets.name","alertNumber":42,` + tc.patch[1:])
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
				assert.Equal(t, 1, calls, "one PATCH, no fallback or policy preflight")
				expected := `{"number":"42","state":"open","resolution":null,"assigned_to":{"login":"actual-owner"},"validity":"unknown"}`
				assert.JSONEq(t, expected, getTextResult(t, result).Text, "return actual API state without secret-bearing fields")
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, expected, mustMarshalJSON(t, result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
				assert.JSONEq(t, mustMarshalJSON(t, ifc.LabelSecurityAlert()), mustMarshalJSON(t, result.Meta["ifc"]))
				read, err := deps.Client.NewRequest(t.Context(), http.MethodGet, "repos/Octo/Widgets.name", nil)
				require.NoError(t, err)
				assert.Equal(t, "2022-11-28", read.Header.Get("X-GitHub-Api-Version"), "do not mutate the shared client's API version")
			})
		}
		t.Run(protocol+"/resolved without assignee or validity", func(t *testing.T) {
			t.Parallel()
			session, _ := secretScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, `{"number":42,"state":"resolved","resolution":"used_in_tests","assigned_to":null,"secret":"must-not-appear"}`)
			}, protocol, false)
			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: map[string]any{
				"owner": "octo", "repo": "widgets", "alertNumber": 42, "assignee": nil,
			}})
			require.NoError(t, err)
			require.False(t, result.IsError, "%+v", result.Content)
			expected := `{"number":"42","state":"resolved","resolution":"used_in_tests","assigned_to":null}`
			assert.JSONEq(t, expected, getTextResult(t, result).Text)
			assert.NotContains(t, result.Meta, "ifc")
			if protocol == inventory.ProtocolVersionMultiRoundTrip {
				assert.JSONEq(t, expected, mustMarshalJSON(t, result.StructuredContent))
			}
		})
	}
}

func TestUpdateSecretScanningAlertConfiguredClient(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tc := range []struct {
			name, baseURL, uploadURL, host, prefix string
		}{
			{"cloud", "", "", "api.github.com", ""},
			{"enterprise", "https://ghe.example/api/v3/", "https://ghe.example/api/uploads/", "ghe.example", "/api/v3"},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				calls := 0
				httpClient := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "https", r.URL.Scheme)
					assert.Equal(t, tc.host, r.URL.Host)
					assert.Equal(t, tc.prefix+"/repos/octo/widgets/secret-scanning/alerts/42", r.URL.Path)
					assert.Equal(t, http.MethodPatch, r.Method)
					assert.Equal(t, "Bearer configured-token", r.Header.Get("Authorization"))
					assert.Equal(t, "configured-agent", r.Header.Get("User-Agent"))
					assert.Equal(t, "2022-11-28", r.Header.Get("X-GitHub-Api-Version"))
					assert.Equal(t, "application/vnd.github.v3+json", r.Header.Get("Accept"))
					_, _ = io.WriteString(w, `{"number":42,"state":"open"}`)
				})
				httpClient.Transport = &transport.BearerAuthTransport{
					Transport: httpClient.Transport, Token: "configured-token", AllowedHosts: []string{tc.host},
				}
				options := []github.ClientOptionsFunc{github.WithHTTPClient(httpClient), github.WithUserAgent("configured-agent")}
				if tc.baseURL != "" {
					options = append(options, github.WithEnterpriseURLs(tc.baseURL, tc.uploadURL))
				}
				client, err := github.NewClient(options...)
				require.NoError(t, err)
				baseline, err := client.NewRequest(t.Context(), http.MethodGet, "repos/octo/widgets", nil)
				require.NoError(t, err)
				deps := &BaseDeps{Client: client}
				server := mcp.NewServer(&mcp.Implementation{Name: "configured-client-test", Version: "1"}, nil)
				server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
				tool := UpdateSecretScanningAlert(translations.NullTranslationHelper)
				tool.RegisterFunc(server, deps)
				session := connectCommentVisibilityClient(t, server, protocol)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "assignee": nil,
				}})
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
				assert.Equal(t, 1, calls)
				after, err := client.NewRequest(t.Context(), http.MethodGet, "repos/octo/widgets", nil)
				require.NoError(t, err)
				assert.Equal(t, baseline.URL, after.URL)
				assert.Equal(t, baseline.Header, after.Header)
			})
		}
	}
}

func TestUpdateSecretScanningAlertRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	base := map[string]any{"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open"}
	var inputs []json.RawMessage
	for _, tc := range []struct {
		field string
		value any
	}{
		{"owner", ""}, {"repo", ""}, {"owner", "../octo"}, {"repo", ".."}, {"repo", "."},
		{"owner", "octo/widgets"}, {"repo", "widgets/../../issues"}, {"repo", `widgets\repo`},
		{"owner", " octo"}, {"repo", "widgets "}, {"repo", "widgets\n"}, {"owner", "octo\u0000"},
		{"owner", "octo?x=y"}, {"repo", "widgets#fragment"}, {"repo", "%2e%2e"}, {"owner", "%252F"},
		{"owner", nil}, {"repo", nil}, {"alertNumber", 0}, {"alertNumber", -1}, {"alertNumber", 1.5},
		{"alertNumber", "42"}, {"alertNumber", 1e30}, {"alertNumber", nil},
		{"state", "closed"}, {"state", ""}, {"state", nil}, {"state", "resolved"},
		{"resolution", "pattern_edited"}, {"resolution", "pattern_deleted"}, {"resolution", ""}, {"resolution", "invalid"},
		{"resolution", false}, {"resolution", 7}, {"resolution_comment", 7}, {"resolution_comment", []string{"comment"}},
		{"assignee", []string{"octocat"}}, {"assignee", 7}, {"validity", "unknown"}, {"validity", ""}, {"validity", false},
		{"confirm", true}, {"secret", "no"}, {"create_request", true}, {"unexpected", true}, {"STATE", "resolved"},
	} {
		args := maps.Clone(base)
		args[tc.field] = tc.value
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		inputs = append(inputs, raw)
	}
	for _, patch := range []string{
		``, `,"resolution_comment":"comment only"`, `,"resolution":"revoked"`, `,"resolution":null`,
		`,"state":"resolved"`, `,"state":"resolved","resolution":null`,
		`,"state":"resolved","resolution": null ,"assignee":null`,
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
			session, deps := secretScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid input reached GitHub")
				w.WriteHeader(http.StatusInternalServerError)
			}, protocol, false)
			tool := UpdateSecretScanningAlert(translations.NullTranslationHelper)
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

func TestUpdateSecretScanningAlertErrors(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, status := range []int{400, 403, 404, 409, 410, 422, 500, 503} {
			t.Run(protocol+"/"+fmt.Sprint(status), func(t *testing.T) {
				t.Parallel()
				var calls int
				session, _ := secretScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"message":"delegated dismissal or update denied"}`)
				}, protocol, true)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "resolved", "resolution": "revoked",
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "delegated dismissal or update denied")
				assert.NotContains(t, getTextResult(t, result).Text, "may already have been applied")
				assert.Equal(t, 1, calls, "no fallback writes")
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
			})
		}
		for _, response := range []string{
			``, `{`, `null`, `{}`, `{"number":42}`, `{"number":42,"state":null}`,
			`{"number":42,"state":"invalid"}`, `{"number":42,"state":true}`,
			`{"number":42,"state":"open","assigned_to":5}`, `{"number":43,"state":"open"}`,
		} {
			t.Run(protocol+"/"+response, func(t *testing.T) {
				t.Parallel()
				calls := 0
				session, _ := secretScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					_, _ = io.WriteString(w, response)
				}, protocol, true)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "validity": nil,
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "may already have been applied")
				assert.Contains(t, getTextResult(t, result).Text, "verify the alert with get_secret_scanning_alert before retrying")
				assert.Equal(t, 1, calls, "never retry an uncertain write")
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
			})
		}
		for _, status := range []int{http.StatusAccepted, http.StatusNoContent} {
			t.Run(protocol+"/"+fmt.Sprint(status), func(t *testing.T) {
				t.Parallel()
				calls := 0
				session, _ := secretScanningUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(status)
				}, protocol, true)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open",
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "may already have been applied")
				assert.Contains(t, getTextResult(t, result).Text, "verify the alert with get_secret_scanning_alert before retrying")
				assert.Equal(t, 1, calls, "never retry an uncertain write")
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
			})
		}
	}
}

func TestUpdateSecretScanningAlertExactNumbers(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, number := range []string{"9007199254740993", "9223372036854775807", "9223372036854775808", "1.0000000000000000001", "9007199254740993.1", "42.000000000000000001", "1e-500"} {
			t.Run(protocol+"/"+number, func(t *testing.T) {
				t.Parallel()
				valid := number == "9007199254740993" || number == "9223372036854775807"
				var calls int
				session, deps := secretScanningUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.True(t, valid, "invalid input must not reach GitHub")
					assert.Equal(t, "/repos/octo/widgets/secret-scanning/alerts/"+number, r.URL.Path)
					_, _ = fmt.Fprintf(w, `{"number":%s,"state":"open"}`, number)
				}, protocol, false)
				args := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":` + number + `,"state":"open"}`)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_secret_scanning_alert", Arguments: args})
				require.NoError(t, err)
				if !valid {
					require.True(t, result.IsError)
					assert.Zero(t, calls)
					tool := UpdateSecretScanningAlert(translations.NullTranslationHelper)
					result, err = tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &mcp.CallToolRequest{
						Params: &mcp.CallToolParamsRaw{Name: tool.Tool.Name, Arguments: args},
					})
					require.NoError(t, err)
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

func TestUpdateSecretScanningAlertInventoryAndSchema(t *testing.T) {
	t.Parallel()
	tool := UpdateSecretScanningAlert(translations.NullTranslationHelper)
	testSecurityToolSnapshot(t, tool.Tool)
	assert.Equal(t, ToolsetMetadataSecretProtection, tool.Toolset)
	assert.Equal(t, scopes.RequireAll(scopes.SecurityEvents).Scopes, tool.ScopeAccess.Scopes)
	assert.NotEmpty(t, tool.ScopeAccess.Challenge(map[string]any{}, nil))
	assert.False(t, tool.IsReadOnly())
	assert.True(t, *tool.Tool.Annotations.DestructiveHint)
	for _, tc := range []struct {
		name     string
		toolsets []string
		readOnly bool
		scopes   []string
		want     bool
	}{
		{"opt-in", []string{"secret_protection"}, false, []string{"security_events"}, true},
		{"default excludes", []string{"default"}, false, []string{"security_events"}, false},
		{"read-only excludes", []string{"secret_protection"}, true, []string{"security_events"}, false},
		{"missing scope excludes", []string{"secret_protection"}, false, []string{}, false},
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
		session, _ := secretScanningUpdateSession(t, func(_ http.ResponseWriter, _ *http.Request) {
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
