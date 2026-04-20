package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/skills"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bundledSkills builds the registry of Agent Skills this server ships.
// Each entry's Enabled closure gates its publication on the relevant
// toolset being enabled under the given inventory.
//
// Adding a new server-bundled skill is one entry here plus a //go:embed
// line in package skills.
func bundledSkills(inv *inventory.Inventory) *skills.Registry {
	return skills.New().Add(skills.Bundled{
		Name:        "pull-requests",
		Description: "Submit a multi-comment GitHub pull request review using the pending-review workflow. Use when leaving line-specific feedback on a pull request, when asked to review a PR, or whenever creating any review with more than one comment.",
		Content:     skills.PullRequestsSKILL,
		Icons:       octicons.Icons("light-bulb"),
		Enabled:     func() bool { return inv.IsToolsetEnabled(ToolsetMetadataPullRequests.ID) },
	})
}

// DeclareSkillsExtensionIfEnabled adds the skills-over-MCP extension
// (SEP-2133) to the server's capabilities when any bundled skill is
// currently enabled. Must be called before mcp.NewServer.
func DeclareSkillsExtensionIfEnabled(opts *mcp.ServerOptions, inv *inventory.Inventory) {
	bundledSkills(inv).DeclareCapability(opts)
}

// RegisterBundledSkills registers all enabled server-bundled skills and
// the skill://index.json discovery document on the given server.
func RegisterBundledSkills(s *mcp.Server, inv *inventory.Inventory) {
	bundledSkills(inv).Install(s)
}
