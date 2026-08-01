package viper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type codec struct{}

func (codec) Encode(_ map[string]any) ([]byte, error) {
	return nil, nil
}

func (codec) Decode(_ []byte, _ map[string]any) error {
	return nil
}

func TestDefaultCodecRegistry(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		registry := NewCodecRegistry()

		c := codec{}

		err := registry.RegisterCodec("myformat", c)
		require.NoError(t, err)

		encoder, err := registry.Encoder("myformat")
		require.NoError(t, err)

		assert.Equal(t, c, encoder)

		decoder, err := registry.Decoder("myformat")
		require.NoError(t, err)

		assert.Equal(t, c, decoder)
	})

	t.Run("CodecNotFound", func(t *testing.T) {
		registry := NewCodecRegistry()

		_, err := registry.Encoder("myformat")
		require.Error(t, err)

		_, err = registry.Decoder("myformat")
		require.Error(t, err)
	})

	t.Run("FormatIsCaseInsensitive", func(t *testing.T) {
		registry := NewCodecRegistry()

		c := codec{}

		err := registry.RegisterCodec("MYFORMAT", c)
		require.NoError(t, err)

		{
			encoder, err := registry.Encoder("myformat")
			require.NoError(t, err)

			assert.Equal(t, c, encoder)
		}

		{
			encoder, err := registry.Encoder("MYFORMAT")
			require.NoError(t, err)

			assert.Equal(t, c, encoder)
		}

		{
			decoder, err := registry.Decoder("myformat")
			require.NoError(t, err)

			assert.Equal(t, c, decoder)
		}

		{
			decoder, err := registry.Decoder("MYFORMAT")
			require.NoError(t, err)

			assert.Equal(t, c, decoder)
		}
	})

	t.Run("OverrideDefault", func(t *testing.T) {
		registry := NewCodecRegistry()

		c := codec{}

		err := registry.RegisterCodec("yaml", c)
		require.NoError(t, err)

		encoder, err := registry.Encoder("yaml")
		require.NoError(t, err)

		assert.Equal(t, c, encoder)

		decoder, err := registry.Decoder("yaml")
		require.NoError(t, err)

		assert.Equal(t, c, decoder)
	})
}
