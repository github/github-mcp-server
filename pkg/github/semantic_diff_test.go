package github

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticDiffJSON(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		base           string
		head           string
		expectedFormat DiffFormat
		expectedDiff   string
		notContains    string
	}{
		{
			name:           "no changes",
			path:           "config.json",
			base:           `{"key": "value"}`,
			head:           `{"key": "value"}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "no changes detected",
		},
		{
			name:           "simple value change",
			path:           "config.json",
			base:           `{"theme": "light"}`,
			head:           `{"theme": "dark"}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   `theme: "light" → "dark"`,
		},
		{
			name:           "added key",
			path:           "config.json",
			base:           `{"a": 1}`,
			head:           `{"a": 1, "b": 2}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "b: added 2",
		},
		{
			name:           "removed key",
			path:           "config.json",
			base:           `{"a": 1, "b": 2}`,
			head:           `{"a": 1}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "b: removed (was 2)",
		},
		{
			name:           "nested object change",
			path:           "config.json",
			base:           `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]}`,
			head:           `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bobby"}]}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   `users[1].name: "Bob" → "Bobby"`,
		},
		{
			name:           "reformatting only - no semantic change",
			path:           "config.json",
			base:           `{"key":"value","number":42}`,
			head:           "{\n  \"key\": \"value\",\n  \"number\": 42\n}",
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "no changes detected",
		},
		{
			name:           "array element added",
			path:           "data.json",
			base:           `[1, 2, 3]`,
			head:           `[1, 2, 3, 4]`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "[3]: added 4",
		},
		{
			name:           "array element removed",
			path:           "data.json",
			base:           `[1, 2, 3]`,
			head:           `[1, 2]`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   "[2]: removed (was 3)",
		},
		{
			name:           "type change",
			path:           "config.json",
			base:           `{"val": "string"}`,
			head:           `{"val": 123}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   `val: "string" → 123`,
		},
		{
			name:           "null values",
			path:           "config.json",
			base:           `{"val": null}`,
			head:           `{"val": "something"}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   `val: null → "something"`,
		},
		{
			name:           "boolean change",
			path:           "config.json",
			base:           `{"enabled": true}`,
			head:           `{"enabled": false}`,
			expectedFormat: DiffFormatJSON,
			expectedDiff:   `enabled: true → false`,
		},
		{
			name:           "invalid base JSON falls back",
			path:           "config.json",
			base:           `not json`,
			head:           `{"key": "value"}`,
			expectedFormat: DiffFormatFallback,
		},
		{
			name:           "invalid head JSON falls back",
			path:           "config.json",
			base:           `{"key": "value"}`,
			head:           `not json`,
			expectedFormat: DiffFormatFallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff(tc.path, []byte(tc.base), []byte(tc.head))
			assert.Equal(t, tc.expectedFormat, result.Format)
			if tc.expectedDiff != "" {
				assert.Contains(t, result.Diff, tc.expectedDiff)
			}
			if tc.notContains != "" {
				assert.NotContains(t, result.Diff, tc.notContains)
			}
		})
	}
}

func TestSemanticDiffYAML(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		base           string
		head           string
		expectedFormat DiffFormat
		expectedDiff   string
	}{
		{
			name:           "no changes",
			path:           "config.yaml",
			base:           "key: value\n",
			head:           "key: value\n",
			expectedFormat: DiffFormatYAML,
			expectedDiff:   "no changes detected",
		},
		{
			name:           "simple value change",
			path:           "config.yml",
			base:           "theme: light\n",
			head:           "theme: dark\n",
			expectedFormat: DiffFormatYAML,
			expectedDiff:   `theme: "light" → "dark"`,
		},
		{
			name:           "nested key change",
			path:           "config.yaml",
			base:           "database:\n  host: localhost\n  port: 5432\n",
			head:           "database:\n  host: production.db\n  port: 5432\n",
			expectedFormat: DiffFormatYAML,
			expectedDiff:   `database.host: "localhost" → "production.db"`,
		},
		{
			name:           "added key",
			path:           "config.yaml",
			base:           "a: 1\n",
			head:           "a: 1\nb: 2\n",
			expectedFormat: DiffFormatYAML,
			expectedDiff:   "b: added 2",
		},
		{
			name:           "invalid YAML falls back",
			path:           "config.yaml",
			base:           ":\n  bad:\nyaml",
			head:           "key: value\n",
			expectedFormat: DiffFormatFallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff(tc.path, []byte(tc.base), []byte(tc.head))
			assert.Equal(t, tc.expectedFormat, result.Format)
			if tc.expectedDiff != "" {
				assert.Contains(t, result.Diff, tc.expectedDiff)
			}
		})
	}
}

func TestSemanticDiffCSV(t *testing.T) {
	tests := []struct {
		name           string
		base           string
		head           string
		expectedFormat DiffFormat
		expectedDiff   string
	}{
		{
			name:           "no changes",
			base:           "name,age\nAlice,30\nBob,25\n",
			head:           "name,age\nAlice,30\nBob,25\n",
			expectedFormat: DiffFormatCSV,
			expectedDiff:   "no changes detected",
		},
		{
			name:           "cell value change",
			base:           "name,status\nAlice,active\nBob,pending\n",
			head:           "name,status\nAlice,active\nBob,shipped\n",
			expectedFormat: DiffFormatCSV,
			expectedDiff:   `row 2.status: "pending" → "shipped"`,
		},
		{
			name:           "row added",
			base:           "name,age\nAlice,30\n",
			head:           "name,age\nAlice,30\nBob,25\n",
			expectedFormat: DiffFormatCSV,
			expectedDiff:   "row 2: added",
		},
		{
			name:           "row removed",
			base:           "name,age\nAlice,30\nBob,25\n",
			head:           "name,age\nAlice,30\n",
			expectedFormat: DiffFormatCSV,
			expectedDiff:   "row 2: removed",
		},
		{
			name:           "header change",
			base:           "name,age\nAlice,30\n",
			head:           "name,email\nAlice,alice@example.com\n",
			expectedFormat: DiffFormatCSV,
			expectedDiff:   "headers changed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff("data.csv", []byte(tc.base), []byte(tc.head))
			assert.Equal(t, tc.expectedFormat, result.Format)
			if tc.expectedDiff != "" {
				assert.Contains(t, result.Diff, tc.expectedDiff)
			}
		})
	}
}

func TestSemanticDiffTOML(t *testing.T) {
	tests := []struct {
		name           string
		base           string
		head           string
		expectedFormat DiffFormat
		expectedDiff   string
	}{
		{
			name:           "no changes",
			base:           "key = \"value\"\n",
			head:           "key = \"value\"\n",
			expectedFormat: DiffFormatTOML,
			expectedDiff:   "no changes detected",
		},
		{
			name:           "value change",
			base:           "title = \"old\"\n",
			head:           "title = \"new\"\n",
			expectedFormat: DiffFormatTOML,
			expectedDiff:   `title: "old" → "new"`,
		},
		{
			name:           "nested table change",
			base:           "[database]\nhost = \"localhost\"\nport = 5432\n",
			head:           "[database]\nhost = \"production.db\"\nport = 5432\n",
			expectedFormat: DiffFormatTOML,
			expectedDiff:   `database.host: "localhost" → "production.db"`,
		},
		{
			name:           "invalid TOML falls back",
			base:           "not valid toml [[[",
			head:           "key = \"value\"\n",
			expectedFormat: DiffFormatFallback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff("config.toml", []byte(tc.base), []byte(tc.head))
			assert.Equal(t, tc.expectedFormat, result.Format)
			if tc.expectedDiff != "" {
				assert.Contains(t, result.Diff, tc.expectedDiff)
			}
		})
	}
}

func TestSemanticDiffUnifiedFallback(t *testing.T) {
	t.Run("Go file uses structural diff", func(t *testing.T) {
		result := SemanticDiff("main.go", []byte("func main() {\n}\n"), []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n"))
		assert.Equal(t, DiffFormatStructural, result.Format)
		assert.Contains(t, result.Diff, "function_declaration main: modified")
	})

	t.Run("no extension uses unified diff", func(t *testing.T) {
		result := SemanticDiff("Makefile", []byte("all:\n\techo hello\n"), []byte("all:\n\techo world\n"))
		assert.Equal(t, DiffFormatUnified, result.Format)
		assert.Contains(t, result.Diff, "--- a/Makefile")
	})
}

func TestSemanticDiffFileSizeLimit(t *testing.T) {
	path := "config.json"
	// Create data larger than MaxSemanticDiffFileSize
	large := strings.Repeat("x", MaxSemanticDiffFileSize+1)

	t.Run("large base file", func(t *testing.T) {
		result := SemanticDiff(path, []byte(large), []byte(`{"key":"value"}`))
		assert.Equal(t, DiffFormatFallback, result.Format)
		assert.Contains(t, result.Message, "exceeds maximum size")
	})

	t.Run("large head file", func(t *testing.T) {
		result := SemanticDiff(path, []byte(`{"key":"value"}`), []byte(large))
		assert.Equal(t, DiffFormatFallback, result.Format)
		assert.Contains(t, result.Message, "exceeds maximum size")
	})
}

func TestSemanticDiffNewAndDeletedFiles(t *testing.T) {
	t.Run("new JSON file", func(t *testing.T) {
		result := SemanticDiff("config.json", nil, []byte(`{"key":"value"}`))
		assert.Equal(t, DiffFormatJSON, result.Format)
		assert.Equal(t, "file added", result.Diff)
	})

	t.Run("deleted JSON file", func(t *testing.T) {
		result := SemanticDiff("config.json", []byte(`{"key":"value"}`), nil)
		assert.Equal(t, DiffFormatJSON, result.Format)
		assert.Equal(t, "file deleted", result.Diff)
	})

	t.Run("new YAML file", func(t *testing.T) {
		result := SemanticDiff("config.yaml", nil, []byte("key: value\n"))
		assert.Equal(t, DiffFormatYAML, result.Format)
		assert.Equal(t, "file added", result.Diff)
	})

	t.Run("deleted Go file", func(t *testing.T) {
		result := SemanticDiff("main.go", []byte("package main\n"), nil)
		assert.Equal(t, DiffFormatStructural, result.Format)
		assert.Equal(t, "file deleted", result.Diff)
	})

	t.Run("both nil", func(t *testing.T) {
		result := SemanticDiff("config.json", nil, nil)
		assert.Equal(t, "no changes detected", result.Diff)
	})
}

func TestDetectDiffFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected DiffFormat
	}{
		{"config.json", DiffFormatJSON},
		{"config.JSON", DiffFormatJSON},
		{"config.yaml", DiffFormatYAML},
		{"config.yml", DiffFormatYAML},
		{"data.csv", DiffFormatCSV},
		{"config.toml", DiffFormatTOML},
		{"main.go", DiffFormatStructural},
		{"README.md", DiffFormatUnified},
		{"Makefile", DiffFormatUnified},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.expected, DetectDiffFormat(tc.path))
		})
	}
}

func TestCompareValuesDeepNesting(t *testing.T) {
	base := `{
		"level1": {
			"level2": {
				"level3": {
					"value": "old"
				}
			}
		}
	}`
	head := `{
		"level1": {
			"level2": {
				"level3": {
					"value": "new"
				}
			}
		}
	}`

	result := SemanticDiff("config.json", []byte(base), []byte(head))
	require.Equal(t, DiffFormatJSON, result.Format)
	assert.Contains(t, result.Diff, `level1.level2.level3.value: "old" → "new"`)
}

func TestSemanticDiffMultipleChanges(t *testing.T) {
	base := `{
		"name": "my-app",
		"version": "1.0.0",
		"dependencies": {
			"lodash": "4.17.20",
			"express": "4.17.1"
		}
	}`
	head := `{
		"name": "my-app",
		"version": "1.1.0",
		"dependencies": {
			"lodash": "4.17.21",
			"express": "4.17.1",
			"axios": "0.21.1"
		}
	}`

	result := SemanticDiff("package.json", []byte(base), []byte(head))
	require.Equal(t, DiffFormatJSON, result.Format)
	assert.Contains(t, result.Diff, `version: "1.0.0" → "1.1.0"`)
	assert.Contains(t, result.Diff, `dependencies.lodash: "4.17.20" → "4.17.21"`)
	assert.Contains(t, result.Diff, `dependencies.axios: added "0.21.1"`)
	assert.NotContains(t, result.Diff, "express")
	assert.NotContains(t, result.Diff, "name")
}
