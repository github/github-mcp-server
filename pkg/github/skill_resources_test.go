package github

import (
	"testing"

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
	assert.Contains(t, content, "  - tool_a\n")
	assert.Contains(t, content, "  - tool_b\n")
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
	assert.Equal(t, 16, len(skills), "expected 16 skills covering all toolsets")
}
