package github

import (
	"path"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// codeExtensionMimeTypes maps common code file extensions to MIME types.
// This is needed because Go's stdlib mime.TypeByExtension has gaps and wrong mappings
// for many code-related extensions (e.g., .ts returns Qt Linguist, .tsx returns nothing).
var codeExtensionMimeTypes = map[string]string{
	// JavaScript/TypeScript
	".ts":     "text/typescript",
	".tsx":    "text/typescript-jsx",
	".mts":    "text/typescript",
	".cts":    "text/typescript",
	".js":     "text/javascript",
	".jsx":    "text/javascript-jsx",
	".mjs":    "text/javascript",
	".cjs":    "text/javascript",
	".vue":    "text/x-vue",
	".svelte": "text/x-svelte",

	// Go
	".go":   "text/x-go",
	".mod":  "text/x-go-mod",
	".sum":  "text/x-go-sum",
	".work": "text/x-go-work",

	// Rust
	".rs":   "text/x-rust",
	".toml": "text/x-toml",

	// Python
	".py":  "text/x-python",
	".pyi": "text/x-python",
	".pyx": "text/x-cython",
	".pxd": "text/x-cython",

	// Ruby
	".rb":      "text/x-ruby",
	".rake":    "text/x-ruby",
	".gemspec": "text/x-ruby",
	".erb":     "text/x-erb",

	// Java/Kotlin/Scala
	".java":   "text/x-java-source",
	".kt":     "text/x-kotlin",
	".kts":    "text/x-kotlin",
	".scala":  "text/x-scala",
	".groovy": "text/x-groovy",

	// C family
	".c":   "text/x-c",
	".h":   "text/x-c",
	".cpp": "text/x-c++",
	".cc":  "text/x-c++",
	".cxx": "text/x-c++",
	".hpp": "text/x-c++",
	".hh":  "text/x-c++",
	".hxx": "text/x-c++",
	".m":   "text/x-objective-c",
	".mm":  "text/x-objective-c++",

	// C#/F#
	".cs": "text/x-csharp",
	".fs": "text/x-fsharp",

	// Swift
	".swift": "text/x-swift",

	// PHP
	".php":   "text/x-php",
	".phtml": "text/x-php",

	// Shell scripts
	".sh":   "text/x-shellscript",
	".bash": "text/x-shellscript",
	".zsh":  "text/x-shellscript",
	".fish": "text/x-shellscript",

	// Config/Data files
	".json": "application/json",
	".yml":  "text/yaml",
	".yaml": "text/yaml",
	".xml":  "text/xml",
	".ini":  "text/x-ini",
	".cfg":  "text/x-ini",
	".conf": "text/plain",
	".env":  "text/plain",

	// Markup/Documentation
	".md":       "text/markdown",
	".markdown": "text/markdown",
	".rst":      "text/x-rst",
	".adoc":     "text/asciidoc",
	".tex":      "text/x-tex",

	// Web
	".html": "text/html",
	".htm":  "text/html",
	".css":  "text/css",
	".scss": "text/x-scss",
	".sass": "text/x-sass",
	".less": "text/x-less",

	// SQL
	".sql": "text/x-sql",

	// Other languages
	".lua":  "text/x-lua",
	".r":    "text/x-r",
	".R":    "text/x-r",
	".jl":   "text/x-julia",
	".ex":   "text/x-elixir",
	".exs":  "text/x-elixir",
	".erl":  "text/x-erlang",
	".hrl":  "text/x-erlang",
	".clj":  "text/x-clojure",
	".cljs": "text/x-clojure",
	".cljc": "text/x-clojure",
	".hs":   "text/x-haskell",
	".lhs":  "text/x-haskell",
	".ml":   "text/x-ocaml",
	".mli":  "text/x-ocaml",
	".nim":  "text/x-nim",
	".dart": "text/x-dart",
	".v":    "text/x-v",
	".zig":  "text/x-zig",

	// Build/Config files
	".dockerfile": "text/x-dockerfile",
	".makefile":   "text/x-makefile",

	// Special files
	".gitignore":    "text/plain",
	".dockerignore": "text/plain",
	".editorconfig": "text/plain",
}

// isTextMIME returns true if the MIME type indicates text content.
func isTextMIME(mimeType string) bool {
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}
	// Common application/* types that are actually text
	textApplicationTypes := []string{
		"application/json",
		"application/xml",
		"application/javascript",
		"application/typescript",
		"application/x-sh",
		"application/x-shellscript",
	}
	for _, t := range textApplicationTypes {
		if mimeType == t {
			return true
		}
	}
	// Types with +json, +xml suffix are text
	if strings.HasSuffix(mimeType, "+json") || strings.HasSuffix(mimeType, "+xml") {
		return true
	}
	return false
}

// inferContentType infers the content type from file extension and optionally content.
// Returns the inferred MIME type and whether it's a text file.
func inferContentType(filePath string, content []byte) (mimeType string, isText bool) {
	ext := strings.ToLower(path.Ext(filePath))

	// Handle special filenames (Dockerfile, Makefile, etc.)
	baseName := strings.ToLower(path.Base(filePath))
	if ext == "" {
		switch baseName {
		case "dockerfile":
			return "text/x-dockerfile", true
		case "makefile", "gnumakefile":
			return "text/x-makefile", true
		case "rakefile":
			return "text/x-ruby", true
		case "gemfile":
			return "text/x-ruby", true
		case "vagrantfile":
			return "text/x-ruby", true
		case "procfile":
			return "text/plain", true
		case "readme", "license", "authors", "changelog", "contributing":
			return "text/plain", true
		}
	}

	// Check our extension map first (more accurate for code files)
	if mtype, ok := codeExtensionMimeTypes[ext]; ok {
		return mtype, isTextMIME(mtype)
	}

	// If we have content, use mimetype library for accurate detection
	if len(content) > 0 {
		mtype := mimetype.Detect(content)
		return mtype.String(), isTextMIME(mtype.String())
	}

	// Fall back to extension-only detection using mimetype library
	return inferContentTypeFromExtension(ext)
}

// inferContentTypeFromExtension infers MIME type from extension only.
// Used when we don't have file content available.
func inferContentTypeFromExtension(ext string) (mimeType string, isText bool) {
	ext = strings.ToLower(ext)

	// Check our extension map first
	if mtype, ok := codeExtensionMimeTypes[ext]; ok {
		return mtype, isTextMIME(mtype)
	}

	// Use mimetype library for other extensions
	// mimetype.Lookup returns the MIME type for a given extension
	mtype := mimetype.Lookup(ext)
	if mtype != nil {
		return mtype.String(), isTextMIME(mtype.String())
	}

	// Default to binary for unknown types
	return "application/octet-stream", false
}
