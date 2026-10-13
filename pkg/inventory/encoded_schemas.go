package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listedToolSchemas struct {
	source       any
	outputSource any
	registered   *mcp.Tool
}

type encodedSchemaKey struct {
	schema *jsonschema.Schema
	// Input schemas receive header annotations at registration; output schemas do not.
	input bool
}

var encodedSchemas sync.Map // encodedSchemaKey -> func() (json.RawMessage, error)

func encodeListedSchema(source, registered, listed any, input bool) (any, error) {
	schema, ok := source.(*jsonschema.Schema)
	if !ok || schema == nil {
		return listed, nil
	}
	finalSchema, ok := registered.(*jsonschema.Schema)
	if !ok || finalSchema == nil {
		return listed, nil
	}
	if listedSchema, ok := listed.(*jsonschema.Schema); !ok || listedSchema != finalSchema {
		// Another middleware or a replacement tool may provide a different schema.
		return listed, nil
	}

	key := encodedSchemaKey{schema: schema, input: input}
	encode, ok := encodedSchemas.Load(key)
	if !ok {
		// Key by the source definition: header annotation clones the input schema
		// on each HTTP registration, but its encoded representation is unchanged.
		encode, _ = encodedSchemas.LoadOrStore(key, sync.OnceValues(func() (json.RawMessage, error) {
			data, err := json.Marshal(finalSchema)
			return json.RawMessage(data), err
		}))
	}
	return encode.(func() (json.RawMessage, error))()
}

func encodedToolSchemasMiddleware(schemas map[string]listedToolSchemas) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != MCPMethodToolsList {
				return result, err
			}
			list, ok := result.(*mcp.ListToolsResult)
			if !ok || list == nil {
				return result, nil
			}
			patched := *list
			if list.Tools != nil {
				patched.Tools = make([]*mcp.Tool, len(list.Tools))
			}
			for i, tool := range list.Tools {
				definition, ok := schemas[tool.Name]
				if !ok {
					patched.Tools[i] = tool
					continue
				}
				toolCopy := *tool
				toolCopy.InputSchema, err = encodeListedSchema(definition.source, definition.registered.InputSchema, tool.InputSchema, true)
				if err != nil {
					return nil, fmt.Errorf("encode input schema for %q: %w", tool.Name, err)
				}
				outputSource := definition.outputSource
				if outputSource == nil {
					outputSource = definition.registered.OutputSchema
				}
				toolCopy.OutputSchema, err = encodeListedSchema(outputSource, definition.registered.OutputSchema, tool.OutputSchema, false)
				if err != nil {
					return nil, fmt.Errorf("encode output schema for %q: %w", tool.Name, err)
				}
				patched.Tools[i] = &toolCopy
			}
			return &patched, nil
		}
	}
}
