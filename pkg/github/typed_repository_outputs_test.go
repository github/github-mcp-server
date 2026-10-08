package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/requeststate"
	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func repositoryOutputFixtures() (*github.Reference, *github.Commit, *github.RepositoryRelease) {
	ref := &github.Reference{Ref: new("refs/heads/main"), URL: new("ref-url"),
		Object: &github.GitObject{Type: new("commit"), SHA: new("base"), URL: new("commit-url")}}
	commit := &github.Commit{SHA: new("new"), Message: new("change"),
		HTMLURL: new("https://github.com/owner/repo/commit/new"), URL: new("api-commit-url"),
		Author: &github.CommitAuthor{Date: &github.Timestamp{Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}},
		Tree:   &github.Tree{SHA: new("tree")}, Parents: []*github.Commit{{SHA: new("base")}}}
	release := &github.RepositoryRelease{ID: 7, TagName: "v1", Name: new("<b>Release</b>"),
		HTMLURL: "https://github.com/owner/repo/releases/tag/v1",
		URL:     "api-release-url", AssetsURL: "api-assets-url", UploadURL: "api-upload-url",
		Body: new("No\u200btes"), CreatedAt: github.Timestamp{Time: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		Author: &github.User{Login: new("octo")}, Assets: []*github.ReleaseAsset{{
			Name: new("app.zip"), Size: new(3), Digest: new("sha256:abc"),
		}}}
	return ref, commit, release
}

func typedRepositoryDeps(t *testing.T) BaseDeps {
	t.Helper()
	ref, commit, release := repositoryOutputFixtures()
	handlers := map[string]http.HandlerFunc{
		"GET /repos/{owner}/{repo}": mockResponse(t, http.StatusOK, &github.Repository{ID: new(int64(1)), DefaultBranch: new("main")}),
		"POST /user/repos": func(w http.ResponseWriter, r *http.Request) {
			var body github.Repository
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "created", body.GetName())
			if body.GetDescription() == "public" {
				assert.False(t, body.GetPrivate())
			} else {
				assert.True(t, body.GetPrivate(), "omitted private retains the secure default")
			}
			mockResponse(t, http.StatusCreated, &github.Repository{ID: new(int64(8)), HTMLURL: new("repo-url")})(w, r)
		},
		"GET /repos/{owner}/{repo}/contents/{path...}": func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, "/forbidden"):
				mockResponse(t, http.StatusForbidden, map[string]string{"message": "Forbidden"})(w, r)
			case strings.HasSuffix(r.URL.Path, "/new"):
				mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"})(w, r)
			case strings.HasSuffix(r.URL.Path, "/directory"):
				mockResponse(t, http.StatusOK, []*github.RepositoryContent{{
					Name: new("file"), Type: new("file"), SHA: new("sha"), Size: new(0),
					URL: new("api-content-url"), GitURL: new("api-git-url"),
					HTMLURL:     new("https://github.com/owner/repo/blob/main/file"),
					DownloadURL: new("https://raw.githubusercontent.com/owner/repo/main/file"),
				}})(w, r)
			case strings.HasSuffix(r.URL.Path, "/submodule-directory"):
				mockResponse(t, http.StatusOK, []*github.RepositoryContent{{
					Name: new("lib"), Path: new("lib"), Type: new("file"), SHA: new("sha"),
					SubmoduleGitURL: new("https://github.com/owner/lib.git"),
				}})(w, r)
			case strings.HasSuffix(r.URL.Path, "/empty-directory"):
				mockResponse(t, http.StatusOK, []*github.RepositoryContent{})(w, r)
			case strings.HasSuffix(r.URL.Path, "/submodule"):
				mockResponse(t, http.StatusOK, &github.RepositoryContent{Type: new("submodule"), SHA: new("abc"), SubmoduleGitURL: new("git-url")})(w, r)
			case strings.HasSuffix(r.URL.Path, "/symlink"):
				mockResponse(t, http.StatusOK, &github.RepositoryContent{Type: new("symlink"), SHA: new("abc"), Target: new("../outside")})(w, r)
			case strings.HasSuffix(r.URL.Path, "/missing"):
				mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"})(w, r)
			default:
				data := []byte("hello\n")
				if strings.HasSuffix(r.URL.Path, "/binary") {
					data = []byte{0, 1, 2}
				}
				if strings.HasSuffix(r.URL.Path, "/empty") {
					data = []byte{}
				}
				size := len(data)
				if strings.HasSuffix(r.URL.Path, "/large") {
					size = 1024 * 1024
				}
				mockResponse(t, http.StatusOK, &github.RepositoryContent{
					Type: new("file"), Name: new("file"), SHA: new(gitBlobSHA(data)),
					Size: new(size), Encoding: new("base64"),
					Content: new(base64.StdEncoding.EncodeToString(data)), DownloadURL: new("download-url"),
				})(w, r)
			}
		},
		"PUT /repos/{owner}/{repo}/contents/{path...}": func(w http.ResponseWriter, r *http.Request) {
			var body github.RepositoryContentFileOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, []byte("plain text"), body.Content, "mutation input is not double base64 encoded")
			assert.Equal(t, "main", body.GetBranch())
			mockResponse(t, http.StatusCreated, &github.RepositoryContentResponse{
				Content: &github.RepositoryContent{Path: new("new"), SHA: new("blob")},
				Commit:  *commit,
			})(w, r)
		},
		GetReposGitRefByOwnerByRepoByRef: func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/forbidden") {
				mockResponse(t, http.StatusForbidden, map[string]string{"message": "Forbidden"})(w, r)
				return
			}
			if strings.HasSuffix(r.URL.Path, "/annotated") {
				mockResponse(t, http.StatusOK, &github.Reference{Ref: new("refs/tags/annotated"), Object: &github.GitObject{
					Type: new("tag"), SHA: new("tag-sha"),
				}})(w, r)
				return
			}
			mockResponse(t, http.StatusOK, ref)(w, r)
		},
		"GET /repos/{owner}/{repo}/git/tags/{tag_sha}": mockResponse(t, http.StatusOK, &github.Tag{
			Tag: new("annotated"), Message: new("Tag message"), SHA: new("tag-sha"),
			Tagger: &github.CommitAuthor{Date: commit.Author.Date}, URL: new("api-tag-url"),
		}),
		"GET /repos/{owner}/{repo}/git/commits/{commit_sha}": mockResponse(t, http.StatusOK, commit),
		"POST /repos/{owner}/{repo}/git/trees": func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Entries []github.TreeEntry `json:"tree"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Len(t, body.Entries, 1)
			assert.Equal(t, "file", body.Entries[0].GetPath())
			if body.Entries[0].Content != nil {
				assert.Equal(t, "", body.Entries[0].GetContent(), "empty push content is valid")
			} else {
				assert.Nil(t, body.Entries[0].SHA, "delete retains null SHA semantics")
			}
			mockResponse(t, http.StatusCreated, &github.Tree{SHA: new("tree")})(w, r)
		},
		"POST /repos/{owner}/{repo}/git/commits": mockResponse(t, http.StatusCreated, commit),
		"POST /repos/{owner}/{repo}/git/refs": func(w http.ResponseWriter, r *http.Request) {
			var body github.CreateRef
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "refs/heads/feature", body.Ref)
			assert.Equal(t, "base", body.SHA)
			mockResponse(t, http.StatusCreated, ref)(w, r)
		},
		PatchReposGitRefsByOwnerByRepoByRef: func(w http.ResponseWriter, r *http.Request) {
			var body github.UpdateRef
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.NotNil(t, body.Force)
			assert.False(t, *body.Force)
			assert.Equal(t, "new", body.SHA)
			mockResponse(t, http.StatusOK, ref)(w, r)
		},
		"GET /repos/{owner}/{repo}/git/trees/{tree_sha}": mockResponse(t, http.StatusOK, &github.Tree{Entries: []*github.TreeEntry{{
			Path: new("nested/missing"), Type: new("blob"),
		}}}),
		"POST /repos/{owner}/{repo}/forks": func(w http.ResponseWriter, r *http.Request) {
			var body github.RepositoryCreateForkOptions
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if body.Organization == "pending" {
				mockResponse(t, http.StatusAccepted, map[string]string{"message": "pending"})(w, r)
				return
			}
			mockResponse(t, http.StatusAccepted, &github.Repository{ID: new(int64(9)), HTMLURL: new("fork-url")})(w, r)
		},
		"GET /repos/{owner}/{repo}/releases/latest":     mockResponse(t, http.StatusOK, release),
		"GET /repos/{owner}/{repo}/releases/tags/{tag}": mockResponse(t, http.StatusOK, release),
		"GET /repos/{owner}/{repo}/releases": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				assert.Equal(t, "7", r.URL.Query().Get("per_page"))
			} else {
				assert.Equal(t, "1", r.URL.Query().Get("page"))
				assert.Equal(t, "30", r.URL.Query().Get("per_page"))
			}
			if strings.Contains(r.URL.Path, "/empty/") {
				mockResponse(t, http.StatusOK, []*github.RepositoryRelease{})(w, r)
				return
			}
			mockResponse(t, http.StatusOK, []*github.RepositoryRelease{release})(w, r)
		},
		"GET /user/starred": mockResponse(t, http.StatusOK, []*github.StarredRepository{{Repository: &github.Repository{
			ID: new(int64(1)), Name: new("repo"), FullName: new("owner/repo"),
		}}}),
		"GET /users/{user}/starred":           mockResponse(t, http.StatusOK, []*github.StarredRepository{}),
		"PUT /user/starred/{owner}/{repo}":    mockResponse(t, http.StatusNoContent, nil),
		"DELETE /user/starred/{owner}/{repo}": mockResponse(t, http.StatusNoContent, nil),
		"GET /repos/{owner}/{repo}/collaborators": func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			assert.Equal(t, "7", r.URL.Query().Get("per_page"))
			assert.Equal(t, "outside", r.URL.Query().Get("affiliation"))
			if strings.Contains(r.URL.Path, "/empty/") {
				mockResponse(t, http.StatusOK, []*github.User{})(w, r)
				return
			}
			w.Header().Set("Link", `<https://api.github.com/repos/owner/repo/collaborators?page=3>; rel="next"`)
			mockResponse(t, http.StatusOK, []*github.User{{Login: new("octo"), ID: new(int64(1)), RoleName: new("write")}})(w, r)
		},
	}
	gql := &http.Client{Transport: recorderTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"repository":{"defaultBranchRef":{"name":"main"},"object":{"__typename":"Commit","blame":{"ranges":[{"startingLine":1,"endingLine":3,"age":0,"commit":{"oid":"abc","message":"Message","committedDate":"2026-01-02T00:00:00Z","author":{"name":"Author","email":"author@example.com","user":null}}}]}}}}}`))
	})}}
	return BaseDeps{
		Client: mustNewGHClient(t, MockHTTPClientWithHandlers(handlers)), GQLClient: githubv4.NewClient(gql),
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
	}
}

func TestTypedRepositoryOutputs(t *testing.T) {
	translate := translations.NullTranslationHelper
	tools := []inventory.ServerTool{
		CreateOrUpdateFile(translate), CreateRepository(translate), DeleteRepository(translate),
		GetFileContents(translate), ForkRepository(translate), DeleteFile(translate), CreateBranch(translate),
		PushFiles(translate), GetTag(translate), ListReleases(translate), GetLatestRelease(translate),
		GetReleaseByTag(translate), ListStarredRepositories(translate), StarRepository(translate),
		UnstarRepository(translate), GetFileBlame(translate), ListRepositoryCollaborators(translate),
	}
	inputContracts := make(map[string]string, len(tools))
	for _, tool := range tools {
		inputContracts[tool.Tool.Name] = mustMarshalJSON(t, tool.Tool.InputSchema)
	}
	inv, err := inventory.NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).
		WithFeatureChecker(func(context.Context, string) (bool, error) { return true, nil }).Build()
	require.NoError(t, err)
	coord := func(extra map[string]any) map[string]any {
		args := map[string]any{"owner": "owner", "repo": "repo"}
		maps.Copy(args, extra)
		return args
	}
	calls := []struct {
		name string
		args map[string]any
	}{
		{"create_repository", map[string]any{"name": "created"}},
		{"create_repository", map[string]any{"name": "created", "private": false, "description": "public"}},
		{"create_or_update_file", coord(map[string]any{"path": "new", "content": "plain text", "message": "change", "branch": "main"})},
		{"create_branch", coord(map[string]any{"branch": "feature"})},
		{"push_files", coord(map[string]any{"branch": "main", "message": "change", "files": []any{map[string]any{"path": "file", "content": ""}}})},
		{"delete_file", coord(map[string]any{"path": "file", "message": "delete", "branch": "main"})},
		{"fork_repository", coord(nil)},
		{"fork_repository", coord(map[string]any{"organization": "pending"})},
		{"get_tag", coord(map[string]any{"tag": "lightweight"})},
		{"get_tag", coord(map[string]any{"tag": "annotated"})},
		{"list_releases", coord(map[string]any{"page": "2.0", "perPage": "7"})},
		{"list_releases", coord(nil)},
		{"list_releases", coord(map[string]any{"page": 0, "perPage": 0})},
		{"list_releases", coord(map[string]any{"page": "0", "perPage": "0"})},
		{"list_releases", coord(map[string]any{"page": "2", "perPage": "7", "fields": []any{"id", "draft"}})},
		{"list_releases", coord(map[string]any{"repo": "empty", "page": "2", "perPage": "7"})},
		{"get_latest_release", coord(nil)},
		{"get_release_by_tag", coord(map[string]any{"tag": "v1"})},
		{"list_starred_repositories", map[string]any{}},
		{"list_starred_repositories", map[string]any{"username": "empty"}},
		{"star_repository", coord(nil)},
		{"unstar_repository", coord(nil)},
		{"list_repository_collaborators", coord(map[string]any{"page": "2", "perPage": "7", "affiliation": "outside"})},
		{"list_repository_collaborators", coord(map[string]any{"repo": "empty", "page": "2", "perPage": "7", "affiliation": "outside"})},
		{"get_file_blame", coord(map[string]any{"path": "file", "start_line": "2", "end_line": "3", "perPage": "1"})},
		{"get_file_blame", coord(map[string]any{"path": "file", "after": encodeBlameCursor(1)})},
	}
	for _, path := range []string{"text", "empty", "binary", "large", "submodule", "symlink", "directory", "submodule-directory", "empty-directory", "missing"} {
		calls = append(calls, struct {
			name string
			args map[string]any
		}{"get_file_contents", coord(map[string]any{"path": path, "sha": strings.Repeat("a", 40)})})
	}
	calls = append(calls, struct {
		name string
		args map[string]any
	}{"get_file_contents", coord(map[string]any{"path": "directory", "sha": strings.Repeat("a", 40), "fields": []any{"name"}})})
	calls = append(calls, struct {
		name string
		args map[string]any
	}{"get_file_contents", coord(map[string]any{"path": "directory", "sha": strings.Repeat("a", 40), "fields": []any{"size", "html_url", "url"}})})

	for _, test := range []struct {
		protocol string
		lockdown bool
	}{
		{"2025-11-25", false}, {inventory.ProtocolVersionMultiRoundTrip, false}, {"", false},
		{"2025-11-25", true}, {inventory.ProtocolVersionMultiRoundTrip, true}, {"", true},
	} {
		protocol := test.protocol
		t.Run(fmt.Sprintf("protocol=%s/lockdown=%t", protocol, test.lockdown), func(t *testing.T) {
			deps := typedRepositoryDeps(t)
			deps.Flags.LockdownMode = test.lockdown
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-repository-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			inv.RegisterTools(context.Background(), server, deps)
			if protocol == "" {
				server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
					return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
						switch request := req.(type) {
						case *mcp.ListToolsRequest:
							request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
						case *mcp.CallToolRequest:
							request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: ""}
						}
						return next(ctx, method, req)
					}
				})
			}
			version := protocol
			if version == "" {
				version = inventory.ProtocolVersionMultiRoundTrip
			}
			session := connectCommentVisibilityClient(t, server, version)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, 16, "deletion remains hidden without form elicitation")
			schemas := make(map[string]*jsonschema.Resolved)
			for _, tool := range list.Tools {
				var inputSchema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.InputSchema)), &inputSchema))
				for _, property := range inputSchema.Properties {
					delete(property.Extra, "x-mcp-header")
				}
				assert.JSONEq(t, inputContracts[tool.Name], mustMarshalJSON(t, inputSchema), "input metadata remains unchanged")
				if protocol != inventory.ProtocolVersionMultiRoundTrip {
					assert.Nil(t, tool.OutputSchema, tool.Name)
					continue
				}
				require.NotNil(t, tool.OutputSchema, tool.Name)
				require.NoError(t, toolsnaps.Test(tool.Name, *tool))
				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, tool.OutputSchema)), &schema))
				schemas[tool.Name], err = schema.Resolve(nil)
				require.NoError(t, err, tool.Name)
			}
			for _, call := range calls {
				t.Run(call.name+"/"+mustMarshalJSON(t, call.args), func(t *testing.T) {
					// Direct invocation provides the legacy content baseline without SDK
					// serialization. Compare every content block, not just a JSON proxy.
					var tool inventory.ServerTool
					for _, candidate := range tools {
						if candidate.Tool.Name == call.name {
							tool = candidate
							break
						}
					}
					request := createMCPRequest(call.args)
					legacy, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
					require.NoError(t, err)
					require.False(t, legacy.IsError, legacy)
					result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
					require.NoError(t, err)
					require.False(t, result.IsError, result)
					assert.JSONEq(t, mustMarshalJSON(t, legacy.Meta["ifc"]), mustMarshalJSON(t, result.Meta["ifc"]), "IFC labels remain protocol-independent")
					if protocol != inventory.ProtocolVersionMultiRoundTrip {
						assert.Equal(t, mustMarshalJSON(t, legacy.Content), mustMarshalJSON(t, result.Content), "legacy content blocks remain byte-exact")
						assertLegacyRepositoryText(t, call.name, call.args, result)
						assert.Nil(t, result.StructuredContent)
						return
					}
					require.NotNil(t, result.StructuredContent)
					if hasOnlyTextContent(result.Content) {
						assert.JSONEq(t, mustMarshalJSON(t, result.StructuredContent), getTextResult(t, result).Text,
							"modern JSON text is the compact structured output")
					} else {
						assert.Equal(t, mustMarshalJSON(t, legacy.Content), mustMarshalJSON(t, result.Content),
							"non-text resource blocks preserve legacy content")
					}
					var output any
					require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
					require.NoError(t, schemas[call.name].Validate(output))
					first := result.Content[0].(*mcp.TextContent).Text
					switch {
					case call.name == "get_file_contents" && !strings.HasPrefix(first, "["):
						assert.JSONEq(t, `{"content":`+mustMarshalJSON(t, legacy.Content)+`}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "get_file_contents":
						var entries []*RepositoryDirectoryEntryOutput
						require.NoError(t, json.Unmarshal([]byte(first), &entries))
						assert.JSONEq(t, mustMarshalJSON(t, entries), mustMarshalJSON(t, result.StructuredContent))
						if call.args["fields"] == nil {
							assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "api-content-url")
						}
						assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "api-git-url")
						if call.args["path"] == "submodule-directory" {
							assert.JSONEq(t, `[{"name":"lib","path":"lib","type":"file","sha":"sha","submodule_git_url":"https://github.com/owner/lib.git"}]`, mustMarshalJSON(t, result.StructuredContent))
						}
						if call.args["path"] == "directory" && call.args["fields"] != nil && len(call.args["fields"].([]any)) == 3 {
							assert.JSONEq(t, `[{"size":0,"html_url":"https://github.com/owner/repo/blob/main/file","url":"api-content-url"}]`, mustMarshalJSON(t, result.StructuredContent))
						}
					case call.name == "create_branch" || call.name == "push_files":
						assert.JSONEq(t, `{"ref":"refs/heads/main","object":{"type":"commit","sha":"base"}}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "get_tag" && call.args["tag"] == "lightweight":
						assert.JSONEq(t, `{"ref":"refs/heads/main","object":{"type":"commit","sha":"base"}}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "get_tag":
						assert.JSONEq(t, `{"tag":"annotated","sha":"tag-sha","message":"Tag message","tagger":{"date":"2026-01-02T03:04:05Z"}}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "delete_file":
						assert.JSONEq(t, `{"commit":{"sha":"new","message":"change","html_url":"https://github.com/owner/repo/commit/new","author":{"date":"2026-01-02T03:04:05Z"},"tree_sha":"tree","parents":["base"]},"content":null}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "get_latest_release" || call.name == "get_release_by_tag":
						assert.NotContains(t, mustMarshalJSON(t, result.StructuredContent), "\u200b")
						assert.JSONEq(t, `{"id":7,"tag_name":"v1","name":"Release","body":"Notes","html_url":"https://github.com/owner/repo/releases/tag/v1","prerelease":false,"draft":false,"author":{"login":"octo"}}`, mustMarshalJSON(t, result.StructuredContent))
					case call.name == "star_repository" || call.name == "unstar_repository" || (call.name == "fork_repository" && call.args["organization"] == "pending"):
						assert.JSONEq(t, mustMarshalJSON(t, RepositoryMessageOutput{Message: getTextResult(t, legacy).Text}), mustMarshalJSON(t, result.StructuredContent))
					default:
					}
					for _, tool := range tools {
						assert.Equal(t, inputContracts[tool.Tool.Name], mustMarshalJSON(t, tool.Tool.InputSchema), "registration/calls must not mutate tool contracts")
					}
				})
			}
			for _, call := range []struct {
				name string
				args map[string]any
			}{
				{"create_repository", map[string]any{"name": ""}},
				{"get_tag", coord(map[string]any{"tag": ""})},
				{"get_tag", coord(map[string]any{"tag": "forbidden"})},
				{"get_file_contents", coord(map[string]any{"path": "forbidden", "sha": strings.Repeat("a", 40)})},
				{"get_file_blame", coord(map[string]any{"path": "../file"})},
				{"push_files", coord(map[string]any{"branch": "main", "message": "change", "files": "invalid"})},
				{"delete_file", coord(map[string]any{"branch": "main", "path": "../file", "message": "delete"})},
			} {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				require.NoError(t, err)
				require.True(t, result.IsError, result)
				assert.Nil(t, result.StructuredContent, "errors never return success-shaped output")
				assert.NotContains(t, result.Meta, "ifc", "errors never receive success IFC labels")
			}
		})
	}
}

func hasOnlyTextContent(content []mcp.Content) bool {
	for _, item := range content {
		if _, ok := item.(*mcp.TextContent); !ok {
			return false
		}
	}
	return true
}

func assertLegacyRepositoryText(t *testing.T, name string, args map[string]any, result *mcp.CallToolResult) {
	t.Helper()
	switch name {
	case "create_branch", "push_files":
		ref, _, _ := repositoryOutputFixtures()
		assert.Equal(t, mustMarshalJSON(t, ref), getTextResult(t, result).Text)
	case "get_tag":
		ref, commit, _ := repositoryOutputFixtures()
		if args["tag"] == "lightweight" {
			assert.Equal(t, mustMarshalJSON(t, ref), getTextResult(t, result).Text)
			return
		}
		tag := &github.Tag{
			Tag: new("annotated"), Message: new("Tag message"), SHA: new("tag-sha"),
			Tagger: &github.CommitAuthor{Date: commit.Author.Date}, URL: new("api-tag-url"),
		}
		assert.Equal(t, mustMarshalJSON(t, tag), getTextResult(t, result).Text)
	case "delete_file":
		_, commit, _ := repositoryOutputFixtures()
		assert.Equal(t, mustMarshalJSON(t, map[string]any{"commit": commit, "content": nil}), getTextResult(t, result).Text)
	case "get_latest_release", "get_release_by_tag":
		_, _, release := repositoryOutputFixtures()
		sanitizeReleaseNameAndBody(release)
		assert.Equal(t, mustMarshalJSON(t, release), getTextResult(t, result).Text)
	case "list_repository_collaborators":
		if args["repo"] != "empty" {
			assert.Equal(t, `{"firstPage":0,"items":[{"login":"octo","id":1,"role_name":"write"}],"lastPage":0,"nextPage":3,"prevPage":0}`, getTextResult(t, result).Text)
		}
	case "get_file_contents":
		fields, ok := args["fields"].([]any)
		if args["path"] == "directory" && ok && len(fields) == 3 {
			assert.Equal(t, `[{"html_url":"https://github.com/owner/repo/blob/main/file","size":0,"url":"api-content-url"}]`, getTextResult(t, result).Text)
		}
	}
}

func TestTypedRepositoryDeletionOutput(t *testing.T) {
	deps := typedRepositoryDeps(t)
	sealer, err := requeststate.NewRandom()
	require.NoError(t, err)
	deps.StateSealer = sealer
	deps.Client = mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposByOwnerByRepo:    mockResponse(t, http.StatusOK, &github.Repository{ID: new(int64(1))}),
		DeleteReposByOwnerByRepo: mockResponse(t, http.StatusNoContent, nil),
	}))
	tool := DeleteRepository(translations.NullTranslationHelper)
	server := mcp.NewServer(&mcp.Implementation{Name: "repository-deletion-test", Version: "v1"}, nil)
	server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
	tool.RegisterFunc(server, deps)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "deletion-client", Version: "v1"}, &mcp.ClientOptions{
		ElicitationHandler: func(_ context.Context, _ *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{deleteRepositoryConfirmationField: "owner/repo"}}, nil
		},
	})
	session, err := client.Connect(context.Background(), commentVisibilityProtocolTransport{clientTransport, inventory.ProtocolVersionMultiRoundTrip}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	list, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, list.Tools, 1)
	require.NoError(t, toolsnaps.Test("delete_repository", *list.Tools[0]))
	request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
	pending, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.Len(t, pending.InputRequests, 1)
	assert.Nil(t, pending.StructuredContent, "confirmation is not a completed deletion")
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "delete_repository", Arguments: map[string]any{"owner": "owner", "repo": "repo"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.Equal(t, `{"message":"Repository owner/repo was deleted."}`, getTextResult(t, result).Text)
	assert.JSONEq(t, `{"message":"Repository owner/repo was deleted."}`, mustMarshalJSON(t, result.StructuredContent))
	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, list.Tools[0].OutputSchema)), &schema))
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	var output any
	require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, result.StructuredContent)), &output))
	require.NoError(t, resolved.Validate(output))
}

func TestRepositoryOutputUnions(t *testing.T) {
	for _, test := range []struct {
		name    string
		schema  *jsonschema.Schema
		valid   []string
		invalid []string
	}{
		{
			"tag", repositoryTagOutputSchema(),
			[]string{`null`, `{"ref":"refs/tags/v1","object":{"type":"commit","sha":"abc"}}`, `{"tag":"v1","sha":"tag-sha","tagger":{"date":"2026-01-02T00:00:00Z"}}`},
			[]string{`{}`, `{"ref":"refs/tags/v1"}`, `{"ref":"refs/tags/v1","object":{"type":"commit","sha":"abc"},"tag":"v1"}`, `{"message":1}`},
		},
		{
			"fork", forkRepositoryOutputSchema(),
			[]string{`null`, `{"id":"9","url":"fork-url"}`, `{"message":"Fork is in progress"}`},
			[]string{`{}`, `{"id":"9"}`, `{"id":"9","url":"fork-url","message":"pending"}`},
		},
		{
			"contents", repositoryContentsOutputSchema(),
			[]string{`null`, `[]`, `[{"name":"file"}]`, `{"content":[]}`, `{"content":[{"type":"text","text":"status"}]}`,
				`{"content":[{"type":"resource","resource":{"uri":"repo://file","mimeType":"text/plain"}}]}`,
				`{"content":[{"type":"resource","resource":{"uri":"repo://file","blob":"AAEC"}}]}`,
				`{"content":[{"type":"resource_link","uri":"repo://file","name":"file","size":1048576}]}`},
			[]string{`{}`, `{"content":[{"type":"text"}]}`, `{"content":[{"type":"resource","resource":{"uri":"repo://file","text":"x","blob":"AA=="}}]}`,
				`{"content":[{"type":"opaque","data":{}}]}`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved, err := test.schema.Resolve(nil)
			require.NoError(t, err)
			for _, text := range test.valid {
				var output any
				require.NoError(t, json.Unmarshal([]byte(text), &output))
				require.NoError(t, resolved.Validate(output), text)
			}
			for _, text := range test.invalid {
				var output any
				require.NoError(t, json.Unmarshal([]byte(text), &output))
				require.Error(t, resolved.Validate(output), text)
			}
		})
	}
}

func TestRepositoryPaginationCompatibility(t *testing.T) {
	for _, raw := range []string{`{}`, `{"page":0,"perPage":0}`, `{"page":"0","perPage":"0"}`} {
		normalized, err := normalizeRepositoryArguments(json.RawMessage(raw))
		require.NoError(t, err)
		var input ListReleasesInput
		require.NoError(t, json.Unmarshal(normalized, &input))
		assert.Equal(t, PaginationParams{Page: 1, PerPage: 30}, repositoryPagination(input.Page, input.PerPage))
	}
	fields, err := normalizeRepositoryFieldsArguments(json.RawMessage(`{"page":"2.0","perPage":"7","fields":null}`))
	require.NoError(t, err)
	normalized, err := normalizeRepositoryArguments(fields)
	require.NoError(t, err)
	assert.JSONEq(t, `{"page":2,"perPage":7}`, string(normalized))
	for _, raw := range []string{`{"page":"1.5"}`, `{"perPage":"NaN"}`} {
		_, err := normalizeRepositoryArguments(json.RawMessage(raw))
		require.Error(t, err)
	}
	for _, raw := range []string{`{"start_line":"0"}`, `{"end_line":0}`, `{"perPage":"0"}`} {
		normalized, err := normalizeRepositoryBlameArguments(json.RawMessage(raw))
		require.NoError(t, err)
		var input GetFileBlameInput
		require.NoError(t, json.Unmarshal(normalized, &input))
		assert.True(t, input.StartLine != nil || input.EndLine != nil || input.PerPage != nil, "blame preserves explicit zero for validation")
	}
}
