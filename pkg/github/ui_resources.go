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
				// MCP Apps UI metadata - CSP configuration to allow loading GitHub avatars
				// See: https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/draft/apps.mdx
				Meta: mcp.Meta{
					"ui": map[string]any{
						"csp": map[string]any{
							// Allow loading images from GitHub's avatar CDN
							"resourceDomains": []string{"https://avatars.githubusercontent.com"},
						},
					},
				},
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
