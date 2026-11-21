package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type pushFilesFixture struct {
	trees           map[string]*github.Tree
	lookupStatus    int
	missingBranch   bool
	emptyRepository bool
	writes          []string
	treeReads       map[string]int
	requests        int
	entries         []*github.TreeEntry
}

func (f *pushFilesFixture) run(t *testing.T, files []any) *mcp.CallToolResult {
	t.Helper()
	f.treeReads = map[string]int{}
	backend := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{"": func(w http.ResponseWriter, r *http.Request) {
		f.requests++
		path := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo")
		if r.Method != http.MethodGet {
			f.writes = append(f.writes, r.Method+" "+path)
		}
		ref := &github.Reference{Ref: new("refs/heads/main"), Object: &github.GitObject{SHA: new("base-commit")}}
		switch {
		case r.Method == http.MethodGet && path == "/git/ref/heads/main":
			switch {
			case f.emptyRepository:
				mockResponse(t, http.StatusConflict, map[string]any{"message": "Git Repository is empty."})(w, r)
			case f.missingBranch:
				mockResponse(t, 404, nil)(w, r)
			default:
				mockResponse(t, 200, ref)(w, r)
			}
		case r.Method == http.MethodGet && path == "":
			mockResponse(t, 200, &github.Repository{DefaultBranch: new("default")})(w, r)
		case r.Method == http.MethodGet && path == "/git/ref/heads/default":
			mockResponse(t, 200, &github.Reference{Ref: new("refs/heads/default"), Object: ref.Object})(w, r)
		case r.Method == http.MethodPut && path == "/contents/README.md":
			mockResponse(t, 201, &github.RepositoryContentResponse{Commit: github.Commit{SHA: new("base-commit")}})(w, r)
		case r.Method == http.MethodGet && path == "/git/commits/base-commit":
			mockResponse(t, 200, &github.Commit{SHA: new("base-commit"), Tree: &github.Tree{SHA: new("root")}})(w, r)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "/git/trees/"):
			require.Empty(t, r.URL.Query().Get("recursive"))
			sha := strings.TrimPrefix(path, "/git/trees/")
			f.treeReads[sha]++
			if f.lookupStatus != 0 {
				mockResponse(t, f.lookupStatus, nil)(w, r)
				return
			}
			tree, ok := f.trees[sha]
			require.True(t, ok, "unexpected tree lookup: %s", sha)
			mockResponse(t, 200, tree)(w, r)
		case r.Method == http.MethodPost && path == "/git/trees":
			var body struct {
				BaseTree string              `json:"base_tree"`
				Entries  []*github.TreeEntry `json:"tree"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "root", body.BaseTree)
			f.entries = body.Entries
			mockResponse(t, 201, &github.Tree{SHA: new("new-tree")})(w, r)
		case r.Method == http.MethodPost && path == "/git/commits":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, []any{"base-commit"}, body["parents"])
			require.Equal(t, "new-tree", body["tree"])
			mockResponse(t, 201, &github.Commit{SHA: new("new-commit")})(w, r)
		case r.Method == http.MethodPost && path == "/git/refs":
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "base-commit", body["sha"])
			require.Equal(t, "refs/heads/main", body["ref"])
			mockResponse(t, 201, ref)(w, r)
		case r.Method == http.MethodPatch && path == "/git/refs/heads/main":
			mockResponse(t, 200, ref)(w, r)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}})
	deps := BaseDeps{Client: mustNewGHClient(t, backend)}
	tool := PushFiles(translations.NullTranslationHelper)
	request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo", "branch": "main", "message": "update", "files": files})
	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	return result
}

func pushFilesTreeEntry(path, mode, kind string) *github.TreeEntry {
	return &github.TreeEntry{Path: new(path), Mode: new(mode), Type: new(kind), SHA: new(path + "-sha")}
}

func Test_PushFiles_FileModes(t *testing.T) {
	for _, content := range []string{"changed content", "same content"} {
		t.Run("preserve executable "+content, func(t *testing.T) {
			existing := pushFilesTreeEntry("run.sh", "100755", "blob")
			existing.SHA = new(gitBlobSHA([]byte("same content")))
			f := pushFilesFixture{trees: map[string]*github.Tree{"root": {Entries: []*github.TreeEntry{existing}}}}
			result := f.run(t, []any{map[string]any{"path": "run.sh", "content": content}})
			require.False(t, result.IsError)
			require.Len(t, f.entries, 1)
			require.Equal(t, "100755", f.entries[0].GetMode())
			require.Equal(t, content, f.entries[0].GetContent())
		})
	}
	t.Run("mixed defaults explicit modes and nested cache", func(t *testing.T) {
		f := pushFilesFixture{trees: map[string]*github.Tree{
			"root":   {Entries: []*github.TreeEntry{pushFilesTreeEntry("ordinary", "100644", "blob"), pushFilesTreeEntry("make-exec", "100644", "blob"), pushFilesTreeEntry("exec", "100755", "blob"), pushFilesTreeEntry("untouched-link", "120000", "blob"), pushFilesTreeEntry("untouched-module", "160000", "commit"), {Path: new("dir"), Mode: new("040000"), Type: new("tree"), SHA: new("nested")}}},
			"nested": {Entries: []*github.TreeEntry{pushFilesTreeEntry("run", "100755", "blob")}},
		}}
		files := []any{
			map[string]any{"path": "ordinary", "content": "a"},
			map[string]any{"path": "new", "content": "b"},
			map[string]any{"path": "exec", "content": "c", "mode": "100644"},
			map[string]any{"path": "make-exec", "content": "d", "mode": "100755"},
			map[string]any{"path": "new-exec", "content": "e", "mode": "100755"},
			map[string]any{"path": "new-ordinary", "content": "f", "mode": "100644"},
			map[string]any{"path": "dir/run", "content": "g"},
			map[string]any{"path": "dir/new", "content": "h"},
			map[string]any{"path": "absent/deep/new", "content": "i"},
		}
		result := f.run(t, files)
		require.False(t, result.IsError)
		require.Len(t, f.entries, len(files))
		for i, mode := range []string{"100644", "100644", "100644", "100755", "100755", "100644", "100755", "100644", "100644"} {
			require.Equal(t, mode, f.entries[i].GetMode())
			require.Equal(t, "blob", f.entries[i].GetType())
		}
		require.Equal(t, map[string]int{"root": 1, "nested": 1}, f.treeReads)
	})
}

func Test_PushFiles_RejectUnsupportedEntries(t *testing.T) {
	for _, entry := range []*github.TreeEntry{pushFilesTreeEntry("target", "120000", "blob"), pushFilesTreeEntry("target", "160000", "commit"), pushFilesTreeEntry("target", "040000", "tree"), pushFilesTreeEntry("target", "100644", "tree")} {
		for _, path := range []string{"target", "target/child"} {
			// A real directory ancestor is supported; test unsupported final directory only.
			if path == "target/child" && entry.GetMode() == "040000" {
				continue
			}
			for _, explicit := range []bool{false, true} {
				t.Run(entry.GetMode()+entry.GetType()+path+map[bool]string{false: " omitted", true: " explicit"}[explicit], func(t *testing.T) {
					f := pushFilesFixture{trees: map[string]*github.Tree{"root": {Entries: []*github.TreeEntry{entry}}}, missingBranch: true}
					file := map[string]any{"path": path, "content": "new"}
					if explicit {
						file["mode"] = "100755"
					}
					result := f.run(t, []any{map[string]any{"path": "safe", "content": "safe"}, file})
					require.True(t, result.IsError)
					require.Empty(t, f.writes)
				})
			}
		}
	}
}

func Test_PushFiles_LookupFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		tree   *github.Tree
	}{
		{name: "HTTP error", status: 500},
		{name: "truncated", tree: &github.Tree{Truncated: new(true)}},
		{name: "missing directory SHA", tree: &github.Tree{Entries: []*github.TreeEntry{{Path: new("dir"), Mode: new("040000"), Type: new("tree")}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := pushFilesFixture{trees: map[string]*github.Tree{"root": tc.tree}, lookupStatus: tc.status, missingBranch: true}
			result := f.run(t, []any{map[string]any{"path": "dir/run", "content": "x", "mode": "100755"}})
			require.True(t, result.IsError)
			require.Empty(t, f.writes)
		})
	}
}

func Test_PushFiles_InvalidModesBeforeRequests(t *testing.T) {
	for _, mode := range []any{nil, 123, true, "", "100600", "120000", "040000", "160000", []any{"100755"}} {
		name, _ := json.Marshal(mode)
		t.Run(string(name), func(t *testing.T) {
			f := pushFilesFixture{}
			result := f.run(t, []any{map[string]any{"path": "safe", "content": "safe"}, map[string]any{"path": "run", "content": "x", "mode": mode}})
			require.True(t, result.IsError)
			require.Zero(t, f.requests)
			require.Empty(t, f.writes)
		})
	}
}

func Test_PushFiles_NestedLookupFailsClosed(t *testing.T) {
	for _, nested := range []*github.Tree{
		{Truncated: new(true)},
		{Entries: []*github.TreeEntry{pushFilesTreeEntry("link", "120000", "blob")}},
	} {
		t.Run(map[bool]string{true: "truncated", false: "symlink ancestor"}[nested.GetTruncated()], func(t *testing.T) {
			f := pushFilesFixture{trees: map[string]*github.Tree{
				"root":   {Entries: []*github.TreeEntry{{Path: new("dir"), Mode: new("040000"), Type: new("tree"), SHA: new("nested")}}},
				"nested": nested,
			}, missingBranch: true}
			result := f.run(t, []any{map[string]any{"path": "dir/link/run", "content": "x"}})
			require.True(t, result.IsError)
			require.Empty(t, f.writes)
			require.Equal(t, map[string]int{"root": 1, "nested": 1}, f.treeReads)
		})
	}
}

func Test_PushFiles_EmptyRepositoryNewBranch(t *testing.T) {
	f := pushFilesFixture{trees: map[string]*github.Tree{"root": {}}, emptyRepository: true}
	result := f.run(t, []any{map[string]any{"path": "run", "content": "x", "mode": "100755"}})
	require.False(t, result.IsError)
	require.Equal(t, "100755", f.entries[0].GetMode())
	require.Equal(t, []string{"PUT /contents/README.md", "POST /git/refs", "POST /git/trees", "POST /git/commits", "PATCH /git/refs/heads/main"}, f.writes)
}

func Test_PushFiles_NewBranchPreservesModes(t *testing.T) {
	f := pushFilesFixture{trees: map[string]*github.Tree{"root": {Entries: []*github.TreeEntry{pushFilesTreeEntry("run", "100755", "blob")}}}, missingBranch: true}
	result := f.run(t, []any{map[string]any{"path": "run", "content": "x"}})
	require.False(t, result.IsError)
	require.Equal(t, "100755", f.entries[0].GetMode())
	require.Equal(t, []string{"POST /git/refs", "POST /git/trees", "POST /git/commits", "PATCH /git/refs/heads/main"}, f.writes)
}

func Test_PushFiles_TreeTraversalLimit(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, depth := range []int{64, 65} {
			name := strings.Repeat("dir/", depth-1) + "run"
			t.Run(name+map[bool]string{false: " omitted", true: " explicit"}[explicit], func(t *testing.T) {
				f := pushFilesFixture{trees: map[string]*github.Tree{}, missingBranch: true}
				sha := "root"
				for i := 1; i < depth; i++ {
					next := sha + "-child"
					f.trees[sha] = &github.Tree{Entries: []*github.TreeEntry{{Path: new("dir"), Mode: new("040000"), Type: new("tree"), SHA: new(next)}}}
					sha = next
				}
				f.trees[sha] = &github.Tree{Entries: []*github.TreeEntry{pushFilesTreeEntry("run", "100755", "blob")}}
				file := map[string]any{"path": name, "content": "x"}
				if explicit {
					file["mode"] = "100755"
				}
				result := f.run(t, []any{file})
				if depth > 64 {
					require.True(t, result.IsError)
					require.Contains(t, result.Content[0].(*mcp.TextContent).Text, "exceeds Git tree traversal limit")
					require.Empty(t, f.treeReads)
					require.Empty(t, f.writes)
				} else {
					require.False(t, result.IsError)
					require.Len(t, f.treeReads, depth)
					require.Equal(t, "100755", f.entries[0].GetMode())
				}
			})
		}
	}
}
