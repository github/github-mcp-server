package github

import (
	"context"
	"slices"
	"strings"

	"github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
)

type GetClientFn func(context.Context) (*github.Client, error)
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// Toolset 元数据 constants - these define 所有available 工具集s 和their descriptions.
// Tools use these constants to declare which 工具集 they belong to.
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
		ID:               "context",
		Description:      "Tools that provide context about the current user and GitHub context you are operating in",
		Default:          true,
		Icon:             "person",
		InstructionsFunc: generateContextToolsetInstructions,
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
		ID:               "issues",
		Description:      "GitHub Issues related tools",
		Default:          true,
		Icon:             "issue-opened",
		InstructionsFunc: generateIssuesToolsetInstructions,
	}
	ToolsetMetadataPullRequests = inventory.ToolsetMetadata{
		ID:               "pull_requests",
		Description:      "GitHub Pull Request related tools",
		Default:          true,
		Icon:             "git-pull-request",
		InstructionsFunc: generatePullRequestsToolsetInstructions,
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
	ToolsetMetadataCodeQuality = inventory.ToolsetMetadata{
		ID:          "code_quality",
		Description: "GitHub Code Quality related tools",
		Icon:        "code-square",
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
		ID:               "discussions",
		Description:      "GitHub Discussions related tools",
		Icon:             "comment-discussion",
		InstructionsFunc: generateDiscussionsToolsetInstructions,
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
		ID:               "projects",
		Description:      "GitHub Projects related tools",
		Icon:             "project",
		InstructionsFunc: generateProjectsToolsetInstructions,
	}
	ToolsetMetadataStargazers = inventory.ToolsetMetadata{
		ID:          "stargazers",
		Description: "GitHub Stargazers related tools",
		Icon:        "star",
	}
	ToolsetLabels = inventory.ToolsetMetadata{
		ID:          "labels",
		Description: "GitHub Labels related tools",
		Icon:        "tag",
	}

	ToolsetMetadataCopilot = inventory.ToolsetMetadata{
		ID:          "copilot",
		Description: "Copilot related tools",
		Default:     true,
		Icon:        "copilot",
	}

	// ToolsetMeta数据CopilotIssueIntents is 一个non-默认工具集 that gates the
	// opt-in intent-aware Copilot 议题 assignment 工具. Kept out 的default
	// configuration so its 输入s (rationale, confidence, is_suggestion) do not
	// add schema bloat 到默认工具 surface.
	ToolsetMetadataCopilotIssueIntents = inventory.ToolsetMetadata{
		ID:          "copilot_issue_intents",
		Description: "Opt-in Copilot issue assignment tools that carry intent metadata (rationale, confidence, suggestion)",
		Icon:        "copilot",
	}

	// Feature flag names f或granular 工具 variants.
	// When active, consolidated 工具 are replaced by 单个-purpose granular 工具.
	FeatureFlagIssuesGranular       = "issues_granular"
	FeatureFlagPullRequestsGranular = "pull_requests_granular"
)

// HeaderAllowedFeatureFlags 返回 功能标志 that 客户端s may 启用 via
// X-MCP-Features header. It delegates to AllowedFeatureFlags as 单个
// source of truth.
func HeaderAllowedFeatureFlags() []string {
	return slices.Clone(AllowedFeatureFlags)
}

var (
	// Remote-仅工具集s - these are 仅available 在remote MCP 服务器
	// 但are documented here f或consistency 和to 启用 automated documentation.
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

// AllTools 返回 所有工具 with their embedded 工具集 元数据.
// Tool 函数s 返回 ServerTool directly with 工具集 info.
func AllTools(t translations.TranslationHelperFunc) []inventory.ServerTool {
	return withCSVOutput([]inventory.ServerTool{
		// Context 工具
		GetMe(t),
		GetTeams(t),
		GetTeamMembers(t),

		// Repository 工具
		SearchRepositories(t),
		GetFileContents(t),
		ListCommits(t),
		SearchCode(t),
		SearchCommits(t),
		GetCommit(t),
		GetFileBlame(t),
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
		ListRepositoryCollaborators(t),

		// Git 工具
		GetRepositoryTree(t),

		// Issue 工具
		IssueRead(t),
		SearchIssues(t),
		ListIssues(t),
		ListIssueTypes(t),
		ListIssueFields(t),
		IssueWrite(t),
		AddIssueComment(t),
		SubIssueWrite(t),
		IssueDependencyRead(t),
		IssueDependencyWrite(t),

		// User 工具
		SearchUsers(t),

		// Organization 工具
		SearchOrgs(t),

		// Pull 请求 工具
		PullRequestRead(t),
		ListPullRequests(t),
		SearchPullRequests(t),
		MergePullRequest(t),
		UpdatePullRequestBranch(t),
		CreatePullRequest(t),
		UpdatePullRequest(t),
		PullRequestReviewWrite(t),
		AddCommentToPendingReview(t),
		AddReplyToPullRequestComment(t),

		// Copilot 工具
		AssignCopilotToIssue(t),
		RequestCopilotReview(t),

		// Copilot 议题 intents (non-default, opt-in)
		AssignCopilotToIssueWithIntent(t),

		// Code quality 工具
		GetCodeQualityFinding(t),

		// Code security 工具
		GetCodeScanningAlert(t),
		ListCodeScanningAlerts(t),

		// Secret protection 工具
		GetSecretScanningAlert(t),
		ListSecretScanningAlerts(t),

		// Dependabot 工具
		GetDependabotAlert(t),
		ListDependabotAlerts(t),

		// Notification 工具
		ListNotifications(t),
		GetNotificationDetails(t),
		DismissNotification(t),
		MarkAllNotificationsRead(t),
		ManageNotificationSubscription(t),
		ManageRepositoryNotificationSubscription(t),

		// Discussion 工具
		ListDiscussions(t),
		GetDiscussion(t),
		GetDiscussionComments(t),
		DiscussionCommentWrite(t),
		ListDiscussionCategories(t),

		// Actions 工具
		ActionsList(t),
		ActionsGet(t),
		ActionsRunTrigger(t),
		ActionsGetJobLogs(t),

		// Security advisories 工具
		ListGlobalSecurityAdvisories(t),
		GetGlobalSecurityAdvisory(t),
		ListRepositorySecurityAdvisories(t),
		ListOrgRepositorySecurityAdvisories(t),

		// Gist 工具
		ListGists(t),
		GetGist(t),
		CreateGist(t),
		UpdateGist(t),

		// Project 工具
		ProjectsList(t),
		ProjectsGet(t),
		ProjectsWrite(t),

		// Label 工具
		GetLabel(t),
		GetLabelForLabelsToolset(t),
		ListLabels(t),
		LabelWrite(t),

		// UI 工具 (insiders only)
		UIGet(t),

		// Granular 议题 工具 (feature-flagged, replace consolidated 议题_写入/sub_议题_写入)
		GranularCreateIssue(t),
		GranularUpdateIssueTitle(t),
		GranularUpdateIssueBody(t),
		GranularUpdateIssueAssignees(t),
		GranularUpdateIssueLabels(t),
		GranularUpdateIssueMilestone(t),
		GranularUpdateIssueType(t),
		GranularUpdateIssueState(t),
		GranularAddSubIssue(t),
		GranularRemoveSubIssue(t),
		GranularReprioritizeSubIssue(t),
		GranularSetIssueFields(t),
		GranularAddIssueReaction(t),
		GranularAddIssueCommentReaction(t),

		// Granular 拉取请求 工具 (feature-flagged, replace consolidated 更新_pull_请求/pull_请求_review_写入)
		GranularUpdatePullRequestTitle(t),
		GranularUpdatePullRequestBody(t),
		GranularUpdatePullRequestState(t),
		GranularUpdatePullRequestDraftState(t),
		GranularRequestPullRequestReviewers(t),
		GranularCreatePullRequestReview(t),
		GranularSubmitPendingPullRequestReview(t),
		GranularDeletePendingPullRequestReview(t),
		GranularAddPullRequestReviewComment(t),
		GranularResolveReviewThread(t),
		GranularUnresolveReviewThread(t),
		GranularAddPullRequestReviewCommentReaction(t),
	})
}

// ToBoolPtr converts 一个bool to 一个*bool pointer.
func ToBoolPtr(b bool) *bool {
	return &b
}

// ToStringPtr converts 一个string to 一个*string pointer.
// Returns nil 如果string is 空.
func ToStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GenerateToolsetsHelp generates help text 用于工具集s flag
func GenerateToolsetsHelp() string {
	// Get 工具集 group to derive defaults 和available 工具集s
	// Build() can 仅fail if WithTools specifies invalid 工具 - 不used here
	r, _ := NewInventory(stubTranslator).Build()

	// Format 默认工具 from 元数据 using strings.Builder
	var defaultBuf strings.Builder
	defaultIDs := r.DefaultToolsetIDs()
	for i, id := range defaultIDs {
		if i > 0 {
			defaultBuf.WriteString(", ")
		}
		defaultBuf.WriteString(string(id))
	}

	// Get 所有available 工具集s (excludes 上下文 f或display)
	allToolsets := r.AvailableToolsets("context")
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

	// Build complete help text using strings.Builder
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

// stubTranslat或is 一个passthrough translat或f或cases where we need 一个Inventory
// 但don't need actual translations (e.g., 获取ting 工具集 IDs f或CLI help).
func stubTranslator(_, fallback string) string { return fallback }

// AddDefaultToolset removes 默认工具集 和expands it 到actual 默认工具集 IDs
func AddDefaultToolset(result []string) []string {
	hasDefault := false
	seen := make(map[string]bool)
	for _, toolset := range result {
		seen[toolset] = true
		if toolset == string(ToolsetMetadataDefault.ID) {
			hasDefault = true
		}
	}

	// 仅exp和if "default" keyword was found
	if !hasDefault {
		return result
	}

	result = RemoveToolset(result, string(ToolsetMetadataDefault.ID))

	// Get 默认工具集 IDs 来自Inventory
	// Build() can 仅fail if WithTools specifies invalid 工具 - 不used here
	r, _ := NewInventory(stubTranslator).Build()
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
	return slices.Contains(tools, toCheck)
}

// CleanTools cleans 工具 names by removing duplicates 和trimming whitespace.
// Validation of 工具 existence is done during registration.
func CleanTools(toolNames []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(toolNames))

	// Remove duplicates 和trim whitespace
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

// GetDefaultToolsetIDs 返回 IDs of 工具集s marked as Default.
// 此is 一个convenience 函数 that builds 一个inventory to determine defaults.
func GetDefaultToolsetIDs() []string {
	// Build() can 仅fail if WithTools specifies invalid 工具 - 不used here
	r, _ := NewInventory(stubTranslator).Build()
	ids := r.DefaultToolsetIDs()
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(id)
	}
	return result
}

// RemoteOnlyToolsets 返回 工具集 元数据 f或工具集s that are only
// available 在remote MCP 服务器. 这些are documented 但不registered
// 在local 服务器.
func RemoteOnlyToolsets() []inventory.ToolsetMetadata {
	return []inventory.ToolsetMetadata{
		ToolsetMetadataCopilotSpaces,
		ToolsetMetadataSupportSearch,
	}
}
