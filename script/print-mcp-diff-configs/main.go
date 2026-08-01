// Command print-mcp-diff-configs 输出由 mcp-server-diff GitHub Action 使用的配置矩阵。
// 该矩阵由三部分组成：
//
//  1. 人工维护的基准配置（默认、只读和常用工具集组合）
//  2. Insiders 配置（--insiders、--insiders --read-only），该元标志会展开为精心维护的 insiders 功能集
//  3. github.AllowedFeatureFlags 中每个条目各有一个配置；它与 Go 源码自动保持同步，
//     因而任何新的用户可控 feature flag 都会被比较，无需修改工作流
//
// 同一逻辑矩阵会渲染为两种传输方式，由 -transport 选择：
//
// stdio        默认方式。参数会附加到 action 的顶层
//
//	start_command（每个配置一个 stdio 进程）。
//
// http-headers 针对共享 HTTP 服务器的 streamable-http 传输方式。该
//
//	服务器仅启动一次且不带额外标志；每个配置通过 X-MCP-* 请求头提供设置，
//	这与生产环境调用远程服务器的方式一致（服务器端默认值加上每用户请求头覆盖）。
//
// 用法：
//
// go run ./script/print-mcp-diff-configs
// go run ./script/print-mcp-diff-configs -transport http-headers
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/github/github-mcp-server/pkg/github"
	mcphdr "github.com/github/github-mcp-server/pkg/http/headers"
)

type config struct {
	Name      string            `json:"name"`
	Args      string            `json:"args,omitempty"`
	Transport string            `json:"transport,omitempty"`
	ServerURL string            `json:"server_url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// baseEntry 以与传输方式无关的形式描述一个逻辑配置。
// settings 会根据目标传输方式转换为 CLI 标志或 X-MCP-* 请求头。
type baseEntry struct {
	name     string
	settings settings
}

type settings struct {
	toolsets     string // 以逗号分隔，"" 表示默认值
	tools        string
	excludeTools string
	features     string
	readOnly     bool
	insiders     bool
	lockdown     bool
}

const httpServerURL = "http://localhost:8082/mcp"

func main() {
	transport := flag.String("transport", "stdio", "Transport to target: stdio or http-headers")
	flag.Parse()

	entries := baseEntries()

	var out []config
	switch *transport {
	case "stdio":
		for _, e := range entries {
			out = append(out, config{Name: e.name, Args: e.settings.toArgs()})
		}
	case "http-headers":
		for _, e := range entries {
			h := e.settings.toHeaders()
			if h == nil {
				h = map[string]string{}
			}
			// action 的顶层请求头可能被每个配置的请求头替换（而非合并），因此始终在此包含 bearer token。
			// token 必须具有已识别的 GitHub 前缀，服务器的 Authorization 解析器才能在不联系 API 的情况下接受它。
			h[mcphdr.AuthorizationHeader] = "Bearer ghp_test"
			out = append(out, config{
				Name:      e.name,
				Transport: "streamable-http",
				ServerURL: httpServerURL,
				Headers:   h,
			})
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown transport %q (want stdio or http-headers)\n", *transport)
		os.Exit(2)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func baseEntries() []baseEntry {
	entries := []baseEntry{
		{name: "default"},
		{name: "read-only", settings: settings{readOnly: true}},
		{name: "toolsets-repos", settings: settings{toolsets: "repos"}},
		{name: "toolsets-issues", settings: settings{toolsets: "issues"}},
		{name: "toolsets-context", settings: settings{toolsets: "context"}},
		{name: "toolsets-pull_requests", settings: settings{toolsets: "pull_requests"}},
		{name: "toolsets-repos,issues", settings: settings{toolsets: "repos,issues"}},
		{name: "toolsets-issues,context", settings: settings{toolsets: "issues,context"}},
		{name: "toolsets-all", settings: settings{toolsets: "all"}},
		{name: "tools-get_me", settings: settings{tools: "get_me"}},
		{name: "tools-get_me,list_issues", settings: settings{tools: "get_me,list_issues"}},
		{name: "toolsets-repos+read-only", settings: settings{toolsets: "repos", readOnly: true}},
		{name: "insiders", settings: settings{insiders: true}},
		{name: "insiders+read-only", settings: settings{insiders: true, readOnly: true}},
		// 组合条目：一并覆盖多个设置，以捕获合并多个 X-MCP-* 请求头（或 CLI 标志）时的回归。
		{name: "combined-toolsets+exclude+readonly", settings: settings{
			toolsets:     "repos,issues",
			excludeTools: "delete_file",
			readOnly:     true,
		}},
		{name: "combined-insiders+toolsets+features", settings: settings{
			insiders: true,
			toolsets: "repos",
			features: firstFeatureFlag(),
		}},
	}

	flags := append([]string(nil), github.AllowedFeatureFlags...)
	sort.Strings(flags)
	for _, f := range flags {
		entries = append(entries, baseEntry{
			name:     "feature-" + f,
			settings: settings{features: f},
		})
	}
	return entries
}

func (s settings) toArgs() string {
	var parts []string
	if s.toolsets != "" {
		parts = append(parts, "--toolsets="+s.toolsets)
	}
	if s.tools != "" {
		parts = append(parts, "--tools="+s.tools)
	}
	if s.excludeTools != "" {
		parts = append(parts, "--exclude-tools="+s.excludeTools)
	}
	if s.features != "" {
		parts = append(parts, "--features="+s.features)
	}
	if s.readOnly {
		parts = append(parts, "--read-only")
	}
	if s.insiders {
		parts = append(parts, "--insiders")
	}
	if s.lockdown {
		parts = append(parts, "--lockdown-mode")
	}
	return strings.Join(parts, " ")
}

func (s settings) toHeaders() map[string]string {
	h := map[string]string{}
	if s.toolsets != "" {
		h[mcphdr.MCPToolsetsHeader] = s.toolsets
	}
	if s.tools != "" {
		h[mcphdr.MCPToolsHeader] = s.tools
	}
	if s.excludeTools != "" {
		h[mcphdr.MCPExcludeToolsHeader] = s.excludeTools
	}
	if s.features != "" {
		h[mcphdr.MCPFeaturesHeader] = s.features
	}
	if s.readOnly {
		h[mcphdr.MCPReadOnlyHeader] = "true"
	}
	if s.insiders {
		h[mcphdr.MCPInsidersHeader] = "true"
	}
	if s.lockdown {
		h[mcphdr.MCPLockdownHeader] = "true"
	}
	if len(h) == 0 {
		return nil
	}
	return h
}

func firstFeatureFlag() string {
	flags := append([]string(nil), github.AllowedFeatureFlags...)
	if len(flags) == 0 {
		return ""
	}
	sort.Strings(flags)
	return flags[0]
}
