package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepositoryRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    RepositoryRef
		wantErr bool
	}{
		{
			name:  "owner slash repo",
			input: "github/github-mcp-server",
			want: RepositoryRef{
				Owner:    "github",
				Repo:     "github-mcp-server",
				FullName: "github/github-mcp-server",
			},
		},
		{
			name:  "https url",
			input: "https://github.com/github/github-mcp-server",
			want: RepositoryRef{
				Owner:    "github",
				Repo:     "github-mcp-server",
				FullName: "github/github-mcp-server",
			},
		},
		{
			name:  "ssh remote",
			input: "git@github.com:github/github-mcp-server.git",
			want: RepositoryRef{
				Owner:    "github",
				Repo:     "github-mcp-server",
				FullName: "github/github-mcp-server",
			},
		},
		{
			name:    "invalid",
			input:   "just-a-name",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRepositoryRef(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestResolveRepositoryFocusConfig(t *testing.T) {
	t.Parallel()

	ref, focus, err := ResolveRepositoryFocusConfig("github/github-mcp-server", false)
	require.NoError(t, err)
	assert.Equal(t, "github/github-mcp-server", ref.FullName)
	assert.True(t, focus)

	_, focus, err = ResolveRepositoryFocusConfig("github/github-mcp-server", true)
	require.NoError(t, err)
	assert.False(t, focus)

	_, focus, err = ResolveRepositoryFocusConfig("", false)
	require.NoError(t, err)
	assert.False(t, focus)
}

func TestCreateRepositoryFocusFilter(t *testing.T) {
	t.Parallel()

	filter := CreateRepositoryFocusFilter(true)
	include, err := filter(context.Background(), &inventory.ServerTool{Tool: mcp.Tool{Name: "search_repositories"}})
	require.NoError(t, err)
	assert.False(t, include)

	include, err = filter(context.Background(), &inventory.ServerTool{Tool: mcp.Tool{Name: "list_issues"}})
	require.NoError(t, err)
	assert.True(t, include)
}

func TestGetRepositoryContext(t *testing.T) {
	t.Parallel()

	serverTool := GetRepositoryContext(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.Equal(t, "get_repository_context", tool.Name)

	mockRepo := &github.Repository{
		Private: github.Ptr(true),
		Permissions: &github.RepositoryPermissions{
			Admin: github.Ptr(false),
			Push:  github.Ptr(false),
			Pull:  github.Ptr(true),
		},
	}

	deps := NewBaseDeps(
		mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, mockRepo),
		})),
		nil,
		nil,
		nil,
		translations.NullTranslationHelper,
		FeatureFlags{},
		0,
		nil,
		RepositoryContextConfig{
			DefaultRepository: &RepositoryRef{
				Owner:    "github",
				Repo:     "github-mcp-server",
				FullName: "github/github-mcp-server",
			},
			FocusMode: true,
			Token:     "github_pat_test",
		},
		stubExporters(),
	)

	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{})
	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.NotNil(t, result)

	text := getTextResult(t, result).Text
	assert.Contains(t, text, `"full_name":"github/github-mcp-server"`)
	assert.Contains(t, text, `"focus_mode":true`)
	assert.Contains(t, text, `"token_type":"fine_grained_pat"`)
	assert.Contains(t, text, `"accessible":true`)
}

func TestVerifyRepositoryAccessFineGrainedHint(t *testing.T) {
	t.Parallel()

	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposByOwnerByRepo: mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}),
	}))

	status := VerifyRepositoryAccess(
		context.Background(),
		client,
		RepositoryRef{Owner: "friend", Repo: "private-repo", FullName: "friend/private-repo"},
		utils.TokenTypeFineGrainedPersonalAccessToken,
	)

	assert.False(t, status.Accessible)
	assert.Contains(t, status.Hint, "Fine-grained personal access tokens")
	assert.Contains(t, status.Hint, "friend/private-repo")
}
