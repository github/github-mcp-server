package inventory

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func wrapHandlerWithStructuredContent(next mcp.ToolHandler) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := next(ctx, req)
		if err != nil || result == nil || result.IsError || result.StructuredContent != nil {
			return result, err
		}

		structuredContent, ok, err := structuredContentFromText(result)
		if err != nil {
			return nil, err
		}
		if ok {
			result.StructuredContent = structuredContent
		}

		return result, nil
	}
}

func structuredContentFromText(result *mcp.CallToolResult) (json.RawMessage, bool, error) {
	if len(result.Content) != 1 {
		return nil, false, nil
	}

	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		return nil, false, nil
	}

	raw := json.RawMessage(text.Text)
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, false, fmt.Errorf("output schema enabled but text content is not a JSON object: %w", err)
	}

	return raw, true, nil
}
