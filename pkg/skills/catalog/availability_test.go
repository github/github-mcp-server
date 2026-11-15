package catalog_test

import (
	"context"
	"sort"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/github"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/skills/catalog"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tools have no suitable bundled workflow today. Add an explicit,
// specific reason here when adding a tool without catalog coverage.
var toolCoverageExemptions = map[string]string{
	"add_issue_comment_reaction":                  "Reactions are optional social signals, not a workflow in the current skills.",
	"add_issue_reaction":                          "Reactions are optional social signals, not a workflow in the current skills.",
	"add_pull_request_review_comment_reaction":    "Reactions are optional social signals, not a workflow in the current skills.",
	"create_gist":                                 "Gist lifecycle is separate from the repository workflows covered by the current catalog.",
	"create_repository_ruleset":                   "Repository governance configuration is a distinct workflow not covered by the current catalog.",
	"custom_properties_read":                      "Custom-property administration is a distinct governance workflow not covered by the current catalog.",
	"custom_properties_write":                     "Custom-property administration is a distinct governance workflow not covered by the current catalog.",
	"get_gist":                                    "Gist lifecycle is separate from the repository workflows covered by the current catalog.",
	"hide_issue_comment":                          "Comment moderation is outside the current issue authoring and backlog workflows.",
	"hide_pull_request_review":                    "Review moderation is outside the current pull-request authoring and review workflows.",
	"hide_pull_request_review_comment":            "Review moderation is outside the current pull-request authoring and review workflows.",
	"issue_artifact_read":                         "Issue artifact management is a separate workflow not covered by the current catalog.",
	"issue_artifact_write":                        "Issue artifact management is a separate workflow not covered by the current catalog.",
	"issue_dependency_read":                       "Issue dependency graph management is distinct from the current backlog triage workflow.",
	"issue_dependency_write":                      "Issue dependency graph management is distinct from the current backlog triage workflow.",
	"list_gists":                                  "Gist lifecycle is separate from the repository workflows covered by the current catalog.",
	"remove_issue_comment_reaction":               "Reactions are optional social signals, not a workflow in the current skills.",
	"remove_issue_reaction":                       "Reactions are optional social signals, not a workflow in the current skills.",
	"remove_pull_request_review_comment_reaction": "Reactions are optional social signals, not a workflow in the current skills.",
	"repository_ruleset_read":                     "Repository governance configuration is a distinct workflow not covered by the current catalog.",
	"ui_get":                                      "This is an app-only UI support tool, not an agent workflow.",
	"unhide_issue_comment":                        "Comment moderation is outside the current issue authoring and backlog workflows.",
	"unhide_pull_request_review":                  "Review moderation is outside the current pull-request authoring and review workflows.",
	"unhide_pull_request_review_comment":          "Review moderation is outside the current pull-request authoring and review workflows.",
	"update_gist":                                 "Gist lifecycle is separate from the repository workflows covered by the current catalog.",
	"update_issue_comment":                        "Editing existing issue comments is outside the current authoring and backlog workflows.",
}

func TestCatalogToolRequirementsAreRegistered(t *testing.T) {
	t.Parallel()
	known := make(map[string]bool)
	for _, tool := range github.AllTools(translations.NullTranslationHelper) {
		known[tool.Tool.Name] = true
	}
	source, err := catalog.New()
	require.NoError(t, err)
	entries, err := source.List(context.Background())
	require.NoError(t, err)
	covered := make(map[string]bool)
	for _, entry := range entries {
		skill, ok := source.Skill(catalog.Authority + "/" + entry.Frontmatter.Name())
		require.True(t, ok)
		require.NotEmpty(t, skill.RequiredTools)
		names := append([]string{}, skill.OptionalTools...)
		for _, group := range skill.RequiredTools {
			require.NotEmpty(t, group)
			names = append(names, group...)
		}
		for _, name := range names {
			assert.True(t, known[name], "%s maps unknown tool %s", skill.Name(), name)
			covered[name] = true
		}
		assert.NotContains(t, entry.Frontmatter, "allowed-tools")
	}

	var unclassified []string
	for name := range known {
		if covered[name] {
			continue
		}
		if reason, exempt := toolCoverageExemptions[name]; exempt && reason != "" {
			continue
		}
		unclassified = append(unclassified, name)
	}
	sort.Strings(unclassified)
	assert.Empty(t, unclassified,
		"add each uncovered tool to requirements in pkg/skills/catalog/tools.go, or give it a reason in toolCoverageExemptions; verify with go test ./pkg/skills/catalog -run TestCatalogToolRequirementsAreRegistered -count=1")

	for name, reason := range toolCoverageExemptions {
		assert.True(t, known[name], "remove stale tool coverage exemption for unregistered tool %q", name)
		assert.NotEmpty(t, reason, "explain why tool %q has no matching bundled skill", name)
	}
}

func TestCatalogAvailabilityModes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		toolsets []string
		tools    []string
		exclude  []string
		readOnly bool
		noScopes bool
		features []string
		present  []string
		absent   []string
	}{
		{name: "context", toolsets: []string{"context"}, present: []string{"account-and-team-context"}, absent: []string{"issue-authoring", "pull-request-authoring"}},
		{name: "explicit", toolsets: []string{}, tools: []string{"search_repositories"}, present: []string{"repository-discovery"}, absent: []string{"account-and-team-context"}},
		{name: "readonly", readOnly: true, present: []string{"codebase-exploration"}, absent: []string{"issue-authoring", "pull-request-authoring", "repository-content-changes"}},
		{name: "exclude", exclude: []string{"get_me", "create_pull_request"}, absent: []string{"account-and-team-context", "pull-request-authoring"}},
		{name: "optional absent", toolsets: []string{}, tools: []string{"create_pull_request"}, present: []string{"pull-request-authoring"}},
		{name: "no scopes", noScopes: true, absent: []string{"issue-authoring", "repository-content-changes"}},
		{name: "granular", features: []string{github.FeatureFlagIssuesGranular, github.FeatureFlagPullRequestsGranular}, present: []string{"issue-authoring", "issue-backlog-triage", "pull-request-code-review"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flags := github.ResolveFeatureFlags(tt.features, false)
			checker := func(_ context.Context, flag string) (bool, error) { return flags[flag], nil }
			builder := github.NewInventory(translations.NullTranslationHelper).
				WithToolsets(tt.toolsets).WithTools(tt.tools).WithExcludeTools(tt.exclude).
				WithReadOnly(tt.readOnly).WithFeatureChecker(checker)
			if tt.noScopes {
				builder.WithFilter(github.CreateToolScopeFilter([]string{}))
			}
			inv, err := builder.Build()
			require.NoError(t, err)
			available := make(map[string]bool)
			for _, tool := range inv.ForMCPRequest(inventory.MCPMethodResourcesRead, "").ConfiguredTools(context.Background()) {
				available[tool.Tool.Name] = true
			}
			source, err := catalog.New()
			require.NoError(t, err)
			filtered, err := source.ForTools(func(name string) bool { return available[name] })
			require.NoError(t, err)
			for _, name := range tt.present {
				_, found := filtered.Skill(catalog.Authority + "/" + name)
				assert.True(t, found, "expected %s", name)
			}
			for _, name := range tt.absent {
				_, found := filtered.Skill(catalog.Authority + "/" + name)
				assert.False(t, found, "must withhold %s", name)
			}
		})
	}
}
