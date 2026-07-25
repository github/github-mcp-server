package inventory

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// toolReturning builds a ServerTool whose handler always emits body as its
// single text block.
func toolReturning(name, body string) ServerTool {
	return NewServerTool(
		mcp.Tool{Name: name, InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
		testToolsetMetadata("toolset1"),
		func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: body}}}, nil
		},
	)
}

// connectToInventory registers the inventory on a server and returns a
// connected client session.
func connectToInventory(t *testing.T, inv *Inventory) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	srv := mcp.NewServer(&mcp.Implementation{Name: "test-server"}, nil)
	inv.RegisterTools(ctx, srv, nil)

	st, ct := mcp.NewInMemoryTransports()
	type result struct {
		session *mcp.ClientSession
		err     error
	}
	ch := make(chan result, 1)
	go func() {
		s, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(ctx, ct, nil)
		ch <- result{s, err}
	}()
	serverSession, err := srv.Connect(ctx, st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })

	got := <-ch
	require.NoError(t, got.err)
	t.Cleanup(func() { _ = got.session.Close() })
	return got.session
}

func featureCheckerFor(flags ...string) FeatureFlagChecker {
	return func(_ context.Context, flag string) (bool, error) {
		return slices.Contains(flags, flag), nil
	}
}

// The whole feature is opt-in: with the flag off, the advertised tool surface
// and the call result must be exactly what they were before output schemas
// existed.
func TestOutputSchemaRegistrationIsFeatureGated(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"id":{"type":"integer"}}}`)

	tests := []struct {
		name           string
		checker        FeatureFlagChecker
		wantSchema     bool
		wantStructured bool
	}{
		{"flag off", nil, false, false},
		{"unrelated flag on", featureCheckerFor("something_else"), false, false},
		{"flag on", featureCheckerFor(outputSchemasFeatureFlag), true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := toolReturning("t", `{"id":1}`).WithOutputSchema(schema)
			inv := mustBuild(t, NewBuilder().SetTools([]ServerTool{tool}).
				WithToolsets([]string{"all"}).WithFeatureChecker(tt.checker))
			session := connectToInventory(t, inv)
			ctx := context.Background()

			listed, err := session.ListTools(ctx, nil)
			require.NoError(t, err)
			require.Len(t, listed.Tools, 1)
			if tt.wantSchema {
				require.NotNil(t, listed.Tools[0].OutputSchema)
			} else {
				require.Nil(t, listed.Tools[0].OutputSchema)
			}

			called, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "t"})
			require.NoError(t, err)
			if tt.wantStructured {
				require.NotNil(t, called.StructuredContent, "a schema'd tool should mirror its text into structuredContent")
			} else {
				require.Nil(t, called.StructuredContent)
			}

			// The text block is identical either way — this feature never
			// changes what an existing client reads.
			require.Len(t, called.Content, 1)
			text, ok := called.Content[0].(*mcp.TextContent)
			require.True(t, ok)
			assert.JSONEq(t, `{"id":1}`, text.Text)
		})
	}
}

// A tool that declares no schema must be completely untouched even with the
// feature on — that is what keeps the other ~120 tools byte-identical.
func TestToolsWithoutSchemaAreUnaffected(t *testing.T) {
	inv := mustBuild(t, NewBuilder().
		SetTools([]ServerTool{toolReturning("plain", `{"id":1}`)}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor(outputSchemasFeatureFlag)))
	session := connectToInventory(t, inv)
	ctx := context.Background()

	listed, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	assert.Nil(t, listed.Tools[0].OutputSchema)

	called, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "plain"})
	require.NoError(t, err)
	assert.Nil(t, called.StructuredContent)
}

// Registration must never write the schema back onto the shared ServerTool, or
// a second registration with the feature off would still carry it.
func TestRegistrationDoesNotMutateTheInventory(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	tool := toolReturning("t", `{}`).WithOutputSchema(schema)
	inv := mustBuild(t, NewBuilder().SetTools([]ServerTool{tool}).
		WithToolsets([]string{"all"}).
		WithFeatureChecker(featureCheckerFor(outputSchemasFeatureFlag)))

	connectToInventory(t, inv)
	assert.Nil(t, inv.AllTools()[0].Tool.OutputSchema,
		"the schema belongs on ServerTool.OutputSchema until registration copies it onto a duplicate")
	assert.Equal(t, schema, inv.AllTools()[0].OutputSchema)
}
