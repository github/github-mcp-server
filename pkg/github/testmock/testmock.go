package mock

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// EndpointPattern mirrors the structure used by migueleliasweb/go-github-mock.
type EndpointPattern struct {
	Pattern string
	Method  string
}

type Option func(map[string]http.HandlerFunc)

// WithRequestMatch registers a handler that returns the provided response with HTTP 200.
func WithRequestMatch(pattern EndpointPattern, response any) Option {
	return func(handlers map[string]http.HandlerFunc) {
		handlers[key(pattern)] = func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			switch body := response.(type) {
			case string:
				if _, err := w.Write([]byte(body)); err != nil {
					panic(err)
				}
			default:
				if body == nil {
					return
				}
				data, err := json.Marshal(body)
				if err != nil {
					panic(err)
				}
				if _, err := w.Write(data); err != nil {
					panic(err)
				}
			}
		}
	}
}

// WithRequestMatchHandler registers a custom handler for the given pattern.
func WithRequestMatchHandler(pattern EndpointPattern, handler http.HandlerFunc) Option {
	return func(handlers map[string]http.HandlerFunc) {
		handlers[key(pattern)] = handler
	}
}

// NewMockedHTTPClient creates an HTTP client that routes requests through registered handlers.
func NewMockedHTTPClient(options ...Option) *http.Client {
	handlers := make(map[string]http.HandlerFunc)
	for _, opt := range options {
		if opt != nil {
			opt(handlers)
		}
	}
	return &http.Client{Transport: &transport{handlers: handlers}}
}

type transport struct {
	handlers map[string]http.HandlerFunc
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if handler, ok := t.handlers[key(EndpointPattern{Method: req.Method, Pattern: req.URL.Path})]; ok {
		return executeHandler(handler, req), nil
	}

	for patternKey, handler := range t.handlers {
		method, pattern, ok := splitKey(patternKey)
		if !ok || method != req.Method {
			continue
		}
		if matchPath(pattern, req.URL.Path) {
			return executeHandler(handler, req), nil
		}
	}

	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		Request:    req,
	}, nil
}

func executeHandler(handler http.HandlerFunc, req *http.Request) *http.Response {
	rec := &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
	}
	handler(rec, req)
	return &http.Response{
		StatusCode: rec.statusCode,
		Header:     rec.header,
		Body:       io.NopCloser(bytes.NewReader(rec.body.Bytes())),
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

func key(p EndpointPattern) string {
	return strings.ToUpper(p.Method) + " " + p.Pattern
}

func splitKey(k string) (method, pattern string, ok bool) {
	parts := strings.SplitN(k, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", "", false
}

func matchPath(pattern, path string) bool {
	if pattern == "" {
		return path == ""
	}

	if pattern == path {
		return true
	}

	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}
	return true
}

// MustMarshal marshals the provided value or panics on error.
// Use this in tests when marshaling test data should halt execution immediately on failure.
func MustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// REST endpoint patterns used in actions, issues and projects tests.
var (
	GetReposActionsJobsLogsByOwnerByRepoByJobID                  = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/jobs/{job_id}/logs", Method: http.MethodGet}
	GetReposActionsRunsByOwnerByRepo                             = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/runs", Method: http.MethodGet}
	GetReposActionsRunsByOwnerByRepoByRunID                      = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/runs/{run_id}", Method: http.MethodGet}
	GetReposActionsRunsJobsByOwnerByRepoByRunID                  = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/runs/{run_id}/jobs", Method: http.MethodGet}
	GetReposActionsRunsLogsByOwnerByRepoByRunID                  = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/runs/{run_id}/logs", Method: http.MethodGet}
	GetReposActionsWorkflowsByOwnerByRepo                        = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/workflows", Method: http.MethodGet}
	GetReposActionsWorkflowsByOwnerByRepoByWorkflowID            = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/workflows/{workflow_id}", Method: http.MethodGet}
	GetReposActionsWorkflowsRunsByOwnerByRepoByWorkflowID        = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/workflows/{workflow_id}/runs", Method: http.MethodGet}
	PostReposActionsWorkflowsDispatchesByOwnerByRepoByWorkflowID = EndpointPattern{Pattern: "/repos/{owner}/{repo}/actions/workflows/{workflow_id}/dispatches", Method: http.MethodPost}

	GetReposIssuesByOwnerByRepoByIssueNumber                    = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}", Method: http.MethodGet}
	GetReposIssuesCommentsByOwnerByRepoByIssueNumber            = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/comments", Method: http.MethodGet}
	GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber           = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/sub_issues", Method: http.MethodGet}
	PostReposIssuesByOwnerByRepo                                = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues", Method: http.MethodPost}
	PostReposIssuesCommentsByOwnerByRepoByIssueNumber           = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/comments", Method: http.MethodPost}
	PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber          = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/sub_issues", Method: http.MethodPost}
	DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber         = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/sub_issue", Method: http.MethodDelete}
	PatchReposIssuesByOwnerByRepoByIssueNumber                  = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}", Method: http.MethodPatch}
	PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber = EndpointPattern{Pattern: "/repos/{owner}/{repo}/issues/{issue_number}/sub_issues/priority", Method: http.MethodPatch}

	GetSearchIssues = EndpointPattern{Pattern: "/search/issues", Method: http.MethodGet}
)
