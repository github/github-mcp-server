package github

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MustOutputSchema infers an object output schema for T or panics during tool initialization.
func MustOutputSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		var zero T
		panic(fmt.Sprintf("failed to infer output schema for %T: %v", zero, err))
	}
	if schema.Type != "object" {
		var zero T
		panic(fmt.Sprintf("output schema for %T must have type object, got %q", zero, schema.Type))
	}
	return schema
}

func outputSchemasEnabled(ctx context.Context, deps ToolDependencies) bool {
	return deps.IsFeatureEnabled(ctx, FeatureFlagOutputSchemas)
}

func structuredTextResult(ctx context.Context, deps ToolDependencies, textValue, structuredContent any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(textValue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	result := utils.NewToolResultText(string(data))
	if outputSchemasEnabled(ctx, deps) {
		result.StructuredContent = structuredContent
	}

	return result, nil
}
