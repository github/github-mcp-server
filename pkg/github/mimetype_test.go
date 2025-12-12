package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferContentType(t *testing.T) {
	tests := []struct {
		name         string
		filePath     string
		content      []byte
		wantMimeType string
		wantIsText   bool
	}{
		// JavaScript/TypeScript
		{
			name:         "TypeScript file",
			filePath:     "src/index.ts",
			content:      []byte("const foo: string = 'bar';"),
			wantMimeType: "text/typescript",
			wantIsText:   true,
		},
		{
			name:         "TSX file",
			filePath:     "src/App.tsx",
			content:      []byte("export const App = () => <div>Hello</div>;"),
			wantMimeType: "text/typescript-jsx",
			wantIsText:   true,
		},
		{
			name:         "JavaScript file",
			filePath:     "src/index.js",
			content:      []byte("const foo = 'bar';"),
			wantMimeType: "text/javascript",
			wantIsText:   true,
		},
		{
			name:         "JSX file",
			filePath:     "src/App.jsx",
			content:      []byte("export const App = () => <div>Hello</div>;"),
			wantMimeType: "text/javascript-jsx",
			wantIsText:   true,
		},
		{
			name:         "Vue file",
			filePath:     "src/App.vue",
			content:      []byte("<template><div>Hello</div></template>"),
			wantMimeType: "text/x-vue",
			wantIsText:   true,
		},

		// Go
		{
			name:         "Go file",
			filePath:     "main.go",
			content:      []byte("package main\n\nfunc main() {}"),
			wantMimeType: "text/x-go",
			wantIsText:   true,
		},
		{
			name:         "Go mod file",
			filePath:     "go.mod",
			content:      []byte("module example.com/project\n\ngo 1.21"),
			wantMimeType: "text/x-go-mod",
			wantIsText:   true,
		},
		{
			name:         "Go sum file",
			filePath:     "go.sum",
			content:      []byte("github.com/pkg/errors v0.9.1 h1:..."),
			wantMimeType: "text/x-go-sum",
			wantIsText:   true,
		},

		// Python
		{
			name:         "Python file",
			filePath:     "script.py",
			content:      []byte("def main():\n    pass"),
			wantMimeType: "text/x-python",
			wantIsText:   true,
		},
		{
			name:         "Python type stub",
			filePath:     "types.pyi",
			content:      []byte("def func() -> int: ..."),
			wantMimeType: "text/x-python",
			wantIsText:   true,
		},

		// Config files
		{
			name:         "JSON file",
			filePath:     "package.json",
			content:      []byte(`{"name": "test"}`),
			wantMimeType: "application/json",
			wantIsText:   true,
		},
		{
			name:         "YAML file",
			filePath:     ".github/workflows/ci.yml",
			content:      []byte("name: CI\non: push"),
			wantMimeType: "text/yaml",
			wantIsText:   true,
		},
		{
			name:         "TOML file",
			filePath:     "Cargo.toml",
			content:      []byte("[package]\nname = \"test\""),
			wantMimeType: "text/x-toml",
			wantIsText:   true,
		},

		// Markup/Documentation
		{
			name:         "Markdown file",
			filePath:     "README.md",
			content:      []byte("# Title\n\nSome text"),
			wantMimeType: "text/markdown",
			wantIsText:   true,
		},
		{
			name:         "HTML file",
			filePath:     "index.html",
			content:      []byte("<!DOCTYPE html><html></html>"),
			wantMimeType: "text/html",
			wantIsText:   true,
		},

		// Special filenames without extensions
		{
			name:         "Dockerfile",
			filePath:     "Dockerfile",
			content:      []byte("FROM ubuntu:latest\nRUN apt-get update"),
			wantMimeType: "text/x-dockerfile",
			wantIsText:   true,
		},
		{
			name:         "Makefile",
			filePath:     "Makefile",
			content:      []byte("all:\n\techo hello"),
			wantMimeType: "text/x-makefile",
			wantIsText:   true,
		},
		{
			name:         "README without extension",
			filePath:     "README",
			content:      []byte("This is a readme file"),
			wantMimeType: "text/plain",
			wantIsText:   true,
		},
		{
			name:         "LICENSE without extension",
			filePath:     "LICENSE",
			content:      []byte("MIT License\n\nCopyright..."),
			wantMimeType: "text/plain",
			wantIsText:   true,
		},

		// Binary content detection
		{
			name:         "PNG image",
			filePath:     "image.png",
			content:      []byte("\x89PNG\r\n\x1a\n"),
			wantMimeType: "image/png",
			wantIsText:   false,
		},
		{
			name:         "JPEG image",
			filePath:     "photo.jpg",
			content:      []byte("\xff\xd8\xff"),
			wantMimeType: "image/jpeg",
			wantIsText:   false,
		},
		{
			name:         "ZIP archive",
			filePath:     "archive.zip",
			content:      []byte("PK\x03\x04"),
			wantMimeType: "application/zip",
			wantIsText:   false,
		},
		{
			name:         "ELF binary",
			filePath:     "program",
			content:      []byte("\x7fELF"),
			wantMimeType: "application/x-elf",
			wantIsText:   false,
		},

		// Shell scripts with shebang
		{
			name:         "Bash script",
			filePath:     "script.sh",
			content:      []byte("#!/bin/bash\necho hello"),
			wantMimeType: "text/x-shellscript",
			wantIsText:   true,
		},
		{
			name:         "Python script with shebang",
			filePath:     "tool",
			content:      []byte("#!/usr/bin/env python3\nprint('hello')"),
			wantMimeType: "text/x-python",
			wantIsText:   true,
		},

		// Other languages
		{
			name:         "Rust file",
			filePath:     "main.rs",
			content:      []byte("fn main() {\n    println!(\"Hello\");\n}"),
			wantMimeType: "text/x-rust",
			wantIsText:   true,
		},
		{
			name:         "Ruby file",
			filePath:     "app.rb",
			content:      []byte("puts 'hello'"),
			wantMimeType: "text/x-ruby",
			wantIsText:   true,
		},
		{
			name:         "Java file",
			filePath:     "Main.java",
			content:      []byte("public class Main { }"),
			wantMimeType: "text/x-java-source",
			wantIsText:   true,
		},
		{
			name:         "C++ file",
			filePath:     "main.cpp",
			content:      []byte("#include <iostream>\nint main() { }"),
			wantMimeType: "text/x-c++",
			wantIsText:   true,
		},
		{
			name:         "C header file",
			filePath:     "header.h",
			content:      []byte("#ifndef HEADER_H\n#define HEADER_H\n#endif"),
			wantMimeType: "text/x-c",
			wantIsText:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMimeType, gotIsText := inferContentType(tt.filePath, tt.content)
			assert.Equal(t, tt.wantMimeType, gotMimeType, "MIME type mismatch")
			assert.Equal(t, tt.wantIsText, gotIsText, "isText flag mismatch")
		})
	}
}

func TestInferContentTypeFromExtension(t *testing.T) {
	tests := []struct {
		name         string
		ext          string
		wantMimeType string
		wantIsText   bool
	}{
		// Code files
		{
			name:         "TypeScript",
			ext:          ".ts",
			wantMimeType: "text/typescript",
			wantIsText:   true,
		},
		{
			name:         "JavaScript",
			ext:          ".js",
			wantMimeType: "text/javascript",
			wantIsText:   true,
		},
		{
			name:         "Go",
			ext:          ".go",
			wantMimeType: "text/x-go",
			wantIsText:   true,
		},
		{
			name:         "Python",
			ext:          ".py",
			wantMimeType: "text/x-python",
			wantIsText:   true,
		},

		// Config files
		{
			name:         "JSON",
			ext:          ".json",
			wantMimeType: "application/json",
			wantIsText:   true,
		},
		{
			name:         "YAML",
			ext:          ".yml",
			wantMimeType: "text/yaml",
			wantIsText:   true,
		},

		// Unknown extension should default to binary
		{
			name:         "Unknown extension",
			ext:          ".xyz",
			wantMimeType: "application/octet-stream",
			wantIsText:   false,
		},

		// Empty extension
		{
			name:         "Empty extension",
			ext:          "",
			wantMimeType: "application/octet-stream",
			wantIsText:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMimeType, gotIsText := inferContentTypeFromExtension(tt.ext)
			assert.Equal(t, tt.wantMimeType, gotMimeType, "MIME type mismatch")
			assert.Equal(t, tt.wantIsText, gotIsText, "isText flag mismatch")
		})
	}
}

func TestIsTextMIME(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
		want     bool
	}{
		// Text types
		{
			name:     "text/plain",
			mimeType: "text/plain",
			want:     true,
		},
		{
			name:     "text/markdown",
			mimeType: "text/markdown",
			want:     true,
		},
		{
			name:     "text/html",
			mimeType: "text/html",
			want:     true,
		},

		// Application types that are text
		{
			name:     "application/json",
			mimeType: "application/json",
			want:     true,
		},
		{
			name:     "application/xml",
			mimeType: "application/xml",
			want:     true,
		},
		{
			name:     "application/javascript",
			mimeType: "application/javascript",
			want:     true,
		},

		// Types with +json suffix
		{
			name:     "application/vnd.api+json",
			mimeType: "application/vnd.api+json",
			want:     true,
		},

		// Types with +xml suffix
		{
			name:     "application/atom+xml",
			mimeType: "application/atom+xml",
			want:     true,
		},

		// Binary types
		{
			name:     "image/png",
			mimeType: "image/png",
			want:     false,
		},
		{
			name:     "application/zip",
			mimeType: "application/zip",
			want:     false,
		},
		{
			name:     "application/pdf",
			mimeType: "application/pdf",
			want:     false,
		},
		{
			name:     "application/octet-stream",
			mimeType: "application/octet-stream",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTextMIME(tt.mimeType)
			assert.Equal(t, tt.want, got)
		})
	}
}
