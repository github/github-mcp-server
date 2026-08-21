package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
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
	assert.ElementsMatch(t, schema.Required, []string{"method", "owner", "repo"})

	t.Run("get defaults includes_parents to true", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/{ruleset_id}": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "main protection", Enforcement: "active"})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo", "ruleset_id": float64(42)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "true", capturedQuery.Get("includes_parents"))

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "main protection", returned.Name)
	})

	t.Run("get forwards explicit includes_parents=false", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/{ruleset_id}": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "rs"})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo", "ruleset_id": float64(42), "includes_parents": false})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "false", capturedQuery.Get("includes_parents"))
	})

	t.Run("get requires ruleset_id", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ruleset_id")
	})

	t.Run("list omits includes_parents when not provided", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{{Name: "rs1"}})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "list", "owner": "owner", "repo": "repo", "perPage": float64(50)})

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

	t.Run("list forwards explicit includes_parents=false", func(t *testing.T) {
		var capturedQuery url.Values
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets": func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.Query()
				mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{})(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "list", "owner": "owner", "repo": "repo", "includes_parents": false})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, "false", capturedQuery.Get("includes_parents"))
	})

	t.Run("get_rules_for_branch", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rules/branches/{branch}": mockResponse(t, http.StatusOK, []map[string]any{{"type": "creation"}}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get_rules_for_branch", "owner": "owner", "repo": "repo", "branch": "main"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "Creation")
	})

	t.Run("get_rules_for_branch requires branch", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get_rules_for_branch", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "branch")
	})

	t.Run("list_rule_suites forwards filters", func(t *testing.T) {
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
			"method":            "list_rule_suites",
			"owner":             "owner",
			"repo":              "repo",
			"ref":               "refs/heads/main",
			"time_period":       "week",
			"actor_name":        "octocat",
			"rule_suite_result": "pass",
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
		assert.Equal(t, "25", capturedQuery.Get("per_page"))
	})

	t.Run("get_rule_suite", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/rulesets/rule-suites/{rule_suite_id}": mockResponse(t, http.StatusOK, map[string]any{"id": 101, "result": "fail"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get_rule_suite", "owner": "owner", "repo": "repo", "rule_suite_id": float64(101)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "fail")
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "frobnicate", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})
}

func Test_OrganizationRepositoryRulesetRead(t *testing.T) {
	toolDef := OrganizationRepositoryRulesetRead(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "organization_repository_ruleset_read", toolDef.Tool.Name)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"method", "org"})

	t.Run("get", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets/{ruleset_id}": mockResponse(t, http.StatusOK, &github.RepositoryRuleset{Name: "org rs", Enforcement: "active"}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get", "org": "octo", "ruleset_id": float64(7)})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		assert.Equal(t, "org rs", returned.Name)
	})

	t.Run("get requires ruleset_id", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "get", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "ruleset_id")
	})

	t.Run("list", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/rulesets": mockResponse(t, http.StatusOK, []*github.RepositoryRuleset{{Name: "org rs"}}),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "list", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []*github.RepositoryRuleset
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "org rs", returned[0].Name)
	})

	t.Run("unknown method", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"method": "frobnicate", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown method")
	})
}

func Test_CreateRepositoryRuleset(t *testing.T) {
	toolDef := CreateRepositoryRuleset(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_repository_ruleset", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "name", "enforcement", "rules"})

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

	// Verify the outbound body preserves all requested rules and the pull_request
	// parameters, rather than silently dropping them in the JSON round-trip.
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
}

func Test_CreateRepositoryRuleset_UnsupportedRuleType(t *testing.T) {
	toolDef := CreateRepositoryRuleset(translations.NullTranslationHelper)
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
}

func Test_CreateRepositoryRuleset_InvalidRules(t *testing.T) {
	toolDef := CreateRepositoryRuleset(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{
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
}

func Test_CreateOrganizationRepositoryRuleset(t *testing.T) {
	toolDef := CreateOrganizationRepositoryRuleset(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_organization_repository_ruleset", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"org", "name", "enforcement", "rules"})

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
}

func Test_CreateEnterpriseRepositoryRuleset(t *testing.T) {
	toolDef := CreateEnterpriseRepositoryRuleset(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_enterprise_repository_ruleset", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"enterprise", "name", "enforcement", "rules"})

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
}
