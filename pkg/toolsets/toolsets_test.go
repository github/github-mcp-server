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
	err := tsg.EnableToolsets([]string{"toolset1", "toolset2"})
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
	err = tsg.EnableToolsets([]string{"toolset1", "non-existent"})
	if err == nil {
		t.Error("Expected error when enabling list with non-existent toolset")
	}
	if !errors.Is(err, NewToolsetDoesNotExistError("non-existent")) {
		t.Errorf("Expected ToolsetDoesNotExistError when enabling non-existent toolset, got: %v", err)
	}

	// Test with empty list
	err = tsg.EnableToolsets([]string{})
	if err != nil {
		t.Errorf("Expected no error with empty toolset list, got: %v", err)
	}

	// Test enabling everything through EnableToolsets
	tsg = NewToolsetGroup(false)
	err = tsg.EnableToolsets([]string{"all"})
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
	err := tsg.EnableToolsets([]string{"all"})
	if err != nil {
		t.Errorf("Expected no error when enabling 'eall', got: %v", err)
	}

	// Verify everythingOn was set
	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true after enabling 'eall'")
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
	err := tsg.EnableToolsets([]string{"all"})
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

func TestEnableToolsets_AllWithOtherNames(t *testing.T) {
	tsg := NewToolsetGroup(false)

	// Add toolsets
	toolset1 := NewToolset("toolset1", "Feature 1")
	toolset2 := NewToolset("toolset2", "Feature 2")
	tsg.AddToolset(toolset1)
	tsg.AddToolset(toolset2)

	// Test enabling "all" along with specific names - "all" should take precedence
	err := tsg.EnableToolsets([]string{"toolset1", "all", "toolset2"})
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true")
	}

	// Both toolsets should be enabled
	if !tsg.IsEnabled("toolset1") {
		t.Error("Expected toolset1 to be enabled")
	}

	if !tsg.IsEnabled("toolset2") {
		t.Error("Expected toolset2 to be enabled")
	}

	// Even non-existent toolsets should return true when everythingOn is true
	if !tsg.IsEnabled("non-existent") {
		t.Error("Expected non-existent toolset to be enabled when everythingOn is true")
	}
}

func TestEnableToolsets_AllWithEmptyToolsets(t *testing.T) {
	// Test enabling "all" when there are no toolsets in the group
	tsg := NewToolsetGroup(false)

	err := tsg.EnableToolsets([]string{"all"})
	if err != nil {
		t.Errorf("Expected no error when enabling 'all' with empty toolsets, got: %v", err)
	}

	if !tsg.everythingOn {
		t.Error("Expected everythingOn to be true")
	}

	// IsEnabled should still return true for any toolset name
	if !tsg.IsEnabled("any-toolset") {
		t.Error("Expected IsEnabled to return true when everythingOn is true, even with empty toolsets")
	}
}

func TestEnableToolsets_ExhaustiveCoverage(t *testing.T) {
	// Test various combinations to ensure full coverage
	
	// Test 1: "all" at the beginning
	tsg1 := NewToolsetGroup(false)
	toolset1 := NewToolset("t1", "T1")
	tsg1.AddToolset(toolset1)
	err := tsg1.EnableToolsets([]string{"all", "t1"})
	if err != nil {
		t.Errorf("Test 1 failed: %v", err)
	}
	
	// Test 2: "all" in the middle
	tsg2 := NewToolsetGroup(false)
	toolset2 := NewToolset("t2", "T2")
	toolset3 := NewToolset("t3", "T3")
	tsg2.AddToolset(toolset2)
	tsg2.AddToolset(toolset3)
	err = tsg2.EnableToolsets([]string{"t2", "all", "t3"})
	if err != nil {
		t.Errorf("Test 2 failed: %v", err)
	}
	
	// Test 3: "all" at the end
	tsg3 := NewToolsetGroup(false)
	toolset4 := NewToolset("t4", "T4")
	tsg3.AddToolset(toolset4)
	err = tsg3.EnableToolsets([]string{"t4", "all"})
	if err != nil {
		t.Errorf("Test 3 failed: %v", err)
	}
	
	// Test 4: Only "all"
	tsg4 := NewToolsetGroup(false)
	toolset5 := NewToolset("t5", "T5")
	toolset6 := NewToolset("t6", "T6")
	tsg4.AddToolset(toolset5)
	tsg4.AddToolset(toolset6)
	err = tsg4.EnableToolsets([]string{"all"})
	if err != nil {
		t.Errorf("Test 4 failed: %v", err)
	}
	// Verify both are enabled
	if !tsg4.IsEnabled("t5") || !tsg4.IsEnabled("t6") {
		t.Error("Test 4: Expected all toolsets to be enabled")
	}
}
