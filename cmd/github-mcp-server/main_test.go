package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/pflag"
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

func TestWriteToolDocScopes(t *testing.T) {
	tool := inventory.ServerTool{
		Tool:        mcp.Tool{Name: "delete", Annotations: &mcp.ToolAnnotations{Title: "Delete"}},
		ScopeAccess: scopes.RequireAll(scopes.DeleteRepo, scopes.Repo),
	}

	var buf strings.Builder
	writeToolDoc(&buf, tool)
	assert.Contains(t, buf.String(), "**OAuth Challenge Scopes**: `delete_repo`, `repo`")
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

func TestParseBoolSetting(t *testing.T) {
	tests := []struct {
		envVal   string
		expected bool
	}{
		{"1", true},
		{"t", true},
		{"T", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"y", true},
		{"Y", true},
		{"on", true},
		{"ON", true},
		{"enabled", true},
		{"ENABLED", true},
		{"0", false},
		{"f", false},
		{"F", false},
		{"false", false},
		{"FALSE", false},
		{"False", false},
		{"no", false},
		{"NO", false},
		{"n", false},
		{"N", false},
		{"off", false},
		{"OFF", false},
		{"disabled", false},
		{"DISABLED", false},
	}

	for _, tc := range tests {
		t.Run("read-only="+tc.envVal, func(t *testing.T) {
			t.Setenv("GITHUB_READ_ONLY", tc.envVal)
			flag := rootCmd.PersistentFlags().Lookup("read-only")
			got, err := parseBoolSetting("read-only", flag)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})

		t.Run("lockdown-mode="+tc.envVal, func(t *testing.T) {
			t.Setenv("GITHUB_LOCKDOWN_MODE", tc.envVal)
			flag := rootCmd.PersistentFlags().Lookup("lockdown-mode")
			got, err := parseBoolSetting("lockdown-mode", flag)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}

	t.Run("invalid boolean fails closed with descriptive error", func(t *testing.T) {
		invalidValues := []string{"tru", "flase", "invalid", "2", "none"}
		for _, v := range invalidValues {
			t.Setenv("GITHUB_READ_ONLY", v)
			flag := rootCmd.PersistentFlags().Lookup("read-only")
			_, err := parseBoolSetting("read-only", flag)
			require.Error(t, err)
			assert.Contains(t, err.Error(), `invalid boolean value "`+v+`" for GITHUB_READ_ONLY`)
		}
	})

	t.Run("empty or unset defaults to flag default value", func(t *testing.T) {
		t.Setenv("GITHUB_READ_ONLY", "")
		flag := rootCmd.PersistentFlags().Lookup("read-only")
		got, err := parseBoolSetting("read-only", flag)
		require.NoError(t, err)
		assert.False(t, got)
	})

	t.Run("explicit CLI flag takes precedence over env var", func(t *testing.T) {
		t.Setenv("GITHUB_READ_ONLY", "false")
		flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flagSet.Bool("read-only", false, "read only")
		err := flagSet.Parse([]string{"--read-only"})
		require.NoError(t, err)

		flag := flagSet.Lookup("read-only")
		got, err := parseBoolSetting("read-only", flag)
		require.NoError(t, err)
		assert.True(t, got)
	})
}
