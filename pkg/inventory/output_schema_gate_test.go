package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasObjectRootOutputSchema(t *testing.T) {
	tests := []struct {
		name   string
		schema any
		want   bool
	}{
		{"nil", nil, false},
		{"object root", json.RawMessage(`{"type":"object"}`), true},
		{
			"object root with inner anyOf is still object root",
			json.RawMessage(`{"type":"object","anyOf":[{"required":["a"]},{"required":["b"]}]}`),
			true,
		},
		{"bare anyOf", json.RawMessage(`{"anyOf":[{"type":"object"},{"type":"array"}]}`), false},
		{"array root", json.RawMessage(`{"type":"array","items":{"type":"object"}}`), false},
		{"string root", json.RawMessage(`{"type":"string"}`), false},
		{"bare $ref", json.RawMessage(`{"$ref":"#/$defs/x","$defs":{"x":{"type":"object"}}}`), false},
		// 2020-12 permits a type array; that is not the 2025-11-25 shape.
		{"type array containing object", json.RawMessage(`{"type":["object","null"]}`), false},
		{"map form", map[string]any{"type": "object"}, true},
		{"unmarshalable fails closed", make(chan int), false},
		{"non-schema JSON fails closed", json.RawMessage(`"just a string"`), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HasObjectRootOutputSchema(tt.schema))
		})
	}
}

// The SDK's listTools hands back the *same* *mcp.Tool pointers it stores in
// its registry, so a gate that mutated them in place would strip the schema
// from the server's own definition and leak that to every later session.
func TestStripNonObjectRootOutputSchemasDoesNotMutateInput(t *testing.T) {
	objectRoot := json.RawMessage(`{"type":"object"}`)
	bareAnyOf := json.RawMessage(`{"anyOf":[{"type":"array"},{"type":"object"}]}`)

	registry := []*mcp.Tool{
		{Name: "keeps", OutputSchema: objectRoot},
		{Name: "gets_stripped", OutputSchema: bareAnyOf},
		{Name: "no_schema"},
	}

	got := stripNonObjectRootOutputSchemas(registry)

	// The returned view is gated...
	require.Len(t, got, 3)
	assert.Equal(t, objectRoot, got[0].OutputSchema, "object roots survive the gate")
	assert.Nil(t, got[1].OutputSchema, "non-object root is stripped from the returned view")

	// ...but the caller's tools are untouched.
	assert.Equal(t, bareAnyOf, registry[1].OutputSchema, "registry tool must not be mutated")
	assert.NotSame(t, registry[1], got[1], "the stripped entry must be a copy")
	assert.Same(t, registry[0], got[0], "unchanged entries are not copied")
}

func TestStripNonObjectRootOutputSchemasNoOp(t *testing.T) {
	tools := []*mcp.Tool{
		{Name: "a", OutputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "b"},
	}
	got := stripNonObjectRootOutputSchemas(tools)
	// Nothing needed gating, so the original slice is returned as-is.
	assert.Equal(t, &tools, &got, "no-op should not allocate a new slice")
}

func listToolsReqAtVersion(version string) *mcp.ListToolsRequest {
	params := &mcp.ListToolsParams{}
	if version != "" {
		params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: version}
	}
	return &mcp.ListToolsRequest{Params: params}
}

func TestOutputSchemaVersionGate(t *testing.T) {
	bareAnyOf := json.RawMessage(`{"anyOf":[{"type":"array"},{"type":"object"}]}`)
	objectRoot := json.RawMessage(`{"type":"object"}`)

	newResult := func() *mcp.ListToolsResult {
		return &mcp.ListToolsResult{Tools: []*mcp.Tool{
			{Name: "object_root", OutputSchema: objectRoot},
			{Name: "bare_anyof", OutputSchema: bareAnyOf},
		}}
	}

	tests := []struct {
		name             string
		version          string
		wantAnyOfStopped bool
	}{
		{"2026-07-28 gets the non-object root", "2026-07-28", false},
		{"a later revision also gets it", "2027-01-01", false},
		{"2025-11-25 does not", "2025-11-25", true},
		{"2025-06-18 does not", "2025-06-18", true},
		{"unknown version fails closed", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var handled string
			next := func(_ context.Context, method string, _ mcp.Request) (mcp.Result, error) {
				handled = method
				return newResult(), nil
			}
			res, err := OutputSchemaVersionGate()(next)(context.Background(), "tools/list", listToolsReqAtVersion(tt.version))
			require.NoError(t, err)
			require.Equal(t, "tools/list", handled)

			list, ok := res.(*mcp.ListToolsResult)
			require.True(t, ok)
			assert.Equal(t, objectRoot, list.Tools[0].OutputSchema, "object roots are never gated")
			if tt.wantAnyOfStopped {
				assert.Nil(t, list.Tools[1].OutputSchema)
			} else {
				assert.Equal(t, bareAnyOf, list.Tools[1].OutputSchema)
			}
		})
	}
}

func TestOutputSchemaVersionGateIgnoresOtherMethods(t *testing.T) {
	called := false
	next := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		called = true
		return &mcp.CallToolResult{}, nil
	}
	// A non-tools/list method must pass straight through, including its
	// result type, which is not a ListToolsResult.
	res, err := OutputSchemaVersionGate()(next)(context.Background(), "tools/call", listToolsReqAtVersion("2025-11-25"))
	require.NoError(t, err)
	assert.True(t, called)
	_, ok := res.(*mcp.CallToolResult)
	assert.True(t, ok)
}
