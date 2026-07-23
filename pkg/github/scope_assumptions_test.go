package github

import (
	"context"
	"testing"

	ghoauth "github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file pins the deliberate, non-obvious assumptions baked into the OAuth
// scope model so they stay visible to developers revisiting them. Each test
// documents WHAT we assume, WHY, and the escape hatch if the assumption ever
// needs to change. If one of these fails, treat it as a prompt to make a
// conscious decision, not to silence the test.

// TestAssumption_PATShowsRepoToolsButOAuthChallengesForRepo encodes the
// intentional asymmetry between the two enforcement paths for a read-only tool
// whose only requirement is repo-ish access:
//
//   - PAT filtering (CreateToolScopeFilter) is best-effort and keeps such tools
//     VISIBLE even when the token advertises no scopes, because they still work
//     against PUBLIC repositories and that access is useful.
//   - The OAuth scope-challenge path (scopes.ToolScopeInfo.Satisfies /
//     HasRequiredScopes) has NO such exception: it treats `repo` as genuinely
//     required and will challenge the user to grant it.
//
// In other words: we'd rather show-and-let-the-API-decide for PATs, but
// proactively request the scope for OAuth where challenging is cheap and clean.
func TestAssumption_PATShowsRepoToolsButOAuthChallengesForRepo(t *testing.T) {
	readOnlyRepoTool := &inventory.ServerTool{
		Tool: mcp.Tool{
			Name:        "read_only_repo_tool",
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
		},
		RequiredScopes: []string{"repo"},
	}

	// PAT side: shown even with no token scopes (public-repo access is useful).
	patFilter := CreateToolScopeFilter([]string{})
	shown, err := patFilter(context.Background(), readOnlyRepoTool)
	require.NoError(t, err)
	assert.True(t, shown, "PAT filtering should keep a read-only repo tool visible without any scope (public repo access)")

	// OAuth side: the same requirement is NOT satisfied by an empty scope set,
	// so the challenge middleware would request `repo`.
	assert.False(t, scopes.HasRequiredScopes([]string{}, []string{"repo"}),
		"OAuth challenge model must treat repo as required (no public-repo exception)")
	info := &scopes.ToolScopeInfo{RequiredScopes: []string{"repo"}}
	assert.False(t, info.Satisfies(),
		"Satisfies must report unsatisfied for a repo tool with no granted scopes (triggers a challenge)")
	assert.Equal(t, []string{"repo"}, info.MissingScopes(),
		"the challenge should ask for exactly the missing repo scope")
}

// TestAssumption_WorkflowScopeIsGrantableButNeverChallenged encodes that the
// `workflow` scope is intentionally reachable only as an up-front grant, never
// via an on-demand scope challenge:
//
//   - It IS advertised in oauth.SupportedScopes, so a classic PAT can carry it
//     and the default OAuth login can request it up front.
//   - But NO tool declares it as a required scope, so the challenge path can
//     never ask for it on demand. (There is also deliberately no scopes.Workflow
//     constant, so a tool cannot declare it via the typed API without someone
//     first adding the constant.)
//
// This is a conscious risk-aversion choice: `workflow` grants control over
// GitHub Actions workflow files, so we don't auto-request it. If a tool ever
// genuinely needs it, the path is: add a scopes.Workflow constant, declare it on
// the tool, and accept that the challenge will then request `workflow` (it is
// already in SupportedScopes, so the mechanics work). This test will fail at
// that point to force that decision to be made deliberately.
func TestAssumption_WorkflowScopeIsGrantableButNeverChallenged(t *testing.T) {
	assert.Contains(t, ghoauth.SupportedScopes, "workflow",
		"workflow should remain a supported/grantable scope (PATs carry it; OAuth can request it up front)")

	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"all"}).
		Build()
	require.NoError(t, err)

	for _, tool := range inv.AllTools() {
		assert.NotContains(t, tool.RequiredScopes, "workflow",
			"tool %q declares the workflow scope as required; the OAuth challenge path would then request it. "+
				"That is an intentional escape hatch — update this test and confirm the risk is acceptable.", tool.Tool.Name)
	}
}

// TestAssumption_MultiScopeRequirementsAreTreatedAsAND encodes that when a tool
// declares more than one required scope we treat them as a conjunction (ALL
// required), because the declaration ([]scopes.Scope) cannot express "any of".
// We cannot distinguish a genuine hard-AND from a genuine hard-OR, so we
// conservatively require all of them. Hierarchy substitution still applies, so
// an ancestor scope satisfies a required descendant.
//
// If a real OR requirement ever appears, the escape hatch is to extend the
// model to OR-groups (AND across groups, OR within a group); see
// scopes.HasRequiredScopes. Until then, AND is the deliberate default.
func TestAssumption_MultiScopeRequirementsAreTreatedAsAND(t *testing.T) {
	required := []string{"repo", "read:org"}

	// AND: one of the two scopes is not enough.
	assert.False(t, scopes.HasRequiredScopes([]string{"repo"}, required),
		"a token with only repo must NOT satisfy a {repo, read:org} tool (treated as AND, not OR)")

	// Both scopes present satisfies the conjunction.
	assert.True(t, scopes.HasRequiredScopes([]string{"repo", "read:org"}, required),
		"a token holding both required scopes satisfies the conjunction")

	// Hierarchy substitution still applies on top of AND: admin:org grants read:org.
	assert.True(t, scopes.HasRequiredScopes([]string{"repo", "admin:org"}, required),
		"an ancestor scope (admin:org) still satisfies a required descendant (read:org) under AND")
}
