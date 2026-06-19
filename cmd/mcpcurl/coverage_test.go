package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// sampleTool returns a Tool exercising every property type handled by
// addCommandFromTool / buildArgumentsMap.
func sampleTool() *Tool {
	return &Tool{
		Name:        "sample_tool",
		Description: "A sample tool",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"name":   {Type: "string", Description: "the name"},
				"status": {Type: "string", Description: "state", Enum: []string{"open", "closed"}},
				"count":  {Type: "integer", Description: "how many"},
				"ratio":  {Type: "number", Description: "a ratio"},
				"active": {Type: "boolean", Description: "is active"},
				"labels": {Type: "array", Description: "labels", Items: &PropertyItem{Type: "string"}},
				"items":  {Type: "array", Description: "items", Items: &PropertyItem{Type: "object"}},
			},
			Required: []string{"name"},
		},
	}
}

// newToolCommand builds the cobra subcommand for a tool via addCommandFromTool
// and returns it, so tests can set flags and call the pure helpers.
func newToolCommand(t *testing.T, tool *Tool) *cobra.Command {
	t.Helper()
	parent := &cobra.Command{Use: "tools"}
	addCommandFromTool(parent, tool, false)
	cmds := parent.Commands()
	require.Len(t, cmds, 1)
	return cmds[0]
}

func TestAddCommandFromTool_RegistersFlags(t *testing.T) {
	cmd := newToolCommand(t, sampleTool())

	require.Equal(t, "sample_tool", cmd.Use)
	require.Equal(t, "A sample tool", cmd.Short)

	// A flag is registered for each scalar/array property.
	for _, name := range []string{"name", "status", "count", "ratio", "active", "labels"} {
		require.NotNil(t, cmd.Flags().Lookup(name), "expected flag %q", name)
	}
	// Object-array properties get a "<name>-json" flag.
	require.NotNil(t, cmd.Flags().Lookup("items-json"))

	// Optional flags advertise themselves as optional in the usage text.
	require.Contains(t, cmd.Flags().Lookup("status").Usage, "(optional)")
	require.NotContains(t, cmd.Flags().Lookup("name").Usage, "(optional)")
}

func TestAddCommandFromTool_EnumValidation(t *testing.T) {
	cmd := newToolCommand(t, sampleTool())
	require.NotNil(t, cmd.PreRunE, "enum property should install a PreRunE validator")

	require.NoError(t, cmd.Flags().Set("status", "open"))
	require.NoError(t, cmd.PreRunE(cmd, nil))

	require.NoError(t, cmd.Flags().Set("status", "bogus"))
	err := cmd.PreRunE(cmd, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "status must be one of")
}

func TestBuildArgumentsMap_AllTypes(t *testing.T) {
	tool := sampleTool()
	cmd := newToolCommand(t, tool)

	require.NoError(t, cmd.Flags().Set("name", "hello"))
	require.NoError(t, cmd.Flags().Set("count", "5"))
	require.NoError(t, cmd.Flags().Set("ratio", "1.5"))
	require.NoError(t, cmd.Flags().Set("active", "true"))
	require.NoError(t, cmd.Flags().Set("labels", "a,b"))
	require.NoError(t, cmd.Flags().Set("items-json", `[{"x":1}]`))

	args, err := buildArgumentsMap(cmd, tool)
	require.NoError(t, err)

	require.Equal(t, "hello", args["name"])
	require.Equal(t, int64(5), args["count"])
	require.InEpsilon(t, 1.5, args["ratio"], 1e-9)
	require.Equal(t, true, args["active"])
	require.Equal(t, []string{"a", "b"}, args["labels"])
	require.Equal(t, []any{map[string]any{"x": float64(1)}}, args["items"])

	// Unset, zero-valued, and empty fields are omitted.
	_, ok := args["status"]
	require.False(t, ok, "unset string should be omitted")
}

func TestBuildArgumentsMap_OmitsZeroAndUnset(t *testing.T) {
	tool := sampleTool()
	cmd := newToolCommand(t, tool)

	// Only set the boolean to false explicitly; it must still be included
	// because Changed() is true, while zero-valued numbers stay omitted.
	require.NoError(t, cmd.Flags().Set("active", "false"))

	args, err := buildArgumentsMap(cmd, tool)
	require.NoError(t, err)

	require.Equal(t, false, args["active"])
	_, hasCount := args["count"]
	require.False(t, hasCount, "zero integer should be omitted")
	_, hasName := args["name"]
	require.False(t, hasName, "unset string should be omitted")
}

func TestBuildArgumentsMap_InvalidObjectJSON(t *testing.T) {
	tool := sampleTool()
	cmd := newToolCommand(t, tool)

	require.NoError(t, cmd.Flags().Set("items-json", "{not valid json"))

	_, err := buildArgumentsMap(cmd, tool)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error parsing JSON for items")
}

func TestBuildJSONRPCRequest(t *testing.T) {
	out, err := buildJSONRPCRequest("tools/call", "my_tool", map[string]any{"a": "b"})
	require.NoError(t, err)

	var req JSONRPCRequest
	require.NoError(t, json.Unmarshal([]byte(out), &req))
	require.Equal(t, "2.0", req.JSONRPC)
	require.Equal(t, "tools/call", req.Method)
	require.Equal(t, "my_tool", req.Params.Name)
	require.Equal(t, "b", req.Params.Arguments["a"])
}

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

func TestPrintResponse_NoPrettyPrint(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"result":{}}`
	out := captureStdout(t, func() {
		require.NoError(t, printResponse(raw, false))
	})
	require.Equal(t, raw, strings.TrimSpace(out))
}

func TestPrintResponse_PrettyObjectContent(t *testing.T) {
	// content.text holds a JSON object string that should be re-indented.
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"hello\":\"world\"}"}]}}`
	out := captureStdout(t, func() {
		require.NoError(t, printResponse(resp, true))
	})
	require.Contains(t, out, "\"hello\": \"world\"")
}

func TestPrintResponse_PrettyArrayContent(t *testing.T) {
	// content.text holds a JSON array string -> JSONL fallback path.
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"[{\"n\":1}]"}]}}`
	out := captureStdout(t, func() {
		require.NoError(t, printResponse(resp, true))
	})
	require.Contains(t, out, "\"n\": 1")
}

func TestPrintResponse_PrettyEmptyContent(t *testing.T) {
	resp := `{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`
	out := captureStdout(t, func() {
		require.NoError(t, printResponse(resp, true))
	})
	require.Contains(t, out, resp)
}

func TestPrintResponse_PrettyInvalidJSON(t *testing.T) {
	err := printResponse("not json", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse JSON")
}
