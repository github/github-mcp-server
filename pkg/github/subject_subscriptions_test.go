package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subjectSubscriptionTool(t *testing.T, name string) inventory.ServerTool {
	t.Helper()
	for _, tool := range AllTools(translations.NullTranslationHelper) {
		if tool.Tool.Name == name {
			return tool
		}
	}
	t.Fatalf("%s is missing from the tool inventory", name)
	return inventory.ServerTool{}
}

// The fake replaces only GitHub's HTTP boundary; typed decoding and GraphQL generation remain real.
func subscriptionHTTPClient(t *testing.T, field string, currentState any, canSubscribe bool, desiredState string, queryError, mutationError string) (*http.Client, *int) {
	t.Helper()
	calls := 0
	client := MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		calls++
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/graphql", r.URL.Path)
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		subject := map[string]any{
			"id": "subject-id", "url": "https://github.com/owner/repo/issues/7",
			"viewerCanSubscribe": canSubscribe, "viewerSubscription": currentState,
		}
		if strings.Contains(body.Query, "updateSubscription") {
			assert.NotContains(t, body.Query, "url", "Subscribable has no url field in GitHub's GraphQL schema")
			assert.Equal(t, 2, calls, "a mutation must follow subject lookup")
			assert.Equal(t, map[string]any{"input": map[string]any{
				"subscribableId": "subject-id", "state": desiredState,
			}}, body.Variables)
			if mutationError != "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": mutationError}}})
				return
			}
			subject["viewerSubscription"] = desiredState
			delete(subject, "url")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"updateSubscription": map[string]any{"subscribable": subject},
			}})
			return
		}
		assert.Equal(t, 1, calls)
		assert.Contains(t, strings.ReplaceAll(body.Query, " ", ""), field+"(number:$number)")
		assert.Equal(t, map[string]any{"owner": "owner", "repo": "repo", "number": float64(7)}, body.Variables)
		if queryError != "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": []any{map[string]any{"message": queryError}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"repository": map[string]any{field: subject},
		}})
	})
	return client, &calls
}

func Test_SubjectSubscriptions(t *testing.T) {
	t.Parallel()
	for _, subject := range []struct{ kind, field string }{
		{"issue", "issue"}, {"pull_request", "pullRequest"}, {"discussion", "discussion"},
	} {
		for _, action := range []struct {
			tool, state, current, desired string
			calls                         int
		}{
			{"get_subject_subscription", "", "SUBSCRIBED", "", 1},
			{"manage_subject_subscription", "subscribed", "UNSUBSCRIBED", "SUBSCRIBED", 2},
			{"manage_subject_subscription", "unsubscribed", "SUBSCRIBED", "UNSUBSCRIBED", 2},
			{"manage_subject_subscription", "ignored", "SUBSCRIBED", "IGNORED", 2},
			{"manage_subject_subscription", "subscribed", "SUBSCRIBED", "SUBSCRIBED", 1},
		} {
			t.Run(subject.kind+"/"+action.tool+"/"+action.state+"/"+action.current, func(t *testing.T) {
				tool := subjectSubscriptionTool(t, action.tool)
				httpClient, calls := subscriptionHTTPClient(t, subject.field, action.current, true, action.desired, "", "")
				deps := BaseDeps{GQLClient: githubv4.NewClient(httpClient), Obsv: stubExporters()}
				args := map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": subject.kind}
				if action.state != "" {
					args["state"] = action.state
				}
				request := createMCPRequest(args)
				result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &request)
				require.NoError(t, err)
				require.False(t, result.IsError, result)
				wantState := action.state
				if wantState == "" {
					wantState = "subscribed"
				}
				want, err := json.Marshal(map[string]any{
					"id": "subject-id", "url": "https://github.com/owner/repo/issues/7",
					"subject_type": subject.kind, "number": 7, "state": wantState, "can_subscribe": true,
				})
				require.NoError(t, err)
				assert.JSONEq(t, string(want), getTextResult(t, result).Text)
				assert.Equal(t, action.calls, *calls)
			})
		}
	}
}

func Test_SubjectSubscriptions_InvalidArguments(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"get_subject_subscription", "manage_subject_subscription"} {
		t.Run(name, func(t *testing.T) {
			tool := subjectSubscriptionTool(t, name)
			cases := []struct {
				field string
				value any
			}{
				{"owner", ""}, {"repo", ""}, {"number", 0}, {"number", -1},
				{"number", 1.5}, {"number", 2147483648}, {"subjectType", "repository"},
				{"subjectType", ""}, {"owner", 9},
			}
			if name == "manage_subject_subscription" {
				cases = append(cases, struct {
					field string
					value any
				}{"state", "watch"})
			}
			for _, tc := range cases {
				args := map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue"}
				if name == "manage_subject_subscription" {
					args["state"] = "subscribed"
				}
				args[tc.field] = tc.value
				deps := stubDeps{
					gqlClientFn: func(context.Context) (*githubv4.Client, error) {
						t.Fatal("invalid arguments must not obtain a GitHub client")
						return nil, nil
					}, obsv: stubExporters(),
				}
				req := createMCPRequest(args)
				result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
				require.NoError(t, err)
				require.True(t, result.IsError, args)
				assert.Contains(t, getErrorResult(t, result).Text, tc.field)
			}
			for _, field := range []string{"owner", "repo", "number", "subjectType", "state"} {
				if field == "state" && name == "get_subject_subscription" {
					continue
				}
				args := map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"}
				delete(args, field)
				deps := stubDeps{gqlClientFn: func(context.Context) (*githubv4.Client, error) {
					t.Fatal("missing argument reached GitHub")
					return nil, nil
				}, obsv: stubExporters()}
				req := createMCPRequest(args)
				result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, field)
			}
		})
	}
}

func Test_SubjectSubscriptions_Errors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, queryError, mutationError string
		canSubscribe                    bool
		calls                           int
		want                            string
	}{
		{"query denied", "Resource not accessible", "", true, 1, "Resource not accessible"},
		{"cannot subscribe", "", "", false, 1, "subscribe"},
		{"mutation denied", "", "Permission denied", true, 2, "Permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tool := subjectSubscriptionTool(t, "manage_subject_subscription")
			client, calls := subscriptionHTTPClient(t, "issue", "UNSUBSCRIBED", tc.canSubscribe, "SUBSCRIBED", tc.queryError, tc.mutationError)
			deps := BaseDeps{GQLClient: githubv4.NewClient(client), Obsv: stubExporters()}
			req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"})
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, tc.want)
			assert.Equal(t, tc.calls, *calls)
			assert.Nil(t, result.StructuredContent)
		})
	}
	for _, name := range []string{"get_subject_subscription", "manage_subject_subscription"} {
		t.Run(name+"/client error", func(t *testing.T) {
			tool := subjectSubscriptionTool(t, name)
			deps := stubDeps{gqlClientFn: stubGQLClientFnErr("client unavailable"), obsv: stubExporters()}
			req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"})
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, "client unavailable")
		})
	}
}

func Test_SubjectSubscriptions_Protocols(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"get_subject_subscription", "manage_subject_subscription"} {
		tool := subjectSubscriptionTool(t, name)
		require.Equal(t, ToolsetMetadataNotifications.ID, tool.Toolset.ID)
		assert.Equal(t, name == "get_subject_subscription", tool.IsReadOnly())
		require.NoError(t, toolsnaps.Test(name, tool.Tool))
		for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
			t.Run(name+"/"+protocol, func(t *testing.T) {
				client, _ := subscriptionHTTPClient(t, "issue", "SUBSCRIBED", true, "SUBSCRIBED", "", "")
				deps := BaseDeps{GQLClient: githubv4.NewClient(client), Obsv: stubExporters()}
				session := connectTypedSearchClient(t, tool, deps, protocol)
				listed, err := session.ListTools(t.Context(), nil)
				require.NoError(t, err)
				require.Len(t, listed.Tools, 1)
				modern := protocol == inventory.ProtocolVersionMultiRoundTrip
				assert.Equal(t, modern, listed.Tools[0].OutputSchema != nil)
				if modern {
					require.NoError(t, toolsnaps.Test(name+"_output", mcp.Tool{Name: name, OutputSchema: listed.Tools[0].OutputSchema}))
				}
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{
					"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed",
				}})
				require.NoError(t, err)
				require.False(t, result.IsError, result)
				text := getTextResult(t, result).Text
				assert.JSONEq(t, `{"id":"subject-id","url":"https://github.com/owner/repo/issues/7","subject_type":"issue","number":7,"state":"subscribed","can_subscribe":true}`, text)
				if modern {
					require.NotNil(t, result.StructuredContent)
					assert.JSONEq(t, text, mustMarshalJSON(t, result.StructuredContent))
				} else {
					assert.Nil(t, result.StructuredContent)
				}
			})
		}
	}
}

func Test_SubjectSubscriptions_PrivateIFC(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"get_subject_subscription", "manage_subject_subscription"} {
		t.Run(name, func(t *testing.T) {
			tool := subjectSubscriptionTool(t, name)
			client, _ := subscriptionHTTPClient(t, "issue", "SUBSCRIBED", true, "SUBSCRIBED", "", "")
			deps := NewBaseDeps(nil, githubv4.NewClient(client), nil, nil, translations.NullTranslationHelper, FeatureFlags{}, 0,
				func(context.Context, string) (bool, error) { return true, nil }, stubExporters())
			req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"})
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
			require.NoError(t, err)
			require.False(t, result.IsError, result)
			label, err := json.Marshal(result.Meta["ifc"])
			require.NoError(t, err)
			assert.JSONEq(t, `{"integrity":"trusted","confidentiality":"private"}`, string(label))
		})
	}
}

func Test_SubjectSubscriptions_MissingSubject(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"get_subject_subscription", "manage_subject_subscription"} {
		for _, response := range []string{
			`{"data":{"repository":null}}`,
			`{"data":{"repository":{"issue":null}}}`,
		} {
			t.Run(name+"/"+response, func(t *testing.T) {
				tool := subjectSubscriptionTool(t, name)
				calls := 0
				client := MockHTTPClientWithHandler(func(w http.ResponseWriter, _ *http.Request) {
					calls++
					require.Equal(t, 1, calls, "a missing subject must never be mutated")
					_, _ = w.Write([]byte(response))
				})
				deps := BaseDeps{GQLClient: githubv4.NewClient(client), Obsv: stubExporters()}
				req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"})
				result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, "not found")
				assert.Equal(t, 1, calls)
			})
		}
	}
}

func Test_SubjectSubscriptions_InvalidMutationResponse(t *testing.T) {
	t.Parallel()
	for _, response := range []string{
		`{"data":{"updateSubscription":null}}`,
		`{"data":{"updateSubscription":{"subscribable":null}}}`,
		`{"data":{"updateSubscription":{"subscribable":{"id":"wrong-id","viewerSubscription":"SUBSCRIBED","viewerCanSubscribe":true}}}}`,
		`{"data":{"updateSubscription":{"subscribable":{"id":"subject-id","viewerSubscription":null,"viewerCanSubscribe":true}}}}`,
		`{"data":{"updateSubscription":{"subscribable":{"id":"subject-id","viewerSubscription":"UNSUBSCRIBED","viewerCanSubscribe":true}}}}`,
	} {
		t.Run(response, func(t *testing.T) {
			tool := subjectSubscriptionTool(t, "manage_subject_subscription")
			calls := 0
			client := MockHTTPClientWithHandler(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				if calls == 1 {
					_, _ = w.Write([]byte(`{"data":{"repository":{"issue":{"id":"subject-id","url":"https://github.com/owner/repo/issues/7","viewerSubscription":"UNSUBSCRIBED","viewerCanSubscribe":true}}}}`))
					return
				}
				_, _ = w.Write([]byte(response))
			})
			deps := BaseDeps{GQLClient: githubv4.NewClient(client), Obsv: stubExporters()}
			req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue", "state": "subscribed"})
			result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
			require.NoError(t, err)
			require.True(t, result.IsError, "incomplete or mismatched mutation data must not report success")
			assert.Nil(t, result.StructuredContent)
			if strings.Contains(response, `"viewerSubscription":"UNSUBSCRIBED"`) {
				text := getErrorResult(t, result).Text
				assert.Contains(t, text, "unsubscribed")
				assert.Contains(t, text, "subscribed")
			}
			assert.Equal(t, 2, calls)
		})
	}
}

func Test_SubjectSubscriptions_NullState(t *testing.T) {
	t.Parallel()
	tool := subjectSubscriptionTool(t, "get_subject_subscription")
	client, calls := subscriptionHTTPClient(t, "issue", nil, false, "", "", "")
	deps := BaseDeps{GQLClient: githubv4.NewClient(client), Obsv: stubExporters()}
	req := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "number": 7, "subjectType": "issue"})
	result, err := tool.Handler(deps)(ContextWithDeps(t.Context(), deps), &req)
	require.NoError(t, err)
	require.False(t, result.IsError, result)
	assert.JSONEq(t, `{"id":"subject-id","url":"https://github.com/owner/repo/issues/7","subject_type":"issue","number":7,"state":null,"can_subscribe":false}`, getTextResult(t, result).Text)
	assert.Equal(t, 1, *calls)
}

func Test_SubjectSubscriptions_NotificationScope(t *testing.T) {
	t.Parallel()
	tool := subjectSubscriptionTool(t, "manage_subject_subscription")
	for _, tc := range []struct {
		scopes  []string
		visible bool
	}{
		{nil, false},
		{[]string{"repo"}, false},
		{[]string{"public_repo"}, false},
		{[]string{"notifications"}, true},
		{[]string{"repo", "notifications"}, true},
	} {
		assert.Equal(t, tc.visible, tool.ScopeAccess.Visible(tc.scopes), tc.scopes)
	}
	assert.Equal(t, []string{"notifications"}, tool.ScopeAccess.Challenge(nil, []string{"repo"}))
	assert.Equal(t, []string{"repo"}, tool.ScopeAccess.Challenge(nil, []string{"notifications"}), "private subject lookup needs repository access")
	assert.Empty(t, tool.ScopeAccess.Challenge(nil, []string{"repo", "notifications"}))
	assert.ElementsMatch(t, []string{"repo", "notifications"}, tool.ScopeAccess.Scopes, "maximum scopes must include the lookup and mutation")
}
