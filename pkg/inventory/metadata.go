package inventory

import (
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func cloneMeta(meta mcp.Meta) mcp.Meta {
	if meta == nil {
		return nil
	}
	clone := make(mcp.Meta, len(meta))
	for key, value := range meta {
		clone[key] = cloneMetaValue(value)
	}
	return clone
}

func cloneMetaValue(value any) any {
	switch value := value.(type) {
	case mcp.Meta:
		return cloneMeta(value)
	case map[string]any:
		clone := make(map[string]any, len(value))
		for key, item := range value {
			clone[key] = cloneMetaValue(item)
		}
		return clone
	case []any:
		clone := make([]any, len(value))
		for i, item := range value {
			clone[i] = cloneMetaValue(item)
		}
		return clone
	case []string:
		return slices.Clone(value)
	default:
		return value
	}
}
