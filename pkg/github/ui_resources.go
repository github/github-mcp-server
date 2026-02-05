package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterUIResources registers MCP App UI resources with the server.
// These are static resources (not templates) that serve HTML content for
// MCP App-enabled tools. The HTML is built from React/Primer components
// in the ui/ directory using `script/build-ui`.
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
			html := MustGetUIAsset("get-me.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      GetMeUIResourceURI,
						MIMEType: "text/html",
						Text:     html,
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
					},
				},
			}, nil
		},
	)

	// Register the issue_write UI resource
	s.AddResource(
		&mcp.Resource{
			URI:         IssueWriteUIResourceURI,
			Name:        "issue_write_ui",
			Description: "MCP App UI for creating and updating GitHub issues",
			MIMEType:    "text/html",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("issue-write.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      IssueWriteUIResourceURI,
						MIMEType: "text/html",
						Text:     html,
					},
				},
			}, nil
		},
	)

	// Register the create_pull_request UI resource
	s.AddResource(
		&mcp.Resource{
			URI:         PullRequestWriteUIResourceURI,
			Name:        "pr_write_ui",
			Description: "MCP App UI for creating GitHub pull requests",
			MIMEType:    "text/html",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("pr-write.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      PullRequestWriteUIResourceURI,
						MIMEType: "text/html",
						Text:     html,
					},
				},
			}, nil
		},
	)
}
