package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
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
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	// Verify toolsnap
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	// Verify tool definition
	assert.Equal(t, "issue_graph", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "issue_number")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "issue_number"})

	// Verify read-only annotation
	assert.NotNil(t, tool.Annotations)
	assert.True(t, *tool.Annotations.ReadOnlyHint)
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
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	request := createMCPRequest(map[string]interface{}{
		"owner":        "testowner",
		"repo":         "testrepo",
		"issue_number": float64(42),
	})

	result, err := handler(context.Background(), request)
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
			result := classifyNode(tc.isPR, tc.labels, tc.title, tc.hasSubIssues)
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

	requestCount := 0
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
				requestCount++
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
		cache,
		translations.NullTranslationHelper,
		stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
	)

	request := createMCPRequest(map[string]interface{}{
		"owner":        "testowner",
		"repo":         "testrepo",
		"issue_number": float64(100),
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError)

	// Check the result contains parent and relationships
	textContent := getTextResult(t, result)
	assert.Contains(t, textContent.Text, "#100")
	assert.Contains(t, textContent.Text, "Parent Issue")
}
