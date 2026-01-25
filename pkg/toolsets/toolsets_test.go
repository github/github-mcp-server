package toolsets

import (
	"errors"
	"testing"
)

func TestNewToolsetGroupIsEmptyWithoutEverythingOn(t *testing.T) {
	tsg := NewToolsetGroup(false)
	if len(tsg.Toolsets) != 0 {
		t.Fatalf("Expected Toolsets map to be empty, got %d items", len(tsg.Toolsets))
	}
	if tsg.everythingOn {
		t.Fatal("Expected everythingOn to be initialized as false")
	}
}

func TestAddToolset(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test adding a toolset
	toolset := NewToolset("test-toolset", "A test toolset")
	toolset.Enabled = true
	tsg.AddToolset(toolset)

	// Verify toolset was added correctly
	if len(tsg.Toolsets) != 1 {
		t.Errorf("Expected 1 toolset, got %d", len(tsg.Toolsets))
	}

	toolset, exists := tsg.Toolsets["test-toolset"]
	if !exists {
		t.Fatal("Feature was not added to the map")
	}

	if toolset.Name != "test-toolset" {
		t.Errorf("Expected toolset name to be 'test-toolset', got '%s'", toolset.Name)
	}

	if toolset.Description != "A test toolset" {
		t.Errorf("Expected toolset description to be 'A test toolset', got '%s'", toolset.Description)
	}

	if !toolset.Enabled {
		t.Error("Expected toolset to be enabled")
	}

	// Test adding another toolset
	anotherToolset := NewToolset("another-toolset", "Another test toolset")
	tsg.AddToolset(anotherToolset)

	if len(tsg.Toolsets) != 2 {
		t.Errorf("Expected 2 toolsets, got %d", len(tsg.Toolsets))
	}

	// Test overriding existing toolset
	updatedToolset := NewToolset("test-toolset", "Updated description")
	tsg.AddToolset(updatedToolset)

	toolset = tsg.Toolsets["test-toolset"]
	if toolset.Description != "Updated description" {
		t.Errorf("Expected toolset description to be updated to 'Updated description', got '%s'", toolset.Description)
	}

	if toolset.Enabled {
		t.Error("Expected toolset to be disabled after update")
	}
}

func TestIsEnabled(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test with non-existent toolset
	if tsg.IsEnabled("non-existent") {
		t.Error("Expected IsEnabled to return false for non-existent toolset")
	}

	// Test with disabled toolset
	disabledToolset := NewToolset("disabled-toolset", "A disabled toolset")
	tsg.AddToolset(disabledToolset)
	if tsg.IsEnabled("disabled-toolset") {
		t.Error("Expected IsEnabled to return false for disabled toolset")
	}

	// Test with enabled toolset
	enabledToolset := NewToolset("enabled-toolset", "An enabled toolset")
	enabledToolset.Enabled = true
	tsg.AddToolset(enabledToolset)
	if !tsg.IsEnabled("enabled-toolset") {
		t.Error("Expected IsEnabled to return true for enabled toolset")
	}
}

func TestEnableFeature(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Test enabling non-existent toolset
	err := tsg.EnableToolset("non-existent")
	if err == nil {
		t.Error("Expected error when enabling non-existent toolset")
	}

	// Test enabling toolset
	testToolset := NewToolset("test-toolset", "A test toolset")
	tsg.AddToolset(testToolset)

	if tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be disabled initially")
	}

	err = tsg.EnableToolset("test-toolset")
	if err != nil {
		t.Errorf("Expected no error when enabling toolset, got: %v", err)
	}

	if !tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be enabled after EnableFeature call")
	}

	// Test enabling already enabled toolset
	err = tsg.EnableToolset("test-toolset")
	if err != nil {
		t.Errorf("Expected no error when enabling already enabled toolset, got: %v", err)
	}
}

func TestEnableToolsets(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Prepare toolsets
	toolset1 := NewToolset("toolset1", "Feature 1")
	toolset2 := NewToolset("toolset2", "Feature 2")
	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	// Test enabling multiple toolsets
	err := tsg.EnableToolsets([]string{"toolset1", "toolset2"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling toolsets, got: %v", err)
	}

	if !tsg.IsEnabled("toolset1") {
		t.Error("Expected toolset1 to be enabled")
	}

	if !tsg.IsEnabled("toolset2") {
		t.Error("Expected toolset2 to be enabled")
	}

	// Test with non-existent toolset in the list
	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, nil)
	if err != nil {
		t.Errorf("Expected no error when ignoring unknown toolsets, got: %v", err)
	}

	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, &EnableToolsetsOptions{
		ErrorOnUnknown: false,
	})
	if err != nil {
		t.Errorf("Expected no error when ignoring unknown toolsets, got: %v", err)
	}

	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"}, &EnableToolsetsOptions{ErrorOnUnknown: true})
	if err == nil {
		t.Error("Expected error when enabling list with non-existent toolset")
	}
	if !errors.Is(err, NewToolsetDoesNotExistError("non-existent")) {
		t.Errorf("Expected ToolsetDoesNotExistError when enabling non-existent toolset, got: %v", err)
	}

	// Test with empty list
	err = tsg.EnableToolsets([]string{}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error with empty toolset list, got: %v", err)
	}

	// Test enabling everything through EnableToolsets
	tsg = NewToolsetGroup(false)
	err = tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true after enabling 'all' via EnableToolsets")
	}
}

func TestEnableEverything(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Add a disabled toolset
	testToolset := NewToolset("test-toolset", "A test toolset")
	tsg.AddToolset(testToolset)

	// Verify it's disabled
	if tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be disabled initially")
	}

	// Enable "all"
	err := tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	// Verify everythingOn was set
	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true after enabling 'all'")
	}

	// Verify the previously disabled toolset is now enabled
	if !tsg.IsEnabled("test-toolset") {
		t.Error("Expected toolset to be enabled when everythingOn is true")
	}

	// Verify a non-existent toolset is also enabled
	if !tsg.IsEnabled("non-existent") {
		t.Error("Expected non-existent toolset to be enabled when everythingOn is true")
	}
}

func TestIsEnabledWithEverythingOn(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Enable "all"
	err := tsg.EnableToolsets([]string{"all"}, &EnableToolsetsOptions{})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all', got: %v", err)
	}

	// Test that any toolset name returns true with IsEnabled
	if !tsg.IsEnabled("some-toolset") {
		t.Error("Expected IsEnabled to return true for any toolset when everythingOn is true")
	}

	if !tsg.IsEnabled("another-toolset") {
		t.Error("Expected IsEnabled to return true for any toolset when everythingOn is true")
	}
}

func TestToolsetGroup_GetToolset(t *testing.T) {
	tsg := NewToolsetGroup(false)
	toolset := NewToolset("my-toolset", "desc")
	tsg.AddToolset(toolset)

	// Should find the toolset
	got, err := tsg.GetToolset("my-toolset")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != toolset {
		t.Errorf("expected to get the same toolset instance")
	}

	// Should not find a non-existent toolset
	_, err = tsg.GetToolset("does-not-exist")
	if err == nil {
		t.Error("expected error for missing toolset, got nil")
	}
	if !errors.Is(err, NewToolsetDoesNotExistError("does-not-exist")) {
		t.Errorf("expected error to be ToolsetDoesNotExistError, got %v", err)
	}
}
