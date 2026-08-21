package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRelativePath(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr string
	}{
		{name: "file", value: "docs/readme.md", want: "docs/readme.md"},
		{name: "normalizes leading slash", value: "/docs/readme.md", want: "docs/readme.md"},
		{name: "normalizes dot segment", value: "./.github/workflows/ci.yml", want: ".github/workflows/ci.yml"},
		{name: "normalizes duplicate separator", value: ".github//workflows/ci.yml", want: ".github/workflows/ci.yml"},
		{name: "empty", value: "", wantErr: "must not be empty"},
		{name: "current directory", value: ".", wantErr: "must identify a file"},
		{name: "double leading slash", value: "//.github/workflows/ci.yml", wantErr: "must be relative"},
		{name: "parent traversal", value: "docs/../.github/workflows/ci.yml", wantErr: "parent directory traversal"},
		{name: "leading traversal", value: "../.github/workflows/ci.yml", wantErr: "parent directory traversal"},
		{name: "backslash traversal", value: `docs\..\.github\workflows\ci.yml`, wantErr: "forward slashes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateRelativePath(tt.value)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileWriteWorkflowScopeResolvers(t *testing.T) {
	tests := []struct {
		name string
		tool inventory.ServerTool
		args map[string]any
		want inventory.ScopePolicy
	}{
		{
			name: "create regular file",
			tool: CreateOrUpdateFile(translations.NullTranslationHelper),
			args: map[string]any{"path": "docs/readme.md"},
			want: scopes.AllOfScopePolicy(scopes.Repo),
		},
		{
			name: "create workflow",
			tool: CreateOrUpdateFile(translations.NullTranslationHelper),
			args: map[string]any{"path": ".github/workflows/ci.yml"},
			want: scopes.AllOfScopePolicy(scopes.Repo, scopes.Workflow),
		},
		{
			name: "delete normalized workflow",
			tool: DeleteFile(translations.NullTranslationHelper),
			args: map[string]any{"path": "./.github/workflows/ci.yml"},
			want: scopes.AllOfScopePolicy(scopes.Repo, scopes.Workflow),
		},
		{
			name: "reject traversal instead of resolving it",
			tool: DeleteFile(translations.NullTranslationHelper),
			args: map[string]any{"path": "docs/../.github/workflows/ci.yml"},
			want: scopes.UnscopedScopePolicy(),
		},
		{
			name: "push regular files",
			tool: PushFiles(translations.NullTranslationHelper),
			args: map[string]any{"files": []any{map[string]any{"path": "README.md"}}},
			want: scopes.AllOfScopePolicy(scopes.Repo),
		},
		{
			name: "push includes workflow",
			tool: PushFiles(translations.NullTranslationHelper),
			args: map[string]any{"files": []any{
				map[string]any{"path": "README.md"},
				map[string]any{"path": ".github/workflows/ci.yml"},
			}},
			want: scopes.AllOfScopePolicy(scopes.Repo, scopes.Workflow),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.tool.ScopeResolver)
			assert.Equal(t, tt.want, tt.tool.ScopeResolver(tt.args))
		})
	}
}

func TestFileWriteToolsRejectUnsafePathsBeforeAPICalls(t *testing.T) {
	tests := []struct {
		name string
		tool inventory.ServerTool
		args map[string]any
	}{
		{
			name: "create or update",
			tool: CreateOrUpdateFile(translations.NullTranslationHelper),
			args: map[string]any{
				"owner": "owner", "repo": "repo", "path": "../workflow.yml",
				"content": "content", "message": "message", "branch": "main",
			},
		},
		{
			name: "delete",
			tool: DeleteFile(translations.NullTranslationHelper),
			args: map[string]any{
				"owner": "owner", "repo": "repo", "path": "//.github/workflows/ci.yml",
				"message": "message", "branch": "main",
			},
		},
		{
			name: "push",
			tool: PushFiles(translations.NullTranslationHelper),
			args: map[string]any{
				"owner": "owner", "repo": "repo", "branch": "main", "message": "message",
				"files": []any{map[string]any{"path": `..\workflow.yml`, "content": "content"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := BaseDeps{}
			request := createMCPRequest(tt.args)
			result, err := tt.tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.True(t, result.IsError)
			assert.Contains(t, getErrorResult(t, result).Text, "path")
		})
	}
}
