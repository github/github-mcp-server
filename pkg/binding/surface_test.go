package binding_test

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/binding"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

func scopeCases(t *testing.T) map[binding.Kind]binding.Context {
	t.Helper()
	repo, err := binding.NewRepoContext("octocat", "hello-world")
	require.NoError(t, err)
	pull, err := binding.NewPullRequestContext("octocat", "hello-world", 42)
	require.NoError(t, err)
	project, err := binding.NewProjectContext("org", "octocat", 7)
	require.NoError(t, err)
	return map[binding.Kind]binding.Context{
		binding.KindRepo:        repo,
		binding.KindPullRequest: pull,
		binding.KindProject:     project,
	}
}

// scopedTools builds the scoped surface through the same path the server uses,
// so feature-flag variants are deduplicated exactly as an operator would see
// them by default.
func scopedTools(t *testing.T, ctx binding.Context) []inventory.ServerTool {
	t.Helper()
	th, _ := translations.TranslationHelper()
	featureSet := github.ResolveFeatureFlags(nil, false)
	checker := func(_ context.Context, flag string) (bool, error) { return featureSet[flag], nil }

	builder, err := github.NewScopedInventory(th, ctx)
	require.NoError(t, err)
	inv, err := builder.
		WithToolsets([]string{"all"}).
		WithReadOnly(false).
		WithServerInstructions().
		WithFeatureChecker(checker).
		Build()
	require.NoError(t, err)
	return inv.ToolsForRegistration(context.Background())
}

// TestManifestsAdmitOnlyRealTools is the fail-closed guard: every tool a
// manifest admits must exist in the full universe and carry a bespoke
// description. A renamed or removed tool fails here rather than silently
// dropping out of a scoped surface.
func TestManifestsAdmitOnlyRealTools(t *testing.T) {
	th, _ := translations.TranslationHelper()
	universe := map[string]bool{}
	for _, st := range github.AllTools(th) {
		universe[st.Tool.Name] = true
	}

	for _, kind := range []binding.Kind{binding.KindRepo, binding.KindPullRequest, binding.KindProject} {
		m, ok := binding.ManifestFor(kind)
		require.Truef(t, ok, "no manifest for %q", kind)
		require.NotEmptyf(t, m.Admit, "%q manifest admits no tools", kind)
		for name, tb := range m.Admit {
			require.Truef(t, universe[name], "%s manifest admits unknown tool %q", kind, name)
			require.NotEmptyf(t, tb.Description, "%s tool %q must carry a bespoke description", kind, name)
		}
	}
}

// TestApplyToolsValidatesBindings exercises the schema transform against the
// real tool schemas. ApplyTools fails if a bound or method-restricted parameter
// no longer exists, so this catches upstream schema changes that would break a
// scoped surface.
func TestApplyToolsValidatesBindings(t *testing.T) {
	th, _ := translations.TranslationHelper()
	universe := github.AllTools(th)
	for kind, ctx := range scopeCases(t) {
		_, err := binding.ApplyTools(universe, ctx)
		require.NoErrorf(t, err, "ApplyTools failed for %s", kind)
	}
}

// TestScopedSurfaceHidesContextParams asserts that no bound or rejected
// parameter survives in any advertised schema: the scoped surface must look
// native, with the context-identifying fields absent rather than merely
// ignored.
func TestScopedSurfaceHidesContextParams(t *testing.T) {
	for kind, ctx := range scopeCases(t) {
		m, _ := binding.ManifestFor(kind)
		tools := scopedTools(t, ctx)
		require.NotEmptyf(t, tools, "%s surface is empty", kind)
		for _, st := range tools {
			tb := m.Admit[st.Tool.Name]
			schema := st.Tool.InputSchema.(*jsonschema.Schema)
			for param := range tb.Bind {
				require.NotContainsf(t, schema.Properties, param, "%s/%s still advertises bound param %q", kind, st.Tool.Name, param)
			}
			for _, param := range tb.ParamReject {
				require.NotContainsf(t, schema.Properties, param, "%s/%s still advertises rejected param %q", kind, st.Tool.Name, param)
			}
		}
	}
}

// TestScopedToolsnaps locks the advertised schema of every tool on every scoped
// surface, plus the membership of each surface, under per-surface snapshot
// subfolders (pkg/binding/__toolsnaps__/{repo,pull_request,project}/). Any
// change to a shared tool's schema, or to a manifest, shows up here and in the
// mcp-diff workflow. Run with UPDATE_TOOLSNAPS=true to regenerate.
func TestScopedToolsnaps(t *testing.T) {
	for kind, ctx := range scopeCases(t) {
		tools := scopedTools(t, ctx)
		require.NotEmptyf(t, tools, "%s surface is empty", kind)

		names := make([]string, 0, len(tools))
		for _, st := range tools {
			names = append(names, st.Tool.Name)
			snap := filepath.Join(string(kind), st.Tool.Name)
			require.NoError(t, toolsnaps.Test(snap, st.Tool))
		}
		sort.Strings(names)
		require.NoError(t, toolsnaps.Test(filepath.Join(string(kind), "_surface"), names))
	}
}
