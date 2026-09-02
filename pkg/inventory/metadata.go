package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func normalizeMeta(meta mcp.Meta) (mcp.Meta, error) {
	if meta == nil {
		return nil, nil
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	var normalized mcp.Meta
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&normalized); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return normalized, nil
}

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
	case nil, bool, string, json.Number:
		return value
	default:
		panic(fmt.Sprintf("metadata value %T is not normalized", value))
	}
}
