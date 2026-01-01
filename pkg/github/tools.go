package github

import (
	"context"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// Toolset metadata constants - these define all available toolsets and their descriptions.
// Tools use these constants to declare which toolset they belong to.
// Icons are Octicon names from https://primer.style/foundations/icons
var (
	ToolsetMetadataAll = inventory.ToolsetMetadata{
		ID:          "all",
		Description: "Special toolset that enables all available toolsets",
		Icon:        "apps",
	}
	ToolsetMetadataDefault = inventory.ToolsetMetadata{
		ID:          "default",
		Description: "Special toolset that enables the default toolset configuration. When no toolsets are specified, this is the set that is enabled",
		Icon:        "check-circle",
	}
	ToolsetMetadataContext = inventory.ToolsetMetadata{
		ID:          "context",
		Description: "Tools that provide context about the current user and GitHub context you are operating in",
		Default:     true,
		Icon:        "person",
	}
	ToolsetMetadataRepos = inventory.ToolsetMetadata{
		ID:          "repos",
		Description: "GitHub Repository related tools",
		Default:     true,
		Icon:        "repo",
	}
	ToolsetMetadataGit = inventory.ToolsetMetadata{
		ID:          "git",
		Description: "GitHub Git API related tools for low-level Git operations",
		Icon:        "git-branch",
	}
	ToolsetMetadataIssues = inventory.ToolsetMetadata{
		ID:          "issues",
		Description: "GitHub Issues related tools",
		Default:     true,
		Icon:        "issue-opened",
	}
	ToolsetMetadataPullRequests = inventory.ToolsetMetadata{
		ID:          "pull_requests",
		Description: "GitHub Pull Request related tools",
		Default:     true,
		Icon:        "git-pull-request",
	}
	ToolsetMetadataUsers = inventory.ToolsetMetadata{
		ID:          "users",
		Description: "GitHub User related tools",
		Default:     true,
		Icon:        "people",
	}
	ToolsetMetadataOrgs = inventory.ToolsetMetadata{
		ID:          "orgs",
		Description: "GitHub Organization related tools",
		Icon:        "organization",
	}
	ToolsetMetadataActions = inventory.ToolsetMetadata{
		ID:          "actions",
		Description: "GitHub Actions workflows and CI/CD operations",
		Icon:        "workflow",
	}
	ToolsetMetadataCodeSecurity = inventory.ToolsetMetadata{
		ID:          "code_security",
		Description: "Code security related tools, such as GitHub Code Scanning",
		Icon:        "codescan",
	}
	ToolsetMetadataSecretProtection = inventory.ToolsetMetadata{
		ID:          "secret_protection",
		Description: "Secret protection related tools, such as GitHub Secret Scanning",
		Icon:        "shield-lock",
	}
	ToolsetMetadataDependabot = inventory.ToolsetMetadata{
		ID:          "dependabot",
		Description: "Dependabot tools",
		Icon:        "dependabot",
	}
	ToolsetMetadataNotifications = inventory.ToolsetMetadata{
		ID:          "notifications",
		Description: "GitHub Notifications related tools",
		Icon:        "bell",
	}
	ToolsetMetadataDiscussions = inventory.ToolsetMetadata{
		ID:          "discussions",
		Description: "GitHub Discussions related tools",
		Icon:        "comment-discussion",
	}
	ToolsetMetadataGists = inventory.ToolsetMetadata{
		ID:          "gists",
		Description: "GitHub Gist related tools",
		Icon:        "logo-gist",
	}
	ToolsetMetadataSecurityAdvisories = inventory.ToolsetMetadata{
		ID:          "security_advisories",
		Description: "Security advisories related tools",
		Icon:        "shield",
	}
	ToolsetMetadataProjects = inventory.ToolsetMetadata{
		ID:          "projects",
		Description: "GitHub Projects related tools",
		Icon:        "project",
	}
	ToolsetMetadataStargazers = inventory.ToolsetMetadata{
		ID:          "stargazers",
		Description: "GitHub Stargazers related tools",
		Icon:        "star",
	}
	ToolsetMetadataDynamic = inventory.ToolsetMetadata{
		ID:          "dynamic",
		Description: "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.",
		Icon:        "tools",
	}
	ToolsetLabels = inventory.ToolsetMetadata{
		ID:          "labels",
		Description: "GitHub Labels related tools",
		Icon:        "tag",
	}

	// Remote-only toolsets - these are only available in the remote MCP server
	// but are documented here for consistency and to enable automated documentation.
	ToolsetMetadataCopilot = inventory.ToolsetMetadata{
		ID:          "copilot",
		Description: "Copilot related tools",
		Icon:        "copilot",
	}
	ToolsetMetadataCopilotSpaces = inventory.ToolsetMetadata{
		ID:          "copilot_spaces",
		Description: "Copilot Spaces tools",
		Icon:        "copilot",
	}
	ToolsetMetadataSupportSearch = inventory.ToolsetMetadata{
		ID:          "github_support_docs_search",
		Description: "Retrieve documentation to answer GitHub product and support questions. Topics include: GitHub Actions Workflows, Authentication, ...",
		Icon:        "book",
	}
)

// AllTools returns all tools with their embedded toolset metadata.
// Tool functions return ServerTool directly with toolset info.
func AllTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return []inventory.ServerTool{
		// Context tools
		GetMe(t),
		GetTeams(t),
		GetTeamMembers(t),

		// Repository tools
		SearchRepositories(t),
		GetFileContents(t),
		ListCommits(t),
		SearchCode(t),
		GetCommit(t),
		ListBranches(t),
		ListTags(t),
		GetTag(t),
		ListReleases(t),
		GetLatestRelease(t),
		GetReleaseByTag(t),
		CreateOrUpdateFile(t),
		CreateRepository(t),
		ForkRepository(t),
		CreateBranch(t),
		PushFiles(t),
		DeleteFile(t),
		ListStarredRepositories(t),
		StarRepository(t),
		UnstarRepository(t),

		// Git tools
		GetRepositoryTree(t),

		// Issue tools
		IssueRead(t),
		SearchIssues(t),
		ListIssues(t),
		ListIssueTypes(t),
		IssueWrite(t),
		AddIssueComment(t),
		AssignCopilotToIssue(t),
		SubIssueWrite(t),

		// User tools
		SearchUsers(t),

		// Organization tools
		SearchOrgs(t),

		// Pull request tools
		PullRequestRead(t),
		ListPullRequests(t),
		SearchPullRequests(t),
		MergePullRequest(t),
		UpdatePullRequestBranch(t),
		CreatePullRequest(t),
		UpdatePullRequest(t),
		RequestCopilotReview(t),
		PullRequestReviewWrite(t),
		AddCommentToPendingReview(t),

		// Code security tools
		GetCodeScanningAlert(t),
		ListCodeScanningAlerts(t),

		// Secret protection tools
		GetSecretScanningAlert(t),
		ListSecretScanningAlerts(t),

		// Dependabot tools
		GetDependabotAlert(t),
		ListDependabotAlerts(t),

		// Notification tools
		ListNotifications(t),
		GetNotificationDetails(t),
		DismissNotification(t),
		MarkAllNotificationsRead(t),
		ManageNotificationSubscription(t),
		ManageRepositoryNotificationSubscription(t),

		// Discussion tools
		ListDiscussions(t),
		GetDiscussion(t),
		GetDiscussionComments(t),
		ListDiscussionCategories(t),

		// Actions tools
		ListWorkflows(t),
		ListWorkflowRuns(t),
		GetWorkflowRun(t),
		GetWorkflowRunLogs(t),
		ListWorkflowJobs(t),
		GetJobLogs(t),
		ListWorkflowRunArtifacts(t),
		DownloadWorkflowRunArtifact(t),
		GetWorkflowRunUsage(t),
		RunWorkflow(t),
		RerunWorkflowRun(t),
		RerunFailedJobs(t),
		CancelWorkflowRun(t),
		DeleteWorkflowRunLogs(t),
		// Consolidated Actions tools (enabled via feature flag)
		ActionsList(t),
		ActionsGet(t),
		ActionsRunTrigger(t),
		ActionsGetJobLogs(t),

		// Security advisories tools
		ListGlobalSecurityAdvisories(t),
		GetGlobalSecurityAdvisory(t),
		ListRepositorySecurityAdvisories(t),
		ListOrgRepositorySecurityAdvisories(t),

		// Gist tools
		ListGists(t),
		GetGist(t),
		CreateGist(t),
		UpdateGist(t),

		// Project tools
		ListProjects(t),
		GetProject(t),
		ListProjectFields(t),
		GetProjectField(t),
		ListProjectItems(t),
		GetProjectItem(t),
		AddProjectItem(t),
		DeleteProjectItem(t),
		UpdateProjectItem(t),

		// Label tools
		GetLabel(t),
		GetLabelForLabelsToolset(t),
		ListLabels(t),
		LabelWrite(t),
	}
}

// ToBoolPtr converts a bool to a *bool pointer.
func ToBoolPtr(b bool) *bool {
	return &b
}

// ToStringPtr converts a string to a *string pointer.
// Returns nil if the string is empty.
func ToStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GenerateToolsetsHelp generates the help text for the toolsets flag
func GenerateToolsetsHelp() string {
	// Get toolset group to derive defaults and available toolsets
	r := NewInventory(stubTranslator).Build()

	// Format default tools from metadata using strings.Builder
	var defaultBuf strings.Builder
	defaultIDs := r.DefaultToolsetIDs()
	for i, id := range defaultIDs {
		if i > 0 {
			defaultBuf.WriteString(", ")
		}
		defaultBuf.WriteString(string(id))
	}

	// Get all available toolsets (excludes context and dynamic for display)
	allToolsets := r.AvailableToolsets("context", "dynamic")
	var availableBuf strings.Builder
	const maxLineLength = 70
	currentLine := ""

	for i, toolset := range allToolsets {
		id := string(toolset.ID)
		switch {
		case i == 0:
			currentLine = id
		case len(currentLine)+len(id)+2 <= maxLineLength:
			currentLine += ", " + id
		default:
			if availableBuf.Len() > 0 {
				availableBuf.WriteString(",\n\t     ")
			}
			availableBuf.WriteString(currentLine)
			currentLine = id
		}
	}
	if currentLine != "" {
		if availableBuf.Len() > 0 {
			availableBuf.WriteString(",\n\t     ")
		}
		availableBuf.WriteString(currentLine)
	}

	// Build the complete help text using strings.Builder
	var buf strings.Builder
	buf.WriteString("Comma-separated list of tool groups to enable (no spaces).\n")
	buf.WriteString("Available: ")
	buf.WriteString(availableBuf.String())
	buf.WriteString("\n")
	buf.WriteString("Special toolset keywords:\n")
	buf.WriteString("  - all: Enables all available toolsets\n")
	buf.WriteString("  - default: Enables the default toolset configuration of:\n\t     ")
	buf.WriteString(defaultBuf.String())
	buf.WriteString("\n")
	buf.WriteString("Examples:\n")
	buf.WriteString("  - --toolsets=actions,gists,notifications\n")
	buf.WriteString("  - Default + additional: --toolsets=default,actions,gists\n")
	buf.WriteString("  - All tools: --toolsets=all")

	return buf.String()
}

// stubTranslator is a passthrough translator for cases where we need an Inventory
// but don't need actual translations (e.g., getting toolset IDs for CLI help).
func stubTranslator(_, fallback string) string { return fallback }

// AddDefaultToolset removes the default toolset and expands it to the actual default toolset IDs
func AddDefaultToolset(result []string) []string {
	hasDefault := false
	seen := make(map[string]bool)
	for _, toolset := range result {
		seen[toolset] = true
		if toolset == string(ToolsetMetadataDefault.ID) {
			hasDefault = true
		}
	}

	// Only expand if "default" keyword was found
	if !hasDefault {
		return result
	}

	result = RemoveToolset(result, string(ToolsetMetadataDefault.ID))

	// Get default toolset IDs from the Inventory
	r := NewInventory(stubTranslator).Build()
	for _, id := range r.DefaultToolsetIDs() {
		if !seen[string(id)] {
			result = append(result, string(id))
		}
	}
	return result
}

func RemoveToolset(tools []string, toRemove string) []string {
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		if tool != toRemove {
			result = append(result, tool)
		}
	}
	return result
}

func ContainsToolset(tools []string, toCheck string) bool {
	for _, tool := range tools {
		if tool == toCheck {
			return true
		}
	}
	return false
}

// CleanTools cleans tool names by removing duplicates and trimming whitespace.
// Validation of tool existence is done during registration.
func CleanTools(toolNames []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(toolNames))

	// Remove duplicates and trim whitespace
	for _, tool := range toolNames {
		trimmed := strings.TrimSpace(tool)
		if trimmed == "" {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}

	return result
}

// GetDefaultToolsetIDs returns the IDs of toolsets marked as Default.
// This is a convenience function that builds an inventory to determine defaults.
func GetDefaultToolsetIDs() []string {
	r := NewInventory(stubTranslator).Build()
	ids := r.DefaultToolsetIDs()
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(id)
	}
	return result
}

// RemoteOnlyToolsets returns toolset metadata for toolsets that are only
// available in the remote MCP server. These are documented but not registered
// in the local server.
func RemoteOnlyToolsets() []inventory.ToolsetMetadata {
	return []inventory.ToolsetMetadata{
		ToolsetMetadataCopilot,
		ToolsetMetadataCopilotSpaces,
		ToolsetMetadataSupportSearch,
	}
}
