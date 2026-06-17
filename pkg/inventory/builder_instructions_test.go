package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestBuilderWithServerInstructions(t *testing.T) {
	// Ensure the test-only opt-out is not inherited from the environment.
	t.Setenv("DISABLE_INSTRUCTIONS", "")

	tools := []ServerTool{mockTool("tool1", "toolset1", true)}

	// Without WithServerInstructions, no instructions are generated.
	plain := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}))
	assert.Empty(t, plain.Instructions())

	// With it, the base server instructions are present.
	withInstructions := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithServerInstructions())
	assert.Contains(t, withInstructions.Instructions(), "The GitHub MCP Server provides tools")
}

func TestBuilderWithServerInstructions_ToolsetInstructionsFunc(t *testing.T) {
	t.Setenv("DISABLE_INSTRUCTIONS", "")

	toolset := ToolsetMetadata{
		ID:          "custom",
		Description: "custom toolset",
		InstructionsFunc: func(_ *Inventory) string {
			return "CUSTOM TOOLSET INSTRUCTIONS"
		},
	}
	tool := NewServerTool(
		mcp.Tool{Name: "custom_tool", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		toolset,
		func(_ context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{}, nil
		},
	)

	inv := mustBuild(t, NewBuilder().SetTools([]ServerTool{tool}).WithToolsets([]string{"all"}).WithServerInstructions())
	assert.Contains(t, inv.Instructions(), "CUSTOM TOOLSET INSTRUCTIONS")
}
