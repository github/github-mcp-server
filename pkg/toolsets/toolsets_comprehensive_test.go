package toolsets

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create test tool with proper annotations
func createTestToolWithAnnotation(name string, readOnly bool) server.ServerTool {
	tool := mcp.NewTool(name)
	tool.Annotations = mcp.ToolAnnotation{
		ReadOnlyHint: &readOnly,
	}
	handler := server.ToolHandlerFunc(func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("test"), nil
	})
	return NewServerTool(tool, handler)
}

func TestToolsetDoesNotExistError_Error(t *testing.T) {
	err := &ToolsetDoesNotExistError{Name: "test-toolset"}
	assert.Equal(t, "toolset test-toolset does not exist", err.Error())
}

func TestToolsetDoesNotExistError_Is(t *testing.T) {
	err1 := &ToolsetDoesNotExistError{Name: "toolset1"}
	err2 := &ToolsetDoesNotExistError{Name: "toolset2"}
	otherErr := errors.New("different error")

	// Should match any ToolsetDoesNotExistError
	assert.True(t, err1.Is(err2))
	assert.True(t, err2.Is(err1))

	// Should not match nil
	assert.False(t, err1.Is(nil))

	// Should not match other error types
	assert.False(t, err1.Is(otherErr))
}

func TestNewServerTool(t *testing.T) {
	tool := mcp.NewTool("test-tool")
	handler := server.ToolHandlerFunc(func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("result"), nil
	})

	serverTool := NewServerTool(tool, handler)

	assert.Equal(t, tool, serverTool.Tool)
	assert.NotNil(t, serverTool.Handler)

	// Test that handler works
	result, err := serverTool.Handler(context.Background(), mcp.CallToolRequest{})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewServerResourceTemplate(t *testing.T) {
	template := mcp.NewResourceTemplate("test://resource", "Test resource")
	handler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})

	serverTemplate := NewServerResourceTemplate(template, handler)

	assert.Equal(t, template, serverTemplate.Template)
	assert.NotNil(t, serverTemplate.Handler)

	// Test that handler works
	result, err := serverTemplate.Handler(context.Background(), mcp.ReadResourceRequest{})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewServerPrompt(t *testing.T) {
	prompt := mcp.NewPrompt("test-prompt", mcp.WithPromptDescription("Test prompt"))
	handler := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	})

	serverPrompt := NewServerPrompt(prompt, handler)

	assert.Equal(t, prompt, serverPrompt.Prompt)
	assert.NotNil(t, serverPrompt.Handler)

	// Test that handler works
	result, err := serverPrompt.Handler(context.Background(), mcp.GetPromptRequest{})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestToolset_GetActiveTools(t *testing.T) {
	tests := []struct {
		name          string
		enabled       bool
		readOnly      bool
		readTools     int
		writeTools    int
		expectedCount int
	}{
		{
			name:          "disabled toolset returns nil",
			enabled:       false,
			readOnly:      false,
			readTools:     2,
			writeTools:    2,
			expectedCount: 0,
		},
		{
			name:          "enabled read/write toolset returns all tools",
			enabled:       true,
			readOnly:      false,
			readTools:     2,
			writeTools:    3,
			expectedCount: 5,
		},
		{
			name:          "enabled read-only toolset returns only read tools",
			enabled:       true,
			readOnly:      true,
			readTools:     3,
			writeTools:    2,
			expectedCount: 3,
		},
		{
			name:          "enabled toolset with no write tools",
			enabled:       true,
			readOnly:      false,
			readTools:     2,
			writeTools:    0,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test", "Test toolset")
			toolset.Enabled = tt.enabled
			if tt.readOnly {
				toolset.SetReadOnly()
			}

			// Add read tools
			for i := 0; i < tt.readTools; i++ {
				readTool := createTestToolWithAnnotation("read-tool-"+string(rune('0'+i)), true)
				toolset.AddReadTools(readTool)
			}

			// Add write tools (will be ignored if read-only)
			for i := 0; i < tt.writeTools; i++ {
				writeTool := createTestToolWithAnnotation("write-tool-"+string(rune('0'+i)), false)
				toolset.AddWriteTools(writeTool)
			}

			activeTools := toolset.GetActiveTools()

			if tt.expectedCount == 0 {
				assert.Nil(t, activeTools)
			} else {
				assert.Len(t, activeTools, tt.expectedCount)
			}
		})
	}
}

func TestToolset_GetAvailableTools(t *testing.T) {
	tests := []struct {
		name          string
		readOnly      bool
		readTools     int
		writeTools    int
		expectedCount int
	}{
		{
			name:          "read/write toolset returns all tools",
			readOnly:      false,
			readTools:     3,
			writeTools:    2,
			expectedCount: 5,
		},
		{
			name:          "read-only toolset returns only read tools",
			readOnly:      true,
			readTools:     4,
			writeTools:    3,
			expectedCount: 4,
		},
		{
			name:          "no tools returns empty",
			readOnly:      false,
			readTools:     0,
			writeTools:    0,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test", "Test toolset")
			if tt.readOnly {
				toolset.SetReadOnly()
			}

			// Add read tools
			for i := 0; i < tt.readTools; i++ {
				readTool := createTestToolWithAnnotation("read-"+string(rune('0'+i)), true)
				toolset.AddReadTools(readTool)
			}

			// Add write tools
			for i := 0; i < tt.writeTools; i++ {
				writeTool := createTestToolWithAnnotation("write-"+string(rune('0'+i)), false)
				toolset.AddWriteTools(writeTool)
			}

			availableTools := toolset.GetAvailableTools()
			assert.Len(t, availableTools, tt.expectedCount)
		})
	}
}

func TestToolset_RegisterTools(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0.0")

	tests := []struct {
		name       string
		enabled    bool
		readOnly   bool
		readTools  int
		writeTools int
	}{
		{
			name:       "register when enabled",
			enabled:    true,
			readOnly:   false,
			readTools:  2,
			writeTools: 2,
		},
		{
			name:       "don't register when disabled",
			enabled:    false,
			readOnly:   false,
			readTools:  2,
			writeTools: 2,
		},
		{
			name:       "register read-only tools only",
			enabled:    true,
			readOnly:   true,
			readTools:  3,
			writeTools: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test-"+tt.name, "Test toolset")
			toolset.Enabled = tt.enabled
			if tt.readOnly {
				toolset.SetReadOnly()
			}

			// Add tools
			for i := 0; i < tt.readTools; i++ {
				readTool := createTestToolWithAnnotation("read-"+string(rune('0'+i)), true)
				toolset.AddReadTools(readTool)
			}
			for i := 0; i < tt.writeTools; i++ {
				writeTool := createTestToolWithAnnotation("write-"+string(rune('0'+i)), false)
				toolset.AddWriteTools(writeTool)
			}

			// Should not panic
			assert.NotPanics(t, func() {
				toolset.RegisterTools(mcpServer)
			})
		})
	}
}

func TestToolset_AddResourceTemplates(t *testing.T) {
	toolset := NewToolset("test", "Test toolset")

	template1 := mcp.NewResourceTemplate("test://resource1", "Resource 1")
	handler1 := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})

	template2 := mcp.NewResourceTemplate("test://resource2", "Resource 2")
	handler2 := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})

	// Add single template
	result := toolset.AddResourceTemplates(NewServerResourceTemplate(template1, handler1))
	assert.Equal(t, toolset, result, "Should return self for chaining")
	assert.Len(t, toolset.resourceTemplates, 1)

	// Add another template
	toolset.AddResourceTemplates(NewServerResourceTemplate(template2, handler2))
	assert.Len(t, toolset.resourceTemplates, 2)
}

func TestToolset_AddPrompts(t *testing.T) {
	toolset := NewToolset("test", "Test toolset")

	prompt1 := mcp.NewPrompt("prompt1", mcp.WithPromptDescription("Prompt 1"))
	handler1 := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	})

	prompt2 := mcp.NewPrompt("prompt2", mcp.WithPromptDescription("Prompt 2"))
	handler2 := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	})

	// Add single prompt
	result := toolset.AddPrompts(NewServerPrompt(prompt1, handler1))
	assert.Equal(t, toolset, result, "Should return self for chaining")
	assert.Len(t, toolset.prompts, 1)

	// Add another prompt
	toolset.AddPrompts(NewServerPrompt(prompt2, handler2))
	assert.Len(t, toolset.prompts, 2)
}

func TestToolset_GetActiveResourceTemplates(t *testing.T) {
	template := mcp.NewResourceTemplate("test://resource", "Test")
	handler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})

	tests := []struct {
		name     string
		enabled  bool
		expected int
	}{
		{
			name:     "disabled toolset returns nil",
			enabled:  false,
			expected: 0,
		},
		{
			name:     "enabled toolset returns templates",
			enabled:  true,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test", "Test")
			toolset.Enabled = tt.enabled
			toolset.AddResourceTemplates(NewServerResourceTemplate(template, handler))

			activeTemplates := toolset.GetActiveResourceTemplates()

			if tt.expected == 0 {
				assert.Nil(t, activeTemplates)
			} else {
				assert.Len(t, activeTemplates, tt.expected)
			}
		})
	}
}

func TestToolset_GetAvailableResourceTemplates(t *testing.T) {
	toolset := NewToolset("test", "Test")

	// Initially empty
	assert.Len(t, toolset.GetAvailableResourceTemplates(), 0)

	// Add templates
	for i := 0; i < 3; i++ {
		template := mcp.NewResourceTemplate("test://resource"+string(rune('0'+i)), "Test")
		handler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{}, nil
		})
		toolset.AddResourceTemplates(NewServerResourceTemplate(template, handler))
	}

	assert.Len(t, toolset.GetAvailableResourceTemplates(), 3)
}

func TestToolset_RegisterResourcesTemplates(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0.0")

	tests := []struct {
		name      string
		enabled   bool
		templates int
	}{
		{
			name:      "register when enabled",
			enabled:   true,
			templates: 2,
		},
		{
			name:      "don't register when disabled",
			enabled:   false,
			templates: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test-"+tt.name, "Test")
			toolset.Enabled = tt.enabled

			// Add templates
			for i := 0; i < tt.templates; i++ {
				template := mcp.NewResourceTemplate("test://res"+string(rune('0'+i)), "Test")
				handler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					return []mcp.ResourceContents{}, nil
				})
				toolset.AddResourceTemplates(NewServerResourceTemplate(template, handler))
			}

			// Should not panic
			assert.NotPanics(t, func() {
				toolset.RegisterResourcesTemplates(mcpServer)
			})
		})
	}
}

func TestToolset_RegisterPrompts(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0.0")

	tests := []struct {
		name    string
		enabled bool
		prompts int
	}{
		{
			name:    "register when enabled",
			enabled: true,
			prompts: 2,
		},
		{
			name:    "don't register when disabled",
			enabled: false,
			prompts: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolset := NewToolset("test-"+tt.name, "Test")
			toolset.Enabled = tt.enabled

			// Add prompts
			for i := 0; i < tt.prompts; i++ {
				prompt := mcp.NewPrompt("prompt"+string(rune('0'+i)), mcp.WithPromptDescription("Test"))
				handler := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
					return &mcp.GetPromptResult{}, nil
				})
				toolset.AddPrompts(NewServerPrompt(prompt, handler))
			}

			// Should not panic
			assert.NotPanics(t, func() {
				toolset.RegisterPrompts(mcpServer)
			})
		})
	}
}

func TestToolset_SetReadOnly(t *testing.T) {
	toolset := NewToolset("test", "Test")
	assert.False(t, toolset.readOnly)

	toolset.SetReadOnly()
	assert.True(t, toolset.readOnly)

	// Call again should be idempotent
	toolset.SetReadOnly()
	assert.True(t, toolset.readOnly)
}

func TestToolset_AddWriteTools(t *testing.T) {
	t.Run("add write tools to normal toolset", func(t *testing.T) {
		toolset := NewToolset("test", "Test")

		writeTool1 := createTestToolWithAnnotation("write1", false)
		writeTool2 := createTestToolWithAnnotation("write2", false)

		toolset.AddWriteTools(writeTool1, writeTool2)
		assert.Len(t, toolset.writeTools, 2)
	})

	t.Run("write tools ignored in read-only toolset", func(t *testing.T) {
		toolset := NewToolset("test", "Test")
		toolset.SetReadOnly()

		writeTool := createTestToolWithAnnotation("write", false)
		toolset.AddWriteTools(writeTool)

		assert.Len(t, toolset.writeTools, 0)
	})

	t.Run("panic when adding read-only tool to write tools", func(t *testing.T) {
		toolset := NewToolset("test", "Test")

		readTool := createTestToolWithAnnotation("read", true)

		assert.Panics(t, func() {
			toolset.AddWriteTools(readTool)
		})
	})
}

func TestToolset_AddReadTools(t *testing.T) {
	t.Run("add read tools successfully", func(t *testing.T) {
		toolset := NewToolset("test", "Test")

		readTool1 := createTestToolWithAnnotation("read1", true)
		readTool2 := createTestToolWithAnnotation("read2", true)

		toolset.AddReadTools(readTool1, readTool2)
		assert.Len(t, toolset.readTools, 2)
	})

	t.Run("panic when adding write tool to read tools", func(t *testing.T) {
		toolset := NewToolset("test", "Test")

		writeTool := createTestToolWithAnnotation("write", false)

		assert.Panics(t, func() {
			toolset.AddReadTools(writeTool)
		})
	})
}

func TestToolsetGroup_RegisterAll(t *testing.T) {
	mcpServer := server.NewMCPServer("test", "1.0.0")
	tsg := NewToolsetGroup(false)

	// Add multiple toolsets
	toolset1 := NewToolset("toolset1", "Toolset 1")
	toolset1.Enabled = true
	readTool1 := createTestToolWithAnnotation("read1", true)
	toolset1.AddReadTools(readTool1)

	template1 := mcp.NewResourceTemplate("test://res1", "Resource 1")
	templateHandler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})
	toolset1.AddResourceTemplates(NewServerResourceTemplate(template1, templateHandler))

	prompt1 := mcp.NewPrompt("prompt1", mcp.WithPromptDescription("Prompt 1"))
	promptHandler := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	})
	toolset1.AddPrompts(NewServerPrompt(prompt1, promptHandler))

	toolset2 := NewToolset("toolset2", "Toolset 2")
	toolset2.Enabled = false
	readTool2 := createTestToolWithAnnotation("read2", true)
	toolset2.AddReadTools(readTool2)

	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	// Should not panic
	assert.NotPanics(t, func() {
		tsg.RegisterAll(mcpServer)
	})
}

func TestToolsetGroup_AddToolset_ReadOnlyMode(t *testing.T) {
	tsg := NewToolsetGroup(true) // Read-only mode

	toolset := NewToolset("test", "Test")
	assert.False(t, toolset.readOnly)

	tsg.AddToolset(toolset)

	// Toolset should now be read-only
	assert.True(t, toolset.readOnly)
}

func TestToolset_CompleteWorkflow(t *testing.T) {
	// Test a complete workflow with all components
	toolset := NewToolset("complete", "Complete toolset")

	// Add read tools
	readTool1 := createTestToolWithAnnotation("list", true)
	readTool2 := createTestToolWithAnnotation("get", true)
	toolset.AddReadTools(readTool1, readTool2)

	// Add write tools
	writeTool1 := createTestToolWithAnnotation("create", false)
	writeTool2 := createTestToolWithAnnotation("update", false)
	toolset.AddWriteTools(writeTool1, writeTool2)

	// Add resource templates
	template := mcp.NewResourceTemplate("test://resource", "Test Resource")
	templateHandler := server.ResourceTemplateHandlerFunc(func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{}, nil
	})
	toolset.AddResourceTemplates(NewServerResourceTemplate(template, templateHandler))

	// Add prompts
	prompt := mcp.NewPrompt("test-prompt", mcp.WithPromptDescription("Test Prompt"))
	promptHandler := server.PromptHandlerFunc(func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{}, nil
	})
	toolset.AddPrompts(NewServerPrompt(prompt, promptHandler))

	// Verify counts
	assert.Len(t, toolset.GetAvailableTools(), 4)
	assert.Len(t, toolset.GetAvailableResourceTemplates(), 1)

	// Enable and check active tools
	toolset.Enabled = true
	assert.Len(t, toolset.GetActiveTools(), 4)
	assert.Len(t, toolset.GetActiveResourceTemplates(), 1)

	// Register everything
	mcpServer := server.NewMCPServer("test", "1.0.0")
	assert.NotPanics(t, func() {
		toolset.RegisterTools(mcpServer)
		toolset.RegisterResourcesTemplates(mcpServer)
		toolset.RegisterPrompts(mcpServer)
	})
}

func TestToolsetGroup_AddToolset_NonReadOnlyMode(t *testing.T) {
	tsg := NewToolsetGroup(false) // Non-read-only mode

	toolset := NewToolset("test", "Test")
	assert.False(t, toolset.readOnly)

	tsg.AddToolset(toolset)

	// Toolset should remain non-read-only
	assert.False(t, toolset.readOnly)
}
