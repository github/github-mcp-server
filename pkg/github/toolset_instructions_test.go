package github

import (
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
)

func TestGenerateContextToolsetInstructionsIncludesAutolinkGuidance(t *testing.T) {
	instructions := generateContextToolsetInstructions(&inventory.Inventory{})

	for _, expected := range []string{
		"#123",
		"@example-username",
		"example-owner/example-repo#123",
		"https://docs.github.com/en/get-started/writing-on-github/working-with-advanced-formatting/autolinked-references-and-urls",
	} {
		if !strings.Contains(instructions, expected) {
			t.Fatalf("expected context instructions to include %q, got %q", expected, instructions)
		}
	}
}
