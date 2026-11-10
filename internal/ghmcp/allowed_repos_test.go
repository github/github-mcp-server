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
}
