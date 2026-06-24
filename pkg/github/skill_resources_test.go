package github

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllSkillsCoverAllToolsets(t *testing.T) {
	// Collect all tool names from AllTools
	allToolNames := make(map[string]bool)
	for _, tool := range AllTools(stubTranslator) {
		allToolNames[tool.Tool.Name] = true
	}

	// Collect all tool names covered by skills
	coveredTools := make(map[string]bool)
	for _, skill := range allSkills() {
		for _, toolName := range skill.allowedTools {
			coveredTools[toolName] = true
		}
	}

	// Every tool should be covered by at least one skill
	for toolName := range allToolNames {
		assert.True(t, coveredTools[toolName], "tool %q is not covered by any skill", toolName)
	}
}

func TestBuildSkillContent(t *testing.T) {
	skill := skillDefinition{
		name:         "test-skill",
		description:  "A test skill",
		allowedTools: []string{"tool_a", "tool_b"},
		body:         "# Test\n\nUse these tools.\n",
	}

	content := buildSkillContent(skill)

	assert.Contains(t, content, "---\n")
	assert.Contains(t, content, "name: test-skill\n")
	assert.Contains(t, content, "description: A test skill\n")
	assert.Contains(t, content, "metadata:\n")
	assert.Contains(t, content, "  io.modelcontextprotocol/tools: \"tool_a tool_b\"\n")
	assert.Contains(t, content, "# Test\n")
}

func TestSkillResourceURIs(t *testing.T) {
	skills := allSkills()
	require.NotEmpty(t, skills)

	uris := make(map[string]bool)
	names := make(map[string]bool)

	for _, skill := range skills {
		uri := "skill://github/" + skill.name + "/SKILL.md"

		assert.False(t, uris[uri], "duplicate skill URI: %s", uri)
		uris[uri] = true

		assert.False(t, names[skill.name], "duplicate skill name: %s", skill.name)
		names[skill.name] = true

		assert.NotEmpty(t, skill.description, "skill %s has empty description", skill.name)
		assert.NotEmpty(t, skill.allowedTools, "skill %s has no allowed tools", skill.name)
		assert.NotEmpty(t, skill.body, "skill %s has empty body", skill.name)
	}
}

func TestRegisterSkillResources(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "0.0.1",
	}, nil)

	// Should not panic
	RegisterSkillResources(server)

	// Verify the expected number of skills were registered by counting definitions
	skills := allSkills()
	assert.Equal(t, 29, len(skills), "expected 29 workflow-oriented skills")
}

func TestBuildSkillIndex(t *testing.T) {
	doc := buildSkillIndex()

	assert.Equal(t, skillIndexSchema, doc.Schema)
	assert.True(t, strings.HasPrefix(doc.Schema, "https://schemas.agentskills.io/discovery/"),
		"schema must use the Agent Skills discovery prefix the runtime gates on")

	skills := allSkills()
	require.Len(t, doc.Skills, len(skills), "index must contain one entry per skill")

	byName := make(map[string]skillIndexEntry, len(doc.Skills))
	for _, entry := range doc.Skills {
		assert.Equal(t, "skill-md", entry.Type, "entry %q must be type skill-md", entry.Name)
		assert.Equal(t, "skill://github/"+entry.Name+"/SKILL.md", entry.URL,
			"entry %q url must point at its registered SKILL.md resource", entry.Name)
		assert.NotEmpty(t, entry.Description, "entry %q must carry a description", entry.Name)
		assert.NotEmpty(t, entry.AllowedTools, "entry %q must advertise its allowed tools", entry.Name)
		byName[entry.Name] = entry
	}

	// Every skill's allow-list must round-trip into the index so clients can defer those tools
	// without first fetching SKILL.md.
	for _, skill := range skills {
		entry, ok := byName[skill.name]
		require.True(t, ok, "skill %q missing from index", skill.name)
		assert.Equal(t, skill.allowedTools, entry.AllowedTools)
	}
}

func TestBuildSkillIndexJSON_IsValid(t *testing.T) {
	body, err := buildSkillIndexJSON()
	require.NoError(t, err)

	var parsed skillIndexDocument
	require.NoError(t, json.Unmarshal([]byte(body), &parsed), "index.json must be valid JSON")
	assert.Equal(t, skillIndexSchema, parsed.Schema)
	assert.Equal(t, len(allSkills()), len(parsed.Skills))
}

// TestSkillsExtensionAndIndexAdvertised verifies the SEP-2640 wire contract end-to-end: the server
// advertises the skills extension in its initialize capabilities and serves skill://index.json with
// the discovery shape skill-aware clients consume.
func TestSkillsExtensionAndIndexAdvertised(t *testing.T) {
	t.Parallel()

	cfg := MCPServerConfig{
		Version:           "test",
		Token:             "test-token",
		EnabledToolsets:   []string{"context"},
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
	}
	deps := stubDeps{obsv: stubExporters()}
	inv, err := NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithToolsets(cfg.EnabledToolsets).
		Build()
	require.NoError(t, err)

	srv, err := NewMCPServer(context.Background(), &cfg, deps, inv)
	require.NoError(t, err)

	st, ct := mcp.NewInMemoryTransports()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)

	type connResult struct {
		session *mcp.ClientSession
		err     error
	}
	connCh := make(chan connResult, 1)
	go func() {
		cs, cerr := client.Connect(context.Background(), ct, nil)
		connCh <- connResult{session: cs, err: cerr}
	}()

	ss, err := srv.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })

	got := <-connCh
	require.NoError(t, got.err)
	require.NotNil(t, got.session)
	t.Cleanup(func() { _ = got.session.Close() })

	// 1. The skills extension must be advertised so SEP-2640 clients opt into skill:// discovery.
	init := got.session.InitializeResult()
	require.NotNil(t, init)
	require.NotNil(t, init.Capabilities)
	require.Contains(t, init.Capabilities.Extensions, skillsExtensionID,
		"server must advertise the skills extension")
	// Tools/resources must still be inferred — advertising the extension must not clobber them.
	assert.NotNil(t, init.Capabilities.Tools, "tools capability must still be advertised")
	assert.NotNil(t, init.Capabilities.Resources, "resources capability must still be advertised")

	// 2. skill://index.json must serve the discovery index.
	res, err := got.session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: skillIndexURI})
	require.NoError(t, err)
	require.Len(t, res.Contents, 1)
	assert.Equal(t, "application/json", res.Contents[0].MIMEType)

	var index skillIndexDocument
	require.NoError(t, json.Unmarshal([]byte(res.Contents[0].Text), &index))
	assert.True(t, strings.HasPrefix(index.Schema, "https://schemas.agentskills.io/discovery/"))
	require.Len(t, index.Skills, len(allSkills()))
	for _, entry := range index.Skills {
		assert.Equal(t, "skill-md", entry.Type)
		assert.NotEmpty(t, entry.AllowedTools)
	}
}
