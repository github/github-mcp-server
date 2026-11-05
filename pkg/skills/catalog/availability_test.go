package catalog_test

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/github"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/skills/catalog"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		}
		assert.NotContains(t, entry.Frontmatter, "allowed-tools")
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
