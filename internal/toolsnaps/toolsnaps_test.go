package toolsnaps

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyTool struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// withIsolatedWorkingDir creates a temp dir, changes to it, and restores the original working dir after the test.
func withIsolatedWorkingDir(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, os.Chdir(origDir)) })
	require.NoError(t, os.Chdir(dir))
}

func TestSnapshotDoesNotExistNotInCI(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given we are not running in CI
	t.Setenv("GITHUB_ACTIONS", "false") // This REALLY is required because the tests run in CI
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed and write the snapshot file
	require.NoError(t, err)
	path := filepath.Join("__toolsnaps__", "dummy.snap")
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "expected snapshot file to be written")
}

func TestSnapshotDoesNotExistInCI(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given we are running in CI
	t.Setenv("GITHUB_ACTIONS", "true")
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about missing snapshot in CI
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool snapshot does not exist", "expected error about missing snapshot in CI")
}

func TestSnapshotExistsMatch(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a matching snapshot file exists
	tool := dummyTool{"foo", 42}
	b, _ := json.MarshalIndent(tool, "", "  ")
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), b, 0600))

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed (no error)
	require.NoError(t, err)
}

func TestSnapshotExistsDiff(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given a non-matching snapshot file exists
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`{"name":"foo","value":1}`), 0600))
	tool := dummyTool{"foo", 2}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about the schema diff
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool schema for dummy has changed unexpectedly", "expected error about diff")
}

func TestUpdateToolsnaps(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given UPDATE_TOOLSNAPS is set, regardless of whether a matching snapshot file exists
	t.Setenv("UPDATE_TOOLSNAPS", "true")
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`{"name":"foo","value":1}`), 0600))
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should succeed and write the snapshot file
	require.NoError(t, err)
	path := filepath.Join("__toolsnaps__", "dummy.snap")
	_, statErr := os.Stat(path)
	assert.NoError(t, statErr, "expected snapshot file to be written")
}

func TestMalformedSnapshotJSON(t *testing.T) {
	withIsolatedWorkingDir(t)
	// Ensure that UPDATE_TOOLSNAPS is not set for this test, which it might be if someone is running
	// UPDATE_TOOLSNAPS=true go test ./...
	t.Setenv("UPDATE_TOOLSNAPS", "false")

	// Given a malformed snapshot file exists
	require.NoError(t, os.MkdirAll("__toolsnaps__", 0700))
	require.NoError(t, os.WriteFile(filepath.Join("__toolsnaps__", "dummy.snap"), []byte(`not-json`), 0600))
	tool := dummyTool{"foo", 42}

	// When we test the snapshot
	err := Test("dummy", tool)

	// Then it should error about malformed snapshot JSON
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse snapshot JSON for dummy", "expected error about malformed snapshot JSON")
}

func TestSortJSONKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple object",
			input:    `{"z": 1, "a": 2, "m": 3}`,
			expected: "{\n  \"a\": 2,\n  \"m\": 3,\n  \"z\": 1\n}",
		},
		{
			name:     "nested object",
			input:    `{"z": {"y": 1, "x": 2}, "a": 3}`,
			expected: "{\n  \"a\": 3,\n  \"z\": {\n    \"x\": 2,\n    \"y\": 1\n  }\n}",
		},
		{
			name:     "array with objects",
			input:    `{"items": [{"z": 1, "a": 2}, {"y": 3, "b": 4}]}`,
			expected: "{\n  \"items\": [\n    {\n      \"a\": 2,\n      \"z\": 1\n    },\n    {\n      \"b\": 4,\n      \"y\": 3\n    }\n  ]\n}",
		},
		{
			name:     "deeply nested",
			input:    `{"z": {"y": {"x": 1, "a": 2}, "b": 3}, "m": 4}`,
			expected: "{\n  \"m\": 4,\n  \"z\": {\n    \"b\": 3,\n    \"y\": {\n      \"a\": 2,\n      \"x\": 1\n    }\n  }\n}",
		},
		{
			name:     "properties field like in toolsnaps",
			input:    `{"name": "test", "properties": {"repo": {"type": "string"}, "owner": {"type": "string"}, "page": {"type": "number"}}}`,
			expected: "{\n  \"name\": \"test\",\n  \"properties\": {\n    \"owner\": {\n      \"type\": \"string\"\n    },\n    \"page\": {\n      \"type\": \"number\"\n    },\n    \"repo\": {\n      \"type\": \"string\"\n    }\n  }\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sortJSONKeys([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestSortJSONKeysIdempotent(t *testing.T) {
	// Given a JSON string that's already sorted
	input := `{"a": 1, "b": {"x": 2, "y": 3}, "c": [{"m": 4, "n": 5}]}`

	// When we sort it once
	sorted1, err := sortJSONKeys([]byte(input))
	require.NoError(t, err)

	// And sort it again
	sorted2, err := sortJSONKeys(sorted1)
	require.NoError(t, err)

	// Then the results should be identical
	assert.Equal(t, string(sorted1), string(sorted2))
}

func TestToolSnapKeysSorted(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a tool with fields that could be in any order
	type complexTool struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Properties  map[string]any `json:"properties"`
		Annotations map[string]any `json:"annotations"`
	}

	tool := complexTool{
		Name:        "test_tool",
		Description: "A test tool",
		Properties: map[string]any{
			"zzz":   "last",
			"aaa":   "first",
			"mmm":   "middle",
			"owner": map[string]any{"type": "string", "description": "Owner"},
			"repo":  map[string]any{"type": "string", "description": "Repo"},
		},
		Annotations: map[string]any{
			"readOnly": true,
			"title":    "Test",
		},
	}

	// When we write the snapshot
	t.Setenv("UPDATE_TOOLSNAPS", "true")
	err := Test("complex", tool)
	require.NoError(t, err)

	// Then the snapshot file should have sorted keys
	snapJSON, err := os.ReadFile("__toolsnaps__/complex.snap")
	require.NoError(t, err)

	// Verify that the JSON is properly sorted by checking key order
	var parsed map[string]any
	err = json.Unmarshal(snapJSON, &parsed)
	require.NoError(t, err)

	// Check that properties are sorted
	propsJSON, _ := json.MarshalIndent(parsed["properties"], "", "  ")
	propsStr := string(propsJSON)
	// The properties should have "aaa" before "mmm" before "zzz"
	aaaIndex := -1
	mmmIndex := -1
	zzzIndex := -1
	for i, line := range propsStr {
		if line == 'a' && i+2 < len(propsStr) && propsStr[i:i+3] == "aaa" {
			aaaIndex = i
		}
		if line == 'm' && i+2 < len(propsStr) && propsStr[i:i+3] == "mmm" {
			mmmIndex = i
		}
		if line == 'z' && i+2 < len(propsStr) && propsStr[i:i+3] == "zzz" {
			zzzIndex = i
		}
	}
	assert.Greater(t, mmmIndex, aaaIndex, "mmm should come after aaa")
	assert.Greater(t, zzzIndex, mmmIndex, "zzz should come after mmm")
}

func TestStructFieldOrderingSortedAlphabetically(t *testing.T) {
	withIsolatedWorkingDir(t)

	// Given a struct with fields defined in non-alphabetical order
	// This test ensures that struct field order doesn't affect the JSON output
	type toolWithNonAlphabeticalFields struct {
		ZField string `json:"zField"` // Should appear last in JSON
		AField string `json:"aField"` // Should appear first in JSON
		MField string `json:"mField"` // Should appear in the middle
	}

	tool := toolWithNonAlphabeticalFields{
		ZField: "z value",
		AField: "a value",
		MField: "m value",
	}

	// When we write the snapshot
	t.Setenv("UPDATE_TOOLSNAPS", "true")
	err := Test("struct_field_order", tool)
	require.NoError(t, err)

	// Then the snapshot file should have alphabetically sorted keys despite struct field order
	snapJSON, err := os.ReadFile("__toolsnaps__/struct_field_order.snap")
	require.NoError(t, err)

	snapStr := string(snapJSON)

	// Find the positions of each field in the JSON string
	aFieldIndex := -1
	mFieldIndex := -1
	zFieldIndex := -1
	for i := range len(snapStr) - 7 {
		switch snapStr[i : i+6] {
		case "aField":
			aFieldIndex = i
		case "mField":
			mFieldIndex = i
		case "zField":
			zFieldIndex = i
		}
	}

	// Verify alphabetical ordering in the JSON output
	require.NotEqual(t, -1, aFieldIndex, "aField should be present")
	require.NotEqual(t, -1, mFieldIndex, "mField should be present")
	require.NotEqual(t, -1, zFieldIndex, "zField should be present")
	assert.Less(t, aFieldIndex, mFieldIndex, "aField should appear before mField")
	assert.Less(t, mFieldIndex, zFieldIndex, "mField should appear before zField")

	// Also verify idempotency - running the test again should produce identical output
	err = Test("struct_field_order", tool)
	require.NoError(t, err)

	snapJSON2, err := os.ReadFile("__toolsnaps__/struct_field_order.snap")
	require.NoError(t, err)

	assert.Equal(t, string(snapJSON), string(snapJSON2), "Multiple runs should produce identical output")
}
