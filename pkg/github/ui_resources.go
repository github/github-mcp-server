package github

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterUIResources registers MCP App UI 资源 使用服务器.
// 这些are static 资源 (不templates) that serve HTML 内容 for
// MCP App-启用 工具. HTML is built from React/Primer components
// 在ui/ directory using `script/build-ui`.
//
// Resource 元数据 follows stable 2026-01-26 MCP Apps spec:
// https://github.com/model上下文protocol/ext-apps/blob/main/specification/2026-01-26/apps.mdx
func RegisterUIResources(s *mcp.Server, readOnly bool) {
	// Register 获取_me UI 资源
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
								// Pro文件 card renders in行 within chat without 一个host border.
								"prefersBorder": false,
							},
						},
					},
				},
			}, nil
		},
	)

	if readOnly {
		return
	}

	// Register 议题_写入 UI 资源
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
								// No external origins 必需; documents secure default.
								"csp": map[string]any{},
								// Form surface benefits from 一个host-provided border.
								"prefersBorder": true,
							},
						},
					},
				},
			}, nil
		},
	)

	// Register 创建_pull_请求 UI 资源
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

	s.AddResource(
		&mcp.Resource{
			URI:         PullRequestEditUIResourceURI,
			Name:        "pr_edit_ui",
			Description: "MCP App UI for editing GitHub pull requests",
			MIMEType:    MCPAppMIMEType,
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			html := MustGetUIAsset("pr-edit.html")
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{
					{
						URI:      PullRequestEditUIResourceURI,
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
