package toolvalidation_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolvalidation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writePackage writes a single Go source file into a fresh temp directory and
// returns that directory, suitable for passing to ScanReadOnlyHint.
func writePackage(t *testing.T, filename, source string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(source), 0o600))
	return dir
}

func TestScanReadOnlyHint(t *testing.T) {
	t.Parallel()

	const compliant = `package fixture

import "github.com/modelcontextprotocol/go-sdk/mcp"

var Tool = mcp.Tool{
	Name: "compliant_tool",
	Annotations: &mcp.ToolAnnotations{
		ReadOnlyHint: true,
	},
}
`

	const missingHint = `package fixture

import "github.com/modelcontextprotocol/go-sdk/mcp"

var Tool = mcp.Tool{
	Name: "missing_hint",
	Annotations: &mcp.ToolAnnotations{
		Title: "no hint",
	},
}
`

	const missingAnnotations = `package fixture

import "github.com/modelcontextprotocol/go-sdk/mcp"

var Tool = mcp.Tool{
	Name: "missing_annotations",
}
`

	const nonLiteralAnnotations = `package fixture

import "github.com/modelcontextprotocol/go-sdk/mcp"

func annotations() *mcp.ToolAnnotations { return &mcp.ToolAnnotations{ReadOnlyHint: true} }

var Tool = mcp.Tool{
	Name:        "non_literal",
	Annotations: annotations(),
}
`

	const unkeyedTool = `package fixture

import "github.com/modelcontextprotocol/go-sdk/mcp"

var Tool = mcp.Tool{"unkeyed", "desc", nil, nil, nil, nil}
`

	const aliasedImport = `package fixture

import sdk "github.com/modelcontextprotocol/go-sdk/mcp"

var Tool = sdk.Tool{
	Name: "aliased",
	Annotations: &sdk.ToolAnnotations{
		ReadOnlyHint: false,
	},
}
`

	const noMCPImport = `package fixture

import "fmt"

var _ = fmt.Sprintln("nothing to scan here")
`

	cases := []struct {
		name           string
		source         string
		expectCount    int
		expectReason   string
		expectToolName string
	}{
		{name: "compliant literal passes", source: compliant, expectCount: 0},
		{name: "aliased import is detected", source: aliasedImport, expectCount: 0},
		{name: "file without mcp import is skipped", source: noMCPImport, expectCount: 0},
		{
			name:           "missing ReadOnlyHint is flagged",
			source:         missingHint,
			expectCount:    1,
			expectReason:   "does not explicitly set ReadOnlyHint",
			expectToolName: "missing_hint",
		},
		{
			name:           "missing Annotations is flagged",
			source:         missingAnnotations,
			expectCount:    1,
			expectReason:   "missing an Annotations field",
			expectToolName: "missing_annotations",
		},
		{
			name:           "non-literal Annotations is flagged",
			source:         nonLiteralAnnotations,
			expectCount:    1,
			expectReason:   "not an &mcp.ToolAnnotations{...} literal",
			expectToolName: "non_literal",
		},
		{
			name:           "positional Tool fields are flagged",
			source:         unkeyedTool,
			expectCount:    1,
			expectReason:   "positional (unkeyed) fields",
			expectToolName: "<unknown>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := writePackage(t, "fixture.go", tc.source)
			violations, err := toolvalidation.ScanReadOnlyHint(dir)
			require.NoError(t, err)
			require.Len(t, violations, tc.expectCount)
			if tc.expectCount == 0 {
				return
			}
			v := violations[0]
			assert.Equal(t, "fixture.go", v.File)
			assert.Greater(t, v.Line, 0)
			assert.Equal(t, tc.expectToolName, v.ToolName)
			assert.Contains(t, v.Reason, tc.expectReason)
		})
	}
}

func TestFormatReadOnlyHintViolations(t *testing.T) {
	t.Parallel()

	assert.Empty(t, toolvalidation.FormatReadOnlyHintViolations(nil))

	msg := toolvalidation.FormatReadOnlyHintViolations([]toolvalidation.ReadOnlyHintViolation{{
		File:     "issues.go",
		Line:     42,
		ToolName: "issue_read",
		Reason:   "ToolAnnotations literal does not explicitly set ReadOnlyHint",
	}})
	assert.True(t, strings.HasPrefix(msg, "Found tool registrations that do not explicitly set ReadOnlyHint:"))
	assert.Contains(t, msg, "issues.go:42 tool=issue_read")
	assert.Contains(t, msg, "true for read-only tools, false for tools with side effects")
}

func TestScanReadOnlyHint_ReturnsErrorForMissingDirectory(t *testing.T) {
	t.Parallel()
	_, err := toolvalidation.ScanReadOnlyHint(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}
