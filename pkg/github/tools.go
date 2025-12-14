package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// Toolset metadata constants - these define all available toolsets and their descriptions.
// Tools use these constants to declare which toolset they belong to.
var (
	ToolsetMetadataAll = toolsets.ToolsetMetadata{
		ID:          "all",
		Description: "Special toolset that enables all available toolsets",
	}
	ToolsetMetadataDefault = toolsets.ToolsetMetadata{
		ID:          "default",
		Description: "Special toolset that enables the default toolset configuration. When no toolsets are specified, this is the set that is enabled",
	}
	ToolsetMetadataContext = toolsets.ToolsetMetadata{
		ID:          "context",
		Description: "Tools that provide context about the current user and GitHub context you are operating in",
		Default:     true,
	}
	ToolsetMetadataRepos = toolsets.ToolsetMetadata{
		ID:          "repos",
		Description: "GitHub Repository related tools",
		Default:     true,
	}
	ToolsetMetadataGit = toolsets.ToolsetMetadata{
		ID:          "git",
		Description: "GitHub Git API related tools for low-level Git operations",
	}
	ToolsetMetadataIssues = toolsets.ToolsetMetadata{
		ID:          "issues",
		Description: "GitHub Issues related tools",
		Default:     true,
	}
	ToolsetMetadataPullRequests = toolsets.ToolsetMetadata{
		ID:          "pull_requests",
		Description: "GitHub Pull Request related tools",
		Default:     true,
	}
	ToolsetMetadataUsers = toolsets.ToolsetMetadata{
		ID:          "users",
		Description: "GitHub User related tools",
		Default:     true,
	}
	ToolsetMetadataOrgs = toolsets.ToolsetMetadata{
		ID:          "orgs",
		Description: "GitHub Organization related tools",
	}
	ToolsetMetadataActions = toolsets.ToolsetMetadata{
		ID:          "actions",
		Description: "GitHub Actions workflows and CI/CD operations",
	}
	ToolsetMetadataCodeSecurity = toolsets.ToolsetMetadata{
		ID:          "code_security",
		Description: "Code security related tools, such as GitHub Code Scanning",
	}
	ToolsetMetadataSecretProtection = toolsets.ToolsetMetadata{
		ID:          "secret_protection",
		Description: "Secret protection related tools, such as GitHub Secret Scanning",
	}
	ToolsetMetadataDependabot = toolsets.ToolsetMetadata{
		ID:          "dependabot",
		Description: "Dependabot tools",
	}
	ToolsetMetadataNotifications = toolsets.ToolsetMetadata{
		ID:          "notifications",
		Description: "GitHub Notifications related tools",
	}
	ToolsetMetadataExperiments = toolsets.ToolsetMetadata{
		ID:          "experiments",
		Description: "Experimental features that are not considered stable yet",
	}
	ToolsetMetadataDiscussions = toolsets.ToolsetMetadata{
		ID:          "discussions",
		Description: "GitHub Discussions related tools",
	}
	ToolsetMetadataGists = toolsets.ToolsetMetadata{
		ID:          "gists",
		Description: "GitHub Gist related tools",
	}
	ToolsetMetadataSecurityAdvisories = toolsets.ToolsetMetadata{
		ID:          "security_advisories",
		Description: "Security advisories related tools",
	}
	ToolsetMetadataProjects = toolsets.ToolsetMetadata{
		ID:          "projects",
		Description: "GitHub Projects related tools",
	}
	ToolsetMetadataStargazers = toolsets.ToolsetMetadata{
		ID:          "stargazers",
		Description: "GitHub Stargazers related tools",
	}
	ToolsetMetadataDynamic = toolsets.ToolsetMetadata{
		ID:          "dynamic",
		Description: "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.",
	}
	ToolsetLabels = toolsets.ToolsetMetadata{
		ID:          "labels",
		Description: "GitHub Labels related tools",
	}
)

// AllTools returns all tools with their embedded toolset metadata.
// Tool functions return ServerTool directly with toolset info.
func AllTools(t translations.TranslationHelperFunc) []toolsets.ServerTool {
	return []toolsets.ServerTool{
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
	r := NewRegistry(stubTranslator)

	// Format default tools from metadata
	defaultIDs := r.DefaultToolsetIDs()
	defaultStrings := make([]string, len(defaultIDs))
	for i, id := range defaultIDs {
		defaultStrings[i] = string(id)
	}
	defaultTools := strings.Join(defaultStrings, ", ")

	// Get all available toolsets (excludes context and dynamic for display)
	allToolsets := r.AvailableToolsets("context", "dynamic")
	var availableToolsLines []string
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
			availableToolsLines = append(availableToolsLines, currentLine)
			currentLine = id
		}
	}
	if currentLine != "" {
		availableToolsLines = append(availableToolsLines, currentLine)
	}

	availableTools := strings.Join(availableToolsLines, ",\n\t     ")

	toolsetsHelp := fmt.Sprintf("Comma-separated list of tool groups to enable (no spaces).\n"+
		"Available: %s\n", availableTools) +
		"Special toolset keywords:\n" +
		"  - all: Enables all available toolsets\n" +
		fmt.Sprintf("  - default: Enables the default toolset configuration of:\n\t     %s\n", defaultTools) +
		"Examples:\n" +
		"  - --toolsets=actions,gists,notifications\n" +
		"  - Default + additional: --toolsets=default,actions,gists\n" +
		"  - All tools: --toolsets=all"

	return toolsetsHelp
}

// stubTranslator is a passthrough translator for cases where we need a Registry
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

	// Get default toolset IDs from the Registry
	r := NewRegistry(stubTranslator)
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
