package tooldiscovery

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

const toolSearchBenchmarkMaxResults = 10

type toolSearchBenchmarkCase struct {
	name     string
	query    string
	toolName string
	toolset  string
}

type toolSearchBenchmarkMetrics struct {
	cases     int
	recallAt1 float64
	recallAt3 float64
	mrrAt10   float64
}

var toolSearchBenchmarkCases = []toolSearchBenchmarkCase{
	{name: "current user", query: "Who am I on GitHub?", toolName: "get_me", toolset: "context"},
	{name: "user teams", query: "Which teams am I a member of?", toolName: "get_teams", toolset: "context"},
	{name: "team members", query: "Who is on the platform team?", toolName: "get_team_members", toolset: "context"},
	{name: "repository discovery", query: "Find repositories about developer tools", toolName: "search_repositories", toolset: "repos"},
	{name: "file contents", query: "Read the contents of a file in a repository", toolName: "get_file_contents", toolset: "repos"},
	{name: "commit history", query: "List commits on a branch", toolName: "list_commits", toolset: "repos"},
	{name: "code search", query: "Search code for a function name", toolName: "search_code", toolset: "repos"},
	{name: "file blame", query: "Show who changed each line in a file", toolName: "get_file_blame", toolset: "repos"},
	{name: "branch listing", query: "List branches in a repository", toolName: "list_branches", toolset: "repos"},
	{name: "repository creation", query: "Create a new GitHub repository", toolName: "create_repository", toolset: "repos"},
	{name: "repository tree", query: "Show the directory tree at a ref", toolName: "get_repository_tree", toolset: "git"},
	{name: "semantic issue search", query: "Search for open issues about login failures", toolName: "search_issues", toolset: "issues"},
	{name: "issue listing", query: "List issues in a repository", toolName: "list_issues", toolset: "issues"},
	{name: "issue details", query: "Read a specific issue", toolName: "issue_read", toolset: "issues"},
	{name: "issue creation and update", query: "Create or update an issue", toolName: "issue_write", toolset: "issues"},
	{name: "issue comments", query: "Add a comment to an issue", toolName: "add_issue_comment", toolset: "issues"},
	{name: "duplicate issues", query: "Find duplicate issues", toolName: "find_duplicate", toolset: "issues"},
	{name: "issue dependencies", query: "Show what issues block an issue", toolName: "issue_dependency_read", toolset: "issues"},
	{name: "pull request details", query: "Read a pull request with its files reviews and diff", toolName: "pull_request_read", toolset: "pull_requests"},
	{name: "pull request listing", query: "List pull requests in a repository", toolName: "list_pull_requests", toolset: "pull_requests"},
	{name: "pull request search", query: "Search pull requests by author", toolName: "search_pull_requests", toolset: "pull_requests"},
	{name: "pull request merge", query: "Merge a pull request", toolName: "merge_pull_request", toolset: "pull_requests"},
	{name: "pull request creation", query: "Open a pull request", toolName: "create_pull_request", toolset: "pull_requests"},
	{name: "pull request reviewers", query: "Request reviewers for a pull request", toolName: "request_pull_request_reviewers", toolset: "pull_requests"},
	{name: "workflow listing", query: "List GitHub Actions workflow runs", toolName: "actions_list", toolset: "actions"},
	{name: "workflow details", query: "Get details for a workflow run", toolName: "actions_get", toolset: "actions"},
	{name: "workflow job logs", query: "Download logs for a failed workflow job", toolName: "get_job_logs", toolset: "actions"},
	{name: "workflow trigger", query: "Run a GitHub Actions workflow", toolName: "actions_run_trigger", toolset: "actions"},
	{name: "code scanning alerts", query: "List code scanning alerts", toolName: "list_code_scanning_alerts", toolset: "code_security"},
	{name: "secret scanning alerts", query: "List secret scanning alerts for exposed secrets", toolName: "list_secret_scanning_alerts", toolset: "secret_protection"},
	{name: "dependabot alerts", query: "List Dependabot alerts", toolName: "list_dependabot_alerts", toolset: "dependabot"},
	{name: "discussion listing", query: "List repository discussions", toolName: "list_discussions", toolset: "discussions"},
	{name: "discussion comments", query: "Read comments on a discussion", toolName: "get_discussion_comments", toolset: "discussions"},
	{name: "project updates", query: "Add an issue to a GitHub project", toolName: "projects_write", toolset: "projects"},
	{name: "notification listing", query: "Show my unread GitHub notifications", toolName: "list_notifications", toolset: "notifications"},
	{name: "organization discovery", query: "Find GitHub organizations", toolName: "search_orgs", toolset: "orgs"},
	{name: "user discovery", query: "Find a GitHub user by username", toolName: "search_users", toolset: "users"},
	{name: "gist listing", query: "List my gists", toolName: "list_gists", toolset: "gists"},
	{name: "code quality finding", query: "Get a code quality finding", toolName: "get_code_quality_finding", toolset: "code_quality"},
	{name: "ruleset reading", query: "Read repository rulesets", toolName: "repository_ruleset_read", toolset: "governance"},
	{name: "security advisories", query: "List global security advisories", toolName: "list_global_security_advisories", toolset: "security_advisories"},
	{name: "Copilot review", query: "Ask Copilot to review a pull request", toolName: "request_copilot_review", toolset: "copilot"},
	{name: "label listing", query: "List repository labels", toolName: "list_label", toolset: "labels"},
	{name: "starred repositories", query: "List repositories I have starred", toolName: "list_starred_repositories", toolset: "stargazers"},
	{name: "release lookup", query: "Get a release by tag", toolName: "get_release_by_tag", toolset: "repos"},
}

func TestToolSearchBenchmark(t *testing.T) {
	firstTools, firstToolsetByName := toolSearchBenchmarkInventory(t)
	secondTools, secondToolsetByName := toolSearchBenchmarkInventory(t)
	validateToolSearchBenchmarkLabels(t, firstToolsetByName)
	validateToolSearchBenchmarkLabels(t, secondToolsetByName)

	first := measureToolSearchBenchmark(t, firstTools)
	second := measureToolSearchBenchmark(t, secondTools)
	require.Equal(t, first, second)
	t.Logf("tool search benchmark: cases=%d recall@1=%.3f recall@3=%.3f mrr@10=%.3f", first.cases, first.recallAt1, first.recallAt3, first.mrrAt10)
}

func BenchmarkToolSearchCorpus(b *testing.B) {
	tools, toolsetByName := toolSearchBenchmarkInventory(b)
	validateToolSearchBenchmarkLabels(b, toolsetByName)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, benchmarkCase := range toolSearchBenchmarkCases {
			_, err := SearchTools(tools, benchmarkCase.query, SearchOptions{MaxResults: toolSearchBenchmarkMaxResults})
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func toolSearchBenchmarkInventory(tb testing.TB) ([]mcp.Tool, map[string]string) {
	tb.Helper()

	inv, err := github.NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"all"}).
		Build()
	require.NoError(tb, err)

	serverTools := inv.AllTools()
	tools := make([]mcp.Tool, 0, len(serverTools))
	toolsetByName := make(map[string]string, len(serverTools))
	for _, serverTool := range serverTools {
		tools = append(tools, serverTool.Tool)
		toolsetByName[serverTool.Tool.Name] = string(serverTool.Toolset.ID)
	}
	return tools, toolsetByName
}

func validateToolSearchBenchmarkLabels(tb testing.TB, toolsetByName map[string]string) {
	tb.Helper()

	seenNames := make(map[string]struct{}, len(toolSearchBenchmarkCases))
	for _, benchmarkCase := range toolSearchBenchmarkCases {
		_, seen := seenNames[benchmarkCase.name]
		require.Falsef(tb, seen, "duplicate benchmark case name %q", benchmarkCase.name)
		seenNames[benchmarkCase.name] = struct{}{}

		actualToolset, exists := toolsetByName[benchmarkCase.toolName]
		require.Truef(tb, exists, "benchmark tool %q does not exist", benchmarkCase.toolName)
		require.Equalf(tb, benchmarkCase.toolset, actualToolset, "benchmark tool %q has unexpected toolset", benchmarkCase.toolName)
	}
}

func measureToolSearchBenchmark(tb testing.TB, tools []mcp.Tool) toolSearchBenchmarkMetrics {
	tb.Helper()

	metrics := toolSearchBenchmarkMetrics{cases: len(toolSearchBenchmarkCases)}
	for _, benchmarkCase := range toolSearchBenchmarkCases {
		results, err := SearchTools(tools, benchmarkCase.query, SearchOptions{MaxResults: toolSearchBenchmarkMaxResults})
		require.NoError(tb, err)

		rank := 0
		for i, result := range results {
			if result.Tool.Name == benchmarkCase.toolName {
				rank = i + 1
				break
			}
		}
		if rank == 1 {
			metrics.recallAt1++
		}
		if rank >= 1 && rank <= 3 {
			metrics.recallAt3++
		}
		if rank >= 1 && rank <= toolSearchBenchmarkMaxResults {
			metrics.mrrAt10 += 1 / float64(rank)
		}
	}

	if metrics.cases > 0 {
		metrics.recallAt1 /= float64(metrics.cases)
		metrics.recallAt3 /= float64(metrics.cases)
		metrics.mrrAt10 /= float64(metrics.cases)
	}
	return metrics
}
