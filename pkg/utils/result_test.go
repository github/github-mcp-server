package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pending form is a successful handoff, not a failed tool call. Hosts drop
// error results from MCP App rendering, so IsError here hides the form.
func TestNewToolResultAwaitingFormSubmissionIsNotAnError(t *testing.T) {
	result := NewToolResultAwaitingFormSubmission("wait for the form")
	assert.False(t, result.IsError, "pending form handoff must not be an error result")
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "wait for the form", text.Text)
	payload, ok := result.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "awaiting_user_submission", payload["status"])
}
