package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yosida95/uritemplate/v3"
)

func Test_GetSkillResourceContent(t *testing.T) {
	t.Run("definition", func(t *testing.T) {
		res := GetSkillResourceContent(translations.NullTranslationHelper)
		assert.Equal(t, "skill_content", res.Template.Name)
		assert.Contains(t, res.Template.URITemplate, "skill://")
		assert.Contains(t, res.Template.URITemplate, "{skill_name}")
		assert.NotEmpty(t, res.Template.Description)
		assert.True(t, res.HasHandler())
	})
}

func Test_GetSkillResourceManifest(t *testing.T) {
	t.Run("definition", func(t *testing.T) {
		res := GetSkillResourceManifest(translations.NullTranslationHelper)
		assert.Equal(t, "skill_manifest", res.Template.Name)
		assert.Contains(t, res.Template.URITemplate, "_manifest")
		assert.NotEmpty(t, res.Template.Description)
		assert.True(t, res.HasHandler())
	})
}

func Test_skillContentHandler(t *testing.T) {
	skillMDContent := "---\nname: my-skill\ndescription: A test skill\n---\n\n# My Skill\n\nInstructions here."
	encodedContent := base64.StdEncoding.EncodeToString([]byte(skillMDContent))

	// Wildcard pattern to match deep paths under /repos/{owner}/{repo}/contents/
	const getContentsWildcard = "GET /repos/{owner}/{repo}/contents/{path:.*}"

	tests := []struct {
		name        string
		uri         string
		handlers    map[string]http.HandlerFunc
		expectError string
		expectText  string
	}{
		{
			name:        "missing owner",
			uri:         "skill:///repo/my-skill/SKILL.md",
			handlers:    map[string]http.HandlerFunc{},
			expectError: "owner is required",
		},
		{
			name:        "missing repo",
			uri:         "skill://owner//my-skill/SKILL.md",
			handlers:    map[string]http.HandlerFunc{},
			expectError: "repo is required",
		},
		{
			name:        "missing skill_name",
			uri:         "skill://owner/repo//SKILL.md",
			handlers:    map[string]http.HandlerFunc{},
			expectError: "skill_name is required",
		},
		{
			name: "successful fetch",
			uri:  "skill://owner/repo/my-skill/SKILL.md",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
				getContentsWildcard: func(w http.ResponseWriter, _ *http.Request) {
					resp := &gogithub.RepositoryContent{
						Type:     gogithub.Ptr("file"),
						Name:     gogithub.Ptr("SKILL.md"),
						Content:  gogithub.Ptr(encodedContent),
						Encoding: gogithub.Ptr("base64"),
					}
					data, _ := json.Marshal(resp)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expectText: skillMDContent,
		},
		{
			name: "skill not found",
			uri:  "skill://owner/repo/nonexistent/SKILL.md",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("README.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expectError: `skill "nonexistent" not found`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))
			deps := BaseDeps{Client: client}
			ctx := ContextWithDeps(t.Context(), deps)

			handler := skillContentHandler(skillResourceContentURITemplate)
			result, err := handler(ctx, &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{URI: tc.uri},
			})

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Contents, 1)
			assert.Equal(t, "text/markdown", result.Contents[0].MIMEType)
			assert.Equal(t, tc.expectText, result.Contents[0].Text)
		})
	}
}

func Test_skillManifestHandler(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		handlers    map[string]http.HandlerFunc
		expectError string
		expectSkill string
		expectFiles int
	}{
		{
			name: "successful manifest",
			uri:  "skill://owner/repo/my-skill/_manifest",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob"), Size: gogithub.Ptr(256)},
							{Path: gogithub.Ptr("skills/my-skill/references/REFERENCE.md"), Type: gogithub.Ptr("blob"), Size: gogithub.Ptr(1024)},
							{Path: gogithub.Ptr("skills/my-skill/references"), Type: gogithub.Ptr("tree"), Size: gogithub.Ptr(0)},
							{Path: gogithub.Ptr("README.md"), Type: gogithub.Ptr("blob"), Size: gogithub.Ptr(100)},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expectSkill: "my-skill",
			expectFiles: 2, // only blobs under the skill dir
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))
			deps := BaseDeps{Client: client}
			ctx := ContextWithDeps(t.Context(), deps)

			handler := skillManifestHandler(skillResourceManifestURITemplate)
			result, err := handler(ctx, &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{URI: tc.uri},
			})

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, result.Contents, 1)
			assert.Equal(t, "application/json", result.Contents[0].MIMEType)

			var manifest SkillManifest
			err = json.Unmarshal([]byte(result.Contents[0].Text), &manifest)
			require.NoError(t, err)
			assert.Equal(t, tc.expectSkill, manifest.Skill)
			assert.Len(t, manifest.Files, tc.expectFiles)

			// Verify each file has a repo:// URI
			for _, f := range manifest.Files {
				assert.True(t, strings.HasPrefix(f.URI, "repo://"), "expected repo:// URI, got %s", f.URI)
			}
		})
	}
}

func Test_discoverSkills(t *testing.T) {
	tests := []struct {
		name        string
		handlers    map[string]http.HandlerFunc
		expectError string
		expect      []string
	}{
		{
			name: "finds skills under standard conventions",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/code-review/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/pdf-processing/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/pdf-processing/references/REF.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"code-review", "pdf-processing"},
		},
		{
			name: "finds namespaced skills",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/acme/data-analysis/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/acme/code-review/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"data-analysis", "code-review"},
		},
		{
			name: "finds plugin convention skills",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("plugins/my-plugin/skills/lint-check/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"lint-check"},
		},
		{
			name: "finds root-level skills",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"my-skill"},
		},
		{
			name: "excludes hidden and convention-prefix root dirs",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr(".github/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("plugins/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("legit-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"legit-skill"},
		},
		{
			name: "no skills found",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("README.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: nil,
		},
		{
			name: "deduplicates skills across conventions",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"my-skill"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))

			skills, err := discoverSkills(t.Context(), client, "owner", "repo")

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tc.expect, skills)
		})
	}
}

func Test_matchSkillConventions(t *testing.T) {
	tests := []struct {
		path      string
		expectNil bool
		name      string
		dir       string
	}{
		// Convention 1: skills/*/SKILL.md
		{path: "skills/code-review/SKILL.md", name: "code-review", dir: "skills/code-review"},
		// Convention 2: skills/{namespace}/*/SKILL.md
		{path: "skills/acme/data-tool/SKILL.md", name: "data-tool", dir: "skills/acme/data-tool"},
		// Convention 3: plugins/*/skills/*/SKILL.md
		{path: "plugins/my-plugin/skills/lint/SKILL.md", name: "lint", dir: "plugins/my-plugin/skills/lint"},
		// Convention 4: */SKILL.md (root-level)
		{path: "my-skill/SKILL.md", name: "my-skill", dir: "my-skill"},
		// Excluded: hidden dirs
		{path: ".github/SKILL.md", expectNil: true},
		// Excluded: convention prefixes as root skills
		{path: "skills/SKILL.md", expectNil: true},
		{path: "plugins/SKILL.md", expectNil: true},
		// Excluded: not SKILL.md
		{path: "skills/code-review/README.md", expectNil: true},
		// Excluded: bare SKILL.md at repo root
		{path: "SKILL.md", expectNil: true},
		// Excluded: too deeply nested without matching convention
		{path: "a/b/c/d/SKILL.md", expectNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := matchSkillConventions(tc.path)
			if tc.expectNil {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, tc.name, result.Name)
			assert.Equal(t, tc.dir, result.Dir)
		})
	}
}

func Test_parseSkillURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		tmpl        string
		expectOwner string
		expectRepo  string
		expectSkill string
		expectError string
	}{
		{
			name:        "valid content URI",
			uri:         "skill://octocat/hello-world/my-skill/SKILL.md",
			tmpl:        "skill://{owner}/{repo}/{skill_name}/SKILL.md",
			expectOwner: "octocat",
			expectRepo:  "hello-world",
			expectSkill: "my-skill",
		},
		{
			name:        "valid manifest URI",
			uri:         "skill://octocat/hello-world/my-skill/_manifest",
			tmpl:        "skill://{owner}/{repo}/{skill_name}/_manifest",
			expectOwner: "octocat",
			expectRepo:  "hello-world",
			expectSkill: "my-skill",
		},
		{
			name:        "missing owner",
			uri:         "skill:///hello-world/my-skill/SKILL.md",
			tmpl:        "skill://{owner}/{repo}/{skill_name}/SKILL.md",
			expectError: "owner is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := uritemplate.MustNew(tc.tmpl)
			owner, repo, skill, err := parseSkillURI(tmpl, tc.uri)

			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectOwner, owner)
			assert.Equal(t, tc.expectRepo, repo)
			assert.Equal(t, tc.expectSkill, skill)
		})
	}
}

func Test_SkillResourceCompletionHandler(t *testing.T) {
	tests := []struct {
		name     string
		request  *mcp.CompleteRequest
		handlers map[string]http.HandlerFunc
		expected int // number of completion values
		wantErr  bool
	}{
		{
			name: "completes skill_name",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "ref/resource",
						URI:  "skill://owner/repo/{skill_name}/SKILL.md",
					},
					Argument: mcp.CompleteParamsArgument{
						Name:  "skill_name",
						Value: "",
					},
					Context: &mcp.CompleteContext{
						Arguments: map[string]string{
							"owner": "owner",
							"repo":  "repo",
						},
					},
				},
			},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/skill-a/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/skill-b/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expected: 2,
		},
		{
			name: "filters skill_name by prefix",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "ref/resource",
						URI:  "skill://owner/repo/{skill_name}/SKILL.md",
					},
					Argument: mcp.CompleteParamsArgument{
						Name:  "skill_name",
						Value: "skill-a",
					},
					Context: &mcp.CompleteContext{
						Arguments: map[string]string{
							"owner": "owner",
							"repo":  "repo",
						},
					},
				},
			},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{
						Entries: []*gogithub.TreeEntry{
							{Path: gogithub.Ptr("skills/skill-a/SKILL.md"), Type: gogithub.Ptr("blob")},
							{Path: gogithub.Ptr("skills/skill-b/SKILL.md"), Type: gogithub.Ptr("blob")},
						},
					}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expected: 1,
		},
		{
			name: "unknown argument returns error",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref: &mcp.CompleteReference{
						Type: "ref/resource",
						URI:  "skill://owner/repo/{skill_name}/SKILL.md",
					},
					Argument: mcp.CompleteParamsArgument{
						Name:  "unknown_arg",
						Value: "",
					},
				},
			},
			handlers: map[string]http.HandlerFunc{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))
			getClient := func(_ context.Context) (*gogithub.Client, error) {
				return client, nil
			}

			handler := SkillResourceCompletionHandler(getClient)
			result, err := handler(t.Context(), tc.request)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Len(t, result.Completion.Values, tc.expected)
		})
	}
}
