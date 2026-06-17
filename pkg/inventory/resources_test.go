package inventory

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubResourceHandler(_ any) mcp.ResourceHandler {
	return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{}, nil
	}
}

func TestServerResourceTemplate_HasHandler(t *testing.T) {
	withHandler := NewServerResourceTemplate(
		testToolsetMetadata("repos"),
		mcp.ResourceTemplate{Name: "repo-content"},
		stubResourceHandler,
	)
	assert.True(t, withHandler.HasHandler())

	noHandler := ServerResourceTemplate{Template: mcp.ResourceTemplate{Name: "x"}}
	assert.False(t, noHandler.HasHandler())
}

func TestServerResourceTemplate_Handler(t *testing.T) {
	srt := NewServerResourceTemplate(
		testToolsetMetadata("repos"),
		mcp.ResourceTemplate{Name: "repo-content"},
		stubResourceHandler,
	)
	require.NotNil(t, srt.Handler(nil))

	noHandler := ServerResourceTemplate{Template: mcp.ResourceTemplate{Name: "boom"}}
	assert.PanicsWithValue(t, "HandlerFunc is nil for resource: boom", func() {
		noHandler.Handler(nil)
	})
}
