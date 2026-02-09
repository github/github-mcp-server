package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSymbol(t *testing.T) {
	t.Run("Go function", func(t *testing.T) {
		source := []byte("package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n\nfunc world() {\n\tfmt.Println(\"world\")\n}\n")
		text, kind, err := ExtractSymbol("main.go", source, "hello")
		require.NoError(t, err)
		assert.Equal(t, "function_declaration", kind)
		assert.Contains(t, text, "func hello()")
		assert.Contains(t, text, "hello")
		assert.NotContains(t, text, "world")
	})

	t.Run("Go method with receiver", func(t *testing.T) {
		source := []byte("package main\n\ntype Server struct{}\n\nfunc (s *Server) Start() {\n\tlog.Println(\"start\")\n}\n\nfunc (s *Server) Stop() {\n\tlog.Println(\"stop\")\n}\n")
		text, kind, err := ExtractSymbol("main.go", source, "(*Server).Start")
		require.NoError(t, err)
		assert.Equal(t, "method_declaration", kind)
		assert.Contains(t, text, "Start")
		assert.NotContains(t, text, "Stop")
	})

	t.Run("Go type", func(t *testing.T) {
		source := []byte("package main\n\ntype Config struct {\n\tHost string\n\tPort int\n}\n")
		text, kind, err := ExtractSymbol("main.go", source, "Config")
		require.NoError(t, err)
		assert.Equal(t, "type_declaration", kind)
		assert.Contains(t, text, "Host string")
	})

	t.Run("Python function", func(t *testing.T) {
		source := []byte("def hello():\n    print('hello')\n\ndef world():\n    print('world')\n")
		text, kind, err := ExtractSymbol("app.py", source, "hello")
		require.NoError(t, err)
		assert.Equal(t, "function_definition", kind)
		assert.Contains(t, text, "print('hello')")
		assert.NotContains(t, text, "world")
	})

	t.Run("Python class method (nested)", func(t *testing.T) {
		source := []byte("class Dog:\n    def bark(self):\n        return 'woof'\n    def fetch(self):\n        return 'ball'\n")
		text, kind, err := ExtractSymbol("app.py", source, "bark")
		require.NoError(t, err)
		assert.Equal(t, "function_definition", kind)
		assert.Contains(t, text, "woof")
		assert.NotContains(t, text, "ball")
	})

	t.Run("TypeScript class", func(t *testing.T) {
		source := []byte("class Api {\n  get() {\n    return fetch('/data');\n  }\n}\n\nfunction helper() { return 1; }\n")
		text, kind, err := ExtractSymbol("api.ts", source, "Api")
		require.NoError(t, err)
		assert.Equal(t, "class_declaration", kind)
		assert.Contains(t, text, "get()")
		assert.NotContains(t, text, "helper")
	})

	t.Run("TypeScript class method (nested)", func(t *testing.T) {
		source := []byte("class Api {\n  get() {\n    return fetch('/data');\n  }\n  post() {\n    return fetch('/post');\n  }\n}\n")
		text, kind, err := ExtractSymbol("api.ts", source, "get")
		require.NoError(t, err)
		assert.Equal(t, "method_definition", kind)
		assert.Contains(t, text, "/data")
		assert.NotContains(t, text, "/post")
	})

	t.Run("symbol not found lists available", func(t *testing.T) {
		source := []byte("package main\n\nfunc hello() {}\n\nfunc world() {}\n")
		_, _, err := ExtractSymbol("main.go", source, "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "hello")
		assert.Contains(t, err.Error(), "world")
	})

	t.Run("unsupported file type", func(t *testing.T) {
		source := []byte("some content")
		_, _, err := ExtractSymbol("README.md", source, "anything")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not supported")
	})

	t.Run("Java class with methods", func(t *testing.T) {
		source := []byte("class Calculator {\n  int add(int a, int b) {\n    return a + b;\n  }\n  int multiply(int a, int b) {\n    return a * b;\n  }\n}\n")
		text, kind, err := ExtractSymbol("Calculator.java", source, "add")
		require.NoError(t, err)
		assert.Equal(t, "method_declaration", kind)
		assert.Contains(t, text, "a + b")
		assert.NotContains(t, text, "a * b")
	})

	t.Run("Rust function", func(t *testing.T) {
		source := []byte("fn hello() {\n    println!(\"hello\");\n}\n\nfn world() {\n    println!(\"world\");\n}\n")
		text, kind, err := ExtractSymbol("main.rs", source, "hello")
		require.NoError(t, err)
		assert.Equal(t, "function_item", kind)
		assert.Contains(t, text, "hello")
		assert.NotContains(t, text, "world")
	})

	t.Run("Go var declaration", func(t *testing.T) {
		source := []byte("package main\n\nvar defaultTimeout = 30\n\nvar maxRetries = 3\n")
		text, kind, err := ExtractSymbol("main.go", source, "defaultTimeout")
		require.NoError(t, err)
		assert.Equal(t, "var_declaration", kind)
		assert.Contains(t, text, "30")
		assert.NotContains(t, text, "maxRetries")
	})
}
