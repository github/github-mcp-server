package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2/internal/assert"
)

func TestConvert(t *testing.T) {
	examples := []struct {
		name     string
		input    string
		expected string
		errors   bool
	}{
		{
			name: "valid toml",
			input: `
mytoml.a = 42.0
`,
			expected: `[mytoml]
a = 42.0
`,
		},
		{
			name:   "invalid toml",
			input:  `[what`,
			errors: true,
		},
	}

	for _, e := range examples {
		b := new(bytes.Buffer)
		err := convert(strings.NewReader(e.input), b)
		if e.errors {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, e.expected, b.String())
		}
	}
}
