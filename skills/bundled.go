// Package skills exposes the server-bundled Agent Skills shipped with this
// binary. The skill files themselves live as ordinary SKILL.md files under
// this directory — they are readable by any agent-skills consumer that
// scans repositories for skills (e.g. Claude Code, the agent-skills CLI),
// and are embedded into the server binary via //go:embed for delivery
// over MCP as skill:// resources.
//
// Keeping the skill content at this top-level location makes the files
// the primary, reusable artifact; the MCP server is one of several
// possible consumers.
package skills

import _ "embed"

//go:embed pull-requests/SKILL.md
var PullRequestsSKILL string
