package viper

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type finderStub struct {
	results []string
}

func (f finderStub) Find(_ afero.Fs) ([]string, error) {
	return f.results, nil
}

func TestFinders(t *testing.T) {
	finder := Finders(
		finderStub{
			results: []string{
				"/home/user/.viper.yaml",
			},
		},
		finderStub{
			results: []string{
				"/etc/viper/config.yaml",
			},
		},
	)

	results, err := finder.Find(afero.NewMemMapFs())
	require.NoError(t, err)

	expected := []string{
		"/home/user/.viper.yaml",
		"/etc/viper/config.yaml",
	}

	assert.Equal(t, expected, results)
}
