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

// mcpImportPath is the canonical module path of the MCP go-sdk that pkg/github
// imports as `mcp` (or under an alias). Per-file alias resolution lets this
// test correctly identify mcp.Tool / mcp.ToolAnnotations literals even when a
// file imports the SDK under a non-default local name.
const mcpImportPath = "github.com/modelcontextprotocol/go-sdk/mcp"

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

	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			aliases := mcpAliasesFor(file)
			if len(aliases) == 0 {
				// File does not import the MCP go-sdk; no tool literals possible.
				continue
			}
			ast.Inspect(file, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if !isQualifiedType(cl.Type, aliases, "Tool") {
					return true
				}

				toolName := extractToolName(cl)
				if toolName == "" {
					toolName = "<unknown>"
				}
				pos := fset.Position(cl.Pos())
				rel, _ := filepath.Rel(pkgDir, filename)
				if rel == "" {
					rel = filepath.Base(filename)
				}

				if hasUnkeyedFields(cl) {
					violations = append(violations, violation{
						file:     rel,
						line:     pos.Line,
						toolName: toolName,
						reason:   "mcp.Tool literal uses positional (unkeyed) fields; this check requires keyed fields so Annotations.ReadOnlyHint can be verified",
					})
					return true
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

				annoLit := unwrapAnnotationsLiteral(annotations, aliases)
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

				if hasUnkeyedFields(annoLit) {
					violations = append(violations, violation{
						file:     rel,
						line:     pos.Line,
						toolName: toolName,
						reason:   "mcp.ToolAnnotations literal uses positional (unkeyed) fields; use keyed fields so ReadOnlyHint can be verified",
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

	// Intentionally do not assert that any literals were observed: if tool
	// registrations move behind constructors/factories there may be nothing
	// for this check to validate, and that is a legitimate state.

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

// mcpAliasesFor returns the set of local identifiers under which the given
// file imports the MCP go-sdk (mcpImportPath). The default unaliased import
// resolves to the package name "mcp". Blank (`_`) and dot (`.`) imports are
// skipped because tool literals cannot meaningfully be qualified through them.
func mcpAliasesFor(file *ast.File) map[string]struct{} {
	aliases := map[string]struct{}{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != mcpImportPath {
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
