package github

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// GitHub API endpoint patterns f或testing
// 这些constants define URL patterns used in HTTP 模拟ing f或tests
const (
	// User endpoints
	GetUser                        = "GET /user"
	GetUsersByUsername             = "GET /users/{username}"
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
	ListCollaborators                    = "GET /repos/{owner}/{repo}/collaborators"

	// Git endpoints
	GetReposGitTreesByOwnerByRepoByTree        = "GET /repos/{owner}/{repo}/git/trees/{tree}"
	GetReposGitRefByOwnerByRepoByRef           = "GET /repos/{owner}/{repo}/git/ref/{ref:.*}"
	PostReposGitRefsByOwnerByRepo              = "POST /repos/{owner}/{repo}/git/refs"
	PatchReposGitRefsByOwnerByRepoByRef        = "PATCH /repos/{owner}/{repo}/git/refs/{ref:.*}"
	GetReposGitCommitsByOwnerByRepoByCommitSHA = "GET /repos/{owner}/{repo}/git/commits/{commit_sha}"
	PostReposGitCommitsByOwnerByRepo           = "POST /repos/{owner}/{repo}/git/commits"
	GetReposGitTagsByOwnerByRepoByTagSHA       = "GET /repos/{owner}/{repo}/git/tags/{tag_sha}"
	PostReposGitTreesByOwnerByRepo             = "POST /repos/{owner}/{repo}/git/trees"
	GetReposCommitsStatusByOwnerByRepoByRef    = "GET /repos/{owner}/{repo}/commits/{ref}/status"
	GetReposCommitsStatusesByOwnerByRepoByRef  = "GET /repos/{owner}/{repo}/commits/{ref}/statuses"
	GetReposCommitsCheckRunsByOwnerByRepoByRef = "GET /repos/{owner}/{repo}/commits/{ref}/check-runs"

	// Issues endpoints
	GetReposIssuesByOwnerByRepoByIssueNumber                    = "GET /repos/{owner}/{repo}/issues/{issue_number}"
	GetReposIssuesCommentByOwnerByRepoByCommentID               = "GET /repos/{owner}/{repo}/issues/comments/{comment_id}"
	GetReposIssuesCommentsByOwnerByRepoByIssueNumber            = "GET /repos/{owner}/{repo}/issues/{issue_number}/comments"
	PostReposIssuesByOwnerByRepo                                = "POST /repos/{owner}/{repo}/issues"
	PostReposIssuesCommentsByOwnerByRepoByIssueNumber           = "POST /repos/{owner}/{repo}/issues/{issue_number}/comments"
	PostReposIssuesReactionsByOwnerByRepoByIssueNumber          = "POST /repos/{owner}/{repo}/issues/{issue_number}/reactions"
	PatchReposIssuesByOwnerByRepoByIssueNumber                  = "PATCH /repos/{owner}/{repo}/issues/{issue_number}"
	GetReposIssuesSubIssuesByOwnerByRepoByIssueNumber           = "GET /repos/{owner}/{repo}/issues/{issue_number}/sub_issues"
	PostReposIssuesSubIssuesByOwnerByRepoByIssueNumber          = "POST /repos/{owner}/{repo}/issues/{issue_number}/sub_issues"
	DeleteReposIssuesSubIssueByOwnerByRepoByIssueNumber         = "DELETE /repos/{owner}/{repo}/issues/{issue_number}/sub_issue"
	PatchReposIssuesSubIssuesPriorityByOwnerByRepoByIssueNumber = "PATCH /repos/{owner}/{repo}/issues/{issue_number}/sub_issues/priority"
	PostReposIssuesCommentsReactionsByOwnerByRepoByCommentID    = "POST /repos/{owner}/{repo}/issues/comments/{comment_id}/reactions"
	DeleteReposIssuesIssueFieldValueByOwnerByRepoByIssueNumber  = "DELETE /repos/{owner}/{repo}/issues/{issue_number}/issue-field-values/{issue_field_id}"

	// Pull 请求 endpoints
	GetReposPullsByOwnerByRepo                                = "GET /repos/{owner}/{repo}/pulls"
	GetReposPullsByOwnerByRepoByPullNumber                    = "GET /repos/{owner}/{repo}/pulls/{pull_number}"
	GetReposPullsCommitsByOwnerByRepoByPullNumber             = "GET /repos/{owner}/{repo}/pulls/{pull_number}/commits"
	GetReposPullsFilesByOwnerByRepoByPullNumber               = "GET /repos/{owner}/{repo}/pulls/{pull_number}/files"
	GetReposPullsReviewsByOwnerByRepoByPullNumber             = "GET /repos/{owner}/{repo}/pulls/{pull_number}/reviews"
	PostReposPullsByOwnerByRepo                               = "POST /repos/{owner}/{repo}/pulls"
	PatchReposPullsByOwnerByRepoByPullNumber                  = "PATCH /repos/{owner}/{repo}/pulls/{pull_number}"
	PutReposPullsMergeByOwnerByRepoByPullNumber               = "PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge"
	PutReposPullsUpdateBranchByOwnerByRepoByPullNumber        = "PUT /repos/{owner}/{repo}/pulls/{pull_number}/update-branch"
	PostReposPullsRequestedReviewersByOwnerByRepoByPullNumber = "POST /repos/{owner}/{repo}/pulls/{pull_number}/requested_reviewers"
	PostReposPullsCommentsByOwnerByRepoByPullNumber           = "POST /repos/{owner}/{repo}/pulls/{pull_number}/comments"
	PostReposPullsCommentsReactionsByOwnerByRepoByCommentID   = "POST /repos/{owner}/{repo}/pulls/comments/{comment_id}/reactions"

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

	// Code quality endpoints
	GetReposCodeQualityFindingsByOwnerByRepoByFindingNumber = "GET /repos/{owner}/{repo}/code-quality/findings/{finding_number}"

	// Code scanning endpoints
	GetReposCodeScanningAlertsByOwnerByRepo              = "GET /repos/{owner}/{repo}/code-scanning/alerts"
	GetReposCodeScanningAlertsByOwnerByRepoByAlertNumber = "GET /repos/{owner}/{repo}/code-scanning/alerts/{alert_number}"

	// Secret scanning endpoints
	GetReposSecretScanningAlertsByOwnerByRepo              = "GET /repos/{owner}/{repo}/secret-scanning/alerts"                //nolint:gosec // False positive - this is 一个API endpoint pattern, 不a credential
	GetReposSecretScanningAlertsByOwnerByRepoByAlertNumber = "GET /repos/{owner}/{repo}/secret-scanning/alerts/{alert_number}" //nolint:gosec // False positive - this is 一个API endpoint pattern, 不a credential

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
	GetReposActionsRunsByOwnerByRepo                             = "GET /repos/{owner}/{repo}/actions/runs"
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
	GetSearchUsers        = "GET /search/users"
	GetSearchRepositories = "GET /search/repositories"
	GetSearchCommits      = "GET /search/commits"

	// Raw 内容 endpoints (用于 GitHub raw 内容 API, 不standard API)
	// 这些are used 使用raw 内容 客户端 that interacts with raw.githubuser内容.com
	GetRawReposContentsByOwnerByRepoByPath         = "GET /{owner}/{repo}/HEAD/{path:.*}"
	GetRawReposContentsByOwnerByRepoByBranchByPath = "GET /{owner}/{repo}/refs/heads/{branch}/{path:.*}"
	GetRawReposContentsByOwnerByRepoByTagByPath    = "GET /{owner}/{repo}/refs/tags/{tag}/{path:.*}"
	GetRawReposContentsByOwnerByRepoBySHAByPath    = "GET /{owner}/{repo}/{sha}/{path:.*}"

	// Projects (ProjectsV2) endpoints
	// Organization-scoped
	GetOrgsProjectsV2                          = "GET /orgs/{org}/projectsV2"
	GetOrgsProjectsV2ByProject                 = "GET /orgs/{org}/projectsV2/{project}"
	GetOrgsProjectsV2FieldsByProject           = "GET /orgs/{org}/projectsV2/{project}/fields"
	GetOrgsProjectsV2FieldsByProjectByFieldID  = "GET /orgs/{org}/projectsV2/{project}/fields/{field_id}"
	GetOrgsProjectsV2ItemsByProject            = "GET /orgs/{org}/projectsV2/{project}/items"
	GetOrgsProjectsV2ItemsByProjectByItemID    = "GET /orgs/{org}/projectsV2/{project}/items/{item_id}"
	PostOrgsProjectsV2ItemsByProject           = "POST /orgs/{org}/projectsV2/{project}/items"
	PatchOrgsProjectsV2ItemsByProjectByItemID  = "PATCH /orgs/{org}/projectsV2/{project}/items/{item_id}"
	DeleteOrgsProjectsV2ItemsByProjectByItemID = "DELETE /orgs/{org}/projectsV2/{project}/items/{item_id}"
	// User-scoped
	GetUsersProjectsV2ByUsername                          = "GET /users/{username}/projectsV2"
	GetUsersProjectsV2ByUsernameByProject                 = "GET /users/{username}/projectsV2/{project}"
	GetUsersProjectsV2FieldsByUsernameByProject           = "GET /users/{username}/projectsV2/{project}/fields"
	GetUsersProjectsV2FieldsByUsernameByProjectByFieldID  = "GET /users/{username}/projectsV2/{project}/fields/{field_id}"
	GetUsersProjectsV2ItemsByUsernameByProject            = "GET /users/{username}/projectsV2/{project}/items"
	GetUsersProjectsV2ItemsByUsernameByProjectByItemID    = "GET /users/{username}/projectsV2/{project}/items/{item_id}"
	PostUsersProjectsV2ItemsByUsernameByProject           = "POST /users/{username}/projectsV2/{project}/items"
	PatchUsersProjectsV2ItemsByUsernameByProjectByItemID  = "PATCH /users/{username}/projectsV2/{project}/items/{item_id}"
	DeleteUsersProjectsV2ItemsByUsernameByProjectByItemID = "DELETE /users/{username}/projectsV2/{project}/items/{item_id}"

	// Organization 议题 types endpoints
	GetOrgsIssueTypesByOrg = "GET /orgs/{org}/issue-types"
)

type expectations struct {
	path        string
	queryParams map[string]string
	requestBody any
}

// mustNewGHClient 创建s 一个新的 GitHub 客户端 f或testing.
// If httpClient is nil, 一个客户端 with no options is 创建d.
// test fails immediately if 客户端 creation fails.
func mustNewGHClient(t *testing.T, httpClient *http.Client) *gogithub.Client {
	t.Helper()
	var client *gogithub.Client
	var err error
	if httpClient == nil {
		client, err = gogithub.NewClient()
	} else {
		client, err = gogithub.NewClient(gogithub.WithHTTPClient(httpClient))
	}
	require.NoError(t, err)
	return client
}

// expect is 一个helper 函数 to 创建 一个partial 模拟 that expects various
// 请求 behaviors, such as 路径, query 参数, 和请求 body.
func expect(t *testing.T, e expectations) *partialMock {
	return &partialMock{
		t:                   t,
		expectedPath:        e.path,
		expectedQueryParams: e.queryParams,
		expectedRequestBody: e.requestBody,
	}
}

// expectPath is 一个helper 函数 to 创建 一个partial 模拟 that expects a
// 请求 使用given 路径, 使用ability to chain 一个响应 处理器.
func expectPath(t *testing.T, expectedPath string) *partialMock {
	return &partialMock{
		t:            t,
		expectedPath: expectedPath,
	}
}

// expectQueryParams is 一个helper 函数 to 创建 一个partial 模拟 that expects a
// 请求 使用given query 参数, 使用ability to chain 一个响应 处理器.
func expectQueryParams(t *testing.T, expectedQueryParams map[string]string) *partialMock {
	return &partialMock{
		t:                   t,
		expectedQueryParams: expectedQueryParams,
	}
}

// expectRequestBody is 一个helper 函数 to 创建 一个partial 模拟 that expects a
// 请求 使用given body, 使用ability to chain 一个响应 处理器.
func expectRequestBody(t *testing.T, expectedRequestBody any) *partialMock {
	return &partialMock{
		t:                   t,
		expectedRequestBody: expectedRequestBody,
	}
}

type partialMock struct {
	t *testing.T

	expectedPath           string
	expectedQueryParams    map[string]string
	expectedRequestBody    any
	expectedHeaderContains map[string]string
}

func (p *partialMock) withHeaders(headers map[string]string) *partialMock {
	p.expectedHeaderContains = headers
	return p
}

func (p *partialMock) andThen(responseHandler http.HandlerFunc) http.HandlerFunc {
	p.t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if p.expectedPath != "" {
			require.Equal(p.t, p.expectedPath, r.URL.Path)
		}

		if p.expectedQueryParams != nil {
			require.Equal(p.t, len(p.expectedQueryParams), len(r.URL.Query()))
			for k, v := range p.expectedQueryParams {
				require.Equal(p.t, v, r.URL.Query().Get(k))
			}
		}

		if p.expectedRequestBody != nil {
			var unmarshaledRequestBody any
			err := json.NewDecoder(r.Body).Decode(&unmarshaledRequestBody)
			require.NoError(p.t, err)

			require.Equal(p.t, p.expectedRequestBody, unmarshaledRequestBody)
		}

		if p.expectedHeaderContains != nil {
			for k, v := range p.expectedHeaderContains {
				require.Contains(p.t, r.Header.Get(k), v, "expected header %q to contain %q", k, v)
			}
		}

		responseHandler(w, r)
	}
}

// 模拟Response is 一个helper 函数 to 创建 一个模拟 HTTP 响应 处理器
// that 返回 一个specified status code 和marshaled body.
func mockResponse(t *testing.T, code int, body any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		// Some tests do 不expect to 返回 一个JSON object, such as fetching 一个raw 拉取请求 diff,
		// so allow strings to be 返回ed directly.
		s, ok := body.(string)
		if ok {
			_, _ = w.Write([]byte(s))
			return
		}

		b, err := json.Marshal(body)
		require.NoError(t, err)
		_, _ = w.Write(b)
	}
}

// 创建MCPRequest is 一个helper 函数 to 创建 一个MCP 请求 使用given 参数.
func createMCPRequest(args any) mcp.CallToolRequest {
	// convert args to map[string]interface{} 和serialize to JSON
	argsMap, ok := args.(map[string]any)
	if !ok {
		argsMap = make(map[string]any)
	}

	argsJSON, err := json.Marshal(argsMap)
	if err != nil {
		return mcp.CallToolRequest{}
	}

	jsonRawMessage := json.RawMessage(argsJSON)

	return mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: jsonRawMessage,
		},
	}
}

// Well-known MCP 客户端 names used in tests.
const (
	ClientNameVSCodeInsiders = "Visual Studio Code - Insiders"
	ClientNameVSCode         = "Visual Studio Code"
)

// 创建MCPRequestWithSession 创建s 一个CallToolRequest with 一个ServerSession
// that has given 客户端 name in its InitializeParams. When withUI is 真
// 会话 advertises MCP Apps UI support via 能力 extension.
func createMCPRequestWithSession(t *testing.T, clientName string, withUI bool, args any) mcp.CallToolRequest {
	t.Helper()

	argsMap, ok := args.(map[string]any)
	if !ok {
		argsMap = make(map[string]any)
	}
	argsJSON, err := json.Marshal(argsMap)
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)

	caps := &mcp.ClientCapabilities{}
	if withUI {
		caps.AddExtension("io.modelcontextprotocol/ui", map[string]any{
			"mimeTypes": []string{"text/html;profile=mcp-app"},
		})
	}

	st, _ := mcp.NewInMemoryTransports()
	session, err := srv.Connect(context.Background(), st, &mcp.ServerSessionOptions{
		State: &mcp.ServerSessionState{
			InitializeParams: &mcp.InitializeParams{
				ClientInfo:   &mcp.Implementation{Name: clientName},
				Capabilities: caps,
			},
		},
	})
	require.NoError(t, err)

	// Close unused 客户端-side transport 和会话
	t.Cleanup(func() {
		_ = session.Close()
	})

	return mcp.CallToolRequest{
		Session: session,
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(argsJSON),
		},
	}
}

// 获取TextResult is 一个helper 函数 that 返回 一个text 结果 from 一个工具 调用.
func getTextResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")
	return textContent
}

func getErrorResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	res := getTextResult(t, result)
	require.True(t, result.IsError, "expected tool call result to be an error")
	return res
}

// 获取TextResourceResult is 一个helper 函数 that 返回 一个text 结果 from 一个工具 调用.

// 获取BlobResourceResult is 一个helper 函数 that 返回 一个blob 结果 from 一个工具 调用.

func TestOptionalParamOK(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		paramName   string
		expectedVal any
		expectedOk  bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "present and correct type (string)",
			args:        map[string]any{"myParam": "hello"},
			paramName:   "myParam",
			expectedVal: "hello",
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present and correct type (bool)",
			args:        map[string]any{"myParam": true},
			paramName:   "myParam",
			expectedVal: true,
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present and correct type (number)",
			args:        map[string]any{"myParam": float64(123)},
			paramName:   "myParam",
			expectedVal: float64(123),
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present but wrong type (string expected, got bool)",
			args:        map[string]any{"myParam": true},
			paramName:   "myParam",
			expectedVal: "",   // Zero 值 f或string
			expectedOk:  true, // ok is 真 因为param exists
			expectError: true,
			errorMsg:    "parameter myParam is not of type string, is bool",
		},
		{
			name:        "present but wrong type (bool expected, got string)",
			args:        map[string]any{"myParam": "true"},
			paramName:   "myParam",
			expectedVal: false, // Zero 值 f或bool
			expectedOk:  true,  // ok is 真 因为param exists
			expectError: true,
			errorMsg:    "parameter myParam is not of type bool, is string",
		},
		{
			name:        "parameter not present",
			args:        map[string]any{"anotherParam": "value"},
			paramName:   "myParam",
			expectedVal: "", // Zero 值 f或string
			expectedOk:  false,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test with string type assertion
			if _, isString := tc.expectedVal.(string); isString || tc.errorMsg == "parameter myParam is not of type string, is bool" {
				val, ok, err := OptionalParamOK[string](tc.args, tc.paramName)
				if tc.expectError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.errorMsg)
					assert.Equal(t, tc.expectedOk, ok)   // Check ok even on 错误
					assert.Equal(t, tc.expectedVal, val) // Check zero 值 on 错误
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}

			// Test with bool type assertion
			if _, isBool := tc.expectedVal.(bool); isBool || tc.errorMsg == "parameter myParam is not of type bool, is string" {
				val, ok, err := OptionalParamOK[bool](tc.args, tc.paramName)
				if tc.expectError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.errorMsg)
					assert.Equal(t, tc.expectedOk, ok)   // Check ok even on 错误
					assert.Equal(t, tc.expectedVal, val) // Check zero 值 on 错误
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}

			// Test with float64 type assertion (f或number case)
			if _, isFloat := tc.expectedVal.(float64); isFloat {
				val, ok, err := OptionalParamOK[float64](tc.args, tc.paramName)
				if tc.expectError {
					// 此case shouldn't happen f或float64 在defined tests
					require.Fail(t, "Unexpected error case for float64")
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}
		})
	}
}

func getResourceResult(t *testing.T, result *mcp.CallToolResult) *mcp.ResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, &mcp.EmbeddedResource{}, content)
	resource, ok := content.(*mcp.EmbeddedResource)
	require.True(t, ok, "expected content to be of type EmbeddedResource")

	require.IsType(t, &mcp.ResourceContents{}, resource.Resource)
	return resource.Resource
}

// MockRoundTripper is 一个模拟 HTTP transport using testify/模拟
type MockRoundTripper struct {
	testifymock.Mock
	handlers map[string]http.HandlerFunc
}

// NewMockRoundTripper 创建s 一个新的 模拟 round tripper
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		handlers: make(map[string]http.HandlerFunc),
	}
}

// RoundTrip implements http.RoundTripper interface
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Normalize 请求 路径 和method f或matching
	key := req.Method + " " + req.URL.Path

	// Check if we have 一个specific 处理器 f或this 请求
	if handler, ok := m.handlers[key]; ok {
		// Use httptest.ResponseRecorder to capture 处理器's 响应
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
		}, nil
	}

	// F所有back to 模拟.Mock assertions if defined
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// On registers 一个expectation using testify/模拟
func (m *MockRoundTripper) OnRequest(method, path string, handler http.HandlerFunc) *MockRoundTripper {
	key := method + " " + path
	m.handlers[key] = handler
	return m
}

// NewMockHTTPClient 创建s 一个HTTP 客户端 with 一个模拟 transport
func NewMockHTTPClient() (*http.Client, *MockRoundTripper) {
	transport := NewMockRoundTripper()
	client := &http.Client{Transport: transport}
	return client, transport
}

// 响应Recorder is 一个simple 响应 recorder 用于模拟 transport
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

// matchPath 检查s if 一个请求 路径 matches 一个pattern (supports simple wildcards)
func matchPath(pattern, path string) bool {
	// Simple exact match f或now
	if pattern == path {
		return true
	}

	// Support f或路径 参数 like /repos/{owner}/{repo}/议题/{议题_number}
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	// Handle patterns with wildcard 路径 like {路径:.*}
	if len(patternParts) > 0 {
		lastPart := patternParts[len(patternParts)-1]
		if strings.HasPrefix(lastPart, "{") && strings.Contains(lastPart, ":") && strings.HasSuffix(lastPart, "}") {
			// 此is 一个wildcard pattern like {路径:.*}
			// Check if 所有parts before wildcard match
			if len(pathParts) < len(patternParts)-1 {
				return false
			}
			for i := range len(patternParts) - 1 {
				if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
					continue // Path 参数 matches anything
				}
				if patternParts[i] != pathParts[i] {
					return false
				}
			}
			return true
		}
	}

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := range patternParts {
		// Check if this is 一个路径 参数 (enclosed in {})
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			continue // Path 参数 match anything
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// executeHandler executes 一个HTTP 处理器 和返回 响应
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

// MockHTTPClientWithHandler 创建s 一个HTTP 客户端 with 一个单个 处理器 函数
func MockHTTPClientWithHandler(handler http.HandlerFunc) *http.Client {
	handlers := map[string]http.HandlerFunc{
		"": handler, // Empty key acts as catch-all
	}
	return MockHTTPClientWithHandlers(handlers)
}

// MockHTTPClientWithHandlers 创建s 一个HTTP 客户端 with 多个 处理器s f或different 路径s
func MockHTTPClientWithHandlers(handlers map[string]http.HandlerFunc) *http.Client {
	transport := &multiHandlerTransport{handlers: handlers}
	return &http.Client{Transport: transport}
}

// Compatibility helpers to replace github.com/migueleliasweb/go-github-模拟 in tests
type EndpointPattern string

type MockBackendOption func(map[string]http.HandlerFunc)

func parseEndpointPattern(p EndpointPattern) (string, string) {
	parts := strings.SplitN(string(p), " ", 2)
	if len(parts) != 2 {
		return http.MethodGet, string(p)
	}
	return parts[0], parts[1]
}

func WithRequestMatch(pattern EndpointPattern, response any) MockBackendOption {
	return func(handlers map[string]http.HandlerFunc) {
		method, path := parseEndpointPattern(pattern)
		handlers[method+" "+path] = func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			switch v := response.(type) {
			case string:
				_, _ = w.Write([]byte(v))
			case []byte:
				_, _ = w.Write(v)
			default:
				data, err := json.Marshal(v)
				if err == nil {
					_, _ = w.Write(data)
				}
			}
		}
	}
}

func WithRequestMatchHandler(pattern EndpointPattern, handler http.HandlerFunc) MockBackendOption {
	return func(handlers map[string]http.HandlerFunc) {
		method, path := parseEndpointPattern(pattern)
		handlers[method+" "+path] = handler
	}
}

func NewMockedHTTPClient(options ...MockBackendOption) *http.Client {
	handlers := map[string]http.HandlerFunc{}
	for _, opt := range options {
		if opt != nil {
			opt(handlers)
		}
	}
	return MockHTTPClientWithHandlers(handlers)
}

func MustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

type multiHandlerTransport struct {
	handlers map[string]http.HandlerFunc
}

func (m *multiHandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check f或catch-所有处理器
	if handler, ok := m.handlers[""]; ok {
		return executeHandler(handler, req), nil
	}

	// Try to find 一个处理器 f或this 请求
	key := req.Method + " " + req.URL.Path

	// First try exact match
	if handler, ok := m.handlers[key]; ok {
		return executeHandler(handler, req), nil
	}

	// Then try pattern matching, prioritizing patterns without wildcards
	// 此is important 因为wildcard patterns like /{owner}/{repo}/{sha}/{路径:.*}
	// can incorrectly match API 路径s like /repos/owner/repo/pulls/42
	var wildcardPattern string
	var wildcardHandler http.HandlerFunc

	for pattern, handler := range m.handlers {
		if pattern == "" {
			continue // Skip catch-all
		}
		parts := strings.SplitN(pattern, " ", 2)
		if len(parts) != 2 {
			continue
		}
		method, pathPattern := parts[0], parts[1]
		if req.Method != method {
			continue
		}

		// Check if this pattern contains 一个wildcard like {路径:.*}
		isWildcard := strings.Contains(pathPattern, ":.*}")

		if matchPath(pathPattern, req.URL.Path) {
			if isWildcard {
				// Save wildcard match f或later, prefer non-wildcard patterns
				wildcardPattern = pattern
				wildcardHandler = handler
			} else {
				// Non-wildcard pattern takes priority
				return executeHandler(handler, req), nil
			}
		}
	}

	// If we found 一个wildcard match 但no specific match, use it
	if wildcardPattern != "" && wildcardHandler != nil {
		return executeHandler(wildcardHandler, req), nil
	}

	// No 处理器 found
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		Request:    req,
	}, nil
}

// extractPathParams extracts 路径 参数 from 一个URL 路径 given 一个pattern
func extractPathParams(pattern, path string) map[string]string {
	params := make(map[string]string)
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return params
	}

	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			paramName := strings.Trim(patternParts[i], "{}")
			params[paramName] = pathParts[i]
		}
	}

	return params
}

// ParseRequestPath is 一个helper to extract 路径 参数
func ParseRequestPath(t *testing.T, req *http.Request, pattern string) url.Values {
	t.Helper()
	params := extractPathParams(pattern, req.URL.Path)
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}
