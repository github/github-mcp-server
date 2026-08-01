package github

import (
	"context"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpAppsExtensionKey is 能力 extension key that 客户端s use to
// advertise MCP Apps UI support.
const mcpAppsExtensionKey = "io.modelcontextprotocol/ui"

// MCPAppMIMEType is MIME type f或MCP App UI 资源.
const MCPAppMIMEType = "text/html;profile=mcp-app"

// 客户端SupportsUI reports whether MCP 客户端 that sent this 请求
// supports MCP Apps UI rendering.
// It 检查s 上下文 第一个 (set by HTTP/stateless 服务器s from stored
// 会话 能力), 然后falls back 到go-sdk Session (f或stdio).
func clientSupportsUI(ctx context.Context, req *mcp.CallToolRequest) bool {
	// Check 上下文 第一个 (works f或HTTP/stateless 服务器s)
	if supported, ok := ghcontext.HasUISupport(ctx); ok {
		return supported
	}
	// F所有back to go-sdk 会话 (works f或stdio/stateful 服务器s)
	if req != nil && req.Session != nil {
		params := req.Session.InitializeParams()
		if params != nil && params.Capabilities != nil {
			_, hasUI := params.Capabilities.Extensions[mcpAppsExtensionKey]
			return hasUI
		}
	}
	return false
}

// uiSubmitted reports whether c所有is itself 一个MCP App form submission.
// form re-invokes its 工具 with _ui_submitted=真; such 调用 must execute
// rather than re-render form.
func uiSubmitted(args map[string]any) bool {
	submitted, _ := OptionalParam[bool](args, "_ui_submitted")
	return submitted
}

// hasNonFormParams reports whether c所有carries any 参数 工具's MCP
// App form can不represent (anything outside formParams). Such 调用 must
// bypass form 和execute directly so supplied 值 aren't silently
// dropped. formParams is set of 参数 form collects 和re-sends
// on submit.
func hasNonFormParams(args map[string]any, formParams map[string]struct{}) bool {
	for key, value := range args {
		if value == nil {
			continue
		}
		if _, ok := formParams[key]; !ok {
			return true
		}
	}
	return false
}

// shouldDeferToForm 是以下内容的唯一事实来源： show/defer decision
// shared 由form-backed 写入 工具 (创建_pull_请求,
// 更新_pull_请求, 议题_写入). It reports whether 一个c所有应当是 handed
// off to its MCP App form instead of executing now: defer 仅when MCP Apps
// are 启用, form deferral has 不been 禁用, 客户端 can render UI,
// c所有is 不itself 一个form submission, 和every supplied 参数 can
// be represented 由form (formParams is 工具's form-参数
// allow列出). When it 返回 假 处理器 executes directly; host may
// still render 工具's view, which renders 结果 rather than 一个输入
// form.
func shouldDeferToForm(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any, formParams map[string]struct{}) bool {
	return deps.IsFeatureEnabled(ctx, MCPAppsFeatureFlag) &&
		!deps.IsFeatureEnabled(ctx, MCPAppsDisableFormDeferralFeatureFlag) &&
		clientSupportsUI(ctx, req) &&
		!uiSubmitted(args) &&
		!hasNonFormParams(args, formParams)
}
