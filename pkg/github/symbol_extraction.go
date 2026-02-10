package github

import (
	"fmt"
	"strings"
)

// ExtractSymbol searches source code for a named symbol and returns its text.
// It searches top-level declarations first, then recursively searches nested
// declarations (e.g. methods inside classes). Returns the symbol text and its
// kind, or an error if the symbol is not found or the language is unsupported.
func ExtractSymbol(path string, source []byte, symbolName string) (text string, kind string, err error) {
	config := languageForPath(path)
	if config == nil {
		return "", "", fmt.Errorf("symbol extraction is not supported for this file type")
	}

	decls, err := extractDeclarations(config, source)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse file: %w", err)
	}

	// Search top-level declarations
	if text, kind, found := findSymbol(decls, symbolName); found {
		return text, kind, nil
	}

	// Search nested declarations (methods inside classes, etc.)
	for _, decl := range decls {
		nested := extractChildDeclarationsFromText(config, decl.Text)
		if text, kind, found := findSymbol(nested, symbolName); found {
			return text, kind, nil
		}
	}

	// Build list of available symbols for the error message
	available := listSymbolNames(config, decls)

	// Suggest closest match for bare method names
	if suggestion := findClosestMatch(available, symbolName); suggestion != "" {
		return "", "", fmt.Errorf("symbol %q not found. Did you mean %q? Available symbols: %s",
			symbolName, suggestion, strings.Join(available, ", "))
	}
	return "", "", fmt.Errorf("symbol %q not found. Available symbols: %s", symbolName, strings.Join(available, ", "))
}

// findSymbol searches a slice of declarations for a matching name.
// It first tries an exact match, then falls back to suffix matching
// (e.g., "RegisterRoutes" matches "(*Handler).RegisterRoutes") when
// there is exactly one unambiguous match.
func findSymbol(decls []declaration, name string) (string, string, bool) {
	for _, d := range decls {
		if d.Name == name {
			return d.Text, d.Kind, true
		}
	}

	// Suffix match: accept bare method name when unambiguous
	var matches []declaration
	suffix := "." + name
	for _, d := range decls {
		if strings.HasSuffix(d.Name, suffix) {
			matches = append(matches, d)
		}
	}
	if len(matches) == 1 {
		return matches[0].Text, matches[0].Kind, true
	}

	return "", "", false
}

// listSymbolNames returns all symbol names from top-level and one level of
// nested declarations, for use in error messages.
func listSymbolNames(config *languageConfig, decls []declaration) []string {
	var names []string
	for _, d := range decls {
		if !strings.HasPrefix(d.Name, "_") {
			names = append(names, d.Name)
		}
		nested := extractChildDeclarationsFromText(config, d.Text)
		for _, n := range nested {
			if !strings.HasPrefix(n.Name, "_") {
				names = append(names, n.Name)
			}
		}
	}
	return names
}

// findClosestMatch looks for a symbol name that ends with ".name" or contains
// the search term as a substring, returning the best suggestion.
func findClosestMatch(available []string, name string) string {
	suffix := "." + name
	var suffixMatches []string
	for _, s := range available {
		if strings.HasSuffix(s, suffix) {
			suffixMatches = append(suffixMatches, s)
		}
	}
	if len(suffixMatches) == 1 {
		return suffixMatches[0]
	}
	return ""
}
