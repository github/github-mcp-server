package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueToFixWorkflowPrompt(t *testing.T) {
	prompt, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	// Verify prompt definition
	assert.Equal(t, "IssueToFixWorkflow", prompt.Name)
	assert.NotEmpty(t, prompt.Description)

	// Verify required arguments
	require.NotNil(t, prompt.Arguments)
	assert.Len(t, prompt.Arguments, 6) // owner, repo, title, description, labels, assignees

	// Check required arguments
	hasOwner := false
	hasRepo := false
	hasTitle := false
	hasDescription := false
	for _, arg := range prompt.Arguments {
		if arg.Name == "owner" && arg.Required {
			hasOwner = true
		}
		if arg.Name == "repo" && arg.Required {
			hasRepo = true
		}
		if arg.Name == "title" && arg.Required {
			hasTitle = true
		}
		if arg.Name == "description" && arg.Required {
			hasDescription = true
		}
	}

	assert.True(t, hasOwner, "Should have required 'owner' argument")
	assert.True(t, hasRepo, "Should have required 'repo' argument")
	assert.True(t, hasTitle, "Should have required 'title' argument")
	assert.True(t, hasDescription, "Should have required 'description' argument")

	// Test handler is not nil
	assert.NotNil(t, handler)
}

func TestIssueToFixWorkflowPrompt_Handler(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	tests := []struct {
		name        string
		arguments   map[string]string
		expectError bool
	}{
		{
			name: "valid arguments with all fields",
			arguments: map[string]string{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"title":       "Fix bug in login",
				"description": "Users cannot login with special characters in password",
				"labels":      "bug,high-priority",
				"assignees":   "developer1,developer2",
			},
			expectError: false,
		},
		{
			name: "valid arguments without optional fields",
			arguments: map[string]string{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"title":       "Add new feature",
				"description": "Need to implement dark mode",
			},
			expectError: false,
		},
		{
			name: "with labels only",
			arguments: map[string]string{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"title":       "Update documentation",
				"description": "Docs are outdated",
				"labels":      "documentation",
			},
			expectError: false,
		},
		{
			name: "with assignees only",
			arguments: map[string]string{
				"owner":       "test-owner",
				"repo":        "test-repo",
				"title":       "Refactor code",
				"description": "Clean up legacy code",
				"assignees":   "maintainer",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: struct {
					Name      string            `json:"name"`
					Arguments map[string]string `json:"arguments,omitempty"`
				}{
					Name:      "IssueToFixWorkflow",
					Arguments: tt.arguments,
				},
			}

			result, err := handler(nil, request)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			// Verify result has messages
			assert.NotEmpty(t, result.Messages, "Result should contain prompt messages")

			// Verify messages are present - they should reference the workflow
			// The exact content may vary based on implementation
		})
	}
}

func TestIssueToFixWorkflowPrompt_MessageStructure(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "facebook",
				"repo":        "react",
				"title":       "Performance issue",
				"description": "App is slow when rendering large lists",
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Should have multiple messages for conversation flow
	assert.GreaterOrEqual(t, len(result.Messages), 2, "Should have at least 2 messages for workflow")

	// Check roles are present
	hasUserRole := false
	hasAssistantRole := false
	for _, msg := range result.Messages {
		if msg.Role == "user" {
			hasUserRole = true
		}
		if msg.Role == "assistant" {
			hasAssistantRole = true
		}
	}
	assert.True(t, hasUserRole, "Should have user role messages")
	assert.True(t, hasAssistantRole, "Should have assistant role messages")
}

func TestIssueToFixWorkflowPrompt_WithLabels(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "owner",
				"repo":        "repo",
				"title":       "Bug fix",
				"description": "Fix the bug",
				"labels":      "bug,urgent,security",
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Messages, "Should have messages in result")

	// Labels are included via the handler logic
	// Just verify the result is valid
}

func TestIssueToFixWorkflowPrompt_WithAssignees(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "owner",
				"repo":        "repo",
				"title":       "Feature request",
				"description": "Add new feature",
				"assignees":   "dev1,dev2,dev3",
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Messages, "Should have messages in result")

	// Assignees are included via the handler logic
	// Just verify the result is valid
}

func TestIssueToFixWorkflowPrompt_SpecialCharactersInTitle(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	titles := []string{
		"Bug: Can't login with special chars!",
		"Feature [High Priority]",
		"Fix \"quote\" handling",
		"Update (dependencies)",
	}

	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			request := mcp.GetPromptRequest{
				Params: struct {
					Name      string            `json:"name"`
					Arguments map[string]string `json:"arguments,omitempty"`
				}{
					Name: "IssueToFixWorkflow",
					Arguments: map[string]string{
						"owner":       "test",
						"repo":        "test",
						"title":       title,
						"description": "Test description",
					},
				},
			}

			result, err := handler(nil, request)
			require.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, result.Messages)
		})
	}
}

func TestIssueToFixWorkflowPrompt_LongDescription(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	// Create a long description
	longDesc := "This is a very long description. "
	for i := 0; i < 50; i++ {
		longDesc += "It contains multiple sentences explaining the issue in detail. "
	}

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "test",
				"repo":        "test",
				"title":       "Complex issue",
				"description": longDesc,
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Messages)
}

func TestIssueToFixWorkflowPrompt_EmptyOptionalFields(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "test",
				"repo":        "test",
				"title":       "Simple issue",
				"description": "Simple description",
				"labels":      "",
				"assignees":   "",
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Messages)
}

func TestIssueToFixWorkflowPrompt_WorkflowGuidance(t *testing.T) {
	_, handler := IssueToFixWorkflowPrompt(translations.NullTranslationHelper)

	request := mcp.GetPromptRequest{
		Params: struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}{
			Name: "IssueToFixWorkflow",
			Arguments: map[string]string{
				"owner":       "test",
				"repo":        "test",
				"title":       "Test",
				"description": "Test",
			},
		},
	}

	result, err := handler(nil, request)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Check that the workflow provides guidance about the process
	foundWorkflowGuidance := false
	workflowKeywords := []string{"create", "issue", "Copilot", "PR", "pull request", "fix"}
	for _, msg := range result.Messages {
		if textContent, ok := msg.Content.(mcp.TextContent); ok {
			for _, keyword := range workflowKeywords {
				if assert.Contains(t, textContent.Text, keyword) {
					foundWorkflowGuidance = true
					break
				}
			}
			if foundWorkflowGuidance {
				break
			}
		}
	}
	assert.True(t, foundWorkflowGuidance, "Should provide workflow guidance")
}

