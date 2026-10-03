package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

func Test_SearchModeNaturalLanguageAndSyntax(t *testing.T) {
	for _, q := range []string{
		"login does not work after password reset", "connecting to postgres and redis", "should I use hooks or plugins",
		`why does "this AND that" fail`, `explain "foo OR bar"`, `why "NOT ready" appears`,
		`explain "label:bug" text`, `why "escaped \" AND operator" fails`,
	} {
		t.Run(q, func(t *testing.T) {
			got, err := resolveIssuesSearchMode(searchModeSemantic, map[string]any{"query": q})
			require.NoError(t, err)
			require.Equal(t, searchModeSemantic, got)
		})
	}
	for _, q := range []string{"foo AND bar", "foo OR bar", "foo NOT bar", "foo AND(bar OR baz)", `label:"needs triage"`, `repo:owner/repo`, `-author:bot`, `(label:bug OR label:critical)`} {
		t.Run(q, func(t *testing.T) {
			got, err := resolveIssuesSearchMode(searchModeSemantic, map[string]any{"query": q})
			require.NoError(t, err)
			require.Equal(t, searchModeLexical, got)
			got, err = resolveIssuesSearchMode(searchModeSemantic, map[string]any{"query": q, "search_type": "semantic"})
			require.NoError(t, err)
			require.Equal(t, searchModeSemantic, got)
		})
	}
}

func Test_SearchIssuesGHESRejectsSemanticOverride(t *testing.T) {
	got, err := resolveIssuesSearchMode(searchModeLexical, map[string]any{"query": "question", "search_type": "semantic"})
	require.ErrorContains(t, err, "not supported")
	require.NotEqual(t, searchModeSemantic, got)
	tool := SearchIssues(translations.NullTranslationHelper, WithHost(utils.HostTypeGHES))
	require.Equal(t, []any{"lexical"}, tool.Tool.InputSchema.(*jsonschema.Schema).Properties["search_type"].Enum)
	// No client: the capability error must be returned before attempting a request.
	deps := &BaseDeps{}
	request := createMCPRequest(map[string]any{"query": "question", "search_type": "semantic"})
	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
}
