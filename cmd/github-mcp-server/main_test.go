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

func TestValidateBoolSettingsFromEnv(t *testing.T) {
	tests := []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "TRUE", want: true},
		{value: "t", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "yes", wantErr: true},
		{value: "on", wantErr: true},
		{value: "enabled", wantErr: true},
		{value: "tru", wantErr: true},
	}

	for _, envName := range []string{"GITHUB_READ_ONLY", "GITHUB_LOCKDOWN_MODE"} {
		key := strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(envName, "GITHUB_")), "_", "-")
		for _, tc := range tests {
			t.Run(envName+"="+tc.value, func(t *testing.T) {
				t.Setenv(envName, tc.value)
				initConfig()

				err := validateBoolSettings(boolSettings...)
				if tc.wantErr {
					require.Error(t, err)
					assert.Contains(t, err.Error(), envName)
					assert.Contains(t, err.Error(), tc.value)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tc.want, viper.GetBool(key))
			})
		}
	}

	t.Run("unset", func(t *testing.T) {
		initConfig()
		require.NoError(t, validateBoolSettings(boolSettings...))
		assert.False(t, viper.GetBool("read-only"))
	})
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
