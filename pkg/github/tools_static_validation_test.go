package github

import (
	"os"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolvalidation"
	"github.com/stretchr/testify/require"
)

// TestAllToolRegistrationsExplicitlySetReadOnlyHint statically scans every
// non-test Go source file in this package and asserts that every mcp.Tool
// composite literal explicitly sets Annotations.ReadOnlyHint.
//
// The AST scan itself lives in pkg/toolvalidation so downstream packages
// (e.g. github/github-mcp-server-remote) can apply the same guardrail to
// their own tool registrations without duplicating the parser logic.
//
// This complements TestAllToolsHaveRequiredMetadata, which can only check
// that Annotations is non-nil at runtime: Go cannot distinguish an unset
// bool field from one explicitly set to false. Source-level validation
// closes that gap and prevents future tool registrations from silently
// defaulting ReadOnlyHint to false (which has caused downstream agents to
// prompt for human approval on read-intent tools).
//
// Related issue: github/github-mcp-server#2483
func TestAllToolRegistrationsExplicitlySetReadOnlyHint(t *testing.T) {
	pkgDir, err := os.Getwd()
	require.NoError(t, err, "must be able to resolve package directory")

	violations, err := toolvalidation.ScanReadOnlyHint(pkgDir)
	require.NoError(t, err)
	if len(violations) > 0 {
		t.Fatal(toolvalidation.FormatReadOnlyHintViolations(violations))
	}
}
