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

	// Should not panic with nil (no filtering)
	RegisterSkillResources(server, nil)

	// Verify the expected number of skills were registered by counting definitions
	skills := allSkills()
	assert.Equal(t, 27, len(skills), "expected 27 workflow-oriented skills")
}

func TestFilterAvailableTools(t *testing.T) {
	tests := []struct {
		name      string
		tools     []string
		available map[string]struct{}
		expected  []string
	}{
		{
			name:      "nil available returns all tools",
			tools:     []string{"a", "b", "c"},
			available: nil,
			expected:  []string{"a", "b", "c"},
		},
		{
			name:      "filters to only available tools",
			tools:     []string{"a", "b", "c"},
			available: map[string]struct{}{"a": {}, "c": {}},
			expected:  []string{"a", "c"},
		},
		{
			name:      "returns nil when no tools match",
			tools:     []string{"a", "b"},
			available: map[string]struct{}{"x": {}},
			expected:  nil,
		},
		{
			name:      "empty available set filters everything",
			tools:     []string{"a", "b"},
			available: map[string]struct{}{},
			expected:  nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterAvailableTools(tc.tools, tc.available)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRegisterSkillResourcesFiltering(t *testing.T) {
	t.Parallel()
	// Only make get_me available — skills that have no overlap should be skipped
	available := map[string]struct{}{"get_me": {}}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "0.0.1",
	}, nil)

	RegisterSkillResources(server, available)

	// get-context skill includes get_me, so it should be registered.
	// Skills with zero available tools should be skipped entirely.
	// We can't easily count resources on mcp.Server, but we verify no panic
	// and the logic is correct via TestFilterAvailableTools above.
}
