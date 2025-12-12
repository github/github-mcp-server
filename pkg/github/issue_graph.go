package github

import (
	"container/heap"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v79/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

const (
	// MaxGraphDepth is the maximum depth to crawl for related issues
	MaxGraphDepth = 4
	// MaxConcurrentFetches is the maximum number of concurrent API calls
	MaxConcurrentFetches = 5
	// RateLimitBackoff is the base backoff duration when rate limited
	RateLimitBackoff = 100 * time.Millisecond
)

// Crawl priority levels (lower = higher priority)
const (
	PriorityParent   = 0 // Parents are highest priority (must traverse up for context)
	PriorityChild    = 1 // Direct children are next (sub-issues, tasklist items)
	PriorityCrossRef = 2 // Cross-references are lowest priority
)

// crawlItem represents an item to crawl with priority
type crawlItem struct {
	owner      string
	repo       string
	number     int
	depth      int
	priority   int  // Lower = higher priority
	isAncestor bool // true if this is an ancestor of the focus
	isCrossRef bool // true if reached via cross-reference (don't crawl further)
}

// crawlQueue implements heap.Interface for priority queue
type crawlQueue []*crawlItem

func (q crawlQueue) Len() int { return len(q) }

func (q crawlQueue) Less(i, j int) bool {
	// Lower priority number = higher priority
	// If same priority, prefer lower depth (closer to focus)
	if q[i].priority != q[j].priority {
		return q[i].priority < q[j].priority
	}
	return q[i].depth < q[j].depth
}

func (q crawlQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q *crawlQueue) Push(x any) {
	*q = append(*q, x.(*crawlItem))
}

func (q *crawlQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*q = old[:n-1]
	return item
}

// NodeType represents the type of a graph node
type NodeType string

const (
	NodeTypeEpic  NodeType = "epic"
	NodeTypeBatch NodeType = "batch"
	NodeTypeTask  NodeType = "task"
	NodeTypePR    NodeType = "pr"
)

// RelationType represents the relationship between nodes
type RelationType string

const (
	RelationTypeParent  RelationType = "parent"
	RelationTypeChild   RelationType = "child"
	RelationTypeRelated RelationType = "related"
)

// GraphNode represents a node in the issue graph
type GraphNode struct {
	Owner         string         `json:"owner"`
	Repo          string         `json:"repo"`
	Number        int            `json:"number"`
	NodeType      NodeType       `json:"nodeType"`
	State         string         `json:"state"`        // "open", "closed", or "merged" (for PRs)
	StateReason   string         `json:"stateReason"`  // For issues: "completed", "not_planned", "duplicate", "reopened"; for PRs: empty or "merged"
	StatusUpdate  string         `json:"statusUpdate"` // For epics/batches: extracted status from body/comments (on-track, delayed, etc.)
	Title         string         `json:"title"`
	BodyPreview   string         `json:"bodyPreview"`
	TasklistItems []TasklistItem `json:"tasklistItems"` // Legacy tasklist items from issue body (for batches/epics)
	Depth         int            `json:"depth"`
	IsFocus       bool           `json:"isFocus"`
}

// GraphEdge represents an edge in the issue graph
type GraphEdge struct {
	FromOwner  string       `json:"fromOwner"`
	FromRepo   string       `json:"fromRepo"`
	FromNumber int          `json:"fromNumber"`
	ToOwner    string       `json:"toOwner"`
	ToRepo     string       `json:"toRepo"`
	ToNumber   int          `json:"toNumber"`
	Relation   RelationType `json:"relation"`
}

// IssueGraph represents the complete graph structure
type IssueGraph struct {
	FocusOwner   string        `json:"focusOwner"`
	FocusRepo    string        `json:"focusRepo"`
	FocusNumber  int           `json:"focusNumber"`
	Nodes        []GraphNode   `json:"nodes"`
	Edges        []GraphEdge   `json:"edges"`
	Summary      string        `json:"summary"`
	FocusProject []ProjectInfo `json:"focusProject,omitempty"` // Project info for the focus node
	CrawlSummary string        `json:"crawlSummary,omitempty"` // Verbose crawl statistics (when verbose=true)
}

// ProjectInfo represents project name and status for an issue
type ProjectInfo struct {
	ProjectTitle string `json:"projectTitle"`
	Status       string `json:"status,omitempty"`
}

// nodeKey creates a unique key for a node
func nodeKey(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", strings.ToLower(owner), strings.ToLower(repo), number)
}

// repoKey creates a unique key for a repository
func repoKey(owner, repo string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(owner), strings.ToLower(repo))
}

// IssueReference represents a reference to an issue/PR extracted from text
type IssueReference struct {
	Owner    string
	Repo     string
	Number   int
	IsParent bool // true if this appears to be a parent (e.g., "closes #X")
}

// TasklistItem represents a single item from a legacy markdown tasklist
type TasklistItem struct {
	Text       string          `json:"text"`       // The text content of the item (cleaned)
	Completed  bool            `json:"completed"`  // Whether the checkbox is checked
	LinkedRef  *IssueReference `json:"linkedRef"`  // Issue/PR reference if the item links to one
	LinkedNode *GraphNode      `json:"linkedNode"` // Resolved node info if available (not serialized)
}

// Regular expressions for extracting issue references
var (
	// Matches #123 style references (same repo)
	sameRepoRefRegex = regexp.MustCompile(`(?:^|[^\w])#(\d+)`)
	// Matches owner/repo#123 style references (cross-repo)
	crossRepoRefRegex = regexp.MustCompile(`([a-zA-Z0-9](?:[a-zA-Z0-9._-]*[a-zA-Z0-9])?)/([a-zA-Z0-9._-]+)#(\d+)`)
	// Matches full GitHub URLs like https://github.com/owner/repo/issues/123 or /pull/123
	// Note: This regex is used for extracting references from text (issue bodies), not for URL validation.
	// The pattern `https?://` ensures github.com immediately follows the protocol - no other host can precede it.
	// nolint:gosec // G107: This is a reference extraction regex, not a URL validator; owner/repo/number are validated downstream
	githubURLRefRegex = regexp.MustCompile(`https?://(?:www\.)?github\.com/([a-zA-Z0-9](?:[a-zA-Z0-9._-]*[a-zA-Z0-9])?)/([a-zA-Z0-9._-]+)/(?:issues|pull)/(\d+)`)
	// Matches "closes #123", "fixes #123", "resolves #123" patterns (PR linking to issue)
	closesRefRegex = regexp.MustCompile(`(?i)(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\s+(?:(?:([a-zA-Z0-9](?:[a-zA-Z0-9._-]*[a-zA-Z0-9])?)/([a-zA-Z0-9._-]+))?#(\d+))`)
	// URL pattern to remove
	urlRegex = regexp.MustCompile(`https?://[^\s<>\[\]]+`)
	// Markdown image pattern to remove
	imageRegex = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	// Multiple whitespace to collapse
	whitespaceRegex = regexp.MustCompile(`\s+`)
	// HTML tags to remove
	htmlTagRegex = regexp.MustCompile(`<[^>]*>`)
	// Code block patterns to remove before extracting references
	fencedCodeBlockRegex = regexp.MustCompile("(?s)```[^`]*```")
	inlineCodeRegex      = regexp.MustCompile("`[^`]+`")
	// Status patterns for epic/batch tracking (case-insensitive)
	statusPatterns = regexp.MustCompile(`(?i)(?:^|\W)(status|on[- ]?track|delayed|at[- ]?risk|blocked|behind|ahead|eta|target|due|deadline)[:\s]+([^\n]{3,80})`)
	// Markdown tasklist checkbox pattern: - [ ] unchecked, - [x] or - [X] checked
	// Also matches * [ ] and * [x] variants
	tasklistCheckboxRegex = regexp.MustCompile(`(?m)^[\t ]*[-*][\t ]+\[([ xX])\][\t ]+(.+?)$`)
)

// stripCodeBlocks removes fenced code blocks and inline code from text
// This prevents extracting issue references from example code
func stripCodeBlocks(text string) string {
	// Remove fenced code blocks first (```...```)
	text = fencedCodeBlockRegex.ReplaceAllString(text, "")
	// Remove inline code (`...`)
	text = inlineCodeRegex.ReplaceAllString(text, "")
	return text
}

// extractIssueReferences extracts all issue/PR references from text
// It strips code blocks first to avoid picking up example references
func extractIssueReferences(text, defaultOwner, defaultRepo string) []IssueReference {
	// Strip code blocks to avoid extracting references from examples
	text = stripCodeBlocks(text)

	refs := make([]IssueReference, 0)
	seen := make(map[string]bool)

	// Extract "closes/fixes/resolves" references (these indicate parent relationship)
	for _, match := range closesRefRegex.FindAllStringSubmatch(text, -1) {
		owner := defaultOwner
		repo := defaultRepo
		if match[1] != "" && match[2] != "" {
			owner = match[1]
			repo = match[2]
		}
		number := 0
		if _, err := fmt.Sscanf(match[3], "%d", &number); err == nil && number > 0 {
			key := nodeKey(owner, repo, number)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, IssueReference{
					Owner:    owner,
					Repo:     repo,
					Number:   number,
					IsParent: true, // This issue/PR closes another, meaning the other is the parent
				})
			}
		}
	}

	// Extract cross-repo references
	for _, match := range crossRepoRefRegex.FindAllStringSubmatch(text, -1) {
		owner := match[1]
		repo := match[2]
		number := 0
		if _, err := fmt.Sscanf(match[3], "%d", &number); err == nil && number > 0 {
			key := nodeKey(owner, repo, number)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, IssueReference{
					Owner:  owner,
					Repo:   repo,
					Number: number,
				})
			}
		}
	}

	// Extract full GitHub URL references (https://github.com/owner/repo/issues/123)
	for _, match := range githubURLRefRegex.FindAllStringSubmatch(text, -1) {
		owner := match[1]
		repo := match[2]
		number := 0
		if _, err := fmt.Sscanf(match[3], "%d", &number); err == nil && number > 0 {
			key := nodeKey(owner, repo, number)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, IssueReference{
					Owner:  owner,
					Repo:   repo,
					Number: number,
				})
			}
		}
	}

	// Extract same-repo references
	for _, match := range sameRepoRefRegex.FindAllStringSubmatch(text, -1) {
		number := 0
		if _, err := fmt.Sscanf(match[1], "%d", &number); err == nil && number > 0 {
			key := nodeKey(defaultOwner, defaultRepo, number)
			if !seen[key] {
				seen[key] = true
				refs = append(refs, IssueReference{
					Owner:  defaultOwner,
					Repo:   defaultRepo,
					Number: number,
				})
			}
		}
	}

	return refs
}

// extractTasklistItems extracts markdown checkbox tasklist items from issue body text
// This handles legacy tasklists (plain text checkboxes) that are not GitHub sub-issues
func extractTasklistItems(body, defaultOwner, defaultRepo string) []TasklistItem {
	if body == "" {
		return nil
	}

	matches := tasklistCheckboxRegex.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}

	items := make([]TasklistItem, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		checkbox := match[1]
		text := strings.TrimSpace(match[2])

		// Skip empty items
		if text == "" {
			continue
		}

		completed := checkbox == "x" || checkbox == "X"

		item := TasklistItem{
			Text:      text,
			Completed: completed,
		}

		// Check if this item references an issue/PR
		refs := extractIssueReferences(text, defaultOwner, defaultRepo)
		if len(refs) > 0 {
			// Use the first reference found in the item
			item.LinkedRef = &refs[0]
		}

		items = append(items, item)
	}

	return items
}

// sanitizeBodyForGraph sanitizes and truncates the body text for graph display
func sanitizeBodyForGraph(body string, maxLines, maxLineLen int) string {
	if body == "" {
		return ""
	}

	// Remove markdown images first (before URL removal)
	body = imageRegex.ReplaceAllString(body, "[image]")
	// Remove URLs
	body = urlRegex.ReplaceAllString(body, "[link]")
	// Remove HTML tags
	body = htmlTagRegex.ReplaceAllString(body, "")

	// Split into lines first, before collapsing whitespace
	lines := strings.Split(body, "\n")
	result := make([]string, 0, maxLines)

	for _, line := range lines {
		// Collapse multiple whitespace within each line
		line = whitespaceRegex.ReplaceAllString(line, " ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Truncate line if too long
		if len(line) > maxLineLen {
			line = line[:maxLineLen-3] + "..."
		}
		result = append(result, line)
		if len(result) >= maxLines {
			break
		}
	}

	return strings.Join(result, " | ")
}

// getBodyLinesForDepth returns the number of body lines based on depth from focus node
func getBodyLinesForDepth(depth int) int {
	switch depth {
	case 0:
		return 8
	case 1:
		return 5
	case 2:
		return 4
	default:
		return 3
	}
}

// getMaxLineLenForDepth returns the max line length based on depth from focus node
func getMaxLineLenForDepth(depth int) int {
	switch depth {
	case 0:
		return 120
	case 1:
		return 100
	case 2:
		return 80
	default:
		return 60
	}
}

// classifyNode determines the type of a node based on its properties
func classifyNode(isPR bool, labels []string, title string, issueType string, hasSubIssues bool) NodeType {
	if isPR {
		return NodeTypePR
	}

	// Check for epic in issue type (GitHub's issue type feature)
	issueTypeLower := strings.ToLower(issueType)
	if strings.Contains(issueTypeLower, "epic") {
		return NodeTypeEpic
	}

	// Check for epic label or title
	titleLower := strings.ToLower(title)
	for _, label := range labels {
		if strings.Contains(strings.ToLower(label), "epic") {
			return NodeTypeEpic
		}
	}
	if strings.Contains(titleLower, "epic") {
		return NodeTypeEpic
	}

	// If it has sub-issues but is not an epic, it's a batch issue
	if hasSubIssues {
		return NodeTypeBatch
	}

	return NodeTypeTask
}

// extractStatusUpdate extracts status information from issue body and milestone
// This is a lightweight "if lucky" check - returns empty string if no clear status found
func extractStatusUpdate(body string, milestone *github.Milestone) string {
	var statusParts []string

	// Check milestone due date first (most reliable)
	if milestone != nil && milestone.DueOn != nil {
		dueDate := milestone.DueOn.Time
		now := time.Now()
		milestoneName := milestone.GetTitle()

		if dueDate.Before(now) {
			daysOverdue := int(now.Sub(dueDate).Hours() / 24)
			if milestoneName != "" {
				statusParts = append(statusParts, fmt.Sprintf("Milestone '%s' overdue by %d days", milestoneName, daysOverdue))
			} else {
				statusParts = append(statusParts, fmt.Sprintf("Milestone overdue by %d days", daysOverdue))
			}
		} else {
			daysUntil := int(dueDate.Sub(now).Hours() / 24)
			if milestoneName != "" {
				statusParts = append(statusParts, fmt.Sprintf("Milestone '%s' due in %d days", milestoneName, daysUntil))
			} else {
				statusParts = append(statusParts, fmt.Sprintf("Milestone due in %d days", daysUntil))
			}
		}
	}

	// Quick scan of body for status keywords
	if body != "" {
		// Look for status patterns in body
		matches := statusPatterns.FindAllStringSubmatch(body, 3) // limit to 3 matches
		for _, match := range matches {
			if len(match) >= 3 {
				keyword := strings.ToLower(match[1])
				value := strings.TrimSpace(match[2])
				// Truncate long values
				if len(value) > 60 {
					value = value[:57] + "..."
				}
				// Normalize keyword
				switch {
				case keyword == "status":
					statusParts = append(statusParts, fmt.Sprintf("Status: %s", value))
				case strings.Contains(keyword, "track"):
					statusParts = append(statusParts, fmt.Sprintf("On-track: %s", value))
				case strings.Contains(keyword, "delay") || strings.Contains(keyword, "behind"):
					statusParts = append(statusParts, fmt.Sprintf("Delayed: %s", value))
				case strings.Contains(keyword, "risk"):
					statusParts = append(statusParts, fmt.Sprintf("At-risk: %s", value))
				case strings.Contains(keyword, "block"):
					statusParts = append(statusParts, fmt.Sprintf("Blocked: %s", value))
				case strings.Contains(keyword, "eta") || strings.Contains(keyword, "target") ||
					strings.Contains(keyword, "due") || strings.Contains(keyword, "deadline"):
					statusParts = append(statusParts, fmt.Sprintf("Target: %s", value))
				}
			}
		}
	}

	if len(statusParts) == 0 {
		return ""
	}

	// Limit to 2 status parts to keep it concise
	if len(statusParts) > 2 {
		statusParts = statusParts[:2]
	}

	return strings.Join(statusParts, "; ")
}

// extractStatusFromComments fetches recent comments and extracts status (for epics/batches only)
// Only fetches 3 most recent comments to minimize API overhead
func (gc *graphCrawler) extractStatusFromComments(ctx context.Context, owner, repo string, number int, issueBody string, milestone *github.Milestone) string {
	// First try to get status from issue body and milestone
	bodyStatus := extractStatusUpdate(issueBody, milestone)

	// For epics/batches, also check recent comments (if context allows)
	select {
	case <-ctx.Done():
		return bodyStatus // Context cancelled, return what we have
	default:
	}

	// Fetch only the 3 most recent comments (sorted by created desc)
	comments, resp, err := gc.client.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{
		Sort:      github.Ptr("created"),
		Direction: github.Ptr("desc"),
		ListOptions: github.ListOptions{
			PerPage: 3,
		},
	})
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err != nil || len(comments) == 0 {
		return bodyStatus
	}

	// Check recent comments for status updates
	for _, comment := range comments {
		if comment.Body == nil {
			continue
		}
		commentStatus := extractStatusUpdate(*comment.Body, nil)
		if commentStatus != "" {
			// Found status in comment - prepend to body status if different
			if bodyStatus == "" {
				return commentStatus
			}
			if commentStatus != bodyStatus {
				return commentStatus + " | " + bodyStatus
			}
			return bodyStatus
		}
	}

	return bodyStatus
}

// FocusSource describes how the focus node was determined
type FocusSource string

const (
	FocusSourceProvided  FocusSource = "provided"        // User-specified issue/PR
	FocusSourceHierarchy FocusSource = "hierarchy"       // Found via sub-issues/closes chain
	FocusSourceCrossRef  FocusSource = "cross-reference" // Found via mention/cross-reference
)

// graphCrawler manages the concurrent crawling of the issue graph
type graphCrawler struct {
	client           *github.Client
	gqlClient        *githubv4.Client // GraphQL client for parent queries
	cache            *lockdown.RepoAccessCache
	flags            FeatureFlags
	focusOwner       string
	focusRepo        string
	focusNumber      int
	focusSource      FocusSource // how the focus was determined
	focusRequested   string      // what focus type was requested ("epic", "batch", or "")
	originalOwner    string      // original user-provided owner
	originalRepo     string      // original user-provided repo
	originalNumber   int         // original user-provided number
	nodes            map[string]*GraphNode
	edges            []GraphEdge
	parentMap        map[string]string // maps child -> parent
	inaccessibleRepo map[string]bool   // repos we don't have access to
	mu               sync.RWMutex
	sem              chan struct{} // semaphore for concurrency control
	// Crawl statistics for verbose mode
	verbose    bool
	crawlStats crawlStatistics
}

// crawlStatistics tracks crawl metrics for verbose output
type crawlStatistics struct {
	nodesVisited        int
	nodesFetched        int
	subIssuesCrawled    int
	tasklistRefsCrawled int
	timelinesCrawled    int
	crossRefsCrawled    int
	depthReached        int
	reposAccessed       map[string]bool
	timedOut            bool
	rateLimitHits       int
}

func newGraphCrawler(client *github.Client, gqlClient *githubv4.Client, cache *lockdown.RepoAccessCache, flags FeatureFlags, owner, repo string, number int, verbose bool) *graphCrawler {
	return &graphCrawler{
		client:           client,
		gqlClient:        gqlClient,
		cache:            cache,
		flags:            flags,
		focusOwner:       owner,
		focusRepo:        repo,
		focusNumber:      number,
		focusSource:      FocusSourceProvided,
		originalOwner:    owner,
		originalRepo:     repo,
		originalNumber:   number,
		nodes:            make(map[string]*GraphNode),
		edges:            make([]GraphEdge, 0),
		parentMap:        make(map[string]string),
		inaccessibleRepo: make(map[string]bool),
		sem:              make(chan struct{}, MaxConcurrentFetches),
		verbose:          verbose,
		crawlStats:       crawlStatistics{reposAccessed: make(map[string]bool)},
	}
}

// isRepoInaccessible checks if a repo is known to be inaccessible
func (gc *graphCrawler) isRepoInaccessible(owner, repo string) bool {
	gc.mu.RLock()
	defer gc.mu.RUnlock()
	return gc.inaccessibleRepo[repoKey(owner, repo)]
}

// markRepoInaccessible marks a repo as inaccessible
func (gc *graphCrawler) markRepoInaccessible(owner, repo string) {
	gc.mu.Lock()
	defer gc.mu.Unlock()
	gc.inaccessibleRepo[repoKey(owner, repo)] = true
}

// fetchNode fetches a single issue or PR and adds it to the graph
// Returns both the node and the raw issue for further processing
func (gc *graphCrawler) fetchNode(ctx context.Context, owner, repo string, number, depth int) (*GraphNode, *github.Issue, error) {
	key := nodeKey(owner, repo, number)

	// Check if already visited
	gc.mu.RLock()
	if node, exists := gc.nodes[key]; exists {
		gc.mu.RUnlock()
		return node, nil, nil // Already visited, no issue to return
	}
	gc.mu.RUnlock()

	// Check if repo is known to be inaccessible
	if gc.isRepoInaccessible(owner, repo) {
		return nil, nil, nil
	}

	// Acquire semaphore
	select {
	case gc.sem <- struct{}{}:
		defer func() { <-gc.sem }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	// Fetch issue/PR details with retry on rate limit
	var issue *github.Issue
	var resp *github.Response
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		issue, resp, err = gc.client.Issues.Get(ctx, owner, repo, number)
		if err == nil {
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
			// Handle rate limiting with backoff
			if resp.StatusCode == 429 || resp.StatusCode == 403 && resp.Rate.Remaining == 0 {
				gc.crawlStats.rateLimitHits++
				backoff := RateLimitBackoff * time.Duration(1<<attempt) // exponential backoff
				select {
				case <-time.After(backoff):
					continue // retry
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
			}
			// Mark repo as inaccessible for 403 (forbidden) or 404 (not found for entire repo)
			if resp.StatusCode == 403 || resp.StatusCode == 404 {
				// Check if it's a repo-level 404 vs issue-level 404
				// For simplicity, we'll just skip this node
				if resp.StatusCode == 403 && resp.Rate.Remaining > 0 {
					gc.markRepoInaccessible(owner, repo)
				}
				return nil, nil, nil
			}
		}
		// For other errors, don't retry - just skip this node
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, nil // exhausted retries, skip node
	}
	defer func() { _ = resp.Body.Close() }()

	// Check lockdown mode
	if gc.flags.LockdownMode && gc.cache != nil {
		login := issue.GetUser().GetLogin()
		if login != "" {
			isSafeContent, err := gc.cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				// Skip this node if we can't verify safety
				return nil, nil, nil
			}
			if !isSafeContent {
				// Content is restricted, skip but don't fail
				return nil, nil, nil
			}
		}
	}

	isPR := issue.IsPullRequest()

	// Get labels
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if label.Name != nil {
			labels = append(labels, *label.Name)
		}
	}

	// Check for sub-issues (only for issues, not PRs)
	hasSubIssues := false
	if !isPR {
		subIssues, subResp, subErr := gc.client.SubIssue.ListByIssue(ctx, owner, repo, int64(number), &github.IssueListOptions{
			ListOptions: github.ListOptions{PerPage: 1},
		})
		if subErr == nil && len(subIssues) > 0 {
			hasSubIssues = true
		}
		if subResp != nil {
			_ = subResp.Body.Close()
		}
	}

	// Get issue type name if available
	issueTypeName := ""
	if issue.Type != nil {
		issueTypeName = issue.Type.GetName()
	}

	// Determine node type
	nodeType := classifyNode(isPR, labels, issue.GetTitle(), issueTypeName, hasSubIssues)

	// Get state and state reason
	// For PRs: check if merged (via PullRequestLinks.MergedAt)
	// For Issues: use StateReason (completed, not_planned, duplicate, reopened)
	state := issue.GetState()
	stateReason := ""

	if isPR {
		// Check if PR was merged
		prLinks := issue.GetPullRequestLinks()
		if prLinks != nil && !prLinks.GetMergedAt().IsZero() {
			state = "merged"
			stateReason = "merged"
		}
	} else if issue.StateReason != nil {
		// For issues, get the state reason if available
		stateReason = *issue.StateReason
	}

	// Extract status update for epics and batches (lightweight check)
	var statusUpdate string
	var tasklistItems []TasklistItem
	if nodeType == NodeTypeEpic || nodeType == NodeTypeBatch {
		statusUpdate = gc.extractStatusFromComments(ctx, owner, repo, number, issue.GetBody(), issue.Milestone)
		// Extract legacy tasklist items from issue body
		tasklistItems = extractTasklistItems(issue.GetBody(), owner, repo)
	}

	// Create node
	node := &GraphNode{
		Owner:         owner,
		Repo:          repo,
		Number:        number,
		NodeType:      nodeType,
		State:         state,
		StateReason:   stateReason,
		StatusUpdate:  statusUpdate,
		Title:         issue.GetTitle(),
		BodyPreview:   sanitizeBodyForGraph(issue.GetBody(), getBodyLinesForDepth(depth), getMaxLineLenForDepth(depth)),
		TasklistItems: tasklistItems,
		Depth:         depth,
		IsFocus:       strings.EqualFold(owner, gc.focusOwner) && strings.EqualFold(repo, gc.focusRepo) && number == gc.focusNumber,
	}

	// Add to graph
	gc.mu.Lock()
	gc.nodes[key] = node
	gc.mu.Unlock()

	return node, issue, nil
}

// crawlResult represents the result of processing a single node
type crawlResult struct {
	key      string
	node     *GraphNode
	newItems []*crawlItem // New items discovered from this node
	err      error
}

// crawl performs a concurrent BFS crawl from the focus node using a priority queue
func (gc *graphCrawler) crawl(ctx context.Context) error {
	// Initialize priority queue with focus node
	queue := &crawlQueue{}
	heap.Init(queue)
	heap.Push(queue, &crawlItem{
		owner:      gc.focusOwner,
		repo:       gc.focusRepo,
		number:     gc.focusNumber,
		depth:      0,
		priority:   PriorityChild,
		isAncestor: false,
	})

	// Track what's been queued to avoid duplicates
	queued := make(map[string]bool)
	queued[nodeKey(gc.focusOwner, gc.focusRepo, gc.focusNumber)] = true

	// Worker pool for concurrent fetching
	const numWorkers = MaxConcurrentFetches
	jobs := make(chan *crawlItem, numWorkers*2)
	results := make(chan *crawlResult, numWorkers*2)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				result := gc.processNode(ctx, item)
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Track in-flight jobs
	inFlight := 0

	// Main dispatch loop
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			gc.crawlStats.timedOut = true
			close(jobs)
			// Drain results to let workers exit
			for range results { //nolint:revive // intentionally empty - draining channel
			}
			return ctx.Err()
		default:
		}

		// If queue has items and we can dispatch more, do so
		for queue.Len() > 0 && inFlight < numWorkers {
			item := heap.Pop(queue).(*crawlItem)
			key := nodeKey(item.owner, item.repo, item.number)

			// Skip if already visited
			gc.mu.RLock()
			_, visited := gc.nodes[key]
			gc.mu.RUnlock()
			if visited {
				gc.crawlStats.nodesVisited++
				continue
			}

			// Skip if repo is inaccessible
			if gc.isRepoInaccessible(item.owner, item.repo) {
				continue
			}

			// Skip if beyond max depth
			if item.depth > MaxGraphDepth {
				continue
			}

			// Track max depth reached
			if item.depth > gc.crawlStats.depthReached {
				gc.crawlStats.depthReached = item.depth
			}

			// Track repo access
			gc.crawlStats.reposAccessed[repoKey(item.owner, item.repo)] = true

			// Dispatch to worker
			select {
			case jobs <- item:
				inFlight++
			case <-ctx.Done():
				gc.crawlStats.timedOut = true
				close(jobs)
				for range results { //nolint:revive // intentionally empty - draining channel
				}
				return ctx.Err()
			}
		}

		// If nothing in queue and nothing in flight, we're done
		if queue.Len() == 0 && inFlight == 0 {
			close(jobs)
			// Drain any remaining results
			for range results { //nolint:revive // intentionally empty - draining channel
			}
			return nil
		}

		// Wait for a result
		select {
		case result, ok := <-results:
			if !ok {
				// Results channel closed, we're done
				return nil
			}
			inFlight--

			if result.err != nil || result.node == nil {
				continue
			}
			gc.crawlStats.nodesFetched++

			// Add discovered items to queue
			for _, newItem := range result.newItems {
				newKey := nodeKey(newItem.owner, newItem.repo, newItem.number)
				if !queued[newKey] {
					queued[newKey] = true
					heap.Push(queue, newItem)
				}
			}

		case <-ctx.Done():
			gc.crawlStats.timedOut = true
			close(jobs)
			for range results { //nolint:revive // intentionally empty - draining channel
			}
			return ctx.Err()
		}
	}
}

// processNode fetches a single node and discovers related items to crawl
func (gc *graphCrawler) processNode(ctx context.Context, item *crawlItem) *crawlResult {
	result := &crawlResult{
		key:      nodeKey(item.owner, item.repo, item.number),
		newItems: []*crawlItem{},
	}

	// Fetch the node
	node, issue, err := gc.fetchNode(ctx, item.owner, item.repo, item.number, item.depth)
	if err != nil {
		result.err = err
		return result
	}
	result.node = node

	if node == nil || issue == nil {
		return result
	}

	// Don't discover more items from nodes at max depth
	// Also stop crawling from cross-referenced nodes (one hop only)
	if item.depth >= MaxGraphDepth || item.isCrossRef {
		return result
	}

	key := result.key

	// For issues (not PRs), fetch parent via GraphQL and sub-issues via REST
	if !issue.IsPullRequest() {
		// Fetch parent via GraphQL (lightweight query)
		if gc.gqlClient != nil {
			if info := fetchIssueGraphQLInfo(ctx, gc.gqlClient, item.owner, item.repo, item.number); info != nil {
				// Process parent
				if info.Parent != nil {
					parentKey := nodeKey(info.Parent.Owner, info.Parent.Repo, info.Parent.Number)

					gc.mu.Lock()
					gc.parentMap[key] = parentKey
					gc.edges = append(gc.edges, GraphEdge{
						FromOwner:  item.owner,
						FromRepo:   item.repo,
						FromNumber: item.number,
						ToOwner:    info.Parent.Owner,
						ToRepo:     info.Parent.Repo,
						ToNumber:   info.Parent.Number,
						Relation:   RelationTypeParent,
					})
					gc.mu.Unlock()

					// Parents get highest priority, same depth (they're at same level in hierarchy)
					// Mark as ancestor so we don't crawl their other children
					result.newItems = append(result.newItems, &crawlItem{
						owner:      info.Parent.Owner,
						repo:       info.Parent.Repo,
						number:     info.Parent.Number,
						depth:      item.depth, // Same depth - parents are at same level
						priority:   PriorityParent,
						isAncestor: true,
					})
				}
			}
		}

		// Fetch sub-issues via REST API (handles cross-repo)
		// Skip for ancestors - we don't want to crawl siblings of our path to focus
		if !item.isAncestor {
			subIssues, subResp, subErr := gc.client.SubIssue.ListByIssue(ctx, item.owner, item.repo, int64(item.number), &github.IssueListOptions{
				ListOptions: github.ListOptions{PerPage: 50},
			})
			if subErr == nil {
				for _, sub := range subIssues {
					subOwner := item.owner
					subRepo := item.repo
					if sub.Repository != nil {
						if sub.Repository.Owner != nil && sub.Repository.Owner.Login != nil {
							subOwner = *sub.Repository.Owner.Login
						}
						if sub.Repository.Name != nil {
							subRepo = *sub.Repository.Name
						}
					}
					if sub.Number == nil {
						continue
					}
					subNumber := *sub.Number
					subKey := nodeKey(subOwner, subRepo, subNumber)

					gc.mu.Lock()
					gc.parentMap[subKey] = key
					gc.edges = append(gc.edges, GraphEdge{
						FromOwner:  item.owner,
						FromRepo:   item.repo,
						FromNumber: item.number,
						ToOwner:    subOwner,
						ToRepo:     subRepo,
						ToNumber:   subNumber,
						Relation:   RelationTypeChild,
					})
					gc.mu.Unlock()

					gc.crawlStats.subIssuesCrawled++
					result.newItems = append(result.newItems, &crawlItem{
						owner:      subOwner,
						repo:       subRepo,
						number:     subNumber,
						depth:      item.depth + 1,
						priority:   PriorityChild,
						isAncestor: false,
					})
				}
			}
			if subResp != nil {
				_ = subResp.Body.Close()
			}
		}
	}

	// Crawl legacy tasklist linked refs (markdown checkbox items that link to issues/PRs)
	// Skip for ancestors - we don't want to crawl siblings of our path to focus
	if !item.isAncestor && node.TasklistItems != nil {
		for _, taskItem := range node.TasklistItems {
			if taskItem.LinkedRef != nil {
				ref := taskItem.LinkedRef
				if gc.isRepoInaccessible(ref.Owner, ref.Repo) {
					continue
				}

				refKey := nodeKey(ref.Owner, ref.Repo, ref.Number)
				if refKey == key {
					continue
				}

				gc.mu.RLock()
				_, alreadyVisited := gc.nodes[refKey]
				gc.mu.RUnlock()
				if alreadyVisited {
					continue
				}

				gc.mu.Lock()
				gc.parentMap[refKey] = key
				gc.edges = append(gc.edges, GraphEdge{
					FromOwner:  item.owner,
					FromRepo:   item.repo,
					FromNumber: item.number,
					ToOwner:    ref.Owner,
					ToRepo:     ref.Repo,
					ToNumber:   ref.Number,
					Relation:   RelationTypeChild,
				})
				gc.mu.Unlock()

				gc.crawlStats.tasklistRefsCrawled++
				result.newItems = append(result.newItems, &crawlItem{
					owner:      ref.Owner,
					repo:       ref.Repo,
					number:     ref.Number,
					depth:      item.depth + 1,
					priority:   PriorityChild,
					isAncestor: false,
				})
			}
		}
	}

	// Process body references
	bodyRefs := extractIssueReferences(issue.GetBody(), item.owner, item.repo)
	for _, ref := range bodyRefs {
		if gc.isRepoInaccessible(ref.Owner, ref.Repo) {
			continue
		}

		refKey := nodeKey(ref.Owner, ref.Repo, ref.Number)
		if refKey == key {
			continue
		}

		relType := RelationTypeRelated
		priority := PriorityCrossRef
		if ref.IsParent {
			relType = RelationTypeParent
			priority = PriorityParent
			gc.mu.Lock()
			gc.parentMap[key] = refKey
			gc.mu.Unlock()
		}

		gc.mu.Lock()
		gc.edges = append(gc.edges, GraphEdge{
			FromOwner:  item.owner,
			FromRepo:   item.repo,
			FromNumber: item.number,
			ToOwner:    ref.Owner,
			ToRepo:     ref.Repo,
			ToNumber:   ref.Number,
			Relation:   relType,
		})
		gc.mu.Unlock()

		result.newItems = append(result.newItems, &crawlItem{
			owner:      ref.Owner,
			repo:       ref.Repo,
			number:     ref.Number,
			depth:      item.depth + 1,
			priority:   priority,
			isAncestor: false,
			isCrossRef: !ref.IsParent, // Only parent refs (closes/fixes) continue crawling
		})
	}

	// Get cross-referenced issues from timeline - only for focus node to avoid timeout
	if node.IsFocus {
		timelineEvents, timelineResp, err := gc.client.Issues.ListIssueTimeline(ctx, item.owner, item.repo, item.number, &github.ListOptions{
			PerPage: 100,
		})
		if err == nil {
			gc.crawlStats.timelinesCrawled++
			for _, event := range timelineEvents {
				if event.GetEvent() != "cross-referenced" {
					continue
				}

				source := event.GetSource()
				if source == nil {
					continue
				}

				sourceIssue := source.GetIssue()
				if sourceIssue == nil || sourceIssue.Number == nil {
					continue
				}

				refOwner, refRepo := item.owner, item.repo
				if sourceIssue.RepositoryURL != nil {
					parts := strings.Split(*sourceIssue.RepositoryURL, "/")
					if len(parts) >= 2 {
						refOwner = parts[len(parts)-2]
						refRepo = parts[len(parts)-1]
					}
				}

				if gc.isRepoInaccessible(refOwner, refRepo) {
					continue
				}

				refNumber := *sourceIssue.Number
				refKey := nodeKey(refOwner, refRepo, refNumber)
				if refKey == key {
					continue
				}

				gc.mu.Lock()
				gc.edges = append(gc.edges, GraphEdge{
					FromOwner:  refOwner,
					FromRepo:   refRepo,
					FromNumber: refNumber,
					ToOwner:    item.owner,
					ToRepo:     item.repo,
					ToNumber:   item.number,
					Relation:   RelationTypeRelated,
				})
				gc.mu.Unlock()

				gc.crawlStats.crossRefsCrawled++
				result.newItems = append(result.newItems, &crawlItem{
					owner:      refOwner,
					repo:       refRepo,
					number:     refNumber,
					depth:      item.depth + 1,
					priority:   PriorityCrossRef,
					isAncestor: false,
					isCrossRef: true, // Cross-refs only get one hop
				})
			}
		}
		if timelineResp != nil {
			_ = timelineResp.Body.Close()
		}
	}

	return result
}

// edgeKey creates a unique key for an edge to enable deduplication
func edgeKey(e GraphEdge) string {
	return fmt.Sprintf("%s/%s#%d->%s/%s#%d:%s",
		strings.ToLower(e.FromOwner), strings.ToLower(e.FromRepo), e.FromNumber,
		strings.ToLower(e.ToOwner), strings.ToLower(e.ToRepo), e.ToNumber,
		e.Relation)
}

// refocusTo changes the focus node after crawling has completed
// This allows shifting focus to an epic or batch that was discovered
func (gc *graphCrawler) refocusTo(owner, repo string, number int, source FocusSource) {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	// Update focus
	gc.focusOwner = owner
	gc.focusRepo = repo
	gc.focusNumber = number
	gc.focusSource = source

	// Update IsFocus on nodes
	for key, node := range gc.nodes {
		node.IsFocus = key == nodeKey(owner, repo, number)
	}
}

// findBestFocus finds the best node to focus on based on the requested focus type.
// Priority order:
// 1. If original node is already the target type, use it
// 2. Walk up explicit parent hierarchy (sub-issues, closes/fixes) for target type
// 3. If looking for epic but only found batch in hierarchy, use the batch
// 4. Fallback: scan cross-referenced nodes for the target type (best effort)
// Returns owner, repo, number, and source of the best focus node.
func (gc *graphCrawler) findBestFocus(focusType string) (string, string, int, FocusSource) {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	originalKey := nodeKey(gc.focusOwner, gc.focusRepo, gc.focusNumber)

	// Determine target node type
	var targetType NodeType
	switch focusType {
	case "epic":
		targetType = NodeTypeEpic
	case "batch":
		targetType = NodeTypeBatch
	default:
		return gc.focusOwner, gc.focusRepo, gc.focusNumber, FocusSourceProvided
	}

	// First, check if the original focus is already the target type
	if node, exists := gc.nodes[originalKey]; exists && node.NodeType == targetType {
		return gc.focusOwner, gc.focusRepo, gc.focusNumber, FocusSourceProvided
	}

	// Walk up the ancestor chain (explicit hierarchy) to find the nearest target type
	ancestors := gc.findAncestorsUnlocked(originalKey)
	for _, ancestorKey := range ancestors {
		if node, exists := gc.nodes[ancestorKey]; exists && node.NodeType == targetType {
			return node.Owner, node.Repo, node.Number, FocusSourceHierarchy
		}
	}

	// If looking for epic and didn't find one in hierarchy, check for batch
	if targetType == NodeTypeEpic {
		for _, ancestorKey := range ancestors {
			if node, exists := gc.nodes[ancestorKey]; exists && node.NodeType == NodeTypeBatch {
				return node.Owner, node.Repo, node.Number, FocusSourceHierarchy
			}
		}
	}

	// Fallback: scan cross-referenced nodes for the target type
	// This handles cases where an epic is linked via mention but not sub-issue/closes
	crossRefTarget := gc.findCrossReferencedNode(originalKey, targetType)
	if crossRefTarget != nil {
		return crossRefTarget.Owner, crossRefTarget.Repo, crossRefTarget.Number, FocusSourceCrossRef
	}

	// If looking for epic via cross-ref, also accept a batch as fallback
	if targetType == NodeTypeEpic {
		crossRefBatch := gc.findCrossReferencedNode(originalKey, NodeTypeBatch)
		if crossRefBatch != nil {
			return crossRefBatch.Owner, crossRefBatch.Repo, crossRefBatch.Number, FocusSourceCrossRef
		}
	}

	// No suitable focus found, keep original
	return gc.focusOwner, gc.focusRepo, gc.focusNumber, FocusSourceProvided
}

// findCrossReferencedNode finds a node of the target type that is cross-referenced
// from the original node (via RelationTypeRelated edges), including checking the
// ancestors of cross-referenced nodes to find parent epics/batches.
func (gc *graphCrawler) findCrossReferencedNode(fromKey string, targetType NodeType) *GraphNode {
	// Parse the fromKey to get owner/repo/number
	fromNode := gc.nodes[fromKey]
	if fromNode == nil {
		return nil
	}

	// Collect all cross-referenced nodes first
	crossRefKeys := make([]string, 0)
	for _, edge := range gc.edges {
		// Check edges where this node is involved in a related (cross-ref) relationship
		if edge.Relation != RelationTypeRelated {
			continue
		}

		// Determine if this edge connects to our node and get the other end
		var refKey string
		isFrom := strings.EqualFold(edge.FromOwner, fromNode.Owner) &&
			strings.EqualFold(edge.FromRepo, fromNode.Repo) &&
			edge.FromNumber == fromNode.Number
		isTo := strings.EqualFold(edge.ToOwner, fromNode.Owner) &&
			strings.EqualFold(edge.ToRepo, fromNode.Repo) &&
			edge.ToNumber == fromNode.Number

		switch {
		case isFrom:
			refKey = nodeKey(edge.ToOwner, edge.ToRepo, edge.ToNumber)
		case isTo:
			refKey = nodeKey(edge.FromOwner, edge.FromRepo, edge.FromNumber)
		default:
			continue
		}

		crossRefKeys = append(crossRefKeys, refKey)
	}

	// First pass: check if any directly cross-referenced node is the target type
	for _, refKey := range crossRefKeys {
		if node, exists := gc.nodes[refKey]; exists && node.NodeType == targetType {
			return node
		}
	}

	// Second pass: check ancestors of cross-referenced nodes for the target type
	// This handles the case where e.g., PR #461 is cross-ref'd by task #886,
	// and #886's parent batch #871 is what we're looking for
	for _, refKey := range crossRefKeys {
		ancestors := gc.findAncestorsUnlocked(refKey)
		for _, ancestorKey := range ancestors {
			if node, exists := gc.nodes[ancestorKey]; exists && node.NodeType == targetType {
				return node
			}
		}
	}

	return nil
}

// findAncestorsUnlocked finds all ancestors of a node (caller must hold lock)
func (gc *graphCrawler) findAncestorsUnlocked(key string) []string {
	ancestors := make([]string, 0)
	seen := make(map[string]bool)
	current := key

	for {
		parentKey, exists := gc.parentMap[current]
		if !exists || seen[parentKey] {
			break
		}
		seen[parentKey] = true
		ancestors = append(ancestors, parentKey)
		current = parentKey
	}

	return ancestors
}

// buildGraph constructs the final IssueGraph
func (gc *graphCrawler) buildGraph() *IssueGraph {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	// Convert nodes map to slice
	nodes := make([]GraphNode, 0, len(gc.nodes))
	for _, node := range gc.nodes {
		nodes = append(nodes, *node)
	}

	// Sort nodes by depth, then by number
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		return nodes[i].Number < nodes[j].Number
	})

	// Deduplicate edges
	seenEdges := make(map[string]bool)
	uniqueEdges := make([]GraphEdge, 0, len(gc.edges))
	for _, edge := range gc.edges {
		key := edgeKey(edge)
		if !seenEdges[key] {
			seenEdges[key] = true
			uniqueEdges = append(uniqueEdges, edge)
		}
	}

	graph := &IssueGraph{
		FocusOwner:  gc.focusOwner,
		FocusRepo:   gc.focusRepo,
		FocusNumber: gc.focusNumber,
		Nodes:       nodes,
		Edges:       uniqueEdges,
		Summary:     gc.generateSummary(),
	}

	// Add crawl summary if verbose mode
	if gc.verbose {
		graph.CrawlSummary = gc.formatCrawlStats()
	}

	return graph
}

// formatCrawlStats formats crawl statistics for verbose output
func (gc *graphCrawler) formatCrawlStats() string {
	var sb strings.Builder
	sb.WriteString("CRAWL STATISTICS\n")
	sb.WriteString("================\n")
	fmt.Fprintf(&sb, "Nodes fetched: %d\n", gc.crawlStats.nodesFetched)
	fmt.Fprintf(&sb, "Nodes skipped (already visited): %d\n", gc.crawlStats.nodesVisited)
	fmt.Fprintf(&sb, "Max depth reached: %d (limit: %d)\n", gc.crawlStats.depthReached, MaxGraphDepth)
	fmt.Fprintf(&sb, "Sub-issues crawled: %d\n", gc.crawlStats.subIssuesCrawled)
	fmt.Fprintf(&sb, "Tasklist refs crawled: %d\n", gc.crawlStats.tasklistRefsCrawled)
	fmt.Fprintf(&sb, "Timelines checked: %d\n", gc.crawlStats.timelinesCrawled)
	fmt.Fprintf(&sb, "Cross-refs found: %d\n", gc.crawlStats.crossRefsCrawled)
	fmt.Fprintf(&sb, "Repos accessed: %d\n", len(gc.crawlStats.reposAccessed))
	for repo := range gc.crawlStats.reposAccessed {
		fmt.Fprintf(&sb, "  - %s\n", repo)
	}
	if gc.crawlStats.rateLimitHits > 0 {
		fmt.Fprintf(&sb, "⚠️  Rate limit backoffs: %d\n", gc.crawlStats.rateLimitHits)
	}
	if gc.crawlStats.timedOut {
		sb.WriteString("⚠️  Crawl timed out - results may be incomplete\n")
	}
	return sb.String()
}

// writeFocusShiftInfo writes information about focus shifting to the summary
func (gc *graphCrawler) writeFocusShiftInfo(sb *strings.Builder, focusNode *GraphNode) {
	originalRef := formatNodeRef(gc.originalOwner, gc.originalRepo, gc.originalNumber, gc.focusOwner, gc.focusRepo)

	// Case 1: Focus was successfully shifted
	if gc.focusSource != FocusSourceProvided {
		switch gc.focusSource {
		case FocusSourceHierarchy:
			fmt.Fprintf(sb, "Focus shifted: from %s via sub-issue/closes hierarchy\n", originalRef)
		case FocusSourceCrossRef:
			fmt.Fprintf(sb, "Focus shifted: from %s via cross-reference (found closest matching %s - verify this is the correct parent)\n",
				originalRef, focusNode.NodeType)
		}
		return
	}

	// Case 2: Focus shift was requested but no suitable target found
	if gc.focusRequested != "" {
		// Check if the current focus already matches what was requested
		requestedType := NodeType(gc.focusRequested)
		if focusNode.NodeType == requestedType {
			return // Already the right type, no message needed
		}

		// Focus shift failed - provide helpful suggestions
		fmt.Fprintf(sb, "No %s found: searched hierarchy and cross-references from %s\n",
			gc.focusRequested, originalRef)
		sb.WriteString("Suggestions:\n")
		fmt.Fprintf(sb, "  1. Provide a link: if you know the %s, share owner/repo#number\n", gc.focusRequested)
		fmt.Fprintf(sb, "  2. Add a link: reference the %s in the issue body using 'Part of owner/repo#N'\n", gc.focusRequested)
		fmt.Fprintf(sb, "  3. Create an %s: use issue_write to create a new tracking issue\n", gc.focusRequested)
	}
}

// generateSummary creates a natural language summary of the graph
func (gc *graphCrawler) generateSummary() string {
	focusKey := nodeKey(gc.focusOwner, gc.focusRepo, gc.focusNumber)
	focusNode := gc.nodes[focusKey]
	if focusNode == nil {
		return "Unable to fetch the requested issue or pull request."
	}

	var sb strings.Builder

	// Focus node info - include cross-repo reference if different from original
	focusRef := fmt.Sprintf("#%d", gc.focusNumber)
	if gc.focusOwner != gc.originalOwner || gc.focusRepo != gc.originalRepo {
		focusRef = fmt.Sprintf("%s/%s#%d", gc.focusOwner, gc.focusRepo, gc.focusNumber)
	}
	sb.WriteString(fmt.Sprintf("Focus: %s (%s) \"%s\"\n",
		focusRef, focusNode.NodeType, focusNode.Title))

	// Show state with reason if available
	stateStr := focusNode.State
	if focusNode.StateReason != "" && focusNode.StateReason != focusNode.State {
		stateStr = fmt.Sprintf("%s (%s)", focusNode.State, focusNode.StateReason)
	}
	sb.WriteString(fmt.Sprintf("State: %s\n", stateStr))

	// Handle focus shift messaging
	gc.writeFocusShiftInfo(&sb, focusNode)

	// Find hierarchy path (ancestors)
	ancestors := gc.findAncestors(focusKey)
	if len(ancestors) > 0 {
		sb.WriteString("Hierarchy: ")
		for i := len(ancestors) - 1; i >= 0; i-- {
			node := gc.nodes[ancestors[i]]
			if node != nil {
				if strings.EqualFold(node.Owner, gc.focusOwner) && strings.EqualFold(node.Repo, gc.focusRepo) {
					sb.WriteString(fmt.Sprintf("#%d (%s)", node.Number, node.NodeType))
				} else {
					sb.WriteString(fmt.Sprintf("%s/%s#%d (%s)", node.Owner, node.Repo, node.Number, node.NodeType))
				}
				sb.WriteString(" → ")
			}
		}
		sb.WriteString(fmt.Sprintf("#%d (%s)\n",
			gc.focusNumber, focusNode.NodeType))
	}

	// Find children of focus node
	childCount := 0
	for _, edge := range gc.edges {
		if strings.EqualFold(edge.FromOwner, gc.focusOwner) && strings.EqualFold(edge.FromRepo, gc.focusRepo) &&
			edge.FromNumber == gc.focusNumber && edge.Relation == RelationTypeChild {
			childCount++
		}
	}
	if childCount > 0 {
		sb.WriteString(fmt.Sprintf("Direct children: %d\n", childCount))
	}

	// Count siblings (same parent)
	if parentKey, exists := gc.parentMap[focusKey]; exists {
		siblingCount := 0
		for childKey, pKey := range gc.parentMap {
			if pKey == parentKey && childKey != focusKey {
				siblingCount++
			}
		}
		if siblingCount > 0 {
			sb.WriteString(fmt.Sprintf("Siblings (same parent): %d\n", siblingCount))
		}
	}

	sb.WriteString("\n")

	// Count nodes by type
	epicCount, batchCount, taskCount, prCount := 0, 0, 0, 0
	for _, node := range gc.nodes {
		switch node.NodeType {
		case NodeTypeEpic:
			epicCount++
		case NodeTypeBatch:
			batchCount++
		case NodeTypeTask:
			taskCount++
		case NodeTypePR:
			prCount++
		}
	}

	sb.WriteString(fmt.Sprintf("Graph contains %d nodes: ", len(gc.nodes)))
	parts := make([]string, 0)
	if epicCount > 0 {
		parts = append(parts, fmt.Sprintf("%d epic(s)", epicCount))
	}
	if batchCount > 0 {
		parts = append(parts, fmt.Sprintf("%d batch issue(s)", batchCount))
	}
	if taskCount > 0 {
		parts = append(parts, fmt.Sprintf("%d task(s)", taskCount))
	}
	if prCount > 0 {
		parts = append(parts, fmt.Sprintf("%d PR(s)", prCount))
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString("\n")

	return sb.String()
}

// findAncestors returns all ancestors (parents, grandparents, etc.) of a node.
// Called after crawling is complete, so parentMap is stable and no lock needed.
func (gc *graphCrawler) findAncestors(key string) []string {
	return gc.findAncestorsUnlocked(key)
}

// formatNodeRef formats a node reference, using short form (#123) for same-repo
func formatNodeRef(owner, repo string, number int, focusOwner, focusRepo string) string {
	if strings.EqualFold(owner, focusOwner) && strings.EqualFold(repo, focusRepo) {
		return fmt.Sprintf("#%d", number)
	}
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}

// formatGraphOutput formats the graph in a human-readable format optimized for LLMs
func formatGraphOutput(graph *IssueGraph) string {
	var sb strings.Builder

	// Summary section
	sb.WriteString("GRAPH SUMMARY\n")
	sb.WriteString("=============\n")
	sb.WriteString(graph.Summary)

	// Project info for focus node (if available)
	if len(graph.FocusProject) > 0 {
		sb.WriteString("Projects: ")
		projectParts := make([]string, 0, len(graph.FocusProject))
		for _, p := range graph.FocusProject {
			if p.Status != "" {
				projectParts = append(projectParts, fmt.Sprintf("%s [%s]", p.ProjectTitle, p.Status))
			} else {
				projectParts = append(projectParts, p.ProjectTitle)
			}
		}
		sb.WriteString(strings.Join(projectParts, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Legend for node types
	sb.WriteString("Node types: epic (large initiative), batch (has sub-issues), task (regular issue), pr (pull request)\n\n")

	// Nodes section
	sb.WriteString(fmt.Sprintf("NODES (%d total)\n", len(graph.Nodes)))
	sb.WriteString("===============\n")
	for _, node := range graph.Nodes {
		focusMarker := ""
		if node.IsFocus {
			focusMarker = " [FOCUS]"
		}
		nodeRef := formatNodeRef(node.Owner, node.Repo, node.Number, graph.FocusOwner, graph.FocusRepo)
		// Format state with reason if available (e.g., "closed (completed)" or "merged")
		stateStr := node.State
		if node.StateReason != "" && node.StateReason != node.State {
			stateStr = fmt.Sprintf("%s (%s)", node.State, node.StateReason)
		}
		sb.WriteString(fmt.Sprintf("%s|%s|%s|%s%s\n",
			nodeRef, node.NodeType, stateStr, node.Title, focusMarker))
		if node.BodyPreview != "" {
			sb.WriteString(fmt.Sprintf("  Preview: %s\n", node.BodyPreview))
		}
		if node.StatusUpdate != "" {
			sb.WriteString(fmt.Sprintf("  Status: %s\n", node.StatusUpdate))
		}
		// Display tasklist items for batch/epic issues
		if len(node.TasklistItems) > 0 {
			completedCount := 0
			for _, item := range node.TasklistItems {
				if item.Completed {
					completedCount++
				}
			}
			sb.WriteString(fmt.Sprintf("  Tasklist (%d/%d completed):\n", completedCount, len(node.TasklistItems)))
			for _, item := range node.TasklistItems {
				checkbox := "[ ]"
				if item.Completed {
					checkbox = "[x]"
				}
				// Format linked reference if present
				linkedInfo := ""
				if item.LinkedRef != nil {
					linkedRef := formatNodeRef(item.LinkedRef.Owner, item.LinkedRef.Repo, item.LinkedRef.Number, graph.FocusOwner, graph.FocusRepo)
					linkedInfo = fmt.Sprintf(" → %s", linkedRef)
				}
				// Truncate long text
				text := item.Text
				if len(text) > 80 {
					text = text[:77] + "..."
				}
				sb.WriteString(fmt.Sprintf("    %s %s%s\n", checkbox, text, linkedInfo))
			}
		}
	}

	// Edges section - parent/child relationships (sub-issues, closes/fixes)
	sb.WriteString("\nSUB-ISSUES (parent → child)\n")
	sb.WriteString("===========================\n")
	parentChildEdges := make([]GraphEdge, 0)
	relatedEdges := make([]GraphEdge, 0)
	for _, edge := range graph.Edges {
		switch edge.Relation {
		case RelationTypeChild:
			parentChildEdges = append(parentChildEdges, edge)
		case RelationTypeParent:
			// Parent edges: from closes ref, so ref is parent of from
			// Reverse the direction for display: parent → child
			parentChildEdges = append(parentChildEdges, GraphEdge{
				FromOwner:  edge.ToOwner,
				FromRepo:   edge.ToRepo,
				FromNumber: edge.ToNumber,
				ToOwner:    edge.FromOwner,
				ToRepo:     edge.FromRepo,
				ToNumber:   edge.FromNumber,
				Relation:   RelationTypeChild,
			})
		case RelationTypeRelated:
			relatedEdges = append(relatedEdges, edge)
		}
	}

	if len(parentChildEdges) == 0 {
		sb.WriteString("(none)\n")
	} else {
		for _, edge := range parentChildEdges {
			fromRef := formatNodeRef(edge.FromOwner, edge.FromRepo, edge.FromNumber, graph.FocusOwner, graph.FocusRepo)
			toRef := formatNodeRef(edge.ToOwner, edge.ToRepo, edge.ToNumber, graph.FocusOwner, graph.FocusRepo)
			sb.WriteString(fmt.Sprintf("%s → %s\n", fromRef, toRef))
		}
	}

	// Related section (cross-references from timeline, body mentions)
	sb.WriteString("\nCROSS-REFERENCES (mentioned/referenced)\n")
	sb.WriteString("=======================================\n")
	if len(relatedEdges) == 0 {
		sb.WriteString("(none)\n")
	} else {
		// Build a lookup map for nodes
		nodeMap := make(map[string]*GraphNode)
		for i := range graph.Nodes {
			key := nodeKey(graph.Nodes[i].Owner, graph.Nodes[i].Repo, graph.Nodes[i].Number)
			nodeMap[key] = &graph.Nodes[i]
		}

		for _, edge := range relatedEdges {
			fromRef := formatNodeRef(edge.FromOwner, edge.FromRepo, edge.FromNumber, graph.FocusOwner, graph.FocusRepo)
			toRef := formatNodeRef(edge.ToOwner, edge.ToRepo, edge.ToNumber, graph.FocusOwner, graph.FocusRepo)

			// Check if from node is a PR and include its status
			fromKey := nodeKey(edge.FromOwner, edge.FromRepo, edge.FromNumber)
			if fromNode, ok := nodeMap[fromKey]; ok && fromNode.NodeType == NodeTypePR {
				status := fromNode.State
				if fromNode.StateReason != "" && fromNode.StateReason != fromNode.State {
					status = fromNode.StateReason
				}
				sb.WriteString(fmt.Sprintf("%s (%s) ↔ %s\n", fromRef, strings.ToUpper(status), toRef))
			} else {
				sb.WriteString(fmt.Sprintf("%s ↔ %s\n", fromRef, toRef))
			}
		}
	}

	// Crawl summary (verbose mode only)
	if graph.CrawlSummary != "" {
		sb.WriteString("\n")
		sb.WriteString(graph.CrawlSummary)
	}

	return sb.String()
}

// IssueRef contains owner/repo/number for an issue reference
type IssueRef struct {
	Owner  string
	Repo   string
	Number int
}

// IssueGraphQLInfo contains parent issue info fetched via GraphQL
type IssueGraphQLInfo struct {
	Parent *IssueRef // Parent issue (if any)
}

// fetchIssueGraphQLInfo fetches parent issue info via GraphQL
// This is lightweight - only fetches parent, not sub-issues or projects
func fetchIssueGraphQLInfo(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, number int) *IssueGraphQLInfo {
	if gqlClient == nil {
		return nil
	}

	// Lightweight GraphQL query for parent only
	var query struct {
		Repository struct {
			Issue struct {
				// Parent issue (can be cross-repo)
				Parent *struct {
					Number     githubv4.Int
					Repository struct {
						Owner struct {
							Login githubv4.String
						}
						Name githubv4.String
					}
				}
			} `graphql:"issue(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(int32(number)), //nolint:gosec // issue numbers are always small positive integers
	}

	// Execute query with a short timeout
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := gqlClient.Query(queryCtx, &query, vars); err != nil {
		// Silently ignore errors - this info is optional
		return nil
	}

	result := &IssueGraphQLInfo{}

	// Extract parent
	if query.Repository.Issue.Parent != nil {
		result.Parent = &IssueRef{
			Owner:  string(query.Repository.Issue.Parent.Repository.Owner.Login),
			Repo:   string(query.Repository.Issue.Parent.Repository.Name),
			Number: int(query.Repository.Issue.Parent.Number),
		}
	}

	return result
}

// fetchProjectInfo fetches project info for an issue via GraphQL
// This is a separate, heavier query - only use for focus node
func fetchProjectInfo(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, number int) []ProjectInfo {
	if gqlClient == nil {
		return nil
	}

	var query struct {
		Repository struct {
			Issue struct {
				ProjectItems struct {
					Nodes []struct {
						Project struct {
							Title githubv4.String
						}
						FieldValueByName struct {
							SingleSelectValue struct {
								Name githubv4.String
							} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
						} `graphql:"fieldValueByName(name: \"Status\")"`
					}
				} `graphql:"projectItems(first: 10)"`
			} `graphql:"issue(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(int32(number)), //nolint:gosec // issue numbers are always small positive integers
	}

	queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := gqlClient.Query(queryCtx, &query, vars); err != nil {
		return nil
	}

	var projects []ProjectInfo
	for _, node := range query.Repository.Issue.ProjectItems.Nodes {
		title := string(node.Project.Title)
		if title == "" {
			continue
		}
		status := string(node.FieldValueByName.SingleSelectValue.Name)
		projects = append(projects, ProjectInfo{
			ProjectTitle: title,
			Status:       status,
		})
	}

	return projects
}

// fetchPRProjects fetches project info for a PR (PRs don't have parent/sub-issues)
func fetchPRProjects(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, number int) []ProjectInfo {
	if gqlClient == nil {
		return nil
	}

	var query struct {
		Repository struct {
			PullRequest struct {
				ProjectItems struct {
					Nodes []struct {
						Project struct {
							Title githubv4.String
						}
						FieldValueByName struct {
							SingleSelectValue struct {
								Name githubv4.String
							} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
						} `graphql:"fieldValueByName(name: \"Status\")"`
					}
				} `graphql:"projectItems(first: 10)"`
			} `graphql:"pullRequest(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]interface{}{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(int32(number)), //nolint:gosec // issue numbers are always small positive integers
	}

	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := gqlClient.Query(queryCtx, &query, vars); err != nil {
		return nil
	}

	var projects []ProjectInfo
	for _, node := range query.Repository.PullRequest.ProjectItems.Nodes {
		title := string(node.Project.Title)
		if title == "" {
			continue
		}
		status := string(node.FieldValueByName.SingleSelectValue.Name)
		projects = append(projects, ProjectInfo{
			ProjectTitle: title,
			Status:       status,
		})
	}

	return projects
}

// GetIssueGraph creates a tool to get a graph representation of issue/PR relationships
func GetIssueGraph(getClient GetClientFn, getGQLClient GetGQLClientFn, cache *lockdown.RepoAccessCache, t translations.TranslationHelperFunc, flags FeatureFlags) (mcp.Tool, mcp.ToolHandlerFor[map[string]any, any]) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"owner": {
				Type:        "string",
				Description: "Repository owner",
			},
			"repo": {
				Type:        "string",
				Description: "Repository name",
			},
			"issue_number": {
				Type:        "number",
				Description: "Issue or pull request number to build the graph from",
			},
			"focus": {
				Type:        "string",
				Description: "Which node type to focus on: 'provided' (default) uses the specified issue/PR, 'epic' shifts focus to the nearest epic in the hierarchy, 'batch' shifts focus to the nearest batch/parent issue",
				Enum:        []any{"provided", "epic", "batch"},
			},
			"verbose": {
				Type:        "boolean",
				Description: "Include crawl statistics showing how the graph was traversed (nodes fetched, depth reached, repos accessed, etc.)",
			},
		},
		Required: []string{"owner", "repo", "issue_number"},
	}

	return mcp.Tool{
			Name: "issue_graph",
			Description: t("TOOL_ISSUE_GRAPH_DESCRIPTION", `Get a graph representation of issue and pull request relationships, showing the full work hierarchy in one call.

Returns a comprehensive view including:
- Node types: epic (large initiatives), batch (parent issues), task (regular issues), pr (pull requests)
- Full hierarchy: epic → batch → task → PR relationships
- Sub-issues and "closes/fixes" references
- Cross-references and related work
- Status updates extracted from issue bodies and comments
- Open/closed/merged state of all related items

Use focus="epic" to automatically find and focus on the parent epic of any issue.
Use focus="batch" to find the nearest batch/parent issue in the hierarchy.`),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_GRAPH_USER_TITLE", "Get issue relationship graph"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		func(ctx context.Context, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			focusType, err := OptionalParam[string](args, "focus")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if focusType == "" {
				focusType = "provided"
			}
			verbose, err := OptionalParam[bool](args, "verbose")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get GQL client for parent queries (optional, nil is ok)
			var gqlClient *githubv4.Client
			if getGQLClient != nil {
				gqlClient, _ = getGQLClient(ctx) // ignore error, gqlClient will be nil
			}

			// Add timeout to prevent runaway crawling
			crawlCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			// Create crawler and build graph
			crawler := newGraphCrawler(client, gqlClient, cache, flags, owner, repo, issueNumber, verbose)
			if err := crawler.crawl(crawlCtx); err != nil {
				// If timeout, continue with partial results; otherwise fail
				if crawlCtx.Err() != context.DeadlineExceeded {
					return nil, nil, fmt.Errorf("failed to crawl issue graph: %w", err)
				}
			}

			// Refocus if requested
			if focusType != "provided" {
				crawler.focusRequested = focusType
				newOwner, newRepo, newNumber, source := crawler.findBestFocus(focusType)
				if newOwner != owner || newRepo != repo || newNumber != issueNumber {
					crawler.refocusTo(newOwner, newRepo, newNumber, source)
				}
			}

			graph := crawler.buildGraph()

			// Fetch project info for the focus node (optional, best-effort)
			if gqlClient != nil {
				// Determine if focus node is a PR
				focusKey := nodeKey(graph.FocusOwner, graph.FocusRepo, graph.FocusNumber)
				isPR := false
				crawler.mu.RLock()
				if focusNode, exists := crawler.nodes[focusKey]; exists {
					isPR = focusNode.NodeType == NodeTypePR
				}
				crawler.mu.RUnlock()

				// Fetch project info for focus node (separate query, only for focus)
				if isPR {
					graph.FocusProject = fetchPRProjects(ctx, gqlClient, graph.FocusOwner, graph.FocusRepo, graph.FocusNumber)
				} else {
					graph.FocusProject = fetchProjectInfo(ctx, gqlClient, graph.FocusOwner, graph.FocusRepo, graph.FocusNumber)
				}
			}

			// Format for LLM consumption - text format is token-efficient and sufficient
			formattedOutput := formatGraphOutput(graph)

			return utils.NewToolResultText(formattedOutput), nil, nil
		}
}
