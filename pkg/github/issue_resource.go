package github

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/octicons"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

var issueResourceURITemplate = uritemplate.MustNew("issue://{owner}/{repo}/issues/{issueNumber}.md")

// htmlImagePattern matches HTML img tags: <img ... src="url" ... />
var htmlImagePattern = regexp.MustCompile(`<img\b([^>]*)\bsrc=["']([^"']+)["']([^>]*)/??>`)

// htmlAltPattern extracts the alt attribute value from an img tag fragment.
var htmlAltPattern = regexp.MustCompile(`\balt=["']([^"']*)["']`)

// GetIssueResourceContent defines the resource template for reading issue content.
func GetIssueResourceContent(t translations.TranslationHelperFunc) inventory.ServerResourceTemplate {
	return inventory.NewServerResourceTemplate(
		ToolsetMetadataIssues,
		mcp.ResourceTemplate{
			Name:        "issue_content",
			URITemplate: issueResourceURITemplate.Raw(),
			Description: t("RESOURCE_ISSUE_CONTENT_DESCRIPTION", "Issue content with comments and embedded images as a single markdown document"),
			Icons:       octicons.Icons("issue-opened"),
		},
		issueResourceHandlerFunc(),
	)
}

func issueResourceHandlerFunc() inventory.ResourceHandlerFunc {
	return func(_ any) mcp.ResourceHandler {
		return IssueResourceHandler()
	}
}

// IssueResourceHandler returns a handler for issue resource requests.
func IssueResourceHandler() mcp.ResourceHandler {
	return func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		deps := MustDepsFromContext(ctx)

		uriValues := issueResourceURITemplate.Match(request.Params.URI)
		if uriValues == nil {
			return nil, fmt.Errorf("failed to match URI: %s", request.Params.URI)
		}

		owner := uriValues.Get("owner").String()
		repo := uriValues.Get("repo").String()
		issueNumberStr := uriValues.Get("issueNumber").String()

		if owner == "" {
			return nil, fmt.Errorf("owner is required")
		}
		if repo == "" {
			return nil, fmt.Errorf("repo is required")
		}
		if issueNumberStr == "" {
			return nil, fmt.Errorf("issueNumber is required")
		}

		issueNumber, err := strconv.Atoi(issueNumberStr)
		if err != nil {
			return nil, fmt.Errorf("invalid issue number: %w", err)
		}

		client, err := deps.GetClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get GitHub client: %w", err)
		}

		// Fetch the issue
		issue, _, err := client.Issues.Get(ctx, owner, repo, issueNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get issue: %w", err)
		}

		// Build unified markdown document
		var doc strings.Builder

		// Issue body with frontmatter
		body := sanitize.Sanitize(issue.GetBody())
		frontmatter := buildIssueFrontmatter(issue)
		doc.WriteString(frontmatter)
		doc.WriteString(convertHTMLImagesToMarkdown(body))

		// Fetch all comments
		comments, err := fetchAllIssueComments(ctx, client, owner, repo, issueNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to get issue comments: %w", err)
		}

		for _, comment := range comments {
			doc.WriteString("\n\n---\n\n")
			commentBody := sanitize.Sanitize(comment.GetBody())
			commentFrontmatter := buildCommentFrontmatter(comment)
			doc.WriteString(commentFrontmatter)
			doc.WriteString(convertHTMLImagesToMarkdown(commentBody))
		}

		resourceURI := fmt.Sprintf("issue://%s/%s/issues/%d.md", owner, repo, issueNumber)
		contents := []*mcp.ResourceContents{
			{
				URI:      resourceURI,
				MIMEType: "text/markdown",
				Text:     doc.String(),
			},
		}

		return &mcp.ReadResourceResult{Contents: contents}, nil
	}
}

func buildIssueFrontmatter(issue *github.Issue) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: %q\n", sanitize.Sanitize(issue.GetTitle())))
	b.WriteString(fmt.Sprintf("state: %s\n", issue.GetState()))
	if user := issue.GetUser(); user != nil {
		b.WriteString(fmt.Sprintf("author: %s\n", user.GetLogin()))
	}
	if issue.CreatedAt != nil {
		b.WriteString(fmt.Sprintf("created_at: %s\n", issue.CreatedAt.Format("2006-01-02T15:04:05Z")))
	}
	if len(issue.Labels) > 0 {
		b.WriteString("labels:\n")
		for _, label := range issue.Labels {
			if label != nil {
				b.WriteString(fmt.Sprintf("  - %s\n", label.GetName()))
			}
		}
	}
	if issue.GetMilestone() != nil {
		b.WriteString(fmt.Sprintf("milestone: %s\n", issue.GetMilestone().GetTitle()))
	}
	b.WriteString("---\n\n")
	return b.String()
}

func buildCommentFrontmatter(comment *github.IssueComment) string {
	var b strings.Builder
	b.WriteString("---\n")
	if user := comment.GetUser(); user != nil {
		b.WriteString(fmt.Sprintf("author: %s\n", user.GetLogin()))
	}
	b.WriteString(fmt.Sprintf("author_association: %s\n", comment.GetAuthorAssociation()))
	if comment.CreatedAt != nil {
		b.WriteString(fmt.Sprintf("created_at: %s\n", comment.CreatedAt.Format("2006-01-02T15:04:05Z")))
	}
	if comment.UpdatedAt != nil {
		b.WriteString(fmt.Sprintf("updated_at: %s\n", comment.UpdatedAt.Format("2006-01-02T15:04:05Z")))
	}
	b.WriteString("---\n\n")
	return b.String()
}

// convertHTMLImagesToMarkdown converts HTML <img> tags to markdown image syntax.
func convertHTMLImagesToMarkdown(body string) string {
	return htmlImagePattern.ReplaceAllStringFunc(body, func(match string) string {
		submatches := htmlImagePattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return match
		}
		imageURL := submatches[2]
		alt := ""
		attrs := submatches[1] + submatches[3]
		if altMatch := htmlAltPattern.FindStringSubmatch(attrs); len(altMatch) >= 2 {
			alt = altMatch[1]
		}
		return fmt.Sprintf("![%s](%s)", alt, imageURL)
	})
}

func fetchAllIssueComments(ctx context.Context, client *github.Client, owner, repo string, issueNumber int) ([]*github.IssueComment, error) {
	var allComments []*github.IssueComment
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
		if err != nil {
			return nil, err
		}
		_ = resp.Body.Close()
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allComments, nil
}

// IssueResourceCompletionHandler returns a completion handler for issue resource URI templates.
func IssueResourceCompletionHandler(getClient GetClientFn) func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	return func(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		if req.Params.Ref.Type != "ref/resource" {
			return nil, nil
		}

		argName := req.Params.Argument.Name
		argValue := req.Params.Argument.Value
		var resolved map[string]string
		if req.Params.Context != nil && req.Params.Context.Arguments != nil {
			resolved = req.Params.Context.Arguments
		} else {
			resolved = map[string]string{}
		}

		client, err := getClient(ctx)
		if err != nil {
			return nil, err
		}

		// Reuse owner and repo resolvers from repository resource completions
		resolvers := map[string]CompleteHandler{
			"owner":       completeOwner,
			"repo":        completeRepo,
			"issueNumber": completeIssueNumber,
		}

		resolver, ok := resolvers[argName]
		if !ok {
			return nil, fmt.Errorf("no resolver for argument: %s", argName)
		}

		values, err := resolver(ctx, client, resolved, argValue)
		if err != nil {
			return nil, err
		}
		if len(values) > 100 {
			values = values[:100]
		}

		return &mcp.CompleteResult{
			Completion: mcp.CompletionResultDetails{
				Values:  values,
				Total:   len(values),
				HasMore: false,
			},
		}, nil
	}
}

func completeIssueNumber(ctx context.Context, client *github.Client, resolved map[string]string, argValue string) ([]string, error) {
	owner := resolved["owner"]
	repo := resolved["repo"]
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("owner or repo not specified")
	}

	issues, _, err := client.Search.Issues(ctx, fmt.Sprintf("repo:%s/%s is:issue", owner, repo), &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	})
	if err != nil {
		return nil, err
	}

	var values []string
	for _, issue := range issues.Issues {
		num := fmt.Sprintf("%d", issue.GetNumber())
		if argValue == "" || strings.HasPrefix(num, argValue) {
			values = append(values, num)
		}
	}
	if len(values) > 100 {
		values = values[:100]
	}
	return values, nil
}
