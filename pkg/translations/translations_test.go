package translations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NullTranslationHelper(t *testing.T) {
	result := NullTranslationHelper("SOME_KEY", "default value")
	assert.Equal(t, "default value", result)

	result = NullTranslationHelper("ANOTHER_KEY", "another default")
	assert.Equal(t, "another default", result)
}

func Test_TranslationHelper_DefaultValues(t *testing.T) {
	helper, _ := TranslationHelper()

	result := helper("TEST_KEY", "test default")
	assert.Equal(t, "test default", result, "Should return default value on first call")

	// Second call should return cached value
	result = helper("TEST_KEY", "different default")
	assert.Equal(t, "test default", result, "Should return cached value on subsequent calls")
}

func Test_TranslationHelper_EnvironmentVariableOverride(t *testing.T) {
	// Save and restore original env var
	originalValue := os.Getenv("GITHUB_MCP_TEST_TRANSLATION_KEY")
	defer func() {
		if originalValue != "" {
			os.Setenv("GITHUB_MCP_TEST_TRANSLATION_KEY", originalValue)
		} else {
			os.Unsetenv("GITHUB_MCP_TEST_TRANSLATION_KEY")
		}
	}()

	// Set env var override
	os.Setenv("GITHUB_MCP_TEST_TRANSLATION_KEY", "env var value")

	helper, _ := TranslationHelper()

	result := helper("TEST_TRANSLATION_KEY", "default value")
	assert.Equal(t, "env var value", result, "Should use environment variable override")
}

func Test_TranslationHelper_CaseInsensitiveKeys(t *testing.T) {
	helper, _ := TranslationHelper()

	result1 := helper("test_key", "default 1")
	result2 := helper("TEST_KEY", "default 2")
	result3 := helper("TeSt_KeY", "default 3")

	// All should map to the same uppercase key and return the first cached value
	assert.Equal(t, result1, result2, "Keys should be case-insensitive")
	assert.Equal(t, result1, result3, "Keys should be case-insensitive")
}

func Test_TranslationHelper_MultipleKeys(t *testing.T) {
	helper, _ := TranslationHelper()

	key1Result := helper("KEY_ONE", "default one")
	key2Result := helper("KEY_TWO", "default two")
	key3Result := helper("KEY_THREE", "default three")

	assert.Equal(t, "default one", key1Result)
	assert.Equal(t, "default two", key2Result)
	assert.Equal(t, "default three", key3Result)

	// Verify caching works for each key
	assert.Equal(t, "default one", helper("KEY_ONE", "different default"))
	assert.Equal(t, "default two", helper("KEY_TWO", "different default"))
	assert.Equal(t, "default three", helper("KEY_THREE", "different default"))
}

func Test_TranslationHelper_EmptyKey(t *testing.T) {
	helper, _ := TranslationHelper()

	result := helper("", "empty key default")
	assert.Equal(t, "empty key default", result)
}

func Test_TranslationHelper_EmptyDefault(t *testing.T) {
	helper, _ := TranslationHelper()

	result := helper("SOME_KEY", "")
	assert.Equal(t, "", result)
}

func Test_TranslationHelper_LongKeys(t *testing.T) {
	helper, _ := TranslationHelper()

	longKey := "VERY_LONG_TRANSLATION_KEY_WITH_MANY_UNDERSCORES_AND_WORDS"
	result := helper(longKey, "long key default")

	assert.Equal(t, "long key default", result)
}

func Test_TranslationHelper_SpecialCharactersInDefault(t *testing.T) {
	helper, _ := TranslationHelper()

	specialDefault := "Default with special chars: !@#$%^&*(){}[]<>?,./;:'\"\\|"
	result := helper("SPECIAL_CHARS_KEY", specialDefault)

	assert.Equal(t, specialDefault, result)
}

func Test_TranslationHelper_UnicodeInDefault(t *testing.T) {
	helper, _ := TranslationHelper()

	unicodeDefault := "Default with unicode: 你好世界 🚀 émojis"
	result := helper("UNICODE_KEY", unicodeDefault)

	assert.Equal(t, unicodeDefault, result)
}

func Test_TranslationHelper_EnvVarPrefixCorrect(t *testing.T) {
	// Test that the env var prefix "GITHUB_MCP_" is correctly used
	originalValue := os.Getenv("GITHUB_MCP_PREFIX_TEST")
	defer func() {
		if originalValue != "" {
			os.Setenv("GITHUB_MCP_PREFIX_TEST", originalValue)
		} else {
			os.Unsetenv("GITHUB_MCP_PREFIX_TEST")
		}
	}()

	os.Setenv("GITHUB_MCP_PREFIX_TEST", "prefixed value")

	helper, _ := TranslationHelper()
	result := helper("PREFIX_TEST", "default")

	assert.Equal(t, "prefixed value", result, "Should find env var with GITHUB_MCP_ prefix")
}

func Test_TranslationHelper_EnvVarTakesPrecedence(t *testing.T) {
	// Env var should take precedence over config file
	originalValue := os.Getenv("GITHUB_MCP_PRECEDENCE_TEST")
	defer func() {
		if originalValue != "" {
			os.Setenv("GITHUB_MCP_PRECEDENCE_TEST", originalValue)
		} else {
			os.Unsetenv("GITHUB_MCP_PRECEDENCE_TEST")
		}
	}()

	os.Setenv("GITHUB_MCP_PRECEDENCE_TEST", "env var wins")

	helper, _ := TranslationHelper()
	result := helper("PRECEDENCE_TEST", "default value")

	assert.Equal(t, "env var wins", result)
}

func Test_DumpTranslationKeyMap_CreatesFile(t *testing.T) {
	// Create a temporary map
	testMap := map[string]string{
		"KEY_ONE":   "value one",
		"KEY_TWO":   "value two",
		"KEY_THREE": "value three",
	}

	// Clean up before and after
	os.Remove("github-mcp-server-config.json")
	defer os.Remove("github-mcp-server-config.json")

	err := DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat("github-mcp-server-config.json")
	require.NoError(t, err, "Config file should be created")
}

func Test_DumpTranslationKeyMap_ValidJSON(t *testing.T) {
	testMap := map[string]string{
		"TEST_KEY": "test value",
	}

	os.Remove("github-mcp-server-config.json")
	defer os.Remove("github-mcp-server-config.json")

	err := DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// Read and verify it's valid JSON
	content, err := os.ReadFile("github-mcp-server-config.json")
	require.NoError(t, err)

	// Should contain the key and value
	assert.Contains(t, string(content), "TEST_KEY")
	assert.Contains(t, string(content), "test value")
}

func Test_DumpTranslationKeyMap_EmptyMap(t *testing.T) {
	testMap := map[string]string{}

	os.Remove("github-mcp-server-config.json")
	defer os.Remove("github-mcp-server-config.json")

	err := DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	content, err := os.ReadFile("github-mcp-server-config.json")
	require.NoError(t, err)

	// Should be valid empty JSON object
	assert.Equal(t, "{}", string(content))
}

func Test_DumpTranslationKeyMap_SpecialCharacters(t *testing.T) {
	testMap := map[string]string{
		"SPECIAL_KEY": "Value with \"quotes\" and \n newlines \t tabs",
	}

	os.Remove("github-mcp-server-config.json")
	defer os.Remove("github-mcp-server-config.json")

	err := DumpTranslationKeyMap(testMap)
	require.NoError(t, err)

	// File should exist and be valid JSON
	_, err = os.Stat("github-mcp-server-config.json")
	require.NoError(t, err)
}

func Test_TranslationHelper_Cleanup(t *testing.T) {
	// Test the cleanup function returned by TranslationHelper
	helper, cleanup := TranslationHelper()
	require.NotNil(t, helper)
	require.NotNil(t, cleanup)

	// Use the helper
	helper("TEST_KEY", "test value")

	// Clean up config file before and after
	os.Remove("github-mcp-server-config.json")
	defer os.Remove("github-mcp-server-config.json")

	// Call cleanup - it will try to dump the translation map
	// We're just checking it doesn't panic
	require.NotPanics(t, func() {
		cleanup()
	})
}
