package headers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single value",
			input:    "foo",
			expected: []string{"foo"},
		},
		{
			name:     "multiple values",
			input:    "foo,bar,baz",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "whitespace trimmed",
			input:    " foo , bar , baz ",
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "empty values filtered",
			input:    "foo,,bar,",
			expected: []string{"foo", "bar"},
		},
		{
			name:     "only commas",
			input:    ",,,",
			expected: []string{},
		},
		{
			name:     "whitespace only values filtered",
			input:    "foo,   ,bar",
			expected: []string{"foo", "bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseCommaSeparated(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
