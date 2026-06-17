package github

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func invokeIssueToFixPrompt(t *testing.T, args map[string]string) *mcp.GetPromptResult {
	t.Helper()

	prompt := IssueToFixWorkflowPrompt(stubTranslation)
	require.Equal(t, "issue_to_fix_workflow", prompt.Prompt.Name)
	require.NotNil(t, prompt.Handler)

	result, err := prompt.Handler(context.Background(), &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{Arguments: args},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	return result
}

func TestIssueToFixWorkflowPrompt(t *testing.T) {
	t.Run("interpolates required args and includes optional labels and assignees", func(t *testing.T) {
		result := invokeIssueToFixPrompt(t, map[string]string{
			"owner":       "octo-org",
			"repo":        "octo-repo",
			"title":       "Fix the flaky test",
			"description": "The login test fails intermittently",
			"labels":      "bug,flaky",
			"assignees":   "alice,bob",
		})

		require.Len(t, result.Messages, 5)

		roles := make([]string, len(result.Messages))
		for i, m := range result.Messages {
			roles[i] = string(m.Role)
		}
		assert.Equal(t, []string{"user", "user", "assistant", "user", "assistant"}, roles)

		// message[1] is the user request carrying the interpolated details.
		userRequest := result.Messages[1].Content.(*mcp.TextContent).Text
		assert.Contains(t, userRequest, "Fix the flaky test")
		assert.Contains(t, userRequest, "octo-org/octo-repo")
		assert.Contains(t, userRequest, "The login test fails intermittently")
		assert.Contains(t, userRequest, "Labels to apply: bug,flaky")
		assert.Contains(t, userRequest, "Assignees: alice,bob")

		// message[2] is the assistant acknowledgement referencing title/owner/repo.
		assistantAck := result.Messages[2].Content.(*mcp.TextContent).Text
		assert.Contains(t, assistantAck, "Fix the flaky test")
		assert.Contains(t, assistantAck, "octo-org/octo-repo")
	})

	t.Run("omits optional sections when labels and assignees are absent", func(t *testing.T) {
		result := invokeIssueToFixPrompt(t, map[string]string{
			"owner":       "octo-org",
			"repo":        "octo-repo",
			"title":       "Add docs",
			"description": "Document the API",
		})

		require.Len(t, result.Messages, 5)
		userRequest := result.Messages[1].Content.(*mcp.TextContent).Text
		assert.NotContains(t, userRequest, "Labels to apply:")
		assert.NotContains(t, userRequest, "Assignees:")
	})
}
