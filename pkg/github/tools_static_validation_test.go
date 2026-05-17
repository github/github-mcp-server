package github

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAllToolRegistrationsExplicitlySetReadOnlyHint statically scans every
// non-test Go source file in this package and asserts that every mcp.Tool
// composite literal explicitly sets Annotations.ReadOnlyHint.
//
// This complements TestAllToolsHaveRequiredMetadata, which can only check
// that Annotations is non-nil at runtime: Go cannot distinguish an
// unset bool field from one explicitly set to false. Source-level
// validation closes that gap and prevents future tool registrations
// from silently defaulting ReadOnlyHint to false (which has caused
// downstream agents to prompt for human approval on read-intent tools).
//
// Related issue: github/github-mcp-server#2483
func TestAllToolRegistrationsExplicitlySetReadOnlyHint(t *testing.T) {
	pkgDir, err := os.Getwd()
	require.NoError(t, err, "must be able to resolve package directory")

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, func(info os.FileInfo) bool {
		// Skip test files: they are allowed to construct mcp.Tool literals
		// for fixtures or mocks where ReadOnlyHint is not meaningful.
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	require.NoError(t, err, "parser.ParseDir on package directory")
	require.NotEmpty(t, pkgs, "expected at least one package parsed")

	type violation struct {
		file     string
		line     int
		toolName string
		reason   string
	}
	var violations []violation
	literalsSeen := 0

	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if !isMCPToolType(cl.Type) {
					return true
				}
				literalsSeen++

				toolName := extractToolName(cl)
				if toolName == "" {
					toolName = "<unknown>"
				}
				pos := fset.Position(cl.Pos())
				rel, _ := filepath.Rel(pkgDir, filename)
				if rel == "" {
					rel = filepath.Base(filename)
				}

				annotations := findFieldValue(cl, "Annotations")
				if annotations == nil {
					violations = append(violations, violation{
						file:     rel,
						line:     pos.Line,
						toolName: toolName,
						reason:   "mcp.Tool literal is missing an Annotations field",
					})
					return true
				}

				annoLit := unwrapAnnotationsLiteral(annotations)
				if annoLit == nil {
					// Annotations is set to something we can't statically
					// verify (e.g. a function call). Flag it so reviewers
					// can confirm ReadOnlyHint is honored.
					violations = append(violations, violation{
						file:     rel,
						line:     pos.Line,
						toolName: toolName,
						reason:   "Annotations is not an &mcp.ToolAnnotations{...} literal; ReadOnlyHint cannot be statically verified",
					})
					return true
				}

				if findFieldValue(annoLit, "ReadOnlyHint") == nil {
					violations = append(violations, violation{
						file:     rel,
						line:     pos.Line,
						toolName: toolName,
						reason:   "ToolAnnotations literal does not explicitly set ReadOnlyHint",
					})
				}
				return true
			})
		}
	}

	require.NotZero(t, literalsSeen,
		"expected to discover at least one mcp.Tool literal; AST walker may be broken")

	if len(violations) > 0 {
		var msg strings.Builder
		msg.WriteString("Found tool registrations that do not explicitly set ReadOnlyHint:\n")
		for _, v := range violations {
			msg.WriteString("  - ")
			msg.WriteString(v.file)
			msg.WriteString(":")
			msg.WriteString(strconv.Itoa(v.line))
			msg.WriteString(" tool=")
			msg.WriteString(v.toolName)
			msg.WriteString(": ")
			msg.WriteString(v.reason)
			msg.WriteString("\n")
		}
		msg.WriteString("\nEvery mcp.Tool registration must declare Annotations.ReadOnlyHint explicitly ")
		msg.WriteString("(true for read-only tools, false for tools with side effects). ")
		msg.WriteString("See pkg/github/tools_static_validation_test.go.")
		t.Fatal(msg.String())
	}
}

// isMCPToolType reports whether the given AST expression refers to mcp.Tool.
func isMCPToolType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "mcp" && sel.Sel != nil && sel.Sel.Name == "Tool"
}

// isMCPToolAnnotationsType reports whether the given AST expression refers to mcp.ToolAnnotations.
func isMCPToolAnnotationsType(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return ident.Name == "mcp" && sel.Sel != nil && sel.Sel.Name == "ToolAnnotations"
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
// &mcp.ToolAnnotations{...} or mcp.ToolAnnotations{...} from an expression.
// Returns nil if the expression is not a statically inspectable literal.
func unwrapAnnotationsLiteral(expr ast.Expr) *ast.CompositeLit {
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	if !isMCPToolAnnotationsType(cl.Type) {
		return nil
	}
	return cl
}

// extractToolName returns the literal value of the Name field of an mcp.Tool
// composite literal, or empty string if the value is not a basic string literal.
func extractToolName(cl *ast.CompositeLit) string {
	v := findFieldValue(cl, "Name")
	if v == nil {
		return ""
	}
	bl, ok := v.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	// Strip surrounding quotes; tolerate raw strings too.
	s := bl.Value
	if len(s) >= 2 && (s[0] == '"' || s[0] == '`') {
		s = s[1 : len(s)-1]
	}
	return s
}
