// Command print-mcp-diff-configs emits the full configuration matrix consumed
// by the mcp-server-diff GitHub Action. The matrix is composed of three parts:
//
//  1. Hand-curated baseline configs (default, read-only, common toolset combos)
//  2. Insiders configs (--insiders, --insiders --read-only) — meta flag that
//     expands to the curated insiders feature set
//  3. One config per entry in github.AllowedFeatureFlags — automatically kept
//     in sync with the Go source so any new user-controllable feature flag
//     gets diffed without touching the workflow
//
// Usage: go run ./script/print-mcp-diff-configs
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/github/github-mcp-server/pkg/github"
)

type config struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

func main() {
	configs := []config{
		{Name: "default", Args: ""},
		{Name: "read-only", Args: "--read-only"},
		{Name: "toolsets-repos", Args: "--toolsets=repos"},
		{Name: "toolsets-issues", Args: "--toolsets=issues"},
		{Name: "toolsets-context", Args: "--toolsets=context"},
		{Name: "toolsets-pull_requests", Args: "--toolsets=pull_requests"},
		{Name: "toolsets-repos,issues", Args: "--toolsets=repos,issues"},
		{Name: "toolsets-issues,context", Args: "--toolsets=issues,context"},
		{Name: "toolsets-all", Args: "--toolsets=all"},
		{Name: "tools-get_me", Args: "--tools=get_me"},
		{Name: "tools-get_me,list_issues", Args: "--tools=get_me,list_issues"},
		{Name: "toolsets-repos+read-only", Args: "--toolsets=repos --read-only"},
		{Name: "insiders", Args: "--insiders"},
		{Name: "insiders+read-only", Args: "--insiders --read-only"},
	}

	flags := append([]string(nil), github.AllowedFeatureFlags...)
	sort.Strings(flags)
	for _, f := range flags {
		configs = append(configs, config{
			Name: "feature-" + f,
			Args: "--features=" + f,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(configs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
