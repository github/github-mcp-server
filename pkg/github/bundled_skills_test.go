package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/skills"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pullRequestsSkillURI is the canonical URI of the bundled pull-requests
// skill, derived from the skills.Bundled entry so tests never drift from
// the single source of truth.
var pullRequestsSkillURI = skills.Bundled{Name: "pull-requests"}.URI()

// Test_PullRequestsSkill_EmbeddedContent verifies the SEP structural requirement
// that the frontmatter `name` field matches the final segment of the skill-path
// in the URI, and that the substantive tool-sequence content is preserved.
func Test_PullRequestsSkill_EmbeddedContent(t *testing.T) {
	require.NotEmpty(t, skills.PullRequestsSKILL, "SKILL.md must be embedded")

	// Normalize line endings so the test is robust to git's autocrlf behavior
	// on Windows checkouts — the embedded SKILL.md may arrive as CRLF.
	md := strings.ReplaceAll(skills.PullRequestsSKILL, "\r\n", "\n")
	require.True(t, strings.HasPrefix(md, "---\n"), "SKILL.md must begin with YAML frontmatter")

	end := strings.Index(md[4:], "\n---\n")
	require.GreaterOrEqual(t, end, 0, "SKILL.md must have closing frontmatter fence")
	frontmatter := md[4 : 4+end]

	var frontmatterName string
	for _, line := range strings.Split(frontmatter, "\n") {
		if strings.HasPrefix(line, "name:") {
			frontmatterName = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			break
		}
	}
	require.NotEmpty(t, frontmatterName, "SKILL.md frontmatter must declare `name`")
	assert.Equal(t, "pull-requests", frontmatterName, "frontmatter name must match final skill-path segment in %s", pullRequestsSkillURI)

	body := md[4+end+5:]
	assert.Contains(t, body, "## Workflow", "skill body must carry the workflow section")
	assert.Contains(t, body, "pull_request_review_write", "review workflow content must be preserved")
	assert.Contains(t, body, "add_comment_to_pending_review", "review workflow content must be preserved")
	assert.Contains(t, body, "submit_pending", "the distinctive tool method must be present")
}

// Test_BundledSkills_Registration verifies that skill resources are
// registered when the backing toolset is enabled, and omitted when it is not.
func Test_BundledSkills_Registration(t *testing.T) {
	ctx := context.Background()

	t.Run("registers when pull_requests toolset enabled", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataPullRequests.ID)}).
			Build()
		require.NoError(t, err)

		srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
		})
		RegisterBundledSkills(srv, inv)

		mimes := map[string]string{}
		for _, r := range listResources(t, ctx, srv) {
			mimes[r.URI] = r.MIMEType
		}
		assert.Equal(t, "text/markdown", mimes[pullRequestsSkillURI])
		assert.Equal(t, "application/json", mimes[skills.IndexURI])
	})

	t.Run("omits when pull_requests toolset disabled", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
			Build()
		require.NoError(t, err)

		srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
		})
		RegisterBundledSkills(srv, inv)

		for _, r := range listResources(t, ctx, srv) {
			assert.NotEqual(t, pullRequestsSkillURI, r.URI)
			assert.NotEqual(t, skills.IndexURI, r.URI)
		}
	})
}

// Test_BundledSkills_ReadContent verifies that reading the skill resource
// returns the embedded SKILL.md content, and the index resource returns a JSON
// document matching the SEP discovery schema shape.
func Test_BundledSkills_ReadContent(t *testing.T) {
	ctx := context.Background()
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{string(ToolsetMetadataPullRequests.ID)}).
		Build()
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
	})
	RegisterBundledSkills(srv, inv)

	session := connectClient(t, ctx, srv)

	t.Run("SKILL.md content", func(t *testing.T) {
		res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: pullRequestsSkillURI})
		require.NoError(t, err)
		require.Len(t, res.Contents, 1)
		assert.Equal(t, "text/markdown", res.Contents[0].MIMEType)
		assert.Equal(t, skills.PullRequestsSKILL, res.Contents[0].Text)
	})

	t.Run("index.json matches SEP discovery schema", func(t *testing.T) {
		res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: skills.IndexURI})
		require.NoError(t, err)
		require.Len(t, res.Contents, 1)
		assert.Equal(t, "application/json", res.Contents[0].MIMEType)

		var idx skills.IndexDoc
		require.NoError(t, json.Unmarshal([]byte(res.Contents[0].Text), &idx))
		assert.Equal(t, skills.IndexSchema, idx.Schema)
		require.Len(t, idx.Skills, 1)
		assert.Equal(t, "pull-requests", idx.Skills[0].Name)
		assert.Equal(t, "skill-md", idx.Skills[0].Type)
		assert.Equal(t, pullRequestsSkillURI, idx.Skills[0].URL)
		assert.NotEmpty(t, idx.Skills[0].Description)
	})
}

// Test_DeclareSkillsExtensionIfEnabled verifies that the skills-over-MCP
// extension (SEP-2133) is declared in ServerOptions.Capabilities when the
// pull_requests toolset is enabled, and is absent when it is not.
func Test_DeclareSkillsExtensionIfEnabled(t *testing.T) {
	t.Run("declares when pull_requests enabled", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataPullRequests.ID)}).
			Build()
		require.NoError(t, err)

		opts := &mcp.ServerOptions{}
		DeclareSkillsExtensionIfEnabled(opts, inv)

		require.NotNil(t, opts.Capabilities)
		_, ok := opts.Capabilities.Extensions[skills.ExtensionKey]
		assert.True(t, ok, "skills extension must be declared")
	})

	t.Run("does not declare when pull_requests disabled", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
			Build()
		require.NoError(t, err)

		opts := &mcp.ServerOptions{}
		DeclareSkillsExtensionIfEnabled(opts, inv)

		if opts.Capabilities != nil {
			_, ok := opts.Capabilities.Extensions[skills.ExtensionKey]
			assert.False(t, ok, "skills extension must NOT be declared when no skills will be registered")
		}
	})

	t.Run("preserves other extensions already declared", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataPullRequests.ID)}).
			Build()
		require.NoError(t, err)

		opts := &mcp.ServerOptions{
			Capabilities: &mcp.ServerCapabilities{},
		}
		opts.Capabilities.AddExtension("io.example/other", map[string]any{"k": "v"})

		DeclareSkillsExtensionIfEnabled(opts, inv)

		_, hasSkills := opts.Capabilities.Extensions[skills.ExtensionKey]
		_, hasOther := opts.Capabilities.Extensions["io.example/other"]
		assert.True(t, hasSkills)
		assert.True(t, hasOther, "existing extensions must not be overwritten")
	})
}

// listResources enumerates resources/list via an in-memory client session.
func listResources(t *testing.T, ctx context.Context, srv *mcp.Server) []*mcp.Resource {
	t.Helper()
	session := connectClient(t, ctx, srv)
	res, err := session.ListResources(ctx, &mcp.ListResourcesParams{})
	require.NoError(t, err)
	return res.Resources
}

// connectClient wires an in-memory transport and returns a connected client session.
func connectClient(t *testing.T, ctx context.Context, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientT, serverT := mcp.NewInMemoryTransports()
	_, err := srv.Connect(ctx, serverT, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}
