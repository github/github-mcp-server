// Package toolvalidation provides source-level (AST) validators for MCP tool
// registrations. It is intended to be consumed from _test.go files in any
// package that registers mcp.Tool literals (including downstream repositories
// such as github-mcp-server-remote) so the same guardrails apply everywhere
// without duplicating the parsing logic.
package toolvalidation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MCPImportPath is the canonical module path of the MCP go-sdk. Source files
// that import this path under any alias (including the default `mcp`) are
// candidates for tool-literal validation.
const MCPImportPath = "github.com/modelcontextprotocol/go-sdk/mcp"

// ReadOnlyHintViolation describes a single mcp.Tool composite literal that
// failed the ReadOnlyHint check.
type ReadOnlyHintViolation struct {
	// File is the path to the offending source file, made relative to the
	// scan directory when possible.
	File string
	// Line is the 1-indexed line number of the offending literal.
	Line int
	// ToolName is the value of the Name field on the mcp.Tool literal, or
	// "<unknown>" when it cannot be statically extracted.
	ToolName string
	// Reason is a human-readable explanation of why the literal failed.
	Reason string
}

// String renders a violation in the format used by FormatReadOnlyHintViolations:
// "<file>:<line> tool=<name>: <reason>".
func (v ReadOnlyHintViolation) String() string {
	return fmt.Sprintf("%s:%d tool=%s: %s", v.File, v.Line, v.ToolName, v.Reason)
}

// ScanReadOnlyHint parses every non-test .go file in dir (a single package
// directory) and returns a violation for each mcp.Tool composite literal that
// does not explicitly set Annotations.ReadOnlyHint.
//
// The Go runtime cannot distinguish an unset bool field from one explicitly
// set to false, so this AST-level check exists to prevent future tool
// registrations from silently defaulting ReadOnlyHint to false — which has
// triggered downstream agents to prompt for human approval on safe read
// operations.
//
// Callers typically invoke this from a _test.go file:
//
//	dir, _ := os.Getwd()
//	violations, err := toolvalidation.ScanReadOnlyHint(dir)
func ScanReadOnlyHint(dir string) ([]ReadOnlyHintViolation, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(info os.FileInfo) bool {
		// Skip test files: they are allowed to construct mcp.Tool literals
		// for fixtures or mocks where ReadOnlyHint is not meaningful.
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse package directory %q: %w", dir, err)
	}

	var violations []ReadOnlyHintViolation
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			aliases := mcpAliasesFor(file)
			if len(aliases) == 0 {
				continue
			}
			rel, relErr := filepath.Rel(dir, filename)
			if relErr != nil || rel == "" {
				rel = filepath.Base(filename)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if !isQualifiedType(cl.Type, aliases, "Tool") {
					return true
				}
				violations = append(violations, checkToolLiteral(cl, aliases, rel, fset.Position(cl.Pos()).Line)...)
				return true
			})
		}
	}
	return violations, nil
}

// FormatReadOnlyHintViolations renders a single multi-line error message
// suitable for passing to t.Fatal. Returns "" when violations is empty.
func FormatReadOnlyHintViolations(violations []ReadOnlyHintViolation) string {
	if len(violations) == 0 {
		return ""
	}
	var msg strings.Builder
	msg.WriteString("Found tool registrations that do not explicitly set ReadOnlyHint:\n")
	for _, v := range violations {
		msg.WriteString("  - ")
		msg.WriteString(v.String())
		msg.WriteByte('\n')
	}
	msg.WriteString("\nEvery mcp.Tool registration must declare Annotations.ReadOnlyHint explicitly ")
	msg.WriteString("(true for read-only tools, false for tools with side effects). ")
	msg.WriteString("See pkg/toolvalidation.ScanReadOnlyHint.")
	return msg.String()
}

func checkToolLiteral(cl *ast.CompositeLit, aliases map[string]struct{}, file string, line int) []ReadOnlyHintViolation {
	toolName := extractToolName(cl)
	if toolName == "" {
		toolName = "<unknown>"
	}
	mk := func(reason string) ReadOnlyHintViolation {
		return ReadOnlyHintViolation{File: file, Line: line, ToolName: toolName, Reason: reason}
	}

	if hasUnkeyedFields(cl) {
		return []ReadOnlyHintViolation{mk("mcp.Tool literal uses positional (unkeyed) fields; this check requires keyed fields so Annotations.ReadOnlyHint can be verified")}
	}

	annotations := findFieldValue(cl, "Annotations")
	if annotations == nil {
		return []ReadOnlyHintViolation{mk("mcp.Tool literal is missing an Annotations field")}
	}

	annoLit := unwrapAnnotationsLiteral(annotations, aliases)
	if annoLit == nil {
		return []ReadOnlyHintViolation{mk("Annotations is not an &mcp.ToolAnnotations{...} literal; ReadOnlyHint cannot be statically verified")}
	}

	if hasUnkeyedFields(annoLit) {
		return []ReadOnlyHintViolation{mk("mcp.ToolAnnotations literal uses positional (unkeyed) fields; use keyed fields so ReadOnlyHint can be verified")}
	}

	if findFieldValue(annoLit, "ReadOnlyHint") == nil {
		return []ReadOnlyHintViolation{mk("ToolAnnotations literal does not explicitly set ReadOnlyHint")}
	}
	return nil
}

// mcpAliasesFor returns the set of local identifiers under which the given
// file imports the MCP go-sdk (MCPImportPath). The default unaliased import
// resolves to the package name "mcp". Blank (`_`) and dot (`.`) imports are
// skipped because tool literals cannot meaningfully be qualified through them.
func mcpAliasesFor(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != MCPImportPath {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			aliases[imp.Name.Name] = struct{}{}
			continue
		}
		aliases["mcp"] = struct{}{}
	}
	return aliases
}

// isQualifiedType reports whether expr is a SelectorExpr of the form
// <alias>.<typeName> where alias is in the provided alias set.
func isQualifiedType(expr ast.Expr, aliases map[string]struct{}, typeName string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := aliases[ident.Name]; !ok {
		return false
	}
	return sel.Sel != nil && sel.Sel.Name == typeName
}

// hasUnkeyedFields reports whether the composite literal has any positional
// (non-key/value) elements. The static check cannot reliably map positional
// fields without full type information, so such literals are rejected with a
// dedicated diagnostic rather than producing false "missing field" violations.
func hasUnkeyedFields(cl *ast.CompositeLit) bool {
	for _, elt := range cl.Elts {
		if _, ok := elt.(*ast.KeyValueExpr); !ok {
			return true
		}
	}
	return false
}

// findFieldValue returns the value expression for the named keyed field of a
// composite literal, or nil if the field is absent.
func findFieldValue(cl *ast.CompositeLit, name string) ast.Expr {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		if key.Name == name {
			return kv.Value
		}
	}
	return nil
}

// unwrapAnnotationsLiteral attempts to extract the *ast.CompositeLit for
// &mcp.ToolAnnotations{...} or mcp.ToolAnnotations{...} from an expression,
// resolving the MCP package's local alias per file.
func unwrapAnnotationsLiteral(expr ast.Expr, aliases map[string]struct{}) *ast.CompositeLit {
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	if !isQualifiedType(cl.Type, aliases, "ToolAnnotations") {
		return nil
	}
	return cl
}

// extractToolName returns the literal value of the Name field of an mcp.Tool
// composite literal, or empty string if the value is not a basic string literal.
// Interpreted ("...") and raw (`...`) string literals are handled via
// strconv.Unquote so embedded escapes are decoded correctly; the raw
// literal value is returned as a best-effort fallback if unquoting fails.
func extractToolName(cl *ast.CompositeLit) string {
	v := findFieldValue(cl, "Name")
	if v == nil {
		return ""
	}
	bl, ok := v.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	if unq, err := strconv.Unquote(bl.Value); err == nil {
		return unq
	}
	return bl.Value
}
