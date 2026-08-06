package github

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"testing"

	"github.com/github/github-mcp-server/skills"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const getReposGitBlobsBySHA = "GET /repos/{owner}/{repo}/git/blobs/{sha}"

const repoSkillMD = "---\nname: my-skill\ndescription: A repo-hosted test skill\n---\n\n# My Skill\n"
const repoGuideMD = "# Guide\n\nDeep details.\n"

// repoSkillHandlers mocks a repository holding skills/my-skill/{SKILL.md,references/GUIDE.md}.
func repoSkillHandlers(skillMD string) map[string]http.HandlerFunc {
	blobs := map[string]string{
		"sha-skill": skillMD,
		"sha-guide": repoGuideMD,
	}
	return map[string]http.HandlerFunc{
		GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
			tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
				{Path: gogithub.Ptr("skills"), Type: gogithub.Ptr("tree")},
				{Path: gogithub.Ptr("skills/my-skill"), Type: gogithub.Ptr("tree")},
				{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob"), SHA: gogithub.Ptr("sha-skill")},
				{Path: gogithub.Ptr("skills/my-skill/references"), Type: gogithub.Ptr("tree")},
				{Path: gogithub.Ptr("skills/my-skill/references/GUIDE.md"), Type: gogithub.Ptr("blob"), SHA: gogithub.Ptr("sha-guide")},
				{Path: gogithub.Ptr("README.md"), Type: gogithub.Ptr("blob"), SHA: gogithub.Ptr("sha-readme")},
			}}
			data, _ := json.Marshal(tree)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(data)
		},
		getReposGitBlobsBySHA: func(w http.ResponseWriter, r *http.Request) {
			// The mock transport is not a real ServeMux, so read the SHA
			// from the URL path directly.
			content, ok := blobs[path.Base(r.URL.Path)]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(content))
		},
	}
}

func dynamicTestContext(t *testing.T, handlers map[string]http.HandlerFunc) context.Context {
	t.Helper()
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(handlers))
	return ContextWithDeps(context.Background(), BaseDeps{Client: client})
}

func Test_RepoSkillEntry(t *testing.T) {
	t.Run("builds a digested entry for a discovered skill", func(t *testing.T) {
		ctx := dynamicTestContext(t, repoSkillHandlers(repoSkillMD))
		entry, err := RepoSkillEntry(ctx, "skill://octocat/hello-world/my-skill/SKILL.md")
		require.NoError(t, err)
		require.NotNil(t, entry)

		assert.Equal(t, "skill://octocat/hello-world/my-skill/SKILL.md", entry.URI)
		assert.Equal(t, "my-skill", entry.Frontmatter["name"])
		assert.Equal(t, "A repo-hosted test skill", entry.Frontmatter["description"])

		require.Len(t, entry.Resources, 2)
		assert.Equal(t, entry.URI, entry.Resources[0].URI)
		assert.Equal(t, skills.Digest([]byte(repoSkillMD)), entry.Resources[0].Digest)
		assert.Equal(t, "skill://octocat/hello-world/my-skill/references/GUIDE.md", entry.Resources[1].URI)
		assert.Equal(t, skills.Digest([]byte(repoGuideMD)), entry.Resources[1].Digest)
	})

	t.Run("nil for URIs that are not a SKILL.md", func(t *testing.T) {
		ctx := dynamicTestContext(t, repoSkillHandlers(repoSkillMD))
		entry, err := RepoSkillEntry(ctx, "skill://octocat/hello-world/my-skill/references/GUIDE.md")
		require.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("nil for skills the repository does not hold", func(t *testing.T) {
		ctx := dynamicTestContext(t, repoSkillHandlers(repoSkillMD))
		entry, err := RepoSkillEntry(ctx, "skill://octocat/hello-world/other-skill/SKILL.md")
		require.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("-32602 for a non-conforming skill", func(t *testing.T) {
		// Frontmatter name disagrees with the directory name, violating the
		// SEP's final-segment rule — the server cannot serve it as a skill.
		badSkillMD := "---\nname: something-else\ndescription: mismatched\n---\nbody\n"
		ctx := dynamicTestContext(t, repoSkillHandlers(badSkillMD))
		_, err := RepoSkillEntry(ctx, "skill://octocat/hello-world/my-skill/SKILL.md")
		require.Error(t, err)
		var jsonrpcErr *jsonrpc.Error
		require.ErrorAs(t, err, &jsonrpcErr)
		assert.EqualValues(t, jsonrpc.CodeInvalidParams, jsonrpcErr.Code)
		assert.Contains(t, jsonrpcErr.Message, "not a conforming skill")
	})
}

func Test_RepoSkillDirectory(t *testing.T) {
	ctx := dynamicTestContext(t, repoSkillHandlers(repoSkillMD))

	t.Run("repo namespace lists discovered skills as directories", func(t *testing.T) {
		children, err := RepoSkillDirectory(ctx, "skill://octocat/hello-world")
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, "skill://octocat/hello-world/my-skill", children[0].URI)
		assert.Equal(t, skills.DirectoryMIMEType, children[0].MIMEType)
	})

	t.Run("skill root lists files and subdirectories", func(t *testing.T) {
		children, err := RepoSkillDirectory(ctx, "skill://octocat/hello-world/my-skill")
		require.NoError(t, err)
		require.Len(t, children, 2)
		assert.Equal(t, "skill://octocat/hello-world/my-skill/SKILL.md", children[0].URI)
		assert.Equal(t, "text/markdown", children[0].MIMEType)
		assert.Equal(t, "skill://octocat/hello-world/my-skill/references", children[1].URI)
		assert.Equal(t, skills.DirectoryMIMEType, children[1].MIMEType)
	})

	t.Run("subdirectory lists its own children", func(t *testing.T) {
		children, err := RepoSkillDirectory(ctx, "skill://octocat/hello-world/my-skill/references")
		require.NoError(t, err)
		require.Len(t, children, 1)
		assert.Equal(t, "skill://octocat/hello-world/my-skill/references/GUIDE.md", children[0].URI)
		assert.Equal(t, "GUIDE.md", children[0].Name)
	})

	t.Run("nil for file URIs and unknown directories", func(t *testing.T) {
		for _, uri := range []string{
			"skill://octocat/hello-world/my-skill/SKILL.md",
			"skill://octocat/hello-world/my-skill/nonexistent",
			"skill://octocat/hello-world/other-skill",
			"skill://octocat",
		} {
			children, err := RepoSkillDirectory(ctx, uri)
			require.NoError(t, err, uri)
			assert.Nil(t, children, uri)
		}
	})
}
