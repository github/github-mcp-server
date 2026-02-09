package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStructuralDiffGo(t *testing.T) {
	tests := []struct {
		name         string
		base         string
		head         string
		expectedDiff string
		notContains  string
	}{
		{
			name: "no changes",
			base: `package main

func hello() {}
`,
			head: `package main

func hello() {}
`,
			expectedDiff: "no structural changes detected",
		},
		{
			name: "function added",
			base: `package main

func hello() {}
`,
			head: `package main

func hello() {}

func goodbye() {}
`,
			expectedDiff: "function_declaration goodbye: added",
		},
		{
			name: "function removed",
			base: `package main

func hello() {}

func goodbye() {}
`,
			head: `package main

func hello() {}
`,
			expectedDiff: "function_declaration goodbye: removed",
		},
		{
			name: "function modified",
			base: `package main

func hello() {
	fmt.Println("hello")
}
`,
			head: `package main

func hello() {
	fmt.Println("world")
}
`,
			expectedDiff: "function_declaration hello: modified",
		},
		{
			name: "function reorder only",
			base: `package main

func a() {}

func b() {}
`,
			head: `package main

func b() {}

func a() {}
`,
			expectedDiff: "no structural changes detected",
		},
		{
			name: "method with receiver",
			base: `package main

type Server struct{}

func (s *Server) Start() {}
`,
			head: `package main

type Server struct{}

func (s *Server) Start() {
	fmt.Println("starting")
}
`,
			expectedDiff: "(*Server).Start: modified",
		},
		{
			name: "type added",
			base: `package main
`,
			head: `package main

type Config struct {
	Host string
}
`,
			expectedDiff: "type_declaration Config: added",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff("main.go", []byte(tc.base), []byte(tc.head))
			require.Equal(t, DiffFormatStructural, result.Format)
			assert.Contains(t, result.Diff, tc.expectedDiff)
			if tc.notContains != "" {
				assert.NotContains(t, result.Diff, tc.notContains)
			}
		})
	}
}

func TestStructuralDiffPython(t *testing.T) {
	tests := []struct {
		name         string
		base         string
		head         string
		expectedDiff string
	}{
		{
			name: "function added",
			base: `def hello():
    pass
`,
			head: `def hello():
    pass

def goodbye():
    pass
`,
			expectedDiff: "function_definition goodbye: added",
		},
		{
			name: "class modified",
			base: `class Foo:
    def bar(self):
        return 1
`,
			head: `class Foo:
    def bar(self):
        return 2
`,
			expectedDiff: "class_definition Foo: modified",
		},
		{
			name: "function reorder",
			base: `def a():
    pass

def b():
    pass
`,
			head: `def b():
    pass

def a():
    pass
`,
			expectedDiff: "no structural changes detected",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff("app.py", []byte(tc.base), []byte(tc.head))
			require.Equal(t, DiffFormatStructural, result.Format)
			assert.Contains(t, result.Diff, tc.expectedDiff)
		})
	}
}

func TestStructuralDiffJavaScript(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		base         string
		head         string
		expectedDiff string
	}{
		{
			name: "function added",
			path: "app.js",
			base: `function hello() {
  console.log("hello");
}
`,
			head: `function hello() {
  console.log("hello");
}

function goodbye() {
  console.log("goodbye");
}
`,
			expectedDiff: "function_declaration goodbye: added",
		},
		{
			name: "const variable modified",
			path: "config.js",
			base: `const PORT = 3000;
`,
			head: `const PORT = 8080;
`,
			expectedDiff: "lexical_declaration PORT: modified",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff(tc.path, []byte(tc.base), []byte(tc.head))
			require.Equal(t, DiffFormatStructural, result.Format)
			assert.Contains(t, result.Diff, tc.expectedDiff)
		})
	}
}

func TestStructuralDiffTypeScript(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		base         string
		head         string
		expectedDiff string
	}{
		{
			name: "interface added",
			path: "types.ts",
			base: `interface User {
  name: string;
}
`,
			head: `interface User {
  name: string;
}

interface Admin {
  role: string;
}
`,
			expectedDiff: "interface_declaration Admin: added",
		},
		{
			name: "TSX component modified",
			path: "App.tsx",
			base: `function App() {
  return <div>Hello</div>;
}
`,
			head: `function App() {
  return <div>World</div>;
}
`,
			expectedDiff: "function_declaration App: modified",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SemanticDiff(tc.path, []byte(tc.base), []byte(tc.head))
			require.Equal(t, DiffFormatStructural, result.Format)
			assert.Contains(t, result.Diff, tc.expectedDiff)
		})
	}
}

func TestStructuralDiffRust(t *testing.T) {
	result := SemanticDiff("lib.rs", []byte(`fn hello() {}
`), []byte(`fn hello() {}

fn goodbye() {}
`))
	require.Equal(t, DiffFormatStructural, result.Format)
	assert.Contains(t, result.Diff, "function_item goodbye: added")
}

func TestStructuralDiffJava(t *testing.T) {
	result := SemanticDiff("Main.java",
		[]byte(`public class Main {
    public static void main(String[] args) {}
}
`),
		[]byte(`public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`))
	require.Equal(t, DiffFormatStructural, result.Format)
	assert.Contains(t, result.Diff, "class_declaration Main: modified")
}

func TestStructuralDiffC(t *testing.T) {
	result := SemanticDiff("main.c",
		[]byte(`#include <stdio.h>

int main() {
    return 0;
}
`),
		[]byte(`#include <stdio.h>

int main() {
    printf("hello\n");
    return 0;
}
`))
	require.Equal(t, DiffFormatStructural, result.Format)
	assert.Contains(t, result.Diff, "main: modified")
}

func TestStructuralDiffRuby(t *testing.T) {
	result := SemanticDiff("app.rb",
		[]byte(`def hello
  puts "hello"
end
`),
		[]byte(`def hello
  puts "hello"
end

def goodbye
  puts "goodbye"
end
`))
	require.Equal(t, DiffFormatStructural, result.Format)
	assert.Contains(t, result.Diff, "method goodbye: added")
}

func TestStructuralDiffUnsupportedFallback(t *testing.T) {
	// .txt files have no tree-sitter grammar, should fall back to unified
	result := SemanticDiff("notes.txt", []byte("hello\n"), []byte("world\n"))
	assert.Equal(t, DiffFormatUnified, result.Format)
	assert.Contains(t, result.Diff, "--- a/notes.txt")
}

func TestLanguageForPath(t *testing.T) {
	supported := []string{
		"main.go", "app.py", "index.js", "index.mjs",
		"app.ts", "App.tsx", "App.jsx",
		"lib.rs", "Main.java", "main.c", "main.h",
		"main.cpp", "main.hpp", "main.cc",
		"app.rb",
	}
	for _, path := range supported {
		t.Run(path, func(t *testing.T) {
			assert.NotNil(t, languageForPath(path), "expected language config for %s", path)
		})
	}

	unsupported := []string{
		"config.json", "data.yaml", "notes.txt", "Makefile", "README.md",
	}
	for _, path := range unsupported {
		t.Run(path, func(t *testing.T) {
			assert.Nil(t, languageForPath(path), "expected no language config for %s", path)
		})
	}
}

func TestDetectDiffFormatStructural(t *testing.T) {
	assert.Equal(t, DiffFormatStructural, DetectDiffFormat("main.go"))
	assert.Equal(t, DiffFormatStructural, DetectDiffFormat("app.py"))
	assert.Equal(t, DiffFormatStructural, DetectDiffFormat("index.js"))
	assert.Equal(t, DiffFormatJSON, DetectDiffFormat("config.json"))
	assert.Equal(t, DiffFormatUnified, DetectDiffFormat("notes.txt"))
}
