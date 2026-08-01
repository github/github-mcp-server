package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2/internal/assert"
)

func TestConvert(t *testing.T) {
	examples := []struct {
		name          string
		input         string
		expected      string
		errors        bool
		useJsonNumber bool
	}{
		{
			name: "valid json",
			input: `
{
  "mytoml": {
    "a": 42
  }
}`,
			expected: `[mytoml]
a = 42.0
`,
		},
		{
			name:          "use json number",
			useJsonNumber: true,
			input: `
{
  "mytoml": {
    "a": 42
  }
}`,
			expected: `[mytoml]
a = 42
`,
		},
		{
			name:   "invalid json",
			input:  `{ foo`,
			errors: true,
		},
	}

	for _, e := range examples {
		b := new(bytes.Buffer)
		useJsonNumber = e.useJsonNumber
		err := convert(strings.NewReader(e.input), b)
		if e.errors {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, e.expected, b.String())
		}
	}
}
