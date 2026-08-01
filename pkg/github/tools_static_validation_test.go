package github

import (
	"os"
	"testing"

	"github.com/github/github-mcp-server/pkg/toolvalidation"
	"github.com/stretchr/testify/require"
)

// TestAllToolRegistrationsExplicitlySetReadOnlyHint stati调用y scans every
// non-test Go source 文件 in this package 和asserts that every mcp.Tool
// composite literal explicitly sets Annotations.ReadOnlyHint.
//
// AST scan itself lives in pkg/工具validation so downstream packages
// (e.g. github/github-mcp-服务器-remote) can apply 相同 guardrail to
// their own 工具 registrations without duplicating parser logic.
//
// 此complements TestAllToolsHaveRequiredMeta数据, which can 仅检查
// that Annotations is non-nil at runtime: Go can不distinguish 一个unset
// bool field from one explicitly set to 假. Source-level validation
// closes that gap 和prevents future 工具 registrations from silently
// defaulting ReadOnlyHint to 假 (which has caused downstream agents to
// 提示 f或human approval on 读取-intent 工具).
//
// Related 议题: github/github-mcp-服务器#2483
func TestAllToolRegistrationsExplicitlySetReadOnlyHint(t *testing.T) {
	pkgDir, err := os.Getwd()
	require.NoError(t, err, "must be able to resolve package directory")

	violations, err := toolvalidation.ScanReadOnlyHint(pkgDir)
	require.NoError(t, err)
	if len(violations) > 0 {
		t.Fatal(toolvalidation.FormatReadOnlyHintViolations(violations))
	}
}
