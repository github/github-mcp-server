package github

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/skills/catalog"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/stretchr/testify/require"
)

const toolsetInstructionsSnapshot = "testdata/toolset-instructions.json"

var toolsetInstructionSkills = map[string][]string{
	"context":       {"account-and-team-context"},
	"issues":        {"issue-authoring", "issue-backlog-triage", "label-scheme-management"},
	"pull_requests": {"pull-request-authoring", "pull-request-code-review", "pull-request-review-feedback"},
	"discussions":   {"discussion-participation"},
	"projects":      {"project-planning"},
}

func TestToolsetInstructionsAreCoveredBySkills(t *testing.T) {
	inv, err := NewInventory(translations.NullTranslationHelper).
		WithToolsets([]string{"all"}).
		WithServerInstructions().
		Build()
	require.NoError(t, err)

	registry, err := catalog.New()
	require.NoError(t, err)
	entries, err := registry.List(t.Context())
	require.NoError(t, err)
	skillNames := make(map[string]bool, len(entries))
	for _, entry := range entries {
		skillNames[entry.Frontmatter.Name()] = true
	}

	actual := make(map[string]string)
	mappingsComplete := true
	for _, toolset := range inv.EnabledToolsets() {
		if toolset.InstructionsFunc == nil {
			continue
		}
		id := string(toolset.ID)
		skillNamesForInstructions, ok := toolsetInstructionSkills[id]
		if !ok {
			mappingsComplete = false
			t.Errorf("toolset %q has instructions suppressed in skills mode; update the affected skills, add their names to toolsetInstructionSkills, then run UPDATE_SKILLS_INSTRUCTIONS=true go test ./pkg/github -run TestToolsetInstructionsAreCoveredBySkills -count=1 to refresh %s", id, toolsetInstructionsSnapshot)
			continue
		}
		require.NotEmpty(t, skillNamesForInstructions, "toolset %q must name skills that replace its suppressed instructions", id)
		for _, name := range skillNamesForInstructions {
			require.True(t, skillNames[name], "toolset %q references missing skill %q", id, name)
		}
		sum := sha256.Sum256([]byte(toolset.InstructionsFunc(inv)))
		actual[id] = hex.EncodeToString(sum[:])
	}

	if os.Getenv("UPDATE_SKILLS_INSTRUCTIONS") == "true" {
		require.True(t, mappingsComplete, "add skill mappings for every toolset with an InstructionsFunc before refreshing the snapshot")
		encoded, err := json.MarshalIndent(actual, "", "  ")
		require.NoError(t, err)
		err = os.WriteFile(toolsetInstructionsSnapshot, append(encoded, '\n'), 0o600)
		require.NoError(t, err)
		return
	}

	contents, err := os.ReadFile(toolsetInstructionsSnapshot)
	require.NoError(t, err, "run UPDATE_SKILLS_INSTRUCTIONS=true go test ./pkg/github -run TestToolsetInstructionsAreCoveredBySkills -count=1 to create the snapshot after updating the affected skills")
	expected := make(map[string]string)
	require.NoError(t, json.Unmarshal(contents, &expected))
	for id, digest := range actual {
		if expected[id] != digest {
			t.Errorf("toolset %q instructions changed, but skills mode suppresses them; update skills %v, then run UPDATE_SKILLS_INSTRUCTIONS=true go test ./pkg/github -run TestToolsetInstructionsAreCoveredBySkills -count=1 to refresh %s", id, toolsetInstructionSkills[id], toolsetInstructionsSnapshot)
		}
	}
	for id := range expected {
		if _, ok := actual[id]; !ok {
			t.Errorf("toolset %q no longer has instructions; remove its stale entry from %s and rerun go test ./pkg/github -run TestToolsetInstructionsAreCoveredBySkills -count=1", id, toolsetInstructionsSnapshot)
		}
	}
}
