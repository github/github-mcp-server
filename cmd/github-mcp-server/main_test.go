package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/github"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAppPrivateKey(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.pem")
		require.NoError(t, os.WriteFile(path, []byte("from-file"), 0o600))

		key, err := loadAppPrivateKey(path, "from-inline")
		require.NoError(t, err)
		assert.Equal(t, []byte("from-file"), key)
	})

	t.Run("inline", func(t *testing.T) {
		key, err := loadAppPrivateKey("", `first\nsecond`)
		require.NoError(t, err)
		assert.Equal(t, []byte("first\nsecond"), key)
	})

	t.Run("missing", func(t *testing.T) {
		_, err := loadAppPrivateKey("", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "private key")
	})
}

func TestGitHubAppFlagsAreStdioOnly(t *testing.T) {
	assert.NotNil(t, stdioCmd.Flags().Lookup("app-id"))
	assert.Nil(t, httpCmd.Flags().Lookup("app-id"))
}

func TestAuthorizationServerConfigurationIsHTTPOnly(t *testing.T) {
	flag := httpCmd.Flags().Lookup("authorization-server")
	require.NotNil(t, flag)
	assert.Empty(t, flag.DefValue)
	assert.Nil(t, stdioCmd.Flags().Lookup("authorization-server"))

	t.Setenv("GITHUB_AUTHORIZATION_SERVER", "")
	initConfig()
	assert.Empty(t, viper.GetString("authorization-server"))

	t.Setenv("GITHUB_AUTHORIZATION_SERVER", "https://oauth-proxy.example.com")
	assert.Equal(t, "https://oauth-proxy.example.com", viper.GetString("authorization-server"))
}

func TestMCPAppsDefaultsOnAndEnvCanDisable(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("mcp-apps")
	require.NotNil(t, flag)
	assert.Equal(t, "true", flag.DefValue)

	original := flag.Value.String()
	originalChanged := flag.Changed
	t.Cleanup(func() {
		require.NoError(t, flag.Value.Set(original))
		flag.Changed = originalChanged
	})

	require.NoError(t, flag.Value.Set("true"))
	flag.Changed = false
	t.Setenv("GITHUB_MCP_APPS", "")
	initConfig()
	assert.True(t, mcpAppsEnabled())

	t.Setenv("GITHUB_MCP_APPS", "false")
	initConfig()
	assert.False(t, mcpAppsEnabled())

	t.Setenv("GITHUB_MCP_APPS", "")
	require.NoError(t, flag.Value.Set("false"))
	flag.Changed = true
	assert.False(t, mcpAppsEnabled())
}

func TestWriteToolDocScopes(t *testing.T) {
	tool := inventory.ServerTool{
		Tool:        mcp.Tool{Name: "delete", Annotations: &mcp.ToolAnnotations{Title: "Delete"}},
		ScopeAccess: scopes.RequireAll(scopes.DeleteRepo, scopes.Repo),
	}

	var buf strings.Builder
	writeToolDoc(&buf, tool)
	assert.Contains(t, buf.String(), "**OAuth Challenge Scopes**: `delete_repo`, `repo`")
}

func TestDependabotUpdateListScopes(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		inv, err := github.NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{"dependabot"}).WithReadOnly(readOnly).Build()
		require.NoError(t, err)
		output := collectToolScopes(inv, readOnly)
		if readOnly {
			assert.NotContains(t, output.ScopesByTool, "update_dependabot_alert")
			continue
		}
		assert.Equal(t, []string{"security_events"}, output.ScopesByTool["update_dependabot_alert"])
		assert.Contains(t, output.ToolsByScope["security_events"], "update_dependabot_alert")
		assert.Contains(t, output.Tools, ToolScopeInfo{
			Name: "update_dependabot_alert", Toolset: "dependabot",
			ReadOnly: false, ChallengeScopes: []string{"security_events"},
		})
	}
}

func TestCodeScanningUpdateListScopes(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		inv, err := github.NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{"code_security"}).WithReadOnly(readOnly).Build()
		require.NoError(t, err)
		output := collectToolScopes(inv, readOnly)
		if readOnly {
			assert.NotContains(t, output.ScopesByTool, "update_code_scanning_alert")
			continue
		}
		assert.Equal(t, []string{"security_events"}, output.ScopesByTool["update_code_scanning_alert"])
		assert.Contains(t, output.ToolsByScope["security_events"], "update_code_scanning_alert")
		assert.Contains(t, output.Tools, ToolScopeInfo{
			Name: "update_code_scanning_alert", Toolset: "code_security",
			ReadOnly: false, ChallengeScopes: []string{"security_events"},
		})
	}
}

func TestSecretScanningUpdateListScopes(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		inv, err := github.NewInventory(translations.NullTranslationHelper).
			WithToolsets([]string{"secret_protection"}).WithReadOnly(readOnly).Build()
		require.NoError(t, err)
		output := collectToolScopes(inv, readOnly)
		if readOnly {
			assert.NotContains(t, output.ScopesByTool, "update_secret_scanning_alert")
			continue
		}
		assert.Equal(t, []string{"security_events"}, output.ScopesByTool["update_secret_scanning_alert"])
		assert.Contains(t, output.ToolsByScope["security_events"], "update_secret_scanning_alert")
		assert.Contains(t, output.Tools, ToolScopeInfo{
			Name: "update_secret_scanning_alert", Toolset: "secret_protection",
			ReadOnly: false, ChallengeScopes: []string{"security_events"},
		})
	}
}

func TestSchemaTypeString(t *testing.T) {
	tests := []struct {
		name   string
		schema *jsonschema.Schema
		want   string
	}{
		{name: "type", schema: &jsonschema.Schema{Type: "string"}, want: "string"},
		{name: "types", schema: &jsonschema.Schema{Types: []string{"string", "number"}}, want: "string | number"},
		{name: "unconstrained", schema: &jsonschema.Schema{}, want: "any"},
		{name: "anyOf", schema: &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{Type: "string"}, {Type: "null"}}}, want: "string | null"},
		{name: "oneOf", schema: &jsonschema.Schema{OneOf: []*jsonschema.Schema{{Type: "number"}, {Type: "string"}}}, want: "number | string"},
		{
			name:   "array",
			schema: &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "string"}},
			want:   "string[]",
		},
		{name: "untyped array", schema: &jsonschema.Schema{Type: "array"}, want: "array"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, schemaTypeString(tc.schema))
		})
	}
}
