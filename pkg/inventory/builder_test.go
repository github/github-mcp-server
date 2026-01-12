package inventory

import (
	"testing"
)

// mockToolWithScopes creates a mock tool with specified scope definitions
func mockToolWithScopes(name string, toolsetID string, requiredScopes, acceptedScopes []string) ServerTool {
	tool := mockTool(name, toolsetID, false)
	tool.RequiredScopes = requiredScopes
	tool.AcceptedScopes = acceptedScopes
	return tool
}

func TestWithRequireScopes_AllToolsHaveScopes(t *testing.T) {
	tools := []ServerTool{
		mockToolWithScopes("tool1", "toolset1", []string{"repo"}, []string{"repo", "admin:org"}),
		mockToolWithScopes("tool2", "toolset1", []string{"repo", "user"}, []string{"repo", "user", "admin:org"}),
		mockToolWithScopes("tool3", "toolset2", []string{}, []string{}), // empty slices are valid
	}

	// Should not panic when all tools have scope definitions
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_ToolMissingBothScopes(t *testing.T) {
	tools := []ServerTool{
		mockToolWithScopes("tool1", "toolset1", []string{"repo"}, []string{"repo"}),
		mockTool("tool2", "toolset1", false), // This tool has nil RequiredScopes and AcceptedScopes
	}

	// Should panic when a tool is missing both scope definitions
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Build() should have panicked for tool missing scope definitions")
		}
		errMsg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected panic message to be a string, got %T", r)
		}
		expectedMsg := `tool "tool2" missing scope definitions (both RequiredScopes and AcceptedScopes are nil)`
		if errMsg != expectedMsg {
			t.Errorf("Expected panic message %q, got %q", expectedMsg, errMsg)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_OnlyRequiredScopesSet(t *testing.T) {
	tool := mockTool("tool1", "toolset1", false)
	tool.RequiredScopes = []string{"repo"}
	tool.AcceptedScopes = nil

	tools := []ServerTool{tool}

	// Should not panic when only RequiredScopes is set (AcceptedScopes can be nil if RequiredScopes is set)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_OnlyAcceptedScopesSet(t *testing.T) {
	tool := mockTool("tool1", "toolset1", false)
	tool.RequiredScopes = nil
	tool.AcceptedScopes = []string{"repo"}

	tools := []ServerTool{tool}

	// Should not panic when only AcceptedScopes is set (RequiredScopes can be nil if AcceptedScopes is set)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_EmptySlicesAllowed(t *testing.T) {
	tools := []ServerTool{
		mockToolWithScopes("tool1", "toolset1", []string{}, []string{}),
		mockToolWithScopes("tool2", "toolset2", []string{}, []string{}),
	}

	// Should not panic when tools have empty slices (explicit "no scopes needed")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_False(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", false), // Missing scope definitions
		mockTool("tool2", "toolset1", false), // Missing scope definitions
	}

	// Should not panic when WithRequireScopes(false) or not set
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(false).
		Build()
}

func TestWithRequireScopes_NotSet(t *testing.T) {
	tools := []ServerTool{
		mockTool("tool1", "toolset1", false), // Missing scope definitions
		mockTool("tool2", "toolset1", false), // Missing scope definitions
	}

	// Should not panic when WithRequireScopes is not called (default is false)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		Build()
}

func TestWithRequireScopes_MixedTools(t *testing.T) {
	tools := []ServerTool{
		mockToolWithScopes("tool1", "toolset1", []string{"repo"}, []string{"repo"}),
		mockToolWithScopes("tool2", "toolset1", []string{}, []string{}),
		mockTool("tool3", "toolset2", false), // Missing scope definitions
	}

	// Should panic on the first tool with missing scope definitions
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Build() should have panicked for tool missing scope definitions")
		}
		errMsg, ok := r.(string)
		if !ok {
			t.Fatalf("Expected panic message to be a string, got %T", r)
		}
		expectedMsg := `tool "tool3" missing scope definitions (both RequiredScopes and AcceptedScopes are nil)`
		if errMsg != expectedMsg {
			t.Errorf("Expected panic message %q, got %q", expectedMsg, errMsg)
		}
	}()

	_ = NewBuilder().
		SetTools(tools).
		WithRequireScopes(true).
		Build()
}

func TestWithRequireScopes_Chaining(t *testing.T) {
	tools := []ServerTool{
		mockToolWithScopes("tool1", "toolset1", []string{"repo"}, []string{"repo"}),
	}

	// Test that WithRequireScopes returns the builder for chaining
	builder := NewBuilder()
	result := builder.WithRequireScopes(true)

	if result != builder {
		t.Error("WithRequireScopes should return the same builder instance for chaining")
	}

	// Verify the build works
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Build() panicked unexpectedly: %v", r)
		}
	}()

	_ = result.SetTools(tools).Build()
}
