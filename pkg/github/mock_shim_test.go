package github

import (
	"encoding/json"
	"net/http"
)

type requestMatchOption func(map[string]http.HandlerFunc)

type EndpointPattern struct {
	Method  string
	Pattern string
}

func endpointKey(endpoint any) string {
	switch v := endpoint.(type) {
	case string:
		return v
	case EndpointPattern:
		return v.Method + " " + v.Pattern
	default:
		panic("unsupported endpoint type")
	}
}

func newMockedHTTPClient(opts ...requestMatchOption) *http.Client {
	handlers := map[string]http.HandlerFunc{}
	for _, opt := range opts {
		if opt != nil {
			opt(handlers)
		}
	}
	return MockHTTPClientWithHandlers(handlers)
}

func withRequestMatch(endpoint any, response any) requestMatchOption {
	return withRequestMatchHandler(endpoint, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch body := response.(type) {
		case string:
			_, _ = w.Write([]byte(body))
			return
		case []byte:
			_, _ = w.Write(body)
			return
		case nil:
			return
		default:
			contentBytes, err := json.Marshal(body)
			if err != nil {
				panic(err)
			}
			_, _ = w.Write(contentBytes)
		}
	}))
}

func withRequestMatchHandler(endpoint any, handler http.HandlerFunc) requestMatchOption {
	return func(handlers map[string]http.HandlerFunc) {
		handlers[endpointKey(endpoint)] = handler
	}
}

// mock is a local shim that preserves a legacy callsite style in tests,
// while delegating to the PR #1629 handler-map mock implementation.
var mock = struct {
	NewMockedHTTPClient     func(...requestMatchOption) *http.Client
	WithRequestMatch        func(any, any) requestMatchOption
	WithRequestMatchHandler func(any, http.HandlerFunc) requestMatchOption

	GetReposIssuesByOwnerByRepoByIssueNumber          string
	GetSearchIssues                                   string
	GetReposIssuesCommentsByOwnerByRepoByIssueNumber  string
	GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber string
	GetReposCommitsStatusesByOwnerByRepoByRef         string
	GetReposCommitsStatusByOwnerByRepoByRef           string
	GetReposPullsByOwnerByRepo                        string
	GetReposPullsByOwnerByRepoByPullNumber            string
	GetReposPullsFilesByOwnerByRepoByPullNumber       string
	GetReposPullsReviewsByOwnerByRepoByPullNumber     string

	PostReposIssuesByOwnerByRepo                              string
	PostReposIssuesCommentsByOwnerByRepoByIssueNumber         string
	PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber        string
	PostReposPullsByOwnerByRepo                               string
	PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber string

	PatchReposIssuesByOwnerByRepoByIssueNumber                  string
	PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber string
	PatchReposPullsByOwnerByRepoByPullNumber                    string

	PutReposPullsMergeByOwnerByRepoByPullNumber        string
	PutReposPullsUpdateBranchByOwnerByRepoByPullNumber string

	DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber string
}{
	NewMockedHTTPClient:     newMockedHTTPClient,
	WithRequestMatch:        withRequestMatch,
	WithRequestMatchHandler: withRequestMatchHandler,

	GetReposIssuesByOwnerByRepoByIssueNumber:          GetReposIssuesByOwnerByRepoByIssueNumber,
	GetSearchIssues:                                   GetSearchIssues,
	GetReposIssuesCommentsByOwnerByRepoByIssueNumber:  GetReposIssuesCommentsByOwnerByRepoByIssueNumber,
	GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber: GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber,
	GetReposCommitsStatusesByOwnerByRepoByRef:         GetReposCommitsStatusesByOwnerByRepoByRef,
	GetReposCommitsStatusByOwnerByRepoByRef:           GetReposCommitsStatusByOwnerByRepoByRef,
	GetReposPullsByOwnerByRepo:                        GetReposPullsByOwnerByRepo,
	GetReposPullsByOwnerByRepoByPullNumber:            GetReposPullsByOwnerByRepoByPullNumber,
	GetReposPullsFilesByOwnerByRepoByPullNumber:       GetReposPullsFilesByOwnerByRepoByPullNumber,
	GetReposPullsReviewsByOwnerByRepoByPullNumber:     GetReposPullsReviewsByOwnerByRepoByPullNumber,

	PostReposIssuesByOwnerByRepo:                              PostReposIssuesByOwnerByRepo,
	PostReposIssuesCommentsByOwnerByRepoByIssueNumber:         PostReposIssuesCommentsByOwnerByRepoByIssueNumber,
	PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber:        PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber,
	PostReposPullsByOwnerByRepo:                               PostReposPullsByOwnerByRepo,
	PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber: PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber,

	PatchReposIssuesByOwnerByRepoByIssueNumber:                  PatchReposIssuesByOwnerByRepoByIssueNumber,
	PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber: PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber,
	PatchReposPullsByOwnerByRepoByPullNumber:                    PatchReposPullsByOwnerByRepoByPullNumber,

	PutReposPullsMergeByOwnerByRepoByPullNumber:        PutReposPullsMergeByOwnerByRepoByPullNumber,
	PutReposPullsUpdateBranchByOwnerByRepoByPullNumber: PutReposPullsUpdateBranchByOwnerByRepoByPullNumber,

	DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber: DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber,
}
