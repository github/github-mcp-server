package github

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// MaxGraphDepth is the maximum depth to crawl for related issues
	MaxGraphDepth = 3
	// MaxConcurrentFetches is the maximum number of concurrent API calls
	MaxConcurrentFetches = 5
)

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
	Owner       string   `json:"owner"`
	Repo        string   `json:"repo"`
	Number      int      `json:"number"`
	NodeType    NodeType `json:"nodeType"`
	State       string   `json:"state"`
	Title       string   `json:"title"`
	BodyPreview string   `json:"bodyPreview"`
	Depth       int      `json:"depth"`
	IsFocus     bool     `json:"isFocus"`
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
	FocusOwner  string      `json:"focusOwner"`
	FocusRepo   string      `json:"focusRepo"`
	FocusNumber int         `json:"focusNumber"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
	Summary     string      `json:"summary"`
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

// Regular expressions for extracting issue references
var (
	// Matches #123 style references (same repo)
	sameRepoRefRegex = regexp.MustCompile(`(?:^|[^\w])#(\d+)`)
	// Matches owner/repo#123 style references (cross-repo)
	crossRepoRefRegex = regexp.MustCompile(`([a-zA-Z0-9](?:[a-zA-Z0-9._-]*[a-zA-Z0-9])?)/([a-zA-Z0-9._-]+)#(\d+)`)
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
func classifyNode(isPR bool, labels []string, title string, hasSubIssues bool) NodeType {
	if isPR {
		return NodeTypePR
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

// graphCrawler manages the concurrent crawling of the issue graph
type graphCrawler struct {
	client           *github.Client
	cache            *lockdown.RepoAccessCache
	flags            FeatureFlags
	focusOwner       string
	focusRepo        string
	focusNumber      int
	nodes            map[string]*GraphNode
	edges            []GraphEdge
	parentMap        map[string]string // maps child -> parent
	inaccessibleRepo map[string]bool   // repos we don't have access to
	mu               sync.RWMutex
	sem              chan struct{} // semaphore for concurrency control
}

func newGraphCrawler(client *github.Client, cache *lockdown.RepoAccessCache, flags FeatureFlags, owner, repo string, number int) *graphCrawler {
	return &graphCrawler{
		client:           client,
		cache:            cache,
		flags:            flags,
		focusOwner:       owner,
		focusRepo:        repo,
		focusNumber:      number,
		nodes:            make(map[string]*GraphNode),
		edges:            make([]GraphEdge, 0),
		parentMap:        make(map[string]string),
		inaccessibleRepo: make(map[string]bool),
		sem:              make(chan struct{}, MaxConcurrentFetches),
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

	// Fetch issue/PR details
	issue, resp, err := gc.client.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		if resp != nil {
			_ = resp.Body.Close()
			// Mark repo as inaccessible for 403 (forbidden) or 404 (not found for entire repo)
			if resp.StatusCode == 403 || resp.StatusCode == 404 {
				// Check if it's a repo-level 404 vs issue-level 404
				// For simplicity, we'll just skip this node
				if resp.StatusCode == 403 {
					gc.markRepoInaccessible(owner, repo)
				}
				return nil, nil, nil
			}
		}
		return nil, nil, fmt.Errorf("failed to get issue %s: %w", key, err)
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

	// Determine node type
	nodeType := classifyNode(isPR, labels, issue.GetTitle(), hasSubIssues)

	// Get state
	state := issue.GetState()

	// Create node
	node := &GraphNode{
		Owner:       owner,
		Repo:        repo,
		Number:      number,
		NodeType:    nodeType,
		State:       state,
		Title:       issue.GetTitle(),
		BodyPreview: sanitizeBodyForGraph(issue.GetBody(), getBodyLinesForDepth(depth), getMaxLineLenForDepth(depth)),
		Depth:       depth,
		IsFocus:     strings.EqualFold(owner, gc.focusOwner) && strings.EqualFold(repo, gc.focusRepo) && number == gc.focusNumber,
	}

	// Add to graph
	gc.mu.Lock()
	gc.nodes[key] = node
	gc.mu.Unlock()

	return node, issue, nil
}

// crawl performs a BFS crawl from the focus node
func (gc *graphCrawler) crawl(ctx context.Context) error {
	type crawlItem struct {
		owner  string
		repo   string
		number int
		depth  int
	}

	// Initialize with focus node
	queue := []crawlItem{{gc.focusOwner, gc.focusRepo, gc.focusNumber, 0}}

	for len(queue) > 0 {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Process current level
		current := queue[0]
		queue = queue[1:]

		// Skip if beyond max depth (shouldn't happen, but defensive check)
		if current.depth > MaxGraphDepth {
			continue
		}

		// Check if repo is inaccessible
		if gc.isRepoInaccessible(current.owner, current.repo) {
			continue
		}

		// Check if already visited
		key := nodeKey(current.owner, current.repo, current.number)
		gc.mu.RLock()
		_, visited := gc.nodes[key]
		gc.mu.RUnlock()
		if visited {
			continue
		}

		// Fetch the node and the raw issue data
		node, issue, err := gc.fetchNode(ctx, current.owner, current.repo, current.number, current.depth)
		if err != nil {
			// Log error but continue crawling
			continue
		}
		if node == nil {
			continue
		}

		// Don't crawl further from nodes at max depth (they are leaf nodes for crawling)
		// Also skip if we didn't get issue data (already visited node)
		if current.depth == MaxGraphDepth || issue == nil {
			continue
		}

		bodyRefs := extractIssueReferences(issue.GetBody(), current.owner, current.repo)

		// Process references and add edges
		for _, ref := range bodyRefs {
			// Skip if repo is known to be inaccessible
			if gc.isRepoInaccessible(ref.Owner, ref.Repo) {
				continue
			}

			refKey := nodeKey(ref.Owner, ref.Repo, ref.Number)

			// Skip self-references
			if refKey == key {
				continue
			}

			// Determine relationship
			relType := RelationTypeRelated
			if ref.IsParent {
				// This node closes ref, so ref is the parent
				relType = RelationTypeParent
				gc.mu.Lock()
				gc.parentMap[key] = refKey
				gc.mu.Unlock()
			}

			// Add edge
			gc.mu.Lock()
			gc.edges = append(gc.edges, GraphEdge{
				FromOwner:  current.owner,
				FromRepo:   current.repo,
				FromNumber: current.number,
				ToOwner:    ref.Owner,
				ToRepo:     ref.Repo,
				ToNumber:   ref.Number,
				Relation:   relType,
			})
			gc.mu.Unlock()

			// Add to queue for further crawling
			queue = append(queue, crawlItem{ref.Owner, ref.Repo, ref.Number, current.depth + 1})
		}

		// Get sub-issues if this is an issue (not PR)
		if !issue.IsPullRequest() {
			subIssues, subResp, err := gc.client.SubIssue.ListByIssue(ctx, current.owner, current.repo, int64(current.number), &github.IssueListOptions{
				ListOptions: github.ListOptions{PerPage: 100},
			})
			if err == nil {
				for _, subIssue := range subIssues {
					// SubIssue is a type alias for Issue, access Number field directly
					if subIssue.Number == nil {
						continue
					}
					subNumber := *subIssue.Number
					subKey := nodeKey(current.owner, current.repo, subNumber)

					// This node is parent of sub-issue
					gc.mu.Lock()
					gc.parentMap[subKey] = key
					gc.edges = append(gc.edges, GraphEdge{
						FromOwner:  current.owner,
						FromRepo:   current.repo,
						FromNumber: current.number,
						ToOwner:    current.owner,
						ToRepo:     current.repo,
						ToNumber:   subNumber,
						Relation:   RelationTypeChild,
					})
					gc.mu.Unlock()

					// Add to queue
					queue = append(queue, crawlItem{current.owner, current.repo, subNumber, current.depth + 1})
				}
			}
			if subResp != nil {
				_ = subResp.Body.Close()
			}
		}

		// Get cross-referenced issues from timeline
		timelineEvents, timelineResp, err := gc.client.Issues.ListIssueTimeline(ctx, current.owner, current.repo, current.number, &github.ListOptions{
			PerPage: 100,
		})
		if err == nil {
			for _, event := range timelineEvents {
				// Only process cross-referenced events
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

				// Parse repository info from the source issue URL
				refOwner, refRepo := current.owner, current.repo
				if sourceIssue.RepositoryURL != nil {
					// URL format: https://api.github.com/repos/{owner}/{repo}
					parts := strings.Split(*sourceIssue.RepositoryURL, "/")
					if len(parts) >= 2 {
						refOwner = parts[len(parts)-2]
						refRepo = parts[len(parts)-1]
					}
				}

				// Skip if repo is known to be inaccessible
				if gc.isRepoInaccessible(refOwner, refRepo) {
					continue
				}

				refNumber := *sourceIssue.Number
				refKey := nodeKey(refOwner, refRepo, refNumber)

				// Skip self-references
				if refKey == key {
					continue
				}

				// Add edge (the source issue mentioned this issue)
				gc.mu.Lock()
				gc.edges = append(gc.edges, GraphEdge{
					FromOwner:  refOwner,
					FromRepo:   refRepo,
					FromNumber: refNumber,
					ToOwner:    current.owner,
					ToRepo:     current.repo,
					ToNumber:   current.number,
					Relation:   RelationTypeRelated,
				})
				gc.mu.Unlock()

				// Add to queue for further crawling
				queue = append(queue, crawlItem{refOwner, refRepo, refNumber, current.depth + 1})
			}
		}
		if timelineResp != nil {
			_ = timelineResp.Body.Close()
		}
	}

	return nil
}

// edgeKey creates a unique key for an edge to enable deduplication
func edgeKey(e GraphEdge) string {
	return fmt.Sprintf("%s/%s#%d->%s/%s#%d:%s",
		strings.ToLower(e.FromOwner), strings.ToLower(e.FromRepo), e.FromNumber,
		strings.ToLower(e.ToOwner), strings.ToLower(e.ToRepo), e.ToNumber,
		e.Relation)
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

	return &IssueGraph{
		FocusOwner:  gc.focusOwner,
		FocusRepo:   gc.focusRepo,
		FocusNumber: gc.focusNumber,
		Nodes:       nodes,
		Edges:       uniqueEdges,
		Summary:     gc.generateSummary(),
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

	// Focus node info
	sb.WriteString(fmt.Sprintf("Focus: #%d (%s) \"%s\"\n",
		gc.focusNumber, focusNode.NodeType, focusNode.Title))
	sb.WriteString(fmt.Sprintf("State: %s\n", focusNode.State))

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

// findAncestors finds all ancestor nodes (parents) of a given node
func (gc *graphCrawler) findAncestors(key string) []string {
	ancestors := make([]string, 0)
	visited := make(map[string]bool)
	current := key

	for {
		parent, exists := gc.parentMap[current]
		if !exists || visited[parent] {
			break
		}
		visited[parent] = true
		ancestors = append(ancestors, parent)
		current = parent
	}

	return ancestors
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
		sb.WriteString(fmt.Sprintf("%s|%s|%s|%s%s\n",
			nodeRef, node.NodeType, node.State, node.Title, focusMarker))
		if node.BodyPreview != "" {
			sb.WriteString(fmt.Sprintf("  Preview: %s\n", node.BodyPreview))
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
		for _, edge := range relatedEdges {
			fromRef := formatNodeRef(edge.FromOwner, edge.FromRepo, edge.FromNumber, graph.FocusOwner, graph.FocusRepo)
			toRef := formatNodeRef(edge.ToOwner, edge.ToRepo, edge.ToNumber, graph.FocusOwner, graph.FocusRepo)
			sb.WriteString(fmt.Sprintf("%s ↔ %s\n", fromRef, toRef))
		}
	}

	return sb.String()
}

// GetIssueGraph creates a tool to get a graph representation of issue/PR relationships
func GetIssueGraph(getClient GetClientFn, cache *lockdown.RepoAccessCache, t translations.TranslationHelperFunc, flags FeatureFlags) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("issue_graph",
			mcp.WithDescription(t("TOOL_ISSUE_GRAPH_DESCRIPTION", `Get a graph representation of an issue or pull request and its related issues/PRs.

This tool helps understand the relationships between issues and PRs in a repository, especially useful for:
- Understanding the scope of work for an issue or PR
- Planning implementation for a task that's part of a larger epic
- Identifying blockers or dependencies
- Finding related work that might conflict or overlap
- Understanding why a piece of work exists (tracing to parent epic)

The graph shows:
- Node types: epic (large initiatives), batch (parent issues), task (regular issues), pr (pull requests)
- Parent/child relationships from sub-issues and "closes/fixes" references
- Related issues mentioned in bodies

Call this tool early when working on an issue to gather appropriate context about the work hierarchy.`)),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_ISSUE_GRAPH_USER_TITLE", "Get issue relationship graph"),
				ReadOnlyHint: ToBoolPtr(true),
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("issue_number",
				mcp.Required(),
				mcp.Description("Issue or pull request number to build the graph from"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := RequiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := RequiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			issueNumber, err := RequiredInt(request, "issue_number")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Add timeout to prevent runaway crawling
			crawlCtx, cancel := context.WithTimeout(ctx, 7*time.Second)
			defer cancel()

			// Create crawler and build graph
			crawler := newGraphCrawler(client, cache, flags, owner, repo, issueNumber)
			if err := crawler.crawl(crawlCtx); err != nil {
				// If timeout, continue with partial results; otherwise fail
				if crawlCtx.Err() != context.DeadlineExceeded {
					return nil, fmt.Errorf("failed to crawl issue graph: %w", err)
				}
			}

			graph := crawler.buildGraph()

			// Format for LLM consumption - text format is token-efficient and sufficient
			formattedOutput := formatGraphOutput(graph)

			return mcp.NewToolResultText(formattedOutput), nil
		}
}
