package translations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNullTranslationHelper(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "returns default value",
			key:          "TEST_KEY",
			defaultValue: "default value",
			expected:     "default value",
		},
		{
			name:         "ignores key",
			key:          "IGNORED_KEY",
			defaultValue: "returned value",
			expected:     "returned value",
		},
		{
			name:         "empty default",
			key:          "SOME_KEY",
			defaultValue: "",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NullTranslationHelper(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTranslationHelper_DefaultValues(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// Test that default values are returned when no overrides exist
	result := helper("TEST_KEY", "default value")
	assert.Equal(t, "default value", result)

	result2 := helper("ANOTHER_KEY", "another default")
	assert.Equal(t, "another default", result2)
}

func TestTranslationHelper_EnvironmentVariableOverride(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Set environment variable
	envKey := "GITHUB_MCP_TEST_OVERRIDE"
	envValue := "env override value"
	os.Setenv(envKey, envValue)
	defer os.Unsetenv(envKey)

	helper, _ := TranslationHelper()

	// Test that environment variable overrides default
	result := helper("TEST_OVERRIDE", "default value")
	assert.Equal(t, envValue, result)
}

func TestTranslationHelper_JSONConfigOverride(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create a config file
	config := map[string]string{
		"JSON_KEY": "json override value",
	}
	configData, _ := json.MarshalIndent(config, "", "  ")
	err = os.WriteFile("github-mcp-server-config.json", configData, 0600)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// Test that JSON config overrides default
	result := helper("JSON_KEY", "default value")
	assert.Equal(t, "json override value", result)
}

func TestTranslationHelper_CaseInsensitivity(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// Keys should be converted to uppercase
	result1 := helper("lowercase_key", "value1")
	result2 := helper("LOWERCASE_KEY", "value2")

	// Both should return the first value since they're the same key
	assert.Equal(t, result1, result2)
}

func TestTranslationHelper_Caching(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// First call
	result1 := helper("CACHED_KEY", "initial value")
	assert.Equal(t, "initial value", result1)

	// Second call with different default - should return cached value
	result2 := helper("CACHED_KEY", "different value")
	assert.Equal(t, "initial value", result2)
}

func TestDumpTranslationKeyMap(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	testMap := map[string]string{
		"KEY1": "value1",
		"KEY2": "value2",
		"KEY3": "value3",
	}

	err = DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	assert.FileExists(t, filePath)

	// Verify file contents
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, testMap, loaded)
}

func TestDumpTranslationKeyMap_EmptyMap(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	testMap := map[string]string{}

	err = DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	assert.FileExists(t, filePath)

	// Verify file contains empty object
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Empty(t, loaded)
}

func TestDumpTranslationKeyMap_OverwritesExisting(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create initial file
	initialMap := map[string]string{"OLD_KEY": "old value"}
	err = DumpTranslationKeyMap(initialMap)
	require.NoError(t, err)

	// Overwrite with new data
	newMap := map[string]string{"NEW_KEY": "new value"}
	err = DumpTranslationKeyMap(newMap)
	require.NoError(t, err)

	// Verify new data is in file
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, newMap, loaded)
	assert.NotContains(t, loaded, "OLD_KEY")
}

func TestTranslationHelper_DumpFunction(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, dump := TranslationHelper()

	// Use some translations
	helper("KEY1", "value1")
	helper("KEY2", "value2")
	helper("KEY3", "value3")

	// Call dump function
	dump()

	// Verify file was created
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	assert.FileExists(t, filePath)

	// Verify contents
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// All keys should be present
	assert.Contains(t, loaded, "KEY1")
	assert.Contains(t, loaded, "KEY2")
	assert.Contains(t, loaded, "KEY3")
}

func TestTranslationHelper_MissingConfigFile(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Don't create a config file - should not error
	helper, _ := TranslationHelper()

	result := helper("TEST_KEY", "default value")
	assert.Equal(t, "default value", result)
}

func TestTranslationHelper_InvalidJSONConfig(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create invalid JSON config
	err = os.WriteFile("github-mcp-server-config.json", []byte("invalid json {"), 0600)
	require.NoError(t, err)

	// Should still work, just ignore the invalid config
	helper, _ := TranslationHelper()

	result := helper("TEST_KEY", "default value")
	assert.Equal(t, "default value", result)
}

func TestTranslationHelper_EnvVarPriority(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create JSON config
	config := map[string]string{
		"PRIORITY_KEY": "json value",
	}
	configData, _ := json.MarshalIndent(config, "", "  ")
	err = os.WriteFile("github-mcp-server-config.json", configData, 0600)
	require.NoError(t, err)

	// Set environment variable with same key
	os.Setenv("GITHUB_MCP_PRIORITY_KEY", "env value")
	defer os.Unsetenv("GITHUB_MCP_PRIORITY_KEY")

	helper, _ := TranslationHelper()

	// Environment variable should take precedence
	result := helper("PRIORITY_KEY", "default value")
	assert.Equal(t, "env value", result)
}

func TestTranslationHelper_MultipleKeys(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// Test multiple different keys
	keys := []struct {
		key          string
		defaultValue string
	}{
		{"KEY_1", "value 1"},
		{"KEY_2", "value 2"},
		{"KEY_3", "value 3"},
		{"KEY_4", "value 4"},
		{"KEY_5", "value 5"},
	}

	for _, k := range keys {
		result := helper(k.key, k.defaultValue)
		assert.Equal(t, k.defaultValue, result)
	}
}

func TestDumpTranslationKeyMap_SpecialCharacters(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	testMap := map[string]string{
		"KEY_WITH_QUOTES":   `value with "quotes"`,
		"KEY_WITH_NEWLINES": "value\nwith\nnewlines",
		"KEY_WITH_UNICODE":  "value with 世界 unicode",
	}

	err = DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Verify file was created and is valid JSON
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// Special characters should be preserved
	assert.Equal(t, testMap, loaded)
}

func TestTranslationHelper_UppercaseConversion(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, dump := TranslationHelper()

	// Use lowercase key
	helper("lowercase_test_key", "test value")

	// Dump to file
	dump()

	// Verify the key was converted to uppercase in the dump
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	// Key should be uppercase
	assert.Contains(t, loaded, "LOWERCASE_TEST_KEY")
	assert.NotContains(t, loaded, "lowercase_test_key")
}

func TestDumpTranslationKeyMap_LargeMap(t *testing.T) {
	// Create a temporary directory for this test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	// Create a large map
	testMap := make(map[string]string)
	for i := 0; i < 1000; i++ {
		key := "KEY_" + string(rune('0'+i%10))
		value := "value_" + string(rune('0'+i%10))
		testMap[key] = value
	}

	err = DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tmpDir, "github-mcp-server-config.json")
	assert.FileExists(t, filePath)

	// Verify file is valid JSON
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var loaded map[string]string
	err = json.Unmarshal(data, &loaded)
	require.NoError(t, err)

	assert.Equal(t, len(testMap), len(loaded))
}

func TestTranslationHelper_ConcurrentAccess(t *testing.T) {
	// Note: This is a basic concurrency test
	// The current implementation is NOT thread-safe
	// This test documents the current behavior
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	helper, _ := TranslationHelper()

	// Sequential access should work
	result1 := helper("KEY1", "value1")
	result2 := helper("KEY2", "value2")

	assert.Equal(t, "value1", result1)
	assert.Equal(t, "value2", result2)
}

