package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/skills"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yosida95/uritemplate/v3"
)

func Test_GetSkillResourceFile(t *testing.T) {
	res := GetSkillResourceFile(translations.NullTranslationHelper)
	assert.Equal(t, "skill_file", res.Template.Name)
	assert.Contains(t, res.Template.URITemplate, "skill://")
	assert.Contains(t, res.Template.URITemplate, "{skill_name}")
	assert.Contains(t, res.Template.URITemplate, "{+file_path}", "must use reserved expansion so multi-segment relative paths round-trip")
	assert.NotEmpty(t, res.Template.Description)
	assert.True(t, res.HasHandler())
}

func Test_skillFileHandler(t *testing.T) {
	const skillMDContent = "---\nname: my-skill\ndescription: A test skill\n---\n\n# My Skill\n\nInstructions here."
	const referenceContent = "# Reference\n\nDeep details for the agent."
	encodedSkillMD := base64.StdEncoding.EncodeToString([]byte(skillMDContent))
	encodedReference := base64.StdEncoding.EncodeToString([]byte(referenceContent))

	// Wildcard pattern to match deep paths under /repos/{owner}/{repo}/contents/
	const getContentsWildcard = "GET /repos/{owner}/{repo}/contents/{path:.*}"

	// Mock that always returns the SKILL.md tree entry, plus a reference file.
	standardTreeMock := func(w http.ResponseWriter, _ *http.Request) {
		tree := &gogithub.Tree{
			Entries: []*gogithub.TreeEntry{
				{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
				{Path: gogithub.Ptr("skills/my-skill/references/REFERENCE.md"), Type: gogithub.Ptr("blob")},
			},
		}
		data, _ := json.Marshal(tree)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}

	tests := []struct {
		name             string
		uri              string
		handlers         map[string]http.HandlerFunc
		expectError      string
		expectText       string
		expectMIME       string
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
			name:        "rejects path traversal",
			uri:         "skill://owner/repo/my-skill/../../etc/passwd",
			handlers:    map[string]http.HandlerFunc{},
			expectError: "must not contain ..",
		},
		{
			name: "fetches SKILL.md",
			uri:  "skill://owner/repo/my-skill/SKILL.md",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: standardTreeMock,
				getContentsWildcard: func(w http.ResponseWriter, _ *http.Request) {
					resp := &gogithub.RepositoryContent{
						Type:     gogithub.Ptr("file"),
						Name:     gogithub.Ptr("SKILL.md"),
						Content:  gogithub.Ptr(encodedSkillMD),
						Encoding: gogithub.Ptr("base64"),
					}
					data, _ := json.Marshal(resp)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expectText: skillMDContent,
			expectMIME: "text/markdown",
		},
		{
			name: "fetches multi-segment relative file (SEP relative-path resolution)",
			uri:  "skill://owner/repo/my-skill/references/REFERENCE.md",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: standardTreeMock,
				getContentsWildcard: func(w http.ResponseWriter, _ *http.Request) {
					resp := &gogithub.RepositoryContent{
						Type:     gogithub.Ptr("file"),
						Name:     gogithub.Ptr("REFERENCE.md"),
						Content:  gogithub.Ptr(encodedReference),
						Encoding: gogithub.Ptr("base64"),
					}
					data, _ := json.Marshal(resp)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expectText: referenceContent,
			expectMIME: "text/markdown",
		},
		{
			name: "skill not found in repo",
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
			ctx := ContextWithDeps(context.Background(), deps)

			handler := skillFileHandler(skillResourceFileURITemplate)
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
			assert.Equal(t, tc.expectMIME, result.Contents[0].MIMEType)
			assert.Equal(t, tc.expectText, result.Contents[0].Text)
			assert.Equal(t, tc.uri, result.Contents[0].URI, "round-trip URI must match the requested URI")
		})
	}
}

func Test_discoverSkills(t *testing.T) {
	tests := []struct {
		name     string
		handlers map[string]http.HandlerFunc
		expect   []string
	}{
		{
			name: "finds skills under standard convention",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("skills/code-review/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/pdf-processing/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/pdf-processing/references/REF.md"), Type: gogithub.Ptr("blob")},
					}}
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
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("skills/acme/data-analysis/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/acme/code-review/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
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
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("plugins/my-plugin/skills/lint-check/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
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
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
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
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr(".github/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("plugins/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("legit-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expect: []string{"legit-skill"},
		},
		{
			name: "deduplicates skills across conventions",
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("skills/my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("my-skill/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
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
			skills, err := discoverSkills(context.Background(), client, "owner", "repo")
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
		{path: "skills/code-review/SKILL.md", name: "code-review", dir: "skills/code-review"},
		{path: "skills/acme/data-tool/SKILL.md", name: "data-tool", dir: "skills/acme/data-tool"},
		{path: "plugins/my-plugin/skills/lint/SKILL.md", name: "lint", dir: "plugins/my-plugin/skills/lint"},
		{path: "my-skill/SKILL.md", name: "my-skill", dir: "my-skill"},
		{path: ".github/SKILL.md", expectNil: true},
		{path: "skills/SKILL.md", expectNil: true},
		{path: "plugins/SKILL.md", expectNil: true},
		{path: "skills/code-review/README.md", expectNil: true},
		{path: "SKILL.md", expectNil: true},
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

func Test_parseSkillFileURI(t *testing.T) {
	tmpl := uritemplate.MustNew("skill://{owner}/{repo}/{skill_name}/{+file_path}")

	tests := []struct {
		name        string
		uri         string
		expectOwner string
		expectRepo  string
		expectSkill string
		expectFile  string
		expectError string
	}{
		{
			name:        "valid SKILL.md URI",
			uri:         "skill://octocat/hello-world/my-skill/SKILL.md",
			expectOwner: "octocat",
			expectRepo:  "hello-world",
			expectSkill: "my-skill",
			expectFile:  "SKILL.md",
		},
		{
			name:        "valid multi-segment file path",
			uri:         "skill://octocat/hello-world/my-skill/references/GUIDE.md",
			expectOwner: "octocat",
			expectRepo:  "hello-world",
			expectSkill: "my-skill",
			expectFile:  "references/GUIDE.md",
		},
		{
			name:        "missing owner",
			uri:         "skill:///hello-world/my-skill/SKILL.md",
			expectError: "owner is required",
		},
		{
			name:        "rejects parent traversal",
			uri:         "skill://o/r/my-skill/../../etc/passwd",
			expectError: "must not contain ..",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, skill, file, err := parseSkillFileURI(tmpl, tc.uri)
			if tc.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectOwner, owner)
			assert.Equal(t, tc.expectRepo, repo)
			assert.Equal(t, tc.expectSkill, skill)
			assert.Equal(t, tc.expectFile, file)
		})
	}
}

func Test_SkillResourceCompletionHandler(t *testing.T) {
	tests := []struct {
		name     string
		request  *mcp.CompleteRequest
		handlers map[string]http.HandlerFunc
		expected int
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
					Argument: mcp.CompleteParamsArgument{Name: "skill_name", Value: ""},
					Context:  &mcp.CompleteContext{Arguments: map[string]string{"owner": "owner", "repo": "repo"}},
				},
			},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("skills/skill-a/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/skill-b/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
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
					Argument: mcp.CompleteParamsArgument{Name: "skill_name", Value: "skill-a"},
					Context:  &mcp.CompleteContext{Arguments: map[string]string{"owner": "owner", "repo": "repo"}},
				},
			},
			handlers: map[string]http.HandlerFunc{
				GetReposGitTreesByOwnerByRepoByTree: func(w http.ResponseWriter, _ *http.Request) {
					tree := &gogithub.Tree{Entries: []*gogithub.TreeEntry{
						{Path: gogithub.Ptr("skills/skill-a/SKILL.md"), Type: gogithub.Ptr("blob")},
						{Path: gogithub.Ptr("skills/skill-b/SKILL.md"), Type: gogithub.Ptr("blob")},
					}}
					data, _ := json.Marshal(tree)
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(data)
				},
			},
			expected: 1,
		},
		{
			name: "file_path completes to SKILL.md as default",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref:      &mcp.CompleteReference{Type: "ref/resource", URI: "skill://owner/repo/my-skill/{file_path}"},
					Argument: mcp.CompleteParamsArgument{Name: "file_path", Value: ""},
				},
			},
			handlers: map[string]http.HandlerFunc{},
			expected: 1,
		},
		{
			name: "unknown argument returns error",
			request: &mcp.CompleteRequest{
				Params: &mcp.CompleteParams{
					Ref:      &mcp.CompleteReference{Type: "ref/resource", URI: "skill://owner/repo/{skill_name}/SKILL.md"},
					Argument: mcp.CompleteParamsArgument{Name: "unknown_arg", Value: ""},
				},
			},
			handlers: map[string]http.HandlerFunc{},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := gogithub.NewClient(MockHTTPClientWithHandlers(tc.handlers))
			getClient := func(_ context.Context) (*gogithub.Client, error) { return client, nil }

			handler := SkillResourceCompletionHandler(getClient)
			result, err := handler(context.Background(), tc.request)

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

// Test_BundledSkills_TemplateInIndex_WhenSkillsToolsetEnabled verifies that
// enabling the `skills` toolset causes the per-repo skill template entry to
// appear in `skill://index.json` with `type: "mcp-resource-template"`. This
// is the SEP-2640 discovery story for parameterized skill families.
func Test_BundledSkills_TemplateInIndex_WhenSkillsToolsetEnabled(t *testing.T) {
	ctx := context.Background()
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{string(ToolsetMetadataSkills.ID)}).
		Build()
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
	})
	RegisterBundledSkills(srv, inv)

	session := connectClient(t, ctx, srv)
	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: skills.IndexURI})
	require.NoError(t, err)

	var idx skills.IndexDoc
	require.NoError(t, json.Unmarshal([]byte(res.Contents[0].Text), &idx))

	var found *skills.IndexEntry
	for i := range idx.Skills {
		if idx.Skills[i].Type == "mcp-resource-template" {
			found = &idx.Skills[i]
			break
		}
	}
	require.NotNil(t, found, "index must include an mcp-resource-template entry when skills toolset is enabled")
	assert.Equal(t, SkillResourceDiscoveryURL, found.URL)
	assert.Empty(t, found.Name, "mcp-resource-template entries omit `name` per SEP example")
	assert.NotEmpty(t, found.Description)
}

// Test_BundledSkills_TemplateAbsent_WhenSkillsToolsetDisabled verifies that
// without the `skills` toolset, the template is not advertised — but the
// always-on bundled skills still are.
func Test_BundledSkills_TemplateAbsent_WhenSkillsToolsetDisabled(t *testing.T) {
	ctx := context.Background()
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
		Build()
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
	})
	RegisterBundledSkills(srv, inv)

	session := connectClient(t, ctx, srv)
	res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: skills.IndexURI})
	require.NoError(t, err)

	var idx skills.IndexDoc
	require.NoError(t, json.Unmarshal([]byte(res.Contents[0].Text), &idx))

	for _, entry := range idx.Skills {
		assert.NotEqual(t, "mcp-resource-template", entry.Type, "template entry must not appear when skills toolset disabled")
	}
}
