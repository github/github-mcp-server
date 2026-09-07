package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression test for https://github.com/github/github-mcp-server/issues/3189.
//
// get_file_contents returns text-file bodies ONLY inside an EmbeddedResource.
// Client surfaces that drop embedded resources (e.g. Claude voice mode) then
// show just "[non-text content: resource]" plus the short "successfully
// downloaded text file (SHA: ...)" TextContent, so the model never sees the
// file. The TextContent must also carry the file body.
func Test_GetFileContents_TextContentIncludesFileBody(t *testing.T) {
	t.Parallel()

	fileBody := "# Test Repository\n\nThis is a test repository."
	serverTool := GetFileContents(translations.NullTranslationHelper)

	mockedClient := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetReposGitRefByOwnerByRepoByRef: mockResponse(t, http.StatusOK, "{\"ref\": \"refs/heads/main\", \"object\": {\"sha\": \"\"}}"),
		GetReposByOwnerByRepo:            mockResponse(t, http.StatusOK, "{\"name\": \"repo\", \"default_branch\": \"main\"}"),
		GetReposContentsByOwnerByRepoByPath: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			fileContent := &github.RepositoryContent{
				Name:    github.Ptr("README.md"),
				Path:    github.Ptr("README.md"),
				SHA:     github.Ptr(gitBlobSHA([]byte(fileBody))),
				Type:    github.Ptr("file"),
				Content: github.Ptr(fileBody),
				Size:    github.Ptr(len(fileBody)),
			}
			contentBytes, _ := json.Marshal(fileContent)
			_, _ = w.Write(contentBytes)
		},
	})

	client := mustNewGHClient(t, mockedClient)
	mockRawClient, err := raw.NewClient(client, &url.URL{Scheme: "https", Host: "raw.example.com", Path: "/"})
	require.NoError(t, err)
	deps := BaseDeps{Client: client, RawClient: mockRawClient}
	handler := serverTool.Handler(deps)
	request := createMCPRequest(map[string]any{
		"owner": "owner",
		"repo":  "repo",
		"path":  "README.md",
		"ref":   "refs/heads/main",
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Len(t, result.Content, 2)

	// The embedded resource keeps carrying the full body (existing contract).
	resource, ok := result.Content[1].(*mcp.EmbeddedResource)
	require.True(t, ok, "expected Content[1] to be EmbeddedResource")
	require.NotNil(t, resource.Resource)
	assert.Equal(t, fileBody, resource.Resource.Text)

	// The text content must ALSO carry the body so resource-dropping clients
	// still expose the file to the model (issue #3189).
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected Content[0] to be TextContent")
	assert.Contains(t, text.Text, fileBody)
}
