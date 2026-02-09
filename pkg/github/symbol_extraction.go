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
	return "", "", fmt.Errorf("symbol %q not found. Available symbols: %s", symbolName, strings.Join(available, ", "))
}

// findSymbol searches a slice of declarations for a matching name.
func findSymbol(decls []declaration, name string) (string, string, bool) {
	for _, d := range decls {
		if d.Name == name {
			return d.Text, d.Kind, true
		}
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
