package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterUIResources registers MCP App UI resources with the server.
// These are static resources (not templates) that serve HTML content for
// MCP App-enabled tools.
func RegisterUIResources(s *mcp.Server) {
	// Register the get_me UI resource
	s.AddResource(
		&mcp.Resource{
			URI:         GetMeUIResourceURI,
			Name:        "get_me_ui",
			Description: "MCP App UI for the get_me tool",
			MIMEType:    "text/html",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      GetMeUIResourceURI,
						MIMEType: "text/html",
						Text:     GetMeUIHTML,
					},
				},
			}, nil
		},
	)
}
