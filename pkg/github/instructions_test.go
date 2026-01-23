package github

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateInstructions(t *testing.T) {
	tests := []struct {
		name            string
		enabledToolsets []string
		expectedEmpty   bool
	}{
		{
			name:            "empty toolsets",
			enabledToolsets: []string{},
			expectedEmpty:   false,
		},
		{
			name:            "only context toolset",
			enabledToolsets: []string{"context"},
			expectedEmpty:   false,
		},
		{
			name:            "pull requests toolset",
			enabledToolsets: []string{"pull_requests"},
			expectedEmpty:   false,
		},
		{
			name:            "issues toolset",
			enabledToolsets: []string{"issues"},
			expectedEmpty:   false,
		},
		{
			name:            "discussions toolset",
			enabledToolsets: []string{"discussions"},
			expectedEmpty:   false,
		},
		{
			name:            "multiple toolsets (context + pull_requests)",
			enabledToolsets: []string{"context", "pull_requests"},
			expectedEmpty:   false,
		},
		{
			name:            "multiple toolsets (issues + pull_requests)",
			enabledToolsets: []string{"issues", "pull_requests"},
			expectedEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateInstructions(tt.enabledToolsets)

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
		enabledToolsets []string
		expectedEmpty   bool
	}{
		{
			name:            "DISABLE_INSTRUCTIONS=true returns empty",
			disableEnvValue: "true",
			enabledToolsets: []string{"context", "issues", "pull_requests"},
			expectedEmpty:   true,
		},
		{
			name:            "DISABLE_INSTRUCTIONS=false returns normal instructions",
			disableEnvValue: "false",
			enabledToolsets: []string{"context"},
			expectedEmpty:   false,
		},
		{
			name:            "DISABLE_INSTRUCTIONS unset returns normal instructions",
			disableEnvValue: "",
			enabledToolsets: []string{"issues"},
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

			result := GenerateInstructions(tt.enabledToolsets)

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

func TestGetToolsetInstructions(t *testing.T) {
	tests := []struct {
		toolset              string
		expectedEmpty        bool
		enabledToolsets      []string
		expectedToContain    string
		notExpectedToContain string
	}{
		{
			toolset:           "pull_requests",
			expectedEmpty:     false,
			enabledToolsets:   []string{"pull_requests", "repos"},
			expectedToContain: "pull_request_template.md",
		},
		{
			toolset:              "pull_requests",
			expectedEmpty:        false,
			enabledToolsets:      []string{"pull_requests"},
			notExpectedToContain: "pull_request_template.md",
		},
		{
			toolset:       "issues",
			expectedEmpty: false,
		},
		{
			toolset:       "discussions",
			expectedEmpty: false,
		},
		{
			toolset:       "nonexistent",
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.toolset, func(t *testing.T) {
			result := getToolsetInstructions(tt.toolset, tt.enabledToolsets)
			if tt.expectedEmpty {
				if result != "" {
					t.Errorf("Expected empty result for toolset '%s', but got: %s", tt.toolset, result)
				}
			} else {
				if result == "" {
					t.Errorf("Expected non-empty result for toolset '%s', but got empty", tt.toolset)
				}
			}

			if tt.expectedToContain != "" && !strings.Contains(result, tt.expectedToContain) {
				t.Errorf("Expected result to contain '%s' for toolset '%s', but it did not. Result: %s", tt.expectedToContain, tt.toolset, result)
			}

			if tt.notExpectedToContain != "" && strings.Contains(result, tt.notExpectedToContain) {
				t.Errorf("Did not expect result to contain '%s' for toolset '%s', but it did. Result: %s", tt.notExpectedToContain, tt.toolset, result)
			}
		})
	}
}

// =============================================================================
// SPIKE TESTS: InstructionResolver with most-specificity rule
// =============================================================================

func TestInstructionResolver_BasicMatching(t *testing.T) {
	resolver := NewInstructionResolver([]InstructionRule{
		NewInstructionRule("rule-a", "Instruction A", "tool1"),
		NewInstructionRule("rule-b", "Instruction B", "tool2"),
		NewInstructionRule("rule-c", "Instruction C", "tool3"),
	})

	tests := []struct {
		name         string
		activeTools  []string
		expectedIDs  []string
		expectedInst []string
	}{
		{
			name:         "single tool matches single rule",
			activeTools:  []string{"tool1"},
			expectedIDs:  []string{"rule-a"},
			expectedInst: []string{"Instruction A"},
		},
		{
			name:         "two tools match two rules",
			activeTools:  []string{"tool1", "tool2"},
			expectedIDs:  []string{"rule-a", "rule-b"},
			expectedInst: []string{"Instruction A", "Instruction B"},
		},
		{
			name:         "no matching tools",
			activeTools:  []string{"tool4"},
			expectedIDs:  []string{},
			expectedInst: []string{},
		},
		{
			name:         "all tools match all rules",
			activeTools:  []string{"tool1", "tool2", "tool3"},
			expectedIDs:  []string{"rule-a", "rule-b", "rule-c"},
			expectedInst: []string{"Instruction A", "Instruction B", "Instruction C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := resolver.MatchingRuleIDs(tt.activeTools)
			instructions := resolver.ResolveInstructions(tt.activeTools)

			if len(ids) != len(tt.expectedIDs) {
				t.Errorf("Expected %d rule IDs, got %d: %v", len(tt.expectedIDs), len(ids), ids)
			}
			for i, id := range tt.expectedIDs {
				if i >= len(ids) || ids[i] != id {
					t.Errorf("Expected rule ID %s at position %d, got %v", id, i, ids)
				}
			}

			if len(instructions) != len(tt.expectedInst) {
				t.Errorf("Expected %d instructions, got %d: %v", len(tt.expectedInst), len(instructions), instructions)
			}
		})
	}
}

func TestInstructionResolver_SupersetShadowing(t *testing.T) {
	// Rule with more tools (superset) shadows rules with fewer tools
	resolver := NewInstructionResolver([]InstructionRule{
		NewInstructionRule("issues-read", "Read issues instruction", "get_issue", "list_issues"),
		NewInstructionRule("issues-all", "All issues instruction", "get_issue", "list_issues", "create_issue"),
		NewInstructionRule("create-only", "Create only instruction", "create_issue"),
	})

	tests := []struct {
		name        string
		activeTools []string
		expectedIDs []string
		description string
	}{
		{
			name:        "superset rule shadows subset rules",
			activeTools: []string{"get_issue", "list_issues", "create_issue"},
			expectedIDs: []string{"issues-all"},
			description: "issues-all shadows issues-read (superset) and create-only (superset)",
		},
		{
			name:        "no shadowing when superset rule doesn't match",
			activeTools: []string{"get_issue", "list_issues"},
			expectedIDs: []string{"issues-read"},
			description: "issues-all doesn't match (missing create_issue), so issues-read is not shadowed",
		},
		{
			name:        "single tool rule not shadowed when superset doesn't match",
			activeTools: []string{"create_issue"},
			expectedIDs: []string{"create-only"},
			description: "Only create-only matches",
		},
		{
			name:        "extra active tools don't affect shadowing",
			activeTools: []string{"get_issue", "list_issues", "create_issue", "get_me", "other_tool"},
			expectedIDs: []string{"issues-all"},
			description: "Extra tools in activeTools don't prevent shadowing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := resolver.MatchingRuleIDs(tt.activeTools)

			if len(ids) != len(tt.expectedIDs) {
				t.Errorf("%s: Expected %d rule IDs %v, got %d: %v",
					tt.description, len(tt.expectedIDs), tt.expectedIDs, len(ids), ids)
				return
			}
			for i, id := range tt.expectedIDs {
				if i >= len(ids) || ids[i] != id {
					t.Errorf("%s: Expected rule ID %s at position %d, got %v",
						tt.description, id, i, ids)
				}
			}
		})
	}
}

func TestInstructionResolver_PartialOverlapNoShadowing(t *testing.T) {
	// Rules with partial overlap (neither is superset of other) should both apply
	resolver := NewInstructionResolver([]InstructionRule{
		NewInstructionRule("rule-ab", "AB instruction", "tool_a", "tool_b"),
		NewInstructionRule("rule-bc", "BC instruction", "tool_b", "tool_c"),
	})

	// When all three tools are active, both rules match
	// Neither is a superset of the other, so neither is shadowed
	ids := resolver.MatchingRuleIDs([]string{"tool_a", "tool_b", "tool_c"})

	if len(ids) != 2 {
		t.Errorf("Expected 2 rules (partial overlap, no shadowing), got %d: %v", len(ids), ids)
	}
}

func TestInstructionResolver_ComplexHierarchy(t *testing.T) {
	// Create a hierarchy: rule1 ⊂ rule2 ⊂ rule3
	resolver := NewInstructionResolver([]InstructionRule{
		NewInstructionRule("level1", "Level 1 instruction", "t1"),
		NewInstructionRule("level2", "Level 2 instruction", "t1", "t2"),
		NewInstructionRule("level3", "Level 3 instruction", "t1", "t2", "t3"),
	})

	tests := []struct {
		name        string
		activeTools []string
		expectedIDs []string
	}{
		{
			name:        "only level1 active",
			activeTools: []string{"t1"},
			expectedIDs: []string{"level1"},
		},
		{
			name:        "level1 and level2 tools active",
			activeTools: []string{"t1", "t2"},
			expectedIDs: []string{"level2"}, // shadows level1
		},
		{
			name:        "all three levels active",
			activeTools: []string{"t1", "t2", "t3"},
			expectedIDs: []string{"level3"}, // shadows level1 and level2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids := resolver.MatchingRuleIDs(tt.activeTools)
			if len(ids) != len(tt.expectedIDs) {
				t.Errorf("Expected %v, got %v", tt.expectedIDs, ids)
			}
		})
	}
}

func TestInstructionResolver_EmptyRules(t *testing.T) {
	resolver := NewInstructionResolver([]InstructionRule{})

	ids := resolver.MatchingRuleIDs([]string{"tool1", "tool2"})
	if len(ids) != 0 {
		t.Errorf("Expected no matches for empty resolver, got %v", ids)
	}

	instructions := resolver.ResolveInstructions([]string{"tool1"})
	if len(instructions) != 0 {
		t.Errorf("Expected no instructions for empty resolver, got %v", instructions)
	}
}

func TestInstructionResolver_EmptyActiveTools(t *testing.T) {
	resolver := NewInstructionResolver([]InstructionRule{
		NewInstructionRule("rule1", "Instruction 1", "tool1"),
	})

	ids := resolver.MatchingRuleIDs([]string{})
	if len(ids) != 0 {
		t.Errorf("Expected no matches for empty active tools, got %v", ids)
	}
}

// =============================================================================
// END SPIKE TESTS
// =============================================================================
