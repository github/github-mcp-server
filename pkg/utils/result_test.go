package utils

import (
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolResultAwaitingFormSubmission_IncludesViewUUID(t *testing.T) {
	t.Parallel()

	result := NewToolResultAwaitingFormSubmission("showing form")
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	require.NotNil(t, result.Meta)

	viewUUID, ok := result.Meta["viewUUID"].(string)
	require.True(t, ok, "viewUUID should be a string")
	assert.NotEmpty(t, viewUUID)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, viewUUID)

	status, ok := result.StructuredContent.(map[string]any)["status"]
	require.True(t, ok)
	assert.Equal(t, "awaiting_user_submission", status)
}

func TestNewToolResultAwaitingFormSubmission_UniqueViewUUIDs(t *testing.T) {
	t.Parallel()

	a := NewToolResultAwaitingFormSubmission("a")
	b := NewToolResultAwaitingFormSubmission("b")
	require.NotEqual(t, a.Meta["viewUUID"], b.Meta["viewUUID"])
}

// TestNewToolResultAwaitingFormSubmission_WireFormatMeta verifies the host-facing
// JSON shape: MCP Apps read result._meta.viewUUID after the host re-delivers the
// original deferral on remount (github/github-mcp-server#2965).
func TestNewToolResultAwaitingFormSubmission_WireFormatMeta(t *testing.T) {
	t.Parallel()

	result := NewToolResultAwaitingFormSubmission("showing form")
	raw, err := json.Marshal(result)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))

	meta, ok := payload["_meta"].(map[string]any)
	require.True(t, ok, "wire JSON must include _meta; got: %s", string(raw))
	viewUUID, ok := meta["viewUUID"].(string)
	require.True(t, ok, "_meta.viewUUID must be a string; got: %s", string(raw))
	assert.NotEmpty(t, viewUUID)
	assert.Equal(t, true, payload["isError"])

	// Round-trip: host re-sends the same JSON → UI must recover the same UUID.
	var decoded mcp.CallToolResult
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NotNil(t, decoded.Meta)
	assert.Equal(t, viewUUID, decoded.Meta["viewUUID"])
}
