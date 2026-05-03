package github

import (
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/skills"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// bundledSkills builds the registry of Agent Skills this server ships.
//
// All bundled skills load uniformly: always-on, no per-skill toolset
// gating, no icons. Their `allowed-tools` frontmatter is advisory only.
// The Registry's `Enabled` closure is still available for future use
// (e.g. feature-flagging a skill behind an experimental toolset).
//
// Adding a new server-bundled skill is one entry here plus a //go:embed
// line in package skills.
func bundledSkills(_ *inventory.Inventory) *skills.Registry {
	return skills.New().
		Add(skills.Bundled{
			Name:        "review-pr",
			Description: "Submit a multi-comment GitHub pull request review using the pending-review workflow (pull_request_review_write → add_comment_to_pending_review → submit_pending). Use when leaving line-specific feedback on a pull request, when asked to review a PR, or whenever creating any review with more than one comment.",
			Content:     skills.ReviewPRSKILL,
		}).
		Add(skills.Bundled{
			Name:        "handle-notifications",
			Description: "Systematically triage the current user's GitHub notifications inbox — enumerate unread items, prioritize by notification reason (review requests, mentions, assignments, security alerts), act on the high-priority ones, then dismiss the rest. Use when the user asks \"what should I work on?\", \"catch me up on GitHub\", \"triage my inbox\", \"what needs my attention?\", or otherwise wants to clear their notifications backlog.",
			Content:     skills.HandleNotificationsSKILL,
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
