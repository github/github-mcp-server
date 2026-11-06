package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotAssignmentPollingCancellation(t *testing.T) {
	for _, tool := range []inventory.ServerTool{
		AssignCopilotToIssue(translations.NullTranslationHelper),
		AssignCopilotToIssueWithIntent(translations.NullTranslationHelper),
	} {
		for _, cancelDelay := range []time.Duration{0, time.Second, -1} {
			name := cancelDelay.String()
			if cancelDelay < 0 {
				name = "not_canceled"
			}
			t.Run(tool.Tool.Name+"/"+name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					polls, assignments := 0, 0
					httpClient := &http.Client{Transport: typedOutputRoundTripper(func(request *http.Request) (*http.Response, error) {
						if err := request.Context().Err(); err != nil {
							return nil, err
						}
						query, err := io.ReadAll(request.Body)
						require.NoError(t, err)
						var body string
						switch {
						case strings.Contains(string(query), "timelineItems"):
							polls++
							body = `{"data":{"repository":{"issue":{"timelineItems":{"nodes":[]}}}}}`
							if cancelDelay == 0 {
								cancel()
							} else if cancelDelay > 0 {
								go func() {
									time.Sleep(cancelDelay)
									cancel()
								}()
							}
						case strings.Contains(string(query), "suggestedActors"):
							body = `{"data":{"repository":{"suggestedActors":{"nodes":[{"id":"BOT","login":"copilot-swe-agent","__typename":"Bot"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`
						case strings.Contains(string(query), "issue(number:"):
							body = `{"data":{"repository":{"id":"REPO","issue":{"id":"ISSUE","assignees":{"nodes":[]}}}}}`
						case strings.Contains(string(query), "updateIssue(input:"):
							assignments++
							body = `{"data":{"updateIssue":{"issue":{"id":"ISSUE","number":7,"url":"https://github.com/owner/repo/issues/7"}}}}`
						default:
							t.Fatalf("unexpected GraphQL query: %s", query)
						}
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     http.Header{"Content-Type": []string{"application/json"}},
							Body:       io.NopCloser(strings.NewReader(body)),
							Request:    request,
						}, nil
					})}
					deps := BaseDeps{GQLClient: githubv4.NewEnterpriseClient("https://api.github.com/graphql", httpClient)}
					ctx = ContextWithDeps(ContextWithPollConfig(ctx, PollConfig{MaxAttempts: 2, Delay: time.Hour}), deps)
					request := createMCPRequest(map[string]any{
						"owner": "owner", "repo": "repo", "issue_number": 7,
						"rationale": "Fix this issue", "confidence": "HIGH", "is_suggestion": false,
					})
					start := time.Now()
					result, err := tool.Handler(deps)(ctx, &request)
					require.NoError(t, err)
					require.NotNil(t, result)
					expectedDuration, expectedPolls := cancelDelay, 1
					if cancelDelay < 0 {
						expectedDuration, expectedPolls = time.Hour, 2
					}
					assert.Equal(t, expectedDuration, time.Since(start), "polling should honor cancellation and otherwise wait between retries")
					assert.False(t, result.IsError, "assignment succeeded before polling was canceled")
					assert.Equal(t, 1, assignments)
					assert.Equal(t, expectedPolls, polls)
					var output map[string]any
					require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &output))
					assert.Equal(t, "successfully assigned copilot to issue - pull request pending", output["message"])
					assert.Equal(t, float64(7), output["issue_number"])
					assert.Equal(t, "https://github.com/owner/repo/issues/7", output["issue_url"])
					assert.NotContains(t, output, "pull_request")
				})
			})
		}
	}
}
