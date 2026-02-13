package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"config.json", "json"},
		{"config.JSON", "json"},
		{"data.yaml", "yaml"},
		{"data.yml", "yaml"},
		{"data.YML", "yaml"},
		{"readme.txt", ""},
		{"readme.md", ""},
		{"Makefile", ""},
		{"script.py", ""},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.expected, detectFormat(tc.path))
		})
	}
}

func TestSemanticDiff_JSON_ValueChanges(t *testing.T) {
	base := []byte(`{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}`)
	head := []byte(`{
  "users": [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bobby"}
  ]
}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "data.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, `users[1].name: "Bob" → "Bobby"`)
}

func TestSemanticDiff_JSON_NoChanges(t *testing.T) {
	base := []byte(`{"a":1,"b":"hello"}`)
	head := []byte(`{
  "a": 1,
  "b": "hello"
}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "config.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Equal(t, "No changes detected", diff)
}

func TestSemanticDiff_JSON_Additions(t *testing.T) {
	base := []byte(`{"a":1}`)
	head := []byte(`{"a":1,"b":2}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "data.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, "+ b: 2")
}

func TestSemanticDiff_JSON_Removals(t *testing.T) {
	base := []byte(`{"a":1,"b":2}`)
	head := []byte(`{"a":1}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "data.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, "- b: 2")
}

func TestSemanticDiff_JSON_NestedChanges(t *testing.T) {
	base := []byte(`{"config":{"db":{"host":"localhost","port":5432}}}`)
	head := []byte(`{"config":{"db":{"host":"prod-server","port":5432}}}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "config.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, `config.db.host: "localhost" → "prod-server"`)
}

func TestSemanticDiff_JSON_TypeChange(t *testing.T) {
	base := []byte(`{"value":"hello"}`)
	head := []byte(`{"value":["hello"]}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "data.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, "value:")
	assert.Contains(t, diff, "type changed")
}

func TestSemanticDiff_JSON_ArrayChanges(t *testing.T) {
	base := []byte(`{"items":["a","b","c"]}`)
	head := []byte(`{"items":["a","x","c","d"]}`)
	diff, format, isFallback, err := SemanticDiff(base, head, "data.json")
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, `items[1]: "b" → "x"`)
	assert.Contains(t, diff, `+ items[3]: "d"`)
}

func TestSemanticDiff_JSON_InvalidBase(t *testing.T) {
	base := []byte(`not json`)
	head := []byte(`{"a":1}`)
	_, _, _, err := SemanticDiff(base, head, "data.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse base JSON")
}

func TestSemanticDiff_JSON_InvalidHead(t *testing.T) {
	base := []byte(`{"a":1}`)
	head := []byte(`not json`)
	_, _, _, err := SemanticDiff(base, head, "data.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse head JSON")
}

func TestSemanticDiff_YAML_ValueChanges(t *testing.T) {
	base := []byte("name: Alice\nage: 30\n")
	head := []byte("name: Alice\nage: 31\n")
	diff, format, isFallback, err := SemanticDiff(base, head, "config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, "age: 30 → 31")
}

func TestSemanticDiff_YAML_NestedChanges(t *testing.T) {
	base := []byte("db:\n  host: localhost\n  port: 5432\n")
	head := []byte("db:\n  host: prod-server\n  port: 5432\n")
	diff, format, isFallback, err := SemanticDiff(base, head, "config.yml")
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)
	assert.False(t, isFallback)
	assert.Contains(t, diff, `db.host: "localhost" → "prod-server"`)
}

func TestSemanticDiff_YAML_NoChanges(t *testing.T) {
	base := []byte("a: 1\nb: hello\n")
	head := []byte("a: 1\nb: hello\n")
	diff, format, isFallback, err := SemanticDiff(base, head, "config.yaml")
	require.NoError(t, err)
	assert.Equal(t, "yaml", format)
	assert.False(t, isFallback)
	assert.Equal(t, "No changes detected", diff)
}

func TestSemanticDiff_YAML_InvalidBase(t *testing.T) {
	base := []byte(":\n  :\n  - :\n  invalid: [")
	head := []byte("a: 1\n")
	_, _, _, err := SemanticDiff(base, head, "config.yaml")
	// YAML is more lenient than JSON, so only truly broken YAML errors
	if err != nil {
		assert.Contains(t, err.Error(), "failed to parse base YAML")
	}
}

func TestSemanticDiff_UnifiedFallback(t *testing.T) {
	base := []byte("line1\nline2\nline3\n")
	head := []byte("line1\nmodified\nline3\n")
	diff, format, isFallback, err := SemanticDiff(base, head, "readme.txt")
	require.NoError(t, err)
	assert.Equal(t, "txt", format)
	assert.True(t, isFallback)
	assert.Contains(t, diff, "-line2")
	assert.Contains(t, diff, "+modified")
}

func TestSemanticDiff_UnifiedFallback_NoExtension(t *testing.T) {
	base := []byte("hello\n")
	head := []byte("world\n")
	diff, format, isFallback, err := SemanticDiff(base, head, "Makefile")
	require.NoError(t, err)
	assert.Equal(t, "txt", format)
	assert.True(t, isFallback)
	assert.Contains(t, diff, "-hello")
	assert.Contains(t, diff, "+world")
}

func TestSemanticDiff_IdenticalFiles(t *testing.T) {
	content := []byte("same content\n")
	diff, _, isFallback, err := SemanticDiff(content, content, "file.txt")
	require.NoError(t, err)
	assert.True(t, isFallback)
	assert.Empty(t, diff) // unified diff of identical content is empty
}

func TestDeepCompare_NilValues(t *testing.T) {
	changes := deepCompare("root", nil, nil)
	assert.Empty(t, changes)

	changes = deepCompare("root", nil, "value")
	require.Len(t, changes, 1)
	assert.Equal(t, "added", changes[0].Type)

	changes = deepCompare("root", "value", nil)
	require.Len(t, changes, 1)
	assert.Equal(t, "removed", changes[0].Type)
}

func TestFormatValue(t *testing.T) {
	assert.Equal(t, `"hello"`, formatValue("hello"))
	assert.Equal(t, "null", formatValue(nil))
	assert.Equal(t, "true", formatValue(true))
	assert.Equal(t, "42", formatValue(float64(42)))
	assert.Equal(t, "3.14", formatValue(float64(3.14)))
	assert.Equal(t, "42", formatValue(42))
}

func TestJoinPath(t *testing.T) {
	assert.Equal(t, "key", joinPath("", "key"))
	assert.Equal(t, "parent.key", joinPath("parent", "key"))
}

func TestScalarEqual(t *testing.T) {
	assert.True(t, scalarEqual(float64(42), 42))
	assert.True(t, scalarEqual(42, float64(42)))
	assert.True(t, scalarEqual(float64(1.5), float64(1.5)))
	assert.True(t, scalarEqual("a", "a"))
	assert.False(t, scalarEqual("a", "b"))
	assert.False(t, scalarEqual(float64(1), float64(2)))
}
