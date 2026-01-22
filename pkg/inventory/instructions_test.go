package inventory

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateInstructions(t *testing.T) {
	tests := []struct {
		name            string
		enabledToolsets []ToolsetID
		expectedEmpty   bool
	}{
		{
			name:            "empty toolsets",
			enabledToolsets: []ToolsetID{},
			expectedEmpty:   false,
		},
		{
			name:            "only context toolset",
			enabledToolsets: []ToolsetID{"context"},
			expectedEmpty:   false,
		},
		{
			name:            "pull requests toolset",
			enabledToolsets: []ToolsetID{"pull_requests"},
			expectedEmpty:   false,
		},
		{
			name:            "issues toolset",
			enabledToolsets: []ToolsetID{"issues"},
			expectedEmpty:   false,
		},
		{
			name:            "discussions toolset",
			enabledToolsets: []ToolsetID{"discussions"},
			expectedEmpty:   false,
		},
		{
			name:            "multiple toolsets (context + pull_requests)",
			enabledToolsets: []ToolsetID{"context", "pull_requests"},
			expectedEmpty:   false,
		},
		{
			name:            "multiple toolsets (issues + pull_requests)",
			enabledToolsets: []ToolsetID{"issues", "pull_requests"},
			expectedEmpty:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateInstructions(tt.enabledToolsets)

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
		enabledToolsets []ToolsetID
		expectedEmpty   bool
	}{
		{
			name:            "DISABLE_INSTRUCTIONS=true returns empty",
			disableEnvValue: "true",
			enabledToolsets: []ToolsetID{"context", "issues", "pull_requests"},
			expectedEmpty:   true,
		},
		{
			name:            "DISABLE_INSTRUCTIONS=false returns normal instructions",
			disableEnvValue: "false",
			enabledToolsets: []ToolsetID{"context"},
			expectedEmpty:   false,
		},
		{
			name:            "DISABLE_INSTRUCTIONS unset returns normal instructions",
			disableEnvValue: "",
			enabledToolsets: []ToolsetID{"issues"},
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

			result := generateInstructions(tt.enabledToolsets)

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
		enabledToolsets      []ToolsetID
		expectedToContain    string
		notExpectedToContain string
	}{
		{
			toolset:           "pull_requests",
			expectedEmpty:     false,
			enabledToolsets:   []ToolsetID{"pull_requests", "repos"},
			expectedToContain: "pull_request_template.md",
		},
		{
			toolset:              "pull_requests",
			expectedEmpty:        false,
			enabledToolsets:      []ToolsetID{"pull_requests"},
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
			result := getToolsetInstructions(ToolsetID(tt.toolset), tt.enabledToolsets)
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
