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

	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dependabotUpdateSession(t *testing.T, handler http.HandlerFunc, protocol string) (*mcp.ClientSession, *BaseDeps) {
	t.Helper()
	deps := &BaseDeps{
		Client:         mustNewGHClient(t, MockHTTPClientWithHandler(handler)),
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "dependabot-update-test", Version: "1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool := UpdateDependabotAlert(translations.NullTranslationHelper)
	tool.RegisterFunc(server, deps)
	return connectCommentVisibilityClient(t, server, protocol), deps
}

func TestUpdateDependabotAlert(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, tc := range []struct {
			name, patch, response string
		}{
			{"assign human", `{"assignees":["octocat"]}`, `{"state":"open","assignees":[{"login":"octocat"}]}`},
			{"assign Copilot", `{"assignees":["copilot-swe-agent[bot]"]}`, `{"state":"open","assignees":[{"login":"copilot-swe-agent[bot]"}]}`},
			{"clear", `{"assignees":[]}`, `{"state":"open","assignees":[]}`},
			{"dismiss", `{"state":"dismissed","dismissed_reason":"tolerable_risk","dismissed_comment":""}`, `{"state":"dismissed","dismissed_reason":"tolerable_risk","dismissed_comment":"","assignees":[{"login":"existing-owner"}]}`},
			{"reopen", `{"state":"open"}`, `{"state":"open","dismissed_reason":null,"dismissed_comment":null,"assignees":[]}`},
			{"combined", `{"state":"dismissed","dismissed_reason":"fix_started","assignees":["octocat"]}`, `{"state":"dismissed","dismissed_reason":"fix_started","assignees":[{"login":"octocat"}]}`},
			{"actual state differs", `{"state":"dismissed","dismissed_reason":"not_used"}`, `{"state":"open","assignees":[]}`},
			{"missing assignments stay unknown", `{"state":"open"}`, `{"state":"open"}`},
			{"280 Unicode characters", `{"state":"dismissed","dismissed_reason":"inaccurate","dismissed_comment":"` + strings.Repeat("\u00e9", 280) + `"}`, `{"state":"dismissed"}`},
		} {
			t.Run(protocol+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				var calls int
				session, deps := dependabotUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.Equal(t, "PATCH /repos/octo/widgets/dependabot/alerts/42", r.Method+" "+r.URL.Path)
					assert.Empty(t, r.URL.RawQuery)
					assert.Equal(t, "2026-03-10", r.Header.Get("X-GitHub-Api-Version"))
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.JSONEq(t, tc.patch, string(body), "forward only provided PATCH fields")
					_, _ = fmt.Fprintf(w, `{"number":42,"html_url":"https://github.com/octo/widgets/security/dependabot/42","security_advisory":{"description":"not returned"},%s}`, tc.response[1:len(tc.response)-1])
				}, protocol)
				args := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":42,` + tc.patch[1:])
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_dependabot_alert", Arguments: args})
				require.NoError(t, err)
				require.False(t, result.IsError, "%+v", result.Content)
				assert.Equal(t, 1, calls, "one PATCH, no policy or issue preflights")
				text := getTextResult(t, result).Text
				assert.NotContains(t, text, "security_advisory")
				assert.NotContains(t, text, "pull_request")
				var expected map[string]any
				require.NoError(t, json.Unmarshal([]byte(tc.response), &expected))
				var actual map[string]any
				require.NoError(t, json.Unmarshal([]byte(text), &actual))
				for key, value := range expected {
					assert.Equal(t, value, actual[key], "return the actual API %s", key)
				}
				assert.Equal(t, "42", actual["number"])
				if _, present := expected["assignees"]; !present {
					assert.NotContains(t, actual, "assignees")
				}
				if protocol == inventory.ProtocolVersionMultiRoundTrip {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, text, mustMarshalJSON(t, result.StructuredContent))
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

func TestUpdateDependabotAlertRejectsInvalidInput(t *testing.T) {
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
		{"alertNumber", 0}, {"alertNumber", -1}, {"alertNumber", 1.5}, {"alertNumber", "42"},
		{"alertNumber", 1e30}, {"alertNumber", nil},
		{"state", "fixed"}, {"state", ""}, {"state", nil},
		{"state", "dismissed"}, {"dismissed_reason", "false_positive"}, {"dismissed_reason", nil},
		{"dismissed_comment", nil}, {"dismissed_comment", strings.Repeat("x", 281)},
		{"dismissed_comment", strings.Repeat("\u00e9", 281)},
		{"assignees", nil}, {"assignees", "octocat"}, {"assignees", []any{nil}},
		{"agent_assignment", map[string]any{"model": "unsupported"}}, {"unexpected", true}, {"STATE", "dismissed"},
	} {
		args := maps.Clone(base)
		args[tc.field] = tc.value
		raw, err := json.Marshal(args)
		require.NoError(t, err)
		inputs = append(inputs, raw)
	}
	for _, patch := range []string{
		``, `,"dismissed_comment":"comment only"`, `,"dismissed_reason":"fix_started"`,
		`,"state":"dismissed"`, `,"state":"dismissed","dismissed_reason":null`,
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
			session, deps := dependabotUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
				t.Error("invalid input reached GitHub")
				w.WriteHeader(http.StatusInternalServerError)
			}, protocol)
			tool := UpdateDependabotAlert(translations.NullTranslationHelper)
			for _, raw := range inputs {
				t.Run(string(raw), func(t *testing.T) {
					result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: tool.Tool.Name, Arguments: raw})
					require.NoError(t, err)
					require.True(t, result.IsError, "%+v", result.Content)
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

func TestUpdateDependabotAlertErrors(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, status := range []int{400, 403, 404, 409, 410, 422, 500} {
			t.Run(protocol+"/"+fmt.Sprint(status), func(t *testing.T) {
				t.Parallel()
				var calls int
				session, _ := dependabotUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
					calls++
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"message":"upstream policy denied this update"}`)
				}, protocol)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_dependabot_alert", Arguments: map[string]any{
					"owner": "octo", "repo": "widgets", "alertNumber": 42, "assignees": []string{"copilot-swe-agent[bot]"},
				}})
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getTextResult(t, result).Text, "upstream policy denied this update")
				assert.Equal(t, 1, calls, "no fallback writes")
				assert.Nil(t, result.StructuredContent)
				assert.NotContains(t, result.Meta, "ifc")
			})
		}
	}
	for _, response := range []string{`{`, `{}`, `{"number":42,"state":"open"}`, `{"number":43,"state":"open","html_url":"https://github.com/octo/widgets/security/dependabot/43"}`} {
		t.Run(response, func(t *testing.T) {
			t.Parallel()
			session, _ := dependabotUpdateSession(t, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, response)
			}, inventory.ProtocolVersionMultiRoundTrip)
			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_dependabot_alert", Arguments: map[string]any{
				"owner": "octo", "repo": "widgets", "alertNumber": 42, "state": "open",
			}})
			require.NoError(t, err)
			require.True(t, result.IsError)
		})
	}
}

func TestUpdateDependabotAlertExactNumbers(t *testing.T) {
	t.Parallel()
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		for _, number := range []string{"9007199254740993", "9223372036854775807", "9223372036854775808", "42.000000000000000001", "1e-500"} {
			t.Run(protocol+"/"+number, func(t *testing.T) {
				t.Parallel()
				valid := number == "9007199254740993" || number == "9223372036854775807"
				var calls int
				session, _ := dependabotUpdateSession(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					assert.True(t, valid, "invalid input must not reach GitHub")
					assert.Equal(t, "/repos/octo/widgets/dependabot/alerts/"+number, r.URL.Path)
					_, _ = fmt.Fprintf(w, `{"number":%s,"state":"open","html_url":"https://github.com/octo/widgets/security/dependabot/%s"}`, number, number)
				}, protocol)
				args := json.RawMessage(`{"owner":"octo","repo":"widgets","alertNumber":` + number + `,"state":"open"}`)
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "update_dependabot_alert", Arguments: args})
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

func TestUpdateDependabotAlertInventoryAndSchema(t *testing.T) {
	t.Parallel()
	tool := UpdateDependabotAlert(translations.NullTranslationHelper)
	testSecurityToolSnapshot(t, tool.Tool)
	assert.Equal(t, ToolsetMetadataDependabot, tool.Toolset)
	assert.Equal(t, scopes.RequireAll(scopes.SecurityEvents).Scopes, tool.ScopeAccess.Scopes)
	assert.False(t, tool.IsReadOnly())
	assert.True(t, *tool.Tool.Annotations.DestructiveHint)
	for _, tc := range []struct {
		name     string
		toolsets []string
		readOnly bool
		scopes   []string
		want     bool
	}{
		{"opt-in", []string{"dependabot"}, false, []string{"security_events"}, true},
		{"default excludes", []string{"default"}, false, []string{"security_events"}, false},
		{"read-only excludes", []string{"dependabot"}, true, []string{"security_events"}, false},
		{"missing scope excludes", []string{"dependabot"}, false, []string{}, false},
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
		session, _ := dependabotUpdateSession(t, func(_ http.ResponseWriter, _ *http.Request) {
			t.Error("listing tools must not call GitHub")
		}, protocol)
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
