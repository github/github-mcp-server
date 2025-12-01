package github

import (
	"context"
	"fmt"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ConnectIssueGraphPrompt provides a guided workflow for finding and connecting missing relationships
// between issues and PRs in the issue graph
func ConnectIssueGraphPrompt(t translations.TranslationHelperFunc) (tool mcp.Prompt, handler server.PromptHandlerFunc) {
	return mcp.NewPrompt("ConnectIssueGraph",
			mcp.WithPromptDescription(t("PROMPT_CONNECT_ISSUE_GRAPH_DESCRIPTION", "Find and connect missing relationships between issues and PRs in the issue graph")),
			mcp.WithArgument("owner", mcp.ArgumentDescription("Repository owner"), mcp.RequiredArgument()),
			mcp.WithArgument("repo", mcp.ArgumentDescription("Repository name"), mcp.RequiredArgument()),
			mcp.WithArgument("issue_number", mcp.ArgumentDescription("Issue or PR number to analyze"), mcp.RequiredArgument()),
			mcp.WithArgument("additional_repos", mcp.ArgumentDescription("Comma-separated list of additional owner/repo to search (e.g., 'github/copilot,microsoft/vscode')")),
			mcp.WithArgument("known_links", mcp.ArgumentDescription("Comma-separated list of known related issue URLs that should be connected (e.g., epic links in other repos)")),
		), func(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			owner := request.Params.Arguments["owner"]
			repo := request.Params.Arguments["repo"]
			issueNumber := request.Params.Arguments["issue_number"]

			additionalRepos := ""
			if r, exists := request.Params.Arguments["additional_repos"]; exists {
				additionalRepos = fmt.Sprintf("%v", r)
			}

			knownLinks := ""
			if k, exists := request.Params.Arguments["known_links"]; exists {
				knownLinks = fmt.Sprintf("%v", k)
			}

			systemPrompt := `You are a GitHub issue graph connection assistant. Your job is to find missing relationships between issues and PRs and help connect them properly.

WORKFLOW:
1. First, use issue_graph tool on the specified issue/PR to see current relationships
2. Search for potentially related issues and PRs using search_issues and search_pull_requests
3. Identify missing connections (orphaned tasks, PRs without issue links, etc.)
4. For each missing connection, help the user add the appropriate reference
5. Verify the connections by running issue_graph again

RELATIONSHIP TYPES TO LOOK FOR:
- Epic → Batch: Large initiatives broken into batches
- Batch → Task: Parent issues with sub-issues
- Task → PR: Issues with PRs that should "close" them
- Cross-repo: Epics/batches in different repos (e.g., planning repo vs implementation repo)

SEARCH STRATEGIES:
- Search by keywords from the issue title
- Search by feature name or component
- Look for PRs that mention the issue number
- Check for issues with similar labels

ADDING CONNECTIONS:
- PRs should reference issues with "Closes #123" or "Fixes #123" in body
- Cross-repo: "Closes owner/repo#123"
- Sub-issues can be added via the sub_issue_write tool
- Issue bodies can reference related work with "Related to #123"

IMPORTANT:
- Ask the user before making any changes
- Cross-repo epics may need user input (they might not be searchable)
- Some relationships are intentionally loose - confirm with user`

			userPrompt := fmt.Sprintf(`I want to analyze and connect the issue graph for %s/%s#%s.

Please:
1. Run issue_graph on this issue/PR to see current state
2. Search for related issues and PRs that should be connected
3. Identify any orphaned or missing relationships
4. Propose specific connections to add
5. After I approve changes, help me add the connections
6. Verify by running issue_graph again`, owner, repo, issueNumber)

			if additionalRepos != "" {
				userPrompt += fmt.Sprintf(`

Also search these additional repositories for related issues/PRs:
%s`, additionalRepos)
			}

			if knownLinks != "" {
				userPrompt += fmt.Sprintf(`

These are known related issues that should be connected (e.g., epics in other repos):
%s

Please verify these are properly referenced and suggest how to connect them if not.`, knownLinks)
			}

			messages := []mcp.PromptMessage{
				{
					Role:    "user",
					Content: mcp.NewTextContent(systemPrompt),
				},
				{
					Role:    "user",
					Content: mcp.NewTextContent(userPrompt),
				},
				{
					Role: "assistant",
					Content: mcp.NewTextContent(fmt.Sprintf(`I'll help you analyze and connect the issue graph for %s/%s#%s.

Let me start by running the issue_graph tool to see the current state of relationships:

**Step 1: Analyze Current Graph**
I'll call issue_graph to see what connections already exist.

**Step 2: Search for Missing Connections**
Then I'll search for:
- PRs that might close this issue but don't reference it
- Related issues that should be sub-issues or parent issues
- Cross-repo references that might be missing

**Step 3: Propose Connections**
I'll list any missing relationships I find and propose how to connect them.

**Step 4: Make Changes (with your approval)**
After you review, I'll help add the connections.

**Step 5: Verify**
Finally, I'll run issue_graph again to confirm the connections.

Let me start by getting the current issue graph...`, owner, repo, issueNumber)),
				},
			}

			return &mcp.GetPromptResult{
				Messages: messages,
			}, nil
		}
}
