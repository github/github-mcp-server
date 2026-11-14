package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGetFileContentsModernProtocolUsesTransportSafeLimit(t *testing.T) {
	const fileName = "medium-file.txt"
	fileSHA := strings.Repeat("b", 40)

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposGitRefByOwnerByRepoByRef: mockResponse(t, http.StatusOK, "{\"ref\": \"refs/heads/main\", \"object\": {\"sha\": \"\"}}"),
		GetReposByOwnerByRepo:            mockResponse(t, http.StatusOK, "{\"name\": \"repo\", \"default_branch\": \"main\"}"),
		"GET /repos/owner/repo/git/trees/refs/heads/main": mockResponse(t, http.StatusOK, &github.Tree{
			Entries: []*github.TreeEntry{{
				Path: new(fileName),
				Mode: new("100644"),
				Type: new("blob"),
				SHA:  new(fileSHA),
			}},
		}),
		GetReposContentsByOwnerByRepoByPath: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			fileContent := map[string]any{
				"name":         fileName,
				"path":         fileName,
				"sha":          fileSHA,
				"type":         "file",
				"size":         500 * 1024,
				"download_url": "https://raw.githubusercontent.com/owner/repo/main/" + fileName,
			}
			contentBytes, err := json.Marshal(fileContent)
			require.NoError(t, err)
			_, err = w.Write(contentBytes)
			require.NoError(t, err)
		},
	})

	deps := BaseDeps{Client: mustNewGHClient(t, mockedClient)}
	tool := GetFileContents(translations.NullTranslationHelper)
	request := createMCPRequest(map[string]any{
		"owner": "owner",
		"repo":  "repo",
		"path":  fileName,
		"ref":   "refs/heads/main",
	})
	request.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: inventory.ProtocolVersionMultiRoundTrip}

	result, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 2)

	resourceLink, ok := result.Content[1].(*mcp.ResourceLink)
	require.True(t, ok, "modern clients should receive a ResourceLink before the 1 MiB wire limit")
	require.Equal(t, "repo://owner/repo/refs/heads/main/contents/"+fileName, resourceLink.URI)
	require.Equal(t, fileName, resourceLink.Name)
}

func TestRepositoryContentSizeLimit(t *testing.T) {
	modernRequest := createMCPRequest(map[string]any{})
	modernRequest.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: inventory.ProtocolVersionMultiRoundTrip}

	legacyRequest := createMCPRequest(map[string]any{})
	legacyRequest.Params.Meta = mcp.Meta{mcp.MetaKeyProtocolVersion: "2025-11-25"}

	require.Equal(t, modernRepositoryContentSizeLimit, repositoryContentSizeLimit(&modernRequest))
	require.Equal(t, legacyRepositoryContentSizeLimit, repositoryContentSizeLimit(&legacyRequest))
	require.Equal(t, legacyRepositoryContentSizeLimit, repositoryContentSizeLimit(nil))
}
