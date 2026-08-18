package requeststate

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSealer(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	sealer, err := New(key)
	require.NoError(t, err)

	t.Run("round trip", func(t *testing.T) {
		plaintext := []byte(`{"owner":"octo","repo":"repo"}`)
		token, err := sealer.Seal(context.Background(), plaintext)
		require.NoError(t, err)
		assert.NotContains(t, token, string(plaintext))

		opened, err := sealer.Open(token)
		require.NoError(t, err)
		assert.Equal(t, plaintext, opened)
	})

	t.Run("rejects tampering", func(t *testing.T) {
		token, err := sealer.Seal(context.Background(), []byte("state"))
		require.NoError(t, err)
		replacement := "A"
		if strings.HasSuffix(token, replacement) {
			replacement = "B"
		}

		_, err = sealer.Open(token[:len(token)-1] + replacement)
		require.Error(t, err)
	})
}

func TestNewRandom(t *testing.T) {
	sealer, err := NewRandom()
	require.NoError(t, err)
	token, err := sealer.Seal(context.Background(), []byte("state"))
	require.NoError(t, err)
	opened, err := sealer.Open(token)
	require.NoError(t, err)
	assert.Equal(t, []byte("state"), opened)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "empty key"},
		{name: "invalid Base64", key: "not-base64"},
		{name: "wrong decoded length", key: base64.StdEncoding.EncodeToString([]byte("too short"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.key)
			require.Error(t, err)
		})
	}
}
