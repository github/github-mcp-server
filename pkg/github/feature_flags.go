package github

import "slices"

// MCPAppsFeatureFlag 是以下功能标志名称： MCP Apps (interactive UI forms).
const MCPAppsFeatureFlag = "remote_mcp_ui_apps"

// MCPAppsDisableFormDeferralFeatureFlag 禁用s handing 写入-工具 调用 off
// to MCP App forms 当preserving MCP Apps UI 元数据 和结果 views.
const MCPAppsDisableFormDeferralFeatureFlag = "mcp_apps_disable_form_deferral"

// FeatureFlagCSVOutput 是以下功能标志名称： CSV 输出 on 列出 工具.
const FeatureFlagCSVOutput = "csv_output"

// FeatureFlagIFCLabels 是以下功能标志名称： IFC security labels in 工具 结果.
const FeatureFlagIFCLabels = "ifc_labels"

// FeatureFlagFileBlame 是以下功能标志名称： 获取_文件_blame 工具,
// which exposes git blame 信息 f或一个文件. It is gated so extra 工具
// is 不advertised by default, keeping 工具 surface sm所有unless opted in.
const FeatureFlagFileBlame = "file_blame"

// FeatureFlagIssueDependencies 是以下功能标志名称： 议题 dependency
// 工具 (议题_dependency_读取 / 议题_dependency_写入), which 读取 和edit an
// 议题's blocked-by / blocking relationships. It is gated so these 工具 are not
// advertised 在默认surface, keeping fixed 工具-schema cost small
// unless explicitly opted in.
const FeatureFlagIssueDependencies = "issue_dependencies"

// AllowedFeatureFlags 是可启用功能标志的允许列表
// by users via --features CLI flag 或X-MCP-Features HTTP header.
// 仅flags in this 列出 are accepted; 未知 flags are silently ignored.
// 此是以下内容的唯一事实来源： which flags are user-controllable.
var AllowedFeatureFlags = []string{
	MCPAppsFeatureFlag,
	MCPAppsDisableFormDeferralFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagIFCLabels,
	FeatureFlagIssuesGranular,
	FeatureFlagPullRequestsGranular,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
}

// InsidersFeatureFlags 是以下功能标志列表： insiders mode 启用s.
// When insiders mode is active, 所有flags in this 列出 are treated as 启用.
// 此是以下内容的唯一事实来源： what "insiders" means in terms of
// 功能标志 expansion.
var InsidersFeatureFlags = []string{
	MCPAppsFeatureFlag,
	FeatureFlagCSVOutput,
	FeatureFlagFileBlame,
	FeatureFlagIssueDependencies,
}

// FeatureFlags defines runtime feature toggles that adjust 工具 behavior.
type FeatureFlags struct {
	LockdownMode bool
}

// ResolveFeatureFlags 计算有效启用的 功能标志 by:
//  1. Taking user-supplied flags (from --features 或X-MCP-Features) and
//     keeping 仅those present in AllowedFeatureFlags. Unknown 或unsafe
//     flags from 请求 输入 are silently dropped here.
//  2. If insiders mode is on, unioning in every flag from InsidersFeatureFlags.
//     Insiders is 一个服务器-controlled meta switch, so its expansion is NOT
//     re-验证d against AllowedFeatureFlags.
//
// AllowedFeatureFlags 和InsidersFeatureFlags are independent sets:
//   - 一个flag in AllowedFeatureFlags 但不InsidersFeatureFlags is 一个regular
//     opt-in flag that insiders mode does 不turn on automati调用y.
//   - 一个flag in InsidersFeatureFlags 但不AllowedFeatureFlags is reachable
//     仅through insiders mode 和can不be 启用 by user 输入.
//
// Returns 一个set (map) f或O(1) lookup 由feature 检查er.
func ResolveFeatureFlags(enabledFeatures []string, insidersMode bool) map[string]bool {
	effective := make(map[string]bool)
	for _, f := range enabledFeatures {
		if slices.Contains(AllowedFeatureFlags, f) {
			effective[f] = true
		}
	}
	if insidersMode {
		for _, f := range InsidersFeatureFlags {
			effective[f] = true
		}
	}
	return effective
}
