package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// ToolsetMetadata holds metadata for a toolset including its ID and description
type ToolsetMetadata struct {
	ID          string
	Description string
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

func AvailableTools() []ToolsetMetadata {
	return []ToolsetMetadata{
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
		ToolsetMetadataDynamic,
		ToolsetLabels,
	}
}

// GetValidToolsetIDs returns a map of all valid toolset IDs for quick lookup
func GetValidToolsetIDs() map[string]bool {
	validIDs := make(map[string]bool)
	for _, tool := range AvailableTools() {
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

func DefaultToolsetGroup(readOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int, flags FeatureFlags, cache *lockdown.RepoAccessCache) *toolsets.ToolsetGroup {
	tsg := toolsets.NewToolsetGroup(readOnly)

	// Create the dependencies struct that will be passed to all tool handlers
	deps := ToolDependencies{
		GetClient:         getClient,
		GetGQLClient:      getGQLClient,
		GetRawClient:      getRawClient,
		RepoAccessCache:   cache,
		T:                 t,
		Flags:             flags,
		ContentWindowSize: contentWindowSize,
	}

	// Define all available features with their default state (disabled)
	// Create toolsets
	repos := toolsets.NewToolset(ToolsetMetadataRepos.ID, ToolsetMetadataRepos.Description).
		SetDependencies(deps).
		AddReadTools(
			SearchRepositories(t),
			toolsets.NewServerToolLegacy(GetFileContents(getClient, getRawClient, t)),
			toolsets.NewServerToolLegacy(ListCommits(getClient, t)),
			SearchCode(t),
			toolsets.NewServerToolLegacy(GetCommit(getClient, t)),
			toolsets.NewServerToolLegacy(ListBranches(getClient, t)),
			toolsets.NewServerToolLegacy(ListTags(getClient, t)),
			toolsets.NewServerToolLegacy(GetTag(getClient, t)),
			toolsets.NewServerToolLegacy(ListReleases(getClient, t)),
			toolsets.NewServerToolLegacy(GetLatestRelease(getClient, t)),
			toolsets.NewServerToolLegacy(GetReleaseByTag(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(CreateOrUpdateFile(getClient, t)),
			toolsets.NewServerToolLegacy(CreateRepository(getClient, t)),
			toolsets.NewServerToolLegacy(ForkRepository(getClient, t)),
			toolsets.NewServerToolLegacy(CreateBranch(getClient, t)),
			toolsets.NewServerToolLegacy(PushFiles(getClient, t)),
			toolsets.NewServerToolLegacy(DeleteFile(getClient, t)),
		).
		AddResourceTemplates(
			toolsets.NewServerResourceTemplate(GetRepositoryResourceContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceBranchContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceCommitContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceTagContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourcePrContent(getClient, getRawClient, t)),
		)
	git := toolsets.NewToolset(ToolsetMetadataGit.ID, ToolsetMetadataGit.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(GetRepositoryTree(getClient, t)),
		)
	issues := toolsets.NewToolset(ToolsetMetadataIssues.ID, ToolsetMetadataIssues.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(IssueRead(getClient, getGQLClient, cache, t, flags)),
			toolsets.NewServerToolLegacy(SearchIssues(getClient, t)),
			toolsets.NewServerToolLegacy(ListIssues(getGQLClient, t)),
			toolsets.NewServerToolLegacy(ListIssueTypes(getClient, t)),
			toolsets.NewServerToolLegacy(GetLabel(getGQLClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(IssueWrite(getClient, getGQLClient, t)),
			toolsets.NewServerToolLegacy(AddIssueComment(getClient, t)),
			toolsets.NewServerToolLegacy(AssignCopilotToIssue(getGQLClient, t)),
			toolsets.NewServerToolLegacy(SubIssueWrite(getClient, t)),
		).AddPrompts(
		toolsets.NewServerPrompt(AssignCodingAgentPrompt(t)),
		toolsets.NewServerPrompt(IssueToFixWorkflowPrompt(t)),
	)
	users := toolsets.NewToolset(ToolsetMetadataUsers.ID, ToolsetMetadataUsers.Description).
		SetDependencies(deps).
		AddReadTools(
			SearchUsers(t),
		)
	orgs := toolsets.NewToolset(ToolsetMetadataOrgs.ID, ToolsetMetadataOrgs.Description).
		SetDependencies(deps).
		AddReadTools(
			SearchOrgs(t),
		)
	pullRequests := toolsets.NewToolset(ToolsetMetadataPullRequests.ID, ToolsetMetadataPullRequests.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(PullRequestRead(getClient, cache, t, flags)),
			toolsets.NewServerToolLegacy(ListPullRequests(getClient, t)),
			toolsets.NewServerToolLegacy(SearchPullRequests(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(MergePullRequest(getClient, t)),
			toolsets.NewServerToolLegacy(UpdatePullRequestBranch(getClient, t)),
			toolsets.NewServerToolLegacy(CreatePullRequest(getClient, t)),
			toolsets.NewServerToolLegacy(UpdatePullRequest(getClient, getGQLClient, t)),
			toolsets.NewServerToolLegacy(RequestCopilotReview(getClient, t)),
			// Reviews
			toolsets.NewServerToolLegacy(PullRequestReviewWrite(getGQLClient, t)),
			toolsets.NewServerToolLegacy(AddCommentToPendingReview(getGQLClient, t)),
		)
	codeSecurity := toolsets.NewToolset(ToolsetMetadataCodeSecurity.ID, ToolsetMetadataCodeSecurity.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(GetCodeScanningAlert(getClient, t)),
			toolsets.NewServerToolLegacy(ListCodeScanningAlerts(getClient, t)),
		)
	secretProtection := toolsets.NewToolset(ToolsetMetadataSecretProtection.ID, ToolsetMetadataSecretProtection.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(GetSecretScanningAlert(getClient, t)),
			toolsets.NewServerToolLegacy(ListSecretScanningAlerts(getClient, t)),
		)
	dependabot := toolsets.NewToolset(ToolsetMetadataDependabot.ID, ToolsetMetadataDependabot.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(GetDependabotAlert(getClient, t)),
			toolsets.NewServerToolLegacy(ListDependabotAlerts(getClient, t)),
		)

	notifications := toolsets.NewToolset(ToolsetMetadataNotifications.ID, ToolsetMetadataNotifications.Description).
		SetDependencies(deps).
		AddReadTools(
			ListNotifications(t),
			GetNotificationDetails(t),
		).
		AddWriteTools(
			DismissNotification(t),
			MarkAllNotificationsRead(t),
			ManageNotificationSubscription(t),
			ManageRepositoryNotificationSubscription(t),
		)

	discussions := toolsets.NewToolset(ToolsetMetadataDiscussions.ID, ToolsetMetadataDiscussions.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(ListDiscussions(getGQLClient, t)),
			toolsets.NewServerToolLegacy(GetDiscussion(getGQLClient, t)),
			toolsets.NewServerToolLegacy(GetDiscussionComments(getGQLClient, t)),
			toolsets.NewServerToolLegacy(ListDiscussionCategories(getGQLClient, t)),
		)

	actions := toolsets.NewToolset(ToolsetMetadataActions.ID, ToolsetMetadataActions.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(ListWorkflows(getClient, t)),
			toolsets.NewServerToolLegacy(ListWorkflowRuns(getClient, t)),
			toolsets.NewServerToolLegacy(GetWorkflowRun(getClient, t)),
			toolsets.NewServerToolLegacy(GetWorkflowRunLogs(getClient, t)),
			toolsets.NewServerToolLegacy(ListWorkflowJobs(getClient, t)),
			toolsets.NewServerToolLegacy(GetJobLogs(getClient, t, contentWindowSize)),
			toolsets.NewServerToolLegacy(ListWorkflowRunArtifacts(getClient, t)),
			toolsets.NewServerToolLegacy(DownloadWorkflowRunArtifact(getClient, t)),
			toolsets.NewServerToolLegacy(GetWorkflowRunUsage(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(RunWorkflow(getClient, t)),
			toolsets.NewServerToolLegacy(RerunWorkflowRun(getClient, t)),
			toolsets.NewServerToolLegacy(RerunFailedJobs(getClient, t)),
			toolsets.NewServerToolLegacy(CancelWorkflowRun(getClient, t)),
			toolsets.NewServerToolLegacy(DeleteWorkflowRunLogs(getClient, t)),
		)

	securityAdvisories := toolsets.NewToolset(ToolsetMetadataSecurityAdvisories.ID, ToolsetMetadataSecurityAdvisories.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(ListGlobalSecurityAdvisories(getClient, t)),
			toolsets.NewServerToolLegacy(GetGlobalSecurityAdvisory(getClient, t)),
			toolsets.NewServerToolLegacy(ListRepositorySecurityAdvisories(getClient, t)),
			toolsets.NewServerToolLegacy(ListOrgRepositorySecurityAdvisories(getClient, t)),
		)

	// // Keep experiments alive so the system doesn't error out when it's always enabled
	experiments := toolsets.NewToolset(ToolsetMetadataExperiments.ID, ToolsetMetadataExperiments.Description).
		SetDependencies(deps)

	contextTools := toolsets.NewToolset(ToolsetMetadataContext.ID, ToolsetMetadataContext.Description).
		SetDependencies(deps).
		AddReadTools(
			GetMe(t),
			GetTeams(t),
			GetTeamMembers(t),
		)

	gists := toolsets.NewToolset(ToolsetMetadataGists.ID, ToolsetMetadataGists.Description).
		SetDependencies(deps).
		AddReadTools(
			ListGists(t),
			GetGist(t),
		).
		AddWriteTools(
			CreateGist(t),
			UpdateGist(t),
		)

	projects := toolsets.NewToolset(ToolsetMetadataProjects.ID, ToolsetMetadataProjects.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(ListProjects(getClient, t)),
			toolsets.NewServerToolLegacy(GetProject(getClient, t)),
			toolsets.NewServerToolLegacy(ListProjectFields(getClient, t)),
			toolsets.NewServerToolLegacy(GetProjectField(getClient, t)),
			toolsets.NewServerToolLegacy(ListProjectItems(getClient, t)),
			toolsets.NewServerToolLegacy(GetProjectItem(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(AddProjectItem(getClient, t)),
			toolsets.NewServerToolLegacy(DeleteProjectItem(getClient, t)),
			toolsets.NewServerToolLegacy(UpdateProjectItem(getClient, t)),
		)
	stargazers := toolsets.NewToolset(ToolsetMetadataStargazers.ID, ToolsetMetadataStargazers.Description).
		SetDependencies(deps).
		AddReadTools(
			toolsets.NewServerToolLegacy(ListStarredRepositories(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerToolLegacy(StarRepository(getClient, t)),
			toolsets.NewServerToolLegacy(UnstarRepository(getClient, t)),
		)
	labels := toolsets.NewToolset(ToolsetLabels.ID, ToolsetLabels.Description).
		SetDependencies(deps).
		AddReadTools(
			// get
			toolsets.NewServerToolLegacy(GetLabel(getGQLClient, t)),
			// list labels on repo or issue
			toolsets.NewServerToolLegacy(ListLabels(getGQLClient, t)),
		).
		AddWriteTools(
			// create or update
			toolsets.NewServerToolLegacy(LabelWrite(getGQLClient, t)),
		)

	// Add toolsets to the group
	tsg.AddToolset(contextTools)
	tsg.AddToolset(repos)
	tsg.AddToolset(git)
	tsg.AddToolset(issues)
	tsg.AddToolset(orgs)
	tsg.AddToolset(users)
	tsg.AddToolset(pullRequests)
	tsg.AddToolset(actions)
	tsg.AddToolset(codeSecurity)
	tsg.AddToolset(dependabot)
	tsg.AddToolset(secretProtection)
	tsg.AddToolset(notifications)
	tsg.AddToolset(experiments)
	tsg.AddToolset(discussions)
	tsg.AddToolset(gists)
	tsg.AddToolset(securityAdvisories)
	tsg.AddToolset(projects)
	tsg.AddToolset(stargazers)
	tsg.AddToolset(labels)

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
			toolsets.NewServerToolLegacy(ListAvailableToolsets(tsg, t)),
			toolsets.NewServerToolLegacy(GetToolsetsTools(tsg, t)),
			toolsets.NewServerToolLegacy(EnableToolset(s, tsg, t)),
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
	allTools := AvailableTools()
	var availableToolsLines []string
	const maxLineLength = 70
	currentLine := ""

	for i, tool := range allTools {
		switch {
		case i == 0:
			currentLine = tool.ID
		case len(currentLine)+len(tool.ID)+2 <= maxLineLength:
			currentLine += ", " + tool.ID
		default:
			availableToolsLines = append(availableToolsLines, currentLine)
			currentLine = tool.ID
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
