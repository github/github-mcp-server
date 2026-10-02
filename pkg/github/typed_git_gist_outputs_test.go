package github

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var typedGitGistProtocols = []string{inventory.ProtocolVersionMultiRoundTrip, "2025-11-25", "", "unknown"}

func gitGistTypedSession(t *testing.T, deps ToolDependencies, protocol string) (*mcp.ClientSession, map[string]*jsonschema.Resolved) {
	t.Helper()
	tr := translations.NullTranslationHelper
	tools := []inventory.ServerTool{GetRepositoryTree(tr), ListGists(tr), GetGist(tr), CreateGist(tr), UpdateGist(tr)}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).Build()
	require.NoError(t, err)
	server := mcp.NewServer(&mcp.Implementation{Name: "git-gist", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	inv.RegisterTools(context.Background(), server, deps)
	if protocol == "" || protocol == "unknown" {
		server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
			return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
				switch req := req.(type) {
				case *mcp.ListToolsRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				case *mcp.CallToolRequest:
					req.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: protocol}
				}
				return next(ctx, method, req)
			}
		})
	}
	version := protocol
	if version == "" || version == "unknown" {
		version = inventory.ProtocolVersionMultiRoundTrip
	}
	session := connectCommentVisibilityClient(t, server, version)
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, len(tools))
	schemas := make(map[string]*jsonschema.Resolved)
	for _, tool := range list.Tools {
		if protocol != inventory.ProtocolVersionMultiRoundTrip {
			assert.Nil(t, tool.OutputSchema, tool.Name)
			continue
		}
		require.NotNil(t, tool.OutputSchema, tool.Name)
		require.NoError(t, toolsnaps.Test(tool.Name+"_typed", *tool))
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

type gitGistCase struct {
	name   string
	args   map[string]any
	method string
	path   string
	query  string
	body   string
	status int
	text   string
}

func gitGistDeps(t *testing.T, current **gitGistCase, apiError bool) BaseDeps {
	t.Helper()
	client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc := *current
		if r.URL.Path == "/repos/owner/repo" {
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
			return
		}
		assert.Equal(t, tc.method, r.Method)
		assert.Equal(t, tc.path, r.URL.Path)
		assert.Equal(t, tc.query, r.URL.RawQuery)
		if apiError {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Forbidden"}`))
			return
		}
		w.WriteHeader(tc.status)
		_, _ = w.Write([]byte(tc.body))
	})}}
	return BaseDeps{Client: mustNewGHClient(t, client), ContentWindowSize: 100, RepoAccessCache: stubRepoAccessCache(nil, time.Minute)}
}

const (
	gistBody  = `{"id":"abc","description":"d","public":true,"comments":2,"html_url":"https://gist.github.com/abc","created_at":"2026-01-01T00:00:00Z","owner":{"login":"octocat","id":1},"files":{"a.txt":{"filename":"a.txt","size":3,"content":"hey"}}}`
	gistText  = `{"id":"abc","description":"d","public":true,"owner":{"login":"octocat","id":1},"files":{"a.txt":{"size":3,"filename":"a.txt","content":"hey"}},"comments":2,"html_url":"https://gist.github.com/abc","created_at":"2026-01-01T00:00:00Z"}`
	treeBody  = `{"sha":"t1","truncated":false,"tree":[{"path":"src/a.go","mode":"100644","type":"blob","sha":"s1","size":5,"url":"u1"},{"path":"src","mode":"040000","type":"tree","sha":"s2","url":"u2"},{"path":"README.md","mode":"100644","type":"blob","sha":"s3","size":1,"url":"u3"}]}`
	minimalOK = `{"id":"abc","url":"https://gist.github.com/abc"}`
)

func gitGistWireCases() []gitGistCase {
	return []gitGistCase{
		{name: "get_repository_tree", args: map[string]any{"owner": "owner", "repo": "repo", "recursive": true, "path_filter": "src"}, method: "GET", path: "/repos/owner/repo/git/trees/main", query: "recursive=1", body: treeBody, status: 200,
			text: `{"sha":"t1","truncated":false,"tree":[{"path":"src/a.go","type":"blob","size":5,"mode":"100644","sha":"s1","url":"u1"},{"path":"src","type":"tree","mode":"040000","sha":"s2","url":"u2"}],"tree_sha":"main","owner":"owner","repo":"repo","recursive":true,"count":2}`},
		{name: "get_repository_tree", args: map[string]any{"owner": "owner", "repo": "repo", "tree_sha": "abc", "path_filter": "none"}, method: "GET", path: "/repos/owner/repo/git/trees/abc", body: treeBody, status: 200,
			text: `{"sha":"t1","truncated":false,"tree":[],"tree_sha":"abc","owner":"owner","repo":"repo","recursive":false,"count":0}`},
		{name: "list_gists", args: map[string]any{"username": "octocat", "since": "2026-01-01T00:00:00Z", "page": "2e0", "perPage": "5"}, method: "GET", path: "/users/octocat/gists", query: "page=2&per_page=5&since=2026-01-01T00%3A00%3A00Z", body: `[` + gistBody + `]`, status: 200, text: `[` + gistText + `]`},
		{name: "list_gists", args: map[string]any{"page": 0, "perPage": 0}, method: "GET", path: "/gists", query: "page=1&per_page=30", body: `[]`, status: 200, text: `[]`},
		{name: "list_gists", args: nil, method: "GET", path: "/gists", query: "page=1&per_page=30", body: `null`, status: 200, text: `null`},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc", body: gistBody, status: 200, text: gistText},
		{name: "create_gist", args: map[string]any{"filename": "a.txt", "content": "hey", "description": "d", "public": true}, method: "POST", path: "/gists", body: gistBody, status: 201, text: minimalOK},
		{name: "update_gist", args: map[string]any{"gist_id": "abc", "filename": "a.txt", "content": "hey"}, method: "PATCH", path: "/gists/abc", body: gistBody, status: 200, text: minimalOK},
	}
}

func assertGitGistResult(t *testing.T, result *mcp.CallToolResult, schema *jsonschema.Resolved, text string, isError bool) {
	t.Helper()
	require.Equal(t, isError, result.IsError, mustMarshalJSON(t, result))
	require.Len(t, result.Content, 1)
	actual := getTextResult(t, result).Text
	if isError {
		assert.Equal(t, text, actual)
	} else {
		assert.JSONEq(t, text, actual)
	}
	if schema == nil || isError {
		assert.Nil(t, result.StructuredContent)
		return
	}
	var output any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
	require.NoError(t, schema.Validate(output))
	assert.JSONEq(t, text, mustMarshalJSON(t, output))
}

func TestTypedGitGistWireOutputs(t *testing.T) {
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *gitGistCase
			session, schemas := gitGistTypedSession(t, gitGistDeps(t, &current, false), protocol)
			for _, tc := range gitGistWireCases() {
				t.Run(tc.name+"/"+mustMarshalJSON(t, tc.args), func(t *testing.T) {
					current = &tc
					args := map[string]any{}
					if tc.name == "get_repository_tree" {
						args["owner"], args["repo"] = "owner", "repo"
					}
					maps.Copy(args, tc.args)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
					require.NoError(t, err)
					assertGitGistResult(t, result, schemas[tc.name], tc.text, false)
				})
			}
		})
	}
}

func TestTypedGitGistWireErrors(t *testing.T) {
	cases := []gitGistCase{
		{name: "get_repository_tree", args: map[string]any{"owner": "", "repo": "r"}, text: "missing required parameter: owner"},
		{name: "get_repository_tree", args: map[string]any{"owner": "o", "repo": "r", "recursive": "true"}, text: "parameter recursive is not of type bool, is string"},
		{name: "get_repository_tree", args: map[string]any{"owner": "o", "repo": "r", "tree_sha": 1}, text: "parameter tree_sha is not of type string, is float64"},
		{name: "list_gists", args: map[string]any{"username": 1}, text: "parameter username is not of type string, is float64"},
		{name: "list_gists", args: map[string]any{"since": "bad"}, text: `invalid since timestamp: invalid ISO 8601 timestamp: bad`},
		{name: "list_gists", args: map[string]any{"page": "bad"}, text: "parameter page is not a valid number: invalid numeric value: bad"},
		{name: "get_gist", args: map[string]any{}, text: "missing required parameter: gist_id"},
		{name: "create_gist", args: map[string]any{"content": "x"}, text: "missing required parameter: filename"},
		{name: "create_gist", args: map[string]any{"filename": "f", "content": "x", "public": "yes"}, text: "parameter public is not of type bool, is string"},
		{name: "update_gist", args: map[string]any{"gist_id": "g", "description": false, "filename": "f", "content": "c"}, text: "parameter description is not of type string, is bool"},
	}
	for _, protocol := range typedGitGistProtocols {
		t.Run("protocol="+protocol, func(t *testing.T) {
			var current *gitGistCase
			session, _ := gitGistTypedSession(t, gitGistDeps(t, &current, false), protocol)
			for _, tc := range cases {
				t.Run(tc.name+"/"+tc.text, func(t *testing.T) {
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
					require.NoError(t, err)
					if strings.HasPrefix(tc.text, "invalid since") {
						require.True(t, result.IsError)
						assert.Contains(t, getTextResult(t, result).Text, "invalid since timestamp")
						assert.Nil(t, result.StructuredContent)
						return
					}
					assertGitGistResult(t, result, nil, tc.text, true)
				})
			}
			session, _ = gitGistTypedSession(t, gitGistDeps(t, &current, true), protocol)
			for _, tc := range gitGistWireCases() {
				current = &tc
				args := map[string]any{"owner": "owner", "repo": "repo"}
				if tc.name != "get_repository_tree" {
					args = map[string]any{}
				}
				maps.Copy(args, tc.args)
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: args})
				require.NoError(t, err, tc.name+mustMarshalJSON(t, tc.args))
				require.True(t, result.IsError, tc.name)
				assert.Contains(t, getTextResult(t, result).Text, "Forbidden")
				assert.Nil(t, result.StructuredContent)
			}
		})
	}
}

func TestGitGistOutputSchemasRejectMismatches(t *testing.T) {
	for name, tc := range map[string]struct {
		schema *jsonschema.Schema
		bad    string
	}{
		"tree":   {treeOutputSchema(), `{"sha":"s","truncated":false,"tree":[{"path":"p","type":"blob","mode":"m","sha":"s","url":"u","size":null}],"tree_sha":"t","owner":"o","repo":"r","recursive":false,"count":1}`},
		"gist":   {gistOutputSchema(), `{"id":null}`},
		"list":   {gistListOutputSchema(), `[{"files":{"a":{"size":"3"}}}]`},
		"create": {gistMutationOutputSchema(), `{"id":"a"}`},
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := tc.schema.Resolve(nil)
			require.NoError(t, err)
			var value any
			require.NoError(t, json.Unmarshal([]byte(tc.bad), &value))
			require.Error(t, resolved.Validate(value))
		})
	}
}
