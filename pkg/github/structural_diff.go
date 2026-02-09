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

// declaration represents a named top-level code construct (function, class, etc).
type declaration struct {
	Kind string // e.g. "function", "class", "type", "import"
	Name string
	Text string
}

// languageConfig maps file extensions to tree-sitter languages and the node
// types that should be treated as top-level declarations.
type languageConfig struct {
	language         *sitter.Language
	declarationKinds map[string]bool
	nameExtractor    func(node *sitter.Node, source []byte) string
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
		nameExtractor: defaultNameExtractor,
	}
}

func javascriptConfig() *languageConfig {
	return &languageConfig{
		language: javascript.GetLanguage(),
		declarationKinds: map[string]bool{
			"function_declaration": true,
			"class_declaration":    true,
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

	root := tree.RootNode()
	var decls []declaration

	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
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

	return decls, nil
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

// goNameExtractor handles Go-specific naming (method receivers, type specs).
func goNameExtractor(node *sitter.Node, source []byte) string {
	switch node.Type() {
	case "method_declaration":
		// Include receiver type: (r *Receiver) MethodName
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
	case "type_declaration":
		// type_declaration contains type_spec children
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "type_spec" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					return nameNode.Content(source)
				}
			}
		}
		return ""
	default:
		return defaultNameExtractor(node, source)
	}
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

	changes := diffDeclarations(baseDecls, headDecls)
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
func diffDeclarations(base, head []declaration) []string {
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
			changes = append(changes, fmt.Sprintf("%s %s: removed", baseDecl.Kind, baseDecl.Name))
		case !inBase && inHead:
			changes = append(changes, fmt.Sprintf("%s %s: added", headDecl.Kind, headDecl.Name))
		case baseDecl.Text != headDecl.Text:
			changes = append(changes, fmt.Sprintf("%s %s: modified", baseDecl.Kind, baseDecl.Name))
		}
	}

	return changes
}

// indexDeclarations creates a lookup map from declaration key to declaration.
// The key combines kind and name to handle same-name declarations of different kinds.
func indexDeclarations(decls []declaration) map[string]declaration {
	result := make(map[string]declaration, len(decls))
	for _, d := range decls {
		key := d.Kind + ":" + d.Name
		result[key] = d
	}
	return result
}
