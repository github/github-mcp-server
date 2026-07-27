package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/githubv4mock"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const baselineUnsafeText = "<script>alert(1)</script>keep\u200B ```go onclick=alert(1)\nfmt.Println(\"x\")\n```"

func TestSanitizationIssueOutputPaths(t *testing.T) {
	t.Run("rest get issue sanitizes title and body", func(t *testing.T) {
		mockIssue := &gogithub.Issue{
			Number:  gogithub.Ptr(42),
			Title:   gogithub.Ptr(baselineUnsafeText),
			Body:    gogithub.Ptr(baselineUnsafeText),
			State:   gogithub.Ptr("open"),
			HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/42"),
			User:    &gogithub.User{Login: gogithub.Ptr("author")},
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposIssuesByOwnerByRepoByIssueNumber: mockResponse(t, http.StatusOK, mockIssue),
		}))
		deps := BaseDeps{
			Client:          client,
			GQLClient:       defaultGQLClient,
			RepoAccessCache: stubRepoAccessCache(nil, 15*time.Minute),
			Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
		}
		serverTool := IssueRead(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"method":       "get",
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(42),
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
		var issue MinimalIssue
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &issue))
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue.Title)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue.Body)
		assert.Equal(t, baselineUnsafeText, mockIssue.GetTitle())
		assert.Equal(t, baselineUnsafeText, mockIssue.GetBody())
	})

	t.Run("rest search issue sanitizes embedded display fields without mutating source object", func(t *testing.T) {
		sourceIssue := &gogithub.Issue{
			Number: gogithub.Ptr(42),
			Title:  gogithub.Ptr(baselineUnsafeText),
			Body:   gogithub.Ptr(baselineUnsafeText),
			State:  gogithub.Ptr("open"),
			Labels: []*gogithub.Label{{
				Name:        gogithub.Ptr(baselineUnsafeText),
				Description: gogithub.Ptr(baselineUnsafeText),
			}},
			Milestone: &gogithub.Milestone{
				Title:       gogithub.Ptr(baselineUnsafeText),
				Description: gogithub.Ptr(baselineUnsafeText),
			},
		}
		searchHit := SearchIssueResult{Issue: sourceIssue}

		raw, err := json.Marshal(searchHit)
		require.NoError(t, err)
		var issue map[string]any
		require.NoError(t, json.Unmarshal(raw, &issue))
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue["title"])
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue["body"])
		labels := issue["labels"].([]any)
		label := labels[0].(map[string]any)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), label["name"])
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), label["description"])
		milestone := issue["milestone"].(map[string]any)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), milestone["title"])
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), milestone["description"])
		assert.Equal(t, baselineUnsafeText, sourceIssue.GetTitle())
		assert.Equal(t, baselineUnsafeText, sourceIssue.GetBody())
		assert.Equal(t, baselineUnsafeText, sourceIssue.Labels[0].GetName())
		assert.Equal(t, baselineUnsafeText, sourceIssue.Milestone.GetTitle())
	})

	t.Run("graphql issue fragment conversion sanitizes title and body", func(t *testing.T) {
		now := githubv4.DateTime{Time: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)}
		fragment := IssueFragment{
			Number:    githubv4.Int(42),
			Title:     githubv4.String(baselineUnsafeText),
			Body:      githubv4.String(baselineUnsafeText),
			State:     githubv4.String("OPEN"),
			CreatedAt: now,
			UpdatedAt: now,
		}
		fragment.Author.Login = githubv4.String("author")
		fragment.Labels.Nodes = append(fragment.Labels.Nodes, struct {
			Name        githubv4.String
			ID          githubv4.String
			Description githubv4.String
		}{Name: githubv4.String(baselineUnsafeText)})

		issue := fragmentToMinimalIssue(fragment)

		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue.Title)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), issue.Body)
		assert.Equal(t, []string{sanitize.Sanitize(baselineUnsafeText)}, issue.Labels)
	})
}

func TestSanitizationPullRequestOutputPaths(t *testing.T) {
	t.Run("rest get pull request sanitizes title and body without mutating source object", func(t *testing.T) {
		mockPR := &gogithub.PullRequest{
			Number:  gogithub.Ptr(42),
			Title:   gogithub.Ptr(baselineUnsafeText),
			Body:    gogithub.Ptr(baselineUnsafeText),
			State:   gogithub.Ptr("open"),
			HTMLURL: gogithub.Ptr("https://github.com/owner/repo/pull/42"),
			User:    &gogithub.User{Login: gogithub.Ptr("author")},
			Labels: []*gogithub.Label{{
				Name: gogithub.Ptr(baselineUnsafeText),
			}},
			Milestone: &gogithub.Milestone{
				Title: gogithub.Ptr(baselineUnsafeText),
			},
			Head: &gogithub.PullRequestBranch{
				Ref: gogithub.Ptr("feature"),
				SHA: gogithub.Ptr("abc123"),
				Repo: &gogithub.Repository{
					FullName:    gogithub.Ptr("owner/repo"),
					Description: gogithub.Ptr(baselineUnsafeText),
				},
			},
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposPullsByOwnerByRepoByPullNumber: mockResponse(t, http.StatusOK, mockPR),
		}))
		deps := BaseDeps{
			Client:          client,
			GQLClient:       githubv4.NewClient(githubv4mock.NewMockedHTTPClient()),
			RepoAccessCache: stubRepoAccessCache(nil, 5*time.Minute),
			Flags:           stubFeatureFlags(map[string]bool{"lockdown-mode": false}),
		}
		serverTool := PullRequestRead(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"method":     "get",
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
		var pr MinimalPullRequest
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &pr))
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), pr.Title)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), pr.Body)
		assert.Equal(t, []string{sanitize.Sanitize(baselineUnsafeText)}, pr.Labels)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), pr.Milestone)
		require.NotNil(t, pr.Head)
		require.NotNil(t, pr.Head.Repo)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), pr.Head.Repo.Description)
		assert.Equal(t, baselineUnsafeText, mockPR.GetTitle())
		assert.Equal(t, baselineUnsafeText, mockPR.GetBody())
		assert.Equal(t, baselineUnsafeText, mockPR.Labels[0].GetName())
		assert.Equal(t, baselineUnsafeText, mockPR.Milestone.GetTitle())
		assert.Equal(t, baselineUnsafeText, mockPR.Head.Repo.GetDescription())
	})

	t.Run("rest list pull requests sanitizes title and body without mutating source objects", func(t *testing.T) {
		mockPRs := []*gogithub.PullRequest{
			{
				Number:  gogithub.Ptr(42),
				Title:   gogithub.Ptr(baselineUnsafeText),
				Body:    gogithub.Ptr(baselineUnsafeText),
				State:   gogithub.Ptr("open"),
				HTMLURL: gogithub.Ptr("https://github.com/owner/repo/pull/42"),
			},
			{
				Number:  gogithub.Ptr(43),
				Title:   gogithub.Ptr("safe title"),
				Body:    gogithub.Ptr("safe body"),
				State:   gogithub.Ptr("closed"),
				HTMLURL: gogithub.Ptr("https://github.com/owner/repo/pull/43"),
			},
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposPullsByOwnerByRepo: expectQueryParams(t, map[string]string{
				"state":     "all",
				"sort":      "created",
				"direction": "desc",
				"per_page":  "30",
				"page":      "1",
			}).andThen(mockResponse(t, http.StatusOK, mockPRs)),
		}))
		deps := BaseDeps{Client: client}
		serverTool := ListPullRequests(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner":     "owner",
				"repo":      "repo",
				"state":     "all",
				"sort":      "created",
				"direction": "desc",
				"perPage":   float64(30),
				"page":      float64(1),
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
		var prs []MinimalPullRequest
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &prs))
		require.Len(t, prs, 2)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), prs[0].Title)
		assert.Equal(t, sanitize.Sanitize(baselineUnsafeText), prs[0].Body)
		assert.Equal(t, "safe title", prs[1].Title)
		assert.Equal(t, "safe body", prs[1].Body)
		assert.Equal(t, baselineUnsafeText, mockPRs[0].GetTitle())
		assert.Equal(t, baselineUnsafeText, mockPRs[0].GetBody())
	})
}

func TestSanitizationCollaborationTextPaths(t *testing.T) {
	sanitizedUnsafeText := sanitize.Sanitize(baselineUnsafeText)

	comment := convertToMinimalIssueComment(&gogithub.IssueComment{
		ID:      gogithub.Ptr(int64(1)),
		Body:    gogithub.Ptr(baselineUnsafeText),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/1#issuecomment-1"),
	})
	assert.Equal(t, sanitizedUnsafeText, comment.Body)

	review := convertToMinimalPullRequestReview(&gogithub.PullRequestReview{
		ID:      gogithub.Ptr(int64(2)),
		State:   gogithub.Ptr("COMMENTED"),
		Body:    gogithub.Ptr(baselineUnsafeText),
		HTMLURL: gogithub.Ptr("https://github.com/owner/repo/pull/1#pullrequestreview-2"),
	})
	assert.Equal(t, sanitizedUnsafeText, review.Body)

	reviewURL, err := url.Parse("https://github.com/owner/repo/pull/1#discussion_r1")
	require.NoError(t, err)
	reviewComment := convertToMinimalReviewComment(reviewCommentNode{
		Body: baselineUnsafeText,
		Path: "README.md",
		URL:  githubv4.URI{URL: reviewURL},
	})
	assert.Equal(t, sanitizedUnsafeText, reviewComment.Body)
	assert.Equal(t, "README.md", reviewComment.Path)

	discussion := fragmentToDiscussion(NodeFragment{
		Number:    githubv4.Int(1),
		Title:     githubv4.String(baselineUnsafeText),
		URL:       githubv4.String("https://github.com/owner/repo/discussions/1"),
		CreatedAt: githubv4.DateTime{Time: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)},
		UpdatedAt: githubv4.DateTime{Time: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)},
	})
	assert.Equal(t, sanitizedUnsafeText, discussion.GetTitle())

	detail := discussionDetailFragment{
		Number:    githubv4.Int(1),
		Title:     githubv4.String(baselineUnsafeText),
		Body:      githubv4.String(baselineUnsafeText),
		URL:       githubv4.String("https://github.com/owner/repo/discussions/1"),
		CreatedAt: githubv4.DateTime{Time: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)},
	}
	detail.Category.Name = githubv4.String(baselineUnsafeText)
	detailResponse := discussionDetailResponse(detail)
	assert.Equal(t, sanitizedUnsafeText, detailResponse["title"])
	assert.Equal(t, sanitizedUnsafeText, detailResponse["body"])
	assert.Equal(t, sanitizedUnsafeText, detailResponse["category"].(map[string]any)["name"])

	discussionComment := convertToMinimalDiscussionComment(githubv4.ID("DC_1"), githubv4.String(baselineUnsafeText), githubv4.Boolean(true))
	assert.Equal(t, sanitizedUnsafeText, discussionComment.Body)
}

func TestSanitizationProjectDisplayTextPaths(t *testing.T) {
	sanitizedUnsafeText := sanitize.Sanitize(baselineUnsafeText)

	project := convertToMinimalProject(&gogithub.ProjectV2{
		Title:            gogithub.Ptr(baselineUnsafeText),
		Description:      gogithub.Ptr(baselineUnsafeText),
		ShortDescription: gogithub.Ptr(baselineUnsafeText),
	})
	require.NotNil(t, project)
	require.NotNil(t, project.Title)
	require.NotNil(t, project.Description)
	require.NotNil(t, project.ShortDescription)
	assert.Equal(t, sanitizedUnsafeText, *project.Title)
	assert.Equal(t, sanitizedUnsafeText, *project.Description)
	assert.Equal(t, sanitizedUnsafeText, *project.ShortDescription)

	content := convertIssueToMinimalProjectItemContent(&gogithub.Issue{
		Number: gogithub.Ptr(42),
		Title:  gogithub.Ptr(baselineUnsafeText),
		State:  gogithub.Ptr("open"),
		Labels: []*gogithub.Label{{Name: gogithub.Ptr(baselineUnsafeText)}},
	})
	require.NotNil(t, content)
	assert.Equal(t, sanitizedUnsafeText, content.Title)
	assert.Equal(t, []string{sanitizedUnsafeText}, content.Labels)

	prContent := convertPullRequestToMinimalProjectItemContent(&gogithub.PullRequest{
		Number: gogithub.Ptr(43),
		Title:  gogithub.Ptr(baselineUnsafeText),
		State:  gogithub.Ptr("open"),
		Labels: []*gogithub.Label{{Name: gogithub.Ptr(baselineUnsafeText)}},
	})
	require.NotNil(t, prContent)
	assert.Equal(t, sanitizedUnsafeText, prContent.Title)
	assert.Equal(t, []string{sanitizedUnsafeText}, prContent.Labels)

	draftIssueContent := convertDraftIssueToMinimalProjectItemContent(&gogithub.ProjectV2DraftIssue{
		Title: gogithub.Ptr(baselineUnsafeText),
	})
	require.NotNil(t, draftIssueContent)
	assert.Equal(t, sanitizedUnsafeText, draftIssueContent.Title)

	fields := convertToMinimalProjectItemFields([]*gogithub.ProjectV2ItemFieldValue{
		{
			ID:       gogithub.Ptr(int64(1)),
			Name:     gogithub.Ptr(baselineUnsafeText),
			DataType: gogithub.Ptr("text"),
			Value:    baselineUnsafeText,
		},
	})
	require.Len(t, fields, 1)
	assert.Equal(t, sanitizedUnsafeText, fields[0].Name)
	assert.Equal(t, sanitizedUnsafeText, fields[0].Value)

	option := minimalProjectFieldValue(&gogithub.ProjectV2FieldOption{
		ID:   gogithub.Ptr("option-id"),
		Name: &gogithub.ProjectV2TextContent{Raw: gogithub.Ptr(baselineUnsafeText)},
	})
	assert.Equal(t, minimalProjectOptionValue{ID: "option-id", Name: sanitizedUnsafeText}, option)

	body := githubv4.String(baselineUnsafeText)
	status := githubv4.String("ON_TRACK")
	statusUpdate := convertToMinimalStatusUpdate(statusUpdateNode{
		ID:        githubv4.ID("SU_1"),
		Body:      &body,
		Status:    &status,
		CreatedAt: githubv4.DateTime{Time: time.Date(2026, 7, 3, 9, 0, 0, 0, time.UTC)},
	})
	assert.Equal(t, sanitizedUnsafeText, statusUpdate.Body)
	assert.Equal(t, "ON_TRACK", statusUpdate.Status)
}

func TestSanitizeCommitMessage(t *testing.T) {
	t.Run("preserves valid trailer addresses while sanitizing the message", func(t *testing.T) {
		message := "<script>alert(1)</script>Subject\n\n" +
			"Co-authored-by: Copilot <copilot@github.com>\n" +
			"Signed-off-by: \"Copilot App\" <223556219+Copilot@users.noreply.github.com>"

		assert.Equal(t, "Subject\n\n"+
			"Co-authored-by: Copilot <copilot@github.com>\n"+
			"Signed-off-by: &#34;Copilot App&#34; <223556219+Copilot@users.noreply.github.com>",
			sanitizeCommitMessage(message))
	})

	t.Run("does not restore address-like text outside a trailer", func(t *testing.T) {
		message := "Contact Copilot <copilot@github.com>"
		assert.Equal(t, sanitizeOutputText(message), sanitizeCommitMessage(message))
	})

	t.Run("does not restore a trailer address with trailing HTML", func(t *testing.T) {
		message := "Co-authored-by: Copilot <copilot@github.com><script>alert(1)</script>"
		assert.Equal(t, sanitizeOutputText(message), sanitizeCommitMessage(message))
	})

	t.Run("does not restore HTML smuggled through address syntax", func(t *testing.T) {
		displayNameHTML := `Co-authored-by: "><img src=x onerror=alert(1)>" <copilot@github.com>`
		sanitized := sanitizeCommitMessage(displayNameHTML)
		assert.NotContains(t, sanitized, "onerror")
		assert.NotContains(t, sanitized, "alert(1)")
		assert.Contains(t, sanitized, "<copilot@github.com>")

		addressHTML := `Co-authored-by: Copilot <"<script>alert(1)</script>"@example.com>`
		assert.Equal(t, sanitizeOutputText(addressHTML), sanitizeCommitMessage(addressHTML))
	})

	t.Run("avoids collisions with placeholder-like message text", func(t *testing.T) {
		message := "GITHUBMCPCOMMITTRAILEREMAIL0PLACEHOLDER\n\nCo-authored-by: Copilot <copilot@github.com>"
		assert.Equal(t, message, sanitizeCommitMessage(message))
	})
}

func TestSanitizationOutputBypassesAndOutliers(t *testing.T) {
	sanitizedUnsafeText := sanitize.Sanitize(baselineUnsafeText)

	repo := &gogithub.Repository{
		ID:          gogithub.Ptr(int64(1)),
		Name:        gogithub.Ptr("repo"),
		FullName:    gogithub.Ptr("owner/repo"),
		Description: gogithub.Ptr(baselineUnsafeText),
		HTMLURL:     gogithub.Ptr("https://github.com/owner/repo"),
	}
	minimalRepo := convertToMinimalRepository(repo)
	assert.Equal(t, sanitizedUnsafeText, minimalRepo.Description)
	assert.Equal(t, baselineUnsafeText, repo.GetDescription())

	searchResult := sanitizedRepositoriesSearchResultCopy(&gogithub.RepositoriesSearchResult{
		Repositories: []*gogithub.Repository{repo},
	})
	require.Len(t, searchResult.Repositories, 1)
	assert.Equal(t, sanitizedUnsafeText, searchResult.Repositories[0].GetDescription())
	assert.Equal(t, baselineUnsafeText, repo.GetDescription())

	release := &gogithub.RepositoryRelease{
		Name: gogithub.Ptr(baselineUnsafeText),
		Body: gogithub.Ptr(baselineUnsafeText),
	}
	minimalRelease := convertToMinimalRelease(release)
	assert.Equal(t, sanitizedUnsafeText, minimalRelease.Name)
	assert.Equal(t, sanitizedUnsafeText, minimalRelease.Body)
	sanitizedRelease := sanitizedReleaseCopy(release)
	assert.Equal(t, sanitizedUnsafeText, sanitizedRelease.GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRelease.GetBody())
	assert.Equal(t, baselineUnsafeText, release.GetName())
	assert.Equal(t, baselineUnsafeText, release.GetBody())

	securityAdvisory := &gogithub.SecurityAdvisory{
		Summary:     gogithub.Ptr(baselineUnsafeText),
		Description: gogithub.Ptr(baselineUnsafeText),
		CollaboratingTeams: []*gogithub.Team{{
			Name:        gogithub.Ptr(baselineUnsafeText),
			Description: gogithub.Ptr(baselineUnsafeText),
			Slug:        gogithub.Ptr("team\u200B<script>"),
			Parent: &gogithub.Team{
				Name:        gogithub.Ptr(baselineUnsafeText),
				Description: gogithub.Ptr(baselineUnsafeText),
				Slug:        gogithub.Ptr("parent-exact"),
			},
		}},
	}
	advisory := sanitizedSecurityAdvisoryCopy(securityAdvisory)
	assert.Equal(t, sanitizedUnsafeText, advisory.GetSummary())
	assert.Equal(t, sanitizedUnsafeText, advisory.GetDescription())
	require.Len(t, advisory.CollaboratingTeams, 1)
	assert.Equal(t, sanitizedUnsafeText, advisory.CollaboratingTeams[0].GetName())
	assert.Equal(t, sanitizedUnsafeText, advisory.CollaboratingTeams[0].GetDescription())
	assert.Equal(t, "team\u200B<script>", advisory.CollaboratingTeams[0].GetSlug())
	assert.Equal(t, sanitizedUnsafeText, advisory.CollaboratingTeams[0].Parent.GetName())
	assert.Equal(t, sanitizedUnsafeText, advisory.CollaboratingTeams[0].Parent.GetDescription())
	assert.Equal(t, "parent-exact", advisory.CollaboratingTeams[0].Parent.GetSlug())
	assert.Equal(t, baselineUnsafeText, securityAdvisory.CollaboratingTeams[0].GetName())
	assert.Equal(t, baselineUnsafeText, securityAdvisory.CollaboratingTeams[0].Parent.GetName())

	dependabotAlert := &gogithub.DependabotAlert{
		SecurityAdvisory: &gogithub.DependabotSecurityAdvisory{
			Summary:     gogithub.Ptr(baselineUnsafeText),
			Description: gogithub.Ptr(baselineUnsafeText),
		},
		DismissedComment: gogithub.Ptr(baselineUnsafeText),
		Repository:       repo,
	}
	sanitizedDependabotAlert := sanitizedDependabotAlertCopy(dependabotAlert)
	assert.Equal(t, sanitizedUnsafeText, sanitizedDependabotAlert.SecurityAdvisory.GetSummary())
	assert.Equal(t, sanitizedUnsafeText, sanitizedDependabotAlert.SecurityAdvisory.GetDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedDependabotAlert.GetDismissedComment())
	assert.Equal(t, sanitizedUnsafeText, sanitizedDependabotAlert.Repository.GetDescription())
	assert.Equal(t, baselineUnsafeText, dependabotAlert.SecurityAdvisory.GetSummary())
	assert.Equal(t, baselineUnsafeText, dependabotAlert.GetDismissedComment())

	codeAlert := &gogithub.Alert{
		RuleDescription:  gogithub.Ptr(baselineUnsafeText),
		DismissedComment: gogithub.Ptr(baselineUnsafeText),
		Rule: &gogithub.Rule{
			Name:            gogithub.Ptr(baselineUnsafeText),
			Description:     gogithub.Ptr(baselineUnsafeText),
			FullDescription: gogithub.Ptr(baselineUnsafeText),
			Help:            gogithub.Ptr(baselineUnsafeText),
		},
		MostRecentInstance: &gogithub.MostRecentInstance{
			Message: &gogithub.Message{Text: gogithub.Ptr(baselineUnsafeText)},
		},
	}
	sanitizedCodeAlert := sanitizedCodeScanningAlertCopy(codeAlert)
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.GetRuleDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.GetDismissedComment())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.Rule.GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.Rule.GetDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.Rule.GetFullDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.Rule.GetHelp())
	assert.Equal(t, sanitizedUnsafeText, sanitizedCodeAlert.MostRecentInstance.Message.GetText())
	assert.Equal(t, baselineUnsafeText, codeAlert.GetRuleDescription())
	assert.Equal(t, baselineUnsafeText, codeAlert.Rule.GetHelp())

	secretAlert := &gogithub.SecretScanningAlert{
		Secret:                                     gogithub.Ptr(baselineUnsafeText),
		ResolutionComment:                          gogithub.Ptr(baselineUnsafeText),
		PushProtectionBypassRequestComment:         gogithub.Ptr(baselineUnsafeText),
		PushProtectionBypassRequestHTMLURL:         gogithub.Ptr("https://github.com/owner/repo/security/secret-scanning/1"),
		PushProtectionBypassRequestReviewer:        &gogithub.User{Login: gogithub.Ptr("reviewer")},
		PushProtectionBypassRequestReviewerComment: gogithub.Ptr(baselineUnsafeText),
	}
	sanitizedSecretAlert := sanitizedSecretScanningAlertCopy(secretAlert)
	assert.Equal(t, baselineUnsafeText, sanitizedSecretAlert.GetSecret())
	assert.Equal(t, sanitizedUnsafeText, sanitizedSecretAlert.GetResolutionComment())
	assert.Equal(t, sanitizedUnsafeText, sanitizedSecretAlert.GetPushProtectionBypassRequestComment())
	assert.Equal(t, sanitizedUnsafeText, sanitizedSecretAlert.GetPushProtectionBypassRequestReviewerComment())
	assert.Equal(t, baselineUnsafeText, secretAlert.GetResolutionComment())

	combinedStatus := &gogithub.CombinedStatus{
		Name: gogithub.Ptr("branch\u200B<script>"),
		SHA:  gogithub.Ptr("abc123"),
		Statuses: []*gogithub.RepoStatus{
			{
				Description: gogithub.Ptr(baselineUnsafeText),
				Context:     gogithub.Ptr("ci\u200B<script>"),
				TargetURL:   gogithub.Ptr("https://example.com/status<script>"),
			},
			nil,
		},
	}
	sanitizedStatus := sanitizedCombinedStatusCopy(combinedStatus)
	assert.Equal(t, "branch\u200B<script>", sanitizedStatus.GetName())
	assert.Equal(t, "abc123", sanitizedStatus.GetSHA())
	require.Len(t, sanitizedStatus.Statuses, 2)
	assert.Equal(t, sanitizedUnsafeText, sanitizedStatus.Statuses[0].GetDescription())
	assert.Equal(t, "ci\u200B<script>", sanitizedStatus.Statuses[0].GetContext())
	assert.Equal(t, "https://example.com/status<script>", sanitizedStatus.Statuses[0].GetTargetURL())
	assert.Nil(t, sanitizedStatus.Statuses[1])
	assert.Equal(t, baselineUnsafeText, combinedStatus.Statuses[0].GetDescription())

	workflow := &gogithub.Workflow{Name: gogithub.Ptr(baselineUnsafeText)}
	sanitizedWorkflow := sanitizedWorkflowCopy(workflow)
	assert.Equal(t, sanitizedUnsafeText, sanitizedWorkflow.GetName())
	assert.Equal(t, baselineUnsafeText, workflow.GetName())

	workflowRun := &gogithub.WorkflowRun{
		Name:         gogithub.Ptr(baselineUnsafeText),
		DisplayTitle: gogithub.Ptr(baselineUnsafeText),
		HeadCommit: &gogithub.HeadCommit{
			Message:   gogithub.Ptr(baselineUnsafeText),
			Author:    &gogithub.CommitAuthor{Name: gogithub.Ptr(baselineUnsafeText), Email: gogithub.Ptr("author@example.com")},
			Committer: &gogithub.CommitAuthor{Name: gogithub.Ptr(baselineUnsafeText), Email: gogithub.Ptr("committer@example.com")},
			Added:     []string{"src/exact<script>.go"},
		},
		PullRequests: []*gogithub.PullRequest{{
			Title:     gogithub.Ptr(baselineUnsafeText),
			Body:      gogithub.Ptr(baselineUnsafeText),
			Labels:    []*gogithub.Label{{Name: gogithub.Ptr(baselineUnsafeText), Description: gogithub.Ptr(baselineUnsafeText)}},
			Milestone: &gogithub.Milestone{Title: gogithub.Ptr(baselineUnsafeText), Description: gogithub.Ptr(baselineUnsafeText)},
			Head:      &gogithub.PullRequestBranch{Repo: repo},
			Base:      &gogithub.PullRequestBranch{Repo: repo},
		}},
		Repository: repo,
	}
	sanitizedRun := sanitizedWorkflowRunCopy(workflowRun)
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.GetDisplayTitle())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.PullRequests[0].GetTitle())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.PullRequests[0].Labels[0].GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.PullRequests[0].Milestone.GetTitle())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.PullRequests[0].Head.Repo.GetDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.PullRequests[0].Base.Repo.GetDescription())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.HeadCommit.GetMessage())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.HeadCommit.Author.GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.HeadCommit.Committer.GetName())
	assert.Equal(t, "author@example.com", sanitizedRun.HeadCommit.Author.GetEmail())
	assert.Equal(t, []string{"src/exact<script>.go"}, sanitizedRun.HeadCommit.Added)
	assert.Equal(t, sanitizedUnsafeText, sanitizedRun.Repository.GetDescription())
	assert.Equal(t, baselineUnsafeText, workflowRun.GetDisplayTitle())
	assert.Equal(t, baselineUnsafeText, workflowRun.HeadCommit.GetMessage())
	assert.Equal(t, baselineUnsafeText, workflowRun.PullRequests[0].Labels[0].GetName())

	commitMessage := "Subject\n\nCo-authored-by: Copilot <copilot@github.com>"
	sanitizedCommit := sanitizedCommitCopy(&gogithub.Commit{Message: gogithub.Ptr(commitMessage)})
	assert.Equal(t, commitMessage, sanitizedCommit.GetMessage())
	sanitizedHeadCommit := sanitizedHeadCommitCopy(&gogithub.HeadCommit{Message: gogithub.Ptr(commitMessage)})
	assert.Equal(t, commitMessage, sanitizedHeadCommit.GetMessage())

	workflowJob := &gogithub.WorkflowJob{
		Name:            gogithub.Ptr(baselineUnsafeText),
		WorkflowName:    gogithub.Ptr(baselineUnsafeText),
		RunnerName:      gogithub.Ptr(baselineUnsafeText),
		RunnerGroupName: gogithub.Ptr(baselineUnsafeText),
		Steps:           []*gogithub.TaskStep{{Name: gogithub.Ptr(baselineUnsafeText)}},
	}
	sanitizedJob := sanitizedWorkflowJobCopy(workflowJob)
	assert.Equal(t, sanitizedUnsafeText, sanitizedJob.GetName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedJob.GetWorkflowName())
	assert.Equal(t, baselineUnsafeText, sanitizedJob.GetRunnerName())
	assert.Equal(t, baselineUnsafeText, sanitizedJob.GetRunnerGroupName())
	assert.Equal(t, sanitizedUnsafeText, sanitizedJob.Steps[0].GetName())
	assert.Equal(t, baselineUnsafeText, workflowJob.GetName())
}

func TestSanitizationRawContentOutputsPreserveExactText(t *testing.T) {
	t.Run("file contents preserve exact text", func(t *testing.T) {
		rawContent := []byte(baselineUnsafeText)
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposGitRefByOwnerByRepoByRef: mockResponse(t, http.StatusOK, `{"ref":"refs/heads/main","object":{"sha":""}}`),
			GetReposByOwnerByRepo:            mockResponse(t, http.StatusOK, `{"name":"repo","default_branch":"main"}`),
			GetReposContentsByOwnerByRepoByPath: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				fileContent := &gogithub.RepositoryContent{
					Name:     gogithub.Ptr("README.md"),
					Path:     gogithub.Ptr("README.md"),
					SHA:      gogithub.Ptr("abc123"),
					Type:     gogithub.Ptr("file"),
					Content:  gogithub.Ptr(base64.StdEncoding.EncodeToString(rawContent)),
					Size:     gogithub.Ptr(len(rawContent)),
					Encoding: gogithub.Ptr("base64"),
				}
				require.NoError(t, json.NewEncoder(w).Encode(fileContent))
			},
		}))
		deps := BaseDeps{Client: client}
		serverTool := GetFileContents(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "README.md",
				"ref":   "refs/heads/main",
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
		resource := getResourceResult(t, result)
		assert.Equal(t, baselineUnsafeText, resource.Text)
	})

	t.Run("diff and patch outputs preserve exact text", func(t *testing.T) {
		patch := "@@ -1 +1 @@\n-unsafe\n+" + baselineUnsafeText
		prFiles := convertToMinimalPRFiles([]*gogithub.CommitFile{{Filename: gogithub.Ptr("README.md"), Patch: gogithub.Ptr(patch)}})
		require.Len(t, prFiles, 1)
		assert.Equal(t, patch, prFiles[0].Patch)

		commit := convertToMinimalCommit(&gogithub.RepositoryCommit{
			SHA:     gogithub.Ptr("abc123"),
			HTMLURL: gogithub.Ptr("https://github.com/owner/repo/commit/abc123"),
			Files:   []*gogithub.CommitFile{{Filename: gogithub.Ptr("README.md"), Patch: gogithub.Ptr(patch)}},
		}, commitDetailFullPatch)
		require.Len(t, commit.Files, 1)
		assert.Equal(t, patch, commit.Files[0].Patch)
	})

	t.Run("workflow logs preserve exact text", func(t *testing.T) {
		logContent := "line 1\n" + baselineUnsafeText + "\nline 3"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(logContent))
		}))
		t.Cleanup(server.Close)

		content, originalLength, resp, err := downloadLogContent(context.Background(), server.URL, 10, 10)
		if resp != nil {
			defer func() { _ = resp.Body.Close() }()
		}

		require.NoError(t, err)
		assert.Equal(t, logContent, content)
		assert.Equal(t, len(strings.Split(logContent, "\n")), originalLength)
	})

	t.Run("code search text matches preserve exact text", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetSearchCode: expectQueryParams(t, map[string]string{
				"q":        "repo:owner/repo unsafe",
				"page":     "1",
				"per_page": "30",
			}).withHeaders(map[string]string{"Accept": "text-match"}).andThen(mockResponse(t, http.StatusOK, &gogithub.CodeSearchResult{
				Total:             gogithub.Ptr(1),
				IncompleteResults: gogithub.Ptr(false),
				CodeResults: []*gogithub.CodeResult{{
					Name:       gogithub.Ptr("main.go"),
					Path:       gogithub.Ptr("main.go"),
					SHA:        gogithub.Ptr("abc123"),
					Repository: &gogithub.Repository{FullName: gogithub.Ptr("owner/repo")},
					TextMatches: []*gogithub.TextMatch{{
						Fragment: gogithub.Ptr(baselineUnsafeText),
					}},
				}},
			})),
		}))
		deps := BaseDeps{Client: client}
		serverTool := SearchCode(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"query": "repo:owner/repo unsafe",
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
		var searchResult MinimalCodeSearchResult
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &searchResult))
		require.Len(t, searchResult.Items, 1)
		require.Len(t, searchResult.Items[0].TextMatches, 1)
		assert.Equal(t, baselineUnsafeText, searchResult.Items[0].TextMatches[0].GetFragment())
	})
}

func TestSanitizationInputContentPreservation(t *testing.T) {
	t.Run("create issue preserves title and body request content", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesByOwnerByRepo: expectRequestBody(t, map[string]any{
				"title":     baselineUnsafeText,
				"body":      baselineUnsafeText,
				"assignees": []any{},
				"labels":    []any{},
			}).andThen(mockResponse(t, http.StatusCreated, &gogithub.Issue{
				ID:      gogithub.Ptr(int64(1)),
				HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/1"),
			})),
		}))

		result, err := CreateIssue(context.Background(), client, "owner", "repo", baselineUnsafeText, baselineUnsafeText, []string{}, []string{}, 0, "", nil)

		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("issue comment preserves body request content", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			PostReposIssuesCommentsByOwnerByRepoByIssueNumber: expectRequestBody(t, map[string]any{
				"body": baselineUnsafeText,
			}).andThen(mockResponse(t, http.StatusCreated, &gogithub.IssueComment{
				ID:      gogithub.Ptr(int64(1)),
				HTMLURL: gogithub.Ptr("https://github.com/owner/repo/issues/1#issuecomment-1"),
			})),
		}))
		deps := BaseDeps{Client: client}
		serverTool := AddIssueComment(translations.NullTranslationHelper)
		handler := serverTool.Handler(deps)

		result, err := handler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner":        "owner",
				"repo":         "repo",
				"issue_number": float64(1),
				"body":         baselineUnsafeText,
			}).Params,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("discussion comment preserves body request content", func(t *testing.T) {
		gqlClient := githubv4.NewClient(githubv4mock.NewMockedHTTPClient(
			githubv4mock.NewQueryMatcher(
				struct {
					Repository struct {
						Discussion struct {
							ID githubv4.ID
						} `graphql:"discussion(number: $discussionNumber)"`
					} `graphql:"repository(owner: $owner, name: $repo)"`
				}{},
				map[string]any{
					"owner":            githubv4.String("owner"),
					"repo":             githubv4.String("repo"),
					"discussionNumber": githubv4.Int(1),
				},
				githubv4mock.DataResponse(map[string]any{
					"repository": map[string]any{
						"discussion": map[string]any{"id": "D_1"},
					},
				}),
			),
			githubv4mock.NewMutationMatcher(
				struct {
					AddDiscussionComment struct {
						Comment struct {
							ID  githubv4.ID
							URL githubv4.String `graphql:"url"`
						}
					} `graphql:"addDiscussionComment(input: $input)"`
				}{},
				githubv4.AddDiscussionCommentInput{
					DiscussionID: githubv4.ID("D_1"),
					Body:         githubv4.String(baselineUnsafeText),
				},
				nil,
				githubv4mock.DataResponse(map[string]any{
					"addDiscussionComment": map[string]any{
						"comment": map[string]any{
							"id":  "DC_1",
							"url": "https://github.com/owner/repo/discussions/1#discussioncomment-1",
						},
					},
				}),
			),
		))

		result, _, err := addDiscussionComment(context.Background(), gqlClient, map[string]any{
			"owner":            "owner",
			"repo":             "repo",
			"discussionNumber": float64(1),
			"body":             baselineUnsafeText,
		})

		require.NoError(t, err)
		require.False(t, result.IsError)
	})

	t.Run("search query preserves syntax except intentional issue qualifier prefix", func(t *testing.T) {
		query, opts, err := prepareSearchArgs(map[string]any{
			"query": `repo:github/github-mcp-server label:critical OR "exact phrase" field.priority:P1`,
		}, "issue")

		require.NoError(t, err)
		assert.Equal(t, `is:issue repo:github/github-mcp-server label:critical OR "exact phrase" field.priority:P1`, query)
		require.NotNil(t, opts.AdvancedSearch)
		assert.True(t, *opts.AdvancedSearch)
	})
}

func TestSanitizationInputPolicyValidation(t *testing.T) {
	t.Run("repo-relative path validation rejects traversal leading slash and control characters", func(t *testing.T) {
		require.NoError(t, validateRepoRelativePath("path", "docs/readme.md"))
		require.NoError(t, validateRepoRelativePath("path", " "))
		require.NoError(t, validateRepoRelativePath("path", "\u00a0"))
		assert.EqualError(t, validateRepoRelativePath("path", ""), "path must not be empty")
		assert.EqualError(t, validateRepoRelativePath("path", "/docs/readme.md"), "path must be relative to the repository root (no leading '/')")
		assert.EqualError(t, validateRepoRelativePath("path", "docs/../secrets.txt"), "path must not contain '..' segments")
		assert.EqualError(t, validateRepoRelativePath("path", "docs/readme.md\x00"), "path must not contain control characters")
		require.NoError(t, validateRepoRelativePathOrRoot("path", "/"))
	})

	t.Run("enum validation rejects undeclared control values", func(t *testing.T) {
		require.NoError(t, validateEnumParam("sort", "", "created", "updated"))
		require.NoError(t, validateEnumParam("sort", "created", "created", "updated"))
		assert.EqualError(t, validateEnumParam("sort", "pushed", "created", "updated"), "sort must be one of: created, updated")
	})

	t.Run("search issue validates sort but preserves query syntax", func(t *testing.T) {
		query, opts, err := prepareSearchArgs(map[string]any{
			"query": `repo:github/github-mcp-server label:critical OR "exact phrase" field.priority:P1`,
			"sort":  "updated",
			"order": "desc",
		}, "issue")
		require.NoError(t, err)
		assert.Equal(t, `is:issue repo:github/github-mcp-server label:critical OR "exact phrase" field.priority:P1`, query)
		assert.Equal(t, "updated", opts.Sort)
		assert.Equal(t, "desc", opts.Order)

		_, _, err = prepareSearchArgs(map[string]any{
			"query": "unsafe",
			"sort":  "pushed",
		}, "issue")
		assert.EqualError(t, err, "sort must be one of: comments, reactions, reactions-+1, reactions--1, reactions-smile, reactions-thinking_face, reactions-heart, reactions-tada, interactions, created, updated")
	})

	t.Run("file handlers reject invalid paths before content mutation", func(t *testing.T) {
		deps := BaseDeps{}
		getFileTool := GetFileContents(translations.NullTranslationHelper)
		getFileHandler := getFileTool.Handler(deps)
		result, err := getFileHandler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "../README.md",
			}).Params,
		})
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "path must not contain '..' segments")

		createTool := CreateOrUpdateFile(translations.NullTranslationHelper)
		createHandler := createTool.Handler(deps)
		result, err = createHandler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"path":    "/README.md",
				"content": baselineUnsafeText,
				"message": baselineUnsafeText,
				"branch":  "main",
			}).Params,
		})
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "path must be relative to the repository root")

		pushTool := PushFiles(translations.NullTranslationHelper)
		pushHandler := pushTool.Handler(deps)
		result, err = pushHandler(ContextWithDeps(context.Background(), deps), &mcp.CallToolRequest{
			Params: createMCPRequest(map[string]any{
				"owner":   "owner",
				"repo":    "repo",
				"branch":  "main",
				"message": baselineUnsafeText,
				"files": []any{
					map[string]any{"path": "bad\x00path", "content": baselineUnsafeText},
				},
			}).Params,
		})
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "path must not contain control characters")
	})
}
