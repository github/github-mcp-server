package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func listThroughEncoder(t *testing.T, definitions map[string]listedToolSchemas, list *mcp.ListToolsResult) *mcp.ListToolsResult {
	t.Helper()
	handler := encodedToolSchemasMiddleware(definitions)(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return list, nil
	})
	result, err := handler(context.Background(), MCPMethodToolsList, nil)
	require.NoError(t, err)
	return result.(*mcp.ListToolsResult)
}

func TestEncodedSchemasVariantIsolation(t *testing.T) {
	t.Parallel()
	for _, property := range []string{"dotcom", "enterprise", "feature_enabled"} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()
			schema := &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{property: {Type: "string"}, "owner": {Type: "string"}},
			}
			tool := &mcp.Tool{Name: "same_name", InputSchema: schema, OutputSchema: schema}
			AnnotateHeaderParams(tool)
			before := &mcp.ListToolsResult{Tools: []*mcp.Tool{tool}, NextCursor: "unchanged"}
			want, err := json.Marshal(before)
			require.NoError(t, err)
			after := listThroughEncoder(t, map[string]listedToolSchemas{
				tool.Name: {source: schema, registered: tool},
			}, before)
			got, err := json.Marshal(after)
			require.NoError(t, err)
			assert.Equal(t, string(want), string(got))
			assert.IsType(t, json.RawMessage{}, after.Tools[0].InputSchema)
			assert.IsType(t, json.RawMessage{}, after.Tools[0].OutputSchema)
			assert.NotSame(t, before, after)
			assert.NotSame(t, tool, after.Tools[0])
			assert.Same(t, schema, tool.OutputSchema)
			assert.IsType(t, &jsonschema.Schema{}, tool.InputSchema)
			assert.Contains(t, string(got), property)
			assert.NotContains(t, string(after.Tools[0].OutputSchema.(json.RawMessage)), "x-mcp-header")
			assert.Contains(t, string(after.Tools[0].InputSchema.(json.RawMessage)), "x-mcp-header")
		})
	}
}

type encodingProbe struct {
	calls atomic.Int64
	err   error
}

func (p *encodingProbe) MarshalJSON() ([]byte, error) {
	p.calls.Add(1)
	return []byte(`true`), p.err
}

func TestEncodedSchemasConcurrentRegistration(t *testing.T) {
	t.Parallel()
	probe := &encodingProbe{}
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}},
		Extra:      map[string]any{"x-probe": probe},
	}
	original, err := json.Marshal(schema)
	require.NoError(t, err)
	initialCalls := probe.calls.Load()
	const workers = 32
	results := make(chan json.RawMessage, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			tool := &mcp.Tool{Name: "concurrent", InputSchema: schema}
			AnnotateHeaderParams(tool)
			before := &mcp.ListToolsResult{Tools: []*mcp.Tool{tool}}
			handler := encodedToolSchemasMiddleware(map[string]listedToolSchemas{
				tool.Name: {source: schema, registered: tool},
			})(func(context.Context, string, mcp.Request) (mcp.Result, error) {
				return before, nil
			})
			for range 10 {
				result, err := handler(context.Background(), MCPMethodToolsList, nil)
				if !assert.NoError(t, err) {
					return
				}
				after := result.(*mcp.ListToolsResult)
				assert.Same(t, tool, before.Tools[0])
				assert.IsType(t, &jsonschema.Schema{}, tool.InputSchema)
				data := after.Tools[0].InputSchema.(json.RawMessage)
				assert.True(t, json.Valid(data))
			}
			result, err := handler(context.Background(), MCPMethodToolsList, nil)
			if assert.NoError(t, err) {
				results <- result.(*mcp.ListToolsResult).Tools[0].InputSchema.(json.RawMessage)
			}
		})
	}
	wg.Wait()
	close(results)
	assert.Equal(t, int64(1), probe.calls.Load()-initialCalls, "all registration clones share one encoding")
	var first json.RawMessage
	for data := range results {
		if first == nil {
			first = data
		}
		assert.Equal(t, first, data)
	}
	after, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.Equal(t, original, after, "source schema must remain unchanged")
}

func TestEncodedSchemasRepeatedInventoryRegistration(t *testing.T) {
	t.Parallel()
	schema := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"owner": {Type: "string"}},
	}
	definition := ServerTool{
		Tool: mcp.Tool{Name: "repeated", InputSchema: schema, OutputSchema: schema},
		HandlerFunc: func(any) mcp.ToolHandler {
			return func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{}, nil
			}
		},
	}
	original, err := json.Marshal(schema)
	require.NoError(t, err)
	var first []byte
	for range 20 {
		inv, err := NewBuilder().SetTools([]ServerTool{definition}).WithToolsets([]string{"all"}).Build()
		require.NoError(t, err)
		server := mcp.NewServer(&mcp.Implementation{Name: "repeated"}, nil)
		inv.ForMCPRequest(MCPMethodToolsList, "").RegisterTools(context.Background(), server, nil)
		st, ct := mcp.NewInMemoryTransports()
		ss, err := server.Connect(context.Background(), st, nil)
		require.NoError(t, err)
		cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(context.Background(), ct, nil)
		require.NoError(t, err)
		list, err := cs.ListTools(context.Background(), nil)
		_ = cs.Close()
		_ = ss.Close()
		require.NoError(t, err)
		data, err := json.Marshal(list)
		require.NoError(t, err)
		if first == nil {
			first = data
		}
		assert.Equal(t, first, data)
	}
	var entries int
	encodedSchemas.Range(func(key, _ any) bool {
		if key.(encodedSchemaKey).schema == schema {
			entries++
		}
		return true
	})
	assert.Equal(t, 2, entries, "new inventories and annotation clones must reuse the two source-schema keys")
	after, err := json.Marshal(schema)
	require.NoError(t, err)
	assert.Equal(t, original, after)
}

func TestEncodedSchemasPreserveReplacementsAndErrors(t *testing.T) {
	t.Parallel()
	source := &jsonschema.Schema{Type: "object"}
	registered := &mcp.Tool{Name: "tool", InputSchema: source}
	replacement := &mcp.Tool{Name: "tool", InputSchema: &jsonschema.Schema{Type: "object", Description: "replacement"}}
	raw := &mcp.Tool{Name: "raw", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)}
	before := &mcp.ListToolsResult{Tools: []*mcp.Tool{replacement, raw}}
	after := listThroughEncoder(t, map[string]listedToolSchemas{
		"tool": {source: source, registered: registered},
		"raw":  {source: raw.InputSchema, registered: raw},
	}, before)
	assert.Same(t, replacement.InputSchema, after.Tools[0].InputSchema)
	assert.Equal(t, raw.InputSchema, after.Tools[1].InputSchema)

	broken := &jsonschema.Schema{Type: "object", Extra: map[string]any{
		"x-probe": &encodingProbe{err: errors.New("encoding failed")},
	}}
	tool := &mcp.Tool{Name: "broken", InputSchema: broken}
	handler := encodedToolSchemasMiddleware(map[string]listedToolSchemas{
		"broken": {source: broken, registered: tool},
	})(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return &mcp.ListToolsResult{Tools: []*mcp.Tool{tool}}, nil
	})
	result, err := handler(context.Background(), MCPMethodToolsList, nil)
	assert.Nil(t, result)
	require.ErrorContains(t, err, `encode input schema for "broken"`)

	wantErr := errors.New("downstream error")
	handler = encodedToolSchemasMiddleware(nil)(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return nil, wantErr
	})
	_, err = handler(context.Background(), MCPMethodToolsList, nil)
	assert.ErrorIs(t, err, wantErr)
}

func TestEncodedSchemasSharedResultIsImmutable(t *testing.T) {
	t.Parallel()
	schema := &jsonschema.Schema{Type: "object", Description: "shared"}
	tool := &mcp.Tool{Name: "shared", InputSchema: schema}
	list := &mcp.ListToolsResult{Tools: []*mcp.Tool{tool}, NextCursor: "cursor"}
	handler := encodedToolSchemasMiddleware(map[string]listedToolSchemas{
		tool.Name: {source: schema, registered: tool},
	})(func(context.Context, string, mcp.Request) (mcp.Result, error) {
		return list, nil
	})
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 10 {
				result, err := handler(context.Background(), MCPMethodToolsList, nil)
				if !assert.NoError(t, err) {
					return
				}
				patched := result.(*mcp.ListToolsResult)
				assert.Equal(t, "cursor", patched.NextCursor)
				assert.Equal(t, "shared", patched.Tools[0].Name)
				assert.Equal(t, json.RawMessage(`{"type":"object","description":"shared"}`), patched.Tools[0].InputSchema)
				patched.NextCursor = "caller cursor"
				patched.Tools[0].Name = "caller tool"
				patched.Tools[0].InputSchema = json.RawMessage(`{}`)
				patched.Tools[0] = nil
			}
		})
	}
	wg.Wait()
	assert.Same(t, tool, list.Tools[0])
	assert.Same(t, schema, tool.InputSchema)
	assert.Equal(t, "shared", tool.Name)
	assert.Equal(t, "cursor", list.NextCursor)
}

func TestEncodedSchemasLeaveSDKValidationIntact(t *testing.T) {
	t.Parallel()
	type values struct {
		Count int `json:"count"`
	}
	input := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"count": {Type: "integer"}},
		Required:   []string{"count"},
	}
	output := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{"count": {Type: "integer", Minimum: new(float64)}},
		Required:   []string{"count"},
	}
	for _, optimized := range []bool{false, true} {
		t.Run(map[bool]string{false: "baseline", true: "encoded"}[optimized], func(t *testing.T) {
			var calls int
			server := mcp.NewServer(&mcp.Implementation{Name: "validation"}, &mcp.ServerOptions{SchemaCache: mcp.NewSchemaCache()})
			tool := &mcp.Tool{Name: "validate", InputSchema: input, OutputSchema: output}
			mcp.AddTool(server, tool, func(_ context.Context, _ *mcp.CallToolRequest, args values) (*mcp.CallToolResult, values, error) {
				calls++
				return nil, args, nil
			})
			if optimized {
				server.AddReceivingMiddleware(encodedToolSchemasMiddleware(map[string]listedToolSchemas{
					tool.Name: {source: input, registered: tool},
				}))
			}
			st, ct := mcp.NewInMemoryTransports()
			ss, err := server.Connect(context.Background(), st, nil)
			require.NoError(t, err)
			defer func() { _ = ss.Close() }()
			cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(context.Background(), ct, nil)
			require.NoError(t, err)
			defer func() { _ = cs.Close() }()
			_, err = cs.ListTools(context.Background(), nil)
			require.NoError(t, err)
			invalid, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Name, Arguments: map[string]any{"count": "wrong"}})
			require.NoError(t, err)
			assert.True(t, invalid.IsError)
			assert.Zero(t, calls, "input validation rejects before calling the handler")
			_, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Name, Arguments: values{Count: -1}})
			require.Error(t, err, "output validation must still reject a negative result")
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool.Name, Arguments: values{Count: 1}})
			require.NoError(t, err)
			assert.False(t, result.IsError)
			assert.Equal(t, 2, calls)
			assert.Same(t, input, tool.InputSchema)
			assert.Same(t, output, tool.OutputSchema)
		})
	}
}
