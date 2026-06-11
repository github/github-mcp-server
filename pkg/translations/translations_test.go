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
	assert.Equal(t, "default value", NullTranslationHelper("ANY_KEY", "default value"))
	assert.Equal(t, "", NullTranslationHelper("ANY_KEY", ""))
}

func TestTranslationHelper_DefaultValue(t *testing.T) {
	// Run in an isolated working directory so no stray config file is read.
	t.Chdir(t.TempDir())

	helper, dump := TranslationHelper()
	require.NotNil(t, helper)
	require.NotNil(t, dump)

	assert.Equal(t, "fallback", helper("some_unset_key", "fallback"))
}

func TestTranslationHelper_EnvOverride(t *testing.T) {
	t.Chdir(t.TempDir())

	// The helper upper-cases the key and looks for GITHUB_MCP_<KEY>.
	t.Setenv("GITHUB_MCP_GREETING", "hola")

	helper, _ := TranslationHelper()
	assert.Equal(t, "hola", helper("greeting", "hello"))
	// A second lookup hits the cached value (exercises the cache branch).
	assert.Equal(t, "hola", helper("greeting", "hello"))
}

func TestTranslationHelper_DumpWritesConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	t.Setenv("GITHUB_MCP_GREETING", "hej")

	helper, dump := TranslationHelper()
	// Populate the map with both an env override and a defaulted value.
	helper("greeting", "hello")
	helper("farewell", "bye")

	dump()

	data, err := os.ReadFile(filepath.Join(dir, "github-mcp-server-config.json"))
	require.NoError(t, err)

	var out map[string]string
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "hej", out["GREETING"])
	assert.Equal(t, "bye", out["FAREWELL"])
}

func TestDumpTranslationKeyMap(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := DumpTranslationKeyMap(map[string]string{"KEY_ONE": "value one"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "github-mcp-server-config.json"))
	require.NoError(t, err)

	var out map[string]string
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "value one", out["KEY_ONE"])
}
