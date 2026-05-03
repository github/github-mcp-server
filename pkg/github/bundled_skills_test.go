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

// reviewPRSkillURI / handleNotificationsSkillURI are the canonical URIs of the
// two bundled skills, derived from skills.Bundled so tests never drift from
// the single source of truth.
var (
	reviewPRSkillURI            = skills.Bundled{Name: "review-pr"}.URI()
	handleNotificationsSkillURI = skills.Bundled{Name: "handle-notifications"}.URI()
)

// Test_ReviewPRSkill_EmbeddedContent verifies the SEP structural requirement
// that the frontmatter `name` field matches the final segment of the skill-path
// in the URI, and that the substantive tool-sequence content is preserved.
func Test_ReviewPRSkill_EmbeddedContent(t *testing.T) {
	require.NotEmpty(t, skills.ReviewPRSKILL, "SKILL.md must be embedded")

	// Normalize line endings so the test is robust to git's autocrlf behavior
	// on Windows checkouts — the embedded SKILL.md may arrive as CRLF.
	md := strings.ReplaceAll(skills.ReviewPRSKILL, "\r\n", "\n")
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
	assert.Equal(t, "review-pr", frontmatterName, "frontmatter name must match final skill-path segment in %s", reviewPRSkillURI)

	body := md[4+end+5:]
	assert.Contains(t, body, "## Workflow", "skill body must carry the workflow section")
	assert.Contains(t, body, "pull_request_review_write", "review workflow content must be preserved")
	assert.Contains(t, body, "add_comment_to_pending_review", "review workflow content must be preserved")
	assert.Contains(t, body, "submit_pending", "the distinctive tool method must be present")
}

// Test_HandleNotificationsSkill_EmbeddedContent verifies the SEP structural
// requirements for the handle-notifications skill and that its substantive
// tool references are preserved.
func Test_HandleNotificationsSkill_EmbeddedContent(t *testing.T) {
	require.NotEmpty(t, skills.HandleNotificationsSKILL, "SKILL.md must be embedded")

	md := strings.ReplaceAll(skills.HandleNotificationsSKILL, "\r\n", "\n")
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
	assert.Equal(t, "handle-notifications", frontmatterName, "frontmatter name must match final skill-path segment in %s", handleNotificationsSkillURI)

	body := md[4+end+5:]
	assert.Contains(t, body, "## Workflow")
	assert.Contains(t, body, "list_notifications", "triage workflow must reference list_notifications")
	assert.Contains(t, body, "dismiss_notification", "triage workflow must reference dismiss_notification")
}

// Test_BundledSkills_RegisterRegardlessOfToolset verifies that bundled skills
// load uniformly — they're always-on and don't depend on which toolsets the
// inventory has enabled. Per-skill toolset gating remains available via the
// Registry's Enabled closure but no shipped skill currently uses it.
func Test_BundledSkills_RegisterRegardlessOfToolset(t *testing.T) {
	ctx := context.Background()

	// Pick a minimal toolset (context only) — doesn't enable pull_requests
	// or notifications, the toolsets the renamed skills used to be gated on.
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
		Build()
	require.NoError(t, err)

	srv := mcp.NewServer(&mcp.Implementation{Name: "test"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Resources: &mcp.ResourceCapabilities{}},
	})
	RegisterBundledSkills(srv, inv)

	uris := map[string]string{}
	for _, r := range listResources(t, ctx, srv) {
		uris[r.URI] = r.MIMEType
	}
	assert.Equal(t, "text/markdown", uris[reviewPRSkillURI], "review-pr registers regardless of toolset")
	assert.Equal(t, "text/markdown", uris[handleNotificationsSkillURI], "handle-notifications registers regardless of toolset")
	assert.Equal(t, "application/json", uris[skills.IndexURI])
}

// Test_BundledSkills_ReadContent verifies that reading the skill resource
// returns the embedded SKILL.md content, and the index resource returns a JSON
// document matching the SEP discovery schema shape.
func Test_BundledSkills_ReadContent(t *testing.T) {
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

	t.Run("SKILL.md content", func(t *testing.T) {
		res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: reviewPRSkillURI})
		require.NoError(t, err)
		require.Len(t, res.Contents, 1)
		assert.Equal(t, "text/markdown", res.Contents[0].MIMEType)
		assert.Equal(t, skills.ReviewPRSKILL, res.Contents[0].Text)
	})

	t.Run("index.json matches SEP discovery schema", func(t *testing.T) {
		res, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: skills.IndexURI})
		require.NoError(t, err)
		require.Len(t, res.Contents, 1)
		assert.Equal(t, "application/json", res.Contents[0].MIMEType)

		var idx skills.IndexDoc
		require.NoError(t, json.Unmarshal([]byte(res.Contents[0].Text), &idx))
		assert.Equal(t, skills.IndexSchema, idx.Schema)

		// Index size must equal the number of currently-enabled bundled skills.
		assert.Len(t, idx.Skills, len(bundledSkills(inv).Enabled()))

		// The review-pr skill must be present.
		var found *skills.IndexEntry
		for i := range idx.Skills {
			if idx.Skills[i].Name == "review-pr" {
				found = &idx.Skills[i]
				break
			}
		}
		require.NotNil(t, found, "review-pr must appear in the index")
		assert.Equal(t, "skill-md", found.Type)
		assert.Equal(t, reviewPRSkillURI, found.URL)
		assert.NotEmpty(t, found.Description)
	})
}

// Test_BundledSkills_Index_MultipleSkills verifies that all bundled skills
// appear in the discovery index, not just the first one.
func Test_BundledSkills_Index_MultipleSkills(t *testing.T) {
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
	names := map[string]string{}
	for _, s := range idx.Skills {
		names[s.Name] = s.URL
	}
	assert.Equal(t, reviewPRSkillURI, names["review-pr"])
	assert.Equal(t, handleNotificationsSkillURI, names["handle-notifications"])
}

// Test_DeclareSkillsExtensionIfEnabled verifies that the skills-over-MCP
// extension (SEP-2133) is declared in ServerOptions.Capabilities whenever the
// server has any bundled skill or template entry to publish, and that
// declaration is additive (doesn't clobber other extensions).
func Test_DeclareSkillsExtensionIfEnabled(t *testing.T) {
	t.Run("declares for any non-empty registry", func(t *testing.T) {
		// All bundled skills are always-on, so any inventory triggers the declaration.
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
			Build()
		require.NoError(t, err)

		opts := &mcp.ServerOptions{}
		DeclareSkillsExtensionIfEnabled(opts, inv)

		require.NotNil(t, opts.Capabilities)
		_, ok := opts.Capabilities.Extensions[skills.ExtensionKey]
		assert.True(t, ok, "always-on skills register, so the extension must be declared")
	})

	t.Run("preserves other extensions already declared", func(t *testing.T) {
		inv, err := NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{string(ToolsetMetadataContext.ID)}).
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

	t.Run("does not declare for an empty registry", func(t *testing.T) {
		// Tests the Registry's failure path directly — bypasses bundledSkills()
		// since no shipped configuration produces an empty registry.
		opts := &mcp.ServerOptions{}
		skills.New().DeclareCapability(opts)
		if opts.Capabilities != nil {
			_, ok := opts.Capabilities.Extensions[skills.ExtensionKey]
			assert.False(t, ok, "empty registry must not declare the extension")
		}
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
