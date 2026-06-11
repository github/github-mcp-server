package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type uiResourceSpec struct {
	toolName string
	register func(s *mcp.Server)
}

// RegisterUIResources registers MCP App UI resources with the server.
// These are static resources (not templates) that serve HTML content for
// MCP App-enabled tools. The HTML is built from React/Primer components
// in the ui/ directory using `script/build-ui`.
//
// UI resources are registered only when their backing tool is present in
// the inventory's available tools (respecting read-only mode and other filters).
//
// Resource metadata follows the stable 2026-01-26 MCP Apps spec:
// https://github.com/modelcontextprotocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx
func RegisterUIResources(ctx context.Context, s *mcp.Server, inv *inventory.Inventory) {
	tools := inv.AvailableTools(ctx)
	available := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		available[tool.Tool.Name] = struct{}{}
	}

	specs := []uiResourceSpec{
		{toolName: "get_me", register: registerGetMeUIResource},
		{toolName: "issue_write", register: registerIssueWriteUIResource},
		{toolName: "create_pull_request", register: registerPullRequestWriteUIResource},
	}

	for _, spec := range specs {
		if _, ok := available[spec.toolName]; ok {
			spec.register(s)
		}
	}
}

func registerGetMeUIResource(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         GetMeUIResourceURI,
			Name:        "get_me_ui",
			Description: "MCP App UI for the get_me tool",
			MIMEType:    MCPAppMIMEType,
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("get-me.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      GetMeUIResourceURI,
						MIMEType: MCPAppMIMEType,
						Text:     html,
						Meta: mcp.Meta{
							"ui": map[string]any{
								// Allow loading images from GitHub's avatar CDN.
								"csp": map[string]any{
									"resourceDomains": []string{"https://avatars.githubusercontent.com"},
								},
								// Profile card renders inline within chat without a host border.
								"prefersBorder": false,
							},
						},
					},
				},
			}, nil
		},
	)
}

func registerIssueWriteUIResource(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         IssueWriteUIResourceURI,
			Name:        "issue_write_ui",
			Description: "MCP App UI for creating and updating GitHub issues",
			MIMEType:    MCPAppMIMEType,
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("issue-write.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      IssueWriteUIResourceURI,
						MIMEType: MCPAppMIMEType,
						Text:     html,
						Meta: mcp.Meta{
							"ui": map[string]any{
								// No external origins required; documents the secure default.
								"csp": map[string]any{},
								// Form surface benefits from a host-provided border.
								"prefersBorder": true,
							},
						},
					},
				},
			}, nil
		},
	)
}

func registerPullRequestWriteUIResource(s *mcp.Server) {
	s.AddResource(
		&mcp.Resource{
			URI:         PullRequestWriteUIResourceURI,
			Name:        "pr_write_ui",
			Description: "MCP App UI for creating GitHub pull requests",
			MIMEType:    MCPAppMIMEType,
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("pr-write.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      PullRequestWriteUIResourceURI,
						MIMEType: MCPAppMIMEType,
						Text:     html,
						Meta: mcp.Meta{
							"ui": map[string]any{
								"csp":           map[string]any{},
								"prefersBorder": true,
							},
						},
					},
				},
			}, nil
		},
	)
}
