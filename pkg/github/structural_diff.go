package github

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/c"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/ruby"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// DiffFormatStructural indicates a tree-sitter based structural diff.
const DiffFormatStructural DiffFormat = "structural"

// maxStructuralDiffDepth limits recursion into nested declarations.
const maxStructuralDiffDepth = 5

// declaration represents a named top-level code construct (function, class, etc).
type declaration struct {
	Kind string // e.g. "function", "class", "type", "import"
	Name string
	Text string
}

// languageConfig maps file extensions to tree-sitter languages and the node
// types that should be treated as top-level declarations.
type languageConfig struct {
	language                 *sitter.Language
	declarationKinds         map[string]bool
	nameExtractor            func(node *sitter.Node, source []byte) string
	indentationIsSignificant bool
}

// languageForPath returns the tree-sitter language config for a file path, or nil if unsupported.
func languageForPath(path string) *languageConfig {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return goConfig()
	case ".py":
		return pythonConfig()
	case ".js", ".mjs", ".cjs":
		return javascriptConfig()
	case ".ts":
		return typescriptConfig()
	case ".tsx", ".jsx":
		return tsxConfig()
	case ".rb":
		return rubyConfig()
	case ".rs":
		return rustConfig()
	case ".java":
		return javaConfig()
	case ".c", ".h":
		return cConfig()
	case ".cpp", ".hpp", ".cc", ".cxx":
		return cppConfig()
	default:
		return nil
	}
}

func goConfig() *languageConfig {
	return &languageConfig{
		language: golang.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_declaration": true,
			"method_declaration":   true,
			"type_declaration":     true,
			"var_declaration":      true,
			"const_declaration":    true,
			"import_declaration":   true,
			"package_clause":       true,
		},
		nameExtractor: goNameExtractor,
	}
}

func pythonConfig() *languageConfig {
	return &languageConfig{
		language: python.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_definition":   true,
			"class_definition":      true,
			"import_statement":      true,
			"import_from_statement": true,
		},
		nameExtractor:            defaultNameExtractor,
		indentationIsSignificant: true,
	}
}

func javascriptConfig() *languageConfig {
	return &languageConfig{
		language: javascript.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_declaration": true,
			"class_declaration":    true,
			"method_definition":    true,
			"export_statement":     true,
			"import_statement":     true,
			"lexical_declaration":  true,
			"variable_declaration": true,
		},
		nameExtractor: jsNameExtractor,
	}
}

func typescriptConfig() *languageConfig {
	return &languageConfig{
		language: typescript.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_declaration":   true,
			"class_declaration":      true,
			"method_definition":      true,
			"export_statement":       true,
			"import_statement":       true,
			"lexical_declaration":    true,
			"variable_declaration":   true,
			"interface_declaration":  true,
			"type_alias_declaration": true,
			"enum_declaration":       true,
		},
		nameExtractor: jsNameExtractor,
	}
}

func tsxConfig() *languageConfig {
	return &languageConfig{
		language: tsx.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_declaration":   true,
			"class_declaration":      true,
			"method_definition":      true,
			"export_statement":       true,
			"import_statement":       true,
			"lexical_declaration":    true,
			"variable_declaration":   true,
			"interface_declaration":  true,
			"type_alias_declaration": true,
			"enum_declaration":       true,
		},
		nameExtractor: jsNameExtractor,
	}
}

func rubyConfig() *languageConfig {
	return &languageConfig{
		language: ruby.GetLanguage(),
		declarationKinds: map[string]bool{
			"method": true,
			"class":  true,
			"module": true,
		},
		nameExtractor: defaultNameExtractor,
	}
}

func rustConfig() *languageConfig {
	return &languageConfig{
		language: rust.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_item":   true,
			"struct_item":     true,
			"enum_item":       true,
			"impl_item":       true,
			"trait_item":      true,
			"mod_item":        true,
			"use_declaration": true,
			"type_item":       true,
			"const_item":      true,
			"static_item":     true,
		},
		nameExtractor: defaultNameExtractor,
	}
}

func javaConfig() *languageConfig {
	return &languageConfig{
		language: java.GetLanguage(),
		declarationKinds: map[string]bool{
			"class_declaration":       true,
			"method_declaration":      true,
			"interface_declaration":   true,
			"enum_declaration":        true,
			"import_declaration":      true,
			"package_declaration":     true,
			"constructor_declaration": true,
		},
		nameExtractor: defaultNameExtractor,
	}
}

func cConfig() *languageConfig {
	return &languageConfig{
		language: c.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_definition": true,
			"declaration":         true,
			"preproc_include":     true,
			"preproc_def":         true,
			"struct_specifier":    true,
			"enum_specifier":      true,
			"type_definition":     true,
		},
		nameExtractor: cNameExtractor,
	}
}

func cppConfig() *languageConfig {
	return &languageConfig{
		language: cpp.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_definition":  true,
			"declaration":          true,
			"preproc_include":      true,
			"preproc_def":          true,
			"struct_specifier":     true,
			"enum_specifier":       true,
			"class_specifier":      true,
			"type_definition":      true,
			"namespace_definition": true,
			"template_declaration": true,
		},
		nameExtractor: cNameExtractor,
	}
}

// extractDeclarations parses source code and extracts top-level declarations.
func extractDeclarations(config *languageConfig, source []byte) ([]declaration, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(config.language)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}
	defer tree.Close()

	return extractChildDeclarations(config, tree.RootNode(), source), nil
}

// extractChildDeclarations extracts declarations from the direct children of a node.
func extractChildDeclarations(config *languageConfig, node *sitter.Node, source []byte) []declaration {
	var decls []declaration

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		nodeType := child.Type()

		if !config.declarationKinds[nodeType] {
			continue
		}

		name := config.nameExtractor(child, source)
		if name == "" {
			name = fmt.Sprintf("_%s_%d", nodeType, i)
		}

		decls = append(decls, declaration{
			Kind: nodeType,
			Name: name,
			Text: child.Content(source),
		})
	}

	return decls
}

// defaultNameExtractor finds the first "name" or "identifier" child node.
func defaultNameExtractor(node *sitter.Node, source []byte) string {
	nameNode := node.ChildByFieldName("name")
	if nameNode != nil {
		return nameNode.Content(source)
	}
	// Try first identifier child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "identifier" || child.Type() == "type_identifier" {
			return child.Content(source)
		}
	}
	return ""
}

// goNameExtractor handles Go-specific naming (method receivers, type/var/const specs).
func goNameExtractor(node *sitter.Node, source []byte) string {
	switch node.Type() {
	case "method_declaration":
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			return ""
		}
		name := nameNode.Content(source)
		receiver := node.ChildByFieldName("receiver")
		if receiver != nil {
			return fmt.Sprintf("(%s).%s", extractReceiverType(receiver, source), name)
		}
		return name
	case "type_declaration", "var_declaration", "const_declaration":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			nameNode := child.ChildByFieldName("name")
			if nameNode != nil {
				return nameNode.Content(source)
			}
		}
		return ""
	case "import_declaration":
		return summarizeImport(node, source)
	case "package_clause":
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "package_identifier" {
				return "package " + child.Content(source)
			}
		}
		return "package"
	default:
		return defaultNameExtractor(node, source)
	}
}

// summarizeImport produces a concise name for an import declaration by
// extracting the imported package paths.
func summarizeImport(node *sitter.Node, source []byte) string {
	var paths []string
	collectImportPaths(node, source, &paths)
	if len(paths) == 0 {
		return node.Content(source)
	}
	if len(paths) <= 3 {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s, %s, ... (%d packages)", paths[0], paths[1], len(paths))
}

// extractReceiverType extracts the type name from a Go method receiver.
func extractReceiverType(receiver *sitter.Node, source []byte) string {
	for i := 0; i < int(receiver.ChildCount()); i++ {
		child := receiver.Child(i)
		if child.Type() == "parameter_declaration" {
			typeNode := child.ChildByFieldName("type")
			if typeNode != nil {
				return typeNode.Content(source)
			}
		}
	}
	return receiver.Content(source)
}

// collectImportPaths extracts package path strings from an import node tree.
func collectImportPaths(node *sitter.Node, source []byte, paths *[]string) {
	if node.Type() == "interpreted_string_literal" || node.Type() == "raw_string_literal" {
		// Strip quotes
		content := node.Content(source)
		content = strings.Trim(content, "\"'`")
		// Use short form: last path component
		if idx := strings.LastIndex(content, "/"); idx >= 0 {
			content = content[idx+1:]
		}
		*paths = append(*paths, content)
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectImportPaths(node.Child(i), source, paths)
	}
}

// jsNameExtractor handles JS/TS-specific naming (variable declarations, exports).
func jsNameExtractor(node *sitter.Node, source []byte) string {
	switch node.Type() {
	case "lexical_declaration", "variable_declaration":
		// const/let/var x = ...
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					return nameNode.Content(source)
				}
			}
		}
		return ""
	case "export_statement":
		// export default/named - use the inner declaration's name
		decl := node.ChildByFieldName("declaration")
		if decl != nil {
			return jsNameExtractor(decl, source)
		}
		return defaultNameExtractor(node, source)
	case "import_statement":
		return node.Content(source)
	default:
		return defaultNameExtractor(node, source)
	}
}

// cNameExtractor handles C/C++ naming where the function name is inside the declarator.
func cNameExtractor(node *sitter.Node, source []byte) string {
	// function_definition: the name is in the declarator field
	declarator := node.ChildByFieldName("declarator")
	if declarator != nil {
		return findIdentifier(declarator, source)
	}
	return defaultNameExtractor(node, source)
}

// findIdentifier recursively searches for the first identifier in a node tree.
func findIdentifier(node *sitter.Node, source []byte) string {
	if node.Type() == "identifier" {
		return node.Content(source)
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if name := findIdentifier(node.Child(i), source); name != "" {
			return name
		}
	}
	return ""
}

// structuralDiff produces a structural diff using tree-sitter AST parsing.
func structuralDiff(path string, base, head []byte) SemanticDiffResult {
	config := languageForPath(path)
	if config == nil {
		return SemanticDiffResult{
			Format: DiffFormatUnified,
			Diff:   unifiedDiff(path, base, head),
		}
	}

	baseDecls, err := extractDeclarations(config, base)
	if err != nil {
		return fallbackResult(path, base, head, "failed to parse base file")
	}

	headDecls, err := extractDeclarations(config, head)
	if err != nil {
		return fallbackResult(path, base, head, "failed to parse head file")
	}

	changes := diffDeclarations(config, baseDecls, headDecls, "", 0)
	if len(changes) == 0 {
		return SemanticDiffResult{
			Format: DiffFormatStructural,
			Diff:   "no structural changes detected",
		}
	}

	return SemanticDiffResult{
		Format: DiffFormatStructural,
		Diff:   strings.Join(changes, "\n"),
	}
}

// diffDeclarations compares two sets of declarations and returns change descriptions.
// indent controls visual nesting, depth limits recursion.
func diffDeclarations(config *languageConfig, base, head []declaration, indent string, depth int) []string {
	baseMap := indexDeclarations(base)
	headMap := indexDeclarations(head)

	// Collect all unique keys
	allKeys := make(map[string]bool)
	for k := range baseMap {
		allKeys[k] = true
	}
	for k := range headMap {
		allKeys[k] = true
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var changes []string
	for _, key := range sortedKeys {
		baseDecl, inBase := baseMap[key]
		headDecl, inHead := headMap[key]

		switch {
		case inBase && !inHead:
			changes = append(changes, fmt.Sprintf("%s%s %s: removed", indent, baseDecl.Kind, baseDecl.Name))
		case !inBase && inHead:
			sig := declarationSignature(headDecl.Text)
			if sig != "" && sig != headDecl.Name {
				changes = append(changes, fmt.Sprintf("%s%s %s: added\n%s  %s", indent, headDecl.Kind, headDecl.Name, indent, sig))
			} else {
				changes = append(changes, fmt.Sprintf("%s%s %s: added", indent, headDecl.Kind, headDecl.Name))
			}
		case baseDecl.Text != headDecl.Text:
			detail := modifiedDetail(config, baseDecl, headDecl, indent, depth)
			changes = append(changes, fmt.Sprintf("%s%s %s: modified\n%s", indent, baseDecl.Kind, baseDecl.Name, detail))
		}
	}

	return changes
}

// indexDeclarations creates a lookup map from declaration key to declaration.
// The key combines kind and name to handle same-name declarations of different kinds.
// Import and package declarations use kind-only keys since they're typically
// singletons and their "name" changes when contents change.
func indexDeclarations(decls []declaration) map[string]declaration {
	result := make(map[string]declaration, len(decls))
	importCount := 0
	for _, d := range decls {
		var key string
		switch d.Kind {
		case "import_declaration", "import_statement", "import_from_statement",
			"package_clause", "package_declaration":
			key = fmt.Sprintf("%s:%d", d.Kind, importCount)
			importCount++
		default:
			key = d.Kind + ":" + d.Name
		}
		result[key] = d
	}
	return result
}

// declarationSignature returns the first line of a declaration, which typically
// contains the signature (e.g., "func hello(name string) error {").
func declarationSignature(text string) string {
	if idx := strings.Index(text, "\n"); idx >= 0 {
		return strings.TrimSpace(text[:idx])
	}
	return strings.TrimSpace(text)
}

// modifiedDetail produces the detail output for a modified declaration. If the
// declaration contains sub-declarations (e.g. methods in a class) and we haven't
// hit the depth limit, it recurses to show which children changed. Otherwise it
// falls back to a line-level diff of the declaration body.
func modifiedDetail(config *languageConfig, baseDecl, headDecl declaration, indent string, depth int) string {
	if depth < maxStructuralDiffDepth {
		baseChildren := extractChildDeclarationsFromText(config, baseDecl.Text)
		headChildren := extractChildDeclarationsFromText(config, headDecl.Text)

		if len(baseChildren) > 0 || len(headChildren) > 0 {
			nested := diffDeclarations(config, baseChildren, headChildren, indent+"  ", depth+1)
			if len(nested) > 0 {
				return strings.Join(nested, "\n")
			}
		}
	}

	return declarationDiff(baseDecl.Text, headDecl.Text, indent+"  ", config.indentationIsSignificant)
}

// extractChildDeclarationsFromText parses a declaration's text and extracts any
// nested declarations (e.g. methods inside a class body).
func extractChildDeclarationsFromText(config *languageConfig, text string) []declaration {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(config.language)

	src := []byte(text)
	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil
	}
	defer tree.Close()

	// Walk all descendants looking for declaration nodes that aren't the root wrapper
	root := tree.RootNode()
	var decls []declaration
	findNestedDeclarations(config, root, src, &decls, 0, true)
	return decls
}

// findNestedDeclarations recursively walks AST nodes to find declarations that
// are nested inside a parent. skipRoot=true on the first call to avoid matching
// the re-parsed wrapper of the parent declaration itself.
func findNestedDeclarations(config *languageConfig, node *sitter.Node, source []byte, decls *[]declaration, depth int, skipRoot bool) {
	if depth > maxStructuralDiffDepth {
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		nodeType := child.Type()

		if !skipRoot && config.declarationKinds[nodeType] {
			name := config.nameExtractor(child, source)
			if name == "" {
				name = fmt.Sprintf("_%s_%d", nodeType, i)
			}
			*decls = append(*decls, declaration{
				Kind: nodeType,
				Name: name,
				Text: child.Content(source),
			})
		} else {
			// Keep walking into non-declaration nodes (e.g. class_body)
			findNestedDeclarations(config, child, source, decls, depth+1, false)
		}
	}
}

// declarationDiff produces a compact, indented diff showing what changed inside
// a modified declaration. For languages where indentation is not significant,
// lines are compared with leading whitespace normalized so that pure formatting
// changes are collapsed. For whitespace-significant languages like Python,
// indentation differences are preserved as meaningful changes.
func declarationDiff(baseText, headText string, indent string, indentationIsSignificant bool) string {
	baseLines := strings.Split(baseText, "\n")
	headLines := strings.Split(headText, "\n")

	var baseCmp, headCmp []string
	if indentationIsSignificant {
		baseCmp = baseLines
		headCmp = headLines
	} else {
		baseCmp = trimLines(baseLines)
		headCmp = trimLines(headLines)
	}

	// Compute LCS — on trimmed content for brace languages, exact for whitespace-significant
	lcsIndices := longestCommonSubsequence(baseCmp, headCmp)

	var buf strings.Builder
	bi, hi, li := 0, 0, 0

	for li < len(lcsIndices) {
		bIdx := lcsIndices[li][0]
		hIdx := lcsIndices[li][1]

		// Lines removed from base before this common line
		for bi < bIdx {
			buf.WriteString(indent + "- " + baseLines[bi] + "\n")
			bi++
		}
		// Lines added in head before this common line
		for hi < hIdx {
			buf.WriteString(indent + "+ " + headLines[hi] + "\n")
			hi++
		}
		// Common line (by trimmed content) — skip silently
		bi++
		hi++
		li++
	}

	// Remaining lines after LCS is exhausted
	for bi < len(baseLines) {
		buf.WriteString(indent + "- " + baseLines[bi] + "\n")
		bi++
	}
	for hi < len(headLines) {
		buf.WriteString(indent + "+ " + headLines[hi] + "\n")
		hi++
	}

	result := strings.TrimRight(buf.String(), "\n")
	if result == "" {
		return indent + "(whitespace/formatting changes only)"
	}
	return result
}

// trimLines returns a slice with each line's leading/trailing whitespace removed.
func trimLines(lines []string) []string {
	trimmed := make([]string, len(lines))
	for i, l := range lines {
		trimmed[i] = strings.TrimSpace(l)
	}
	return trimmed
}

// longestCommonSubsequence returns index pairs [baseIdx, headIdx] for matching
// lines between a and b.
func longestCommonSubsequence(a, b []string) [][2]int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			switch {
			case a[i-1] == b[j-1]:
				dp[i][j] = dp[i-1][j-1] + 1
			case dp[i-1][j] >= dp[i][j-1]:
				dp[i][j] = dp[i-1][j]
			default:
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to find index pairs
	result := make([][2]int, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		switch {
		case a[i-1] == b[j-1]:
			result = append(result, [2]int{i - 1, j - 1})
			i--
			j--
		case dp[i-1][j] >= dp[i][j-1]:
			i--
		default:
			j--
		}
	}

	// Reverse
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}

	return result
}
