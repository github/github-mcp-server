package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIssueGraph(t *testing.T) {
	// Create mock client for tool definition verification
	mockClient := github.NewClient(nil)
	mockGQLClient := githubv4.NewClient(nil)
	cache := stubRepoAccessCache(mockGQLClient, 15*time.Minute)

	tool, _ := GetIssueGraph(
		stubGetClientFn(mockClient),
		stubGetGQLClientFn(mockGQLClient),
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	// Verify toolsnap
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify tool definition
	assert.Equal(t, "issue_graph", tool.Name)
	assert.NotEmpty(t, tool.Description)
	inputSchema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, inputSchema.Properties, "owner")
	assert.Contains(t, inputSchema.Properties, "repo")
	assert.Contains(t, inputSchema.Properties, "issue_number")
	assert.ElementsMatch(t, inputSchema.Required, []string{"owner", "repo", "issue_number"})

	// Verify read-only annotation
	assert.NotNil(t, tool.Annotations)
	assert.True(t, tool.Annotations.ReadOnlyHint)
}

func TestGetIssueGraph_SingleIssue(t *testing.T) {
	// Mock issue data
	mockIssue := &github.Issue{
		Number: github.Ptr(42),
		Title:  github.Ptr("Test Issue"),
		Body:   github.Ptr("This is a test issue body"),
		State:  github.Ptr("open"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Labels: []*github.Label{
			{Name: github.Ptr("bug")},
		},
	}

	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			mock.GetReposIssuesByOwnerByRepoByIssueNumber,
			mockIssue,
		),
		mock.WithRequestMatchHandler(
			mock.GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber,
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
			}),
		),
	)

	mockClient := github.NewClient(mockedHTTPClient)
	mockGQLClient := githubv4.NewClient(nil)
	cache := stubRepoAccessCache(mockGQLClient, 15*time.Minute)

	_, handler := GetIssueGraph(
		stubGetClientFn(mockClient),
		stubGetGQLClientFn(mockGQLClient),
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	args := map[string]any{
		"owner":        "testowner",
		"repo":         "testrepo",
		"issue_number": float64(42),
	}
	request := createMCPRequest(args)

	result, _, err := handler(context.Background(), &request, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Check the result contains expected content
	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "GRAPH SUMMARY")
	assert.Contains(t, textContent.Text, "#42")
	assert.Contains(t, textContent.Text, "Test Issue")
	assert.Contains(t, textContent.Text, "task") // Should be classified as task
}

func TestExtractIssueReferences(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		defaultOwner string
		defaultRepo  string
		expected     []IssueReference
	}{
		{
			name:         "same repo reference",
			text:         "This fixes #123",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "owner", Repo: "repo", Number: 123, IsParent: true},
			},
		},
		{
			name:         "cross repo reference",
			text:         "Related to other/repo#456",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "other", Repo: "repo", Number: 456, IsParent: false},
			},
		},
		{
			name:         "multiple references",
			text:         "Closes #1, related to #2 and other/project#3",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "owner", Repo: "repo", Number: 1, IsParent: true},
				{Owner: "other", Repo: "project", Number: 3, IsParent: false},
				{Owner: "owner", Repo: "repo", Number: 2, IsParent: false},
			},
		},
		{
			name:         "no references",
			text:         "This is just a comment",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected:     []IssueReference{},
		},
		{
			name:         "fixes keyword",
			text:         "Fixes #100",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "owner", Repo: "repo", Number: 100, IsParent: true},
			},
		},
		{
			name:         "resolves keyword",
			text:         "Resolves #200",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "owner", Repo: "repo", Number: 200, IsParent: true},
			},
		},
		{
			name:         "full github issue URL",
			text:         "Related to https://github.com/other/project/issues/789",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "other", Repo: "project", Number: 789, IsParent: false},
			},
		},
		{
			name:         "full github PR URL",
			text:         "See https://github.com/other/project/pull/456 for the fix",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "other", Repo: "project", Number: 456, IsParent: false},
			},
		},
		{
			name:         "mixed URL and shorthand references",
			text:         "Fixes #100, see https://github.com/other/repo/issues/200 and other/project#300",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []IssueReference{
				{Owner: "owner", Repo: "repo", Number: 100, IsParent: true},
				{Owner: "other", Repo: "project", Number: 300, IsParent: false},
				{Owner: "other", Repo: "repo", Number: 200, IsParent: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			refs := extractIssueReferences(tc.text, tc.defaultOwner, tc.defaultRepo)
			assert.Equal(t, len(tc.expected), len(refs))
			for i, expected := range tc.expected {
				if i < len(refs) {
					assert.Equal(t, expected.Owner, refs[i].Owner)
					assert.Equal(t, expected.Repo, refs[i].Repo)
					assert.Equal(t, expected.Number, refs[i].Number)
					assert.Equal(t, expected.IsParent, refs[i].IsParent)
				}
			}
		})
	}
}

func TestSanitizeBodyForGraph(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		maxLines   int
		maxLineLen int
		expected   string
	}{
		{
			name:       "removes URLs",
			body:       "Check https://example.com for details",
			maxLines:   3,
			maxLineLen: 100,
			expected:   "Check [link] for details",
		},
		{
			name:       "removes markdown images",
			body:       "See ![image](https://example.com/img.png) here",
			maxLines:   3,
			maxLineLen: 100,
			expected:   "See [image] here",
		},
		{
			name:       "truncates long lines",
			body:       "This is a very long line that should be truncated because it exceeds the maximum length allowed",
			maxLines:   3,
			maxLineLen: 30,
			expected:   "This is a very long line th...",
		},
		{
			name:       "limits number of lines",
			body:       "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
			maxLines:   2,
			maxLineLen: 100,
			expected:   "Line 1 | Line 2",
		},
		{
			name:       "empty body",
			body:       "",
			maxLines:   3,
			maxLineLen: 100,
			expected:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeBodyForGraph(tc.body, tc.maxLines, tc.maxLineLen)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestClassifyNode(t *testing.T) {
	tests := []struct {
		name         string
		isPR         bool
		labels       []string
		title        string
		issueType    string
		hasSubIssues bool
		expected     NodeType
	}{
		{
			name:     "pull request",
			isPR:     true,
			labels:   []string{},
			title:    "Fix bug",
			expected: NodeTypePR,
		},
		{
			name:     "epic by label",
			isPR:     false,
			labels:   []string{"type: epic", "priority: high"},
			title:    "Project X",
			expected: NodeTypeEpic,
		},
		{
			name:     "epic by title",
			isPR:     false,
			labels:   []string{},
			title:    "[Epic] Major refactoring",
			expected: NodeTypeEpic,
		},
		{
			name:      "epic by issue type",
			isPR:      false,
			labels:    []string{},
			title:     "Major initiative",
			issueType: "Epic",
			expected:  NodeTypeEpic,
		},
		{
			name:         "batch issue",
			isPR:         false,
			labels:       []string{},
			title:        "Backend improvements",
			hasSubIssues: true,
			expected:     NodeTypeBatch,
		},
		{
			name:     "regular task",
			isPR:     false,
			labels:   []string{"bug"},
			title:    "Fix login issue",
			expected: NodeTypeTask,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := classifyNode(tc.isPR, tc.labels, tc.title, tc.issueType, tc.hasSubIssues)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatNodeRef(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		repo       string
		number     int
		focusOwner string
		focusRepo  string
		expected   string
	}{
		{
			name:       "same repo uses short form",
			owner:      "owner",
			repo:       "repo",
			number:     123,
			focusOwner: "owner",
			focusRepo:  "repo",
			expected:   "#123",
		},
		{
			name:       "cross repo uses full form",
			owner:      "other",
			repo:       "project",
			number:     456,
			focusOwner: "owner",
			focusRepo:  "repo",
			expected:   "other/project#456",
		},
		{
			name:       "case insensitive match",
			owner:      "Owner",
			repo:       "Repo",
			number:     789,
			focusOwner: "owner",
			focusRepo:  "repo",
			expected:   "#789",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatNodeRef(tc.owner, tc.repo, tc.number, tc.focusOwner, tc.focusRepo)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatGraphOutput(t *testing.T) {
	graph := &IssueGraph{
		FocusOwner:  "owner",
		FocusRepo:   "repo",
		FocusNumber: 42,
		Summary:     "Focus: #42 (task) \"Test Issue\"\nState: open\n",
		Nodes: []GraphNode{
			{
				Owner:       "owner",
				Repo:        "repo",
				Number:      42,
				NodeType:    NodeTypeTask,
				State:       "open",
				Title:       "Test Issue",
				BodyPreview: "This is a test",
				Depth:       0,
				IsFocus:     true,
			},
		},
		Edges: []GraphEdge{},
	}

	result := formatGraphOutput(graph)

	assert.Contains(t, result, "GRAPH SUMMARY")
	assert.Contains(t, result, "#42|task|open|Test Issue [FOCUS]")
	assert.Contains(t, result, "Preview: This is a test")
	assert.Contains(t, result, "NODES (1 total)")
}

func TestIssueGraphWithSubIssues(t *testing.T) {
	// Mock parent issue
	parentIssue := &github.Issue{
		Number: github.Ptr(100),
		Title:  github.Ptr("Parent Issue"),
		Body:   github.Ptr("Parent body"),
		State:  github.Ptr("open"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
		Labels: []*github.Label{},
	}

	// Mock sub-issues response
	subIssuesJSON := `[{"number": 101, "title": "Sub Issue 1"}, {"number": 102, "title": "Sub Issue 2"}]`

	// Mock sub-issue details
	subIssue1 := &github.Issue{
		Number: github.Ptr(101),
		Title:  github.Ptr("Sub Issue 1"),
		Body:   github.Ptr("Sub issue 1 body"),
		State:  github.Ptr("open"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	subIssue2 := &github.Issue{
		Number: github.Ptr(102),
		Title:  github.Ptr("Sub Issue 2"),
		Body:   github.Ptr("Sub issue 2 body"),
		State:  github.Ptr("open"),
		User: &github.User{
			Login: github.Ptr("testuser"),
		},
	}

	requestCount := int32(0)
	mockedHTTPClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatchHandler(
			mock.GetReposIssuesByOwnerByRepoByIssueNumber,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Determine which issue is being requested based on URL
				path := r.URL.Path
				var issue *github.Issue
				switch {
				case strings.Contains(path, "/101"):
					issue = subIssue1
				case strings.Contains(path, "/102"):
					issue = subIssue2
				default:
					issue = parentIssue
				}
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(issue)
			}),
		),
		mock.WithRequestMatchHandler(
			mock.GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)
				// Return sub-issues only for the parent issue
				path := r.URL.Path
				if strings.Contains(path, "/100/") {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(subIssuesJSON))
				} else {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`[]`))
				}
			}),
		),
	)

	mockClient := github.NewClient(mockedHTTPClient)
	mockGQLClient := githubv4.NewClient(nil)
	cache := stubRepoAccessCache(mockGQLClient, 15*time.Minute)

	_, handler := GetIssueGraph(
		stubGetClientFn(mockClient),
		stubGetGQLClientFn(mockGQLClient),
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	args := map[string]any{
		"owner":        "testowner",
		"repo":         "testrepo",
		"issue_number": float64(100),
	}
	request := createMCPRequest(args)

	result, _, err := handler(context.Background(), &request, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Check the result contains parent and relationships
	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "#100")
	assert.Contains(t, textContent.Text, "Parent Issue")
}

func TestFindCrossReferencedNodeWithAncestors(t *testing.T) {
	// Test scenario: PR #461 is cross-referenced by task #886, and #886's parent is batch #871
	// When searching for batch from #461, we should find #871 via the ancestor chain

	// Create a mock crawler with the graph structure
	crawler := &graphCrawler{
		focusOwner:     "github",
		focusRepo:      "github-mcp-server-remote",
		focusNumber:    461,
		originalOwner:  "github",
		originalRepo:   "github-mcp-server-remote",
		originalNumber: 461,
		nodes:          make(map[string]*GraphNode),
		edges:          make([]GraphEdge, 0),
		parentMap:      make(map[string]string),
	}

	// Add nodes: PR #461 (focus), task #886, batch #871
	prNode := &GraphNode{
		Owner:    "github",
		Repo:     "github-mcp-server-remote",
		Number:   461,
		NodeType: NodeTypePR,
		State:    "merged",
		Title:    "Implement initial scope challenge",
	}
	taskNode := &GraphNode{
		Owner:    "github",
		Repo:     "copilot-agent-services",
		Number:   886,
		NodeType: NodeTypeTask,
		State:    "closed",
		Title:    "Add initial scope challenge to MCP remote server",
	}
	batchNode := &GraphNode{
		Owner:    "github",
		Repo:     "copilot-agent-services",
		Number:   871,
		NodeType: NodeTypeBatch,
		State:    "open",
		Title:    "[Batch] Support scope challenge in remote MCP",
	}

	crawler.nodes[nodeKey("github", "github-mcp-server-remote", 461)] = prNode
	crawler.nodes[nodeKey("github", "copilot-agent-services", 886)] = taskNode
	crawler.nodes[nodeKey("github", "copilot-agent-services", 871)] = batchNode

	// Add edge: task #886 cross-references PR #461
	crawler.edges = append(crawler.edges, GraphEdge{
		FromOwner:  "github",
		FromRepo:   "copilot-agent-services",
		FromNumber: 886,
		ToOwner:    "github",
		ToRepo:     "github-mcp-server-remote",
		ToNumber:   461,
		Relation:   RelationTypeRelated,
	})

	// Add parent relationship: batch #871 is parent of task #886
	taskKey := nodeKey("github", "copilot-agent-services", 886)
	batchKey := nodeKey("github", "copilot-agent-services", 871)
	crawler.parentMap[taskKey] = batchKey

	// Test: findCrossReferencedNode should find batch #871 by traversing ancestors of #886
	prKey := nodeKey("github", "github-mcp-server-remote", 461)
	foundNode := crawler.findCrossReferencedNode(prKey, NodeTypeBatch)

	require.NotNil(t, foundNode, "Should find batch node via cross-ref ancestor traversal")
	assert.Equal(t, "github", foundNode.Owner)
	assert.Equal(t, "copilot-agent-services", foundNode.Repo)
	assert.Equal(t, 871, foundNode.Number)
	assert.Equal(t, NodeTypeBatch, foundNode.NodeType)
}

func TestFindBestFocusCrossRepoAncestors(t *testing.T) {
	// Similar test but through the findBestFocus interface

	crawler := &graphCrawler{
		focusOwner:     "github",
		focusRepo:      "github-mcp-server-remote",
		focusNumber:    461,
		originalOwner:  "github",
		originalRepo:   "github-mcp-server-remote",
		originalNumber: 461,
		nodes:          make(map[string]*GraphNode),
		edges:          make([]GraphEdge, 0),
		parentMap:      make(map[string]string),
	}

	// Add nodes
	crawler.nodes[nodeKey("github", "github-mcp-server-remote", 461)] = &GraphNode{
		Owner:    "github",
		Repo:     "github-mcp-server-remote",
		Number:   461,
		NodeType: NodeTypePR,
	}
	crawler.nodes[nodeKey("github", "copilot-agent-services", 886)] = &GraphNode{
		Owner:    "github",
		Repo:     "copilot-agent-services",
		Number:   886,
		NodeType: NodeTypeTask,
	}
	crawler.nodes[nodeKey("github", "copilot-agent-services", 871)] = &GraphNode{
		Owner:    "github",
		Repo:     "copilot-agent-services",
		Number:   871,
		NodeType: NodeTypeBatch,
	}

	// Add cross-reference edge
	crawler.edges = append(crawler.edges, GraphEdge{
		FromOwner:  "github",
		FromRepo:   "copilot-agent-services",
		FromNumber: 886,
		ToOwner:    "github",
		ToRepo:     "github-mcp-server-remote",
		ToNumber:   461,
		Relation:   RelationTypeRelated,
	})

	// Add parent relationship
	crawler.parentMap[nodeKey("github", "copilot-agent-services", 886)] = nodeKey("github", "copilot-agent-services", 871)

	// Test findBestFocus with "batch" should find #871
	owner, repo, number, source := crawler.findBestFocus("batch")

	assert.Equal(t, "github", owner)
	assert.Equal(t, "copilot-agent-services", repo)
	assert.Equal(t, 871, number)
	assert.Equal(t, FocusSourceCrossRef, source)
}

func TestExtractTasklistItems(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		defaultOwner string
		defaultRepo  string
		expected     []TasklistItem
	}{
		{
			name: "basic unchecked items",
			body: `- [ ] Task one
- [ ] Task two
- [ ] Task three`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{Text: "Task one", Completed: false},
				{Text: "Task two", Completed: false},
				{Text: "Task three", Completed: false},
			},
		},
		{
			name: "mixed checked and unchecked",
			body: `- [x] Completed task
- [ ] Pending task
- [X] Another completed`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{Text: "Completed task", Completed: true},
				{Text: "Pending task", Completed: false},
				{Text: "Another completed", Completed: true},
			},
		},
		{
			name: "items with issue references",
			body: `- [ ] Implement feature #123
- [x] Fix bug in other/repo#456
- [ ] Review https://github.com/owner/repo/pull/789`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{
					Text:      "Implement feature #123",
					Completed: false,
					LinkedRef: &IssueReference{Owner: "owner", Repo: "repo", Number: 123},
				},
				{
					Text:      "Fix bug in other/repo#456",
					Completed: true,
					LinkedRef: &IssueReference{Owner: "other", Repo: "repo", Number: 456},
				},
				{
					Text:      "Review https://github.com/owner/repo/pull/789",
					Completed: false,
					LinkedRef: &IssueReference{Owner: "owner", Repo: "repo", Number: 789},
				},
			},
		},
		{
			name: "asterisk syntax",
			body: `* [ ] Task with asterisk
* [x] Completed asterisk task`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{Text: "Task with asterisk", Completed: false},
				{Text: "Completed asterisk task", Completed: true},
			},
		},
		{
			name: "indented items",
			body: `  - [ ] Indented task
    - [x] More indented task`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{Text: "Indented task", Completed: false},
				{Text: "More indented task", Completed: true},
			},
		},
		{
			name:         "no tasklist items",
			body:         "This is just a regular body without any tasklist items.",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected:     nil,
		},
		{
			name:         "empty body",
			body:         "",
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected:     nil,
		},
		{
			name: "mixed content with tasklist",
			body: `## Tasks

Some description here.

- [ ] First task
- [x] Second task

More text after the list.`,
			defaultOwner: "owner",
			defaultRepo:  "repo",
			expected: []TasklistItem{
				{Text: "First task", Completed: false},
				{Text: "Second task", Completed: true},
			},
		},
		{
			name: "real world example - scope challenge",
			body: `## Tasks

- [ ] Spike OAuth scope challenge escalation (in collaboration with Tyler for VS Code)
- [ ] Reduce scopes initially requested to match VS Code's
- [x] Build production ready scope challenge support in remote server
- [ ] Release scope challenge
- [ ] Work with VS Code team to establish if included by default has any other blockers`,
			defaultOwner: "github",
			defaultRepo:  "copilot-agent-services",
			expected: []TasklistItem{
				{Text: "Spike OAuth scope challenge escalation (in collaboration with Tyler for VS Code)", Completed: false},
				{Text: "Reduce scopes initially requested to match VS Code's", Completed: false},
				{Text: "Build production ready scope challenge support in remote server", Completed: true},
				{Text: "Release scope challenge", Completed: false},
				{Text: "Work with VS Code team to establish if included by default has any other blockers", Completed: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTasklistItems(tc.body, tc.defaultOwner, tc.defaultRepo)

			if tc.expected == nil {
				assert.Nil(t, result)
				return
			}

			require.Equal(t, len(tc.expected), len(result), "number of items should match")

			for i, expected := range tc.expected {
				assert.Equal(t, expected.Text, result[i].Text, "item %d text should match", i)
				assert.Equal(t, expected.Completed, result[i].Completed, "item %d completed status should match", i)

				if expected.LinkedRef != nil {
					require.NotNil(t, result[i].LinkedRef, "item %d should have a linked reference", i)
					assert.Equal(t, expected.LinkedRef.Owner, result[i].LinkedRef.Owner, "item %d linked owner should match", i)
					assert.Equal(t, expected.LinkedRef.Repo, result[i].LinkedRef.Repo, "item %d linked repo should match", i)
					assert.Equal(t, expected.LinkedRef.Number, result[i].LinkedRef.Number, "item %d linked number should match", i)
				} else {
					assert.Nil(t, result[i].LinkedRef, "item %d should not have a linked reference", i)
				}
			}
		})
	}
}

func TestFormatGraphOutputWithTasklist(t *testing.T) {
	graph := &IssueGraph{
		FocusOwner:  "owner",
		FocusRepo:   "repo",
		FocusNumber: 100,
		Summary:     "Focus: #100 (batch) \"Batch with tasklist\"\nState: open\n",
		Nodes: []GraphNode{
			{
				Owner:       "owner",
				Repo:        "repo",
				Number:      100,
				NodeType:    NodeTypeBatch,
				State:       "open",
				Title:       "Batch with tasklist",
				BodyPreview: "Tasks to complete",
				Depth:       0,
				IsFocus:     true,
				TasklistItems: []TasklistItem{
					{Text: "Task one", Completed: true},
					{Text: "Task two", Completed: false},
					{Text: "Task three with #123", Completed: false, LinkedRef: &IssueReference{Owner: "owner", Repo: "repo", Number: 123}},
				},
			},
		},
		Edges: []GraphEdge{},
	}

	result := formatGraphOutput(graph)

	// Verify tasklist section is present
	assert.Contains(t, result, "Tasklist (1/3 completed):")
	assert.Contains(t, result, "[x] Task one")
	assert.Contains(t, result, "[ ] Task two")
	assert.Contains(t, result, "[ ] Task three with #123 → #123")
}
