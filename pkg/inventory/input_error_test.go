package inventory

import (
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestToolInputErrorPreservesText(t *testing.T) {
	result := invalidArgumentsResult(fmt.Errorf("normalize tool arguments: %w", &ToolInputError{Message: "missing required parameter: title"}))
	assert.True(t, result.IsError)
	assert.Equal(t, "missing required parameter: title", result.Content[0].(*mcp.TextContent).Text)
	assert.Nil(t, result.StructuredContent)

	result = invalidArgumentsResult(fmt.Errorf("ordinary error"))
	assert.Equal(t, "invalid arguments: ordinary error", result.Content[0].(*mcp.TextContent).Text)
}
