package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
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
