package inventory

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// createTestInventory creates an inventory with the specified toolsets for testing.
// All toolsets are enabled by default using WithToolsets([]string{"all"}).
func createTestInventory(toolsets []ToolsetMetadata) *Inventory {
	// Create tools for each toolset so they show up in AvailableToolsets()
	var tools []ServerTool
	for _, ts := range toolsets {
		tools = append(tools, ServerTool{
			Toolset: ts,
		})
	}

	inv, _ := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"all"}).
		Build()

	return inv
}

func TestGenerateInstructions(t *testing.T) {
	tests := []struct {
		name          string
		toolsets      []ToolsetMetadata
		expectedEmpty bool
	}{
		{
			name:          "empty toolsets",
			toolsets:      []ToolsetMetadata{},
			expectedEmpty: false, // base instructions are always included
		},
		{
			name: "toolset with instructions",
			toolsets: []ToolsetMetadata{
				{
					ID:          "test",
					Description: "Test toolset",
					InstructionsFunc: func(_ *Inventory) string {
						return "Test instructions"
					},
				},
			},
			expectedEmpty: false,
		},
		{
			name: "toolset without instructions",
			toolsets: []ToolsetMetadata{
				{
					ID:          "test",
					Description: "Test toolset",
				},
			},
			expectedEmpty: false, // base instructions still included
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := createTestInventory(tt.toolsets)
			result := generateInstructions(inv)

			if tt.expectedEmpty {
				if result != "" {
					t.Errorf("Expected empty instructions but got: %s", result)
				}
			} else {
				if result == "" {
					t.Errorf("Expected non-empty instructions but got empty result")
				}
			}
		})
	}
}

func TestGenerateInstructionsWithDisableFlag(t *testing.T) {
	tests := []struct {
		name            string
		disableEnvValue string
		expectedEmpty   bool
	}{
		{
			name:            "DISABLE_INSTRUCTIONS=true returns empty",
			disableEnvValue: "true",
			expectedEmpty:   true,
		},
		{
			name:            "DISABLE_INSTRUCTIONS=false returns normal instructions",
			disableEnvValue: "false",
			expectedEmpty:   false,
		},
		{
			name:            "DISABLE_INSTRUCTIONS unset returns normal instructions",
			disableEnvValue: "",
			expectedEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original env value
			originalValue := os.Getenv("DISABLE_INSTRUCTIONS")
			defer func() {
				if originalValue == "" {
					os.Unsetenv("DISABLE_INSTRUCTIONS")
				} else {
					os.Setenv("DISABLE_INSTRUCTIONS", originalValue)
				}
			}()

			// Set test env value
			if tt.disableEnvValue == "" {
				os.Unsetenv("DISABLE_INSTRUCTIONS")
			} else {
				os.Setenv("DISABLE_INSTRUCTIONS", tt.disableEnvValue)
			}

			inv := createTestInventory([]ToolsetMetadata{
				{ID: "test", Description: "Test"},
			})
			result := generateInstructions(inv)

			if tt.expectedEmpty {
				if result != "" {
					t.Errorf("Expected empty instructions but got: %s", result)
				}
			} else {
				if result == "" {
					t.Errorf("Expected non-empty instructions but got empty result")
				}
			}
		})
	}
}

func TestToolsetInstructionsFunc(t *testing.T) {
	tests := []struct {
		name                 string
		toolsets             []ToolsetMetadata
		expectedToContain    string
		notExpectedToContain string
	}{
		{
			name: "toolset with context-aware instructions includes extra text when dependency present",
			toolsets: []ToolsetMetadata{
				{ID: "repos", Description: "Repos"},
				{
					ID:          "pull_requests",
					Description: "PRs",
					InstructionsFunc: func(inv *Inventory) string {
						instructions := "PR base instructions"
						if inv.HasToolset("repos") {
							instructions += " PR template instructions"
						}
						return instructions
					},
				},
			},
			expectedToContain: "PR template instructions",
		},
		{
			name: "toolset with context-aware instructions excludes extra text when dependency missing",
			toolsets: []ToolsetMetadata{
				{
					ID:          "pull_requests",
					Description: "PRs",
					InstructionsFunc: func(inv *Inventory) string {
						instructions := "PR base instructions"
						if inv.HasToolset("repos") {
							instructions += " PR template instructions"
						}
						return instructions
					},
				},
			},
			notExpectedToContain: "PR template instructions",
		},
		{
			name: "toolset without InstructionsFunc returns no toolset-specific instructions",
			toolsets: []ToolsetMetadata{
				{ID: "test", Description: "Test without instructions"},
			},
			notExpectedToContain: "## Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := createTestInventory(tt.toolsets)
			result := generateInstructions(inv)

			if tt.expectedToContain != "" && !strings.Contains(result, tt.expectedToContain) {
				t.Errorf("Expected result to contain '%s', but it did not. Result: %s", tt.expectedToContain, result)
			}

			if tt.notExpectedToContain != "" && strings.Contains(result, tt.notExpectedToContain) {
				t.Errorf("Did not expect result to contain '%s', but it did. Result: %s", tt.notExpectedToContain, result)
			}
		})
	}
}

// TestGenerateInstructionsOnlyEnabledToolsets verifies that generateInstructions
// only includes instructions from enabled toolsets, not all available toolsets.
// This is a regression test for https://github.com/github/github-mcp-server/issues/1897
func TestGenerateInstructionsOnlyEnabledToolsets(t *testing.T) {
	// Create tools for multiple toolsets
	reposToolset := ToolsetMetadata{
		ID:          "repos",
		Description: "Repository tools",
		InstructionsFunc: func(_ *Inventory) string {
			return "REPOS_INSTRUCTIONS"
		},
	}
	issuesToolset := ToolsetMetadata{
		ID:          "issues",
		Description: "Issue tools",
		InstructionsFunc: func(_ *Inventory) string {
			return "ISSUES_INSTRUCTIONS"
		},
	}
	prsToolset := ToolsetMetadata{
		ID:          "pull_requests",
		Description: "PR tools",
		InstructionsFunc: func(_ *Inventory) string {
			return "PRS_INSTRUCTIONS"
		},
	}

	tools := []ServerTool{
		{Toolset: reposToolset},
		{Toolset: issuesToolset},
		{Toolset: prsToolset},
	}

	// Build inventory with only "repos" toolset enabled
	inv, err := NewBuilder().
		SetTools(tools).
		WithToolsets([]string{"repos"}).
		Build()
	if err != nil {
		t.Fatalf("Failed to build inventory: %v", err)
	}

	result := generateInstructions(inv)

	// Should contain instructions from enabled toolset
	if !strings.Contains(result, "REPOS_INSTRUCTIONS") {
		t.Errorf("Expected instructions to contain 'REPOS_INSTRUCTIONS' for enabled toolset, but it did not. Result: %s", result)
	}

	// Should NOT contain instructions from non-enabled toolsets
	if strings.Contains(result, "ISSUES_INSTRUCTIONS") {
		t.Errorf("Did not expect instructions to contain 'ISSUES_INSTRUCTIONS' for disabled toolset, but it did. Result: %s", result)
	}
	if strings.Contains(result, "PRS_INSTRUCTIONS") {
		t.Errorf("Did not expect instructions to contain 'PRS_INSTRUCTIONS' for disabled toolset, but it did. Result: %s", result)
	}
}

func TestToolsetInstructionsOmitWriteGuidanceWhenWriteToolsAreFiltered(t *testing.T) {
	issuesToolset := ToolsetMetadata{
		ID:          "issues",
		Description: "Issue tools",
		InstructionsFunc: func(inv *Inventory) string {
			instructions := "Use search_issues before creating new issues."
			if inv.HasAvailableTool(context.Background(), "issue_write") {
				instructions += " Always set state_reason when closing issues."
			}
			return instructions
		},
	}

	tools := []ServerTool{
		{
			Tool:    mcp.Tool{Name: "search_issues", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}},
			Toolset: issuesToolset,
		},
		{
			Tool:    mcp.Tool{Name: "issue_write", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false}},
			Toolset: issuesToolset,
		},
	}

	reg := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithServerInstructions())
	if !strings.Contains(reg.instructions, "Always set state_reason when closing issues.") {
		t.Fatalf("Expected write guidance when issue_write is available, got %q", reg.instructions)
	}

	readOnly := mustBuild(t, NewBuilder().SetTools(tools).WithToolsets([]string{"all"}).WithReadOnly(true).WithServerInstructions())
	if strings.Contains(readOnly.instructions, "Always set state_reason when closing issues.") {
		t.Fatalf("Did not expect write guidance in read-only mode, got %q", readOnly.instructions)
	}
}
