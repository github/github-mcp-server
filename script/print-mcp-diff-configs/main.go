// Command print-mcp-diff-configs emits the configuration matrix consumed by
// the mcp-server-diff GitHub Action. The matrix is composed of three parts:
//
//  1. Hand-curated baseline configs (default, read-only, common toolset combos)
//  2. Insiders configs (--insiders, --insiders --read-only) — meta flag that
//     expands to the curated insiders feature set
//  3. One config per entry in github.AllowedFeatureFlags — automatically kept
//     in sync with the Go source so any new user-controllable feature flag
//     gets diffed without touching the workflow
//
// The same logical matrix is rendered for two transports, selected by
// -transport:
//
// stdio        Default. Args are appended to the action's top-level
//
//	start_command (one stdio process per config).
//
// http-headers streamable-http transport against a shared HTTP server. The
//
//	server is started once with no extra flags and every config
//	provides its settings via X-MCP-* request headers, mirroring
//	how the remote server is invoked in production (server-side
//	defaults + per-user header overrides).
//
// Usage:
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

// baseEntry describes one logical configuration in transport-agnostic form.
// settings are translated to either CLI flags or X-MCP-* headers depending on
// the target transport.
type baseEntry struct {
	name     string
	settings settings
}

type settings struct {
	toolsets     string // comma-separated, "" for defaults
	tools        string
	excludeTools string
	features     string
	readOnly     bool
	insiders     bool
	lockdown     bool
	// scope is a complete scoped-mode CLI fragment (e.g.
	// "--repository=octocat/hello-world"). Scoped modes are stdio-only and have
	// no X-MCP-* header equivalent yet, so entries that set it are skipped by
	// the http-headers transport.
	scope string
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
			// Scoped modes are stdio-only; there is no header transport for
			// them yet, so they are not part of the http-headers matrix.
			if e.settings.scope != "" {
				continue
			}
			h := e.settings.toHeaders()
			if h == nil {
				h = map[string]string{}
			}
			// The action's top-level headers may be replaced (not merged) by
			// per-config headers, so always include the bearer token here.
			// The token must match a recognized GitHub prefix so the server's
			// Authorization parser accepts it without contacting the API.
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
		// Combined entries: exercise multiple settings together so we catch
		// regressions when several X-MCP-* headers (or CLI flags) are merged.
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
		// Context-scoped server modes (stdio only). These bind the server to a
		// single repository, pull request, or project and expose the bespoke
		// scoped tool surface for that context. Including them here ensures any
		// change to a shared tool's schema is diffed on every scoped surface,
		// not just the full server.
		{name: "scope-repository", settings: settings{scope: "--repository=octocat/hello-world"}},
		{name: "scope-repository+read-only", settings: settings{scope: "--repository=octocat/hello-world", readOnly: true}},
		{name: "scope-pull-request", settings: settings{scope: "--pull-request=octocat/hello-world#42"}},
		{name: "scope-project", settings: settings{scope: "--project=orgs/octocat/7"}},
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
	if s.scope != "" {
		parts = append(parts, s.scope)
	}
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
