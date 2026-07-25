package inventory

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func runMirror(t *testing.T, version string, res *mcp.CallToolResult) *mcp.CallToolResult {
	t.Helper()
	params := &mcp.CallToolParamsRaw{}
	if version != "" {
		params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: version}
	}
	req := &mcp.CallToolRequest{Params: params}
	out, err := mirrorStructuredContent(func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return res, nil
	})(context.Background(), req)
	require.NoError(t, err)
	return out
}

func TestMirrorStructuredContent(t *testing.T) {
	tests := []struct {
		name    string
		version string
		result  *mcp.CallToolResult
		want    string // expected structuredContent bytes, "" for none
	}{
		{
			name: "object mirrors on any version", version: "2025-11-25",
			result: textResult(`{"number":1,"title":"x"}`), want: `{"number":1,"title":"x"}`,
		},
		{
			// Non-object structuredContent is only representable from 2026-07-28.
			name: "array is withheld from an older client", version: "2025-11-25",
			result: textResult(`[{"id":1}]`), want: "",
		},
		{
			name: "array mirrors on 2026-07-28", version: "2026-07-28",
			result: textResult(`[{"id":1}]`), want: `[{"id":1}]`,
		},
		{
			name: "empty array mirrors on 2026-07-28", version: "2026-07-28",
			result: textResult(`[]`), want: `[]`,
		},
		{
			name: "unknown version withholds a non-object", version: "",
			result: textResult(`[]`), want: "",
		},
		{
			// A raw diff or log body is not structured data.
			name: "non-JSON text is left alone", version: "2026-07-28",
			result: textResult("diff --git a/x b/x\n@@ -1 +1 @@\n-a\n+b"), want: "",
		},
		{
			name: "error results never carry structured content", version: "2026-07-28",
			result: func() *mcp.CallToolResult {
				r := textResult(`{"message":"boom"}`)
				r.IsError = true
				return r
			}(), want: "",
		},
		{
			name: "multi-part results are left alone", version: "2026-07-28",
			result: &mcp.CallToolResult{Content: []mcp.Content{
				&mcp.TextContent{Text: `{"a":1}`}, &mcp.TextContent{Text: `{"b":2}`},
			}}, want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runMirror(t, tt.version, tt.result)
			if tt.want == "" {
				assert.Nil(t, got.StructuredContent)
				return
			}
			raw, ok := got.StructuredContent.(json.RawMessage)
			require.True(t, ok, "structured content should be raw bytes, not a re-marshaled value")
			assert.JSONEq(t, tt.want, string(raw))
		})
	}
}

// The mirror must not overwrite a value a handler set deliberately.
func TestMirrorStructuredContentPreservesExplicitValue(t *testing.T) {
	res := textResult(`{"from":"text"}`)
	res.StructuredContent = map[string]any{"from": "handler"}
	got := runMirror(t, "2026-07-28", res)
	assert.Equal(t, map[string]any{"from": "handler"}, got.StructuredContent)
}

// Mirroring must be byte-exact: no re-marshaling, so large integers and number
// formatting survive untouched. A round-trip through any/float64 would not.
func TestMirrorStructuredContentIsByteExact(t *testing.T) {
	const body = `{"id":9007199254740993,"ratio":1.10,"pad":"0042"}`
	got := runMirror(t, "2026-07-28", textResult(body))
	raw, ok := got.StructuredContent.(json.RawMessage)
	require.True(t, ok)
	assert.Equal(t, body, string(raw), "structured content must be the text block's exact bytes")

	// And it survives serialization to the wire unchanged.
	encoded, err := json.Marshal(map[string]any{"structuredContent": got.StructuredContent})
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `9007199254740993`)
	assert.Contains(t, string(encoded), `1.10`)
}
