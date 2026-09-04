package github

import (
	"context"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_RepositoryRulesetRead(t *testing.T) {
	toolDef := RepositoryRulesetRead(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "repository_ruleset_read", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"level", "method"})

	t.Run("repository level: get defaults includes_parents to true", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/{ruleset_id}": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "main protection", Enforcement: "active"})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get", "owner": "owner", "repo": "repo", "ruleset_id": float64(42)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "true", capturedQuery.Get("includes_parents"))

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "main protection", returned.Name)
	})

	t.Run("repository level: get forwards explicit includes_parents=false", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/{ruleset_id}": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "rs"})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get", "owner": "owner", "repo": "repo", "ruleset_id": float64(42), "includes_parents": false})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "false", capturedQuery.Get("includes_parents"))
	})

	t.Run("repository level: get requires ruleset_id", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ruleset_id")
	})

	t.Run("repository level: requires owner and repo", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "list"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "owner")
	})

	t.Run("repository level: list omits includes_parents when not provided", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{{Name: "rs1"}})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "list", "owner": "owner", "repo": "repo", "perPage": float64(50)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.False(t, capturedQuery.Has("includes_parents"), "includes_parents must not be sent when omitted")
		assert.Equal(t, "50", capturedQuery.Get("per_page"))

		var returned []*github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "rs1", returned[0].Name)
	})

	t.Run("repository level: list forwards explicit includes_parents=false", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "list", "owner": "owner", "repo": "repo", "includes_parents": false})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "false", capturedQuery.Get("includes_parents"))
	})

	t.Run("repository level: get_rules_for_branch", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rules/branches/{branch}": mockResponse(t, http.StatusOK, []map[string]any{{"type": "creation"}}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get_rules_for_branch", "owner": "owner", "repo": "repo", "branch": "main"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "Creation")
	})

	t.Run("repository level: get_rules_for_branch requires branch", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get_rules_for_branch", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "branch")
	})

	t.Run("repository level: list_rule_suites forwards filters", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/rule-suites": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []map[string]any{{"id": 101, "result": "pass"}})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":             "repository",
			"method":            "list_rule_suites",
			"owner":             "owner",
			"repo":              "repo",
			"ref":               "refs/heads/main",
			"time_period":       "week",
			"actor_name":        "octocat",
			"rule_suite_result": "pass",
			"evaluate_status":   "evaluate",
			"perPage":           float64(25),
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "pass")

		assert.Equal(t, "refs/heads/main", capturedQuery.Get("ref"))
		assert.Equal(t, "week", capturedQuery.Get("time_period"))
		assert.Equal(t, "octocat", capturedQuery.Get("actor_name"))
		assert.Equal(t, "pass", capturedQuery.Get("rule_suite_result"))
		assert.Equal(t, "evaluate", capturedQuery.Get("evaluate_status"))
		assert.Equal(t, "25", capturedQuery.Get("per_page"))
	})

	t.Run("repository level: get_rule_suite", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/rule-suites/{rule_suite_id}": mockResponse(t, http.StatusOK, map[string]any{"id": 101, "result": "fail"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "get_rule_suite", "owner": "owner", "repo": "repo", "rule_suite_id": float64(101)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "fail")
	})

	t.Run("repository level: unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "method": "frobnicate", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})

	t.Run("organization level: get", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets/{ruleset_id}": mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "org rs", Enforcement: "active"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "get", "org": "octo", "ruleset_id": float64(7)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "org rs", returned.Name)
	})

	t.Run("organization level: get requires ruleset_id", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "get", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ruleset_id")
	})

	t.Run("organization level: requires org", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "list"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "org")
	})

	t.Run("organization level: list", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets": mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{{Name: "org rs"}}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "list", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []*github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "org rs", returned[0].Name)
	})

	t.Run("organization level: list_rule_suites forwards organization filters", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets/rule-suites": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []map[string]any{{"id": 101, "repository_name": "repo"}})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":           "organization",
			"method":          "list_rule_suites",
			"org":             "octo",
			"repository_name": "repo",
			"evaluate_status": "active",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "repo", capturedQuery.Get("repository_name"))
		assert.Equal(t, "active", capturedQuery.Get("evaluate_status"))
	})

	t.Run("organization level: get_rule_suite", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets/rule-suites/{rule_suite_id}": mockResponse(t, http.StatusOK, map[string]any{"id": 101, "result": "pass"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "get_rule_suite", "org": "octo", "rule_suite_id": float64(101)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "pass")
	})

	t.Run("organization level: repository-only method is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "get_rules_for_branch", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "not supported for level \"organization\"")
	})

	t.Run("organization level: unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "method": "frobnicate", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "not supported for level")
	})

	t.Run("enterprise level: get", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /enterprises/{enterprise}/rulesets/{ruleset_id}": mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "enterprise rs", Enforcement: "active"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "enterprise", "method": "get", "enterprise": "acme", "ruleset_id": float64(9)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "enterprise rs", returned.Name)
	})

	t.Run("enterprise level: requires enterprise", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "enterprise", "method": "list"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "enterprise")
	})

	t.Run("enterprise level: list", func(t *testing.T) {
		t.Run("success preserves total count and pagination", func(t *testing.T) {
			var capturedQuery url.Values
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /enterprises/{enterprise}/rulesets": func(w http.ResponseWriter, r *http.Request) {
					capturedQuery = r.URL.Query()
					mockResponse(t, http.StatusOK, map[string]any{
						"total_count": 17,
						"rulesets":    []*github.RepositoryRuleset{{Name: "enterprise rs"}},
					})(w, r)
				},
			}))
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{
				"level":      "enterprise",
				"method":     "list",
				"enterprise": "acme",
				"page":       float64(2),
				"perPage":    float64(1),
			})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.Equal(t, "2", capturedQuery.Get("page"))
			assert.Equal(t, "1", capturedQuery.Get("per_page"))

			var returned struct {
				TotalCount int                         `json:"total_count"`
				Rulesets   []*github.RepositoryRuleset `json:"rulesets"`
			}
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
			assert.Equal(t, 17, returned.TotalCount)
			require.Len(t, returned.Rulesets, 1)
			assert.Equal(t, "enterprise rs", returned.Rulesets[0].Name)
		})

		t.Run("empty response preserves object shape", func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /enterprises/{enterprise}/rulesets": mockResponse(t, http.StatusOK, map[string]any{
					"total_count": 0,
					"rulesets":    []*github.RepositoryRuleset{},
				}),
			}))
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{"level": "enterprise", "method": "list", "enterprise": "acme"})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)

			var returned struct {
				TotalCount int                         `json:"total_count"`
				Rulesets   []*github.RepositoryRuleset `json:"rulesets"`
			}
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
			assert.Zero(t, returned.TotalCount)
			assert.Empty(t, returned.Rulesets)
		})

		t.Run("API error", func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"GET /enterprises/{enterprise}/rulesets": mockResponse(t, http.StatusInternalServerError, map[string]string{"message": "Internal Server Error"}),
			}))
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{"level": "enterprise", "method": "list", "enterprise": "acme"})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, "failed to list enterprise repository rulesets")
		})
	})

	t.Run("enterprise level: rule suite method is rejected", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "enterprise", "method": "list_rule_suites", "enterprise": "acme"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "not supported for level \"enterprise\"")
	})

	t.Run("unknown level defers to normal validation", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "planet", "method": "list"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
	})

	t.Run("mismatched-case level is rejected rather than silently normalized", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				called = true
				mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "Organization", "method": "list", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
		assert.False(t, called, "a mismatched-case level must not reach the organization-level API call")

		assert.Empty(t, toolDef.ScopeAccess.Challenge(map[string]any{"level": "Organization"}, nil))
	})
}

func Test_GetRepositoryRulesForBranchEscapesBranch(t *testing.T) {
	tests := []struct {
		name        string
		branch      string
		escapedPath string
	}{
		{
			name:        "slash",
			branch:      "release/1.0",
			escapedPath: "/repos/owner/repo/rules/branches/release%2F1.0",
		},
		{
			name:        "special characters",
			branch:      "release/#1%ready",
			escapedPath: "/repos/owner/repo/rules/branches/release%2F%231%25ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := mustNewGHClient(t, MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.escapedPath, r.URL.EscapedPath())
				mockResponse(t, http.StatusOK, []map[string]any{{"type": "creation"}})(w, r)
			}))

			result, err := GetRepositoryRulesForBranch(t.Context(), client, "owner", "repo", tt.branch, PaginationParams{})
			require.NoError(t, err)
			require.False(t, result.IsError)
		})
	}
}

func Test_CreateRepositoryRuleset(t *testing.T) {
	toolDef := CreateRepositoryRuleset(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_repository_ruleset", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"level", "name", "enforcement", "rules"})
	require.NotNil(t, schema.AdditionalProperties)
	require.NotNil(t, schema.AdditionalProperties.Not)

	propertyNames := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		propertyNames = append(propertyNames, name)
	}
	assert.ElementsMatch(t, []string{
		"level",
		"owner",
		"repo",
		"org",
		"enterprise",
		"name",
		"enforcement",
		"target",
		"rules",
		"conditions",
		"bypass_actors",
	}, propertyNames)

	resolvedSchema, err := schema.Resolve(nil)
	require.NoError(t, err)
	bypassActorSchema := schema.Properties["bypass_actors"].Items
	require.NotNil(t, bypassActorSchema)
	assert.ElementsMatch(t, []string{"actor_type", "bypass_mode"}, bypassActorSchema.Required)
	assert.ElementsMatch(t, []any{"always", "pull_request", "exempt"}, bypassActorSchema.Properties["bypass_mode"].Enum)

	validArgs := map[string]any{
		"level":       "repository",
		"owner":       "owner",
		"repo":        "repo",
		"org":         "org",
		"enterprise":  "enterprise",
		"name":        "main protection",
		"enforcement": "active",
		"target":      "branch",
		"rules":       []any{map[string]any{"type": "creation"}},
		"conditions":  map[string]any{"ref_name": map[string]any{"include": []any{"refs/heads/main"}}},
		"bypass_actors": []any{
			map[string]any{"actor_type": "OrganizationAdmin", "bypass_mode": "always"},
		},
	}
	require.NoError(t, resolvedSchema.Validate(validArgs))

	t.Run("bypass actor requires an explicit documented mode", func(t *testing.T) {
		args := maps.Clone(validArgs)
		args["bypass_actors"] = []any{map[string]any{"actor_type": "OrganizationAdmin"}}
		require.Error(t, resolvedSchema.Validate(args))

		args["bypass_actors"] = []any{map[string]any{"actor_type": "OrganizationAdmin", "bypass_mode": "never"}}
		require.Error(t, resolvedSchema.Validate(args))
	})

	for _, test := range []struct {
		field string
		typo  string
	}{
		{field: "target", typo: "targte"},
		{field: "conditions", typo: "conditons"},
	} {
		t.Run("rejects misspelled "+test.field, func(t *testing.T) {
			args := maps.Clone(validArgs)
			args[test.typo] = args[test.field]
			delete(args, test.field)
			require.Error(t, resolvedSchema.Validate(args))
		})
	}

	t.Run("repository level", func(t *testing.T) {
		var capturedBody github.RepositoryRuleset
		var capturedRaw []byte
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				capturedRaw = body
				_ = json.Unmarshal(body, &capturedBody)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(body)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "main protection",
			"enforcement": "active",
			"target":      "branch",
			"rules": []any{
				map[string]any{"type": "creation"},
				map[string]any{"type": "deletion"},
				map[string]any{
					"type": "pull_request",
					"parameters": map[string]any{
						"required_approving_review_count": float64(2),
					},
				},
			},
			"conditions": map[string]any{
				"ref_name": map[string]any{
					"include": []any{"refs/heads/main"},
					"exclude": []any{},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		assert.Equal(t, "main protection", capturedBody.Name)
		assert.Equal(t, github.RulesetEnforcement("active"), capturedBody.Enforcement)
		require.NotNil(t, capturedBody.Rules)

		var outbound struct {
			Rules []struct {
				Type       string         `json:"type"`
				Parameters map[string]any `json:"parameters"`
			} `json:"rules"`
			Conditions struct {
				RefName struct {
					Include []string `json:"include"`
				} `json:"ref_name"`
			} `json:"conditions"`
		}
		require.NoError(t, json.Unmarshal(capturedRaw, &outbound))

		sentTypes := make([]string, 0, len(outbound.Rules))
		var pullRequestParams map[string]any
		for _, rule := range outbound.Rules {
			sentTypes = append(sentTypes, rule.Type)
			if rule.Type == "pull_request" {
				pullRequestParams = rule.Parameters
			}
		}
		assert.ElementsMatch(t, []string{"creation", "deletion", "pull_request"}, sentTypes)
		require.NotNil(t, pullRequestParams)
		assert.EqualValues(t, 2, pullRequestParams["required_approving_review_count"])
		assert.Equal(t, []string{"refs/heads/main"}, outbound.Conditions.RefName.Include)
	})

	t.Run("repository level requires owner and repo", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "owner")
	})

	t.Run("unsupported rule type", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{"type": "creation"},
				map[string]any{"type": "totally_made_up_rule"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "totally_made_up_rule")
		assert.False(t, called, "request must not be sent when a rule type is unsupported")
	})

	t.Run("unrecognized rule key", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{
					"type":          "pull_request",
					"configuration": map[string]any{"required_approving_review_count": float64(2)},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "configuration")
		assert.False(t, called)
	})

	t.Run("invalid rules", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules":       "not-an-array",
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "rules parameter must be an array")
	})

	t.Run("duplicate rule type", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{"type": "creation"},
				map[string]any{"type": "creation"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "duplicate rule type")
		assert.False(t, called, "request must not be sent when rules contain a duplicate type")
	})

	t.Run("unrecognized rule parameter is rejected even though the rule type is valid", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{
					"type": "pull_request",
					"parameters": map[string]any{
						"require_code_owners_review": true, // typo: should be require_code_owner_review
					},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "require_code_owners_review")
		assert.False(t, called, "request must not be sent when a rule parameter is unrecognized")
	})

	t.Run("zero-valued parameters are not flagged as unrecognized", func(t *testing.T) {
		var capturedBody []byte
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(capturedBody)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{
					"type": "pull_request",
					"parameters": map[string]any{
						"required_approving_review_count": float64(0),
						"allowed_merge_methods":           []any{},
					},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		if result.IsError {
			t.Fatalf("unexpected error: %s", getErrorResult(t, result).Text)
		}
		assert.NotEmpty(t, capturedBody)
	})

	t.Run("unrecognized key inside a rule parameter array element is rejected", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{
					"type": "required_status_checks",
					"parameters": map[string]any{
						"required_status_checks": []any{
							map[string]any{
								"context":         "ci",
								"integration_ids": float64(42), // typo: should be integration_id
							},
						},
						"strict_required_status_checks_policy": true,
					},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "required_status_checks[0].integration_ids")
		assert.False(t, called, "request must not be sent when a nested array element key is unrecognized")
	})

	t.Run("valid rule parameter array elements round-trip and are not misflagged", func(t *testing.T) {
		var capturedBody []byte
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(capturedBody)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules": []any{
				map[string]any{
					"type": "required_status_checks",
					"parameters": map[string]any{
						"required_status_checks": []any{
							map[string]any{
								"context":        "ci",
								"integration_id": float64(42),
							},
						},
						"strict_required_status_checks_policy": true,
					},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		if result.IsError {
			t.Fatalf("unexpected error: %s", getErrorResult(t, result).Text)
		}
		assert.NotEmpty(t, capturedBody)
	})

	for _, bypassMode := range []string{"always", "pull_request", "exempt"} {
		t.Run("bypass_actors serializes "+bypassMode+" exactly", func(t *testing.T) {
			var capturedRaw []byte
			client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
					capturedRaw, _ = io.ReadAll(r.Body)
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write(capturedRaw)
				},
			}))
			deps := BaseDeps{Client: client}
			handler := toolDef.Handler(deps)
			request := createMCPRequest(map[string]any{
				"level":       "repository",
				"owner":       "owner",
				"repo":        "repo",
				"name":        "main protection",
				"enforcement": "active",
				"rules":       []any{map[string]any{"type": "creation"}},
				"bypass_actors": []any{
					map[string]any{"actor_type": "OrganizationAdmin", "bypass_mode": bypassMode},
				},
			})

			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)

			var outbound struct {
				BypassActors []struct {
					BypassMode string `json:"bypass_mode"`
				} `json:"bypass_actors"`
			}
			require.NoError(t, json.Unmarshal(capturedRaw, &outbound))
			require.Len(t, outbound.BypassActors, 1)
			assert.Equal(t, bypassMode, outbound.BypassActors[0].BypassMode)
		})
	}

	t.Run("bypass_actors without bypass_mode is rejected before request", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "main protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"bypass_actors": []any{
				map[string]any{"actor_type": "OrganizationAdmin"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "bypass_actors[0].bypass_mode")
		assert.False(t, called)
	})

	t.Run("bypass_actors accepts exempt bypass mode and enterprise actor types", func(t *testing.T) {
		var capturedBody github.RepositoryRuleset
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /enterprises/{enterprise}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &capturedBody)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(body)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "enterprise",
			"enterprise":  "acme",
			"name":        "enterprise protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"bypass_actors": []any{
				map[string]any{"actor_type": "EnterpriseOwner", "bypass_mode": "exempt"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		require.Len(t, capturedBody.BypassActors, 1)
		assert.Equal(t, github.BypassActorType("EnterpriseOwner"), *capturedBody.BypassActors[0].ActorType)
		assert.Equal(t, github.BypassMode("exempt"), *capturedBody.BypassActors[0].BypassMode)
	})

	t.Run("unrecognized bypass_actors key is rejected", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"bypass_actors": []any{
				map[string]any{"actor_type": "Team", "actor_id": float64(1), "bypass_modes": "pull_request"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "bypass_modes")
		assert.False(t, called, "request must not be sent when a bypass_actors key is unrecognized")
	})

	t.Run("unrecognized top-level condition key is rejected", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"conditions": map[string]any{
				"ref_names": map[string]any{ // typo: should be ref_name
					"include": []any{"refs/heads/main"},
					"exclude": []any{},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ref_names")
		assert.False(t, called, "request must not be sent when a condition key is unrecognized")
	})

	t.Run("unrecognized nested condition key is rejected", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"conditions": map[string]any{
				"ref_name": map[string]any{
					"includes": []any{"refs/heads/main"}, // typo: should be include
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ref_name.includes")
		assert.False(t, called, "request must not be sent when a nested condition key is unrecognized")
	})

	t.Run("valid conditions round-trip and are not misflagged", func(t *testing.T) {
		var capturedBody []byte
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedBody, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(capturedBody)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "repository",
			"owner":       "owner",
			"repo":        "repo",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
			"conditions": map[string]any{
				"ref_name": map[string]any{
					"include": []any{"refs/heads/main"},
					"exclude": []any{},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		if result.IsError {
			t.Fatalf("unexpected error: %s", getErrorResult(t, result).Text)
		}
		assert.NotEmpty(t, capturedBody)
	})

	t.Run("organization level", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /orgs/{org}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(body)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "organization",
			"org":         "octo",
			"name":        "org protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "org protection", returned.Name)
	})

	t.Run("organization level requires org", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "organization",
			"name":        "org protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "org")
	})

	t.Run("enterprise level", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /enterprises/{enterprise}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write(body)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "enterprise",
			"enterprise":  "acme",
			"name":        "enterprise protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "enterprise protection", returned.Name)
	})

	t.Run("enterprise level requires enterprise", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "enterprise",
			"name":        "enterprise protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "enterprise")
	})

	t.Run("unknown level", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "planet",
			"name":        "x",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
	})

	t.Run("mismatched-case level is rejected rather than silently normalized", func(t *testing.T) {
		called := false
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"POST /orgs/{org}/rulesets": func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusCreated)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":       "Organization",
			"org":         "octo",
			"name":        "org protection",
			"enforcement": "active",
			"rules":       []any{map[string]any{"type": "creation"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
		assert.False(t, called, "a mismatched-case level must not reach the organization-level create call")

		assert.Empty(t, toolDef.ScopeAccess.Challenge(map[string]any{"level": "Organization"}, nil))
	})
}

func Test_RulesetScopeChallenges(t *testing.T) {
	tests := []struct {
		name       string
		tool       inventory.ServerTool
		arguments  map[string]any
		allowed    []string
		disallowed []string
	}{
		{
			name:       "read repository level",
			tool:       RepositoryRulesetRead(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "repository", "method": "get"},
			allowed:    []string{"repo"},
			disallowed: []string{"read:org"},
		},
		{
			name:       "read organization level",
			tool:       RepositoryRulesetRead(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "organization", "method": "list"},
			allowed:    []string{"read:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "read enterprise level",
			tool:       RepositoryRulesetRead(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "enterprise", "method": "get"},
			allowed:    []string{"read:enterprise"},
			disallowed: []string{"repo", "read:org"},
		},
		{
			name:       "read missing level defers to validation",
			tool:       RepositoryRulesetRead(translations.NullTranslationHelper),
			arguments:  map[string]any{"method": "get"},
			allowed:    nil,
			disallowed: nil,
		},
		{
			name:       "read unknown level defers to validation",
			tool:       RepositoryRulesetRead(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "planet", "method": "get"},
			allowed:    nil,
			disallowed: nil,
		},
		{
			name:       "write repository level",
			tool:       CreateRepositoryRuleset(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "repository"},
			allowed:    []string{"repo"},
			disallowed: []string{"admin:org"},
		},
		{
			name:       "write organization level",
			tool:       CreateRepositoryRuleset(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "organization"},
			allowed:    []string{"admin:org"},
			disallowed: []string{"repo"},
		},
		{
			name:       "write enterprise level",
			tool:       CreateRepositoryRuleset(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": "enterprise"},
			allowed:    []string{"admin:enterprise"},
			disallowed: []string{"repo", "admin:org"},
		},
		{
			name:       "write missing level defers to validation",
			tool:       CreateRepositoryRuleset(translations.NullTranslationHelper),
			arguments:  map[string]any{},
			allowed:    nil,
			disallowed: nil,
		},
		{
			name:       "write malformed level defers to validation",
			tool:       CreateRepositoryRuleset(translations.NullTranslationHelper),
			arguments:  map[string]any{"level": 123},
			allowed:    nil,
			disallowed: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.tool.ScopeAccess.Challenge)
			assert.True(t, tt.tool.ScopeAccess.Dynamic)
			assert.Empty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.allowed))
			if tt.disallowed == nil {
				assert.Empty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, nil))
			} else {
				assert.NotEmpty(t, tt.tool.ScopeAccess.Challenge(tt.arguments, tt.disallowed))
			}
		})
	}
}

func Test_RulesetScopeVisibility(t *testing.T) {
	readAccess := RepositoryRulesetRead(translations.NullTranslationHelper).ScopeAccess
	writeAccess := CreateRepositoryRuleset(translations.NullTranslationHelper).ScopeAccess

	assert.True(t, readAccess.Visible(nil))
	assert.False(t, writeAccess.Visible(nil))
	assert.True(t, writeAccess.Visible([]string{"repo"}))
	assert.True(t, writeAccess.Visible([]string{"admin:org"}))
	assert.True(t, writeAccess.Visible([]string{"admin:enterprise"}))
	assert.False(t, writeAccess.Visible([]string{"read:org", "read:enterprise"}))
}

func Test_RulesetScopeMetadataIsExhaustive(t *testing.T) {
	tests := []struct {
		tool      inventory.ServerTool
		maxScopes []string
	}{
		{tool: RepositoryRulesetRead(translations.NullTranslationHelper), maxScopes: []string{"repo", "read:org", "read:enterprise"}},
		{tool: CreateRepositoryRuleset(translations.NullTranslationHelper), maxScopes: []string{"repo", "admin:org", "admin:enterprise"}},
	}

	for _, tt := range tests {
		t.Run(tt.tool.Tool.Name, func(t *testing.T) {
			assert.True(t, tt.tool.ScopeAccess.Dynamic)
			assert.Equal(t, tt.maxScopes, tt.tool.ScopeAccess.Scopes)
			assert.NotNil(t, tt.tool.ScopeAccess.Challenge)
		})
	}
}
