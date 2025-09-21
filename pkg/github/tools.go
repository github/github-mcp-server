package github

import (
	"context"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/toolsets"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v74/github"
	"github.com/mark3labs/mcp-go/server"
	"github.com/shurcooL/githubv4"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

var DefaultTools = []string{"all"}

func DefaultToolsetGroup(readOnly bool, getClient GetClientFn, getGQLClient GetGQLClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc, contentWindowSize int) *toolsets.ToolsetGroup {
	tsg := toolsets.NewToolsetGroup(readOnly)

	// Define all available features with their default state (disabled)
	// Create toolsets
	repos := toolsets.NewToolset("repos", "GitHub Repository related tools").
		AddReadTools(
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
			toolsets.NewServerTool(ListStarredRepositories(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateOrUpdateFile(getClient, t)),
			toolsets.NewServerTool(CreateRepository(getClient, t)),
			toolsets.NewServerTool(ForkRepository(getClient, t)),
			toolsets.NewServerTool(CreateBranch(getClient, t)),
			toolsets.NewServerTool(PushFiles(getClient, t)),
			toolsets.NewServerTool(DeleteFile(getClient, t)),
			toolsets.NewServerTool(StarRepository(getClient, t)),
			toolsets.NewServerTool(UnstarRepository(getClient, t)),
		).
		AddResourceTemplates(
			toolsets.NewServerResourceTemplate(GetRepositoryResourceContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceBranchContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceCommitContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourceTagContent(getClient, getRawClient, t)),
			toolsets.NewServerResourceTemplate(GetRepositoryResourcePrContent(getClient, getRawClient, t)),
		)
	issues := toolsets.NewToolset("issues", "GitHub Issues related tools").
		AddReadTools(
			toolsets.NewServerTool(GetIssue(getClient, t)),
			toolsets.NewServerTool(SearchIssues(getClient, t)),
			toolsets.NewServerTool(ListIssues(getGQLClient, t)),
			toolsets.NewServerTool(GetIssueComments(getClient, t)),
			toolsets.NewServerTool(ListIssueTypes(getClient, t)),
			toolsets.NewServerTool(ListSubIssues(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateIssue(getClient, t)),
			toolsets.NewServerTool(AddIssueComment(getClient, t)),
			toolsets.NewServerTool(UpdateIssue(getClient, getGQLClient, t)),
			toolsets.NewServerTool(AssignCopilotToIssue(getGQLClient, t)),
			toolsets.NewServerTool(AddSubIssue(getClient, t)),
			toolsets.NewServerTool(RemoveSubIssue(getClient, t)),
			toolsets.NewServerTool(ReprioritizeSubIssue(getClient, t)),
		).AddPrompts(
		toolsets.NewServerPrompt(AssignCodingAgentPrompt(t)),
		toolsets.NewServerPrompt(IssueToFixWorkflowPrompt(t)),
	)
	users := toolsets.NewToolset("users", "GitHub User related tools").
		AddReadTools(
			toolsets.NewServerTool(SearchUsers(getClient, t)),
		)
	orgs := toolsets.NewToolset("orgs", "GitHub Organization related tools").
		AddReadTools(
			toolsets.NewServerTool(SearchOrgs(getClient, t)),
		)
	pullRequests := toolsets.NewToolset("pull_requests", "GitHub Pull Request related tools").
		AddReadTools(
			toolsets.NewServerTool(GetPullRequest(getClient, t)),
			toolsets.NewServerTool(ListPullRequests(getClient, t)),
			toolsets.NewServerTool(GetPullRequestFiles(getClient, t)),
			toolsets.NewServerTool(SearchPullRequests(getClient, t)),
			toolsets.NewServerTool(GetPullRequestStatus(getClient, t)),
			toolsets.NewServerTool(GetPullRequestReviewComments(getClient, t)),
			toolsets.NewServerTool(GetPullRequestReviews(getClient, t)),
			toolsets.NewServerTool(GetPullRequestDiff(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(MergePullRequest(getClient, t)),
			toolsets.NewServerTool(UpdatePullRequestBranch(getClient, t)),
			toolsets.NewServerTool(CreatePullRequest(getClient, t)),
			toolsets.NewServerTool(UpdatePullRequest(getClient, getGQLClient, t)),
			toolsets.NewServerTool(RequestCopilotReview(getClient, t)),

			// Reviews
			toolsets.NewServerTool(CreateAndSubmitPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(CreatePendingPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(AddCommentToPendingReview(getGQLClient, t)),
			toolsets.NewServerTool(SubmitPendingPullRequestReview(getGQLClient, t)),
			toolsets.NewServerTool(DeletePendingPullRequestReview(getGQLClient, t)),
		)
	codeSecurity := toolsets.NewToolset("code_security", "Code security related tools, such as GitHub Code Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetCodeScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListCodeScanningAlerts(getClient, t)),
		)
	secretProtection := toolsets.NewToolset("secret_protection", "Secret protection related tools, such as GitHub Secret Scanning").
		AddReadTools(
			toolsets.NewServerTool(GetSecretScanningAlert(getClient, t)),
			toolsets.NewServerTool(ListSecretScanningAlerts(getClient, t)),
		)
	dependabot := toolsets.NewToolset("dependabot", "Dependabot tools").
		AddReadTools(
			toolsets.NewServerTool(GetDependabotAlert(getClient, t)),
			toolsets.NewServerTool(ListDependabotAlerts(getClient, t)),
		)

	notifications := toolsets.NewToolset("notifications", "GitHub Notifications related tools").
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

	discussions := toolsets.NewToolset("discussions", "GitHub Discussions related tools").
		AddReadTools(
			toolsets.NewServerTool(ListDiscussions(getGQLClient, t)),
			toolsets.NewServerTool(GetDiscussion(getGQLClient, t)),
			toolsets.NewServerTool(GetDiscussionComments(getGQLClient, t)),
			toolsets.NewServerTool(ListDiscussionCategories(getGQLClient, t)),
		)

	actions := toolsets.NewToolset("actions", "GitHub Actions workflows and CI/CD operations").
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

	securityAdvisories := toolsets.NewToolset("security_advisories", "Security advisories related tools").
		AddReadTools(
			toolsets.NewServerTool(ListGlobalSecurityAdvisories(getClient, t)),
			toolsets.NewServerTool(GetGlobalSecurityAdvisory(getClient, t)),
			toolsets.NewServerTool(ListRepositorySecurityAdvisories(getClient, t)),
			toolsets.NewServerTool(ListOrgRepositorySecurityAdvisories(getClient, t)),
		)

	// Keep experiments alive so the system doesn't error out when it's always enabled
	experiments := toolsets.NewToolset("experiments", "Experimental features that are not considered stable yet")

	contextTools := toolsets.NewToolset("context", "Tools that provide context about the current user and GitHub context you are operating in").
		AddReadTools(
			toolsets.NewServerTool(GetMe(getClient, t)),
			toolsets.NewServerTool(GetTeams(getClient, getGQLClient, t)),
			toolsets.NewServerTool(GetTeamMembers(getGQLClient, t)),
		)

	gists := toolsets.NewToolset("gists", "GitHub Gist related tools").
		AddReadTools(
			toolsets.NewServerTool(ListGists(getClient, t)),
		).
		AddWriteTools(
			toolsets.NewServerTool(CreateGist(getClient, t)),
			toolsets.NewServerTool(UpdateGist(getClient, t)),
		)

	// Add toolsets to the group
	tsg.AddToolset(contextTools)
	tsg.AddToolset(repos)
	tsg.AddToolset(issues)
	tsg.AddToolset(orgs)
	tsg.AddToolset(users)
	tsg.AddToolset(pullRequests)
	tsg.AddToolset(actions)
	tsg.AddToolset(codeSecurity)
	tsg.AddToolset(secretProtection)
	tsg.AddToolset(dependabot)
	tsg.AddToolset(notifications)
	tsg.AddToolset(experiments)
	tsg.AddToolset(discussions)
	tsg.AddToolset(gists)
	tsg.AddToolset(securityAdvisories)

	return tsg
}

// InitDynamicToolset creates a dynamic toolset that can be used to enable other toolsets, and so requires the server and toolset group as arguments
func InitDynamicToolset(s *server.MCPServer, tsg *toolsets.ToolsetGroup, t translations.TranslationHelperFunc) *toolsets.Toolset {
	// Create a new dynamic toolset
	// Need to add the dynamic toolset last so it can be used to enable other toolsets
	dynamicToolSelection := toolsets.NewToolset("dynamic", "Discover GitHub MCP tools that can help achieve tasks by enabling additional sets of tools, you can control the enablement of any toolset to access its tools when this toolset is enabled.").
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
