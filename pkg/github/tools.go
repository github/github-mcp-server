package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// ToolsetMetadata is an alias to toolsets.ToolsetMetadata for convenience
type ToolsetMetadata = toolsets.ToolsetMetadata

// NewToolMeta creates tool metadata with the given toolset (required) and scopes.
// Returns mcp.Meta (map[string]any) for direct use in mcp.Tool.Meta.
func NewToolMeta(toolset ToolsetMetadata, requiredScopes ...scopes.Scope) mcp.Meta {
	if toolset.ID == "" {
		panic("toolset ID is required for ToolMeta")
	}
	meta := mcp.Meta{"toolset": toolset.ID}

	// Filter out NoScope and collect required scope strings
	var scopeStrs []string
	for _, s := range requiredScopes {
		if s != scopes.NoScope {
			scopeStrs = append(scopeStrs, s.String())
		}
	}
	if len(scopeStrs) > 0 {
		meta[scopes.MetaKey] = scopeStrs
	}

	return meta
}

// NewResourceMeta creates resource template metadata with the given toolset.
// Returns mcp.Meta (map[string]any) for direct use in mcp.ResourceTemplate.Meta.
func NewResourceMeta(toolset ToolsetMetadata) mcp.Meta {
	if toolset.ID == "" {
		panic("toolset ID is required for ResourceMeta")
	}
	return mcp.Meta{"toolset": toolset.ID}
}

// NewPromptMeta creates prompt metadata with the given toolset.
// Returns mcp.Meta (map[string]any) for direct use in mcp.Prompt.Meta.
func NewPromptMeta(toolset ToolsetMetadata) mcp.Meta {
	if toolset.ID == "" {
		panic("toolset ID is required for PromptMeta")
	}
	return mcp.Meta{"toolset": toolset.ID}
}

var (
	ToolsetMetadataAll = ToolsetMetadata{
		ID:          "all",
		Description: "Special toolset that enables all available toolsets",
	}
	ToolsetMetadataDefault = ToolsetMetadata{
		ID:          "default",
		Description: "Special toolset that enables the default toolset configuration. When no toolsets are specified, this is the set that is enabled",
	}
	ToolsetMetadataContext = ToolsetMetadata{
		ID:          "context",
		Description: "Tools that provide context about the current user and GitHub context you are operating in",
	}
	ToolsetMetadataRepos = ToolsetMetadata{
		ID:          "repos",
		Description: "GitHub Repository related tools",
	}
	ToolsetMetadataGit = ToolsetMetadata{
		ID:          "git",
		Description: "GitHub Git API related tools for low-level Git operations",
	}
	ToolsetMetadataIssues = ToolsetMetadata{
		ID:          "issues",
		Description: "GitHub Issues related tools",
	}
	ToolsetMetadataPullRequests = ToolsetMetadata{
		ID:          "pull_requests",
		Description: "GitHub Pull Request related tools",
	}
	ToolsetMetadataUsers = ToolsetMetadata{
		ID:          "users",
		Description: "GitHub User related tools",
	}
	ToolsetMetadataOrgs = ToolsetMetadata{
		ID:          "orgs",
		Description: "GitHub Organization related tools",
	}
	ToolsetMetadataActions = ToolsetMetadata{
		ID:          "actions",
		Description: "GitHub Actions workflows and CI/CD operations",
	}
	ToolsetMetadataCodeSecurity = ToolsetMetadata{
		ID:          "code_security",
		Description: "Code security related tools, such as GitHub Code Scanning",
	}
	ToolsetMetadataSecretProtection = ToolsetMetadata{
		ID:          "secret_protection",
		Description: "Secret protection related tools, such as GitHub Secret Scanning",
	}
	ToolsetMetadataDependabot = ToolsetMetadata{
		ID:          "dependabot",
		Description: "Dependabot tools",
	}
	ToolsetMetadataNotifications = ToolsetMetadata{
		ID:          "notifications",
		Description: "GitHub Notifications related tools",
	}
	ToolsetMetadataExperiments = ToolsetMetadata{
		ID:          "experiments",
		Description: "Experimental features that are not considered stable yet",
	}
	ToolsetMetadataDiscussions = ToolsetMetadata{
		ID:          "discussions",
		Description: "GitHub Discussions related tools",
	}
	ToolsetMetadataGists = ToolsetMetadata{
		ID:          "gists",
		Description: "GitHub Gist related tools",
	}
	ToolsetMetadataSecurityAdvisories = ToolsetMetadata{
		ID:          "security_advisories",
		Description: "Security advisories related tools",
	}
	ToolsetMetadataProjects = ToolsetMetadata{
		ID:          "projects",
		Description: "GitHub Projects related tools",
	}
	ToolsetMetadataStargazers = ToolsetMetadata{
		ID:          "stargazers",
		Description: "GitHub Stargazers related tools",
	}
	ToolsetMetadataDynamic = ToolsetMetadata{
		ID:          "dynamic",
		Description: "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.",
	}
	ToolsetLabels = ToolsetMetadata{
		ID:          "labels",
		Description: "GitHub Labels related tools",
	}
)

func AvailableToolsets() []toolsets.ToolsetMetadata {
	return []toolsets.ToolsetMetadata{
		ToolsetMetadataContext,
		ToolsetMetadataRepos,
		ToolsetMetadataIssues,
		ToolsetMetadataPullRequests,
		ToolsetMetadataUsers,
		ToolsetMetadataOrgs,
		ToolsetMetadataActions,
		ToolsetMetadataCodeSecurity,
		ToolsetMetadataSecretProtection,
		ToolsetMetadataDependabot,
		ToolsetMetadataNotifications,
		ToolsetMetadataExperiments,
		ToolsetMetadataDiscussions,
		ToolsetMetadataGists,
		ToolsetMetadataSecurityAdvisories,
		ToolsetMetadataProjects,
		ToolsetMetadataStargazers,
		ToolsetLabels,
	}
}

// GetValidToolsetIDs returns a map of all valid toolset IDs for quick lookup
func GetValidToolsetIDs() map[string]bool {
	validIDs := make(map[string]bool)
	for _, tool := range AvailableToolsets() {
		validIDs[tool.ID] = true
	}
	// Add special keywords
	validIDs[ToolsetMetadataAll.ID] = true
	validIDs[ToolsetMetadataDefault.ID] = true
	return validIDs
}

func GetDefaultToolsetIDs() []string {
	return []string{
		ToolsetMetadataContext.ID,
		ToolsetMetadataRepos.ID,
		ToolsetMetadataIssues.ID,
		ToolsetMetadataPullRequests.ID,
		ToolsetMetadataUsers.ID,
	}
}

// NewDefaultToolsetRegistry creates a ToolsetRegistry with all available GitHub tools.
// The registry can then be used to create ToolsetGroups with different configurations.
func NewDefaultToolsetRegistry(getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int, flags FeatureFlags, cache *lockdown.RepoAccessCache) *toolsets.ToolsetRegistry {
	// Collect all tools - they are self-describing with toolset and read/write info in Meta and Annotations
	tools := []toolsets.ServerTool{
		// Context tools
		toolsets.NewServerTool(GetMe(getClient, t)),
		toolsets.NewServerTool(GetTeams(getClient, getGQLClient, t)),
		toolsets.NewServerTool(GetTeamMembers(getGQLClient, t)),

		// Repository tools
		toolsets.NewServerTool(SearchRepositories(getClient, t)),
		toolsets.NewServerTool(GetFileContents(getClient, getRawClient, t)),
		toolsets.NewServerTool(ListCommits(getClient, t)),
		toolsets.NewServerTool(SearchCode(getClient, t)),
		toolsets.NewServerTool(GetCommit(getClient, t)),
		toolsets.NewServerTool(ListBranches(getClient, t)),
		toolsets.NewServerTool(ListTags(getClient, t)),
		toolsets.NewServerTool(GetTag(getClient, t)),
		toolsets.NewServerTool(ListReleases(getClient, t)),
		toolsets.NewServerTool(GetLatestRelease(getClient, t)),
		toolsets.NewServerTool(GetReleaseByTag(getClient, t)),
		toolsets.NewServerTool(CreateOrUpdateFile(getClient, t)),
		toolsets.NewServerTool(CreateRepository(getClient, t)),
		toolsets.NewServerTool(ForkRepository(getClient, t)),
		toolsets.NewServerTool(CreateBranch(getClient, t)),
		toolsets.NewServerTool(PushFiles(getClient, t)),
		toolsets.NewServerTool(DeleteFile(getClient, t)),

		// Git tools
		toolsets.NewServerTool(GetRepositoryTree(getClient, t)),

		// Issue tools
		toolsets.NewServerTool(IssueRead(getClient, getGQLClient, cache, t, flags)),
		toolsets.NewServerTool(SearchIssues(getClient, t)),
		toolsets.NewServerTool(ListIssues(getGQLClient, t)),
		toolsets.NewServerTool(ListIssueTypes(getClient, t)),
		toolsets.NewServerTool(IssueWrite(getClient, getGQLClient, t)),
		toolsets.NewServerTool(AddIssueComment(getClient, t)),
		toolsets.NewServerTool(AssignCopilotToIssue(getGQLClient, t)),
		toolsets.NewServerTool(SubIssueWrite(getClient, t)),

		// User tools
		toolsets.NewServerTool(SearchUsers(getClient, t)),

		// Org tools
		toolsets.NewServerTool(SearchOrgs(getClient, t)),

		// Pull request tools
		toolsets.NewServerTool(PullRequestRead(getClient, cache, t, flags)),
		toolsets.NewServerTool(ListPullRequests(getClient, t)),
		toolsets.NewServerTool(SearchPullRequests(getClient, t)),
		toolsets.NewServerTool(MergePullRequest(getClient, t)),
		toolsets.NewServerTool(UpdatePullRequestBranch(getClient, t)),
		toolsets.NewServerTool(CreatePullRequest(getClient, t)),
		toolsets.NewServerTool(UpdatePullRequest(getClient, getGQLClient, t)),
		toolsets.NewServerTool(RequestCopilotReview(getClient, t)),
		toolsets.NewServerTool(PullRequestReviewWrite(getGQLClient, t)),
		toolsets.NewServerTool(AddCommentToPendingReview(getGQLClient, t)),

		// Code security tools
		toolsets.NewServerTool(GetCodeScanningAlert(getClient, t)),
		toolsets.NewServerTool(ListCodeScanningAlerts(getClient, t)),

		// Secret protection tools
		toolsets.NewServerTool(GetSecretScanningAlert(getClient, t)),
		toolsets.NewServerTool(ListSecretScanningAlerts(getClient, t)),

		// Dependabot tools
		toolsets.NewServerTool(GetDependabotAlert(getClient, t)),
		toolsets.NewServerTool(ListDependabotAlerts(getClient, t)),

		// Notification tools
		toolsets.NewServerTool(ListNotifications(getClient, t)),
		toolsets.NewServerTool(GetNotificationDetails(getClient, t)),
		toolsets.NewServerTool(DismissNotification(getClient, t)),
		toolsets.NewServerTool(MarkAllNotificationsRead(getClient, t)),
		toolsets.NewServerTool(ManageNotificationSubscription(getClient, t)),
		toolsets.NewServerTool(ManageRepositoryNotificationSubscription(getClient, t)),

		// Discussion tools
		toolsets.NewServerTool(ListDiscussions(getGQLClient, t)),
		toolsets.NewServerTool(GetDiscussion(getGQLClient, t)),
		toolsets.NewServerTool(GetDiscussionComments(getGQLClient, t)),
		toolsets.NewServerTool(ListDiscussionCategories(getGQLClient, t)),

		// Actions tools
		toolsets.NewServerTool(ListWorkflows(getClient, t)),
		toolsets.NewServerTool(ListWorkflowRuns(getClient, t)),
		toolsets.NewServerTool(GetWorkflowRun(getClient, t)),
		toolsets.NewServerTool(GetWorkflowRunLogs(getClient, t)),
		toolsets.NewServerTool(ListWorkflowJobs(getClient, t)),
		toolsets.NewServerTool(GetJobLogs(getClient, t, contentWindowSize)),
		toolsets.NewServerTool(ListWorkflowRunArtifacts(getClient, t)),
		toolsets.NewServerTool(DownloadWorkflowRunArtifact(getClient, t)),
		toolsets.NewServerTool(GetWorkflowRunUsage(getClient, t)),
		toolsets.NewServerTool(RunWorkflow(getClient, t)),
		toolsets.NewServerTool(RerunWorkflowRun(getClient, t)),
		toolsets.NewServerTool(RerunFailedJobs(getClient, t)),
		toolsets.NewServerTool(CancelWorkflowRun(getClient, t)),
		toolsets.NewServerTool(DeleteWorkflowRunLogs(getClient, t)),

		// Security advisories tools
		toolsets.NewServerTool(ListGlobalSecurityAdvisories(getClient, t)),
		toolsets.NewServerTool(GetGlobalSecurityAdvisory(getClient, t)),
		toolsets.NewServerTool(ListRepositorySecurityAdvisories(getClient, t)),
		toolsets.NewServerTool(ListOrgRepositorySecurityAdvisories(getClient, t)),

		// Gist tools
		toolsets.NewServerTool(ListGists(getClient, t)),
		toolsets.NewServerTool(GetGist(getClient, t)),
		toolsets.NewServerTool(CreateGist(getClient, t)),
		toolsets.NewServerTool(UpdateGist(getClient, t)),

		// Project tools
		toolsets.NewServerTool(ListProjects(getClient, t)),
		toolsets.NewServerTool(GetProject(getClient, t)),
		toolsets.NewServerTool(ListProjectFields(getClient, t)),
		toolsets.NewServerTool(GetProjectField(getClient, t)),
		toolsets.NewServerTool(ListProjectItems(getClient, t)),
		toolsets.NewServerTool(GetProjectItem(getClient, t)),
		toolsets.NewServerTool(AddProjectItem(getClient, t)),
		toolsets.NewServerTool(DeleteProjectItem(getClient, t)),
		toolsets.NewServerTool(UpdateProjectItem(getClient, t)),

		// Stargazer tools
		toolsets.NewServerTool(ListStarredRepositories(getClient, t)),
		toolsets.NewServerTool(StarRepository(getClient, t)),
		toolsets.NewServerTool(UnstarRepository(getClient, t)),

		// Label tools
		toolsets.NewServerTool(GetLabel(getGQLClient, t)),
		toolsets.NewServerTool(ListLabels(getGQLClient, t)),
		toolsets.NewServerTool(LabelWrite(getGQLClient, t)),
	}

	// Include all available toolsets plus experiments (which has no tools but needs to exist)
	toolsetMetadatas := append(AvailableToolsets(), ToolsetMetadataExperiments)

	// Resource templates - self-describing with toolset in Meta
	resourceTemplates := []toolsets.ServerResourceTemplate{
		toolsets.NewServerResourceTemplate(GetRepositoryResourceContent(getClient, getRawClient, t)),
		toolsets.NewServerResourceTemplate(GetRepositoryResourceBranchContent(getClient, getRawClient, t)),
		toolsets.NewServerResourceTemplate(GetRepositoryResourceCommitContent(getClient, getRawClient, t)),
		toolsets.NewServerResourceTemplate(GetRepositoryResourceTagContent(getClient, getRawClient, t)),
		toolsets.NewServerResourceTemplate(GetRepositoryResourcePrContent(getClient, getRawClient, t)),
	}

	// Prompts - self-describing with toolset in Meta
	prompts := []toolsets.ServerPrompt{
		toolsets.NewServerPrompt(AssignCodingAgentPrompt(t)),
		toolsets.NewServerPrompt(IssueToFixWorkflowPrompt(t)),
	}

	return toolsets.NewToolsetRegistry(toolsetMetadatas, tools).
		WithResourceTemplates(resourceTemplates...).
		WithPrompts(prompts...)
}

// DefaultToolsetGroup creates a ToolsetGroup with the default configuration.
// This is a convenience wrapper around NewDefaultToolsetRegistry for backwards compatibility.
func DefaultToolsetGroup(readOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int, flags FeatureFlags, cache *lockdown.RepoAccessCache) *toolsets.ToolsetGroup {
	registry := NewDefaultToolsetRegistry(getClient, getGQLClient, getRawClient, t, contentWindowSize, flags, cache)
	tsg := registry.NewToolsetGroup(toolsets.ToolsetGroupConfig{
		ReadOnly:        readOnly,
		AvailableScopes: nil, // No scope filtering for backwards compatibility
	})

	tsg.AddDeprecatedToolAliases(DeprecatedToolAliases)

	return tsg
}

// InitDynamicToolset creates a dynamic toolset that can be used to enable other toolsets, and so requires the server and toolset group as arguments
//
//nolint:unused
func InitDynamicToolset(s *mcp.Server, tsg *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) *toolsets.Toolset {
	// Create a new dynamic toolset
	// Need to add the dynamic toolset last so it can be used to enable other toolsets
	dynamicToolSelection := toolsets.NewToolset(ToolsetMetadataDynamic.ID, ToolsetMetadataDynamic.Description).
		AddReadTools(
			toolsets.NewServerTool(ListAvailableToolsets(tsg, t)),
			toolsets.NewServerTool(GetToolsetsTools(tsg, t)),
			toolsets.NewServerTool(EnableToolset(s, tsg, t)),
		)

	dynamicToolSelection.Enabled = true
	return dynamicToolSelection
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
	// Format default tools
	defaultTools := strings.Join(GetDefaultToolsetIDs(), ", ")

	// Format available tools with line breaks for better readability
	allToolsets := AvailableToolsets()
	var availableToolsetsLines []string
	const maxLineLength = 70
	currentLine := ""

	for i, toolset := range allToolsets {
		switch {
		case i == 0:
			currentLine = toolset.ID
		case len(currentLine)+len(toolset.ID)+2 <= maxLineLength:
			currentLine += ", " + toolset.ID
		default:
			availableToolsetsLines = append(availableToolsetsLines, currentLine)
			currentLine = toolset.ID
		}
	}
	if currentLine != "" {
		availableToolsetsLines = append(availableToolsetsLines, currentLine)
	}

	availableToolsets := strings.Join(availableToolsetsLines, ",\n\t     ")

	toolsetsHelp := fmt.Sprintf("Comma-separated list of tool groups to enable (no spaces).\n"+
		"Available: %s\n", availableToolsets) +
		"Special toolset keywords:\n" +
		"  - all: Enables all available toolsets\n" +
		fmt.Sprintf("  - default: Enables the default toolset configuration of:\n\t     %s\n", defaultTools) +
		"Examples:\n" +
		"  - --toolsets=actions,gists,notifications\n" +
		"  - Default + additional: --toolsets=default,actions,gists\n" +
		"  - All tools: --toolsets=all"

	return toolsetsHelp
}

// AddDefaultToolset removes the default toolset and expands it to the actual default toolset IDs
func AddDefaultToolset(result []string) []string {
	hasDefault := false
	seen := make(map[string]bool)
	for _, toolset := range result {
		seen[toolset] = true
		if toolset == ToolsetMetadataDefault.ID {
			hasDefault = true
		}
	}

	// Only expand if "default" keyword was found
	if !hasDefault {
		return result
	}

	result = RemoveToolset(result, ToolsetMetadataDefault.ID)

	for _, defaultToolset := range GetDefaultToolsetIDs() {
		if !seen[defaultToolset] {
			result = append(result, defaultToolset)
		}
	}
	return result
}

// cleanToolsets cleans and handles special toolset keywords:
// - Duplicates are removed from the result
// - Removes whitespaces
// - Validates toolset names and returns invalid ones separately - for warning reporting
// Returns: (toolsets, invalidToolsets)
func CleanToolsets(enabledToolsets []string) ([]string, []string) {
	seen := make(map[string]bool)
	result := make([]string, 0, len(enabledToolsets))
	invalid := make([]string, 0)
	validIDs := GetValidToolsetIDs()

	// Add non-default toolsets, removing duplicates and trimming whitespace
	for _, toolset := range enabledToolsets {
		trimmed := strings.TrimSpace(toolset)
		if trimmed == "" {
			continue
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
			if !validIDs[trimmed] {
				invalid = append(invalid, trimmed)
			}
		}
	}

	return result, invalid
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
