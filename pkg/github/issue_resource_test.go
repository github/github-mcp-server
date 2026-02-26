package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_issueResource(t *testing.T) {
	t.Run("missing owner", func(t *testing.T) {
		client := github.NewClient(MockHTTPClientWithHandler(nil))
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue:///repo/issues/1.md"},
		})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "owner is required")
	})

	t.Run("missing repo", func(t *testing.T) {
		client := github.NewClient(MockHTTPClientWithHandler(nil))
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner//issues/1.md"},
		})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "repo is required")
	})

	t.Run("missing issue number", func(t *testing.T) {
		client := github.NewClient(MockHTTPClientWithHandler(nil))
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/.md"},
		})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "issueNumber is required")
	})

	t.Run("invalid issue number", func(t *testing.T) {
		client := github.NewClient(MockHTTPClientWithHandler(nil))
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/abc.md"},
		})
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid issue number")
	})

	t.Run("issue with no comments and no images", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				issue := &github.Issue{
					Number: github.Ptr(42),
					Title:  github.Ptr("Test Issue"),
					Body:   github.Ptr("This is the issue body."),
					State:  github.Ptr("open"),
					User:   &github.User{Login: github.Ptr("testuser")},
					Labels: []*github.Label{{Name: github.Ptr("bug")}, {Name: github.Ptr("help wanted")}},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(issue)
				_, _ = w.Write(data)
			},
			GetReposIssuesCommentsByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			},
		})

		client := github.NewClient(mockClient)
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/42.md"},
		})
		require.NoError(t, err)
		require.Len(t, result.Contents, 1)

		body := result.Contents[0]
		assert.Equal(t, "issue://owner/repo/issues/42.md", body.URI)
		assert.Equal(t, "text/markdown", body.MIMEType)
		assert.Contains(t, body.Text, "title: \"Test Issue\"")
		assert.Contains(t, body.Text, "state: open")
		assert.Contains(t, body.Text, "author: testuser")
		assert.Contains(t, body.Text, "  - bug")
		assert.Contains(t, body.Text, "  - help wanted")
		assert.Contains(t, body.Text, "This is the issue body.")
	})

	t.Run("issue with comments", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				issue := &github.Issue{
					Number: github.Ptr(7),
					Title:  github.Ptr("Feature request"),
					Body:   github.Ptr("Please add this feature."),
					State:  github.Ptr("open"),
					User:   &github.User{Login: github.Ptr("author")},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(issue)
				_, _ = w.Write(data)
			},
			GetReposIssuesCommentsByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				comments := []*github.IssueComment{
					{
						ID:                github.Ptr(int64(101)),
						Body:              github.Ptr("I agree, this is needed."),
						User:              &github.User{Login: github.Ptr("commenter1")},
						AuthorAssociation: github.Ptr("CONTRIBUTOR"),
					},
					{
						ID:                github.Ptr(int64(102)),
						Body:              github.Ptr("Working on it now."),
						User:              &github.User{Login: github.Ptr("maintainer")},
						AuthorAssociation: github.Ptr("MEMBER"),
					},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(comments)
				_, _ = w.Write(data)
			},
		})

		client := github.NewClient(mockClient)
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/7.md"},
		})
		require.NoError(t, err)
		require.Len(t, result.Contents, 1)

		content := result.Contents[0]
		assert.Equal(t, "issue://owner/repo/issues/7.md", content.URI)

		// Check body
		assert.Contains(t, content.Text, "Please add this feature.")

		// Check comment delimiters and metadata
		assert.Contains(t, content.Text, "\n\n---\n\n")
		assert.Contains(t, content.Text, "author: commenter1")
		assert.Contains(t, content.Text, "author_association: CONTRIBUTOR")
		assert.Contains(t, content.Text, "I agree, this is needed.")
		assert.Contains(t, content.Text, "author: maintainer")
		assert.Contains(t, content.Text, "Working on it now.")
	})

	t.Run("issue with markdown images preserved", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				issue := &github.Issue{
					Number: github.Ptr(10),
					Title:  github.Ptr("Bug with screenshot"),
					Body:   github.Ptr("See this:\n![screenshot](https://user-images.githubusercontent.com/123/image.png)\nEnd."),
					State:  github.Ptr("closed"),
					User:   &github.User{Login: github.Ptr("reporter")},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(issue)
				_, _ = w.Write(data)
			},
			GetReposIssuesCommentsByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			},
		})

		client := github.NewClient(mockClient)
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/10.md"},
		})
		require.NoError(t, err)
		require.Len(t, result.Contents, 1)

		// Markdown images should be kept as-is
		assert.Contains(t, result.Contents[0].Text, "![screenshot](https://user-images.githubusercontent.com/123/image.png)")
	})

	t.Run("issue with HTML img tags converted to markdown", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				issue := &github.Issue{
					Number: github.Ptr(11),
					Title:  github.Ptr("HTML images"),
					Body:   github.Ptr(`See: <img width="468" height="230" alt="Image" src="https://user-images.githubusercontent.com/123/screenshot.png" />`),
					State:  github.Ptr("open"),
					User:   &github.User{Login: github.Ptr("reporter")},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(issue)
				_, _ = w.Write(data)
			},
			GetReposIssuesCommentsByOwnerByRepoByIssueNumber: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			},
		})

		client := github.NewClient(mockClient)
		deps := BaseDeps{Client: client}
		ctx := ContextWithDeps(context.Background(), deps)
		handler := IssueResourceHandler()

		result, err := handler(ctx, &mcp.ReadResourceRequest{
			Params: &mcp.ReadResourceParams{URI: "issue://owner/repo/issues/11.md"},
		})
		require.NoError(t, err)
		require.Len(t, result.Contents, 1)

		content := result.Contents[0]
		// HTML img should be converted to markdown with original URL
		assert.Contains(t, content.Text, "![Image](https://user-images.githubusercontent.com/123/screenshot.png)")
		assert.NotContains(t, content.Text, "<img")
	})
}

func Test_convertHTMLImagesToMarkdown(t *testing.T) {
	t.Run("no images", func(t *testing.T) {
		body := "Just some text with no images."
		assert.Equal(t, body, convertHTMLImagesToMarkdown(body))
	})

	t.Run("markdown images unchanged", func(t *testing.T) {
		body := "![alt](https://example.com/image.png)"
		assert.Equal(t, body, convertHTMLImagesToMarkdown(body))
	})

	t.Run("html img with alt", func(t *testing.T) {
		body := `<img width="468" alt="Screenshot" src="https://example.com/img.png" />`
		expected := "![Screenshot](https://example.com/img.png)"
		assert.Equal(t, expected, convertHTMLImagesToMarkdown(body))
	})

	t.Run("html img without alt", func(t *testing.T) {
		body := `<img src="https://example.com/img.png" />`
		expected := "![](https://example.com/img.png)"
		assert.Equal(t, expected, convertHTMLImagesToMarkdown(body))
	})

	t.Run("self-closing without space", func(t *testing.T) {
		body := `<img src="https://example.com/img.png"/>`
		expected := "![](https://example.com/img.png)"
		assert.Equal(t, expected, convertHTMLImagesToMarkdown(body))
	})
}

func Test_buildIssueFrontmatter(t *testing.T) {
	issue := &github.Issue{
		Title: github.Ptr("My Issue"),
		State: github.Ptr("open"),
		User:  &github.User{Login: github.Ptr("testuser")},
		Labels: []*github.Label{
			{Name: github.Ptr("bug")},
			{Name: github.Ptr("priority")},
		},
	}

	fm := buildIssueFrontmatter(issue)
	assert.Contains(t, fm, "---\n")
	assert.Contains(t, fm, `title: "My Issue"`)
	assert.Contains(t, fm, "state: open")
	assert.Contains(t, fm, "author: testuser")
	assert.Contains(t, fm, "  - bug")
	assert.Contains(t, fm, "  - priority")
}

func Test_buildCommentFrontmatter(t *testing.T) {
	comment := &github.IssueComment{
		User:              &github.User{Login: github.Ptr("commenter")},
		AuthorAssociation: github.Ptr("MEMBER"),
	}

	fm := buildCommentFrontmatter(comment)
	assert.Contains(t, fm, "---\n")
	assert.Contains(t, fm, "author: commenter")
	assert.Contains(t, fm, "author_association: MEMBER")
}

func TestIssueResourceCompletionHandler(t *testing.T) {
	t.Run("non-resource completion returns nil", func(t *testing.T) {
		getClient := func(_ context.Context) (*github.Client, error) {
			return &github.Client{}, nil
		}
		handler := IssueResourceCompletionHandler(getClient)
		result, err := handler(t.Context(), &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Ref: &mcp.CompleteReference{Type: "something-else"},
			},
		})
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("unknown argument returns error", func(t *testing.T) {
		getClient := func(_ context.Context) (*github.Client, error) {
			return &github.Client{}, nil
		}
		handler := IssueResourceCompletionHandler(getClient)
		_, err := handler(t.Context(), &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Ref:     &mcp.CompleteReference{Type: "ref/resource"},
				Context: &mcp.CompleteContext{},
				Argument: mcp.CompleteParamsArgument{
					Name:  "unknown_arg",
					Value: "test",
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no resolver for argument")
	})

	t.Run("issueNumber completion", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /search/issues": func(w http.ResponseWriter, _ *http.Request) {
				result := &github.IssuesSearchResult{
					Issues: []*github.Issue{
						{Number: github.Ptr(42)},
						{Number: github.Ptr(43)},
						{Number: github.Ptr(100)},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(result)
				_, _ = w.Write(data)
			},
		})
		client := github.NewClient(mockClient)
		getClient := func(_ context.Context) (*github.Client, error) { return client, nil }
		handler := IssueResourceCompletionHandler(getClient)

		result, err := handler(t.Context(), &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Ref: &mcp.CompleteReference{Type: "ref/resource"},
				Context: &mcp.CompleteContext{
					Arguments: map[string]string{"owner": "testowner", "repo": "testrepo"},
				},
				Argument: mcp.CompleteParamsArgument{
					Name:  "issueNumber",
					Value: "4",
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Contains(t, result.Completion.Values, "42")
		assert.Contains(t, result.Completion.Values, "43")
	})

	t.Run("issueNumber requires owner and repo", func(t *testing.T) {
		getClient := func(_ context.Context) (*github.Client, error) {
			return github.NewClient(nil), nil
		}
		handler := IssueResourceCompletionHandler(getClient)
		_, err := handler(t.Context(), &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Ref:     &mcp.CompleteReference{Type: "ref/resource"},
				Context: &mcp.CompleteContext{},
				Argument: mcp.CompleteParamsArgument{
					Name:  "issueNumber",
					Value: "",
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "owner or repo not specified")
	})
}

func TestCompletionsHandler_IssueResource(t *testing.T) {
	t.Run("routes issue:// to issue completion handler", func(t *testing.T) {
		mockClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /search/issues": func(w http.ResponseWriter, _ *http.Request) {
				result := &github.IssuesSearchResult{
					Issues: []*github.Issue{
						{Number: github.Ptr(1)},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				data, _ := json.Marshal(result)
				_, _ = w.Write(data)
			},
		})
		client := github.NewClient(mockClient)
		getClient := func(_ context.Context) (*github.Client, error) { return client, nil }

		handler := CompletionsHandler(getClient)
		result, err := handler(t.Context(), &mcp.CompleteRequest{
			Params: &mcp.CompleteParams{
				Ref: &mcp.CompleteReference{
					Type: "ref/resource",
					URI:  "issue://{owner}/{repo}/issues/{issueNumber}.md",
				},
				Context: &mcp.CompleteContext{
					Arguments: map[string]string{"owner": "o", "repo": "r"},
				},
				Argument: mcp.CompleteParamsArgument{
					Name:  "issueNumber",
					Value: "",
				},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, 1, result.Completion.Total)
	})
}
