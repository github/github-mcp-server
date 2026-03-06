package ghmcp

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func makeScopeTestTool(
	name string,
	readOnly bool,
	requiredScopes []string,
	acceptedScopes []string,
) inventory.ServerTool {
	return inventory.ServerTool{
		Tool: mcp.Tool{
			Name: name,
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint: readOnly,
			},
		},
		RequiredScopes: requiredScopes,
		AcceptedScopes: acceptedScopes,
	}
}

func TestShouldValidateTokenScopesAtStartup(t *testing.T) {
	require.True(t, shouldValidateTokenScopesAtStartup("ghp_test"))
	require.True(t, shouldValidateTokenScopesAtStartup("gho_test"))
	require.False(t, shouldValidateTokenScopesAtStartup("ghs_test"))
	require.False(t, shouldValidateTokenScopesAtStartup("github_pat_test"))
}

func TestEvaluateScopeRequirementsReportsMissingScopesAndBlockedTools(t *testing.T) {
	tools := []inventory.ServerTool{
		makeScopeTestTool(
			"repo_write",
			false,
			[]string{"repo"},
			[]string{"repo"},
		),
	}

	missingScopes, blockedTools, err := evaluateScopeRequirements(tools, []string{})
	require.NoError(t, err)
	require.Equal(t, []string{"repo"}, missingScopes)
	require.Equal(t, []string{"repo_write"}, blockedTools)
}

func TestEvaluateScopeRequirementsAllowsReadOnlyRepoToolsWithoutScopes(t *testing.T) {
	tools := []inventory.ServerTool{
		makeScopeTestTool(
			"repo_read_only",
			true,
			[]string{"repo"},
			[]string{"repo", "public_repo"},
		),
	}

	missingScopes, blockedTools, err := evaluateScopeRequirements(tools, []string{})
	require.NoError(t, err)
	require.Empty(t, missingScopes)
	require.Empty(t, blockedTools)
}

func TestEvaluateScopeRequirementsSortsOutputDeterministically(t *testing.T) {
	tools := []inventory.ServerTool{
		makeScopeTestTool(
			"z_tool",
			false,
			[]string{"admin:org"},
			[]string{"admin:org"},
		),
		makeScopeTestTool(
			"a_tool",
			false,
			[]string{"repo"},
			[]string{"repo"},
		),
	}

	missingScopes, blockedTools, err := evaluateScopeRequirements(tools, []string{})
	require.NoError(t, err)
	require.Equal(t, []string{"admin:org", "repo"}, missingScopes)
	require.Equal(t, []string{"a_tool", "z_tool"}, blockedTools)
}
