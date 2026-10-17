package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
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
		for _, canonical := range tools {
			if canonical.Tool.Name == tool.Name {
				assert.JSONEq(t, mustMarshalJSON(t, canonical.Tool.OutputSchema), mustMarshalJSON(t, tool.OutputSchema))
			}
		}
		var schema jsonschema.Schema
		require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
		resolved, err := schema.Resolve(nil)
		require.NoError(t, err)
		schemas[tool.Name] = resolved
	}
	return session, schemas
}

type gitGistCase struct {
	name       string
	args       map[string]any
	method     string
	path       string
	query      string
	body       string
	status     int
	text       string
	structured string
}

func gitGistDeps(t *testing.T, current **gitGistCase, apiError bool, private ...bool) BaseDeps {
	t.Helper()
	client := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc := *current
		if r.URL.Path == "/repos/owner/repo" {
			_, _ = fmt.Fprintf(w, `{"default_branch":"main","private":%t}`, len(private) > 0 && private[0])
			return
		}
		assert.Equal(t, tc.method, r.Method)
		assert.Equal(t, tc.path, r.URL.Path)
		assert.Equal(t, tc.query, r.URL.RawQuery)
		switch tc.name {
		case "create_gist":
			var payload struct {
				Description string `json:"description"`
				Public      bool   `json:"public"`
				Files       map[string]struct {
					Content string `json:"content"`
				} `json:"files"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			assert.Equal(t, tc.args["description"], payload.Description)
			assert.Equal(t, tc.args["public"], payload.Public)
			assert.Equal(t, tc.args["content"], payload.Files[tc.args["filename"].(string)].Content)
		case "update_gist":
			var payload struct {
				Description *string `json:"description"`
				Files       map[string]struct {
					Filename string `json:"filename"`
					Content  string `json:"content"`
				} `json:"files"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			if description, present := tc.args["description"]; present {
				require.NotNil(t, payload.Description)
				assert.Equal(t, description, *payload.Description)
			} else {
				assert.Nil(t, payload.Description)
			}
			file := payload.Files[tc.args["filename"].(string)]
			assert.Equal(t, tc.args["filename"], file.Filename)
			assert.Equal(t, tc.args["content"], file.Content)
		}
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
	gistBody       = `{"id":"abc","description":"d","public":true,"comments":2,"html_url":"https://gist.github.com/abc","created_at":"2026-01-01T00:00:00Z","owner":{"login":"octocat","id":1,"url":"https://api.github.com/users/octocat","html_url":"https://github.com/octocat","node_id":"user-node"},"files":{"a.txt":{"filename":"a.txt","size":3,"content":"hey","raw_url":"https://gist.githubusercontent.com/raw"}},"node_id":"gist-node","git_pull_url":"https://gist.github.com/abc.git","git_push_url":"https://gist.github.com/abc.git"}`
	gistText       = `{"id":"abc","description":"d","public":true,"owner":{"login":"octocat","id":1,"node_id":"user-node","html_url":"https://github.com/octocat","url":"https://api.github.com/users/octocat"},"files":{"a.txt":{"size":3,"filename":"a.txt","raw_url":"https://gist.githubusercontent.com/raw","content":"hey"}},"comments":2,"html_url":"https://gist.github.com/abc","git_pull_url":"https://gist.github.com/abc.git","git_push_url":"https://gist.github.com/abc.git","created_at":"2026-01-01T00:00:00Z","node_id":"gist-node"}`
	gistStructured = `{"id":"abc","description":"d","public":true,"owner":{"login":"octocat","id":1,"profile_url":"https://github.com/octocat"},"files":{"a.txt":{"size":3,"filename":"a.txt","content":"hey","raw_url":"https://gist.githubusercontent.com/raw"}},"comments":2,"html_url":"https://gist.github.com/abc","git_pull_url":"https://gist.github.com/abc.git","created_at":"2026-01-01T00:00:00Z"}`
	treeBody       = `{"sha":"t1","truncated":false,"tree":[{"path":"src/a.go","mode":"100644","type":"blob","sha":"s1","size":5,"url":"u1"},{"path":"src","mode":"040000","type":"tree","sha":"s2","url":"u2"},{"path":"README.md","mode":"100644","type":"blob","sha":"s3","size":1,"url":"u3"}]}`
	minimalOK      = `{"id":"abc","url":"https://gist.github.com/abc"}`
)

func gitGistWireCases() []gitGistCase {
	return []gitGistCase{
		{name: "get_repository_tree", args: map[string]any{"owner": "owner", "repo": "repo", "recursive": true, "path_filter": "src"}, method: "GET", path: "/repos/owner/repo/git/trees/main", query: "recursive=1", body: treeBody, status: 200,
			text:       `{"sha":"t1","truncated":false,"tree":[{"path":"src/a.go","type":"blob","size":5,"mode":"100644","sha":"s1","url":"u1"},{"path":"src","type":"tree","mode":"040000","sha":"s2","url":"u2"}],"tree_sha":"main","owner":"owner","repo":"repo","recursive":true,"count":2}`,
			structured: `{"sha":"t1","truncated":false,"tree":[{"path":"src/a.go","type":"blob","size":5,"mode":"100644","sha":"s1"},{"path":"src","type":"tree","mode":"040000","sha":"s2"}],"tree_sha":"main","owner":"owner","repo":"repo","recursive":true,"count":2}`},
		{name: "get_repository_tree", args: map[string]any{"owner": "owner", "repo": "repo", "tree_sha": "abc", "path_filter": "none"}, method: "GET", path: "/repos/owner/repo/git/trees/abc", body: treeBody, status: 200,
			text: `{"sha":"t1","truncated":false,"tree":[],"tree_sha":"abc","owner":"owner","repo":"repo","recursive":false,"count":0}`},
		{name: "list_gists", args: map[string]any{"username": "octocat", "since": "2026-01-01T00:00:00Z", "page": "2e0", "perPage": "5"}, method: "GET", path: "/users/octocat/gists", query: "page=2&per_page=5&since=2026-01-01T00%3A00%3A00Z", body: `[` + gistBody + `]`, status: 200, text: `[` + gistText + `]`, structured: `[` + gistStructured + `]`},
		{name: "list_gists", args: map[string]any{"page": 0, "perPage": 0}, method: "GET", path: "/gists", query: "page=1&per_page=30", body: `[]`, status: 200, text: `[]`},
		{name: "list_gists", args: map[string]any{"page": -1, "perPage": 101}, method: "GET", path: "/gists", query: "page=-1&per_page=101", body: `[]`, status: 200, text: `[]`},
		{name: "list_gists", args: map[string]any{"page": "-1", "perPage": "-2"}, method: "GET", path: "/gists", query: "page=-1&per_page=-2", body: `[]`, status: 200, text: `[]`},
		{name: "list_gists", args: nil, method: "GET", path: "/gists", query: "page=1&per_page=30", body: `null`, status: 200, text: `null`},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc", body: gistBody, status: 200, text: gistText, structured: gistStructured},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc", body: `{"public":false,"description":"","files":{"empty":{"filename":"","content":"","size":0,"language":"","type":""}}}`, status: 200,
			text: `{"description":"","public":false,"files":{"empty":{"size":0,"filename":"","language":"","type":"","content":""}}}`},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc", body: `{}`, status: 200, text: `{}`},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc", body: `null`, status: 200, text: `null`},
		{name: "get_gist", args: map[string]any{"gist_id": "abc"}, method: "GET", path: "/gists/abc",
			body: `{"created_at":"2026-01-01T00:00:00.123456789+02:00","updated_at":"2026-01-02T12:00:00Z"}`, status: 200,
			text: `{"created_at":"2026-01-01T00:00:00.123456789+02:00","updated_at":"2026-01-02T12:00:00Z"}`},
		{name: "list_gists", method: "GET", path: "/gists", query: "page=1&per_page=30", body: `[null,{}]`, status: 200, text: `[null,{}]`},
		{name: "create_gist", args: map[string]any{"filename": "a.txt", "content": "hey", "description": "d", "public": true}, method: "POST", path: "/gists", body: gistBody, status: 201, text: minimalOK},
		{name: "update_gist", args: map[string]any{"gist_id": "abc", "filename": "a.txt", "content": "hey"}, method: "PATCH", path: "/gists/abc", body: gistBody, status: 200, text: minimalOK},
		{name: "update_gist", args: map[string]any{"gist_id": "abc", "filename": "a.txt", "content": "hey", "description": ""}, method: "PATCH", path: "/gists/abc", body: gistBody, status: 200, text: minimalOK},
	}
}

func assertGitGistResult(t *testing.T, result *mcp.CallToolResult, schema *jsonschema.Resolved, text string, isError bool, structured ...string) {
	t.Helper()
	require.Equal(t, isError, result.IsError, mustMarshalJSON(t, result))
	require.Len(t, result.Content, 1)
	actual := getTextResult(t, result).Text
	if schema == nil || isError {
		assert.Equal(t, text, actual)
		assert.Nil(t, result.StructuredContent)
		return
	}
	var output any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
	require.NoError(t, schema.Validate(output))
	expected := text
	if len(structured) > 0 && structured[0] != "" {
		expected = structured[0]
	}
	assert.JSONEq(t, expected, mustMarshalJSON(t, output))
	assert.JSONEq(t, expected, actual)
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
					assertGitGistResult(t, result, schemas[tc.name], tc.text, false, tc.structured)
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
		{name: "list_gists", args: map[string]any{"since": "bad"}, text: `invalid since timestamp: invalid ISO 8601 timestamp: bad (supported formats: YYYY-MM-DDThh:mm:ssZ or YYYY-MM-DD)`},
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
		"tree":      {treeOutputSchema(), `{"sha":"s","truncated":false,"tree":[{"path":"p","type":"blob","mode":"m","sha":"s","url":"u","size":null}],"tree_sha":"t","owner":"o","repo":"r","recursive":false,"count":1}`},
		"gist":      {gistOutputSchema(), `{"id":null}`},
		"list":      {gistListOutputSchema(), `[{"files":{"a":{"size":"3"}}}]`},
		"create":    {gistMutationOutputSchema(), `{"id":"a"}`},
		"api_url":   {gistOutputSchema(), `{"git_push_url":"https://gist.github.com/abc.git"}`},
		"node_id":   {gistOutputSchema(), `{"owner":{"login":"octocat","node_id":"node"}}`},
		"raw_url":   {gistOutputSchema(), `{"files":{"a":{"raw_url":null}}}`},
		"timestamp": {gistOutputSchema(), `{"created_at":42}`},
		"tree_type": {treeOutputSchema(), `{"sha":"s","truncated":false,"tree":[{"path":"p","type":"invalid","mode":"100644","sha":"s"}],"tree_sha":"t","owner":"o","repo":"r","recursive":false,"count":1}`},
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

func TestGitGistInputMetadata(t *testing.T) {
	tool := ListGists(translations.NullTranslationHelper)
	schema := tool.Tool.InputSchema.(*jsonschema.Schema)
	require.Equal(t, new(1.0), schema.Properties["page"].Minimum)
	require.Equal(t, new(1.0), schema.Properties["perPage"].Minimum)
	require.Equal(t, new(100.0), schema.Properties["perPage"].Maximum)
	assert.Equal(t, "Page number for pagination (min 1)", schema.Properties["page"].Description)
	assert.Equal(t, "Results per page for pagination (min 1, max 100)", schema.Properties["perPage"].Description)
	for _, schemaFor := range []func() *jsonschema.Schema{
		treeOutputSchema, gistOutputSchema, gistListOutputSchema, gistMutationOutputSchema,
	} {
		assert.Same(t, schemaFor(), schemaFor())
	}
}

func TestListGistsLegacyPaginationBounds(t *testing.T) {
	for _, tc := range gitGistWireCases() {
		if tc.name != "list_gists" || (tc.query != "page=-1&per_page=101" && tc.query != "page=-1&per_page=-2") {
			continue
		}
		t.Run(tc.query, func(t *testing.T) {
			current := &tc
			deps := gitGistDeps(t, &current, false)
			tool := ListGists(translations.NullTranslationHelper)
			ctx := ContextWithDeps(context.Background(), deps)
			request := createMCPRequest(tc.args)
			result, err := tool.HandlerFunc(deps)(ctx, &request)
			require.NoError(t, err)
			assertGitGistResult(t, result, nil, "[]", false)
		})
	}
}

func TestGitGistIFCScopesAndLockdown(t *testing.T) {
	tr := translations.NullTranslationHelper
	for _, tc := range []struct {
		tool  inventory.ServerTool
		scope inventory.ScopeAccess
	}{
		{GetRepositoryTree(tr), scopes.PublicRead(scopes.Repo)},
		{ListGists(tr), scopes.NoScopes()},
		{GetGist(tr), scopes.NoScopes()},
		{CreateGist(tr), scopes.RequireAll(scopes.Gist)},
		{UpdateGist(tr), scopes.RequireAll(scopes.Gist)},
	} {
		actual := tc.tool.ScopeAccess
		assert.Equal(t, tc.scope.Scopes, actual.Scopes, tc.tool.Tool.Name)
		assert.Equal(t, tc.scope.Dynamic, actual.Dynamic)
		for _, active := range [][]string{nil, {"gist"}, {"repo"}} {
			if tc.scope.Visible == nil {
				assert.Nil(t, actual.Visible)
			} else {
				require.NotNil(t, actual.Visible)
				assert.Equal(t, tc.scope.Visible(active), actual.Visible(active))
			}
			if tc.scope.Challenge == nil {
				assert.Nil(t, actual.Challenge)
			} else {
				require.NotNil(t, actual.Challenge)
				args := map[string]any{"owner": "owner", "repo": "repo"}
				assert.Equal(t, tc.scope.Challenge(args, active), actual.Challenge(args, active))
			}
		}
	}

	for _, protocol := range typedGitGistProtocols {
		for _, private := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, lockdown := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/private=%t/ifc=%t/lockdown=%t", protocol, private, enabled, lockdown), func(t *testing.T) {
						var current *gitGistCase
						deps := gitGistDeps(t, &current, false, private)
						deps.Flags = stubFeatureFlags(map[string]bool{"lockdown-mode": lockdown})
						if enabled {
							deps.featureChecker = featureCheckerFor(FeatureFlagIFCLabels)
						}
						session, schemas := gitGistTypedSession(t, deps, protocol)
						seen := make(map[string]bool)
						for _, tc := range gitGistWireCases() {
							if seen[tc.name] {
								continue
							}
							seen[tc.name] = true
							current = &tc
							result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
							require.NoError(t, err)
							assertGitGistResult(t, result, schemas[tc.name], tc.text, false, tc.structured)
							if !enabled || tc.name == "create_gist" || tc.name == "update_gist" {
								assert.Nil(t, result.Meta["ifc"])
								continue
							}
							expected := ifc.LabelGist()
							switch tc.name {
							case "get_repository_tree":
								expected = ifc.LabelCommitContents(private)
							case "list_gists":
								expected = ifc.LabelGistList()
							}
							assert.JSONEq(t, mustMarshalJSON(t, expected), mustMarshalJSON(t, result.Meta["ifc"]))
						}
					})
				}
			}
		}
	}
}
