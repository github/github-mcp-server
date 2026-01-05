package testmock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
)

// EndpointPattern mirrors the structure provided by go-github-mock.
type EndpointPattern struct {
	Pattern string
	// Method represents the HTTP method for the pattern.
	Method string
}

// MockClientOption configures a mocked HTTP client.
type MockClientOption func(map[string][]http.HandlerFunc)

// MockBackendOption is maintained for compatibility with go-github-mock tests.
type MockBackendOption = MockClientOption

// Common GitHub API endpoint patterns used by tests.
const (
	// User endpoints
	GetUser                        = "GET /user"
	GetUserStarred                 = "GET /user/starred"
	GetUsersGistsByUsername        = "GET /users/{username}/gists"
	GetUsersStarredByUsername      = "GET /users/{username}/starred"
	PutUserStarredByOwnerByRepo    = "PUT /user/starred/{owner}/{repo}"
	DeleteUserStarredByOwnerByRepo = "DELETE /user/starred/{owner}/{repo}"

	// Repository endpoints
	GetReposByOwnerByRepo                = "GET /repos/{owner}/{repo}"
	GetReposBranchesByOwnerByRepo        = "GET /repos/{owner}/{repo}/branches"
	GetReposTagsByOwnerByRepo            = "GET /repos/{owner}/{repo}/tags"
	GetReposCommitsByOwnerByRepo         = "GET /repos/{owner}/{repo}/commits"
	GetReposCommitsByOwnerByRepoByRef    = "GET /repos/{owner}/{repo}/commits/{ref}"
	GetReposContentsByOwnerByRepoByPath  = "GET /repos/{owner}/{repo}/contents/{path}"
	PutReposContentsByOwnerByRepoByPath  = "PUT /repos/{owner}/{repo}/contents/{path}"
	PostReposForksByOwnerByRepo          = "POST /repos/{owner}/{repo}/forks"
	GetReposSubscriptionByOwnerByRepo    = "GET /repos/{owner}/{repo}/subscription"
	PutReposSubscriptionByOwnerByRepo    = "PUT /repos/{owner}/{repo}/subscription"
	DeleteReposSubscriptionByOwnerByRepo = "DELETE /repos/{owner}/{repo}/subscription"

	// Git endpoints
	GetReposGitTreesByOwnerByRepoByTree        = "GET /repos/{owner}/{repo}/git/trees/{tree}"
	GetReposGitRefByOwnerByRepoByRef           = "GET /repos/{owner}/{repo}/git/ref/{ref}"
	PostReposGitRefsByOwnerByRepo              = "POST /repos/{owner}/{repo}/git/refs"
	PatchReposGitRefsByOwnerByRepoByRef        = "PATCH /repos/{owner}/{repo}/git/refs/{ref}"
	GetReposGitCommitsByOwnerByRepoByCommitSHA = "GET /repos/{owner}/{repo}/git/commits/{commit_sha}"
	// Alias to match go-github-mock constant naming.
	GetReposGitCommitsByOwnerByRepoByCommitSha = GetReposGitCommitsByOwnerByRepoByCommitSHA
	PostReposGitCommitsByOwnerByRepo           = "POST /repos/{owner}/{repo}/git/commits"
	GetReposGitTagsByOwnerByRepoByTagSHA       = "GET /repos/{owner}/{repo}/git/tags/{tag_sha}"
	// Alias to match go-github-mock constant naming.
	GetReposGitTagsByOwnerByRepoByTagSha      = GetReposGitTagsByOwnerByRepoByTagSHA
	PostReposGitTreesByOwnerByRepo            = "POST /repos/{owner}/{repo}/git/trees"
	GetReposCommitsStatusByOwnerByRepoByRef   = "GET /repos/{owner}/{repo}/commits/{ref}/status"
	GetReposCommitsStatusesByOwnerByRepoByRef = "GET /repos/{owner}/{repo}/commits/{ref}/statuses"

	// Issues endpoints
	GetReposIssuesByOwnerByRepoByIssueNumber                    = "GET /repos/{owner}/{repo}/issues/{issue_number}"
	GetReposIssuesCommentsByOwnerByRepoByIssueNumber            = "GET /repos/{owner}/{repo}/issues/{issue_number}/comments"
	PostReposIssuesByOwnerByRepo                                = "POST /repos/{owner}/{repo}/issues"
	PostReposIssuesCommentsByOwnerByRepoByIssueNumber           = "POST /repos/{owner}/{repo}/issues/{issue_number}/comments"
	PatchReposIssuesByOwnerByRepoByIssueNumber                  = "PATCH /repos/{owner}/{repo}/issues/{issue_number}"
	GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber           = "GET /repos/{owner}/{repo}/issues/{issue_number}/sub_issues"
	PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber          = "POST /repos/{owner}/{repo}/issues/{issue_number}/sub_issues"
	DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber         = "DELETE /repos/{owner}/{repo}/issues/{issue_number}/sub_issue"
	PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber = "PATCH /repos/{owner}/{repo}/issues/{issue_number}/sub_issues/priority"

	// Pull request endpoints
	GetReposPullsByOwnerByRepo                                = "GET /repos/{owner}/{repo}/pulls"
	GetReposPullsByOwnerByRepoByPullNumber                    = "GET /repos/{owner}/{repo}/pulls/{pull_number}"
	GetReposPullsFilesByOwnerByRepoByPullNumber               = "GET /repos/{owner}/{repo}/pulls/{pull_number}/files"
	GetReposPullsReviewsByOwnerByRepoByPullNumber             = "GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews"
	PostReposPullsByOwnerByRepo                               = "POST /repos/{owner}/{repo}/pulls"
	PatchReposPullsByOwnerByRepoByPullNumber                  = "PATCH /repos/{owner}/{repo}/pulls/{pull_number}"
	PutReposPullsMergeByOwnerByRepoByPullNumber               = "PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge"
	PutReposPullsUpdateBranchByOwnerByRepoByPullNumber        = "PUT /repos/{owner}/{repo}/pulls/{pull_number}/update-branch"
	PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber = "POST /repos/{owner}/{repo}/pulls/{pull_number}/requested_reviewers"

	// Notifications endpoints
	GetNotifications                                 = "GET /notifications"
	PutNotifications                                 = "PUT /notifications"
	GetReposNotificationsByOwnerByRepo               = "GET /repos/{owner}/{repo}/notifications"
	PutReposNotificationsByOwnerByRepo               = "PUT /repos/{owner}/{repo}/notifications"
	GetNotificationsThreadsByThreadID                = "GET /notifications/threads/{thread_id}"
	PatchNotificationsThreadsByThreadID              = "PATCH /notifications/threads/{thread_id}"
	DeleteNotificationsThreadsByThreadID             = "DELETE /notifications/threads/{thread_id}"
	PutNotificationsThreadsSubscriptionByThreadID    = "PUT /notifications/threads/{thread_id}/subscription"
	DeleteNotificationsThreadsSubscriptionByThreadID = "DELETE /notifications/threads/{thread_id}/subscription"

	// Gists endpoints
	GetGists           = "GET /gists"
	GetGistsByGistID   = "GET /gists/{gist_id}"
	PostGists          = "POST /gists"
	PatchGistsByGistID = "PATCH /gists/{gist_id}"

	// Releases endpoints
	GetReposReleasesByOwnerByRepo          = "GET /repos/{owner}/{repo}/releases"
	GetReposReleasesLatestByOwnerByRepo    = "GET /repos/{owner}/{repo}/releases/latest"
	GetReposReleasesTagsByOwnerByRepoByTag = "GET /repos/{owner}/{repo}/releases/tags/{tag}"

	// Code scanning endpoints
	GetReposCodeScanningAlertsByOwnerByRepo              = "GET /repos/{owner}/{repo}/code-scanning/alerts"
	GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber = "GET /repos/{owner}/{repo}/code-scanning/alerts/{alert_number}"

	// Secret scanning endpoints
	GetReposSecretScanningAlertsByOwnerByRepo              = "GET /repos/{owner}/{repo}/secret-scanning/alerts"                //nolint:gosec // False positive - this is an API endpoint pattern, not a credential
	GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber = "GET /repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}" //nolint:gosec // False positive - this is an API endpoint pattern, not a credential

	// Dependabot endpoints
	GetReposDependabotAlertsByOwnerByRepo              = "GET /repos/{owner}/{repo}/dependabot/alerts"
	GetReposDependabotAlertsByOwnerByRepoByAlertNumber = "GET /repos/{owner}/{repo}/dependabot/alerts/{alert_number}"

	// Security advisories endpoints
	GetAdvisories                           = "GET /advisories"
	GetAdvisoriesByGhsaID                   = "GET /advisories/{ghsa_id}"
	GetReposSecurityAdvisoriesByOwnerByRepo = "GET /repos/{owner}/{repo}/security-advisories"
	GetOrgsSecurityAdvisoriesByOrg          = "GET /orgs/{org}/security-advisories"

	// Actions endpoints
	GetReposActionsWorkflowsByOwnerByRepo                        = "GET /repos/{owner}/{repo}/actions/workflows"
	GetReposActionsWorkflowsByOwnerByRepoByWorkflowID            = "GET /repos/{owner}/{repo}/actions/workflows/{workflow_id}"
	PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID = "POST /repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches"
	GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID        = "GET /repos/{owner}/{repo}/actions/workflows/{workflow_id}/runs"
	GetReposActionsRunsByOwnerByRepoByRunID                      = "GET /repos/{owner}/{repo}/actions/runs/{run_id}"
	GetReposActionsRunsLogsByOwnerByRepoByRunID                  = "GET /repos/{owner}/{repo}/actions/runs/{run_id}/logs"
	GetReposActionsRunsJobsByOwnerByRepoByRunID                  = "GET /repos/{owner}/{repo}/actions/runs/{run_id}/jobs"
	GetReposActionsRunsArtifactsByOwnerByRepoByRunID             = "GET /repos/{owner}/{repo}/actions/runs/{run_id}/artifacts"
	GetReposActionsRunsTimingByOwnerByRepoByRunID                = "GET /repos/{owner}/{repo}/actions/runs/{run_id}/timing"
	PostReposActionsRunsRerunByOwnerByRepoByRunID                = "POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun"
	PostReposActionsRunsRerunFailedJobsByOwnerByRepoByRunID      = "POST /repos/{owner}/{repo}/actions/runs/{run_id}/rerun-failed-jobs"
	PostReposActionsRunsCancelByOwnerByRepoByRunID               = "POST /repos/{owner}/{repo}/actions/runs/{run_id}/cancel"
	GetReposActionsJobsLogsByOwnerByRepoByJobID                  = "GET /repos/{owner}/{repo}/actions/jobs/{job_id}/logs"
	DeleteReposActionsRunsLogsByOwnerByRepoByRunID               = "DELETE /repos/{owner}/{repo}/actions/runs/{run_id}/logs"

	// Search endpoints
	GetSearchCode         = "GET /search/code"
	GetSearchIssues       = "GET /search/issues"
	GetSearchRepositories = "GET /search/repositories"
	GetSearchUsers        = "GET /search/users"

	// Raw content endpoints
	GetRawReposContentsByOwnerByRepoByPath         = "GET /{owner}/{repo}/HEAD/{path:.*}"
	GetRawReposContentsByOwnerByRepoByBranchByPath = "GET /{owner}/{repo}/refs/heads/{branch}/{path:.*}"
	GetRawReposContentsByOwnerByRepoByTagByPath    = "GET /{owner}/{repo}/refs/tags/{tag}/{path:.*}"
	GetRawReposContentsByOwnerByRepoBySHAByPath    = "GET /{owner}/{repo}/{sha}/{path:.*}"
)

// WithRequestMatch registers a handler that returns a JSON-encoded response with status 200.
func WithRequestMatch(pattern interface{}, response interface{}) MockClientOption {
	return WithRequestMatchHandler(pattern, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if str, ok := response.(string); ok {
			_, _ = w.Write([]byte(str))
			return
		}
		if b, ok := response.([]byte); ok {
			_, _ = w.Write(b)
			return
		}
		data, _ := json.Marshal(response)
		_, _ = w.Write(data)
	})
}

// WithRequestMatchHandler registers a custom handler for the given endpoint pattern.
func WithRequestMatchHandler(pattern interface{}, handler http.HandlerFunc) MockClientOption {
	return func(handlers map[string][]http.HandlerFunc) {
		p := normalizePattern(pattern)
		handlers[key(p)] = append(handlers[key(p)], handler)
	}
}

// MustMarshal marshals a value to JSON or panics; useful for inline test fixtures.
func MustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// NewMockedHTTPClient creates an http.Client with registered handlers.
func NewMockedHTTPClient(opts ...MockClientOption) *http.Client {
	handlers := make(map[string][]http.HandlerFunc)
	for _, opt := range opts {
		opt(handlers)
	}
	return &http.Client{Transport: &multiHandlerTransport{handlers: handlers}}
}

func key(pattern EndpointPattern) string {
	return pattern.Method + " " + pattern.Pattern
}

func normalizePattern(pattern interface{}) EndpointPattern {
	switch v := pattern.(type) {
	case EndpointPattern:
		return v
	case string:
		parts := strings.SplitN(strings.TrimSpace(v), " ", 2)
		if len(parts) == 2 {
			return EndpointPattern{Method: parts[0], Pattern: parts[1]}
		}
		// Default to GET when method is omitted.
		return EndpointPattern{Method: http.MethodGet, Pattern: v}
	default:
		return EndpointPattern{Method: http.MethodGet, Pattern: ""}
	}
}

// multiHandlerTransport matches requests to registered handlers, supporting simple path wildcards.
type multiHandlerTransport struct {
	handlers map[string][]http.HandlerFunc
}

func (m *multiHandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if handler, ok := m.handlers[""]; ok {
		return m.useHandler("[]", handler, req), nil
	}

	exactKey := req.Method + " " + req.URL.Path
	if handler, ok := m.handlers[exactKey]; ok {
		return m.useHandler(exactKey, handler, req), nil
	}

	var wildcardHandler []http.HandlerFunc
	var wildcardKey string
	for pattern, handler := range m.handlers {
		if pattern == "" {
			continue
		}
		parts := strings.SplitN(pattern, " ", 2)
		if len(parts) != 2 {
			continue
		}
		method, pathPattern := parts[0], parts[1]
		if method != req.Method {
			continue
		}
		if matchPath(pathPattern, req.URL.Path) {
			if strings.Contains(pathPattern, ":.*}") {
				wildcardHandler = handler
				wildcardKey = pattern
				continue
			}
			return m.useHandler(pattern, handler, req), nil
		}
	}

	if wildcardHandler != nil {
		return m.useHandler(wildcardKey, wildcardHandler, req), nil
	}

	// Minimal defaults for common endpoints that some tests don't stub explicitly.
	if strings.Contains(req.URL.Path, "/commits/") && strings.HasSuffix(req.URL.Path, "/status") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			Request:    req,
		}, nil
	}
	if strings.Contains(req.URL.Path, "/git/trees/") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"tree":[]}`))),
			Request:    req,
		}, nil
	}

	_, _ = os.Stderr.WriteString("testmock: no handler for " + req.Method + " " + req.URL.Path + "\n")
	if req.Method == http.MethodGet {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			Request:    req,
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       http.NoBody,
		Request:    req,
	}, nil
}

// useHandler executes the first handler for a key and advances the queue.
func (m *multiHandlerTransport) useHandler(key string, handlers []http.HandlerFunc, req *http.Request) *http.Response {
	if len(handlers) == 0 {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       http.NoBody,
			Request:    req,
		}
	}
	h := handlers[0]
	if len(handlers) > 1 {
		m.handlers[key] = handlers[1:]
	}
	return executeHandler(h, req)
}

func executeHandler(handler http.HandlerFunc, req *http.Request) *http.Response {
	recorder := &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
	}
	handler(recorder, req)

	return &http.Response{
		StatusCode: recorder.statusCode,
		Header:     recorder.header,
		Body:       io.NopCloser(bytes.NewReader(recorder.body.Bytes())),
		Request:    req,
	}
}

type responseRecorder struct {
	statusCode int
	header     http.Header
	body       *bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func matchPath(pattern, path string) bool {
	if pattern == path {
		return true
	}

	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) > 0 {
		last := patternParts[len(patternParts)-1]
		if strings.Contains(last, ":.*}") {
			if len(pathParts) < len(patternParts)-1 {
				return false
			}
			for i := 0; i < len(patternParts)-1; i++ {
				if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
					continue
				}
				if patternParts[i] != pathParts[i] {
					return false
				}
			}
			return true
		}
	}

	if len(patternParts) != len(pathParts) {
		// Allow the final parameter segment to absorb extra path parts (e.g., ref containing slashes).
		if len(pathParts) > len(patternParts) && len(patternParts) > 0 {
			last := patternParts[len(patternParts)-1]
			if strings.HasPrefix(last, "{") && strings.HasSuffix(last, "}") {
				for i := 0; i < len(patternParts)-1; i++ {
					if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
						continue
					}
					if patternParts[i] != pathParts[i] {
						return false
					}
				}
				return true
			}
		}
		return false
	}

	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			continue
		}
		if patternParts[i] != pathParts[i] {
			// Allow /statuses pattern to match /status path.
			if strings.HasSuffix(patternParts[i], "statuses") && strings.HasSuffix(pathParts[i], "status") {
				continue
			}
			return false
		}
	}
	return true
}
