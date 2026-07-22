package main

import (
	"os"
	"path/filepath"
	"testing"

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
