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

//go:embed inbox-triage/SKILL.md
var InboxTriageSKILL string

//go:embed get-context/SKILL.md
var GetContextSKILL string

//go:embed explore-repo/SKILL.md
var ExploreRepoSKILL string

//go:embed search-code/SKILL.md
var SearchCodeSKILL string

//go:embed trace-history/SKILL.md
var TraceHistorySKILL string

//go:embed create-pr/SKILL.md
var CreatePRSKILL string

//go:embed self-review-pr/SKILL.md
var SelfReviewPRSKILL string

//go:embed address-pr-feedback/SKILL.md
var AddressPRFeedbackSKILL string

//go:embed merge-pr/SKILL.md
var MergePRSKILL string

//go:embed triage-issues/SKILL.md
var TriageIssuesSKILL string

//go:embed create-issue/SKILL.md
var CreateIssueSKILL string

//go:embed manage-sub-issues/SKILL.md
var ManageSubIssuesSKILL string

//go:embed debug-ci/SKILL.md
var DebugCISKILL string

//go:embed trigger-workflow/SKILL.md
var TriggerWorkflowSKILL string

//go:embed security-audit/SKILL.md
var SecurityAuditSKILL string

//go:embed fix-dependabot/SKILL.md
var FixDependabotSKILL string

//go:embed research-vulnerability/SKILL.md
var ResearchVulnerabilitySKILL string

//go:embed manage-project/SKILL.md
var ManageProjectSKILL string

//go:embed prepare-release/SKILL.md
var PrepareReleaseSKILL string

//go:embed manage-repo/SKILL.md
var ManageRepoSKILL string

//go:embed manage-labels/SKILL.md
var ManageLabelsSKILL string

//go:embed contribute-oss/SKILL.md
var ContributeOSSSKILL string

//go:embed browse-discussions/SKILL.md
var BrowseDiscussionsSKILL string

//go:embed delegate-to-copilot/SKILL.md
var DelegateToCopilotSKILL string

//go:embed discover-github/SKILL.md
var DiscoverGitHubSKILL string

//go:embed share-snippet/SKILL.md
var ShareSnippetSKILL string

//go:embed discover-mcp-skills/SKILL.md
var DiscoverMCPSkillsSKILL string
