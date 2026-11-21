package ghmcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoAllowlist(t *testing.T) {
	list, err := parseRepoAllowlist(nil)
	require.NoError(t, err)
	assert.Nil(t, list)

	list, err = parseRepoAllowlist([]string{" ", ""})
	require.NoError(t, err)
	assert.Nil(t, list)

	list, err = parseRepoAllowlist([]string{"Octo-Org/*", " octocat/Hello-World ", "acme/svc-*"})
	require.NoError(t, err)
	require.NotNil(t, list)
	assert.Equal(t, "octo-org/*, octocat/hello-world, acme/svc-*", list.String())

	for _, bad := range []string{"octocat", "/repo", "owner/", "a/b/c", "*/repo", "owner/[abc"} {
		_, err := parseRepoAllowlist([]string{bad})
		assert.Error(t, err, bad)
	}
}

func TestRepoAllowlistCheckToolCall(t *testing.T) {
	list, err := parseRepoAllowlist([]string{"octo-org/*", "octocat/hello-world", "acme/svc-*"})
	require.NoError(t, err)

	tests := []struct {
		name    string
		tool    string
		args    string
		allowed bool
	}{
		{"exact repo", "get_file_contents", `{"owner":"octocat","repo":"hello-world","path":"README.md"}`, true},
		{"case insensitive", "get_file_contents", `{"owner":"OctoCat","repo":"Hello-World"}`, true},
		{"other repo same owner", "get_file_contents", `{"owner":"octocat","repo":"spoon-knife"}`, false},
		{"owner wildcard", "list_issues", `{"owner":"octo-org","repo":"anything"}`, true},
		{"repo glob", "list_issues", `{"owner":"acme","repo":"svc-billing"}`, true},
		{"repo glob miss", "list_issues", `{"owner":"acme","repo":"website"}`, false},
		{"other owner", "list_issues", `{"owner":"someone","repo":"public"}`, false},
		{"owner only needs wildcard", "projects_list", `{"owner":"octo-org","method":"list_projects"}`, true},
		{"owner only without wildcard", "projects_list", `{"owner":"octocat","method":"list_projects"}`, false},
		{"project item in other repo", "projects_write", `{"owner":"octo-org","item_owner":"someone","item_repo":"x"}`, false},
		{"org advisories", "list_org_repository_security_advisories", `{"org":"octo-org"}`, true},
		{"org advisories other org", "list_org_repository_security_advisories", `{"org":"acme"}`, false},
		{"unscoped notifications", "list_notifications", `{}`, false},
		{"scoped notifications", "list_notifications", `{"owner":"octocat","repo":"hello-world"}`, true},
		{"notification by id", "get_notification_details", `{"notificationID":"1"}`, false},
		{"exempt tool", "get_me", ``, true},
		{"exempt search users", "search_users", `{"query":"location:berlin"}`, true},
		{"search with repo qualifier", "search_code", `{"query":"func main repo:octocat/hello-world"}`, true},
		{"search with org qualifier", "search_code", `{"query":"org:octo-org TODO"}`, true},
		{"search without scope", "search_code", `{"query":"func main"}`, false},
		{"search with outside repo", "search_code", `{"query":"repo:octocat/hello-world repo:evil/leak"}`, false},
		{"search negated qualifier ignored", "search_code", `{"query":"org:octo-org -repo:evil/leak"}`, true},
		{"search org without wildcard", "search_repositories", `{"query":"user:octocat"}`, false},
		{"search issues scoped by args", "search_issues", `{"query":"is:open","owner":"octocat","repo":"hello-world"}`, true},
		{"search issues args plus outside qualifier", "search_issues", `{"query":"repo:evil/leak","owner":"octocat","repo":"hello-world"}`, false},
		{"create repository in allowed org", "create_repository", `{"name":"new","organization":"octo-org"}`, true},
		{"create repository for user", "create_repository", `{"name":"new"}`, false},
		{"fork into allowed org", "fork_repository", `{"owner":"octocat","repo":"hello-world","organization":"octo-org"}`, true},
		{"fork into other org", "fork_repository", `{"owner":"octocat","repo":"hello-world","organization":"acme"}`, false},
		{"fork into personal account", "fork_repository", `{"owner":"octocat","repo":"hello-world"}`, false},
		{"owner-only notifications", "list_notifications", `{"owner":"octo-org"}`, false},
		{"owner-only mark all read", "mark_all_notifications_read", `{"owner":"octo-org"}`, false},
		{"notification thread by id", "dismiss_notification", `{"threadID":"1","state":"read"}`, false},
		{"notification thread with decoy repo", "get_notification_details", `{"notificationID":"1","owner":"octocat","repo":"hello-world"}`, false},
		{"notification subscription by id", "manage_notification_subscription", `{"notificationID":"1","action":"watch"}`, false},
		{"repository level needs pair", "repository_ruleset_read", `{"level":"repository","method":"list","owner":"octo-org"}`, false},
		{"repository level", "repository_ruleset_read", `{"level":"repository","method":"list","owner":"octocat","repo":"hello-world"}`, true},
		{"organization level", "custom_properties_read", `{"level":"organization","org":"octo-org"}`, true},
		{"organization level other org", "custom_properties_read", `{"level":"organization","org":"acme"}`, false},
		{"organization level with decoy repo", "custom_properties_read", `{"level":"organization","owner":"octocat","repo":"hello-world"}`, false},
		{"enterprise level", "repository_ruleset_read", `{"level":"enterprise","method":"list","enterprise":"x","owner":"octocat","repo":"hello-world"}`, false},
		{"parent issue same repo", "issue_write", `{"method":"create","owner":"octocat","repo":"hello-world","parent_issue_number":1,"parent_repo":"hello-world"}`, true},
		{"parent issue other repo", "issue_write", `{"method":"create","owner":"octocat","repo":"hello-world","parent_issue_number":1,"parent_repo":"spoon-knife"}`, false},
		{"parent issue other owner", "issue_write", `{"method":"create","owner":"octocat","repo":"hello-world","parent_owner":"someone","parent_repo":"hello-world"}`, false},
		{"related issue defaults", "issue_dependency_write", `{"owner":"octocat","repo":"hello-world","related_owner":"octocat"}`, true},
		{"related issue other repo", "issue_dependency_write", `{"owner":"octocat","repo":"hello-world","related_repo":"spoon-knife"}`, false},
		{"project items listing", "projects_list", `{"owner":"octo-org","method":"list_project_items","project_number":1}`, false},
		{"project item by id", "projects_get", `{"owner":"octo-org","method":"get_project_item","project_number":1,"item_id":5}`, false},
		{"project view by node id", "projects_get", `{"method":"get_project_view","view_id":"PVTV_1"}`, false},
		{"project status update by node id", "projects_get", `{"owner":"octo-org","method":"get_project_status_update","status_update_id":"PVTSU_1"}`, false},
		{"project get", "projects_get", `{"owner":"octo-org","method":"get_project","project_number":1}`, true},
		{"add project item in allowed repo", "projects_write", `{"owner":"octo-org","method":"add_project_item","project_number":1,"item_type":"issue","item_owner":"octocat","item_repo":"hello-world","issue_number":1}`, true},
		{"update project item by id", "projects_write", `{"owner":"octo-org","method":"update_project_item","project_number":1,"item_id":5}`, false},
		{"update project item by issue", "projects_write", `{"owner":"octo-org","method":"update_project_item","project_number":1,"item_owner":"octo-org","item_repo":"x","issue_number":2}`, true},
		{"batch items all allowed", "projects_write", `{"owner":"octo-org","method":"update_project_items","project_number":1,"items":[{"item_owner":"octocat","item_repo":"hello-world","issue_number":1},{"item_owner":"acme","item_repo":"svc-a","issue_number":2}]}`, true},
		{"batch item outside", "projects_write", `{"owner":"octo-org","method":"update_project_items","project_number":1,"items":[{"item_owner":"octocat","item_repo":"hello-world","issue_number":1},{"item_owner":"acme","item_repo":"web","issue_number":2}]}`, false},
		{"batch item by node id", "projects_write", `{"owner":"octo-org","method":"update_project_items","project_number":1,"items":[{"node_id":"PVTI_1"}]}`, false},
		{"batch without items", "projects_write", `{"owner":"octo-org","method":"update_project_items","project_number":1}`, false},
		{"search issues args with negated repo in query", "search_issues", `{"query":"is:open -repo:octocat/other","owner":"octocat","repo":"hello-world"}`, false},
		{"search issues args with allowed positive repo", "search_issues", `{"query":"repo:octocat/hello-world -repo:octocat/other","owner":"octocat","repo":"hello-world"}`, true},
		{"search pull requests args only", "search_pull_requests", `{"query":"is:open","owner":"octocat","repo":"hello-world"}`, true},
		{"search code ignores owner args", "search_code", `{"query":"func main","owner":"octocat","repo":"hello-world"}`, false},
		{"search quoted qualifier is free text", "search_issues", `{"query":""repo:octocat/hello-world" crash"}`, false},
		{"search quoted outside repo", "search_issues", `{"query":""repo:evil/leak" repo:octocat/hello-world"}`, false},
		{"search OR branch", "search_code", `{"query":"repo:octocat/hello-world foo OR bar"}`, false},
		{"search parentheses", "search_code", `{"query":"(repo:octocat/hello-world OR repo:evil/leak) foo"}`, false},
		{"search NOT", "search_code", `{"query":"repo:octocat/hello-world NOT foo"}`, false},
		{"search negated owner ignored", "search_code", `{"query":"org:octo-org -user:someone TODO"}`, true},
		{"invalid json", "get_file_contents", `{`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason := list.checkToolCall(tc.tool, json.RawMessage(tc.args))
			if tc.allowed {
				assert.Empty(t, reason)
			} else {
				assert.NotEmpty(t, reason)
			}
		})
	}
}

func TestRepoAllowlistCheckResourceURI(t *testing.T) {
	list, err := parseRepoAllowlist([]string{"octocat/hello-world"})
	require.NoError(t, err)

	assert.Empty(t, list.checkResourceURI("repo://octocat/hello-world/contents/README.md"))
	assert.Empty(t, list.checkResourceURI("repo://OctoCat/Hello-World/refs/heads/main/contents/"))
	assert.NotEmpty(t, list.checkResourceURI("repo://octocat/spoon-knife/contents/README.md"))
	assert.NotEmpty(t, list.checkResourceURI("https://example.com/x"))
	assert.Empty(t, list.checkResourceURI("ui://github-mcp-server/get-me"))
	assert.Empty(t, list.checkResourceURI("ui://github-mcp-server/issue-write"))
}

func TestRepoAllowlistCheckCompletion(t *testing.T) {
	list, err := parseRepoAllowlist([]string{"octo-org/*", "octocat/hello-world"})
	require.NoError(t, err)

	resource := &mcp.CompleteReference{Type: "ref/resource", URI: "repo://{owner}/{repo}/contents{/path*}"}
	complete := func(arg string, resolved map[string]string) string {
		return list.checkCompletion(&mcp.CompleteParams{
			Ref:      resource,
			Argument: mcp.CompleteParamsArgument{Name: arg, Value: ""},
			Context:  &mcp.CompleteContext{Arguments: resolved},
		})
	}

	assert.Empty(t, list.checkCompletion(nil))
	assert.Empty(t, list.checkCompletion(&mcp.CompleteParams{Ref: &mcp.CompleteReference{Type: "ref/prompt", Name: "x"}}))
	assert.NotEmpty(t, complete("owner", nil))
	assert.NotEmpty(t, complete("repo", map[string]string{"owner": "octocat"}))
	assert.Empty(t, complete("repo", map[string]string{"owner": "octo-org"}))
	assert.NotEmpty(t, complete("branch", map[string]string{"owner": "octocat"}))
	assert.NotEmpty(t, complete("branch", map[string]string{"owner": "octocat", "repo": "spoon-knife"}))
	assert.Empty(t, complete("path", map[string]string{"owner": "octocat", "repo": "hello-world"}))
}

func TestAllowedReposMiddleware(t *testing.T) {
	list, err := parseRepoAllowlist([]string{"octocat/hello-world"})
	require.NoError(t, err)

	called := 0
	next := func(_ context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		called++
		return &mcp.CallToolResult{}, nil
	}
	handler := allowedReposMiddleware(list)(next)

	allowed := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "get_file_contents",
		Arguments: json.RawMessage(`{"owner":"octocat","repo":"hello-world"}`),
	}}
	res, err := handler(context.Background(), "tools/call", allowed)
	require.NoError(t, err)
	assert.False(t, res.(*mcp.CallToolResult).IsError)
	assert.Equal(t, 1, called)

	denied := &mcp.CallToolRequest{Params: &mcp.CallToolParamsRaw{
		Name:      "get_file_contents",
		Arguments: json.RawMessage(`{"owner":"octocat","repo":"spoon-knife"}`),
	}}
	res, err = handler(context.Background(), "tools/call", denied)
	require.NoError(t, err)
	result := res.(*mcp.CallToolResult)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "octocat/spoon-knife is not in the allowed repositories")
	assert.Equal(t, 1, called)

	_, err = handler(context.Background(), "resources/read", &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{
		URI: "repo://octocat/spoon-knife/contents/README.md",
	}})
	require.Error(t, err)
	assert.Equal(t, 1, called)

	_, err = handler(context.Background(), "resources/read", &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{
		URI: "repo://octocat/hello-world/contents/README.md",
	}})
	require.NoError(t, err)
	assert.Equal(t, 2, called)

	_, err = handler(context.Background(), "resources/read", &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{
		URI: "ui://github-mcp-server/get-me",
	}})
	require.NoError(t, err)
	assert.Equal(t, 3, called)

	resource := &mcp.CompleteReference{Type: "ref/resource", URI: "repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}"}
	_, err = handler(context.Background(), "completion/complete", &mcp.CompleteRequest{Params: &mcp.CompleteParams{
		Ref:      resource,
		Argument: mcp.CompleteParamsArgument{Name: "branch"},
		Context:  &mcp.CompleteContext{Arguments: map[string]string{"owner": "octocat", "repo": "spoon-knife"}},
	}})
	require.Error(t, err)
	assert.Equal(t, 3, called)

	_, err = handler(context.Background(), "completion/complete", &mcp.CompleteRequest{Params: &mcp.CompleteParams{
		Ref:      resource,
		Argument: mcp.CompleteParamsArgument{Name: "branch"},
		Context:  &mcp.CompleteContext{Arguments: map[string]string{"owner": "octocat", "repo": "hello-world"}},
	}})
	require.NoError(t, err)
	assert.Equal(t, 4, called)
}
