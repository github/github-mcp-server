package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type typedValidationInput struct {
	Mode  string `json:"mode"`
	Count int    `json:"count"`
}

func compareTypedInputWithSDK[In any](t *testing.T, advertised, validation *jsonschema.Schema) {
	t.Helper()
	type output struct {
		Input In `json:"input"`
	}
	handler := func(_ context.Context, _ *mcp.CallToolRequest, input In) (*mcp.CallToolResult, output, error) {
		return nil, output{Input: input}, nil
	}
	tool := NewServerToolWithContextHandlerAndSchemaOptions(
		mcp.Tool{Name: "validation", InputSchema: advertised, OutputSchema: &jsonschema.Schema{Type: "object"}},
		testToolsetMetadata("test"),
		handler,
		TypedSchemaOptions{ValidationInputSchema: validation},
	)
	typedServer := mcp.NewServer(&mcp.Implementation{Name: "typed", Version: "test"}, nil)
	tool.RegisterFunc(typedServer, nil)
	typedSession := connectTypedTestClient(t, typedServer, "")
	sdkServer := mcp.NewServer(&mcp.Implementation{Name: "sdk", Version: "test"}, nil)
	mcp.AddTool(sdkServer, &mcp.Tool{
		Name: "validation", InputSchema: validation, OutputSchema: &jsonschema.Schema{Type: "object"},
	}, handler)
	sdkSession := connectTypedTestClient(t, sdkServer, "")
	for _, raw := range []string{
		`{"mode":"valid"}`, `{"mode":"valid","count":9}`, `{"mode":"valid","Count":9}`,
		`{"mode":"invalid"}`, `{"mode":42}`, `{}`, `{"mode":"valid","count":"bad"}`,
		`{"mode":"valid","owner":"octo"}`, `{"mode":"valid","owner":"invalid"}`,
		`{"mode":"valid","owner":42}`, `{"mode":"valid","owner":null}`,
		`[]`, `"not an object"`,
	} {
		t.Run(raw, func(t *testing.T) {
			params := &mcp.CallToolParams{Name: "validation", Arguments: json.RawMessage(raw)}
			control, err := sdkSession.CallTool(context.Background(), params)
			require.NoError(t, err)
			result, err := typedSession.CallTool(context.Background(), params)
			require.NoError(t, err)
			assert.Equal(t, control.IsError, result.IsError)
			assert.JSONEq(t, mustMarshalJSON(t, control.StructuredContent), mustMarshalJSON(t, result.StructuredContent))
			assert.Equal(t, control.Content, result.Content)
			if !result.IsError {
				assert.Contains(t, mustMarshalJSON(t, result.StructuredContent), `"count":`)
			}
		})
	}
	list, err := typedSession.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	assert.JSONEq(t, mustMarshalJSON(t, advertised), mustMarshalJSON(t, list.Tools[0].InputSchema))
}

func TestTypedInputValidationMatchesSDK(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"mode":  {Type: "string", Enum: []any{"valid"}},
			"count": {Type: "integer", Default: json.RawMessage(`2`)},
		},
		Required: []string{"mode"},
	}
	t.Run("struct", func(t *testing.T) { compareTypedInputWithSDK[typedValidationInput](t, schema, schema) })
	t.Run("pointer", func(t *testing.T) { compareTypedInputWithSDK[*typedValidationInput](t, schema, schema) })
	t.Run("any", func(t *testing.T) {
		compareTypedInputWithSDK[any](t, schema, schema)
	})
	t.Run("runtime-schema", func(t *testing.T) {
		runtime := CloneSchema(schema)
		runtime.Properties["mode"].Enum = nil
		runtime.Properties["count"].Default = json.RawMessage(`3`)
		compareTypedInputWithSDK[typedValidationInput](t, schema, runtime)
	})
	t.Run("header-annotations", func(t *testing.T) {
		type input struct {
			Owner string `json:"owner"`
			typedValidationInput
		}
		advertised := CloneSchema(schema)
		advertised.Properties["owner"] = &jsonschema.Schema{
			Type: "string", Enum: []any{"octo"}, Default: json.RawMessage(`"octo"`),
			Extra: map[string]any{"x-mcp-header": "owner"},
		}
		compareTypedInputWithSDK[input](t, advertised, advertised)
	})
}

func TestResolvedTypedInputSchemaCache(t *testing.T) {
	schema := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"value": {Type: "string"}}}
	first, err := cachedResolvedInputSchema(schema)
	require.NoError(t, err)
	second, err := cachedResolvedInputSchema(CloneSchema(schema))
	require.NoError(t, err)
	assert.Same(t, first, second)
	assert.Equal(t, "string", schema.Properties["value"].Type)
}
