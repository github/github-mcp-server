package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

type Toolset string

const (
	ToolsetContext            Toolset = "context"
	ToolsetRepos              Toolset = "repos"
	ToolsetContents           Toolset = "contents"
	ToolsetReleases           Toolset = "releases"
	ToolsetIssues             Toolset = "issues"
	ToolsetSubIssues          Toolset = "sub_issues"
	ToolsetUsers              Toolset = "users"
	ToolsetOrgs               Toolset = "orgs"
	ToolsetPullRequests       Toolset = "pull_requests"
	ToolsetPullRequestReviews Toolset = "pull_request_reviews"
	ToolsetCodeSecurity       Toolset = "code_security"
	ToolsetSecretProtection   Toolset = "secret_protection"
	ToolsetDependabot         Toolset = "dependabot"
	ToolsetNotifications      Toolset = "notifications"
	ToolsetDiscussions        Toolset = "discussions"
	ToolsetActions            Toolset = "actions"
	ToolsetSecurityAdvisories Toolset = "security_advisories"
	ToolsetExperiments        Toolset = "experiments"
	ToolsetGists              Toolset = "gists"
	ToolsetProjects           Toolset = "projects"
	ToolsetStargazers         Toolset = "stargazers"
	ToolsetDynamic            Toolset = "dynamic"
)

// DefaultToolsets contains the default toolsets to enable
var DefaultToolsets = []Toolset{ToolsetContext, ToolsetUsers, ToolsetRepos, ToolsetContents, ToolsetPullRequests}

// AvailableToolsets contains all available toolsets
var AvailableToolsets = []Toolset{
	ToolsetContext,
	ToolsetRepos,
	ToolsetContents,
	ToolsetReleases,
	ToolsetIssues,
	ToolsetSubIssues,
	ToolsetUsers,
	ToolsetOrgs,
	ToolsetPullRequests,
	ToolsetPullRequestReviews,
	ToolsetCodeSecurity,
	ToolsetSecretProtection,
	ToolsetDependabot,
	ToolsetNotifications,
	ToolsetDiscussions,
	ToolsetActions,
	ToolsetSecurityAdvisories,
	ToolsetExperiments,
	ToolsetGists,
	ToolsetProjects,
	ToolsetStargazers,
	ToolsetDynamic,
}

// DefaultTools returns the default toolset names as strings for CLI flags
func DefaultTools() []string {
	tools := make([]string, len(DefaultToolsets))
	for i, toolset := range DefaultToolsets {
		tools[i] = string(toolset)
	}
	return tools
}

func AvailableTools() []string {
	tools := make([]string, len(AvailableToolsets))
	for i, toolset := range AvailableToolsets {
		tools[i] = string(toolset)
	}
	return tools
}

func DefaultToolsetGroup(readOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int) *toolsets.ToolsetGroup {
	tsg := toolsets.NewToolsetGroup(readOnly)

	// Define all available features with their default state (disabled)
	// Create toolsets
	repos := toolsets.NewToolset(string(ToolsetRepos), "GitHub Repository management").
		AddReadTools(
			toolsets.NewServerTool(SearchRepositories(getClient, t)),
			toolsets.NewServerTool(ListCommits(getClient, t)),
			toolsets.NewServerTool(SearchCode(getClient, t)),
			toolsets.NewServerTool(GetCommit(getClient, t)),
			toolsets.NewServerTool(ListBranches(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateRepository(getClient, t)),
			toolsets.NewServerTool(ForkRepository(getClient, t)),
			toolsets.NewServerTool(CreateBranch(getClient, t)),
		)

	contents := toolsets.NewToolset(string(ToolsetContents), "Repository contents").
		AddReadTools(
			toolsets.NewServerTool(GetFileContents(getClient, getRawClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateOrUpdateFile(getClient, t)),
			toolsets.NewServerTool(PushFiles(getClient, t)),
			toolsets.NewServerTool(DeleteFile(getClient, t)),
		).
		AddResourceTemplates(
			toolsets.NewServerResourceTemplate(GetRepositoryResourceContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceBranchContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceCommitContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceTagContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourcePrContent(getClient, getRawClient, t)),
		)

	releases := toolsets.NewToolset(string(ToolsetReleases), "GitHub Repository releases/tags").
		AddReadTools(
			toolsets.NewServerTool(ListReleases(getClient, t)),
			toolsets.NewServerTool(GetLatestRelease(getClient, t)),
			toolsets.NewServerTool(GetReleaseByTag(getClient, t)),
			toolsets.NewServerTool(ListTags(getClient, t)),
			toolsets.NewServerTool(GetTag(getClient, t)),
		)

	issues := toolsets.NewToolset(string(ToolsetIssues), "GitHub Issues").
		AddReadTools(
			toolsets.NewServerTool(GetIssue(getClient, t)),
			toolsets.NewServerTool(SearchIssues(getClient, t)),
			toolsets.NewServerTool(ListIssues(getGQLClient, t)),
			toolsets.NewServerTool(GetIssueComments(getClient, t)),
			toolsets.NewServerTool(ListIssueTypes(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateIssue(getClient, t)),
			toolsets.NewServerTool(AddIssueComment(getClient, t)),
			toolsets.NewServerTool(UpdateIssue(getClient, getGQLClient, t)),
			toolsets.NewServerTool(AssignCopilotToIssue(getGQLClient, t)),
		).AddPrompts(
		toolsets.NewServerPrompt(AssignCodingAgentPrompt(t)),
		toolsets.NewServerPrompt(IssueToFixWorkflowPrompt(t)),
	)

	subIssues := toolsets.NewToolset(string(ToolsetSubIssues), "Sub-issue management").
		AddReadTools(
			toolsets.NewServerTool(ListSubIssues(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(AddSubIssue(getClient, t)),
			toolsets.NewServerTool(RemoveSubIssue(getClient, t)),
			toolsets.NewServerTool(ReprioritizeSubIssue(getClient, t)),
		)
	users := toolsets.NewToolset(string(ToolsetUsers), "GitHub User related tools").
		AddReadTools(
			toolsets.NewServerTool(SearchUsers(getClient, t)),
		)
	orgs := toolsets.NewToolset(string(ToolsetOrgs), "GitHub Organization related tools").
		AddReadTools(
			toolsets.NewServerTool(SearchOrgs(getClient, t)),
		)
	pullRequests := toolsets.NewToolset(string(ToolsetPullRequests), "GitHub Pull Request operations").
		AddReadTools(
			toolsets.NewServerTool(GetPullRequest(getClient, t)),
			toolsets.NewServerTool(ListPullRequests(getClient, t)),
			toolsets.NewServerTool(GetPullRequestFiles(getClient, t)),
			toolsets.NewServerTool(SearchPullRequests(getClient, t)),
			toolsets.NewServerTool(GetPullRequestStatus(getClient, t)),
			toolsets.NewServerTool(GetPullRequestDiff(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreatePullRequest(getClient, t)),
			toolsets.NewServerTool(UpdatePullRequest(getClient, getGQLClient, t)),
			toolsets.NewServerTool(MergePullRequest(getClient, t)),
			toolsets.NewServerTool(UpdatePullRequestBranch(getClient, t)),
		)

	pullRequestReviews := toolsets.NewToolset(string(ToolsetPullRequestReviews), "Pull request review operations").
		AddReadTools(
			toolsets.NewServerTool(GetPullRequestReviewComments(getClient, t)),
			toolsets.NewServerTool(GetPullRequestReviews(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(RequestCopilotReview(getClient, t)),
			toolsets.NewServerTool(CreateAndSubmitPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(CreatePendingPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(AddCommentToPendingReview(getGQLClient, t)),
			toolsets.NewServerTool(SubmitPendingPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(DeletePendingPullRequestReview(getGQLClient, t)),
		)
	codeSecurity := toolsets.NewToolset(string(ToolsetCodeSecurity), "Code security related tools, such as GitHub Code Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetCodeScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListCodeScanningAlerts(getClient, t)),
		)
	secretProtection := toolsets.NewToolset(string(ToolsetSecretProtection), "Secret protection related tools, such as GitHub Secret Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetSecretScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListSecretScanningAlerts(getClient, t)),
		)
	dependabot := toolsets.NewToolset(string(ToolsetDependabot), "Dependabot tools").
		AddReadTools(
			toolsets.NewServerTool(GetDependabotAlert(getClient, t)),
			toolsets.NewServerTool(ListDependabotAlerts(getClient, t)),
		)

	notifications := toolsets.NewToolset(string(ToolsetNotifications), "GitHub Notifications related tools").
		AddReadTools(
			toolsets.NewServerTool(ListNotifications(getClient, t)),
			toolsets.NewServerTool(GetNotificationDetails(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(DismissNotification(getClient, t)),
			toolsets.NewServerTool(MarkAllNotificationsRead(getClient, t)),
			toolsets.NewServerTool(ManageNotificationSubscription(getClient, t)),
			toolsets.NewServerTool(ManageRepositoryNotificationSubscription(getClient, t)),
		)

	discussions := toolsets.NewToolset(string(ToolsetDiscussions), "GitHub Discussions related tools").
		AddReadTools(
			toolsets.NewServerTool(ListDiscussions(getGQLClient, t)),
			toolsets.NewServerTool(GetDiscussion(getGQLClient, t)),
			toolsets.NewServerTool(GetDiscussionComments(getGQLClient, t)),
			toolsets.NewServerTool(ListDiscussionCategories(getGQLClient, t)),
		)

	actions := toolsets.NewToolset(string(ToolsetActions), "GitHub Actions workflows and CI/CD operations").
		AddReadTools(
			toolsets.NewServerTool(ListWorkflows(getClient, t)),
			toolsets.NewServerTool(ListWorkflowRuns(getClient, t)),
			toolsets.NewServerTool(GetWorkflowRun(getClient, t)),
			toolsets.NewServerTool(GetWorkflowRunLogs(getClient, t)),
			toolsets.NewServerTool(ListWorkflowJobs(getClient, t)),
			toolsets.NewServerTool(GetJobLogs(getClient, t, contentWindowSize)),
			toolsets.NewServerTool(ListWorkflowRunArtifacts(getClient, t)),
			toolsets.NewServerTool(DownloadWorkflowRunArtifact(getClient, t)),
			toolsets.NewServerTool(GetWorkflowRunUsage(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(RunWorkflow(getClient, t)),
			toolsets.NewServerTool(RerunWorkflowRun(getClient, t)),
			toolsets.NewServerTool(RerunFailedJobs(getClient, t)),
			toolsets.NewServerTool(CancelWorkflowRun(getClient, t)),
			toolsets.NewServerTool(DeleteWorkflowRunLogs(getClient, t)),
		)

	securityAdvisories := toolsets.NewToolset(string(ToolsetSecurityAdvisories), "Security advisories related tools").
		AddReadTools(
			toolsets.NewServerTool(ListGlobalSecurityAdvisories(getClient, t)),
			toolsets.NewServerTool(GetGlobalSecurityAdvisory(getClient, t)),
			toolsets.NewServerTool(ListRepositorySecurityAdvisories(getClient, t)),
			toolsets.NewServerTool(ListOrgRepositorySecurityAdvisories(getClient, t)),
		)

	// Keep experiments alive so the system doesn't error out when it's always enabled
	experiments := toolsets.NewToolset(string(ToolsetExperiments), "Experimental features that are not considered stable yet")

	contextTools := toolsets.NewToolset(string(ToolsetContext), "Tools that provide context about the current user and GitHub context you are operating in").
		AddReadTools(
			toolsets.NewServerTool(GetMe(getClient, t)),
			toolsets.NewServerTool(GetTeams(getClient, getGQLClient, t)),
			toolsets.NewServerTool(GetTeamMembers(getGQLClient, t)),
		)

	gists := toolsets.NewToolset(string(ToolsetGists), "GitHub Gist related tools").
		AddReadTools(
			toolsets.NewServerTool(ListGists(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateGist(getClient, t)),
			toolsets.NewServerTool(UpdateGist(getClient, t)),
		)

	projects := toolsets.NewToolset(string(ToolsetProjects), "GitHub Projects related tools").
		AddReadTools(
			toolsets.NewServerTool(ListProjects(getClient, t)),
			toolsets.NewServerTool(GetProject(getClient, t)),
			toolsets.NewServerTool(ListProjectFields(getClient, t)),
			toolsets.NewServerTool(GetProjectField(getClient, t)),
			toolsets.NewServerTool(ListProjectItems(getClient, t)),
			toolsets.NewServerTool(GetProjectItem(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(AddProjectItem(getClient, t)),
			toolsets.NewServerTool(DeleteProjectItem(getClient, t)),
		)

	stargazers := toolsets.NewToolset(string(ToolsetStargazers), "GitHub Starring related tools").
		AddReadTools(toolsets.NewServerTool(ListStarredRepositories(getClient, t))).AddWriteTools(
		toolsets.NewServerTool(StarRepository(getClient, t)),
		toolsets.NewServerTool(UnstarRepository(getClient, t)),
	)

	// Add toolsets to the group
	tsg.AddToolset(contextTools)
	tsg.AddToolset(repos)
	tsg.AddToolset(contents)
	tsg.AddToolset(releases)
	tsg.AddToolset(issues)
	tsg.AddToolset(subIssues)
	tsg.AddToolset(orgs)
	tsg.AddToolset(users)
	tsg.AddToolset(pullRequests)
	tsg.AddToolset(pullRequestReviews)
	tsg.AddToolset(actions)
	tsg.AddToolset(codeSecurity)
	tsg.AddToolset(secretProtection)
	tsg.AddToolset(dependabot)
	tsg.AddToolset(notifications)
	tsg.AddToolset(experiments)
	tsg.AddToolset(discussions)
	tsg.AddToolset(gists)
	tsg.AddToolset(securityAdvisories)
	tsg.AddToolset(projects)
	tsg.AddToolset(stargazers)

	return tsg
}

// InitDynamicToolset creates a dynamic toolset that can be used to enable other toolsets, and so requires the server and toolset group as arguments
func InitDynamicToolset(s *server.MCPServer, tsg *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) *toolsets.Toolset {
	// Create a new dynamic toolset
	// Need to add the dynamic toolset last so it can be used to enable other toolsets
	dynamicToolSelection := toolsets.NewToolset(string(ToolsetDynamic), "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.").
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
	defaultTools := strings.Join(DefaultTools(), ", ")

	// Format available tools with line breaks for better readability
	allTools := AvailableTools()
	var availableToolsLines []string
	const maxLineLength = 70
	currentLine := ""

	for i, tool := range allTools {
		if i == 0 {
			currentLine = tool
		} else if len(currentLine)+len(tool)+2 <= maxLineLength {
			currentLine += ", " + tool
		} else {
			availableToolsLines = append(availableToolsLines, currentLine)
			currentLine = tool
		}
	}
	if currentLine != "" {
		availableToolsLines = append(availableToolsLines, currentLine)
	}

	availableTools := strings.Join(availableToolsLines, ",\n\t     ")

	toolsetsHelp := fmt.Sprintf("Comma-separated list of tool groups to enable (no spaces).\n"+
		"Default: %s\n"+
		"To enable all tools, use all\n"+
		"Available: %s", defaultTools, availableTools)
	return toolsetsHelp
}
